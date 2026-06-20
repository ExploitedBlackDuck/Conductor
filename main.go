// Command conductor is the desktop control panel for rclone. This file is the
// composition root (ADR-0014, §5): it resolves paths and configuration,
// constructs and wires dependencies, and starts the desktop shell. It contains
// no business logic — that is a defect here by definition. It does not import
// Wails; the shell does (ADR-0001).
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/conductor-app/conductor/app"
	"github.com/conductor-app/conductor/frontend"
	"github.com/conductor-app/conductor/internal/adapters/procrunner"
	"github.com/conductor-app/conductor/internal/adapters/rcclient"
	"github.com/conductor-app/conductor/internal/adapters/sqlitestore"
	"github.com/conductor-app/conductor/internal/buildinfo"
	"github.com/conductor-app/conductor/internal/core/control"
	"github.com/conductor-app/conductor/internal/core/daemon"
	"github.com/conductor-app/conductor/internal/platform/config"
	"github.com/conductor-app/conductor/internal/platform/logging"
	"github.com/conductor-app/conductor/internal/platform/paths"
	"github.com/conductor-app/conductor/shell"
)

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
		return rcclient.New(addr, user, pass)
	}, logger)

	application := app.New(logger, buildinfo.Version(), ctrl)

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
