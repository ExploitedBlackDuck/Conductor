// Package procrunner is the os/exec implementation of daemon.Runner (ADR-0004).
// Every process is constructed with an explicit argument slice via
// exec.CommandContext — never a shell — so operator-influenced arguments cannot
// be interpreted as commands.
package procrunner

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/conductor-app/conductor/internal/core/daemon"
)

// Runner spawns processes with os/exec.
type Runner struct{}

// New constructs a Runner.
func New() Runner { return Runner{} }

// Start launches a long-lived process. The supplied context governs hard
// cancellation (a last-resort kill); graceful shutdown is driven by the
// supervisor via signals.
func (Runner) Start(ctx context.Context, spec daemon.Spec) (daemon.Process, error) {
	cmd := exec.CommandContext(ctx, spec.Path, spec.Args...) //nolint:gosec // argv-style, path is a resolved absolute binary (ADR-0004)
	cmd.Env = spec.Env
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting %s: %w", spec.Path, err)
	}
	return &process{cmd: cmd}, nil
}

// Output runs a short-lived command to completion and returns combined output.
func (Runner) Output(ctx context.Context, spec daemon.Spec) ([]byte, error) {
	cmd := exec.CommandContext(ctx, spec.Path, spec.Args...) //nolint:gosec // argv-style, path is a resolved absolute binary (ADR-0004)
	cmd.Env = spec.Env
	out, err := cmd.CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("running %s: %w", spec.Path, err)
	}
	return out, nil
}

// process adapts *exec.Cmd to daemon.Process.
type process struct {
	cmd *exec.Cmd
}

func (p *process) Pid() int { return p.cmd.Process.Pid }

func (p *process) Signal(sig os.Signal) error {
	if err := p.cmd.Process.Signal(sig); err != nil {
		return fmt.Errorf("signalling pid %d: %w", p.Pid(), err)
	}
	return nil
}

func (p *process) Wait() error {
	// Wait returns a non-nil error for any non-zero exit, including a signalled
	// termination; the supervisor interprets that against its intent.
	return p.cmd.Wait()
}
