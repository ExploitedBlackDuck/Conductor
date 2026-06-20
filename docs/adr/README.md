# Architecture Decision Records

Each ADR records one architectural decision and its rationale. Deviating from a
decision is a **new ADR with a "supersedes" link**, not an undocumented edit
(PROJECT-BOOK.md §0). When the book appears wrong, the path is to raise it and
amend the book — see `CONTRIBUTING.md`.

| ADR | Decision |
|---|---|
| [0001](0001-wails-v2-with-contained-v3-migration.md) | Wails v2 now, v3 as a contained migration |
| [0002](0002-svelte-frontend.md) | Svelte for the frontend |
| [0003](0003-drive-rcd-over-rc-api.md) | Drive `rclone rcd` over the rc API |
| [0004](0004-argv-style-subprocesses.md) | Subprocesses spawned argv-style; never a shell |
| [0005](0005-supervised-daemon-lifecycle.md) | Supervised daemon lifecycle |
| [0006](0006-no-telemetry.md) | No telemetry |
| [0007](0007-embedded-sqlite.md) | Embedded pure-Go SQLite for history/audit |
| [0008](0008-rclone-binary-pinned-and-verified.md) | rclone binary pinned and integrity-verified |
| [0009](0009-secrets-keyring-and-at-rest-encryption.md) | Keyring secrets + app-layer at-rest encryption |
| [0010](0010-operation-capture-and-hash-chained-audit-log.md) | Operation capture + hash-chained audit log |
| [0011](0011-typed-option-catalog-with-impact-warnings.md) | Typed option catalog with impact warnings |
| [0012](0012-signed-macos-reproducible-linux.md) | Signed macOS, reproducible Linux packaging |
| [0013](0013-bandwidth-concurrency-governance.md) | Bandwidth/concurrency governance |
