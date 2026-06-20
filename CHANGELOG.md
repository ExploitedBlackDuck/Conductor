# Changelog

All notable changes to Conductor are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- **P0 — foundation.** Repository skeleton and the engineering-charter tooling:
  - Architecture Decision Records 0001–0013 (`docs/adr/`).
  - Curated `.golangci.yml`, `gofumpt`, and `govulncheck`, wired into a
    `Taskfile.yml` whose `lint` / `test` / `build` targets are the phase gates.
  - XDG-respecting path resolver and a single typed TOML configuration loader.
  - Structured `log/slog` operational logging with a redacting handler that
    masks secrets at the boundary (proved by test).
  - The `shell` package isolating the Wails runtime behind Conductor's own
    interface (ADR-0001), a thin `app` binding layer, and a composition root
    that opens a window and logs startup.
  - Governance files: `LICENSE` (Apache-2.0), `NOTICE`, `SECURITY.md`,
    `CONTRIBUTING.md`, and this changelog.

[Unreleased]: https://github.com/conductor-app/conductor/commits/main
