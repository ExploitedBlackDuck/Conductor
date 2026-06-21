# Changelog

All notable changes to Conductor are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- **P0 — foundation.** Repository skeleton and the engineering-charter tooling:
  - Architecture Decision Records 0001–0013 (`docs/adr/`).
  - Curated `.golangci.yml`, `gofumpt`, and `govulncheck`, wired into a
    `Taskfile.yml` whose `lint` / `test` / `build` targets are the phase gates.
  - XDG-respecting path resolver and a single typed TOML configuration loader.
  - Structured `log/slog` operational logging with a redacting handler that
    masks secrets at the boundary (proved by test).
  - The `shell` package isolating the Wails runtime behind Conductor's own
    interface (ADR-0001), a thin `app` binding layer, and a composition root
    that opens a window and logs startup.
  - Governance files: `LICENSE` (Apache-2.0), `NOTICE`, `SECURITY.md`,
    `CONTRIBUTING.md`, and this changelog.
- **P1 — persistence foundation.** `Store` port + pure-Go SQLite adapter with
  embedded forward-only migrations and a schema-version check; the append-only,
  hash-chained audit log; keyring `SecretStore`, per-install data key, and
  AEAD seal/open at rest (ADR-0007/0009/0010).
- **P2 — supervised daemon + read-only rc client.** Pinned, checksum-verified
  `rcd` lifecycle (start/health/restart/graceful shutdown, no orphans);
  typed rc methods with fixtures; read-only status view (ADR-0005/0008).
- **P3 — option catalog + flag builder + impact engine.** Typed catalog for the
  pinned rclone, validated flag builder (no free-text flags; `conflicts_with`/
  `requires`/ceiling clamp), impact rules, and a generated option UI with risk
  badges (ADR-0011).
- **P4 — live dashboard.** Context-owned poll loop → typed diff events → store →
  aggregate throughput, active jobs, and an error feed.
- **P5 — transfers (copy/move) with cancel + capture + history.** Runs through
  the rc daemon, live per-file progress, cancel via `job/stop`+context; the
  immutable `Operation` is persisted with its sealed captured log and audited.
- **P6 — mounts.** Mount/unmount/list with audit entries.
- **P7 — sync + bisync with the destructive-op preview gate + governance.**
  - The **dry-run preview gate** (ADR-0015): a destructive op is refused until
    it has been previewed with a parsed `--dry-run` change set and that concrete
    change set is acknowledged; the change set is parsed from rclone's structured
    output (validated against the pinned binary), sealed at rest, and
    hash-chained into the audit log.
  - Saved sync/bisync pairs, named option profiles, and per-remote governance
    ceilings (ADR-0013); a new pair's first run defaults to dry-run.
  - A Conductor-level **operation-concurrency cap** (configurable; queues
    excess), server-side-eligibility detection, and per-operation cap labelling.
- **P8 — integrity verification + operation history & export.**
  - `check`/`cryptcheck` with audited, persisted results (§7.12).
  - History browser with intention-revealing queries, a "what was moved" CSV/JSON
    export, retention/clear, and per-operation detail.
  - The audit log gained a **signed chain head** (a separate keyring key) so a
    full recompute is detectable, surfaced in the audit view (ADR-0010).
- **Resilience.** A single-instance data-dir lock (the single-writer/single-
  appender precondition) and daemon-restart reconciliation that closes orphaned
  operations as `interrupted` (§2.3).
- **P9 — native polish (in progress).** Operator-initiated rclone acquisition
  (download + verify against the manifest) with degraded-state routing, a
  keyboard command palette, and native completion notifications for long
  operations.

### Changed
- ADR-0015's capture mechanism corrected in the project book to match the shipped
  binary: the dry-run preview is captured via a one-shot CLI (`--combined` for
  sync/copy/move, `--use-json-log` for bisync), not over rc.

[Unreleased]: https://github.com/conductor-app/conductor/commits/main
