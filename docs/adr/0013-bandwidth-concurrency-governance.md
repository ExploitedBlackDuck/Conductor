# ADR-0013 — Bandwidth/concurrency governance with conservative defaults

- Status: Accepted
- Date: 2026-06

## Context

Unbounded transfers can saturate the operator's link and trip cloud-provider
rate limits or bans (high concurrency on object stores).

## Decision

Conductor exposes bandwidth and concurrency governance — `--bwlimit`,
`--transfers`, `--checkers`, `--tpslimit` — with conservative defaults set
globally, overridable per operation, and an optional per-remote ceiling (e.g. a
rate-limited cloud remote). Limits are surfaced in the option builder and
recorded with the operation.

## Consequences

Safe-by-default behaviour; going faster is an explicit, recorded choice. Cost:
slower default transfers.
