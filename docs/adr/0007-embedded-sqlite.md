# ADR-0007 — Embedded SQLite (pure-Go) for history, saved pairs, profiles, and the audit log

- Status: Accepted
- Date: 2026-06

## Context

Live transfer state is ephemeral and belongs to the daemon, but operation
*history*, saved sync/bisync pairs, named option *profiles*, and the audit log
must persist, be queryable ("what did I move to X last week"), and survive
restarts. Cross-platform Wails builds strongly favour a CGO-free toolchain.

## Decision

Use SQLite through the pure-Go `modernc.org/sqlite` driver — a single file in
the platform data dir — with versioned, embedded, forward-only migrations run on
startup behind a schema-version check. The core depends on a `Store` port; SQL
lives only in the `sqlitestore` adapter. Live stats are **not** persisted; only
completed-operation records and audit entries are.

## Consequences

Rich relational queries and history without a server; CGO-free keeps
cross-compilation trivial. Cost: pure-Go SQLite has no transparent at-rest
encryption, so sensitive captured data is sealed at the application layer
(ADR-0009). Alternatives (flat files, bbolt) rejected for a history/query
workload.
