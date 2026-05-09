# Changelog

All notable changes to forge will be documented in this file. Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/); the project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- **`forge postmortem [path]`** (verb #13, DEV-M3-05) — lints post-mortem documents in `docs/postmortems/INC-*.md` per ADR-020. Checks: all 8 mandatory sections present, ≥1 action item in canonical shape (`- [ ] AI-NN — … — owner: @… — due: YYYY-MM-DD — issue: #NNN`), ≥1 action item references a failure-register entry (`register: FR-NNN`) or a commit SHA. `--json` emits `[]FileReport` for dashboards. Exit non-zero for CI gate.
- **`forge stats`** (verb #14, DEV-M3-02) — local telemetry rollup from `.forge/audit.log`. Aggregates per-verb event counts with action breakdown and last-seen timestamps. `--since YYYY-MM-DD` filter. `--json` emits `Report`. No remote calls.
- **`forge audit failure-register <verify|list|lint>`** (DEV-M3-04) — manages the ADR-016 failure register at `.forge/failure-register.json`. `lint` validates schema; `list --json` dumps active entries; `verify` detects drift (entries missing `test_anchor`). Exit non-zero on drift (FORGE-3702).
- **`internal/failure`** package — ADR-016 failure-register data model. `Register`, `Entry`, `Status`, `Severity` types. JSON persistence (`Load`/`Save`/`LoadDefault`). `Validate()` detects duplicates, unknown status, missing required fields. `Active()` filters out retired entries.
- **`internal/plugin/discovery.go`** (DEV-M2-06) — `.forge/plugins.json` discovery. `DiscoverFile` reads a JSON array of `Manifest` objects and registers them as `ExternalPlugin` stubs (callable body deferred to DEV-M2-05 WASM runtime). Built-in names take precedence. `Discover(root)` wired into `root.go`'s `PersistentPreRunE` so external plugins appear in `forge plugin list`.
- Reserved error-code ranges: `cli/failure-register` (3700..3799), `cli/postmortem` (3800..3899), `cli/stats` (3900..3999).
- Tests: 8 for `internal/plugin` discovery (happy, missing file no-op, bad JSON, invalid manifest, built-in precedence, idempotency, manifest round-trip, `Discover` path contract); 18 for `internal/failure` (entry validation, register validation, `Active()` filter, save/load round-trip, idempotency, JSON keys, `LoadDefault` path contract); 11 for `cmdpostmortem` (happy, missing sections, no action item, no register ref, commit-ref satisfies, absent dir, non-INC ignored, find-all sorted, JSON, CI gate, idempotency); 10 for `cmdstats` (count aggregate, sort, empty, `--since` filter, false-positive guard, JSON, text, empty ledger, invalid `--since`, idempotency); 5 for `cmdaudit` failure-register integration (lint OK, list JSON, verify drift FORGE-3702, verify OK, empty list).
- Generated `docs/ERROR_CODES.md` now lists 30 codes (was 23).

### Changed
- README: bumped to "14 verbs"; added `forge stats` and `forge postmortem` rows.
- `cmd/gen-errors`: added side-effect imports for `cmdpostmortem` and `cmdstats`.


## [0.2.0-m2-preview] — plugin loader, codemod runner, audit ledger

Pulls forward the **M1 expansion + M2 scaffolding + M3 spike** in one preview cut. Adds 2 new top-level verbs (10 total) plus three new scanner families, an in-process plugin registry, a tamper-evident action ledger, and the first NFR benchmarks.

### Added

- **`forge upgrade <codemod>` [`--apply`] [`--root`] [`--json`]** ⭐ **(M2 codemod runner)** — deterministic, idempotent transformations with dry-run as default. Built-in codemods:
  - `gitignore-marker` — insert/refresh the `# forge:gitignore:start/end` marker block.
  - `gitleaks-baseline` — drop `.gitleaks.toml` baseline rules if missing.
  - `forge upgrade list [--json]` enumerates all registered codemods.
- **`forge audit <show|verify|append>` [`--root`] [`--json`]** ⭐ **(M2 audit ledger)** — append-only, hash-chained log at `.forge/audit.log` with tamper-evident `sha256(prev_hash + entry)` linking. `verify` walks the chain; `show` lists entries; `append --verb X --action Y` adds a record.
- **`forge scan` (expanded)** — three new scanner families plus `all`:
  - `forge scan rls` — flags `CREATE TABLE`/`SELECT` SQL without `tenant`/`workspace` columns/predicates.
  - `forge scan prompt-injection` — detects `ignore previous`, role-override, system-prompt-leak, unsafe `eval` patterns in prompts/code.
  - `forge scan supply-chain` — flags loose version ranges, unpinned git URLs, `curl … | sh` pipes, `go.mod replace` directives.
  - `forge scan all` — runs every family and merges results.
  - `forge scan secrets` — real built-in regex engine (5 rules: AWS access key, OpenAI sk-*, GitHub tokens, PEM private-key block, generic Bearer) when gitleaks is unavailable.
- **`internal/plugin`** — plugin contract + in-process `Registry` (M2 scaffold for the wazero ABI gated behind `forge_wasm` build tag). `Manifest`, `Plugin`, `Scanner`, `Codemod`, `Finding`, `Result` types; thread-safe `Default()` registry; sorted `All()` and `ByKind()` views.
- **`internal/audit`** — generic hash-chained ledger (`Open`, `Append`, `All`, `Verify`). 25-goroutine concurrency test confirms chain stays valid under contention.
- **`internal/codemod`** — codemod contract + registry; ships `gitignore-marker` and `gitleaks-baseline` built-ins.
- **`cmd/gen-errors`** — generates `docs/ERROR_CODES.md` from the live `errcode` registry. `--check` mode for CI drift detection.
- **`docs/ERROR_CODES.md`** — auto-generated catalogue of every `FORGE-XXXX` code (now 18).
- **NFR benchmarks** — `BenchmarkScanSecrets_500Files` and `BenchmarkScaffold_GoService` (NFR §16.4 budgets: scan ≤2s/1k files, scaffold ≤1s/op).

### Changed

- `errcode` reserved-range table now spans `3300–3399` (`cli/upgrade`) and `3400–3499` (`cli/audit`).
- Root command wires 10 verbs (was 8); `internal/cli` `TestRootCommand_VerbsRegistered` expanded to match.
- `cmdscan` coverage 64% → 75%; new `internal/plugin` 92%, `internal/audit` 85%, `internal/codemod` 78%, `internal/cli/cmdupgrade` 87%, `internal/cli/cmdaudit` 70%.

### Deferred to M2.x / M3+

- wazero WASM plugin runtime (`forge_wasm` build tag) — interface in place; runtime ships in M2.2.
- `forge eval` scenario harness — design spike pending (ADR-005 finalised).
- Wire `forge scan` families through `plugin.Registry` (currently hard-coded; trivial follow-up).
- Spec-Lock, LLM gateway, governance, full ship workflow.

## [0.1.0-mvp] — community-launch preview

The first runnable slice of forge with **8 working verbs** (5 core M0 + 3 M1 security/hygiene/preview). Goal: contributors can clone, build, and scaffold a working Go service in under a minute, plus scan secrets, verify hygiene, and preview the ship workflow.

### Added

- **`forge version` [`--json`]** — prints version + Go/OS/arch.
- **`forge new <template> <path>`** — embedded template renderer. Ships one template (`go-service`) with:
  - HTTP server with `/healthz`, `/readyz`, graceful shutdown via `signal.NotifyContext`.
  - Healthz `httptest` regression test.
  - Managed `.gitignore` with `forge:gitignore:start/end` marker block.
  - Baseline `.gitleaks.toml` (generic-api-key, private-key-block, OpenAI sk-*, AWS AKIA*).
  - `.forge/manifest` baseline (scratch + managed sections).
  - Flags: `--name`, `--module`, `--force`, `--json`.
- **`forge doctor` [`--json`]** — env health checks (git, go, temp-dir writable). Non-zero exit on required-check failure.
- **`forge clean` [`--check`|`--apply`] [`--root`] [`--json`]** — manifest-driven LLM-scratch sweeper. `--check` is the default and exits non-zero when candidates exist (CI-gateable).
- **`forge explain [verb]` [`--json`]** — verb-manifest browser. With no arg lists all registered verbs; with one arg prints inputs / outputs / side-effects / gates touched / error codes.
- **`forge scan secrets` [`--root`] [`--json`]** ⭐ **(M1 headline)** — secret scanner (attempts gitleaks; fallback to built-in patterns). Outputs findings with file/line/rule/match. Exit non-zero on findings.
- **`forge lint` [`--root`] [`--json`]** ⭐ **(M1 hygiene)** — hygiene checker (manifest presence, .gitignore markers, .gitleaks.toml baseline). Outputs structured {file, level, code, message}. Error exit if any check fails.
- **`forge ship [--dry-run]` [`--json`]** ⭐ **(M1 preview)** — validates 5-checkpoint pipeline without executing (spec, test, breakdown, code, hygiene). MVP: hygiene checkpoint only. Exit non-zero if any checkpoint fails.
- **`internal/errcode`** — `FORGE-XXXX` registry with reserved code ranges (1000s = CLI verbs, 2000s = config/fs/scaffold/manifest, 3000s = scan/lint/ship, 9000s = test). Panics on duplicate or out-of-range registration.
- **`internal/logobs`** — slog wrapper. Auto / JSON / text formatter, secret-key redaction (`secret_*`, `token_*`, `api_key*`, `password`, `token`, `secret`), `Explain=true` opt-in to bypass redaction (for `forge explain`-class verbs).
- **`internal/verbmeta`** — verb manifest registry powering `forge explain`.
- **`internal/manifest`** — `.forge/manifest` text-format reader. Sections: `[scratch]`, `[managed]`. Glob matcher supports `**`, `*`, `?`. **Managed wins over scratch** to prevent false-positive deletions.
- **`internal/scaffold`** — `embed.FS`-backed template renderer (`all:` glob to include dotfiles), `text/template` substitution with `missingkey=error`, `__name__` path interpolation, force-overwrite gate.

### Test coverage

All 14 packages with unit tests covering the [9-point design checklist](https://github.com/teragrid/forge/blob/main/CONTRIBUTING.md): happy / boundary / negative / idempotency / cross-tenant (where applicable) / regression / data-accuracy / false-positive guard.

| Package | Status | Package | Status |
|---------|--------|---------|--------|
| `internal/cli` | ✅ | `internal/cli/cmdscan` | ✅ |
| `internal/cli/cmdclean` | ✅ | `internal/cli/cmdlint` | ✅ |
| `internal/cli/cmddoctor` | ✅ | `internal/cli/cmdship` | ✅ |
| `internal/cli/cmdexplain` | ✅ | `internal/cli/cmdversion` | ✅ |
| `internal/cli/cmdnew` | ✅ | `internal/errcode` | ✅ |
| `internal/logobs` | ✅ | `internal/manifest` | ✅ |
| `internal/scaffold` | ✅ | `internal/verbmeta` | ✅ |

### Pre-push quality gates

All commits pass a 7-stage gate (installed via `git config core.hooksPath .githooks`):

1. `goimports` — import formatting
2. `gofmt -s` — code formatting
3. `go vet` — correctness checks
4. `golangci-lint` — static analysis (staticcheck, gocritic, gosec, errcheck, ineffassign, govet)
5. `go build` — compilation (CGO_ENABLED=0 for cross-platform)
6. `go test -count=1` — all unit tests
7. `govulncheck` — no known CVEs
8. `go mod verify` — module integrity

### Deferred to M1+ releases

Plugin runtime (wazero ABI), audit ledger, LLM gateway, Spec-Lock, governance, telemetry, full ship workflow (spec validation, test orchestration, breakdown composition, code generation). See [DEVELOPMENT_PLAN.md](docs/DEVELOPMENT_PLAN.md) for roadmap.

[Unreleased]: https://github.com/teragrid/forge/compare/v0.2.0-m2-preview...HEAD
[0.2.0-m2-preview]: https://github.com/teragrid/forge/releases/tag/v0.2.0-m2-preview
[0.1.0-mvp]: https://github.com/teragrid/forge/releases/tag/v0.1.0-mvp
