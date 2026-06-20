# ADR-0014 — Composition root at the repository root (supersedes the §5 cmd/conductor path)

- Status: Accepted
- Date: 2026-06
- Supersedes: the PROJECT-BOOK.md §5 placement of the composition root at `cmd/conductor/main.go`

## Context

PROJECT-BOOK.md §5 places the composition root at `cmd/conductor/main.go`, the
common Go convention. However, the Wails v2 toolchain — `wails build`,
`wails dev`, and `wails generate module` — discovers the application's bound
types by compiling the **main package at the project root**. With the entrypoint
under `cmd/conductor`, `wails generate module` fails with "no Go files in
<root>", so the **generated, typed frontend bindings** that §2.8 of the charter
*requires* cannot be produced. §2.8 (typed generated bindings, never hand-rolled
bridges) is a binding charter rule; the §5 file path is a layout convention.

## Decision

Place the composition root at the repository-root `main.go` (package `main`).
It remains thin — it resolves paths/config, constructs and wires dependencies,
and calls `shell.Run`; it contains no business logic, exactly as §5 requires of
the composition root. Wails confinement is unchanged: `main.go` does **not**
import Wails (ADR-0001); it calls `shell.Run`, and Wails stays inside `app/` and
`shell/`. depguard continues to forbid Wails imports everywhere except `app/`
and `shell/`, now including the root `main.go`.

`wails generate module` (driven by the `bindings` Task target) regenerates
`frontend/wailsjs/`, which the frontend imports as its only Go bridge.

## Consequences

Generated typed bindings work, satisfying §2.8. The composition root moves from
`cmd/conductor/main.go` to `main.go`; the "only composition root, kept thin,
business logic here is a defect" rule is unchanged. The §5 directory sketch is
updated to match.
