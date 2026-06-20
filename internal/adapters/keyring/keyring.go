// Package keyring adapts the OS keyring (macOS Keychain / Linux Secret Service)
// to the ports.SecretStore interface (ADR-0009). It holds Conductor's
// per-install data key. rclone remote credentials are never stored here —
// rclone.conf owns those.
package keyring

import (
	"context"
	"errors"
	"fmt"

	gokeyring "github.com/zalando/go-keyring"

	"github.com/conductor-app/conductor/internal/core/coreerr"
	"github.com/conductor-app/conductor/internal/core/ports"
)

// service is the keyring service name under which Conductor's secrets are filed.
const service = "conductor"

// Store is the OS-keyring-backed SecretStore.
type Store struct {
	service string
}

// New constructs a keyring Store using Conductor's service name.
func New() *Store {
	return &Store{service: service}
}

// Get returns the stored value for key, mapping an absent entry to
// ports.ErrSecretNotFound so callers can distinguish first-run from failure.
func (s *Store) Get(_ context.Context, key string) (string, error) {
	v, err := gokeyring.Get(s.service, key)
	if errors.Is(err, gokeyring.ErrNotFound) {
		return "", ports.ErrSecretNotFound
	}
	if err != nil {
		return "", coreerr.New(coreerr.CodeSecretUnavailable, "reading from keyring", err)
	}
	return v, nil
}

// Set stores value under key.
func (s *Store) Set(_ context.Context, key, value string) error {
	if err := gokeyring.Set(s.service, key, value); err != nil {
		return coreerr.New(coreerr.CodeSecretUnavailable, "writing to keyring", err)
	}
	return nil
}

// Delete removes key. Deleting an absent key is not an error.
func (s *Store) Delete(_ context.Context, key string) error {
	err := gokeyring.Delete(s.service, key)
	if err != nil && !errors.Is(err, gokeyring.ErrNotFound) {
		return fmt.Errorf("deleting from keyring: %w", err)
	}
	return nil
}
