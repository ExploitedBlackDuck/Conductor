# ADR-0012 — Signed/notarized macOS, reproducibly packaged Linux

- Status: Accepted
- Date: 2026-06

## Context

An unsigned utility that fights Gatekeeper, or ships as a mystery binary, will
not be trusted or used. Credible distribution is part of the product.

## Decision

macOS builds are signed with a Developer ID and notarized (hardened runtime,
minimal entitlements, stapled). Linux ships as a versioned AppImage plus a
`.deb`, built reproducibly in CI from pinned toolchains. Every release is a
signed semver git tag with a maintained `CHANGELOG.md` and published SHA-256
checksums.

## Consequences

Installs cleanly with verifiable provenance. Cost: a signing identity and a
notarization step. Updates are operator-initiated and integrity-checked — no
silent auto-update.
