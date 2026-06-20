//go:build integration

// P4 acceptance demo, automated: start a real rcd, run a real copy through it,
// and assert the live-stats poll loop reflects the transfer to the emitter.
// Build-tagged; skips when the pinned rclone is absent.
package control_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/conductor-app/conductor/internal/adapters/procrunner"
	"github.com/conductor-app/conductor/internal/adapters/rcclient"
	"github.com/conductor-app/conductor/internal/core/control"
	"github.com/conductor-app/conductor/internal/core/daemon"
	"github.com/conductor-app/conductor/internal/core/livestats"
	"github.com/conductor-app/conductor/internal/platform/logging"
)

type rcAdapter struct{ *rcclient.Client }

func (a rcAdapter) RunningJobIDs(ctx context.Context) ([]int64, error) {
	jobs, err := a.Client.JobList(ctx)
	if err != nil {
		return nil, err
	}
	return jobs.RunningIDs, nil
}

type recordingEmitter struct {
	mu    sync.Mutex
	last  livestats.Snapshot
	count int
}

func (e *recordingEmitter) EmitStats(s livestats.Snapshot) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.last = s
	e.count++
}

func (e *recordingEmitter) maxBytes() int64 {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.last.Stats.Bytes
}

func pinnedBinaryPath(t *testing.T) string {
	t.Helper()
	if env := os.Getenv("CONDUCTOR_RCLONE_BIN"); env != "" {
		return env
	}
	home, err := os.UserHomeDir()
	require.NoError(t, err)
	if runtime.GOOS == "darwin" {
		return filepath.Join(home, "Library", "Application Support", "conductor", "data", "bin", "rclone")
	}
	return filepath.Join(home, ".local", "share", "conductor", "bin", "rclone")
}

func TestLiveStatsReflectARealCopy(t *testing.T) {
	bin := pinnedBinaryPath(t)
	if _, err := os.Stat(bin); err != nil {
		t.Skipf("pinned rclone not installed at %s", bin)
	}
	ctx := context.Background()

	// Isolated config with a memory remote; a local source dir with a payload.
	dir := t.TempDir()
	cfg := filepath.Join(dir, "rclone.conf")
	require.NoError(t, os.WriteFile(cfg, []byte("[mem]\ntype = memory\n"), 0o600))
	srcDir := filepath.Join(dir, "src")
	require.NoError(t, os.MkdirAll(srcDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "payload.bin"), make([]byte, 32<<20), 0o600))

	sup, err := daemon.New(daemon.Config{
		BinaryPath: bin, ConfigPath: cfg,
		Logger: logging.New(os.Stderr, logging.Options{}), Runner: procrunner.New(),
		StartTimeout: 10 * time.Second,
	})
	require.NoError(t, err)

	svc := control.New(sup, func(addr, user, pass string) control.RC {
		return rcAdapter{rcclient.New(addr, user, pass)}
	}, logging.New(os.Stderr, logging.Options{}))

	emitter := &recordingEmitter{}
	require.NoError(t, svc.Start(ctx, emitter))
	t.Cleanup(func() { _ = svc.Stop(ctx) })

	// Drive a real copy through the same daemon.
	addr, err := sup.Addr()
	require.NoError(t, err)
	creds, err := sup.Credentials()
	require.NoError(t, err)
	driver := rcclient.New(addr, creds.User, creds.Pass)
	_, err = driver.SyncCopy(ctx, srcDir, "mem:bucket", nil, nil, true)
	require.NoError(t, err)

	// The live poll loop must reflect the bytes moved.
	require.Eventually(t, func() bool { return emitter.maxBytes() > 0 }, 8*time.Second, 200*time.Millisecond,
		"live stats should reflect the running copy")
}
