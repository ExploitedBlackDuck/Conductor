# ADR-0008 — rclone binary pinned and integrity-verified; never silently downloaded

- Status: Accepted
- Date: 2026-06

## Context

Conductor executes the rclone binary with the operator's privileges. Its
provenance is a supply-chain concern, and silent background downloads are
unacceptable.

## Decision

Pin an exact rclone version and resolve it from a configured location. At
startup, verify its SHA-256 against a committed, version-locked manifest (and the
upstream-published checksum where available) in addition to parsing
`rclone version`. Acquisition is an explicit, operator-initiated step, never an
automatic fetch. A distribution-bundled binary is verified the same way.

## Consequences

A known, reproducible engine; tampering is detected before the daemon starts.
On mismatch the daemon refuses to start and the UI shows a remediation message
(`ERR_RCLONE_BINARY_CHECKSUM`, §8.4). Cost: a deliberate install/update step.
