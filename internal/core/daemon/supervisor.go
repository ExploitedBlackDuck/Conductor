package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/conductor-app/conductor/internal/core/coreerr"
	"github.com/conductor-app/conductor/internal/core/ports"
	"github.com/conductor-app/conductor/internal/core/rclonebin"
)

// ErrNotRunning is returned by accessors when the daemon is not running.
var ErrNotRunning = errors.New("daemon not running")

// Config configures a Supervisor. Required fields: BinaryPath, Logger, Runner.
type Config struct {
	// BinaryPath is the absolute path to the pinned, checksum-verified rclone.
	BinaryPath string
	// ConfigPath optionally points rclone at a specific rclone.conf; empty uses
	// rclone's default config location.
	ConfigPath string
	// Logger receives structured operational logs (never secrets, §2.4).
	Logger *slog.Logger
	// Runner spawns processes (procrunner in production, a fake under test).
	Runner Runner
	// Clock is used for restart timing; defaults to the system clock.
	Clock ports.Clock
	// Probe checks daemon health; defaults to an HTTP rc/noop probe.
	Probe HealthProbe
	// StartTimeout bounds how long Start waits for the daemon to become healthy.
	StartTimeout time.Duration
	// HealthInterval is the health-poll interval during startup.
	HealthInterval time.Duration
	// ShutdownGrace is how long graceful shutdown waits after SIGTERM before
	// escalating to SIGKILL.
	ShutdownGrace time.Duration
	// VerifyBinary checks the binary's integrity before launch; defaults to the
	// pinned-checksum verification (ADR-0008). Injectable for tests.
	VerifyBinary func(path string) error
	// RestartBackoffBase and RestartBackoffMax bound the exponential restart
	// backoff; defaults are 200ms and 10s.
	RestartBackoffBase time.Duration
	RestartBackoffMax  time.Duration
}

type state int

const (
	stateStopped state = iota
	stateRunning
)

// Supervisor owns the lifecycle of one `rclone rcd` process.
type Supervisor struct {
	cfg     Config
	backoff backoff

	mu    sync.Mutex
	state state
	addr  string
	creds Credentials

	stopCh   chan struct{}
	stopOnce sync.Once
	done     chan struct{}
	ready    chan error
}

// New constructs a Supervisor, applying defaults for optional fields. It returns
// an error if a required dependency is missing.
func New(cfg Config) (*Supervisor, error) {
	if cfg.BinaryPath == "" {
		return nil, errors.New("daemon: BinaryPath is required")
	}
	if cfg.Logger == nil {
		return nil, errors.New("daemon: Logger is required")
	}
	if cfg.Runner == nil {
		return nil, errors.New("daemon: Runner is required")
	}
	if cfg.Clock == nil {
		cfg.Clock = ports.SystemClock{}
	}
	if cfg.Probe == nil {
		cfg.Probe = httpHealthProbe
	}
	if cfg.StartTimeout == 0 {
		cfg.StartTimeout = 15 * time.Second
	}
	if cfg.HealthInterval == 0 {
		cfg.HealthInterval = 100 * time.Millisecond
	}
	if cfg.ShutdownGrace == 0 {
		cfg.ShutdownGrace = 5 * time.Second
	}
	if cfg.VerifyBinary == nil {
		cfg.VerifyBinary = rclonebin.VerifyChecksum
	}
	if cfg.RestartBackoffBase == 0 {
		cfg.RestartBackoffBase = 200 * time.Millisecond
	}
	if cfg.RestartBackoffMax == 0 {
		cfg.RestartBackoffMax = 10 * time.Second
	}
	return &Supervisor{
		cfg:     cfg,
		backoff: backoff{base: cfg.RestartBackoffBase, max: cfg.RestartBackoffMax},
	}, nil
}

// Addr returns the loopback address the daemon is listening on, or ErrNotRunning.
func (s *Supervisor) Addr() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state != stateRunning {
		return "", ErrNotRunning
	}
	return s.addr, nil
}

// Credentials returns the in-memory rc session credentials, or ErrNotRunning.
func (s *Supervisor) Credentials() (Credentials, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state != stateRunning {
		return Credentials{}, ErrNotRunning
	}
	return s.creds, nil
}

// Start verifies the pinned binary, generates per-session credentials, launches
// the daemon, and blocks until it is healthy. It then supervises the process,
// restarting it with backoff on unexpected exit until Stop or context
// cancellation. Start fails if integrity verification or the first launch fails.
func (s *Supervisor) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.state == stateRunning {
		s.mu.Unlock()
		return errors.New("daemon already running")
	}
	s.mu.Unlock()

	if err := s.cfg.VerifyBinary(s.cfg.BinaryPath); err != nil {
		return err
	}
	if err := s.verifyVersion(ctx); err != nil {
		return err
	}

	creds, err := generateCredentials()
	if err != nil {
		return err
	}
	addr, err := freeLoopbackAddr()
	if err != nil {
		return err
	}

	s.mu.Lock()
	s.creds = creds
	s.addr = addr
	s.stopCh = make(chan struct{})
	s.stopOnce = sync.Once{}
	s.done = make(chan struct{})
	s.ready = make(chan error, 1)
	s.mu.Unlock()

	go s.manage(ctx)

	// Block until the first launch reports ready (or failed).
	if err := <-s.ready; err != nil {
		<-s.done // ensure the manager goroutine has fully exited
		return err
	}

	s.mu.Lock()
	s.state = stateRunning
	s.mu.Unlock()
	return nil
}

// Stop gracefully shuts the daemon down (SIGTERM, then SIGKILL after the grace
// period) and waits for the supervising goroutine to exit, leaving no orphaned
// process. Stopping an already-stopped supervisor is a no-op.
func (s *Supervisor) Stop(ctx context.Context) error {
	s.mu.Lock()
	if s.state != stateRunning {
		s.mu.Unlock()
		return nil
	}
	done := s.done
	s.mu.Unlock()

	s.signalStop()

	select {
	case <-done:
	case <-ctx.Done():
		return fmt.Errorf("waiting for daemon shutdown: %w", ctx.Err())
	}

	s.mu.Lock()
	s.state = stateStopped
	s.mu.Unlock()
	return nil
}

func (s *Supervisor) signalStop() {
	s.stopOnce.Do(func() { close(s.stopCh) })
}

// manage is the single goroutine owning the process. It launches, waits, and
// either shuts down (on stop/cancel) or restarts with backoff (on unexpected
// exit). It closes s.done on return.
func (s *Supervisor) manage(parentCtx context.Context) {
	defer close(s.done)

	first := true
	for {
		proc, cancel, err := s.launch(parentCtx)
		if err != nil {
			if first {
				s.ready <- err
				return
			}
			s.cfg.Logger.Error("rcd relaunch failed", "error", err)
			if !s.waitBackoff(parentCtx) {
				return
			}
			continue
		}
		if first {
			s.ready <- nil
			first = false
			s.backoff.reset()
		}

		exit := make(chan error, 1)
		go func() { exit <- proc.Wait() }()

		select {
		case <-s.stopCh:
			s.gracefulShutdown(proc, cancel, exit)
			return
		case <-parentCtx.Done():
			s.cfg.Logger.Info("context cancelled; shutting down rcd")
			s.gracefulShutdown(proc, cancel, exit)
			return
		case werr := <-exit:
			cancel()
			s.cfg.Logger.Warn("rcd exited unexpectedly; restarting", "error", werr, "pid", proc.Pid())
			if !s.waitBackoff(parentCtx) {
				return
			}
		}
	}
}

// launch starts one rcd process and waits for it to become healthy. It returns
// the process and a cancel func that hard-kills it (last resort), or an error.
func (s *Supervisor) launch(parentCtx context.Context) (Process, context.CancelFunc, error) {
	// The process context is deliberately independent of parentCtx: cancelling
	// the parent must trigger a *graceful* SIGTERM shutdown (handled in manage),
	// not exec's hard kill. We hold cancel for last-resort escalation only.
	procCtx, cancel := context.WithCancel(context.Background())

	proc, err := s.cfg.Runner.Start(procCtx, s.daemonSpec()) //nolint:contextcheck // intentional: see comment above
	if err != nil {
		cancel()
		return nil, nil, coreerr.New(coreerr.CodeDaemonStart, "starting rcd", err)
	}

	healthCtx, healthCancel := context.WithTimeout(parentCtx, s.cfg.StartTimeout)
	defer healthCancel()
	if err := waitHealthy(healthCtx, s.cfg.Probe, s.addr, s.creds, s.cfg.StartTimeout, s.cfg.HealthInterval); err != nil {
		// Tear down the unhealthy process before returning.
		_ = proc.Signal(syscall.SIGKILL)
		cancel()
		_ = proc.Wait()
		return nil, nil, coreerr.New(coreerr.CodeDaemonStart, "rcd did not become healthy", err)
	}

	s.cfg.Logger.Info("rcd healthy", "addr", s.addr, "pid", proc.Pid())
	return proc, cancel, nil
}

// gracefulShutdown sends SIGTERM, waits up to the grace period for exit, then
// escalates to SIGKILL via context cancellation, finally reaping the process.
func (s *Supervisor) gracefulShutdown(proc Process, cancel context.CancelFunc, exit <-chan error) {
	_ = proc.Signal(syscall.SIGTERM)

	timer := time.NewTimer(s.cfg.ShutdownGrace)
	defer timer.Stop()

	select {
	case <-exit:
		cancel()
		s.cfg.Logger.Info("rcd stopped gracefully", "pid", proc.Pid())
		return
	case <-timer.C:
		s.cfg.Logger.Warn("rcd did not stop in time; sending SIGKILL", "pid", proc.Pid())
		_ = proc.Signal(syscall.SIGKILL)
		cancel()
		<-exit // reap
	}
}

// waitBackoff sleeps for the next backoff interval, returning false if the
// supervisor was stopped or the context cancelled during the wait.
func (s *Supervisor) waitBackoff(parentCtx context.Context) bool {
	d := s.backoff.next()
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-s.stopCh:
		return false
	case <-parentCtx.Done():
		return false
	}
}

// daemonSpec assembles the argv for `rclone rcd` (ADR-0004). Credentials are
// passed as explicit arguments to the loopback-bound daemon.
func (s *Supervisor) daemonSpec() Spec {
	args := []string{
		"rcd",
		"--rc-addr", s.addr,
		"--rc-user", s.creds.User,
		"--rc-pass", s.creds.Pass,
	}
	if s.cfg.ConfigPath != "" {
		args = append(args, "--config", s.cfg.ConfigPath)
	}
	return Spec{Path: s.cfg.BinaryPath, Args: args, Env: os.Environ()}
}

// verifyVersion runs `rclone version` and confirms it reports PinnedVersion, in
// addition to the checksum check (ADR-0008).
func (s *Supervisor) verifyVersion(ctx context.Context) error {
	out, err := s.cfg.Runner.Output(ctx, Spec{Path: s.cfg.BinaryPath, Args: []string{"version"}})
	if err != nil {
		return coreerr.New(coreerr.CodeRcloneBinaryMissing, "running rclone version", err)
	}
	if !strings.Contains(string(out), rclonebin.PinnedVersion) {
		return coreerr.New(coreerr.CodeRcloneChecksum,
			fmt.Sprintf("rclone reports an unexpected version; expected %s", rclonebin.PinnedVersion), nil)
	}
	return nil
}
