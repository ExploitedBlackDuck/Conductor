// Package ports holds the small, cross-cutting interfaces the core depends on
// but does not implement: the clock and the OS secret store. Each is consumed
// by more than one core package, which is why they live here rather than next
// to a single consumer (§2.1, §5). Adapters under internal/adapters provide the
// real implementations; tests provide fakes.
package ports

import (
	"context"
	"errors"
	"time"
)

// Clock abstracts the current time so time-dependent logic (audit timestamps,
// backoff) is deterministic under test.
type Clock interface {
	// Now returns the current instant.
	Now() time.Time
}

// SystemClock is the production Clock backed by the wall clock.
type SystemClock struct{}

// Now returns time.Now().
func (SystemClock) Now() time.Time { return time.Now() }

// ErrSecretNotFound is returned by SecretStore.Get when no value is stored for
// the key. Callers branch on it with errors.Is to distinguish "absent" (first
// run) from a backend failure.
var ErrSecretNotFound = errors.New("secret not found")

// SecretStore is the OS keyring abstraction (ADR-0009). It holds the per-install
// data key; rclone remote credentials are never placed here — rclone.conf owns
// those.
type SecretStore interface {
	// Get returns the stored value for key, or ErrSecretNotFound if absent.
	Get(ctx context.Context, key string) (string, error)
	// Set stores value under key, overwriting any existing value.
	Set(ctx context.Context, key, value string) error
	// Delete removes key; deleting an absent key is not an error.
	Delete(ctx context.Context, key string) error
}
