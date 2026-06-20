# ADR-0010 — Operation capture + an append-only, hash-chained audit log

- Status: Accepted
- Date: 2026-06

## Context

A data-movement tool must be able to answer, durably and verifiably, what it did
— especially for destructive operations.

## Decision

Every operation's resolved parameters, full argv/rc-params, and captured rclone
job log/stats are persisted (ADR-0007, ADR-0009). Every consequential action
(operation start/stop, destructive-op confirmation and risk acknowledgement,
mount/unmount, governance-ceiling change, export) is recorded as an append-only
audit entry whose hash chains to the previous entry:
`hash = SHA256(prev_hash || canonical(entry))`. The chain is verifiable and
exportable.

## Consequences

A complete, tamper-evident record. Tamper detection is surfaced in the UI
(`ERR_AUDIT_CHAIN_BROKEN`, §8.4). Cost: storage volume, managed by retention,
compression, and encryption (§7.7–7.8).
