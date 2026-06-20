# ADR-0009 — Secrets in the OS keyring; sensitive persisted data encrypted at the application layer

- Status: Accepted
- Date: 2026-06

## Context

The rc session credentials are sensitive; captured job logs may contain paths or
tokens-in-URLs; pure-Go SQLite (ADR-0007) offers no transparent encryption.
Conductor does not store rclone remote credentials — `rclone.conf` owns those.

## Decision

The rc session user/pass is generated per session and held in memory only (never
written to disk or logged). On first run the app generates a random per-install
data key, stores it in the OS keyring (macOS Keychain / Linux Secret Service),
and uses it to seal sensitive persisted fields — captured job logs and any saved
sensitive values — with an AEAD (XChaCha20-Poly1305) before they touch disk. The
database and data dir are OS-permission-restricted (0700/0600).

## Consequences

Secrets are never on disk or in logs; captured data at rest is unreadable
without the keyring entry. Cost: sealed fields are not directly searchable;
search runs over non-sensitive history columns (remote, kind, time).
