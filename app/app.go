// Package app is the Wails binding layer (§3): together with shell/ it is one
// of only two packages permitted to depend on the desktop shell. It is
// deliberately thin — frontend calls are translated into core service calls and
// core errors are mapped to typed DTOs (§2.2). No business logic lives here.
package app

import (
	"context"
	"log/slog"

	"github.com/conductor-app/conductor/internal/core/control"
	"github.com/conductor-app/conductor/internal/core/mounts"
	"github.com/conductor-app/conductor/internal/core/options"
	"github.com/conductor-app/conductor/internal/core/transfers"
	"github.com/conductor-app/conductor/shell"
)

// App is the root object bound to the frontend; its exported methods become the
// generated typed bindings the frontend calls (§2.8). Dependencies are injected
// from the composition root (§5). Business logic lives in core services; App
// only adapts calls and maps errors.
type App struct {
	log       *slog.Logger
	version   string
	control   *control.Service
	catalog   *options.Catalog
	transfers *transfers.Service
	mounts    *mounts.Service
	stats     *statsEmitter
}

// New constructs the binding-layer App with its dependencies injected.
func New(log *slog.Logger, version string, ctrl *control.Service, catalog *options.Catalog, tr *transfers.Service, mt *mounts.Service) *App {
	return &App{log: log, version: version, control: ctrl, catalog: catalog, transfers: tr, mounts: mt}
}

// OnReady is the shell's startup hook (shell.Config.OnReady). It wires the live
// event emitter to the runtime and starts the supervised daemon (and its
// poll loop) in the background so the window paints immediately; a startup
// failure surfaces as a degraded Status rather than blocking the UI (§7.11.9).
func (a *App) OnReady(ctx context.Context, rt shell.Runtime) {
	a.log.InfoContext(ctx, "frontend ready", "version", a.version)
	a.stats = &statsEmitter{rt: rt}
	go func() {
		// control.Start records its own error and owns the supervision and poll
		// goroutines. This goroutine exits when Start returns.
		_ = a.control.Start(ctx, a.stats)
	}()
}

// StatsSnapshot returns the most recent live stats snapshot. The frontend uses
// it for an initial value; subsequent updates arrive via EventStatsUpdate.
func (a *App) StatsSnapshot() StatsEventDTO {
	if a.stats == nil {
		return StatsEventDTO{}
	}
	return a.stats.snapshot()
}

// OnShutdown is the shell's shutdown hook. It stops the daemon cleanly so no
// rcd is orphaned when the window closes (§2.3).
func (a *App) OnShutdown(ctx context.Context) {
	a.log.InfoContext(ctx, "shutting down")
	// Finalize in-flight operations before the daemon goes away, then stop the
	// daemon's supervision and poll loop.
	a.transfers.Close()
	if err := a.control.Stop(ctx); err != nil {
		a.log.ErrorContext(ctx, "error stopping daemon", "error", err)
	}
}

// Version returns Conductor's build version.
func (a *App) Version() string {
	return a.version
}

// StatusDTO is the read-only status the frontend renders (§7.11). It is a
// JSON-tagged transport shape, not a domain type.
type StatusDTO struct {
	DaemonRunning bool      `json:"daemonRunning"`
	Remotes       []string  `json:"remotes"`
	Bytes         int64     `json:"bytes"`
	Speed         float64   `json:"speed"`
	Transfers     int64     `json:"transfers"`
	Errors        int64     `json:"errorsCount"`
	Error         *ErrorDTO `json:"error"`
}

// Status returns the current daemon and transfer status. It is the P2 read-only
// binding the frontend polls; live event streaming arrives in P4.
func (a *App) Status() StatusDTO {
	st := a.control.Status(context.Background())
	dto := StatusDTO{
		DaemonRunning: st.DaemonRunning,
		Remotes:       st.Remotes,
		Bytes:         st.Stats.Bytes,
		Speed:         st.Stats.Speed,
		Transfers:     st.Stats.Transfers,
		Errors:        st.Stats.Errors,
		Error:         errorToDTO(st.Err),
	}
	if dto.Remotes == nil {
		dto.Remotes = []string{}
	}
	return dto
}
