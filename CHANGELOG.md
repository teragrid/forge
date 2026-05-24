# Changelog

All notable changes to forge will be documented in this file. Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/); the project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.1.7] — 2026-05-24

### Added

- **`forge test spec <feature>`** — writes a structured YAML test spec to `.forge/specs/<feature>/spec.yml` covering all 9 test-design categories: happy path, boundary, negative, idempotency/replay, concurrency/race, cross-tenant/authz, regression, data-accuracy, and false-positive guard. Use `--dry-run` to preview without writing.
- **`forge test run --spec <path>`** — executes (or dry-runs) the test families declared in a spec file. Use `--feature <name>` as a shorthand to locate `.forge/specs/<name>/spec.yml` automatically.
- **`forge config set <key> <value>`** — persists defaults to `forge.yml`. Valid keys: `llm.provider`, `llm.model`, `llm.daily_budget_usd`, `llm.monthly_budget_usd`, `telemetry.enabled`, `telemetry.install_id`, `log.format`, `log.level`. Re-running is idempotent and does not clobber unrelated keys.
- **`--budget-usd <float>` global flag** — per-invocation spend cap passed as `FORGE_BUDGET_USD`; complements the persisted `llm.daily_budget_usd` config key.
- **`--profile` flag end-to-end** — validates `fast | safe | paranoid` in `PersistentPreRunE` and applies the profile's `MaxLLMTokenBudget` to every LLM call via a `profileProvider` wrapper in `internal/llmprovider`.
- **GitHub Copilot LLM provider** — auto-detected from `GH_TOKEN` or `gh` CLI config. `Capabilities()` fetches the live model list from `GET /models` (cached via `sync.Once`) and falls back to a curated known-models list when the endpoint is unreachable.

### Changed

- **`forge bugfix` real-world improvements** — new `--stack`, `--file` (repeatable), `--context`, and `--model` flags; `--bug -` reads the bug description from stdin; `applyPatch` saves patches to `.forge/patches/<ts>-<file>.patch` and applies them via `git apply`; `MaxTokens` no longer hardcoded — governed by active `--profile`.
- **`forge explain` UX** — verbs are now grouped into 10 logical categories with next-step hints; the `--format json` flag works end-to-end.
- **`forge.yml llm.model` → `FORGE_COPILOT_MODEL` bridge** — `PersistentPreRunE` in `root.go` reads the persisted model and sets the env var automatically, so `forge config set llm.model gpt-4o` takes effect without any shell-profile change.

## [1.1.6] — 2026-05-24

### Added

- **`forge bugfix`** — new verb for the post-delivery bug fix workflow. Accepts bugs from three sources: `--bug "<description>"` (plain-language report), `--finding <id>` (review finding ID from `forge review`), or `--test "<pattern>"` (failing test name). With an LLM configured, diagnoses the root cause, writes a surgical patch, and generates a regression test to prevent recurrence. Dry-run by default; `--apply` writes the patch and test to disk and records in `.forge/audit.log`. Error range `FORGE-6550..6599`.
- **Strengthened LLM prompt templates** — all seven verb prompts (`ask`, `review`, `fix`, `scan`, `ship`, `optimize`, `learn`) now use directive, imperative language ("hunt the bug to its root cause", "fix it once and for all", "leave nothing unchecked") for more thorough and direct LLM responses.

## [1.1.5] — 2026-05-23

### Changed

- **`forge init` always injects baseline files** — every `forge init` invocation (with any template or `--minimal`) now automatically runs four codemods after scaffolding: injects a `# forge:gitignore:start … # forge:gitignore:end` marker block into `.gitignore` (user content outside the block is preserved), and creates `.gitleaks.toml`, `.pre-commit-config.yaml`, and `.github/dependabot.yml` if they are absent. No new flag required — this is the safe default. Re-running `forge init` is idempotent; the `.gitignore` block is never duplicated.
- **`--force` now covers managed-block drift** — `--force` additionally overwrites any forge-managed blocks in `.gitignore` that have drifted from the canonical forge template. The removed `--merge` flag is superseded by this default behaviour.

## [1.1.4] — 2026-05-23

### Added

- **`forge init --minimal`** — lightweight init for existing projects. Injects forge knowledge (`.forge/` config files, `AGENTS.md`, hygiene rules, conventions) without touching `go.mod`, `package.json`, CI files, or any other project structure. Project name is auto-detected from the current directory — no `--name` flag required. `cd ai-marketing-platform && forge init --minimal` just works.
- **`forge ship` feature-branch workflow** — when `forge ship <feature>` is run from a protected branch (`main`, `master`, `develop`, `dev`, `trunk`, `production`, `prod`), Forge automatically creates and checks out `feature/<slug>` before the pipeline starts. After all six checkpoints pass, Forge prints the exact commands to push the branch and open a pull request. Use `--no-branch` to skip this behaviour and stay on the current branch.

### Removed

- **VS Code extension** (`packages/vscode-forge/`) — extracted to its own repository. The `.github/workflows/vscode-publish.yml` workflow and `.forge/specs/vscode-forge-extension/` spec directory have been removed from this repo.

### Fixed

- **Dependabot PR sprawl** — grouped all GitHub Actions updates into one PR and all Go module updates into one PR, replacing the prior per-dependency PR behaviour that flooded the Actions queue.

## [1.1.3] \u2014 2026-05-22

### Added

- **`forge ship arch` checkpoint** \u2014 a new checkpoint 2 (`arch`) inserted between `spec` and `test` makes the pipeline 6 stages: `spec \u2192 arch \u2192 test \u2192 breakdown \u2192 code \u2192 ship`. The arch checkpoint generates both `arch.md` and `openapi.yaml` under `.forge/specs/<slug>/` via a KB-enriched LLM call.
- **KB injection in ship pipeline** (`InvokeWithKnowledge`, ADR-026) \u2014 all four LLM-backed checkpoints (`arch`, `test`, `breakdown`, `code`) now prepend the top-5 relevant knowledge-base entries to the system prompt automatically. Add project-specific guidance to `.forge/knowledge/` to influence all generated artifacts.
- **Supabase RPC auto-detection** (`detectAPIStyle`) \u2014 after `arch` generates `openapi.yaml`, Forge reads path prefixes: if `/rest/v1/rpc/` paths are present the feature is flagged as `supabase-rpc`, and all downstream checkpoints inject targeted guidance (PostgreSQL function creation, `GRANT EXECUTE`, RLS policies, `.rpc()` TypeScript client calls, integration tests). Standard REST features are unaffected.
- **`RoleAPIDesign` Supabase concern** \u2014 the six-role self-debate engine (ADR-025) now has a dedicated concern for undeclared API style, with actionable guidance to choose between `/rest/v1/rpc/{fn}` and `/api/v1/{resource}`.
- **DAB Full template updated** (`docs/adr/dab-full/`) \u2014 sections 02, 03, 06, and 09 reflect the new arch checkpoint, KB injection note, API style declaration, and Supabase RPC governance rules.
- **DAB Light template updated** (`docs/adr/dab-light/`) \u2014 same updates in condensed form.
- **`docs/verbs/ship.md` updated** \u2014 synopsis, checkpoint list, and examples now document the full 6-stage pipeline including the `arch` checkpoint, KB injection callout, and Supabase RPC detection.

### Changed

- README: "5-stage quality gate" \u2192 "6-stage quality gate"; Arch stage added to the pipeline table; KB description updated to mention ship-checkpoint injection.
- `GETTING_STARTED.md`: Step 5 updated to 6-stage table with Arch as stage 2.

## [1.0.0] — 2026-05-16 — All 82 gap tasks complete

This release closes every item in the spec\u2013implementation gap list. All packages pass `go test ./... -count=1`; all golangci-lint checks pass.

### Added

- **Semantic LLM cache** (`internal/llmcache`) — token-based Jaccard similarity (threshold 0.85) deduplicates repeat LLM calls without CGO or vector databases. Fixed punctuation-stripping bug so trailing `.`/`,` no longer prevents cache hits.
- **Tier-router cascade** (`internal/tierrouter`) — exact-hit → semantic-cache → remote-LLM cascade with configurable fallback policy.
- **Streaming LLM adapter** (`internal/llmprovider/adapter.go`) — `StreamUntilComplete` with early-stop on sentinel tokens; `BatchComplete` for parallel inference.
- **Token-budget YAML config** (`internal/contextbudgeter`) — per-verb token limits in `.forge/budget.yml`; `LoadBudgetConfig` + integration test.
- **Six-role self-debate** (`forge optimize`, `docs/rfcs/ADR-025-six-role-self-debate.md`) — Architect / Devil\u2019s Advocate / Security / QA / Performance / Product roles debate specs before shipping.
- **Third-party scanner plugins** (`tests/fixtures/scan-plugin/`) — full scanner-family contract; `TestThirdPartyPlugin_RegistersInScanFamily` integration test.
- **forge learn share** (`internal/cli/cmdlearn/learn_extended.go`) — opt-in/out of anonymized convention-count sharing via `forge.yaml`; `forge learn promote` promotes a validated spec.
- **forge generate test --from-bug** (`internal/cli/cmdgenerate/generate.go`) — generates regression tests from an incident/bug record.
- **forge audit erase** (`internal/cli/cmdaudit/audit.go`) — GDPR right-to-erasure: removes all ledger entries for a subject.
- **forge rollback --advise** (`internal/cli/cmddeploy/deploy.go`) — correlates deploy history with SLO regression and recommends a minimal revert target.
- **Incident auto-triage** (`internal/cli/cmdincident/incident.go`) — `forge incident triage <id>` LLM-assisted root-cause classification.
- **Doctor drift detector** (`internal/cli/cmddoctor/doctor.go`) — detects schema and convention drift between runs; added to `forge doctor` health check.
- **Pre-commit hook gate** (`scripts/forge-pre-commit`) — runs `forge scan security` + `forge lint` on staged files; rejects commits with critical findings.
- **CI cost gate** (`.github/workflows/ci-gates.yml`, `eval-cost-gate` job) — fails CI when `forge eval` total LLM spend exceeds configured threshold.
- **Auto-generate PR body** (`internal/cli/cmdship/pr.go`) — `forge ship --pr` populates the GitHub PR description from `spec.md` + `tasks.md`.
- **forge context privacy** (`internal/cli/cmdcontext/privacy.go`) — PII redaction for context snapshots; `--redact` flag.
- **forge insights cli** (`internal/cli/cmdinsights/cli_insights.go`) — unused-verb detection, common misspellings, schema drift analysis.
- **forge insights hygiene** (`internal/cli/cmdinsights/hygiene_digest.go`) — weekly hygiene digest: un-manifested patterns, stale artefacts, per-contributor debt.
- **Canonical project fixture** (`tests/fixtures/canonical-project/`) — representative Go project for all 9 scanner families; `TestAllScannerFamilies_CanonicalProject`.
- **Hygiene manifest schema + drift detection** (`internal/cli/cmdhygiene/hygiene_extended.go`) — `TestHygieneDriftDetection` validates schema round-trip.
- **forge docs heal** (`internal/cli/cmddocs/docs.go`) — `newHealCmd` repairs stale doc cross-references.
- **Capability registry** (`internal/capability/`) — `Define`/`Register`/`Execute`/`List` API for LLM-accessible tools.
- **Prompt compiler** (`internal/promptcompiler/`) — template compilation with variable injection and safety validation.
- **Outbox pattern** (`internal/outbox/`) — durable `Event` records written before mutations; idempotency key deduplication.
- **Guardrails** (`internal/guardrails/`) — policy-based output filtering for LLM responses.
- **Healer** (`internal/healer/`) — automated remediation suggestions for common scan findings.
- **CLI config profiles** (`internal/config/profiles.go`) — named profiles (`--profile prod`) with per-profile LLM and budget overrides.
- **Token ledger + KV cache** (`internal/tokenledger/`, `internal/llmprovider/kvcache.go`) — persistent token accounting and prompt/response KV cache.

### Fixed

- `tokenSet()` in `internal/llmcache/semantic.go` now strips trailing punctuation (`.`, `,`, `;`, `:`, `!`, `?`, quotes, brackets) so `"Go."` and `"Go"` tokenize identically.

### Changed

- README: expanded Commands table to 26 verbs; updated \u201cWhat it protects you from\u201d to include new capabilities.
- `GETTING_STARTED.md`: updated Step 6 with learning loop, incident management, deploy/rollback, privacy, and insights examples.
- All `os.WriteFile` calls use `0o600` permissions (OWASP A05 / gosec G306).
- `forge doctor` extended to include LLM-provider drift detection.
- `forge audit` extended with `query` and `erase` subcommands.
- `forge incident` extended with `triage` subcommand.
- `forge insights` split into `cli` and `hygiene` subcommands.

## [Unreleased — previous batch]

### Added
- **`forge spend`** (verb #15, DEV-M3-03) — LLM spend tracker. Subcommands: `status`, `set --daily USD --monthly USD`, `reset [--limits]`. Budget persisted as JSON at `.forge/llm-budget.json`. `--json` emits `daily_spend_usd / monthly_spend_usd / daily_limit_usd / monthly_limit_usd / record_count`. Zero limit = unlimited. Error codes: FORGE-2400..2402.
- **`forge incident`** (verb #16, DEV-M3-06) — ADR-021 incident lifecycle. Subcommands: `new --id INC-042 --title "…" --severity S1 --systems "CLI,Registry"`, `update <id> --state investigating [--note "…"]`, `list [--open] [--json]`, `close <id> [--postmortem path]`. State machine: `identified → investigating ↔ monitoring → mitigated → fixed → post-mortem-published`. Incidents stored as JSON at `.forge/incidents/<id>.json`. Error codes: FORGE-4000..4002.
- **`forge telemetry`** (verb #17, DEV-M3-01) — opt-in file-based spans (ADR-006). Subcommands: `enable`, `disable`, `status`, `rotate-id`. Config at `.forge/telemetry.json`; spans appended as JSON-Lines to `.forge/telemetry.jsonl` when opted in. Span fields: `trace_id, span_id, verb, exit_code, duration_ms, error_code, install_id, version, os, arch, timestamp` (no PII). Error codes: FORGE-4100..4199.
- **`forge audit query`** (DEV-M2-09) — sub-subcommand `forge audit query` with AND-filter semantics: `--root`, `--verb`, `--action`, `--since YYYY-MM-DD`, `--limit N`, `--json`. Empty or unmatched results return 0 rows (not error). Error code: FORGE-3402.
- **WASM plugin runtime stub** (DEV-M2-05) — `internal/plugin/wasm_stub.go` (default) provides `NewExternalPlugin` + `Call` → `ErrNotLoaded`. Build with `-tags forge_wasm` for the real wazero-backed runtime in `wasm.go`. `WASMPath string` field added to `Manifest`. Error codes: FORGE-4200..4299 (reserved).
- **`internal/llmbudget`** package — `Budget`, `Config`, `Record` types. `New()`, `Load(path)`, `Save(path)`, `Add(r)`, `DailySpend(t)`, `MonthlySpend(t)`, `CheckLimits(t)`, `SetLimits(daily, monthly)`, `Reset(resetLimits)`.
- **`internal/incident`** package — `Incident`, `State`, `Severity` types. `New()`, `Save(dir, inc)`, `Load(dir, id)`, `LoadAll(dir)`, `Transition(inc, state)`, `RenderMarkdown(inc)`, `CanTransition(from, to)`. `IsOpen()` returns false for `fixed` and `post-mortem-published`.
- **`internal/telemetry`** package — `Config`, `Span` types. `LoadConfig(path)`, `SaveConfig(path, cfg)`, `Emit(spanPath, cfg, span)`, `ReadSpans(spanPath)`, `RotateInstallID(cfg)`.
- Reserved error-code ranges: `cli/incident` (4000..4099), `cli/telemetry` (4100..4199), `plugin/wasm` (4200..4299).
- Tests: 29 for `internal/llmbudget` + `cmdspend`; 29 for `internal/incident` + `cmdincident`; 21 for `internal/telemetry` + `cmdtelemetry`; 11 for `cmdaudit query`; 8 for WASM stub. Full suite: 31 packages, all green.
- Generated `docs/ERROR_CODES.md` now lists 38 codes (was 30).

### Changed
- README: bumped to "17 verbs"; added `forge spend`, `forge incident`, `forge telemetry` rows.
- `internal/plugin.Manifest` — added `WASMPath string` field (`json:"wasm_path,omitempty"`).
- `cmd/gen-errors`: added side-effect imports for `cmdspend`, `cmdincident`, `cmdtelemetry`.
- `internal/cli/root.go`: wired verbs #15, #16, #17.
- `tasks/DEVELOPMENT_TASKS.md`: marked DEV-M2-05/09 + DEV-M3-01/03/06 as shipped.

### Added (previous Unreleased batch — now also in this release)
- **`forge postmortem [path]`** (verb #13, DEV-M3-05) — lints post-mortem documents in `docs/postmortems/INC-*.md` per ADR-020. Checks: all 8 mandatory sections present, ≥1 action item in canonical shape (`- [ ] AI-NN — … — owner: @… — due: YYYY-MM-DD — issue: #NNN`), ≥1 action item references a failure-register entry (`register: FR-NNN`) or a commit SHA. `--json` emits `[]FileReport` for dashboards. Exit non-zero for CI gate.
- **`forge insights`** (verb #14, DEV-M3-02) — local telemetry rollup from `.forge/audit.log`. Aggregates per-verb event counts with action breakdown and last-seen timestamps. `--since YYYY-MM-DD` filter. `--json` emits `Report`. No remote calls.
- **`forge audit failure-register <verify|list|lint>`** (DEV-M3-04) — manages the ADR-016 failure register at `.forge/failure-register.json`. `lint` validates schema; `list --json` dumps active entries; `verify` detects drift (entries missing `test_anchor`). Exit non-zero on drift (FORGE-3702).
- **`internal/failure`** package — ADR-016 failure-register data model. `Register`, `Entry`, `Status`, `Severity` types. JSON persistence (`Load`/`Save`/`LoadDefault`). `Validate()` detects duplicates, unknown status, missing required fields. `Active()` filters out retired entries.
- **`internal/plugin/discovery.go`** (DEV-M2-06) — `.forge/plugins.json` discovery. `DiscoverFile` reads a JSON array of `Manifest` objects and registers them as `ExternalPlugin` stubs (callable body deferred to DEV-M2-05 WASM runtime). Built-in names take precedence. `Discover(root)` wired into `root.go`'s `PersistentPreRunE` so external plugins appear in `forge plugin list`.
- Reserved error-code ranges: `cli/failure-register` (3700..3799), `cli/postmortem` (3800..3899), `cli/insights` (3900..3999).
- Tests: 8 for `internal/plugin` discovery (happy, missing file no-op, bad JSON, invalid manifest, built-in precedence, idempotency, manifest round-trip, `Discover` path contract); 18 for `internal/failure` (entry validation, register validation, `Active()` filter, save/load round-trip, idempotency, JSON keys, `LoadDefault` path contract); 11 for `cmdpostmortem` (happy, missing sections, no action item, no register ref, commit-ref satisfies, absent dir, non-INC ignored, find-all sorted, JSON, CI gate, idempotency); 10 for `cmdinsights` (count aggregate, sort, empty, `--since` filter, false-positive guard, JSON, text, empty ledger, invalid `--since`, idempotency); 5 for `cmdaudit` failure-register integration (lint OK, list JSON, verify drift FORGE-3702, verify OK, empty list).
- Generated `docs/ERROR_CODES.md` now lists 30 codes (was 23).



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
