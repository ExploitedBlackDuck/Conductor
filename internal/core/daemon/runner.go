// Package daemon supervises the single long-lived `rclone rcd` process Conductor
// talks to over the rc API (ADR-0003, ADR-0005). It owns the daemon's full
// lifecycle: integrity-verify the pinned binary, start it bound to loopback with
// per-session credentials, wait for health, restart with backoff on unexpected
// exit, and shut down gracefully (SIGTERM then SIGKILL) with no orphaned
// process. Subprocesses are spawned argv-style, never through a shell (ADR-0004).
package daemon

import (
	"context"
	"os"
)

// Spec describes a subprocess to launch. Args is an explicit argument slice — it
// is never concatenated into a shell command (ADR-0004).
type Spec struct {
	// Path is the absolute path to the executable.
	Path string
	// Args are the arguments after the program name.
	Args []string
	// Env is the process environment; nil inherits the parent's.
	Env []string
}

// Process is a handle to a started subprocess.
type Process interface {
	// Pid returns the process identifier.
	Pid() int
	// Signal delivers sig to the process.
	Signal(sig os.Signal) error
	// Wait blocks until the process exits and returns its exit error, if any.
	Wait() error
}

// Runner abstracts process creation so the supervisor's lifecycle logic is
// testable without spawning real processes (§2.5). The procrunner adapter
// provides the os/exec implementation.
type Runner interface {
	// Start launches a long-lived process and returns immediately with a handle.
	Start(ctx context.Context, spec Spec) (Process, error)
	// Output runs a short-lived command to completion and returns its combined
	// standard output, for one-shot calls such as `rclone version`.
	Output(ctx context.Context, spec Spec) ([]byte, error)
}
