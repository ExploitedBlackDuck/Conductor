package daemon

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/conductor-app/conductor/internal/core/coreerr"
	"github.com/conductor-app/conductor/internal/core/rclonebin"
)

// fakeProcess simulates a child process: it stays alive until signalled or until
// its controller forces an immediate exit.
type fakeProcess struct {
	pid     int
	mu      sync.Mutex
	signals []os.Signal
	exited  chan struct{}
	once    sync.Once
}

func newFakeProcess(pid int) *fakeProcess {
	return &fakeProcess{pid: pid, exited: make(chan struct{})}
}

func (p *fakeProcess) Pid() int { return p.pid }

func (p *fakeProcess) Signal(sig os.Signal) error {
	p.mu.Lock()
	p.signals = append(p.signals, sig)
	p.mu.Unlock()
	// SIGTERM/SIGKILL cause the process to exit.
	p.exit()
	return nil
}

func (p *fakeProcess) Wait() error {
	<-p.exited
	return nil
}

func (p *fakeProcess) exit() { p.once.Do(func() { close(p.exited) }) }

func (p *fakeProcess) gotSignal(sig os.Signal) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, s := range p.signals {
		if s == sig {
			return true
		}
	}
	return false
}

// fakeRunner hands out fakeProcesses and records how many were started.
type fakeRunner struct {
	mu          sync.Mutex
	started     int
	versionText string
	// makeProcess optionally customises each started process (e.g. to exit
	// immediately, simulating a crash).
	makeProcess func(n int) *fakeProcess
	last        *fakeProcess
}

func (r *fakeRunner) Start(_ context.Context, _ Spec) (Process, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.started++
	var p *fakeProcess
	if r.makeProcess != nil {
		p = r.makeProcess(r.started)
	} else {
		p = newFakeProcess(1000 + r.started)
	}
	r.last = p
	return p, nil
}

func (r *fakeRunner) Output(_ context.Context, _ Spec) ([]byte, error) {
	return []byte(r.versionText), nil
}

func (r *fakeRunner) startCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.started
}

func quietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func baseConfig(r *fakeRunner) Config {
	return Config{
		BinaryPath:         "/fake/rclone",
		Logger:             quietLogger(),
		Runner:             r,
		Probe:              func(context.Context, string, Credentials) error { return nil },
		VerifyBinary:       func(string) error { return nil },
		StartTimeout:       2 * time.Second,
		HealthInterval:     5 * time.Millisecond,
		ShutdownGrace:      time.Second,
		RestartBackoffBase: time.Millisecond,
		RestartBackoffMax:  5 * time.Millisecond,
	}
}

func TestStartStopLifecycle(t *testing.T) {
	t.Parallel()
	r := &fakeRunner{versionText: "rclone " + rclonebin.PinnedVersion}
	s, err := New(baseConfig(r))
	require.NoError(t, err)

	ctx := context.Background()
	require.NoError(t, s.Start(ctx))

	addr, err := s.Addr()
	require.NoError(t, err)
	assert.NotEmpty(t, addr)

	creds, err := s.Credentials()
	require.NoError(t, err)
	assert.NotEmpty(t, creds.User)
	assert.NotEmpty(t, creds.Pass)

	proc := r.last
	require.NoError(t, s.Stop(ctx))

	assert.True(t, proc.gotSignal(syscall.SIGTERM), "graceful stop must send SIGTERM")

	// After stop, accessors report not running.
	_, err = s.Addr()
	assert.ErrorIs(t, err, ErrNotRunning)
}

func TestStartFailsOnVerifyBinary(t *testing.T) {
	t.Parallel()
	r := &fakeRunner{versionText: "rclone " + rclonebin.PinnedVersion}
	cfg := baseConfig(r)
	cfg.VerifyBinary = func(string) error {
		return coreerr.New(coreerr.CodeRcloneChecksum, "bad checksum", nil)
	}
	s, err := New(cfg)
	require.NoError(t, err)

	err = s.Start(context.Background())
	require.Error(t, err)
	code, ok := coreerr.CodeOf(err)
	require.True(t, ok)
	assert.Equal(t, coreerr.CodeRcloneChecksum, code)
	assert.Zero(t, r.startCount(), "daemon must not start when integrity fails")
}

func TestStartFailsOnVersionMismatch(t *testing.T) {
	t.Parallel()
	r := &fakeRunner{versionText: "rclone v0.0.1"}
	s, err := New(baseConfig(r))
	require.NoError(t, err)

	err = s.Start(context.Background())
	require.Error(t, err)
	code, ok := coreerr.CodeOf(err)
	require.True(t, ok)
	assert.Equal(t, coreerr.CodeRcloneChecksum, code)
}

func TestStartFailsWhenNeverHealthy(t *testing.T) {
	t.Parallel()
	r := &fakeRunner{versionText: "rclone " + rclonebin.PinnedVersion}
	cfg := baseConfig(r)
	cfg.StartTimeout = 150 * time.Millisecond
	cfg.Probe = func(context.Context, string, Credentials) error {
		return errors.New("connection refused")
	}
	s, err := New(cfg)
	require.NoError(t, err)

	err = s.Start(context.Background())
	require.Error(t, err)
	code, ok := coreerr.CodeOf(err)
	require.True(t, ok)
	assert.Equal(t, coreerr.CodeDaemonStart, code)
}

func TestUnexpectedExitTriggersRestart(t *testing.T) {
	t.Parallel()
	r := &fakeRunner{versionText: "rclone " + rclonebin.PinnedVersion}
	// The first process crashes immediately; later ones stay up.
	r.makeProcess = func(n int) *fakeProcess {
		p := newFakeProcess(2000 + n)
		if n == 1 {
			p.exit() // already-dead process simulates an immediate crash
		}
		return p
	}
	s, err := New(baseConfig(r))
	require.NoError(t, err)

	ctx := context.Background()
	require.NoError(t, s.Start(ctx))

	// The supervisor should relaunch after the first process's unexpected exit.
	require.Eventually(t, func() bool { return r.startCount() >= 2 }, time.Second, 5*time.Millisecond,
		"supervisor must restart the daemon after an unexpected exit")

	require.NoError(t, s.Stop(ctx))
}
