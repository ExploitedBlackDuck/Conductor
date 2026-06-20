// Package secrets implements Conductor's at-rest sealing (ADR-0009): a
// per-install data key held in the OS keyring, and an XChaCha20-Poly1305 AEAD
// used to seal sensitive persisted fields — captured job logs and any saved
// sensitive values — before they touch disk. The core depends on the
// ports.SecretStore abstraction; the concrete keyring lives in an adapter.
package secrets

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"

	"golang.org/x/crypto/chacha20poly1305"

	"github.com/conductor-app/conductor/internal/core/coreerr"
	"github.com/conductor-app/conductor/internal/core/ports"
)

// dataKeyName is the keyring entry under which the per-install data key is
// stored.
const dataKeyName = "data-key"

// Sealed is the result of sealing plaintext: a random nonce and the AEAD
// ciphertext. Both are stored; neither is secret on its own.
type Sealed struct {
	// Nonce is the per-seal random XChaCha20 nonce (24 bytes).
	Nonce []byte
	// Ciphertext is the AEAD output (ciphertext + authentication tag).
	Ciphertext []byte
}

// Sealer seals and opens data with the per-install data key using
// XChaCha20-Poly1305. Its large random nonce makes per-seal random nonces safe
// without a counter.
type Sealer struct {
	aead cipher
	rand io.Reader
}

// cipher is the subset of cipher.AEAD the Sealer uses; naming it locally keeps
// the dependency explicit.
type cipher interface {
	Seal(dst, nonce, plaintext, additionalData []byte) []byte
	Open(dst, nonce, ciphertext, additionalData []byte) ([]byte, error)
	NonceSize() int
}

// NewSealer constructs a Sealer from a 32-byte data key. It returns an error
// for a key of the wrong length.
func NewSealer(key []byte) (*Sealer, error) {
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, fmt.Errorf("constructing AEAD: %w", err)
	}
	return &Sealer{aead: aead, rand: rand.Reader}, nil
}

// Seal encrypts plaintext, binding additionalData (which is authenticated but
// not encrypted — e.g. an operation ID) to the ciphertext. additionalData may
// be nil.
func (s *Sealer) Seal(plaintext, additionalData []byte) (Sealed, error) {
	nonce := make([]byte, s.aead.NonceSize())
	if _, err := io.ReadFull(s.rand, nonce); err != nil {
		return Sealed{}, fmt.Errorf("generating nonce: %w", err)
	}
	ct := s.aead.Seal(nil, nonce, plaintext, additionalData)
	return Sealed{Nonce: nonce, Ciphertext: ct}, nil
}

// Open decrypts a Sealed value, verifying additionalData matches what was sealed.
// A tampered nonce, ciphertext, or additionalData fails authentication.
func (s *Sealer) Open(sealed Sealed, additionalData []byte) ([]byte, error) {
	if len(sealed.Nonce) != s.aead.NonceSize() {
		return nil, fmt.Errorf("invalid nonce length %d", len(sealed.Nonce))
	}
	pt, err := s.aead.Open(nil, sealed.Nonce, sealed.Ciphertext, additionalData)
	if err != nil {
		return nil, fmt.Errorf("opening sealed value: %w", err)
	}
	return pt, nil
}

// LoadOrCreateDataKey returns the per-install data key, generating and storing a
// fresh random key in the secret store on first run (ADR-0009). The key is
// stored base64-encoded.
func LoadOrCreateDataKey(ctx context.Context, store ports.SecretStore) ([]byte, error) {
	return loadOrCreateDataKey(ctx, store, rand.Reader)
}

// loadOrCreateDataKey is the testable core, taking the randomness source
// explicitly.
func loadOrCreateDataKey(ctx context.Context, store ports.SecretStore, randSrc io.Reader) ([]byte, error) {
	encoded, err := store.Get(ctx, dataKeyName)
	switch {
	case err == nil:
		key, decErr := base64.StdEncoding.DecodeString(encoded)
		if decErr != nil {
			return nil, coreerr.New(coreerr.CodeSecretUnavailable, "stored data key is corrupt", decErr)
		}
		if len(key) != chacha20poly1305.KeySize {
			return nil, coreerr.New(coreerr.CodeSecretUnavailable, "stored data key has wrong length", nil)
		}
		return key, nil
	case errors.Is(err, ports.ErrSecretNotFound):
		key := make([]byte, chacha20poly1305.KeySize)
		if _, rErr := io.ReadFull(randSrc, key); rErr != nil {
			return nil, fmt.Errorf("generating data key: %w", rErr)
		}
		if sErr := store.Set(ctx, dataKeyName, base64.StdEncoding.EncodeToString(key)); sErr != nil {
			return nil, coreerr.New(coreerr.CodeSecretUnavailable, "storing data key in keyring", sErr)
		}
		return key, nil
	default:
		return nil, coreerr.New(coreerr.CodeSecretUnavailable, "reading data key from keyring", err)
	}
}
