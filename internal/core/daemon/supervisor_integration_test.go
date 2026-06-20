//go:build integration

// This is the P2 acceptance gate: it starts a real `rclone rcd` using the
// pinned, checksum-verified binary, lists remotes over the rc API, stops the
// daemon, and asserts no orphaned process remains. It is build-tagged and skips
// cleanly when the pinned rclone is not installed, so `go test ./...` stays
// green on a bare machine (§2.5, §7.10).
package daemon_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/conductor-app/conductor/internal/adapters/procrunner"
	"github.com/conductor-app/conductor/internal/adapters/rcclient"
	"github.com/conductor-app/conductor/internal/core/daemon"
	"github.com/conductor-app/conductor/internal/platform/logging"
)

func pinnedBinaryPath(t *testing.T) string {
	t.Helper()
	if env := os.Getenv("CONDUCTOR_RCLONE_BIN"); env != "" {
		return env
	}
	home, err := os.UserHomeDir()
	require.NoError(t, err)
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "conductor", "data", "bin", "rclone")
	default:
		return filepath.Join(home, ".local", "share", "conductor", "bin", "rclone")
	}
}

// processAlive reports whether a process with the given pid exists.
func processAlive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// On Unix, signal 0 probes for existence without affecting the process.
	return proc.Signal(syscall.Signal(0)) == nil
}

func TestRealDaemonLifecycle(t *testing.T) {
	binPath := pinnedBinaryPath(t)
	if _, err := os.Stat(binPath); err != nil {
		t.Skipf("pinned rclone not installed at %s", binPath)
	}

	ctx := context.Background()

	// Use an isolated, empty rclone config with two harmless example remotes so
	// listremotes is deterministic and no real config is touched (§2.10).
	cfgPath := filepath.Join(t.TempDir(), "rclone.conf")
	require.NoError(t, os.WriteFile(cfgPath,
		[]byte("[example-local]\ntype = local\n\n[example-mem]\ntype = memory\n"), 0o600))

	sup, err := daemon.New(daemon.Config{
		BinaryPath:    binPath,
		ConfigPath:    cfgPath,
		Logger:        logging.New(os.Stderr, logging.Options{}),
		Runner:        procrunner.New(),
		StartTimeout:  10 * time.Second,
		ShutdownGrace: 3 * time.Second,
	})
	require.NoError(t, err)

	require.NoError(t, sup.Start(ctx), "daemon should start and become healthy")

	addr, err := sup.Addr()
	require.NoError(t, err)
	creds, err := sup.Credentials()
	require.NoError(t, err)

	client := rcclient.New(addr, creds.User, creds.Pass)

	remotes, err := client.ConfigListRemotes(ctx)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"example-local", "example-mem"}, remotes)

	stats, err := client.CoreStats(ctx)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, stats.Bytes, int64(0))

	// Capture the daemon pid so we can prove it is reaped, not orphaned.
	pid, err := client.CorePID(ctx)
	require.NoError(t, err)
	require.True(t, processAlive(pid), "daemon process should be alive while running")

	require.NoError(t, sup.Stop(ctx), "daemon should stop gracefully")

	// No orphaned process: accessors report stopped, the pid is reaped, and the
	// rc port no longer answers.
	_, err = sup.Addr()
	assert.ErrorIs(t, err, daemon.ErrNotRunning)

	require.Eventually(t, func() bool { return !processAlive(pid) }, 3*time.Second, 50*time.Millisecond,
		"stopped daemon must leave no orphaned process")

	deadClient := rcclient.New(addr, creds.User, creds.Pass)
	assert.Error(t, deadClient.Ping(ctx), "stopped daemon must not answer")
}
