# ADR-0003 — Drive `rclone rcd` over the rc API; do not shell out per command

- Status: Accepted
- Date: 2026-06

## Context

rclone exposes a full JSON control API via the `rcd` daemon. Per-invocation
shelling loses stateful features (running jobs, live transfer stats, the mount
registry) and is slower.

## Decision

Supervise a single long-lived `rclone rcd` (auth on, bound to loopback) and talk
to it over the rc API. Use one-shot CLI calls only where no rc equivalent
exists.

## Consequences

We own the daemon lifecycle (ADR-0005). We get jobs, stats, and mounts cleanly.
The rc client is the main adapter to test; every rc response shape gets a typed
Go struct and a captured fixture under `testdata/rc/`.
