# Conductor

**A desktop control panel for [`rclone`](https://rclone.org), built with Wails.**

Conductor makes rclone's power legible: manage remotes, run and monitor
transfers and syncs, manage mounts, and watch live throughput — without touching
a terminal. It lets the operator choose rclone's options safely, warns before a
flag combination puts data at risk, records every operation for audit, and keeps
a queryable history of what was moved, when, and to where.

Targets **macOS and Linux**.

## Why it is safe to run

Conductor moves and deletes data, so safety is a first-class property:

- **No destructive operation runs without an explicit, typed confirmation.** A
  `sync` that deletes, a `delete`, a `purge`, or a bisync `--resync` requires an
  acknowledgement that is recorded in a tamper-evident audit log. There is no
  flag that disables this.
- **Flags are chosen from a typed catalog**, not typed free-hand. An impact
  engine explains what an option does and warns when a combination changes the
  risk to your data. The exact effective command is previewed before it runs.
- **rclone is spawned argv-style, never through a shell** — no command-injection
  surface.
- **No telemetry.** Zero analytics, crash reporting, or network calls you did
  not initiate. Logs stay on your machine. (ADR-0006.)
- **The pinned rclone binary is checksum-verified at every launch**; a tampered
  or mismatched binary disables operations with a clear message (ADR-0008).

## Status

Under active development against `PROJECT-BOOK.md`, phase by phase (§7.10).

- **P0 — foundation: complete.** Repository skeleton, engineering charter
  tooling (golangci-lint, gofumpt, govulncheck), XDG configuration, structured
  logging with secret redaction, ADRs, and a Wails window that opens and logs
  startup.

## Building from source

Requirements: Go (toolchain pinned in `go.mod`), Node.js + npm, and the
[Wails v2](https://wails.io) CLI. On macOS, the Xcode Command Line Tools.

```sh
# one-off: install the developer tooling used by the gates
go install mvdan.cc/gofumpt@latest
go install golang.org/x/vuln/cmd/govulncheck@latest
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
go install github.com/go-task/task/v3/cmd/task@latest

task frontend:install   # install frontend dependencies from the lockfile
task build              # build the embedded, window-opening binary
task lint               # gofumpt + go vet + golangci-lint + govulncheck
task test               # unit tests (green with no rclone/keyring present)
```

For live development with reload:

```sh
task run                # wails dev
```

## License

Conductor is licensed under the Apache License 2.0 — see [`LICENSE`](LICENSE).
Conductor supervises rclone but does not reimplement it; the rclone binary
remains under its own license — see [`NOTICE`](NOTICE).
