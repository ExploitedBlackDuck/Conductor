//go:build integration

// P5 acceptance gate, end-to-end with real rclone: run a copy through a real
// rcd, cancel it, and assert the job count returns to baseline and a complete
// operation row + sealed captured log + audit entries are written and
// queryable through the real SQLite store. Build-tagged; skips when the pinned
// rclone is absent.
package transfers_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/chacha20poly1305"

	"github.com/conductor-app/conductor/internal/adapters/procrunner"
	"github.com/conductor-app/conductor/internal/adapters/rcclient"
	"github.com/conductor-app/conductor/internal/adapters/sqlitestore"
	"github.com/conductor-app/conductor/internal/core/audit"
	"github.com/conductor-app/conductor/internal/core/daemon"
	"github.com/conductor-app/conductor/internal/core/domain"
	"github.com/conductor-app/conductor/internal/core/options"
	"github.com/conductor-app/conductor/internal/core/ports"
	"github.com/conductor-app/conductor/internal/core/secrets"
	"github.com/conductor-app/conductor/internal/core/transfers"
	"github.com/conductor-app/conductor/internal/platform/logging"
)

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

func TestRealCopyCaptureAndCancel(t *testing.T) {
	bin := pinnedBinaryPath(t)
	if _, err := os.Stat(bin); err != nil {
		t.Skipf("pinned rclone not installed at %s", bin)
	}
	ctx := context.Background()

	dir := t.TempDir()
	cfg := filepath.Join(dir, "rclone.conf")
	require.NoError(t, os.WriteFile(cfg, []byte("[mem]\ntype = memory\n"), 0o600))
	srcDir := filepath.Join(dir, "src")
	require.NoError(t, os.MkdirAll(srcDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "payload.bin"), make([]byte, 96<<20), 0o600))

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
	newRC := func() (transfers.RC, error) { return rcclient.New(addr, creds.User, creds.Pass), nil }

	store, err := sqlitestore.Open(ctx, filepath.Join(dir, "conductor.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	auditSvc := audit.New(store, ports.SystemClock{})
	sealer, err := secrets.NewSealer(bytes.Repeat([]byte{0x55}, chacha20poly1305.KeySize))
	require.NoError(t, err)
	cat, err := options.Load()
	require.NoError(t, err)

	svc := transfers.New(transfers.Config{
		RC: newRC, Store: store, Audit: auditSvc, Sealer: sealer, Catalog: cat,
		Version: "v1.74.3", Logger: logging.New(os.Stderr, logging.Options{}), PollEvery: 100 * time.Millisecond,
	})
	t.Cleanup(svc.Close)

	h, err := svc.Start(ctx, transfers.RunRequest{
		Kind:      domain.KindCopy,
		Src:       domain.Endpoint{Path: srcDir},
		Dst:       domain.Endpoint{Remote: "mem", Path: "bucket"},
		Selection: options.Selection{Single: map[string]string{"--transfers": "1"}},
	})
	require.NoError(t, err)

	// Cancel promptly; whether it catches the job mid-flight or just after, the
	// outcome must be a clean, recorded terminal state with no running jobs.
	_ = svc.Cancel(ctx, h.OperationID)

	driver := rcclient.New(addr, creds.User, creds.Pass)
	require.Eventually(t, func() bool {
		st, err := driver.JobStatus(ctx, h.JobID)
		return err == nil && st.Finished
	}, 10*time.Second, 100*time.Millisecond, "the job must reach a terminal state (no work left running)")

	// The operation, its sealed log, and audit entries are written and queryable.
	require.Eventually(t, func() bool {
		_, _, found, err := store.OperationByID(ctx, h.OperationID)
		return err == nil && found
	}, 5*time.Second, 50*time.Millisecond)

	op, opts, found, err := store.OperationByID(ctx, h.OperationID)
	require.NoError(t, err)
	require.True(t, found)
	assert.Contains(t, []domain.Result{domain.ResultCancelled, domain.ResultSuccess}, op.Result)
	assert.NotEmpty(t, opts)
	assert.NotEmpty(t, op.LogRef)

	gotLog, ok, err := store.OperationLog(ctx, h.OperationID)
	require.NoError(t, err)
	require.True(t, ok)
	opened, err := sealer.Open(secrets.Sealed{Nonce: gotLog.Nonce, Ciphertext: gotLog.SealedBytes}, []byte(h.OperationID))
	require.NoError(t, err)
	assert.Contains(t, string(opened), "jobStatus")

	res, err := auditSvc.Verify(ctx)
	require.NoError(t, err)
	assert.True(t, res.Intact)
	assert.GreaterOrEqual(t, res.Count, 2, "at least operation start and stop are recorded")
}
