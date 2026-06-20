# ADR-0001 — Wails v2 now, v3 as a contained migration

- Status: Accepted
- Date: 2026-06

## Context

Wails v3 is the framework's future (GTK4 on Linux, better bindings) but is still
on the alpha line as of mid-2026 (`v3.0.0-alpha.96`), not a tagged stable
release. Wails v2 is the stable, battle-tested production line.

## Decision

Build on Wails v2. Confine all Wails runtime calls (events, dialogs, lifecycle)
behind a thin internal `shell` package that depends on *our* interface, not on
Wails directly. A future v3 migration then touches only `app/` and `shell/`, not
the core.

## Consequences

A little indirection now; near-zero core churn when v3 stabilizes. Revisit when
v3 cuts a stable tag. The `shell.Runtime` interface is the single seam; no
package under `internal/core` may import Wails, enforced by `depguard`.
