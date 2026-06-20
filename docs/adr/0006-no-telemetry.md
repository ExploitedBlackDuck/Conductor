# ADR-0006 — No telemetry

- Status: Accepted
- Date: 2026-06

## Context

Trust is part of the product. A data-movement tool that phones home is a
liability.

## Decision

Zero analytics, crash reporting, or network calls the user did not initiate.
Logs are local. This is stated in the README as a feature.

## Consequences

No usage insight by design. Operator-initiated, integrity-checked updates only;
no silent auto-update (see ADR-0008, ADR-0012).
