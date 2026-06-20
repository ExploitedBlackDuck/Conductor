//go:build integration

// These tests exercise the real OS keyring. They are build-tagged and skip
// cleanly when no keyring backend is available, so `go test ./...` stays green
// on a bare machine (§2.5).
package keyring_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/conductor-app/conductor/internal/adapters/keyring"
	"github.com/conductor-app/conductor/internal/core/ports"
)

func TestKeyringRoundTrip(t *testing.T) {
	ctx := context.Background()
	store := keyring.New()
	const key = "conductor-integration-test-key"

	// Probe: if the backend is unavailable (headless CI without a keyring),
	// skip rather than fail.
	if err := store.Set(ctx, key, "probe"); err != nil {
		t.Skipf("keyring backend unavailable: %v", err)
	}
	t.Cleanup(func() { _ = store.Delete(ctx, key) })

	require.NoError(t, store.Set(ctx, key, "s3cret-value"))

	got, err := store.Get(ctx, key)
	require.NoError(t, err)
	assert.Equal(t, "s3cret-value", got)

	require.NoError(t, store.Delete(ctx, key))

	_, err = store.Get(ctx, key)
	assert.True(t, errors.Is(err, ports.ErrSecretNotFound), "deleted key should be reported absent")
}
