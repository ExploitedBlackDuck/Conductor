# ADR-0005 — Supervised daemon lifecycle

- Status: Accepted
- Date: 2026-06

## Context

An unmanaged `rcd` can orphan; its rc surface and behaviour can drift across
rclone releases.

## Decision

Conductor owns the daemon's full lifecycle — start, health, restart with
backoff, graceful-then-hard shutdown (SIGTERM then SIGKILL after a deadline),
and reaping — bound to a pinned rclone version (integrity per ADR-0008).

## Consequences

Reproducible behaviour; no orphaned daemons (verified by an integration test).
Cost: explicit lifecycle code.
