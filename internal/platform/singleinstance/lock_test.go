//go:build unix

package singleinstance_test

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/conductor-app/conductor/internal/platform/singleinstance"
)

func TestSecondAcquireIsRefused(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "conductor.lock")

	first, err := singleinstance.Acquire(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = first.Release() })

	// A second acquire on the same lockfile must be refused while the first holds.
	_, err = singleinstance.Acquire(path)
	require.Error(t, err)
	assert.True(t, errors.Is(err, singleinstance.ErrAlreadyRunning))
}

func TestReleaseAllowsReacquire(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "conductor.lock")

	first, err := singleinstance.Acquire(path)
	require.NoError(t, err)
	require.NoError(t, first.Release())

	// After release the lock is free to take again (the unclean-exit case the
	// OS handles for us, exercised here explicitly).
	second, err := singleinstance.Acquire(path)
	require.NoError(t, err)
	require.NoError(t, second.Release())

	// Release is idempotent.
	require.NoError(t, second.Release())
}
