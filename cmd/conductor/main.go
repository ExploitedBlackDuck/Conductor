// Command conductor is the desktop control panel for rclone. This file is the
// composition root (§5): it resolves paths and configuration, constructs and
// wires dependencies, and starts the desktop shell. It contains no business
// logic — that is a defect here by definition.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/conductor-app/conductor/app"
	"github.com/conductor-app/conductor/frontend"
	"github.com/conductor-app/conductor/internal/buildinfo"
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

	application := app.New(logger, buildinfo.Version())

	return shell.Run(shell.Config{
		Window: shell.Window{
			Title:     "Conductor",
			Width:     cfg.Window.Width,
			Height:    cfg.Window.Height,
			MinWidth:  940,
			MinHeight: 600,
		},
		Assets:  frontend.Dist(),
		OnReady: application.OnReady,
		Bind:    []any{application},
	})
}
