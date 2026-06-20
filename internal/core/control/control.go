// Package control is the read-only application service the binding layer calls
// for daemon lifecycle and live status (P2). It orchestrates the supervised
// daemon and the rc client behind small, consumer-defined ports so the binding
// layer stays thin (§3) and this logic is testable without real processes.
package control

import (
	"context"
	"log/slog"
	"sync"

	"github.com/conductor-app/conductor/internal/core/daemon"
	"github.com/conductor-app/conductor/internal/core/domain"
)

// Daemon is the subset of the daemon supervisor this service needs.
type Daemon interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Addr() (string, error)
	Credentials() (daemon.Credentials, error)
}

// RC is the subset of the rc client this service needs (read-only in P2).
type RC interface {
	Ping(ctx context.Context) error
	ConfigListRemotes(ctx context.Context) ([]string, error)
	CoreStats(ctx context.Context) (domain.TransferStats, error)
}

// RCFactory builds an RC client for a daemon's live address and credentials.
// Credentials are passed per call because they change each session.
type RCFactory func(addr, user, pass string) RC

// Status is the read-only snapshot the UI renders (§7.11): whether the daemon is
// up, the configured remotes, and aggregate live stats. Err carries any failure
// for boundary mapping.
type Status struct {
	DaemonRunning bool
	Remotes       []string
	Stats         domain.TransferStats
	Err           error
}

// Service supervises the daemon and answers status queries.
type Service struct {
	daemon Daemon
	newRC  RCFactory
	log    *slog.Logger

	mu       sync.RWMutex
	running  bool
	startErr error
}

// New constructs the control Service.
func New(d Daemon, newRC RCFactory, log *slog.Logger) *Service {
	return &Service{daemon: d, newRC: newRC, log: log}
}

// Start brings up the supervised daemon, recording any failure so Status can
// report a degraded state rather than crashing (§7.11.9).
func (s *Service) Start(ctx context.Context) error {
	err := s.daemon.Start(ctx)
	s.mu.Lock()
	s.running = err == nil
	s.startErr = err
	s.mu.Unlock()
	if err != nil {
		s.log.Error("daemon failed to start", "error", err)
	}
	return err
}

// Stop shuts the daemon down.
func (s *Service) Stop(ctx context.Context) error {
	s.mu.Lock()
	s.running = false
	s.mu.Unlock()
	return s.daemon.Stop(ctx)
}

// Status returns the current snapshot. When the daemon never started, it carries
// the start error; when the daemon is up but the rc call fails, it reports
// running with the rc error.
func (s *Service) Status(ctx context.Context) Status {
	s.mu.RLock()
	running := s.running
	startErr := s.startErr
	s.mu.RUnlock()

	if !running {
		return Status{DaemonRunning: false, Err: startErr}
	}

	rc, err := s.client()
	if err != nil {
		return Status{DaemonRunning: true, Err: err}
	}

	remotes, err := rc.ConfigListRemotes(ctx)
	if err != nil {
		return Status{DaemonRunning: true, Err: err}
	}
	stats, err := rc.CoreStats(ctx)
	if err != nil {
		return Status{DaemonRunning: true, Remotes: remotes, Err: err}
	}
	return Status{DaemonRunning: true, Remotes: remotes, Stats: stats}
}

// client builds an RC client bound to the daemon's current address and
// credentials.
func (s *Service) client() (RC, error) {
	addr, err := s.daemon.Addr()
	if err != nil {
		return nil, err
	}
	creds, err := s.daemon.Credentials()
	if err != nil {
		return nil, err
	}
	return s.newRC(addr, creds.User, creds.Pass), nil
}
