# Project Book — Conductor

**A desktop control panel for `rclone`, built with Wails.**

Conductor makes rclone's power legible: manage remotes, run and monitor transfers and syncs, manage mounts, and watch live throughput — without touching a terminal. It lets the operator choose rclone's options safely, warns before a flag combination puts data at risk, records every operation for audit, and keeps a queryable history of what was moved, when, and to where. It is the GUI that rclone's `rcd` control API always deserved.

Targets **macOS and Linux**. This book is the engineering contract for the project: the standards and specification any implementation is held to.

> **Verified facts this book relies on (June 2026):** Wails v3 is still pre-release (alpha line, `v3.0.0-alpha.96`); maintainers call the API "reasonably stable" with production users, but it is not a tagged stable release. Wails v2 is the stable production line. `rclone` exposes a full JSON control API via the `rclone rcd` daemon. These drive ADR-0001 and ADR-0003. Re-verify before locking dependency versions.

---

## 0. How to use this book

This document is the source of truth. Implementers do not invent architecture, pick libraries, or reinterpret scope on their own; where the book appears wrong, the path is to **raise it and amend the book**, not to diverge silently.

1. **Build in the phases defined in §7.10, in order.** Do not start a phase until the previous phase's acceptance gate is green. Do not work ahead.
2. **Every phase ends with a verifiable gate** — a command that passes or fails. "It looks done" is not a gate. Run it; record the output in the PR.
3. **The Engineering Charter (§2) is not advisory.** Each rule maps to a lint rule, a test, or a review-checklist item. Code that violates it is rejected, not patched later.
4. **Write the ADRs (§6) into the repo as the opening commits.** Deviating from a decision is a new ADR with a "supersedes" link, not an undocumented edit.
5. **No scaffolding theater.** No empty packages, no placeholder `TODO` functions, no stub tests asserting `true`. Build thin vertical slices that work end to end, then widen them.
6. **Keep `git` history legible.** Conventional Commits, one logical change per commit, no "wip"/"fix"/"stuff" messages, no commits that don't build.

The failure mode we design against: **code that compiles and demos but reads as machine-extruded** — no boundaries, no error taxonomy, no real tests, magic strings, a 600-line `main.go`. The whole book exists to prevent that.

**One thing is special about this app:** it moves and deletes data. **A destructive operation never runs without an explicit, typed confirmation, and dangerous flag combinations are surfaced before execution.** See ADR-0011 and §7.4. A data-movement tool that can silently delete the wrong thing is a liability, not a product.

---

## 1. Goals and non-goals

### Goals
- A native-feeling desktop control surface for a power-user CLI whose own GUI is weak.
- A **headless core** fully usable and testable without the GUI — the UI is presentation, never the system of record.
- **Safe, explained control:** the operator selects rclone options from a catalog that explains each flag and warns when a combination changes the risk to the data being moved.
- **An accountable tool:** every operation is recorded with its exact parameters and result, kept in a queryable history and an append-only audit log, so "what did I move, when, and to where" always has an answer.
- Cross-platform (macOS + Linux) from one codebase, accepting webview chrome over per-OS native widgets.
- Code quality that survives a hostile senior review, and a repository credible enough to publish.

### Non-goals
- Windows is out of scope for v1 (not excluded by design — just not a tested/supported target yet).
- Mobile is permanently out of scope.
- We do not reimplement rclone. We supervise and present; the upstream binary remains the engine, and `rclone.conf` remains rclone's to own (Conductor reads remote names, never duplicates their credentials).
- We are not a backup scheduler or a multi-user server. Conductor is a local-first single-operator control panel.
- No telemetry, analytics, or phone-home. Silent by default (ADR-0006).

---

## 2. Engineering Charter — the non-negotiables

The anti-"vibe-coded" section. Each rule is enforceable; §12 checks every one.

### 2.1 Architecture boundaries
- **The core tree imports zero UI code.** No `wails` import anywhere under `internal/core/...`. The core compiles and its tests run with `go test ./internal/core/...` on a machine with no webview. Enforced by `depguard`.
- **Dependencies point inward.** Frontend → Wails bindings → application services → domain core. Never the reverse. The domain core knows nothing about rclone's rc HTTP shape, SQLite, or the OS keyring; adapters translate at the edges.
- **Interfaces are declared by the consumer, kept small,** defined where used — not in a junk-drawer `interfaces.go`. A service that needs the rc API declares a 1–3 method port next to itself; the real HTTP client lives in an adapter. Likewise `Store` and `SecretStore` are consumer-defined ports with adapter implementations.

### 2.2 Errors
- **No naked `panic` in library code.** Permitted only in `main` for unrecoverable startup failures, and in tests.
- **Wrap with context using `%w`:** `fmt.Errorf("starting rcd on %s: %w", addr, err)`. Each wrap adds *what we were doing*.
- **Typed/sentinel errors for anything a caller branches on** (`var ErrDaemonNotRunning = errors.New(...)`), checked with `errors.Is`/`errors.As`. No string-matching on `err.Error()`.
- **Errors crossing the Wails boundary are mapped to a typed DTO** (`{code, message, retryable}`) drawn from the enumerated error-code catalog (§8.4). The frontend never receives or parses a raw Go error string.

### 2.3 Concurrency & lifecycle
- **`context.Context` is the first parameter** of every function that does I/O, spawns a process, or makes a network call. Cancellation propagates from the UI ("stop") to `cmd.Cancel`/HTTP request cancellation.
- **No goroutine without a defined exit.** Every `go func()` has an owner that can stop it and a place that waits for it.
- **No global mutable state.** No package-level vars holding services, config, or clients. Constructed in `main`/`app`, injected via constructors.
- **Daemon supervision is explicit:** start, health, restart-with-backoff, graceful shutdown (SIGTERM then SIGKILL after a deadline), reaping. No orphaned `rclone rcd` after the app quits — verified by an integration test.

### 2.4 Logging & observability
- **Structured logging via `log/slog`.** No `fmt.Println`/`log.Printf` debugging in the tree. Levels used meaningfully.
- **Secrets never hit the logs.** rc auth tokens, remote credentials, and the data key are redacted at the logging boundary. A `redact()` helper exists and a test proves a known token never appears in emitted lines.
- **Operational logs are distinct from the audit log.** slog output is for the operator/developer and is rotated/disposable; the audit log (§7.8) is durable, append-only, and tamper-evident. They are never conflated.

### 2.5 Tests
- **Behavior, not coverage theater.** No `assert.True(t, true)`, no tests that only check a constructor returns non-nil.
- **Table-driven** for parsers, mappers, validators, and the impact-rule engine.
- **The HTTP (rc), process, store, and secret layers are mockable** because they sit behind interfaces (§2.1). Pure logic (mapping rc responses, stats diffing, destructive-op guards, command/flag assembly, impact rules) is unit-tested with no external processes and no real database.
- **Integration tests that need the real binary or a real keyring** are behind `//go:build integration` and a presence check; they skip cleanly when unavailable so `go test ./...` is always green on a bare machine.
- **Parsers are tested against captured real fixtures** under `testdata/rc/` — real rc JSON responses, not hand-typed approximations.
- **Store tests run against a real SQLite file** (temp dir), exercising migrations up, queries, and the encrypted-column round-trip — the SQL is the thing under test.

### 2.6 Tooling gates (CI-enforced)
- `gofumpt` (stricter than `gofmt`).
- `golangci-lint` with a **curated, committed `.golangci.yml`**. Minimum: `govet, staticcheck, errcheck, ineffassign, unused, depguard, gocritic, revive, bodyclose, contextcheck, errorlint, nilerr, gosec, sqlclosecheck, rowserrcheck`.
- `go vet ./...` clean.
- `govulncheck ./...` clean — the dependency tree is checked for known vulnerabilities on every CI run.
- Frontend: ESLint + Prettier + `tsc --noEmit` (strict TS). No `any` without an inline justification.
- CI fails on any of the above. No ignored "warning" tier.

### 2.7 Hygiene
- **Every exported symbol has a godoc comment** starting with the symbol name.
- **No commented-out code** in commits.
- **No `TODO`/`FIXME` without an issue reference** (`// TODO(#42): ...`). Bare TODO is a lint failure.
- **Dependencies pinned** (`go.mod`/`go.sum`; frontend lockfile committed). No `@latest` in build scripts. The rclone binary version is pinned and checksum-verified (ADR-0008).
- **No magic strings/numbers** crossing boundaries. rc command names, event names, error codes, operation kinds, and audit action types are typed constants in one place.
- **No AI/authorship fingerprints in the repository.** No "generated by" comments, no assistant names in files, commit authors, or metadata. The repository reads as a normally-authored project.

### 2.8 Frontend discipline
- Typed API only: the frontend calls **generated Wails bindings**, never hand-rolled stringly-typed bridges.
- State lives in defined stores, not scattered component-local state; runtime state (live transfer stream) and view state are distinguished.
- Components small and role-named. No 800-line "App.svelte".
- Event payloads from Go are typed (generated) and validated at the boundary — never `JSON.parse`-and-pray.

### 2.9 Data, history & audit handling
- **An operation is a recorded event.** Every copy/sync/move/delete/purge/mount/bisync carries its resolved parameters, source/dest, tool version, timing, bytes/files moved, and result into persistent history. History rows are immutable; a re-run is a new row.
- **Capture the result.** Each operation's rclone job log/stats is captured and stored with the operation, before it ages out of the live daemon.
- **The audit log is append-only and tamper-evident** (hash-chained — §7.8). It records every operation, with emphasis on destructive ones, and every risk acknowledgement.
- **Sensitive persisted data is encrypted at rest** (ADR-0009): captured logs (which may contain paths or tokens-in-URLs) and any saved sensitive values are sealed before they touch disk.

### 2.10 No real data in the repository
- Fixtures under `testdata/` are sanitized: real remote names, bucket names, hostnames, paths, and credentials are replaced with `example`/documentation placeholders.
- A pre-commit hook **and** a test scan the tree for patterns that look like real targets — non-example registrable domains, anything resembling a bearer token or access key — and fail the build on a hit.
- This mirrors standard OPSEC: worked examples are sanitized before they leave the operator's machine.

---

## 3. Architecture

```
┌──────────────────────────────────────────────────────────────┐
│  Frontend (Svelte, in webview)                                 │
│  - remotes, transfers/jobs, mounts, bisync, option builder     │
│  - live dashboard, operation history, audit view               │
│  - calls generated bindings; subscribes to typed events        │
└───────────────▲────────────────────────────────┬──────────────┘
                │ generated bindings              │ typed events (live stats)
┌───────────────┴────────────────────────────────▼──────────────┐
│  app/  — Wails binding layer (ONLY place importing wails)       │
│  - thin: UI calls → core service calls                          │
│  - map core errors → typed DTOs; core events → UI events        │
│  - NO business logic                                            │
└───────────────▲────────────────────────────────┬──────────────┘
┌───────────────┴────────────────────────────────▼──────────────┐
│  internal/core/  — headless engine (no UI imports)              │
│  - daemon    : rcd supervision                                  │
│  - transfers : jobs + live stats                                │
│  - mounts    : mount service                                    │
│  - options   : rclone option catalog + flag builder + rules     │
│  - history   : operation history queries                        │
│  - audit     : append-only, hash-chained log                    │
│  - store     : Store port + query layer                         │
│  - domain    : Remote/Job/Mount/Stats/Operation/SavedPair/...   │
│  - ports     : small consumer-defined interfaces                │
└───────────────▲────────────────────────────────┬──────────────┘
┌───────────────┴────────────────────────────────▼──────────────┐
│  internal/adapters/                                             │
│  - rcclient   : HTTP client for the rclone rcd rc API           │
│  - procrunner : os/exec impl for daemon supervision             │
│  - sqlitestore: SQLite impl of Store (migrations)               │
│  - keyring    : OS keyring impl of SecretStore                  │
└──────────────────────────────────────────────────────────────┘
```

Defining property: **`internal/core` is a library you could ship without a GUI.** You could put a CLI or a local HTTP server in front of it tomorrow. The Wails layer is one consumer, not the system. rclone already works this way — a daemon with a JSON rc API — so we lean into it; the datastore and secrets are likewise behind ports so the core never depends on a concrete backend.

---

## 4. Tech stack

| Concern | Choice | Notes |
|---|---|---|
| Desktop shell | **Wails v2 (stable)** | ADR-0001. Runtime abstracted behind a `shell` package so v3 migration is contained. |
| Core language | **Go 1.23+** | Pin the toolchain in `go.mod`. |
| Frontend | **Svelte + TypeScript + Vite** | ADR-0002. |
| Frontend state | Typed Svelte stores | Live stats stream + view state kept distinct. |
| Styling | Plain CSS / CSS modules + design tokens | Consult the `frontend-design` skill when building the UI. |
| HTTP (rc client) | stdlib `net/http` + typed client | rc API is small; no client framework. |
| Datastore | **SQLite via `modernc.org/sqlite` (pure Go, no CGO)** | ADR-0007. Operation history, saved pairs/profiles, audit log. Forward-only embedded migrations. |
| At-rest encryption | **XChaCha20-Poly1305 (`golang.org/x/crypto/chacha20poly1305`)** | ADR-0009. Captured logs / sensitive values sealed app-side with a per-install data key. |
| Secrets | **OS keyring (`github.com/zalando/go-keyring`)** | ADR-0009. macOS Keychain / Linux Secret Service. Holds the data key. rc session creds stay in memory. |
| Process supervision | `os/exec` + `context` + supervisor | argv-style, never a shell (ADR-0004). |
| Logging | `log/slog` | JSON to rotating file + pretty in dev. Distinct from the audit log. |
| Config | TOML via a single typed loader | XDG-respecting paths (§4.1). |
| Tests | stdlib `testing` + `testify` (assert/require) | No BDD frameworks. |
| Lint | `golangci-lint`, `gofumpt`, `govulncheck` | Committed config (§2.6). |
| Task runner | `Taskfile` (go-task) | |

### 4.1 Filesystem & config
XDG-style resolver:
- Config: `$XDG_CONFIG_HOME/conductor/config.toml` (Linux `~/.config/conductor/`, macOS `~/Library/Application Support/conductor/`).
- Data (SQLite DB, audit log), pinned rclone binary: platform data dir, created with restrictive permissions (`0700` dir, `0600` files).
- **Never** write into the app bundle or CWD.

---

## 5. Repo layout & conventions

```
conductor/
├── cmd/conductor/main.go     # composition root: build deps, wire, run. Thin.
├── app/                      # Wails binding layer (only place importing wails)
│   ├── app.go
│   ├── events.go             # typed event names + emit helpers
│   └── errors.go             # core error -> DTO mapping (uses §8.4 catalog)
├── shell/                    # Wails runtime behind our own interface (ADR-0001)
├── internal/
│   ├── core/
│   │   ├── domain/           # Remote/Job/Mount/Stats/Operation/SavedPair/Profile + validation
│   │   ├── daemon/           # rcd supervision service
│   │   ├── transfers/        # jobs + live stats service
│   │   ├── mounts/           # mount service
│   │   ├── options/          # rclone option catalog, flag builder, impact-rule engine
│   │   ├── history/          # operation history queries
│   │   ├── audit/            # append-only hash-chained audit log
│   │   ├── store/            # Store port + query helpers (no SQL dialect here)
│   │   └── ports/            # rc port, CommandRunner, Store, SecretStore, Clock...
│   └── adapters/
│       ├── rcclient/         # rc API client
│       ├── procrunner/       # os/exec supervisor impl
│       ├── sqlitestore/      # SQLite impl of Store (+ embedded migrations)
│       └── keyring/          # OS keyring impl of SecretStore
├── catalogs/                 # versioned rclone option catalog (embedded) — see §7.5
│   └── rclone@<version>.toml
├── migrations/               # forward-only SQL migrations (embedded)
├── frontend/
│   └── src/{lib/api,lib/stores,lib/components,routes}
├── testdata/rc/              # sanitized captured rc JSON fixtures (§2.10)
├── .golangci.yml
├── Taskfile.yml
├── docs/adr/
├── CONTRIBUTING.md
├── SECURITY.md               # vulnerability disclosure policy (§10)
├── LICENSE
├── NOTICE                    # third-party / bundled-binary license notices (§10)
├── CHANGELOG.md              # keepachangelog format, semver
└── README.md
```

Conventions:
- Package names are nouns, lower-case, no stutter (`rcclient.Client`, not `rcclient.RcClient`).
- `cmd/conductor/main.go` is the only composition root. Business logic there is a defect.
- Generated code is marked generated and reviewed when regenerated.

> **Shared-kit note:** the supervisor, slog setup, XDG resolver, error-DTO mapping, the `Store`/`SecretStore` ports, and the SQLite migration harness are generic. If a sibling Wails app (e.g. a ProjectDiscovery cockpit) already exists with these extracted into a shared module, depend on it; otherwise build them cleanly here and extract on second use, not first.

---

## 6. Architecture Decision Records

Commit these into `docs/adr/` as the opening commits.

### ADR-0001 — Wails v2 now, v3 as a contained migration
**Context.** v3 is the framework's future (GTK4 on Linux, better bindings) but is still alpha as of mid-2026, not a tagged stable release. v2 is stable and battle-tested.
**Decision.** Build on v2. Confine all Wails runtime calls (events, dialogs, lifecycle) behind a thin internal `shell` package depending on *our* interface, not Wails directly. v3 migration then touches `app/` and `shell/`, not the core.
**Consequences.** A little indirection; near-zero core churn when v3 stabilizes. Revisit when v3 cuts a stable tag.

### ADR-0002 — Svelte for the frontend
**Context.** A dense control surface (tables, live stats), not a content site. We want minimal webview runtime overhead and low ceremony.
**Decision.** Svelte + TS + Vite.
**Consequences.** Smaller bundle and simpler state than React. Reactivity fits live stat streams well.

### ADR-0003 — Drive `rclone rcd` over the rc API; do not shell out per command
**Context.** rclone exposes a full JSON control API via `rcd`. Per-invocation shelling loses stateful features (running jobs, live transfer stats, mount registry) and is slower.
**Decision.** Supervise a single long-lived `rclone rcd` (auth on, bound to loopback) and talk to it over the rc API. One-shot CLI calls only where no rc equivalent exists.
**Consequences.** We own daemon lifecycle (ADR-0005). We get jobs/stats/mounts cleanly. The rc client is the main adapter to test.

### ADR-0004 — Subprocesses spawned argv-style; never through a shell
**Context.** We spawn rclone with operator-influenced arguments (remotes, paths, flags). Shell interpolation is a command-injection vector; this tool must not have one.
**Decision.** `exec.CommandContext(ctx, bin, args...)` with explicit argument slices. No `sh -c`, no command-string concatenation, no operator input reaching a shell. The rclone binary path is resolved once to an absolute path and validated.
**Consequences.** Slightly more verbose construction; eliminates an entire vulnerability class. Enforced by `gosec` + review.

### ADR-0005 — Supervised daemon lifecycle
**Context.** An unmanaged `rcd` can orphan; rc surface and behaviour can drift across rclone releases.
**Decision.** Conductor owns the daemon's full lifecycle — start, health, restart-with-backoff, graceful-then-hard shutdown, reaping — bound to a pinned rclone version (integrity per ADR-0008).
**Consequences.** Reproducible behaviour; no orphaned daemons (integration-tested). Cost: explicit lifecycle code.

### ADR-0006 — No telemetry
**Decision.** Zero analytics, crash reporting, or network calls the user didn't initiate. Logs are local. Stated in the README as a feature.

### ADR-0007 — Embedded SQLite (pure-Go) for history, saved pairs, profiles, and the audit log
**Context.** Live transfer state is ephemeral and belongs to the daemon, but operation *history*, saved sync/bisync pairs, named option *profiles*, and the audit log must persist, be queryable ("what did I move to X last week"), and survive restarts. Cross-platform Wails builds strongly favour a CGO-free toolchain.
**Decision.** Use SQLite through the **pure-Go `modernc.org/sqlite` driver** — a single file in the platform data dir — with **versioned, embedded, forward-only migrations** run on startup behind a schema-version check. The core depends on a `Store` port; SQL lives only in the `sqlitestore` adapter. Live stats are **not** persisted; only completed-operation records and audit entries are.
**Consequences.** Rich relational queries and history without a server; CGO-free keeps cross-compilation trivial. Cost: pure-Go SQLite has no transparent at-rest encryption, so sensitive captured data is sealed at the application layer (ADR-0009). Alternatives (flat files, bbolt) rejected for a history/query workload.

### ADR-0008 — rclone binary pinned and integrity-verified; never silently downloaded
**Context.** Conductor executes the rclone binary with the operator's privileges. Its provenance is a supply-chain concern, and silent background downloads are unacceptable.
**Decision.** Pin an exact rclone version and resolve it from a configured location. At startup, verify its **SHA-256 against a committed, version-locked manifest** (and the upstream-published checksum where available) in addition to the `rclone version` parse. Acquisition is an **explicit, operator-initiated step**, never an automatic fetch. A distribution-bundled binary is verified the same way.
**Consequences.** A known, reproducible engine; tampering is detected before the daemon starts. Cost: a deliberate install/update step.

### ADR-0009 — Secrets in the OS keyring; sensitive persisted data encrypted at the application layer
**Context.** The rc session credentials are sensitive; captured job logs may contain paths or tokens-in-URLs; pure-Go SQLite (ADR-0007) offers no transparent encryption. Conductor does not store rclone remote credentials — `rclone.conf` owns those.
**Decision.** The **rc session user/pass is generated per session and held in memory only** (never written to disk or logged). On first run the app generates a random **per-install data key**, stores it in the **OS keyring** (macOS Keychain / Linux Secret Service), and uses it to seal sensitive persisted fields — captured job logs and any saved sensitive values — with an **AEAD (XChaCha20-Poly1305)** before they touch disk. The DB and data dir are OS-permission-restricted.
**Consequences.** Secrets are never on disk or in logs; captured data at rest is unreadable without the keyring entry. Cost: sealed fields aren't directly searchable; search runs over non-sensitive history columns (remote, kind, time).

### ADR-0010 — Operation capture + an append-only, hash-chained audit log
**Context.** A data-movement tool must be able to answer, durably and verifiably, what it did — especially for destructive operations.
**Decision.** Every operation's resolved parameters, full argv/rc-params, and captured rclone job log/stats are persisted (ADR-0007/0009). Every consequential action (operation start/stop, destructive-op confirmation and risk acknowledgement, mount/unmount, export) is recorded as an **append-only audit entry whose hash chains to the previous entry** (`hash = SHA256(prev_hash || canonical(entry))`). The chain is verifiable and exportable.
**Consequences.** A complete, tamper-evident record. Cost: storage volume, managed by retention + compression + encryption (§7.7–7.8).

### ADR-0011 — rclone options are a typed, data-driven catalog with impact warnings — not free-text flags
**Context.** rclone exposes a very large, evolving flag set, and some flags or combinations change the **risk to the data being moved** (delete-on-sync, `--no-check-dest`, `--ignore-existing`, bisync `--resync`) or the **load on the remote** (high `--transfers`/`--checkers`, no `--bwlimit`). Free-text flag strings are both an injection surface and a footgun.
**Decision.** A versioned **catalog** describes every exposed option as typed metadata (flag, value type, default, category, help text, risk level, `affects_data`, `conflicts_with`, `requires`, `impacts`). The UI is generated from it; a **flag builder** turns selections into validated rc parameters / argv (per ADR-0004, never a shell). An **impact-rule engine** evaluates the selection (plus operation kind and source/dest) and surfaces warnings, requires acknowledgement, or hard-blocks before execution.
**Consequences.** Flag selection is safe, explained, validated, and auditable; the catalog doubles as documentation. Cost: the catalog is maintained per pinned rclone version and tested against the binary's actual flags so it can't drift silently. Any "advanced raw flags" escape hatch is catalog-validated and flagged in the audit log.

### ADR-0012 — Signed/notarized macOS, reproducibly packaged Linux
**Context.** An unsigned utility that fights Gatekeeper, or ships as a mystery binary, won't be trusted or used. Credible distribution is part of the product.
**Decision.** macOS builds are **signed with a Developer ID and notarized** (hardened runtime, minimal entitlements, stapled). Linux ships as a versioned **AppImage** plus a **`.deb`**, built **reproducibly in CI** from pinned toolchains. Every release is a signed semver git tag with a maintained `CHANGELOG.md` and **published SHA-256 checksums**.
**Consequences.** Installs cleanly with verifiable provenance. Cost: a signing identity + notarization step. Updates are operator-initiated and integrity-checked — no silent auto-update.

### ADR-0013 — Bandwidth/concurrency governance with conservative defaults
**Context.** Unbounded transfers can saturate the operator's link and trip cloud-provider rate limits or bans (high concurrency on object stores).
**Decision.** Conductor exposes **bandwidth and concurrency governance** — `--bwlimit`, `--transfers`, `--checkers`, `--tpslimit` — with conservative defaults set globally, overridable per operation, and an optional **per-remote ceiling** (e.g. a rate-limited cloud remote). Limits are surfaced in the option builder and recorded with the operation.
**Consequences.** Safe-by-default behaviour; going faster is an explicit, recorded choice. Cost: slower default transfers.

---

## 7. Conductor — specification

### 7.1 Domain model (`internal/core/domain`)
- `Remote{ Name, Type, Config (redacted view) }`
- `Job{ ID, Kind (copy|sync|move|delete|purge|bisync|mount), Src, Dst, Status, Stats, StartedAt }`
- `TransferStats{ Bytes, TotalBytes, Speed, ETA, Errors, Transferring []FileProgress }`
- `Mount{ ID, Fs, MountPoint, Opts, Status }`
- `SavedPair{ ID, Name, Kind (sync|bisync), Path1, Path2, ProfileRef, LastRun }`
- `Profile{ ID, Name, Kind, OptionSelections }` — a named, reusable option set (§7.5)
- `Operation{ ID, Kind, Src, Dst, OptionSet, RcloneVersion, Intensity, StartedAt, EndedAt, BytesMoved, FilesMoved, Result, LogRef }` — the persisted history record (§7.7)
- `AuditEntry{ Seq, At, Action, Subject, Detail, PrevHash, Hash }` (§7.8)

Validation lives with these types: a `sync` requires confirmed source/dest; destructive ops require an explicit confirm flag in the call (§7.4).

### 7.2 rc API integration (`internal/adapters/rcclient`)
- Supervise one `rclone rcd --rc-addr 127.0.0.1:<ephemeral> --rc-user ... --rc-pass ...`, loopback-only, auth always on, credentials generated per session, held in memory, never written to disk or logged (ADR-0009).
- Typed methods map to rc endpoints: `core/stats`, `job/list`, `job/status`, `job/stop`, `sync/copy`, `sync/sync`, `sync/bisync`, `operations/*`, `mount/mount`, `mount/unmount`, `mount/listmounts`, `config/listremotes`, `config/get`, `operations/about`.
- **Polling:** rc is request/response; the core polls `core/stats` and `job/status` on a context-owned ticker (default 1s, configurable) and emits diffs as typed events. The poll loop stops on shutdown.
- Every rc response shape gets a typed Go struct and a fixture under `testdata/rc/`.

### 7.3 Feature set (v1)
1. **Remotes** — list, view (credentials redacted), test connectivity (`operations/about`). Create/edit deferred to v1.1 (config writing is a sharp edge; do it last).
2. **Transfers** — start copy/sync/move between remotes or local, with options chosen via the builder (§7.5); live per-file + aggregate progress; **stop/cancel** wired through `job/stop` + context.
3. **Mounts** — mount/unmount, list active mounts, surface mount health.
4. **Bisync** — configure and run pairs; show last-run summary; **dry-run is the default** for a new pair's first run.
5. **Live dashboard** — aggregate throughput, active job count, error feed.
6. **Operation history** — browse and query past operations with their parameters and results (§7.7, §7.11.7).
7. **Saved pairs & profiles** — reusable sync/bisync pairs and named option profiles (§7.5).

### 7.4 Destructive-operation safety (the central safety property)
- Any **destructive** operation (`sync` with deletion; `delete`; `purge`; bisync `--resync`) requires an explicit UI confirmation **and** a non-default boolean in the core call. The core refuses destructive ops without it — enforced by a unit test. There is no flag that disables this.
- **Dry-run is one click away** on every sync/bisync, and is the default for a new pair's first run.
- Source/dest are shown **resolved** (remote + path) before execution; no silent path interpolation.
- The impact engine (§7.5) classifies the operation and surfaces what will happen (e.g. "files at dest not present at source will be deleted") before the confirm step; destructive confirmations and risk acknowledgements are written to the audit log (§7.8).

### 7.5 Operation options & impact warnings (`internal/core/options`)
Implements ADR-0011 — the control that lets the operator choose rclone flags safely, with explanations and combination-impact alerts.
- **Catalog format** (`catalogs/rclone@<version>.toml`, embedded): each option = `{ flag, aliases[], type (bool|int|size|duration|enum|string|list), default, category, summary, description, risk (passive|mutating|destructive), affects_data (bool), conflicts_with[], requires[], impacts[] }`.
- **Flag builder:** takes a `Profile`/selection, validates types/`conflicts_with`/`requires`, clamps anything above a per-remote or global governance ceiling (§7.6), and assembles the validated rc parameters / argv (ADR-0004). It returns the exact effective command the UI previews; unknown flags cannot be injected.
- **Impact-rule engine:** declarative, pure, table-tested rules over `(selection, kind, src, dst)` producing `warn` / `require_ack` / `block`. Examples:
  - `sync` (default deletes at dest) → "sync makes the destination match the source; files at the destination not present at the source **will be deleted**. Run dry-run first." (`require_ack`)
  - `--delete-before` / `--delete-during` / `--delete-after`, `purge`, `delete` → destructive (`require_ack`, audited).
  - bisync `--resync` → "resync re-establishes the baseline and can overwrite both sides; back up first." (`require_ack`)
  - `--no-check-dest` / `--ignore-existing` / `--size-only` / `--no-traverse` → "weakens change detection; may skip or overwrite unexpectedly." (`warn`)
  - no `--bwlimit` set → "no bandwidth cap; may saturate your link." (`warn`); high `--transfers`/`--checkers` → "high concurrency may trip cloud-provider rate limits or bans." (`warn` / clamp to ceiling).
  - filters (`--include`/`--exclude`/`--filter`) → "verify the filter matches the intended set; preview before running." (`warn`)
- **Risk badges:** every option and the resolved operation show `passive` / `mutating` / `destructive`. Destructive selections require an explicit acknowledgement recorded in the audit log.
- **Profiles:** named option sets (e.g. "safe copy", "mirror with backup-dir"), persisted and reusable on operations and saved pairs.
- **Catalog/binary drift guard:** a tagged test parses the pinned rclone binary's flags and asserts the catalog references only real flags for that version, so an rclone upgrade that changes flags fails CI until the catalog is updated.

### 7.6 Transfer & bandwidth governance (`internal/core/transfers`)
Implements ADR-0013.
- Conservative global defaults for `--bwlimit`, `--transfers`, `--checkers`, `--tpslimit`, overridable per operation in the option builder.
- An optional **per-remote ceiling**: a remote can carry saved caps (e.g. a rate-limited object store) that the flag builder clamps to and cannot be exceeded without editing the remote's profile (audited).
- A **live intensity indicator** in the run view; the effective caps are recorded with the operation.

### 7.7 Operation history & persistence (`internal/core/store` + `history` + `sqlitestore`)
The SQLite schema (ADR-0007) holds everything durable. Tables (forward-only migrations in `migrations/`, embedded):
- `operations` — id, kind, src, dst, rclone_version, intensity (JSON), started_at, ended_at, bytes_moved, files_moved, result, log_blob_id.
- `operation_options` — operation_id, flag, value, risk, acknowledged.
- `saved_pairs` — id, name, kind, path1, path2, profile_id, last_run_at.
- `profiles` — id, name, kind; `profile_options` — profile_id, flag, value.
- `remote_ceilings` — remote, bwlimit, transfers, checkers, tpslimit.
- `log_blobs` — id, operation_id, nonce, sealed_bytes (AEAD), sha256_plaintext, bytes_len.
- `audit_log` — seq, at, action, subject, detail (JSON), prev_hash, hash (§7.8).
- `schema_migrations` — version, applied_at.

Rules: all writes go through transactions; captured logs are sealed before insert (ADR-0009); the `Store` port exposes intention-revealing queries (`OperationsByRemote`, `OperationsInRange`, `DestructiveOperations`, `LastRunForPair`) — the UI never composes SQL. Retention is configurable; an export ("what was moved") produces a CSV/JSON of operations for a range or remote. Backup is a documented file-copy of the data dir while the app is quiesced.

### 7.8 Operation capture & audit log (`internal/core/audit`)
- **Capture:** each operation's rclone job log/stats is pulled from the daemon on completion and stored (sealed) as a `log_blob` linked to the `operations` row — captured before it ages out of the live daemon.
- **Audit log:** append-only, hash-chained (`hash = SHA256(prev_hash || canonical(entry))`). Recorded actions include: operation start/stop, every **destructive-op confirmation and risk acknowledgement**, mount/unmount, governance-ceiling changes, and exports. The chain is verifiable on demand and surfaced in the UI (§7.11.8).
- The audit log is exportable as part of an operation-history export, giving a tamper-evident record of what the tool did.

### 7.9 Secrets & data-at-rest (`internal/adapters/keyring`)
Implements ADR-0009.
- rc session user/pass generated per session, in memory only, redacted from logs.
- Per-install data key generated on first run, stored in the OS keyring; sealed-field round-trip (seal/open) is unit-tested.
- Conductor never copies rclone remote credentials into its own store; it references remote names only.

### 7.10 Phased build plan (gates are commands)
- **P0 — skeleton + charter scaffolding.** Repo layout, `.golangci.yml`, Taskfile, CI (incl. `govulncheck`), slog, XDG config, ADRs, governance files committed. *Gate:* `task lint test build` green on an app that opens a window and logs startup; `govulncheck` clean.
- **P1 — persistence foundation.** `Store` port + `sqlitestore` adapter + embedded migrations; the append-only hash-chained audit log; keyring `SecretStore` + per-install data key + AEAD seal/open. *Gate:* migrations up + schema-version check against a real temp SQLite file; audit chain verify test (tamper detection); keyring round-trip (tagged); AEAD seal/open round-trip.
- **P2 — supervised `rcd` + rc client (read-only).** Start/stop daemon (pinned + checksum-verified, ADR-0008), auth, `core/stats` + `config/listremotes` + `job/list`, typed structs, fixtures, mock-based unit tests. Raw status view only. *Gate:* tagged integration test starts a real `rcd`, lists remotes, stops it, asserts no orphaned process.
- **P3 — option catalog + flag builder + impact rules + option UI.** Typed catalog for the pinned rclone, the validated flag builder, the impact-rule engine, generated option UI with inline help and risk badges (§7.5, §7.11.5). *Gate:* catalog validated against the pinned binary's actual flags (tagged); flag-builder tests incl. `conflicts_with`/`requires` and ceiling clamp; impact-rule tests (sync-deletes → require_ack; resync → require_ack; no-bwlimit → warn).
- **P4 — live dashboard.** Poll loop → typed events → store → throughput/jobs view. *Gate:* unit tests on the stats-diff emitter; demo of a running copy reflecting live.
- **P5 — transfers (copy/move) with cancel + capture + history.** Start jobs from the builder, live per-file progress, cancel via `job/stop`+context; persist the `Operation` with captured log; audit entry written. *Gate:* cancel propagates and process count returns to baseline; an operation row + sealed log + audit entry are written and queryable (test).
- **P6 — mounts.** *Gate:* mount/unmount round-trip integration test; audit entries for mount/unmount.
- **P7 — sync + bisync with destructive-op safety + governance.** Destructive confirms + acknowledgements, dry-run default for new pairs, saved pairs/profiles, governance ceilings. *Gate:* unit tests proving destructive ops are refused without explicit confirm, bisync defaults to dry-run on new pairs, and the governor clamps above-ceiling selections.
- **P8 — operation history & export view.** History browser, queries, "what was moved" export, audit-chain verification view (§7.11.7–7.11.8). *Gate:* query tests; export round-trip; audit chain verifies/export.
- **P9 — packaging, signing & release.** macOS sign+notarize, Linux AppImage + `.deb`, reproducible CI build, published checksums, semver tag + changelog (§9). *Gate:* CI produces a signed+notarized macOS artifact and Linux packages; checksums published; reproducible-build check matches across two runs.
- **P10 — remote create/edit (v1.1, optional).** Last, behind extra review (config writing is a sharp edge).

### 7.11 UI/UX specification

This app is a GUI; the front end carries half the value, so it is specified as seriously as the core. The governing principle: **the interface makes the consequential things visible and the resolved operation, not the raw flags, the thing the operator confirms.** Visual direction (type scale, spacing, color tokens) follows the `frontend-design` skill; this section specifies structure, state, and behavior.

#### 7.11.1 Layout & navigation
A primary nav with views: **Transfers** (set up and watch operations), **Remotes**, **Mounts**, **Bisync**, **History**, and **Audit**. The Transfers view is the live cockpit: a source/destination picker, the option builder, run controls, and a live progress surface. The window has defined minimum dimensions; below them, secondary panes collapse rather than crushing the progress table.

#### 7.11.2 Remotes
List remotes with type and a connectivity indicator (`operations/about`); detail view shows configuration with **credentials redacted**. Per-remote governance ceilings (§7.6) are edited here. Create/edit is deferred to v1.1 and clearly marked.

#### 7.11.3 Transfer setup & resolved-operation preview
- Source and destination are chosen as **remote + path** and shown **resolved** before anything runs — no silent interpolation.
- The operation's **risk badge** (`passive`/`mutating`/`destructive`) is shown prominently, derived from kind + options.
- A **live command preview** shows the exact effective rclone operation (rc params / argv) the run will execute — no hidden flags.

#### 7.11.4 Live transfer & run controls
- During a run: aggregate throughput, ETA, active transfers, and a **virtualized per-file progress list** (a large sync touches thousands of files; §8.5).
- **Start** becomes **Stop** while running — an always-visible, single-action cancel that drives `job/stop` + context cancellation. Stopped jobs are recorded as `cancelled`, not `failed`.
- The error feed shows per-file and operation-level errors in context, not as blocking modals.

#### 7.11.5 Option builder & impact warnings
- Options presented **by category** (transfer, checking, deletion, filters, performance, output), **searchable**, each row showing the flag, an input matched to its type, the **default**, and **inline help** (a one-line summary expandable to the full description). Nothing is a bare flag string the operator must remember.
- Each option carries a **risk badge**; deletion/overwrite options are visually distinct.
- The **impact panel** evaluates the current selection (plus kind, src, dst, and governance ceilings) and surfaces warnings, required acknowledgements, and clamps (§7.5). Destructive selections require an explicit acknowledgement before the run is allowed.
- **Profiles:** save/load named option sets; the active profile is recorded with the operation.

#### 7.11.6 Mounts & bisync
- Mounts: mount/unmount, list active mounts, surface health.
- Bisync: configure/run saved pairs; **dry-run is the default for a new pair's first run**, with the result summary shown before a live run is offered; `--resync` is gated behind an explicit acknowledgement.

#### 7.11.7 Operation history & data browser
- Browse past operations with kind, source/dest, time, bytes/files moved, result, and the exact options used.
- Queries the workflow needs: operations by remote; operations in a date range; destructive operations; the last run for a saved pair; reproduce an operation's exact option set into a new run (re-validated through the impact engine).
- **Export** ("what was moved") to CSV/JSON for a range or remote, including the audit trail.
- Retention controls and a "clear history" action that deletes rows + sealed logs.

#### 7.11.8 Evidence & audit view
- The **audit log viewer**: append-only entries with a visible **chain-verification indicator** (green = chain intact, red = tampering detected), filterable to destructive operations. Exportable with a history export.
- Per-operation captured log (decrypted on read) is viewable from the history detail.

#### 7.11.9 Empty, loading, error, and degraded states
Every view defines all four.
- **Empty:** Transfers reads "Pick a source and destination to begin"; History reads "No operations yet."
- **Loading/running:** live progress with a streaming indicator, not a blocking spinner.
- **Error:** typed, human-readable messages from the error DTO (§2.2/§8.4), shown in context — never a raw Go string.
- **Degraded (rclone missing/mismatched):** if the pinned binary is absent or fails checksum verification, operations are disabled with a clear remediation message at startup, not mid-run (ADR-0008).

#### 7.11.10 Front-end structure (maps to §2.8)
- Generated typed bindings only; the live stats stream arrives as typed events validated at the boundary.
- Stores split by concern: `remotes`, `options` (selection + impact + profiles), `run` (jobs/stats + lifecycle + intensity), `mounts`, `history` (queries + results), `audit` (entries + chain status). Runtime and view state are not commingled.
- Components are small and role-named: `RemoteList`, `RemoteDetail`, `SourceDestPicker`, `OptionBuilder`, `OptionRow`, `ImpactPanel`, `CommandPreview`, `TransferProgress`, `JobStatusBadge`, `MountList`, `BisyncPanel`, `OperationHistory`, `AuditLogView`. No monolithic `App.svelte`.

---

## 8. Security & threat model

A tool that moves and deletes data must reason about its own security explicitly. Individual mitigations live in the ADRs and spec and are cross-referenced.

### 8.1 Assets
The rc session credentials; the per-install data key (OS keyring); the rclone configuration Conductor reads (and must not leak); the integrity of the audit log; the integrity of the pinned rclone binary; the operator's data at source and destination.

### 8.2 Trust boundaries
The webview ↔ Go bridge (local IPC, not a network port); Go ↔ `rcd` over loopback HTTP (authed); Go ↔ rclone subprocess; `rcd` ↔ remote endpoints (egress the operator initiates); the app ↔ OS keyring and filesystem; the app ↔ the pinned rclone binary it executes.

### 8.3 Abuse cases & mitigations
| Threat | Mitigation |
|---|---|
| `rcd` reachable beyond the local app | Daemon bound to loopback on an ephemeral port, auth always on, per-session credentials in memory (§7.2, ADR-0009). |
| Command/parameter injection via path or flag | argv-only execution (ADR-0004); options validated against a typed catalog (ADR-0011); no shell anywhere (`gosec`-enforced). |
| Accidental destructive operation (wrong delete/sync) | Explicit typed confirm + impact acknowledgement; resolved src/dest shown; dry-run default for new pairs; no bypass (§7.4, ADR-0011). |
| Secret leakage via logs, DB, or export | rc creds in memory only; captured logs sealed with AEAD at rest; `redact()` at the logging boundary; OPSEC sanitization on export (ADR-0009, §2.4, §2.10). |
| Tampered or substituted rclone binary | SHA-256 verification against a committed manifest at startup; daemon refuses to start on mismatch (ADR-0008). |
| Audit-log tampering | Append-only, hash-chained entries; verification surfaced in the UI (ADR-0010, §7.8). |
| Overwhelming a remote / provider ban | Bandwidth/concurrency governance with conservative defaults and per-remote ceilings (ADR-0013, §7.6). |
| Vulnerable Go dependency | `govulncheck` in CI; pinned `go.sum` (§2.6). |
| Sensitive data committed to the repo | `testdata` sanitization rule + pre-commit/test scan (§2.10). |

### 8.4 Error-code catalog
The typed error DTO (§2.2) draws `code` from a single enumerated catalog (typed constants), e.g. `ERR_DAEMON_NOT_RUNNING`, `ERR_DAEMON_START`, `ERR_RCLONE_BINARY_MISSING`, `ERR_RCLONE_BINARY_CHECKSUM`, `ERR_RC_REQUEST`, `ERR_OPTION_CONFLICT`, `ERR_OPTION_OVER_CEILING`, `ERR_DESTRUCTIVE_NOT_CONFIRMED`, `ERR_JOB_CANCELLED`, `ERR_STORE_MIGRATION`, `ERR_SECRET_UNAVAILABLE`, `ERR_AUDIT_CHAIN_BROKEN`. Each maps to a stable UI message and a `retryable` flag. The frontend switches on `code`, never on message text.

### 8.5 Accessibility & performance targets
- **Accessibility:** keyboard operability for core flows; focus management on view changes; ARIA labels on controls; job/operation status conveyed by label/shape in addition to colour, with WCAG-AA contrast — meaning is never colour-only.
- **Performance:** the per-file progress list and history table are virtualized and must stay responsive at the scale a real sync produces — target ≥10,000 file rows without UI stall; live stat updates are batched/coalesced so a fast transfer doesn't thrash rendering.

---

## 9. Distribution, signing & release

Implements ADR-0012; credible distribution is part of the product — the same lesson that makes a small open-source utility trustworthy.
- **macOS:** sign with a Developer ID, hardened runtime with minimal entitlements, notarize and staple. Installs and launches without Gatekeeper friction.
- **Linux:** ship a versioned **AppImage** and a **`.deb`** with a desktop entry. Note the system-tray caveat (freedesktop `StatusNotifierItem`; GNOME needs an extension) if a tray presence is added.
- **Bundled vs. fetched rclone (ADR-0008):** state clearly how the pinned rclone is provided (bundled or installed via a documented task) and verify it by checksum on every launch regardless.
- **Reproducible builds:** build in CI from pinned Go and frontend toolchains; a reproducibility check compares artifacts across two runs. Optionally publish an SBOM.
- **Releases:** signed semver git tags, `CHANGELOG.md` (keepachangelog), and **published SHA-256 checksums** for every artifact.
- **Updates:** operator-initiated and integrity-checked; no silent auto-update (consistent with ADR-0006/0008).

---

## 10. Legal & project governance

> This section is engineering guidance, not legal advice; confirm licensing specifics for the actual jurisdiction.

- **Licensing.** Choose a license for the app — **Apache-2.0** is recommended (permissive, explicit patent grant); MIT is the lighter alternative. Independently, **bundling/distributing the rclone binary means complying with its license** (rclone is MIT; verify at the pinned version) — ship a `NOTICE`/`THIRD-PARTY-LICENSES` file with its notice. The app's license does not override the bundled binary's license.
- **Vulnerability disclosure.** `SECURITY.md` defines how to report a vulnerability *in Conductor itself*: contact channel, scope, and response expectations.
- **Governance files** (also in §5): `LICENSE`, `NOTICE`, `SECURITY.md`, `CONTRIBUTING.md` (§11), `CHANGELOG.md`, `README.md` (what it is, verified install steps, quick start, the no-telemetry statement), plus issue/PR templates and optionally `CODEOWNERS`.

---

## 11. `CONTRIBUTING.md` template (commit to the repo)

The operating rules live in a standard `CONTRIBUTING.md` so the repository reads as a normally-governed project. Anyone (or anything) implementing against it follows the same standards; tooling that wants these rules loaded can be pointed at this file and `PROJECT-BOOK.md` directly.

```markdown
# Contributing to Conductor

`PROJECT-BOOK.md` is the specification and these standards are binding. Work in
the phases defined in PROJECT-BOOK.md §7.10, in order; a phase is not started
until the previous phase's gate is green.

## Before writing code
- Read PROJECT-BOOK.md §2 (Engineering Charter) and §7 (specification).
- Destructive-operation safety (§7.4) has NO bypass. Do not add one.

## Every change
- context.Context is the first parameter of any I/O / process / network function.
- Errors wrapped with %w + context; typed sentinels for branched-on errors
  (e.g. ErrDaemonNotRunning); error codes from the §8.4 catalog; no string
  matching on error text; no panic outside main/tests.
- Structured logging via slog; secrets are never logged (use redact()). The
  operational log is not the audit log.
- No global mutable state; dependencies constructed and injected in
  cmd/conductor/main.go.
- Subprocesses spawned with exec.CommandContext and argv slices. Never a shell.
- rclone flags come from the catalog and the flag builder; no free-text flags.
- Captured logs / sensitive values are sealed before disk; the data key lives in
  the keyring; rclone remote credentials are never copied into our store.
- No wails import outside app/ and shell/ (enforced by depguard).
- No authorship/tooling fingerprints in any file, commit, or metadata.

## Definition of a finished phase
Run and record the output of:
    task lint               # gofumpt + golangci-lint + go vet + govulncheck
    task test               # go test ./... green on a bare machine (no rclone)
    task test:integration   # tagged; runs against a real rclone/keyring if present
    task build              # runnable app for darwin + linux
A phase is not done until its gate (PROJECT-BOOK.md §7.10) passes. "It runs" is
not the gate.

## Not acceptable
- Placeholder packages, stub tests that assert true, or empty TODOs.
- Business logic in app/ or main.go; SQL composed outside the store adapter.
- A dependency added without pinning it and recording why.
- Letting a destructive rclone op run without an explicit confirm + acknowledgement.
- Real data (remotes, paths, credentials) committed anywhere, incl. testdata.

## When the book is wrong
Open the disagreement as a proposed ADR and amend the book. Do not diverge silently.
```

---

## 12. Definition of Done — review checklist

A change/phase is done only when **all** hold. Maps 1:1 to §2 and the security model.

**Boundaries**
- [ ] No `wails` import outside `app/` and `shell/` (depguard green).
- [ ] `go test ./internal/core/...` passes with no rclone, no webview.
- [ ] Domain core has no knowledge of rc wire formats, SQL, or the keyring.

**Errors & lifecycle**
- [ ] No `panic` outside `main`/tests. Errors wrapped with `%w` + context.
- [ ] Branched-on errors are typed; error codes from the §8.4 catalog; no `err.Error()` string matching.
- [ ] `context` first param on all I/O; cancellation reaches the request/process.
- [ ] No goroutine without a defined owner/exit. No orphaned `rcd` after quit (integration-tested).
- [ ] No package-level mutable state.

**Destructive-op safety (the central safety property)**
- [ ] Destructive ops refused without explicit confirm; no bypass path exists.
- [ ] Impact engine classifies the operation and requires acknowledgement for destructive selections.
- [ ] Resolved src/dest shown before execution; bisync defaults to dry-run on new pairs.

**Options & impact**
- [ ] rclone flags come from the catalog + flag builder; rc params/argv assembled, never a shell.
- [ ] Catalog references only real flags for the pinned rclone version (drift test).
- [ ] Impact rules tested: sync-deletes → require_ack; resync → require_ack; over-ceiling → clamp/warn.
- [ ] Any raw-flags escape hatch is catalog-validated and audited.

**Data, history & audit**
- [ ] Migrations run forward against a real SQLite file; schema-version checked.
- [ ] Captured logs / sensitive values sealed with AEAD before insert; data key only in the keyring (round-trip tested).
- [ ] Each operation persists its params + result; full rc params/argv recorded in the audit log.
- [ ] Audit log is append-only and hash-chained; tamper detection tested.
- [ ] rclone remote credentials are never written to our store.

**Tests**
- [ ] Tests assert behavior; no coverage theater.
- [ ] rc mappers tested table-driven against sanitized fixtures.
- [ ] Integration tests build-tagged; skip cleanly when rclone/keyring absent.

**Observability & security**
- [ ] slog only; no stray `fmt.Println`/`log.Printf`; operational log distinct from audit log.
- [ ] A redaction test proves rc credentials never appear in logs.
- [ ] All subprocesses spawned argv-style (gosec green); no shell.
- [ ] `govulncheck` clean; rclone binary pinned + checksum-verified.
- [ ] Threat-model mitigations for touched areas (§8.3) hold.

**Distribution**
- [ ] Release artifacts: macOS signed+notarized, Linux AppImage+.deb, checksums published, reproducible-build check passes.

**Tooling & hygiene**
- [ ] `gofumpt`, `golangci-lint` (curated), `go vet`, `tsc --noEmit` clean in CI.
- [ ] Every exported symbol has a godoc comment.
- [ ] No commented-out code; no bare TODO/FIXME; no authorship/tooling fingerprints.
- [ ] Conventional Commits; every commit builds.
- [ ] rclone version pinned + checksum-verified; relevant ADR written/updated.

**Frontend**
- [ ] Frontend calls generated typed bindings only; event payloads typed/validated.
- [ ] State in defined stores (§7.11.10); components small/role-named; no unjustified `any`.
- [ ] Option builder shows inline help + risk badges + live command preview + impact panel.
- [ ] A run shows live progress and an always-visible **Stop**; stopped jobs are `cancelled`, not `failed`.
- [ ] Resolved src/dest shown; destructive operations clearly flagged and acknowledged.
- [ ] Job/operation status conveyed with non-colour cue + WCAG-AA contrast; progress/history tables virtualized.
- [ ] History and Audit views exist; audit view shows chain-verification status.
- [ ] Every view defines empty / loading / error / degraded states; errors render from the typed DTO.

---

## 13. Getting started
1. Execute **P0**; record the green gate.
2. Write ADR-0001..0013 into `docs/adr/` and `CONTRIBUTING.md` (from §11) as the opening commits, alongside `LICENSE`, `NOTICE`, `SECURITY.md`, `CHANGELOG.md`, and `README.md`.
3. Build the **persistence foundation (P1)** before anything that persists operations or secrets.
4. Proceed phase by phase. Do not skip gates.
