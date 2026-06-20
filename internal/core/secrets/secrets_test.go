package secrets

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/chacha20poly1305"

	"github.com/conductor-app/conductor/internal/core/coreerr"
	"github.com/conductor-app/conductor/internal/core/ports"
)

// fakeSecretStore is an in-memory ports.SecretStore for tests.
type fakeSecretStore struct {
	values  map[string]string
	failGet error
}

func newFakeSecretStore() *fakeSecretStore {
	return &fakeSecretStore{values: map[string]string{}}
}

func (f *fakeSecretStore) Get(_ context.Context, key string) (string, error) {
	if f.failGet != nil {
		return "", f.failGet
	}
	v, ok := f.values[key]
	if !ok {
		return "", ports.ErrSecretNotFound
	}
	return v, nil
}

func (f *fakeSecretStore) Set(_ context.Context, key, value string) error {
	f.values[key] = value
	return nil
}

func (f *fakeSecretStore) Delete(_ context.Context, key string) error {
	delete(f.values, key)
	return nil
}

func newTestSealer(t *testing.T) *Sealer {
	t.Helper()
	key := bytes.Repeat([]byte{0x42}, chacha20poly1305.KeySize)
	s, err := NewSealer(key)
	require.NoError(t, err)
	return s
}

func TestSealOpenRoundTrip(t *testing.T) {
	t.Parallel()
	s := newTestSealer(t)

	plaintext := []byte("captured rclone job log with a token in a url")
	aad := []byte("operation-42")

	sealed, err := s.Seal(plaintext, aad)
	require.NoError(t, err)
	assert.NotEqual(t, plaintext, sealed.Ciphertext, "ciphertext must not equal plaintext")
	assert.Len(t, sealed.Nonce, chacha20poly1305.NonceSizeX)

	opened, err := s.Open(sealed, aad)
	require.NoError(t, err)
	assert.Equal(t, plaintext, opened)
}

func TestSealNonceIsUniquePerCall(t *testing.T) {
	t.Parallel()
	s := newTestSealer(t)

	a, err := s.Seal([]byte("same"), nil)
	require.NoError(t, err)
	b, err := s.Seal([]byte("same"), nil)
	require.NoError(t, err)

	assert.NotEqual(t, a.Nonce, b.Nonce, "each seal must use a fresh nonce")
	assert.NotEqual(t, a.Ciphertext, b.Ciphertext)
}

func TestOpenRejectsTampering(t *testing.T) {
	t.Parallel()
	s := newTestSealer(t)

	sealed, err := s.Seal([]byte("integrity matters"), []byte("aad"))
	require.NoError(t, err)

	t.Run("flipped ciphertext byte", func(t *testing.T) {
		t.Parallel()
		bad := Sealed{Nonce: sealed.Nonce, Ciphertext: bytes.Clone(sealed.Ciphertext)}
		bad.Ciphertext[0] ^= 0xff
		_, err := s.Open(bad, []byte("aad"))
		require.Error(t, err)
	})

	t.Run("wrong additional data", func(t *testing.T) {
		t.Parallel()
		_, err := s.Open(sealed, []byte("different-aad"))
		require.Error(t, err)
	})

	t.Run("wrong key cannot open", func(t *testing.T) {
		t.Parallel()
		other, err := NewSealer(bytes.Repeat([]byte{0x01}, chacha20poly1305.KeySize))
		require.NoError(t, err)
		_, err = other.Open(sealed, []byte("aad"))
		require.Error(t, err)
	})
}

func TestNewSealerRejectsBadKeyLength(t *testing.T) {
	t.Parallel()
	_, err := NewSealer([]byte("too-short"))
	require.Error(t, err)
}

func TestLoadOrCreateDataKey(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	store := newFakeSecretStore()
	// Deterministic randomness so we can assert persistence.
	randSrc := bytes.NewReader(bytes.Repeat([]byte{0x7e}, chacha20poly1305.KeySize))

	created, err := loadOrCreateDataKey(ctx, store, randSrc)
	require.NoError(t, err)
	assert.Len(t, created, chacha20poly1305.KeySize)

	// Second call returns the same persisted key, not a new one.
	loaded, err := loadOrCreateDataKey(ctx, store, bytes.NewReader(nil))
	require.NoError(t, err)
	assert.Equal(t, created, loaded)
}

func TestLoadOrCreateDataKeyKeyringFailure(t *testing.T) {
	t.Parallel()

	store := newFakeSecretStore()
	store.failGet = errors.New("keyring locked")

	_, err := loadOrCreateDataKey(context.Background(), store, bytes.NewReader(nil))
	require.Error(t, err)
	code, ok := coreerr.CodeOf(err)
	require.True(t, ok)
	assert.Equal(t, coreerr.CodeSecretUnavailable, code)
}
