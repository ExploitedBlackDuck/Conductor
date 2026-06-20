// Package shell isolates the Wails runtime behind Conductor's own interface so
// that the eventual migration to Wails v3 is contained to this package and the
// app/ binding layer (ADR-0001). No package under internal/core may import
// Wails; this is the single seam where the desktop shell is started and where
// the live application context is adapted into a Runtime callers can use to
// reach the frontend.
package shell

import (
	"context"
	"fmt"
	"io/fs"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// Window describes the desktop window Conductor opens (§7.11.1). MinWidth and
// MinHeight enforce the documented minimum below which secondary panes collapse
// rather than crushing the progress table.
type Window struct {
	Title     string
	Width     int
	Height    int
	MinWidth  int
	MinHeight int
}

// Runtime is the subset of desktop-shell capabilities Conductor depends on:
// emitting typed events to the frontend (§7.2 live stats stream). Abstracting
// it keeps callers — notably the app/ binding layer — free of any direct Wails
// dependency beyond this package.
type Runtime interface {
	// EmitEvent publishes a named event with an optional payload to the
	// frontend. Event names are typed constants defined in the app layer.
	EmitEvent(name string, data ...any)
}

// Config configures the desktop shell.
type Config struct {
	Window Window
	// Assets is the embedded production frontend build, rooted at "dist".
	Assets fs.FS
	// OnReady is invoked once the webview is initialised, with a Runtime bound
	// to the live application context. It is the application's startup hook.
	OnReady func(ctx context.Context, rt Runtime)
	// OnShutdown is invoked as the window closes, before the process exits, so
	// the application can stop the daemon and close the store cleanly.
	OnShutdown func(ctx context.Context)
	// Bind lists the structs whose exported methods are exposed to the frontend
	// as generated typed bindings (§2.8).
	Bind []any
}

// Run opens the desktop window and blocks until it closes. It is the only
// function in the codebase that starts the Wails runtime; everything else
// reaches the shell through Runtime.
func Run(cfg Config) error {
	assets, err := fs.Sub(cfg.Assets, "dist")
	if err != nil {
		return fmt.Errorf("locating embedded frontend assets: %w", err)
	}

	runErr := wails.Run(&options.App{
		Title:     cfg.Window.Title,
		Width:     cfg.Window.Width,
		Height:    cfg.Window.Height,
		MinWidth:  cfg.Window.MinWidth,
		MinHeight: cfg.Window.MinHeight,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup: func(ctx context.Context) {
			if cfg.OnReady != nil {
				cfg.OnReady(ctx, &wailsRuntime{ctx: ctx})
			}
		},
		OnShutdown: func(ctx context.Context) {
			if cfg.OnShutdown != nil {
				cfg.OnShutdown(ctx)
			}
		},
		Bind: cfg.Bind,
	})
	if runErr != nil {
		return fmt.Errorf("running desktop shell: %w", runErr)
	}
	return nil
}

// wailsRuntime adapts the Wails runtime package to the Runtime interface,
// binding emitted events to the live application context.
type wailsRuntime struct {
	ctx context.Context
}

func (r *wailsRuntime) EmitEvent(name string, data ...any) {
	wailsruntime.EventsEmit(r.ctx, name, data...)
}
