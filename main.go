// Command conductor is the desktop control panel for rclone. This file is the
// composition root (ADR-0014, §5): it resolves paths and configuration,
// constructs and wires dependencies, and starts the desktop shell. It contains
// no business logic — that is a defect here by definition. It does not import
// Wails; the shell does (ADR-0001).
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/conductor-app/conductor/app"
	"github.com/conductor-app/conductor/frontend"
	"github.com/conductor-app/conductor/internal/adapters/keyring"
	"github.com/conductor-app/conductor/internal/adapters/procrunner"
	"github.com/conductor-app/conductor/internal/adapters/rcclient"
	"github.com/conductor-app/conductor/internal/adapters/sqlitestore"
	"github.com/conductor-app/conductor/internal/buildinfo"
	"github.com/conductor-app/conductor/internal/core/audit"
	"github.com/conductor-app/conductor/internal/core/control"
	"github.com/conductor-app/conductor/internal/core/coreerr"
	"github.com/conductor-app/conductor/internal/core/daemon"
	"github.com/conductor-app/conductor/internal/core/domain"
	"github.com/conductor-app/conductor/internal/core/history"
	"github.com/conductor-app/conductor/internal/core/mounts"
	"github.com/conductor-app/conductor/internal/core/options"
	"github.com/conductor-app/conductor/internal/core/pairs"
	"github.com/conductor-app/conductor/internal/core/ports"
	"github.com/conductor-app/conductor/internal/core/preview"
	"github.com/conductor-app/conductor/internal/core/rclonebin"
	"github.com/conductor-app/conductor/internal/core/secrets"
	"github.com/conductor-app/conductor/internal/core/transfers"
	"github.com/conductor-app/conductor/internal/platform/config"
	"github.com/conductor-app/conductor/internal/platform/logging"
	"github.com/conductor-app/conductor/internal/platform/paths"
	"github.com/conductor-app/conductor/internal/platform/singleinstance"
	"github.com/conductor-app/conductor/shell"
)

// governanceDefaults are Conductor's conservative global bandwidth/concurrency
// ceilings (ADR-0013): safe by default, with going faster an explicit,
// per-remote choice. Per-remote ceilings only tighten these further (§7.6).
var governanceDefaults = options.Ceilings{Transfers: 4, Checkers: 8}

// rcAdapter wraps the rc client to satisfy control.RC, projecting the daemon's
// job list down to the running job ids the live-stats poller needs. Keeping this
// projection in the composition root keeps the rc adapter and the core service
// free of each other's types.
type rcAdapter struct {
	*rcclient.Client
}

// RunningJobIDs returns the currently-running rc job ids.
func (a rcAdapter) RunningJobIDs(ctx context.Context) ([]int64, error) {
	jobs, err := a.JobList(ctx)
	if err != nil {
		return nil, err
	}
	return jobs.RunningIDs, nil
}

func main() {
	if err := run(); err != nil {
		// main is the only place a fatal startup error terminates the process;
		// library code returns errors rather than calling os.Exit (§2.2).
		fmt.Fprintf(os.Stderr, "conductor: fatal: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	ctx := context.Background()

	p, err := paths.Resolve()
	if err != nil {
		return fmt.Errorf("resolving application paths: %w", err)
	}
	if err := paths.EnsureDirs(p); err != nil {
		return fmt.Errorf("creating application directories: %w", err)
	}

	cfg, err := config.Load(filepath.Join(p.ConfigDir, "config.toml"))
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}

	logger := logging.New(os.Stderr, logging.Options{
		Level: cfg.Log.SlogLevel(),
		JSON:  cfg.Log.JSON,
	})
	logger.Info(
		"starting conductor",
		"version", buildinfo.Version(),
		"config_dir", p.ConfigDir,
		"data_dir", p.DataDir,
	)

	// Single-instance lock (§2.3): exactly one process may own the data dir, so
	// the single-writer store and the single-appender audit chain are never
	// contended. Acquire it before opening the database it guards.
	lock, err := singleinstance.Acquire(filepath.Join(p.DataDir, "conductor.lock"))
	if err != nil {
		if errors.Is(err, singleinstance.ErrAlreadyRunning) {
			return coreerr.New(coreerr.CodeSingleInstanceLock,
				"another Conductor instance is already running for this data directory", err)
		}
		return fmt.Errorf("acquiring single-instance lock: %w", err)
	}
	defer func() { _ = lock.Release() }()

	// Persistence foundation: open the store, running forward-only migrations
	// behind the schema-version check (ADR-0007).
	store, err := sqlitestore.Open(ctx, filepath.Join(p.DataDir, "conductor.db"))
	if err != nil {
		return fmt.Errorf("opening store: %w", err)
	}
	defer func() { _ = store.Close() }()
	if version, verr := store.SchemaVersion(ctx); verr == nil {
		logger.Info("store ready", "schema_version", version)
	}

	// Daemon supervision + the read-only control service.
	binaryPath := cfg.Rclone.BinaryPath
	if binaryPath == "" {
		binaryPath = filepath.Join(p.DataDir, "bin", "rclone")
	}
	supervisor, err := daemon.New(daemon.Config{
		BinaryPath: binaryPath,
		ConfigPath: cfg.Rclone.ConfigPath,
		Logger:     logger,
		Runner:     procrunner.New(),
	})
	if err != nil {
		return fmt.Errorf("configuring daemon supervisor: %w", err)
	}
	ctrl := control.New(supervisor, func(addr, user, pass string) control.RC {
		return rcAdapter{rcclient.New(addr, user, pass)}
	}, logger)

	// Load the embedded option catalog for the pinned rclone (ADR-0011).
	catalog, err := options.Load()
	if err != nil {
		return fmt.Errorf("loading option catalog: %w", err)
	}

	// Secrets foundation (ADR-0009): the per-install data key from the OS
	// keyring drives the AEAD that seals captured logs at rest.
	secretStore := keyring.New()
	dataKey, err := secrets.LoadOrCreateDataKey(ctx, secretStore)
	if err != nil {
		return fmt.Errorf("loading data key: %w", err)
	}
	sealer, err := secrets.NewSealer(dataKey)
	if err != nil {
		return fmt.Errorf("initialising sealer: %w", err)
	}

	auditSvc := audit.New(store, ports.SystemClock{})

	// Reconcile state orphaned by an unclean exit (§2.3): rclone jobs die with
	// the daemon, so any operation still marked running on this launch is closed
	// as interrupted and the reconciliation is audited. A clean restart is a
	// no-op. This is exactly the re-acquire-after-crash path the lock above
	// distinguishes from a live second instance.
	if n, rerr := store.ReconcileRunningOperations(ctx, time.Now().UTC()); rerr != nil {
		logger.Error("reconciling orphaned operations failed", "error", rerr)
	} else if n > 0 {
		logger.Warn("closed orphaned operations from an unclean exit", "count", n)
		if _, aerr := auditSvc.Record(ctx, domain.ActionOperationInterrupted, "startup",
			map[string]any{"reconciled": n}); aerr != nil {
			logger.Error("auditing reconciliation failed", "error", aerr)
		}
	}

	// The destructive-op preview gate (ADR-0015) runs the pinned rclone with
	// --dry-run as a sanctioned one-shot subprocess and parses the change set.
	previewSvc := preview.New(preview.Config{
		BinaryPath: binaryPath,
		ConfigPath: cfg.Rclone.ConfigPath,
		Runner:     procrunner.New(),
	})

	// Transfers run through the rc daemon; the provider builds an rc client
	// bound to the live session when a run starts.
	transferRC := func() (transfers.RC, error) {
		addr, err := supervisor.Addr()
		if err != nil {
			return nil, err
		}
		creds, err := supervisor.Credentials()
		if err != nil {
			return nil, err
		}
		return rcclient.New(addr, creds.User, creds.Pass), nil
	}
	transferSvc := transfers.New(transfers.Config{
		RC:        transferRC,
		Store:     store,
		Audit:     auditSvc,
		Sealer:    sealer,
		Previewer: previewSvc,
		Catalog:   catalog,
		Version:   rclonebin.PinnedVersion,
		Logger:    logger,
	})

	mountSvc := mounts.New(func() (mounts.RC, error) {
		addr, err := supervisor.Addr()
		if err != nil {
			return nil, err
		}
		creds, err := supervisor.Credentials()
		if err != nil {
			return nil, err
		}
		return rcclient.New(addr, creds.User, creds.Pass), nil
	}, auditSvc)

	// Saved pairs & profiles run through the transfers service, with governance
	// ceilings resolved from conservative global defaults plus per-remote caps
	// (ADR-0013, §7.6). A never-run pair defaults to dry-run (§7.4).
	pairsSvc := pairs.New(pairs.Config{
		Store:    store,
		Runner:   transferSvc,
		Catalog:  catalog,
		Audit:    auditSvc,
		Defaults: governanceDefaults,
		Clock:    ports.SystemClock{},
	})

	// Operation history browsing, queries, and the audited "what was moved"
	// export over the persisted operations and audit chain (§7.7, §7.11.7).
	historySvc := history.New(history.Config{Store: store, Audit: auditSvc})

	application := app.New(logger, buildinfo.Version(), ctrl, catalog, transferSvc, mountSvc, pairsSvc, historySvc)

	return shell.Run(shell.Config{
		Window: shell.Window{
			Title:     "Conductor",
			Width:     cfg.Window.Width,
			Height:    cfg.Window.Height,
			MinWidth:  940,
			MinHeight: 600,
		},
		Assets:     frontend.Dist(),
		OnReady:    application.OnReady,
		OnShutdown: application.OnShutdown,
		Bind:       []any{application},
	})
}
