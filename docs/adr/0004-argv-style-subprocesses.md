# ADR-0004 — Subprocesses spawned argv-style; never through a shell

- Status: Accepted
- Date: 2026-06

## Context

We spawn rclone with operator-influenced arguments (remotes, paths, flags).
Shell interpolation is a command-injection vector; this tool must not have one.

## Decision

Use `exec.CommandContext(ctx, bin, args...)` with explicit argument slices. No
`sh -c`, no command-string concatenation, no operator input reaching a shell.
The rclone binary path is resolved once to an absolute path and validated.

## Consequences

Slightly more verbose construction; eliminates an entire vulnerability class.
Enforced by `gosec` and review.
