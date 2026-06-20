# Contributing to Conductor

`PROJECT-BOOK.md` is the specification and these standards are binding. Work in
the phases defined in PROJECT-BOOK.md §7.10, in order; a phase is not started
until the previous phase's gate is green.

## Before writing code
- Read PROJECT-BOOK.md §2 (Engineering Charter) and §7 (specification).
- Destructive-operation safety (§7.4) has NO bypass. Do not add one.

## Every change
- context.Context is the first parameter of any I/O / process / network function.
- Errors wrapped with %w + context; typed sentinels for branched-on errors
  (e.g. ErrDaemonNotRunning); error codes from the §8.4 catalog; no string
  matching on error text; no panic outside main/tests.
- Structured logging via slog; secrets are never logged (use redact()). The
  operational log is not the audit log.
- No global mutable state; dependencies constructed and injected in
  cmd/conductor/main.go.
- Subprocesses spawned with exec.CommandContext and argv slices. Never a shell.
- rclone flags come from the catalog and the flag builder; no free-text flags.
- Captured logs / sensitive values are sealed before disk; the data key lives in
  the keyring; rclone remote credentials are never copied into our store.
- No wails import outside app/ and shell/ (enforced by depguard).
- No authorship/tooling fingerprints in any file, commit, or metadata.

## Definition of a finished phase
Run and record the output of:

    task lint               # gofumpt + golangci-lint + go vet + govulncheck
    task test               # go test ./... green on a bare machine (no rclone)
    task test:integration   # tagged; runs against a real rclone/keyring if present
    task build              # runnable app for darwin + linux

A phase is not done until its gate (PROJECT-BOOK.md §7.10) passes. "It runs" is
not the gate.

## Not acceptable
- Placeholder packages, stub tests that assert true, or empty TODOs.
- Business logic in app/ or main.go; SQL composed outside the store adapter.
- A dependency added without pinning it and recording why.
- Letting a destructive rclone op run without an explicit confirm + acknowledgement.
- Real data (remotes, paths, credentials) committed anywhere, incl. testdata.

## Commits
- Conventional Commits, one logical change per commit, every commit builds.
- No "wip"/"fix"/"stuff" messages.

## When the book is wrong
Open the disagreement as a proposed ADR and amend the book. Do not diverge
silently.
