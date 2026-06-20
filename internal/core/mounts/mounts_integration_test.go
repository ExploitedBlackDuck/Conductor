//go:build integration

// P6 acceptance gate: a real mount/unmount round-trip through a real rcd, with
// audit entries written. Build-tagged; skips when the pinned rclone is absent
// or when no FUSE backend is available (e.g. macFUSE not installed).
package mounts_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/conductor-app/conductor/internal/adapters/procrunner"
	"github.com/conductor-app/conductor/internal/adapters/rcclient"
	"github.com/conductor-app/conductor/internal/core/daemon"
	"github.com/conductor-app/conductor/internal/core/domain"
	"github.com/conductor-app/conductor/internal/core/mounts"
	"github.com/conductor-app/conductor/internal/platform/logging"
)

// recordingAudit records audit actions for the integration assertions.
type recordingAudit struct{ actions []string }

func (a *recordingAudit) Record(_ context.Context, action domain.AuditAction, _ string, _ any) (domain.AuditEntry, error) {
	a.actions = append(a.actions, string(action))
	return domain.AuditEntry{}, nil
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

func TestRealMountUnmount(t *testing.T) {
	bin := pinnedBinaryPath(t)
	if _, err := os.Stat(bin); err != nil {
		t.Skipf("pinned rclone not installed at %s", bin)
	}
	ctx := context.Background()

	dir := t.TempDir()
	cfg := filepath.Join(dir, "rclone.conf")
	require.NoError(t, os.WriteFile(cfg, []byte("[mem]\ntype = memory\n"), 0o600))
	mp := filepath.Join(dir, "mnt")
	require.NoError(t, os.MkdirAll(mp, 0o755))

	sup, err := daemon.New(daemon.Config{
		BinaryPath: bin, ConfigPath: cfg,
		Logger: logging.New(os.Stderr, logging.Options{}), Runner: procrunner.New(),
		StartTimeout: 10 * time.Second,
	})
	require.NoError(t, err)
	require.NoError(t, sup.Start(ctx))
	t.Cleanup(func() { _ = sup.Stop(ctx) })

	addr, err := sup.Addr()
	require.NoError(t, err)
	creds, err := sup.Credentials()
	require.NoError(t, err)
	client := rcclient.New(addr, creds.User, creds.Pass)

	audit := &recordingAudit{}
	svc := mounts.New(func() (mounts.RC, error) { return client, nil }, audit)

	if err := svc.Mount(ctx, "mem:bucket", mp, ""); err != nil {
		if strings.Contains(err.Error(), "fuse") || strings.Contains(err.Error(), "mount helper") || strings.Contains(err.Error(), "FUSE") {
			t.Skipf("no FUSE backend available: %v", err)
		}
		require.NoError(t, err)
	}
	t.Cleanup(func() { _ = svc.Unmount(ctx, mp) })

	require.Eventually(t, func() bool {
		ms, err := svc.List(ctx)
		return err == nil && len(ms) == 1
	}, 5*time.Second, 100*time.Millisecond)

	require.NoError(t, svc.Unmount(ctx, mp))
	require.Eventually(t, func() bool {
		ms, err := svc.List(ctx)
		return err == nil && len(ms) == 0
	}, 5*time.Second, 100*time.Millisecond)

	assert.Equal(t, []string{"mount.mount", "mount.unmount"}, audit.actions)
}
