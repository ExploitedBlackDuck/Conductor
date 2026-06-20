# ADR-0011 — rclone options are a typed, data-driven catalog with impact warnings — not free-text flags

- Status: Accepted
- Date: 2026-06

## Context

rclone exposes a very large, evolving flag set, and some flags or combinations
change the risk to the data being moved (delete-on-sync, `--no-check-dest`,
`--ignore-existing`, bisync `--resync`) or the load on the remote (high
`--transfers`/`--checkers`, no `--bwlimit`). Free-text flag strings are both an
injection surface and a footgun.

## Decision

A versioned catalog describes every exposed option as typed metadata (flag,
value type, default, category, help text, risk level, `affects_data`,
`conflicts_with`, `requires`, `impacts`). The UI is generated from it. A flag
builder turns selections into validated rc parameters / argv (per ADR-0004,
never a shell). An impact-rule engine evaluates the selection (plus operation
kind and source/dest) and surfaces warnings, requires acknowledgement, or
hard-blocks before execution.

## Consequences

Flag selection is safe, explained, validated, and auditable; the catalog doubles
as documentation. Cost: the catalog is maintained per pinned rclone version and
tested against the binary's actual flags so it cannot drift silently. Any
"advanced raw flags" escape hatch is catalog-validated and flagged in the audit
log.
