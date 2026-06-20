// Package app is the Wails binding layer (§3): together with shell/ it is one
// of only two packages permitted to depend on the desktop shell. It is
// deliberately thin — frontend calls are translated into core service calls and
// core errors are mapped to typed DTOs (§2.2). No business logic lives here.
package app

import (
	"context"
	"log/slog"

	"github.com/conductor-app/conductor/shell"
)

// App is the root object bound to the frontend; its exported methods become
// the generated typed bindings the frontend calls (§2.8). Dependencies are
// injected from the composition root (§5). In P0 it carries lifecycle wiring
// and a single real binding (Version); service methods arrive in later phases.
type App struct {
	log     *slog.Logger
	version string
	rt      shell.Runtime
}

// New constructs the binding-layer App with its dependencies injected.
func New(log *slog.Logger, version string) *App {
	return &App{log: log, version: version}
}

// OnReady is the shell's startup hook (shell.Config.OnReady). It records the
// Runtime handle used to emit events to the frontend and logs a structured
// startup line (the P0 gate's "logs startup").
func (a *App) OnReady(ctx context.Context, rt shell.Runtime) {
	a.rt = rt
	a.log.InfoContext(ctx, "frontend ready", "version", a.version)
}

// Version returns Conductor's build version. It exists so P0 ships one real,
// end-to-end binding (frontend → generated binding → app) rather than an empty
// surface.
func (a *App) Version() string {
	return a.version
}
