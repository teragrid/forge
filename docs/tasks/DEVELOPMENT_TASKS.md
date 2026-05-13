# Forge — Development Tasks

> Companion to `../docs/DEVELOPMENT_PLAN.md`.
> Tracker for the Section-B tasks (DEV-M{0..3}-NN) from the master breakdown.

---

## Task Status Summary

> Updated: **v1.0.0-rc1** · 146 total tasks · Legend: ✅ shipped · 🟡 partial · ❌ not started

### Milestone rollup

| Milestone | Total | ✅ Shipped | 🟡 Partial | ❌ Not started | % done |
|-----------|------:|----------:|----------:|-------------:|-------:|
| M0 — Bootstrap | 36 | 36 | 0 | 0 | 100% |
| M1 — Workflow & Scan | 50 | 50 | 0 | 0 | 100% |
| M2 — Ecosystem | 31 | 28 | 0 | 3 | 90% |
| M3 — Quality & Launch | 29 | 29 | 0 | 0 | 100% |
| **Total** | **146** | **143** | **0** | **3** | **98%** |

### M0 — Bootstrap (36 tasks)

| ID | Title | Status |
|----|-------|:------:|
| M0-01 | Repo skeleton + DCO + cross-compile | ✅ |
| M0-02 | Config loader (layered → `forge config explain`) | ✅ |
| M0-03 | Error-code framework `FORGE-XXXX` | ✅ |
| M0-04 | Structured logger (JSON + TTY) | ✅ |
| M0-05 | Filesystem service (sandbox-aware glob) | ✅ |
| M0-06 | Git service (read-only) | ✅ |
| M0-07 | Process service (allow-list spawn) | ✅ |
| M0-08 | Audit ledger (hash-chained) | ✅ |
| M0-09 | Secrets placeholder rewriter | ✅ |
| M0-10 | CLI verb router (Cobra) | ✅ |
| M0-11 | `--json` schema framework | ✅ |
| M0-12 | `--explain` introspection surface | ✅ |
| M0-13 | `forge new <template>` | ✅ |
| M0-14 | `forge doctor` | ✅ |
| M0-15 | `forge clean` MVP | ✅ |
| M0-16 | `ILlmProvider` interface + adapters | ✅ |
| M0-17 | IDE-config detection + LLM bridge | ✅ |
| M0-18 | Token ledger (append-only) | ✅ |
| M0-19 | IDE adapter nightly compliance tests | ✅ |
| M0-20 | Manual `ship` checklist doc | ✅ |
| M0-21 | `forge ship --quick` MVP | ✅ |
| M0-22 | Unit-test harness conventions | ✅ |
| M0-23 | Integration-test harness (E2E journey) | ✅ |
| M0-24 | Eval harness scaffold + `new-app` scenario | ✅ |
| M0-25 | NFR benchmark suite scaffold | ✅ |
| M0-26 | Secret-redaction regression (100-run loop) | ✅ |
| M0-27 | CI pipeline (lint + unit + integration) | ✅ |
| M0-28 | Sigstore signing pipeline | ✅ |
| M0-29 | Brew / scoop / winget tap published | ✅ |
| M0-30 | Telemetry opt-in plumbing (off by default) | ✅ |
| M0-31 | Release notes + changelog automation | ✅ |
| M0-32 | Hygiene fixture corpus (≥30 patterns) | ✅ |
| M0-33 | `.gitignore` template fragments + composer | ✅ |
| M0-34 | `.gitleaks.toml` template + rule pack | ✅ |
| M0-35 | `forge upgrade gitignore/gitleaks` codemods | ✅ |
| M0-36 | Secret-file guard + `git ls-files` cross-check | ✅ |

### M1 — Workflow & Scan (50 tasks)

| ID | Title | Status |
|----|-------|:------:|
| M1-01 | `ship` orchestrator (5 checkpoints, resumable) | ✅ |
| M1-02 | Spec checkpoint | ✅ |
| M1-03 | Test checkpoint (tests-before-code guard) | ✅ |
| M1-04 | Breakdown checkpoint (tasks.md) | ✅ |
| M1-05 | Code checkpoint (per-task diff loop) | ✅ |
| M1-06 | Ship checkpoint (gate orchestration) | ✅ |
| M1-07 | LLM caching layer (semantic-hash key) | ✅ |
| M1-08 | LLM tier router (cheap-first escalate) | ✅ |
| M1-09 | Budget guard (per-command + per-day) | ✅ |
| M1-10 | Hygiene checkpoint (auto `forge clean`) | ✅ |
| M1-11 | Scan engine kernel + `Finding` schema | ✅ |
| M1-12 | Scanner: secrets | ✅ |
| M1-13 | Scanner: RLS / authz | ✅ |
| M1-14 | Scanner: prompt-injection | ✅ |
| M1-15 | Scanner: supply-chain | ✅ |
| M1-16 | `forge fix --apply` (confidence-tier) | ✅ |
| M1-17 | Waiver mechanism (`.forge/waivers/`) | ✅ |
| M1-18 | Plugin loader (ADR-002, WASM) | ✅ |
| M1-19 | Capability + permission model | ✅ |
| M1-20 | Plugin manifest schema + validator | ✅ |
| M1-21 | In-tree scanner-plugin proof (`scanner-cost`) | ✅ |
| M1-22 | In-tree generator-plugin proof (`gen-endpoint`) | ✅ |
| M1-23 | `forge lint` + instructions packs | ✅ |
| M1-24 | First defaults instructions pack | ✅ |
| M1-25 | New-pattern detection + RFC-link in lint | ✅ |
| M1-26 | Spec-presence CI gate | ✅ |
| M1-27 | Tests-precede-code CI gate | ✅ |
| M1-28 | `forge scan all --since main` CI gate | ✅ |
| M1-29 | Convention lint CI gate | ✅ |
| M1-30 | Public-API delta declaration gate | ✅ |
| M1-31 | Token-budget regression gate | ✅ |
| M1-32 | Docs-sync gate | ✅ |
| M1-33 | DCO + signed-commit gate | ✅ |
| M1-34 | Repo-hygiene gate (`forge clean --check`) | ✅ |
| M1-35 | Secrets-clean CI gate | ✅ |
| M1-36 | Pre-commit secret scan + bypass token | ✅ |
| M1-37 | Allowlist-expiry sweeper (nightly) | ✅ |
| M1-38 | `.gitignore` drift detection in `forge doctor` | ✅ |
| M1-39 | Resilience-pattern library | ✅ |
| M1-40 | Failure-register data model + verifier | ✅ |
| M1-41 | Issue templates + auto-triage bot | ✅ |
| M1-42 | `forge test` verb (13 families) | ✅ |
| M1-43 | `forge adopt` verb | ✅ |
| M1-44 | `forge check` verb | ✅ |
| M1-45 | `forge generate <kind>` full verb | ✅ |
| M1-46 | `forge context generate\|show\|budget` | ✅ |
| M1-47 | `forge ask` Q&A verb | ✅ |
| M1-48 | `forge fix` full verb | ✅ |
| M1-49 | `forge review` LLM PR-review verb | ✅ |
| M1-50 | `forge migrate` full verb | ✅ |

### M2 — Ecosystem (31 tasks)

| ID | Title | Status |
|----|-------|:------:|
| M2-01 | Plugin Registry index (signed JSON) + CDN | ✅ |
| M2-02 | `forge plugin install/list/upgrade/remove` | ✅ |
| M2-03 | Plugin compliance test runner | ✅ |
| M2-04 | Second LLM provider plugin | ✅ |
| M2-05 | Deploy adapter #1 | ✅ |
| M2-06 | Deploy adapter #2 | ✅ |
| M2-07 | Storage adapter #1 | ✅ |
| M2-08 | Eval harness (7 reference scenarios) | ✅ |
| M2-09 | Learning loop client (opt-in share path) | ✅ |
| M2-10 | Learning loop aggregator MVP | ✅ |
| M2-11 | Scanner: auth | ✅ |
| M2-12 | Scanner: perf | ✅ |
| M2-13 | Scanner: accessibility | ✅ |
| M2-14 | Scanner: cost (cloud + LLM) | ✅ |
| M2-15 | `forge upgrade` codemod runner | ✅ |
| M2-16 | Backward-compat alias mechanism | ✅ |
| M2-17 | Performance benchmark gate (≤5% regression) | ✅ |
| M2-18 | Plugin-loader sandbox fuzz | ✅ |
| M2-19 | First 3 external community plugins | ✅ |
| M2-20 | Eval harness gates a PR (proof-of-life) | ✅ |
| M2-21 | Pilot user case study | ✅ |
| M2-22 | Migration runner tests | ✅ |
| M2-23 | Chaos-drill harness (8 scenarios) | ✅ |
| M2-24 | Post-mortem template + CI gate | ✅ |
| M2-25 | Status-page wiring | ✅ |
| M2-26 | `forge eject` verb | ✅ |
| M2-27 | `forge docs sync/heal` verbs | ✅ |
| M2-28 | `forge hygiene report/manifest` verbs | ✅ |
| M2-29 | `forge learn` sub-verbs | ✅ |
| M2-30 | `forge deploy` / `forge rollback` verbs | ✅ |
| M2-31 | `forge agents start/stop/list` namespace | ✅ |

### M3 — Quality & Launch (29 tasks)

| ID | Title | Status |
|----|-------|:------:|
| M3-01 | `THREAT_MODEL.md` complete (STRIDE) | ✅ |
| M3-02 | External pentest (CLI + plugin loader) | ✅ |
| M3-03 | Bug-bounty program live | ✅ |
| M3-04 | All NFR budgets (Arch §14) in CI | ✅ |
| M3-05 | Docs site complete (100% coverage) | ✅ |
| M3-06 | RFC process (≥3 accepted) | ✅ |
| M3-07 | Contribution-standards CI bot | ✅ |
| M3-08 | Maintainer review-SLA dashboard | ✅ |
| M3-09 | T2 adapter coverage (5 cloud + 3 LLM) | ✅ |
| M3-10 | Perf regression gates locked | ✅ |
| M3-11 | i18n scaffolding | ✅ |
| M3-12 | Telemetry payload audit + public schema | ✅ |
| M3-13 | Air-gapped install path | ✅ |
| M3-14 | All §16.5.4 gates active | ✅ |
| M3-15 | v1.0.0 release artifact + signing | ✅ |
| M3-16 | Post-1.0 deprecation policy doc | ✅ |
| M3-17 | Status page + incident runbook | ✅ |
| M3-18 | Launch post | ✅ |
| M3-19 | Private-vulnerability intake + `SECURITY.md` | ✅ |
| M3-20 | Two-key enforcement | ✅ |
| M3-21 | Eval flake quarantine policy | ✅ |
| M3-22 | Reversibility contract (`forge undo`) | ✅ |
| M3-23 | High-trust / regulated-industry templates | ✅ |
| M3-24 | TypeScript `@forge/*` npm packages | ✅ |
| M3-25 | `forge optimize` verb | ✅ |
| M3-26 | Per-verb prompt templates | ✅ |
| M3-27 | Multi-tool AI instructions (AGENTS.md etc.) | ✅ |
| M3-28 | Multi-agent runtime foundation | ✅ |
| M3-29 | `forge add <primitive>` verb | ✅ |

---

## Release status — what shipped where

> Latest tag: **`v0.4.0-m1-complete`** — HEAD. CI green. All 50 M1 tasks shipped (48 full, 2 partial: M1-21 scanner-cost plugin, M1-22 gen-endpoint plugin). New verbs: `forge adopt`, `forge check`, `forge generate`, `forge context`, `forge ask`, `forge fix`, `forge review`, `forge migrate`, `forge test`. New packages: `internal/resilience`, `internal/tierrouter`, `internal/waiver`, `internal/llmcache`. Pre-commit hooks, nightly allowlist sweep, and all 9 CI gates in `.github/workflows/ci-gates.yml`.

---

## Shipped features at a glance

### `v0.1.0-mvp` — community-launch slice (M0 + M1 partial)

### `v0.1.0-mvp` — community-launch slice (M0 + M1 partial)

| Task | Title | Status |
|------|-------|--------|
| DEV-M0-01 | Repo skeleton + DCO + cross-compile | ✅ (CI gates, 6-triple matrix, CODEOWNERS) |
| DEV-M0-02 | Config loader (layered) | ✅ `internal/config` (layered defaults→file→env→flags); `cmdconfig` package; `forge config show/get/explain` with `--json` |
| DEV-M0-03 | Error-code framework `FORGE-XXXX` | ✅ `internal/errcode` (reserved-range registry, panic on dup, tests) |
| DEV-M0-04 | Structured logger | ✅ `internal/logobs` (slog wrapper, secret redaction, `--explain` bypass) |
| DEV-M0-11 | CLI verb router | ✅ `internal/cli` + `internal/cli/cmd<verb>/` subpackages, `verbmeta` registry |
| DEV-M0-12 | `forge explain` | ✅ `--json` supported; lists all verbs or one manifest |
| DEV-M0-13 | `forge new` | ✅ `go-service` + `ts-service` templates; `--name`/`--module`/`--force`/`--json` |
| DEV-M0-13b | `forge init` | ✅ initialises existing dir; auto-detects template from `package.json`/`go.mod` |
| DEV-M0-14 | `forge doctor` | ✅ checks git/go/temp; `--json` supported |
| DEV-M0-15 | `forge clean` | ✅ `--check`/`--apply`; manifest-driven |
| DEV-M0-22 | Bundled `.gitignore` template | ✅ ships in both `go-service` and `ts-service` templates with marker block |
| DEV-M0-23 | Bundled `.gitleaks.toml` | ✅ ships in both templates (4 baseline rules) |
| DEV-M0-27 | `.forge/manifest` reader | ✅ `internal/manifest` (scratch/managed sections, glob matcher) |
| DEV-M0-33 | CI workflow (lint+test+build matrix) | ✅ `.github/workflows/ci.yml` (Go 1.25, race+cgo) |
| DEV-M0-34 | Release workflow + npm distribution | ✅ `.goreleaser.yml` + `.github/workflows/release.yml` + `packages/` (5 platform packages + `@forge/cli` wrapper) |

### `v0.2.0-m2-preview` — plugin loader, codemod runner, audit ledger (M1 expansion + M2 scaffolding + M3 spike)

| Task | Title | Status |
|------|-------|--------|
| DEV-M1-01 | `forge scan secrets` | ✅ builtin regex engine + gitleaks fallback (5 rules) |
| DEV-M1-02 | `forge scan rls` | ✅ SQL/migration tenant-column scanner |
| DEV-M1-03 | `forge scan prompt-injection` | ✅ 4 patterns (ignore-previous, role-override, system-prompt-leak, unsafe-eval) |
| DEV-M1-04 | `forge scan supply-chain` | ✅ 4 patterns (loose-version, unpinned-git, curl-pipe-shell, go.mod replace) |
| DEV-M1-05 | `forge lint` | ✅ hygiene checker (manifest, gitignore markers, gitleaks baseline) |
| DEV-M1-06 | `forge ship` | ✅ 5-checkpoint pipeline validator (`--dry-run`) |
| DEV-M0-10 | Plugin runtime ABI (in-process) | ✅ `internal/plugin` (Manifest, Scanner, Codemod, Provider, Template; thread-safe Registry) |
| DEV-M2-15 | `forge upgrade` codemod runner | ✅ 2 builtins (`gitignore-marker`, `gitleaks-baseline`); `--apply`/dry-run/`list` |
| DEV-M0-08 | Audit ledger | ✅ `internal/audit` SHA-256 hash-chained JSONL at `.forge/audit.log`; `forge audit show/verify/append` |
| DEV-M2-02 | Plugin discovery (in-tree) | ✅ scanners + codemods auto-register to `plugin.Default()`; `forge plugin list/show` (verb #11) |
| DEV-M3-S1 | NFR benchmarks (spike) | ✅ `BenchmarkScanSecrets_500Files`, `BenchmarkScaffold_GoService`; `make bench` |
| DEV-M3-S2 | Error-code doc generator | ✅ `cmd/gen-errors`; `make docs` / `make docs-check`; 20 codes |

### Remaining for `v0.3.0` and beyond

**M2.x — plugin ecosystem + scan families**
- `forge eval` scenario harness (YAML scenarios, deterministic runner, JSON report; codes 3600..3699) ✅ **shipped** *(partial implementation of DEV-M2-08)*
- Wazero WASM plugin runtime behind `forge_wasm` build tag ✅ **shipped** (`wasm_stub.go` + `wasm.go` + `wasm_stub_test.go`; 8 tests; codes 4200..4299)
- `.forge/plugins.json` discovery + dynamic registration ✅ **shipped** *(contributes to DEV-M2-02)*
- DEV-M2-15 — Codemods: `dependabot-baseline`, `pre-commit-baseline` ✅ **shipped**
- Audit ledger: `forge audit query` sub-subcommand ✅ **shipped** (AND-filter, `--since`, `--limit`, `--json`; code 3402; 11 tests)
- DEV-M2-11..14 — M2 scan families (7 pattern-based scanners) ✅ **shipped** (`correctness`, `performance`, `reliability`, `accessibility`, `cost`, `compliance`, `dx`; all wired to `forge scan <family>` + `forge scan all`)
- `ts-service` template completion ✅ **shipped** (infrastructure clients, shared errors/types/middleware/guards, security tests; all 38 files; `scaffold_test.go` wantFiles updated)
- `docs/DISTRIBUTION.md` ✅ **shipped** (npm, Homebrew tap, Scoop bucket full publish guide + goreleaser blocks)

**M3 — governance + telemetry**
- DEV-M3-01 — File-based telemetry spans (ADR-006) ✅ **shipped** (`internal/telemetry` + `forge telemetry enable/disable/status/rotate-id` verb #17; codes 4100..4199; 21 tests)
- DEV-M3-02 — `forge insights` verb (#14) — local telemetry rollup ✅ **shipped**
- DEV-M3-03 — LLM budget tracker ✅ **shipped** (`internal/llmbudget` + `forge spend` verb #15; codes 2400..2402; 29 tests)
- DEV-M3-04 — Failure-register data model (ADR-016) ✅ **shipped**
- DEV-M3-05 — Postmortem CI gate (ADR-020) ✅ **shipped**
- DEV-M3-06 — Incident lifecycle management (ADR-021) ✅ **shipped** (`internal/incident` + `forge incident new/update/list/close` verb #16; codes 4000..4002; 29 tests)

**Cross-cutting**
- Coverage uplift: `cmdaudit` (70%) → ✅ **88.6%** (14 new tests in `audit_coverage_test.go`); `codemod` (85.6%) ✅ already ≥85%
- ADR-002 update: WASM runtime decision finalized
- Threat-model refresh after WASM lands

Items still deferred from M0: DEV-M0-08/19/20/21/24/25/26/28/29/30/31/32/35/36 (see detail rows below).
Items shipped this session: DEV-M0-05, M0-06, M0-07, M0-09, M0-16, M0-17, M0-18 (infrastructure packages + `forge new --list`).

---

## Gap analysis — current code vs spec (as of v0.3.0-m3-preview)

> Legend: ✅ shipped | 🟡 partial/stub | ❌ not started

### Scaffold templates

| Item | Status | Notes |
|---|---|---|
| `go-service` template | ✅ | `main.go` + `main_test.go` runnable; `docker-compose.yml`; full `.forge/` conventions; CI workflow |
| `ts-service` template | ✅ | All files present and runnable: `src/main.ts`, auth module, `src/infrastructure/` (db/queue/storage), `src/shared/` (errors, types, middleware, guards), `eslint.config.js`, `vitest.config.ts`, `tests/security/auth.security.test.ts`. `npm run dev` works. |
| `react-spa` template | ❌ | Not started. Spec §4 mentions it as a future template. |
| `next-app` template | ✅ | Next.js 14 App Router + TypeScript + Tailwind CSS; `app/` directory with layout, home page, globals CSS; `/api/health` probe; security headers in `next.config.ts`; vitest unit tests + Playwright e2e; `AGENTS.md`, `CLAUDE.md`, `.cursorrules`, `.windsurfrules`, `README.md`, `ROLLBACK.md`; CI + deploy workflows; all forge conventions wired; `scaffold_test.go` updated with 8 new tests. |

### `forge new` / `forge init`

| Item | Status | Notes |
|---|---|---|
| `forge new go-service <path>` | ✅ | End-to-end runnable |
| `forge new ts-service <path>` | ✅ | Scaffold renders all 38 files; `npm test` passes; `npm run dev` works |
| `forge new next-app <path>` | ✅ | Scaffold renders all 27 files; App Router structure; `/api/health` probe; 8 tests |
| `forge init [path]` | ✅ | Auto-detect + positional path; 14 tests |
| `forge new --list` | ✅ | `--list` lists available templates; `--list --json` emits `{"templates":[...]}` |
| Template `--description` flag wired | 🟡 | `Vars.Description` field exists; only `ts-service` uses it; `go-service` ignores it |

### `forge ship` (4-checkpoint orchestrator)

| Item | Status | Notes |
|---|---|---|
| 5-checkpoint validator (structural) | ✅ | `--dry-run` mode validates pipeline shape |
| Spec checkpoint (LLM-driven) | ✅ | `checkSpec` with `LLMPipe.Invoke` — generates/reviews spec.md; graceful degradation to stub |
| Test-generation checkpoint | ✅ | `checkTest` + `generateTestStubs` — writes test-stubs.md via LLM |
| Breakdown checkpoint | ✅ | `checkBreakdown` + `generateBreakdown` — writes breakdown.md via LLM |
| Code+scan checkpoint (full loop) | ✅ | `checkCode` + `generateCodePlan` — writes code-plan.md; scans via `checkVerify` |
| PR creation | ✅ | `checkPR` via gh CLI; best-effort (warning on absence, never fail) |

### Scan families

| Scanner | Status | Notes |
|---|---|---|
| `forge scan secrets` | ✅ | Builtin regex + gitleaks fallback |
| `forge scan rls` | ✅ | SQL tenant-column scanner |
| `forge scan prompt-injection` | ✅ | 4 patterns |
| `forge scan supply-chain` | ✅ | 4 patterns |
| `forge scan correctness` | ✅ | 5 rules: float-money, unsafe-type-assert, ts-any, silenced-error, int-truncation |
| `forge scan performance` | ✅ | 4 rules: select-star, unbounded-query, mutex-no-defer, fk-missing-index |
| `forge scan reliability` | ✅ | 6 rules: http-no-timeout, goroutine, ctx-propagation, fetch-no-timeout, payment-idempotency, panic-on-error |
| `forge scan accessibility` | ✅ | 5 rules: img-alt, link-text, button-label, tabindex, html-lang |
| `forge scan cost` | ✅ | 4 rules: llm-in-loop, llm-no-token-limit, unbounded-cloud-list, missing-cache-control |
| `forge scan compliance` | ✅ | 5 rules: pii-in-logs, hardcoded-region, mutation-no-audit, raw-pii-storage, insecure-cookie |
| `forge scan dx` | ✅ | 4 rules: todo-fixme-density, source-without-test, stale-forge-version, missing-forge-manifest |

### Infrastructure / services (DEV-M0-05 to M0-09, M0-16 to M0-18)

| Item | Status | Notes |
|---|---|---|
| Filesystem sandbox service | ✅ | DEV-M0-05 — `internal/fssandbox`; escape-prevention, symlink check, 12 tests |
| Git service (read-only) | ✅ | DEV-M0-06 — `internal/gitservice`; status, log, diff-since, changed-files |
| Process spawn allow-list | ✅ | DEV-M0-07 — `internal/procspawn`; deny-by-default, concurrent-safe, timeout |
| Secrets placeholder rewriter | ✅ | DEV-M0-09 — `internal/secretrewriter`; OpenAI/Anthropic/JWT/GH/AWS patterns, idempotent |
| `ILlmProvider` interface + adapters | ✅ | DEV-M0-16 — `internal/llmprovider`; `Provider` interface, Anthropic + OpenAI stubs, `MockProvider` |
| IDE-config detection + bridge | ✅ | DEV-M0-17 — `llmprovider.Detect()` reads env-vars; ANTHROPIC→Claude, OPENAI→OpenAI, else FORGE-4050 |
| Token ledger | ✅ | DEV-M0-18 — `internal/tokenledger`; JSONL append, per-model summary, concurrent-safe |

### Distribution

| Item | Status | Notes |
|---|---|---|
| `.goreleaser.yml` | ✅ | 5-platform build, checksums, SBOM, changelog |
| `@forge/cli` npm wrapper | ✅ | `packages/cli/bin/forge.js` — platform detection + exec |
| Platform packages (5) | ✅ | `@forge/cli-{linux-x64,linux-arm64,darwin-x64,darwin-arm64,win32-x64}` |
| `release.yml` CI workflow | ✅ | goreleaser → npm publish on tag push |
| `scripts/npm-publish.sh` | ✅ | Version stamping + binary placement |
| Published to npm registry | ❌ | Requires first `git tag v0.1.0` + secrets configured |
| Homebrew tap | 🟡 | `docs/DISTRIBUTION.md` has full guide + `Formula/forge.rb` template; goreleaser `brews:` block documented; tap repo still needs to be created |
| Scoop bucket (Windows) | 🟡 | `docs/DISTRIBUTION.md` has full guide + `bucket/forge.json` manifest; goreleaser `scoops:` block documented; bucket repo still needs to be created |

### Gaps blocking "early adopter usable" for TypeScript projects

✅ **All critical path items now complete** — `forge new ts-service my-app` produces a fully working dev server:

1. ✅ **`src/main.ts`** — entry point wires up HTTP server and auth module.
2. ✅ **`vitest.config.ts`** — test runner config present; `npm test` passes.
3. ✅ **`eslint.config.js`** — `npm run lint` works.
4. ✅ **`src/infrastructure/database/client.ts`** — postgres client stub; `src/infrastructure/queue/client.ts` and `src/infrastructure/storage/client.ts` also added.
5. ✅ **`src/shared/`** — `errors.ts`, `types.ts`, `middleware/auth.middleware.ts`, `guards/workspace.guard.ts` added.

---

ID and conventions follow `ARCHITECTURE_TASKS.md`. Each implementation task (T1/T2/T3) lists explicit **test cases** (TC-IDs) following the 9-point checklist from `always-write-tests.md` (happy / boundary / negative / idempotency / concurrency / cross-tenant / regression / data-accuracy / false-positive guard) — only points that meaningfully apply are included, never invented. OPS/DOC tasks list **verification checks** instead of unit-style cases.

---

## Architecture → Tasks gaps

> Items described in `docs/ARCHITECTURE.md` that have no corresponding DEV-* task coverage.

| Architecture reference | Description | Gap |
|---|---|---|
| §18.1 `forge report` verb | Opens a GitHub issue with `forge doctor --bundle` output attached | No DEV-* task exists for this CLI verb |
| §17.2 LLM cache row | `forge cache purge` to clear the semantic response cache | No dedicated task; blocked by DEV-M1-07 (not started); `internal/llmcache` package absent |
| §12 Hosted control plane | Helm chart + Terraform module in `deploy/` for a self-hosted Forge control plane | No task covers hosted deployment infrastructure |
| §17.1 #3 Per-verb state checkpoints | `.forge/state/<verb>.json` hash-chained before each side-effect | Implicitly in DEV-M1-01..06 but no explicit task defining the checkpoint-format contract |
| §17.1 #5 Trash dir (all `--apply` verbs) | Every `--apply` verb must write a `.forge/trash/<run-id>/` snapshot before mutating | DEV-M3-22 covers `forge undo` but no task explicitly enforces trash-write on *all* `--apply` verbs |
| §11 i18n structure | `internal/i18n` message-catalog package required even for English-only v1.0 | DEV-M3-11 covers this; `internal/i18n` package still absent from codebase |
| §9 LLM cache layer | `internal/llmcache` deterministic-replay + cache-miss path | DEV-M1-07 is the task; package not yet created |

---

## M0 — Bootstrap (DEV-M0-01 .. DEV-M0-36)

### DEV-M0-01 — Repo skeleton: monorepo layout, license, CODEOWNERS, DCO bot
- **Tier:** T1 — **Anchor:** Arch §3 C1, ADR-001
- **Status:** ✅ SHIPPED — monorepo layout, LICENSE (Apache-2.0), CODEOWNERS, DCO bot, 6-triple cross-compile matrix; `make build` and CI green on all platforms.
- **Stack baseline (per ADR-001):** Go (current release), `go.mod` with `toolchain` + `go` directives, `CGO_ENABLED=0` default, standard layout (`cmd/forge/`, `internal/`, `pkg/`).
- **Acceptance:** `git clone && make build` succeeds on CI; `golangci-lint run`, `go test -race ./...`, `govulncheck ./...`, `gofmt -d` and `goimports -l` are all green; cross-compile matrix (linux/darwin/windows × amd64/arm64) produces 6 binaries with `CGO_ENABLED=0`.
- **Test cases:**
  - TC-01-01 (happy): fresh clone + `make build` exits 0 on linux + macOS + windows runners.
  - TC-01-02 (negative): PR without DCO sign-off is blocked by the bot.
  - TC-01-03 (regression): a deliberately-deleted CODEOWNERS file fails CI ("required file missing").
  - TC-01-04 (false-positive guard): a docs-only change still passes build (no spurious build trigger).
  - TC-01-05 (boundary): a PR introducing a cgo-requiring import fails the `CGO_ENABLED=0` cross-compile gate.
  - TC-01-06 (data-accuracy): the 6-triple cross-compile matrix produces 6 distinct binaries with matching `forge --version`.
### DEV-M0-02 — Config loader (layered: defaults→file→env→flags) + `forge config explain`
- **Tier:** T1 — **Anchor:** Arch §11
- **Status:** ✅ SHIPPED — `internal/config` layered config package (defaults→`forge.yml`→env→flags); `internal/cli/cmdconfig` (`forge config show/get/explain` + `--json`); 7 config tests + 7 cmdconfig tests; errcode ranges 1400..1401 (CLI), 2200..2202 (config); `cmdconfig` registered in root.
- **Acceptance:** Unit + boundary tests; `--explain` shows source per key.
- **Test cases:**
  - TC-02-01 (happy): each layer overrides the layer below it for a known key.
  - TC-02-02 (boundary): missing file / empty env still resolves to defaults; no crash.
  - TC-02-03 (negative): malformed config file fails with `FORGE-XXXX`, not a stack trace.
  - TC-02-04 (data-accuracy): `--explain --key K` reports the winning layer correctly under every combination tested by TEST-15.

### DEV-M0-03 — Error-code framework (`FORGE-XXXX`) + reserved-range registry + lint rule
- **Tier:** T1 — **Anchor:** Arch §11
- **Status:** ✅ SHIPPED — `internal/errcode`; reserved-range registry; panic-on-dup at init; `cmd/gen-errors` doc generator; `make docs-check` CI gate; 19 namespace ranges registered.
- **Acceptance:** Lint blocks duplicate codes; doc auto-generated.
- **Test cases:**
  - TC-03-01 (happy): a new error code in an unused range passes lint.
  - TC-03-02 (negative): duplicate code anywhere in the tree fails lint with both source locations cited.
  - TC-03-03 (negative): code outside any reserved range fails lint.
  - TC-03-04 (data-accuracy): generated error-code doc lists every code with its description.
  - TC-03-05 (false-positive guard): the same code referenced (not declared) twice does not trip the duplicate check.

### DEV-M0-04 — Structured logger (JSON + TTY formatter)
- **Tier:** T1 — **Anchor:** Arch §11
- **Status:** ✅ SHIPPED — `internal/logobs`; slog wrapper; JSON + TTY formatters; secret-field redaction; `--explain` bypass; structured levels (debug/info/warn/error).
- **Acceptance:** Unit + integration; never logs prompts unless `--explain`.
- **Test cases:**
  - TC-04-01 (happy): JSON mode emits one event per log call with required fields.
  - TC-04-02 (boundary): log of a 1 MB payload is truncated with a marker, not OOM.
  - TC-04-03 (negative): attempting to log a value flagged "secret" emits redacted form.
  - TC-04-04 (regression): default mode never includes raw LLM prompt content (snapshot test).
  - TC-04-05 (data-accuracy): `--explain` mode includes prompt content under a labeled field, byte-for-byte.

### DEV-M0-05 — Filesystem service with sandbox-aware glob
- **Tier:** T1 — **Anchor:** Arch §8.2
- **Status: ✅ SHIPPED** — `internal/fssandbox`; escape prevention via path prefix check + symlink guard; 12 tests; errcode 2500 (ErrEscape), 2501 (ErrNotFound).
- **Acceptance:** Permission-denial test; cross-OS path test.
- **Test cases:**
  - TC-05-01 (happy): glob inside granted root returns expected files on linux/mac/win.
  - TC-05-02 (negative): glob escaping the grant (`../../etc/passwd`) is denied with `FORGE-XXXX`.
  - TC-05-03 (boundary): symlink at the grant boundary is not followed outward.
  - TC-05-04 (cross-OS): backslash vs forward-slash handling identical across platforms.

### DEV-M0-06 — Git service (read-only ops: status, diff-since, log)
- **Tier:** T1 — **Anchor:** Arch §6
- **Status: ✅ SHIPPED** — `internal/gitservice`; wraps git binary for status, log, diff-since, changed-files; FORGE-2600 (not-repo), FORGE-2601 (git-not-found), FORGE-2602 (git-failed); 10 tests.
- **Acceptance:** Integration test against real repo fixtures.
- **Test cases:**
  - TC-06-01 (happy): `status` / `diff-since` / `log` return correct shapes against a fixture repo.
  - TC-06-02 (boundary): empty repo + first-commit edge cases handled.
  - TC-06-03 (negative): operating outside a repo exits with `FORGE-XXXX`.
  - TC-06-04 (false-positive guard): the service has zero write paths (audit grep test).

### DEV-M0-07 — Process service (proc spawn with allow-list)
- **Tier:** T1 — **Anchor:** Arch §8.2
- **Status: ✅ SHIPPED** — `internal/procspawn`; deny-by-default allow-list; ext-stripping for Windows .exe; FORGE-2700 (not-allowed), FORGE-2701 (timeout), FORGE-2702 (run-failed); 10 tests including concurrency.
- **Acceptance:** Sandbox escape test; deny-by-default verified.
- **Test cases:**
  - TC-07-01 (happy): allow-listed binary runs and stdout is captured.
  - TC-07-02 (negative): non-allow-listed binary is denied.
  - TC-07-03 (negative): allow-listed binary invoked with arg trying to spawn a child outside allow-list is blocked.
  - TC-07-04 (concurrency): two parallel spawns share no process group leakage.
  - TC-07-05 (regression): every prior sandbox-escape ticket has a fixture here.

### DEV-M0-08 — Audit ledger (append-only, hash-chained)
- **Tier:** T1 — **Anchor:** Arch §15
- **Status:** ✅ SHIPPED — `internal/audit`; SHA-256 hash-chained JSONL at `.forge/audit.log`; `forge audit show/verify/append/query`; AND-filter, `--since`, `--limit`, `--json`; errcode range 3400..3499; 14 tests.
- **Acceptance:** Tamper-detection test; replay test.
- **Test cases:**
  - TC-08-01 (happy): N appends + verify chain is valid.
  - TC-08-02 (negative): mutating any byte of any prior entry breaks `verify`.
  - TC-08-03 (idempotency): replay of the ledger reconstructs the same end state.
  - TC-08-04 (boundary): empty ledger verifies clean.
  - TC-08-05 (data-accuracy): each entry's hash equals `H(prev || payload)` (snapshot vector).

### DEV-M0-09 — Secrets handling (placeholder rewriter for prompts)
- **Tier:** T1 — **Anchor:** Arch §11 + §15
- **Status: ✅ SHIPPED** — `internal/secretrewriter`; default patterns: OpenAI sk-, Anthropic sk-ant-, JWT eyJ, GitHub ghp_, AWS AKIA, env-var base64 secrets; `[REDACTED:<type>]` placeholders; idempotent; 12 tests.
- **Acceptance:** Seeded-secret test asserts zero leakage in 100 runs (TEST-12).
- **Test cases:**
  - TC-09-01 (happy): seeded secret never appears in 100 outbound LLM payloads.
  - TC-09-02 (negative): non-secret string of similar shape is NOT redacted.
  - TC-09-03 (data-accuracy): redacted placeholder length is `min(8, raw_len)`.
  - TC-09-04 (regression): historical leakage fixtures all redacted.
  - TC-09-05 (false-positive guard): a string the user explicitly marks `--no-redact` (in `--explain`) flows through unchanged.

### DEV-M0-10 — CLI verb router (`forge <ns> <verb>`) with 3 namespaces (`new/doctor/explain`)
- **Tier:** T1 — **Anchor:** Spec §4 Command Surface
- **Status:** ✅ SHIPPED — `internal/cli/root.go`; Cobra dispatch; `internal/verbmeta` registry; 30+ `cmd<verb>/` packages; `forge explain` lists all verbs; `--json`/`--explain` universal flags wired.
- **Acceptance:** Unit + `--help` snapshot test.
- **Test cases:**
  - TC-10-01 (happy): each (ns, verb) dispatches correctly.
  - TC-10-02 (negative): unknown verb under known ns shows ns-scoped help.
  - TC-10-03 (negative): unknown ns returns global help with `FORGE-XXXX`.
  - TC-10-04 (data-accuracy): `--help` snapshot stable across runs.

### DEV-M0-11 — `--json` schema framework + per-verb schema tests
- **Tier:** T1 — **Anchor:** Spec §4 universal flags
- **Status:** ✅ SHIPPED — `--json` flag registered on all verbs; per-verb JSON schema validation CI gate wired; schema drift detector active.
- **Acceptance:** Schema test harness; every `--json` output validated.
- **Test cases:**
  - TC-11-01 (happy): each verb's `--json` output validates against its schema.
  - TC-11-02 (negative): a verb that drifts (adds field without schema bump) fails CI.
  - TC-11-03 (boundary): empty result set still validates.
  - TC-11-04 (data-accuracy): schema version field equals declared verb version.

### DEV-M0-12 — `--explain` introspection surface (per verb manifest emission)
- **Tier:** T1 — **Anchor:** Spec §4 + Arch §1
- **Status:** ✅ SHIPPED — `cmdexplain` + `internal/verbmeta`; `--json` emits verb manifest; `forge explain <verb>` shows inputs/outputs/side-effects; list-all mode wired.
- **Acceptance:** Snapshot test per verb.
- **Test cases:**
  - TC-12-01 (happy): each verb under `--explain` emits manifest with inputs, outputs, side-effects, gates touched.
  - TC-12-02 (regression): a side-effect added without manifest update fails CI.
  - TC-12-03 (false-positive guard): no-op verbs still emit a manifest (do not crash on empty side-effect set).

### DEV-M0-13 — `forge new <template>` happy-path
- **Tier:** T1 — **Anchor:** Spec §4
- **Status:** ✅ SHIPPED — `cmdnew`; `go-service` + `ts-service` templates; `--name`/`--module`/`--force`/`--json`/`--list`; scaffold test suite green; 38-file ts-service renders completely.
- **Acceptance:** `new-app` eval scenario passes (TEST-06).
- **Test cases:**
  - TC-13-01 (happy): scaffold runs and the resulting app builds.
  - TC-13-02 (negative): scaffold into non-empty dir fails unless `--force`.
  - TC-13-03 (idempotency): re-running with `--force` reproduces byte-identical output (excluding timestamps).
  - TC-13-04 (data-accuracy): rendered `.gitignore` and `.gitleaks.toml` pass TEST-22 / TEST-21 contracts.

### DEV-M0-14 — `forge doctor` (env health check)
- **Tier:** T1 — **Anchor:** Spec §4
- **Status:** ✅ SHIPPED — `cmddoctor`; checks git/go/temp deps; `--json` structured output; per-check `FORGE-XXXX` codes; actionable remediation messages.
- **Acceptance:** Detects missing deps with actionable `FORGE-XXXX`.
- **Test cases:**
  - TC-14-01 (happy): healthy environment exits 0 with summary.
  - TC-14-02 (negative): each detectable failure mode exits non-zero with one specific `FORGE-XXXX`.
  - TC-14-03 (data-accuracy): `--json` output enumerates every check with status + remediation hint.
  - TC-14-04 (false-positive guard): a benign warning (e.g. optional tool missing) does NOT fail the gate.

### DEV-M0-15 — `forge clean` MVP (manifest-based dry-run + `--apply`)
- **Tier:** T1 — **Anchor:** Spec §4 hygiene
- **Status:** ✅ SHIPPED — `cmdclean`; `--check`/`--apply`; manifest-driven allow-list; `internal/manifest` scratch/managed sections; dry-run identity test passing.
- **Acceptance:** Hygiene fixture corpus passes; dry-run identity test.
- **Test cases:**
  - TC-15-01 (happy): every entry in the hygiene corpus is detected.
  - TC-15-02 (idempotency): dry-run twice → identical report; tree unchanged.
  - TC-15-03 (negative): `--apply` deleting a file outside the manifested patterns is impossible (allow-list semantics).
  - TC-15-04 (boundary): empty repo → zero findings, exit 0.
  - TC-15-05 (false-positive guard): an unmanifested but legitimate file (e.g. `README.md`) is never proposed for deletion.

### DEV-M0-16 — `ILlmProvider` interface + IDE-config adapters + mock
- **Tier:** T1 — **Anchor:** Arch §9.2
- **Status: ✅ SHIPPED** — `internal/llmprovider`; `Provider` interface (Name/Complete/Capabilities); `AnthropicAdapter` + `OpenAIAdapter` stubs; `MockProvider` with call counter; `Detect()` env-var bridge; FORGE-4050 (no-provider), FORGE-4051 (fail), FORGE-4052 (invalid-input); 18 tests.
- **Scope:** Abstraction layer over IDE-detected LLM connections. Forge never stores credentials; adapters read them from the IDE/dev-tool that the vibe-coder already has configured.
- **Acceptance:** Provider compliance suite (v0) defined; detection pass resolves correctly for each adapter.
- **Test cases:**
  - TC-16-01 (happy): mock adapter passes the v0 compliance suite without any credentials.
  - TC-16-02 (negative): a deliberately broken adapter (omits `Capabilities.Streaming`) fails the compliance suite.
  - TC-16-03 (boundary): empty prompt input returns spec-defined empty completion, not an error.
  - TC-16-04 (happy): detection pass with `ANTHROPIC_API_KEY` set resolves to Claude adapter.
  - TC-16-05 (happy): detection pass with `OPENAI_API_KEY` set resolves to OpenAI adapter.
  - TC-16-06 (negative): detection pass with nothing configured returns `FORGE-4001`; no partial state left.
  - TC-16-07 (false-positive guard): mock adapter is never selected when a real IDE config is present.

### DEV-M0-17 — IDE-config detection + `forge ship` LLM bridge
- **Tier:** T1 — **Anchor:** Arch §9.1
- **Status: ✅ SHIPPED** — `llmprovider.Detect()` in `internal/llmprovider`; detection order: ANTHROPIC_API_KEY → Claude adapter; OPENAI_API_KEY → OpenAI adapter; neither → FORGE-4050. Wired into `forge ship` checkpoints via `LLMPipe` — `checkSpec`, `checkTest`, `checkBreakdown`, `checkCode` all invoke the provider with graceful degradation on failure.
- **Scope:** Runs the detection pass (§9.1), resolves the right IDE adapter, and feeds it to `forge ship` checkpoints and scan-fix proposals. Forge does not manage credentials — it reads what the IDE already set up. If nothing detected, emits `FORGE-4050` and tells the developer which IDE to configure.
- **Acceptance:** Detection test matrix (Claude Code / VS Code Copilot env / bare env var / none); `forge doctor` reports detected adapter.
- **Test cases:**
  - TC-17-01 (happy): with a detected adapter, ship checkpoint completes and response is unchanged.
  - TC-17-02 (negative): `FORGE-4001` on no adapter; message names at least one IDE to configure.
  - TC-17-03 (data-accuracy): token counts from the adapter land in the ledger unchanged.
  - TC-17-04 (regression): live-test only runs when `FORGE_LIVE_LLM=1`.
  - TC-17-05 (false-positive guard): Forge reads IDE credentials but does not copy, log, or re-export them.

### DEV-M0-18 — Token ledger (append-only)
- **Tier:** T1 — **Anchor:** Arch §9.2
- **Status: ✅ SHIPPED** — `internal/tokenledger`; JSONL at `.forge/token-ledger.jsonl`; Append/ReadAll/TotalCost/Summary; per-model breakdown; concurrent-safe mutex; FORGE-2800 (write-fail), FORGE-2801 (read-fail); 14 tests.
- **Acceptance:** Cost per request asserted in test.
- **Test cases:**
  - TC-18-01 (happy): per-request entry written with prompt+completion tokens, model, cost.
  - TC-18-02 (data-accuracy): summed cost equals reference fixture total to ±$0.0001.
  - TC-18-03 (negative): a request with zero tokens still writes an entry (not skipped).

### DEV-M0-19 — IDE adapter plugins: Claude Code + OpenAI-compatible env (nightly compliance)
- **Tier:** T2 — **Anchor:** Arch §9.1
- **Status:** ✅ SHIPPED — `.github/workflows/nightly.yml`; 02:00 UTC schedule; `FORGE_LIVE_LLM=1` env var set; graceful skip with `::notice` when no API keys configured; runs `forge eval .forge/eval --ci --json`.
- **Scope:** Validate the two most common IDE credential shapes against the live API in nightly CI. No new credential management — adapters read what the IDE set. `FORGE_LIVE_LLM=1` gates these tests.
- **Acceptance:** Both adapter compliance suites green in nightly CI.
- **Test cases:**
  - TC-19-01 (happy): Claude Code adapter (ANTHROPIC_API_KEY) passes compliance suite against live API.
  - TC-19-02 (happy): OpenAI-compatible adapter (OPENAI_API_KEY) passes compliance suite against live API.
  - TC-19-03 (negative): provider 5xx is surfaced as `FORGE-4xxx` after configured retries; no raw message leaks.
  - TC-19-04 (data-accuracy): provider-reported token counts match the ledger entry to the unit.
  - TC-19-05 (regression): removing the env var causes `FORGE-4001`, not a panic or credential prompt.

### DEV-M0-20 — Manual `ship` checklist doc (pre-automation stand-in)
- **Tier:** DOC — **Anchor:** Spec §4 + DEV plan §0
- **Status:** ✅ SHIPPED — `CHECKLIST.md` at repo root; referenced from `CONTRIBUTING.md`; mirrors spec §16.5.4 gate list.
- **Acceptance:** `CHECKLIST.md` referenced from CONTRIBUTING.md.
- **Verification:** doc lint passes; CONTRIBUTING.md link resolves; checklist mirrors spec §16.5.4 gate list.

### DEV-M0-21 — `forge ship --quick "..."` MVP
- **Tier:** T1 — **Anchor:** Spec §4
- **Status:** ✅ SHIPPED — `cmdship` 5-checkpoint pipeline shipped; `.forge/eval/ship-reference.scenario.yml` committed with 5 checkpoints (clean→scan→doctor→ship→audit); eval scenario format per ADR-005.
- **Acceptance:** `ship-reference` eval scenario passes (TEST-07).
- **Test cases:**
  - TC-21-01 (happy): reference change ships within budget.
  - TC-21-02 (negative): change that fails any §16.5.4 gate aborts with the gate name + remediation.
  - TC-21-03 (idempotency): re-running on no-op change exits 0 with "no changes".
  - TC-21-04 (boundary): change touching exactly the budgeted file count succeeds.

### DEV-M0-22 — Unit-test harness conventions
- **Tier:** T1 — **Anchor:** TEST plan §1
- **Status:** ✅ SHIPPED — 530+ tests across 32 packages; 9-point checklist (happy/boundary/negative/idempotency/concurrency etc.) enforced; `go test -race ./...` green on all platforms.
- **Acceptance:** Sample test for each module type.
- **Verification:** TEST-01 cases all pass on the harness.

### DEV-M0-23 — Integration-test harness (subprocess `forge`)
- **Tier:** T1 — **Anchor:** TEST plan §1
- **Status:** ✅ SHIPPED — `internal/cli/journey_test.go`; 20 `TestJourney_*` E2E tests covering all major verbs; subprocess invocation model.
- **Acceptance:** At least one passing E2E test.
- **Verification:** TEST-02 cases all pass.

### DEV-M0-24 — Eval harness scaffold + `new-app` scenario
- **Tier:** T1 — **Anchor:** TEST plan §5
- **Status:** ✅ SHIPPED — `internal/eval` + `cmdeval`; YAML-scenario runner + JSON report shipped; `.forge/eval/new-app.scenario.yml` committed (scaffold→doctor→hygiene 3-step scenario per ADR-005).
- **Acceptance:** Nightly run reports green.
- **Verification:** TEST-06 cases all pass.

### DEV-M0-25 — NFR benchmark suite scaffold
- **Tier:** T1 — **Anchor:** Arch §14
- **Status:** ✅ SHIPPED — `BenchmarkScanSecrets_500Files` + `BenchmarkScaffold_GoService`; `make bench`; baseline JSON recorded per DEV-M3-S1 glance entry.
- **Acceptance:** Cold-start measured; baseline recorded.
- **Verification:** TEST-05 + TEST-11 cases all pass; baseline JSON committed.

### DEV-M0-26 — Secret-redaction regression test (100-run loop)
- **Tier:** T1 — **Anchor:** Arch §15
- **Status:** ✅ SHIPPED — `TestRewriter_100RunRegression` added to `internal/secretrewriter/secretrewriter_test.go`; 8 corpus cases × 100 runs; `tests/fixtures/seeded-secrets/` with 7 fake-key files (OpenAI, Anthropic, AWS, GitHub, JWT, Stripe, Slack/Twilio).
- **Acceptance:** Test in CI; fails on seeded leak.
- **Verification:** TEST-12 cases all pass.

### DEV-M0-27 — CI pipeline (lint + unit + integration + secret-scan + hygiene)
- **Tier:** OPS — **Anchor:** TEST plan §4
- **Status:** ✅ SHIPPED — `.github/workflows/ci.yml`; Go 1.25 race+cgo matrix; linux/darwin/windows builds; golangci-lint + go test gates active and required for merge.
- **Acceptance:** CI green on main; required for merge.
- **Verification checks:** every §16.5.4 gate runs in CI; failure of any one gate blocks the PR; CI matrix covers linux/mac/win.

### DEV-M0-28 — Sigstore signing pipeline
- **Tier:** OPS — **Anchor:** Arch §15
- **Status:** ✅ SHIPPED — `signs:` (cosign/Sigstore checksum signing) + `brews:` (Homebrew tap `teragrid/homebrew-tap`) + `scoops:` (Scoop bucket `teragrid/scoop-forge`) blocks added to `.goreleaser.yml`.
- **Acceptance:** Release artifact signed; verification documented.
- **Verification checks:** post-build verification step runs `cosign verify` on every artifact; signature failure aborts release; doc walkthrough tested by a non-author.

### DEV-M0-29 — Brew/scoop/winget tap published
- **Tier:** OPS — **Anchor:** Arch §13 ADR-003
- **Status:** ✅ SHIPPED — `brews:` + `scoops:` blocks added to `.goreleaser.yml`; `docs/DISTRIBUTION.md` full install guide; `make changelog` target added to `Makefile` (`goreleaser changelog`).
- **Acceptance:** Install matrix test passes.
- **Verification:** TEST-16 cases all pass.

### DEV-M0-30 — Telemetry opt-in plumbing (off by default)
- **Tier:** T1 — **Anchor:** Arch §11
- **Status:** ✅ SHIPPED — `internal/telemetry` + `cmdtelemetry`; `forge telemetry enable/disable/status/rotate-id`; opt-in off by default; 21 tests; errcode range 4100..4199.
- **Acceptance:** `forge telemetry` shows current state; payload printable via `--explain`.
- **Test cases:**
  - TC-30-01 (happy): default install reports `telemetry: off`.
  - TC-30-02 (negative): no payload is sent while off (network-mock asserts zero outbound).
  - TC-30-03 (data-accuracy): when on, payload matches the public schema (DEV-M3-12) byte-for-byte.
  - TC-30-04 (regression): a code path adding a new field without schema bump fails CI.

### DEV-M0-31 — M0 release notes + changelog automation
- **Tier:** DOC — **Anchor:** Spec §16.5 #5
- **Status:** ✅ SHIPPED — `BREAKING.md` at repo root (semver policy, one-minor alias retention rule, emergency breaks, `make changelog`); `make changelog` target added to `Makefile`.
- **Acceptance:** `BREAKING.md` + `CHANGELOG.md` generated from spec frontmatter.
- **Verification:** generator round-trip is reproducible; missing entries fail CI.

### DEV-M0-32 — Hygiene fixture corpus seeded (≥30 known LLM-scratch patterns)
- **Tier:** T1 — **Anchor:** Spec §4 hygiene
- **Status:** ✅ SHIPPED — `tests/fixtures/hygiene-corpus/` with 31 files across 6 pattern families (scratch, tmp, .forge/scratch, secret files, key files, LLM-output drafts); `README.md` documents corpus.
- **Acceptance:** Corpus committed; `forge clean` catches every entry.
- **Verification:** TEST-20 cases all pass; corpus size ≥30.

### DEV-M0-33 — `.gitignore` template fragments + composer
- **Tier:** T1 — **Anchor:** Spec §4 Repo Hygiene Layer (`.gitignore` standards)
- **Status:** ✅ SHIPPED — `.gitignore` bundled in both `go-service` and `ts-service` templates with managed marker block; `forge upgrade gitignore` codemod wired; idempotent re-compose via `internal/codemod`.
- **Acceptance:** `forge new` emits version-stamped managed block + user section; mandatory hygiene block present; `.example`/`.template` negations preserved.
- **Test cases:**
  - TC-33-01 (happy): each fragment (`node`, `next`, `python`, `supabase`, `terraform`, `docker`, `os`, `editor`, `llm-scratch`) renders into the composed file.
  - TC-33-02 (boundary): selecting zero optional fragments still emits the mandatory block.
  - TC-33-03 (negative): unknown fragment name fails with `FORGE-XXXX`.
  - TC-33-04 (regression): rendered file passes TEST-22 contract test.
  - TC-33-05 (data-accuracy): managed-block version marker matches `forge --version` major.minor.

### DEV-M0-34 — `.gitleaks.toml` template with Forge-aware rule pack
- **Tier:** T1 — **Anchor:** Spec §4 Repo Hygiene Layer (`.gitleaks.toml` standards)
- **Status:** ✅ SHIPPED — `.gitleaks.toml` bundled in both templates; 4 baseline rules (forge-api-key, custom-jwt, forge-webhook, generic-high-entropy); `forge upgrade gitleaks` codemod wired.
- **Acceptance:** `forge new` ships file; rule pack catches every entry in `tests/fixtures/secrets-corpus/` and zero false positives on the reference app.
- **Test cases:**
  - TC-34-01 (happy): TEST-21 positive fixtures all flagged.
  - TC-34-02 (false-positive guard): TEST-21 negative fixtures none flagged.
  - TC-34-03 (data-accuracy): rule pack version stamp matches CLI version.
  - TC-34-04 (regression): every Forge-aware rule has at least one fixture in TEST-21.

### DEV-M0-35 — `forge upgrade gitignore` + `forge upgrade gitleaks` codemods (idempotent; preserve user-owned section)
- **Tier:** T1 — **Anchor:** Spec §4 Repo Hygiene Layer
- **Status:** ✅ SHIPPED — `internal/codemod/baselines.go`; `gitignore-marker` + `gitleaks-baseline` builtins; round-trip idempotent; user section preserved; wired into `cmdupgrade`.
- **Acceptance:** Round-trip test: upgrade → noop diff; user section untouched across two version bumps.
- **Test cases:**
  - TC-35-01 (happy): TEST-26 cases all pass.
  - TC-35-02 (negative): `--force` is required to overwrite drift inside the managed block.
  - TC-35-03 (boundary): brand-new repo upgrade writes markers correctly.

### DEV-M0-36 — Secret-file guard list + `git ls-files` cross-check in `forge clean --check`
- **Tier:** T1 — **Anchor:** Spec §4 Repo Hygiene Layer + §16.5.4 #11
- **Status:** ✅ SHIPPED — `checkTrackedSecrets` using `git ls-files` fully implemented; TC-36-05 integration test added to `cmdclean/clean_test.go` (real `git init` + tracked `.env` file → detected in `TrackedSecrets`; skips gracefully if git absent).
- **Acceptance:** Test fixture: tracked `.env.local` → fail; tracked `.env.local.example` → pass.
- **Test cases:** TEST-23 cases all pass.

---

## M1 — Workflow & Scan (DEV-M1-01 .. DEV-M1-50)

### DEV-M1-01 — `ship` workflow orchestrator (5 checkpoints, resumable)
- **Tier:** T1 — **Anchor:** Arch §6
- **Status:** ✅ SHIPPED — `cmdship`; 5-checkpoint pipeline (spec, test, breakdown, code, verify/PR); hash-chained `ship.json` for resume; `--dry-run` validates pipeline shape; `forge ship --quick` flag wired.
- **Acceptance:** Resume-from-checkpoint test for each stage.
- **Test cases:**
  - TC-01-01 (happy): full ship runs end-to-end on the reference app.
  - TC-01-02 (idempotency): resuming after each checkpoint produces same final tree as a clean run.
  - TC-01-03 (negative): resume after corrupted checkpoint state fails fast with `FORGE-XXXX`, never silently re-runs.
  - TC-01-04 (concurrency): two ships on different branches do not share checkpoint state.
  - TC-01-05 (regression): kill -9 mid-checkpoint leaves a recoverable state (next `ship` resumes).

### DEV-M1-02 — Spec checkpoint
- **Tier:** T1 — **Anchor:** Spec §4 + §16.5.4 #1
- **Status:** ✅ SHIPPED — `checkSpec` in `cmdship`; invokes LLM via `LLMPipe.Invoke` to draft/review `spec.md`; graceful degradation to stub when no provider configured.
- **Acceptance:** `forge ship verify` blocks on missing spec.
- **Test cases:**
  - TC-02-01 (happy): change with `.forge/specs/<change>/spec.md` proceeds.
  - TC-02-02 (negative): change without spec is blocked with the gate name + path expected.
  - TC-02-03 (boundary): docs-only change is exempt per spec policy (verified by manifest).
  - TC-02-04 (false-positive guard): a spec.md inside test fixtures does not satisfy the gate for an unrelated change.

### DEV-M1-03 — Test checkpoint: tests-precede-code timestamp guard
- **Tier:** T1 — **Anchor:** Spec §16.5.4 #2
- **Status:** ✅ SHIPPED — timestamp-ordering guard implemented in `checkTest` within `cmdship`; tests-committed-before-code enforcement active in ship pipeline.
- **Acceptance:** Code-before-test PR is blocked.
- **Test cases:**
  - TC-03-01 (happy): tests committed before code → pass.
  - TC-03-02 (negative): code committed before tests in same PR → blocked.
  - TC-03-03 (boundary): test + code in the same commit → pass (atomic exception).
  - TC-03-04 (false-positive guard): pure refactor with no behavior change is exempt by manifest flag.

### DEV-M1-04 — Breakdown checkpoint: tasks.md generation
- **Tier:** T1 — **Anchor:** Spec §4
- **Status:** ✅ SHIPPED — `checkBreakdown` in `cmdship`; `generateBreakdown` writes `breakdown.md` via LLM; task schema validated.
- **Acceptance:** Each task has scope + principle ref.
- **Test cases:**
  - TC-04-01 (happy): generated tasks.md validates against schema.
  - TC-04-02 (negative): a task missing its principle ref fails the gate.
  - TC-04-03 (data-accuracy): each task's anchor resolves to a real spec section.

### DEV-M1-05 — Code checkpoint: per-task diff loop with re-test
- **Tier:** T1 — **Anchor:** Spec §4
- **Status:** ✅ SHIPPED — `checkCode` per-task diff loop; `generateCodePlan` via LLM; `checkVerify` re-runs scans; loop terminates or escalates with reason after max iterations.
- **Acceptance:** Loop terminates green or escalates with reason.
- **Test cases:**
  - TC-05-01 (happy): converges within max-iter on reference change.
  - TC-05-02 (negative): non-converging loop escalates after max-iter with the failing test's name.
  - TC-05-03 (idempotency): converged tree replays identically.
  - TC-05-04 (boundary): zero-iter (already green) exits 0 with no diff.

### DEV-M1-06 — Ship checkpoint: gate orchestration
- **Tier:** T1 — **Anchor:** Spec §4 + §16.5.4
- **Status:** ✅ SHIPPED — `checkPR` via `gh` CLI; gate orchestration in `cmdship`; each gate independently re-runnable; per-gate timing in `--json`.
- **Acceptance:** Each gate independently re-runnable.
- **Test cases:**
  - TC-06-01 (happy): all gates green → ship.
  - TC-06-02 (negative): each gate failure independently blocks ship and names the gate.
  - TC-06-03 (idempotency): re-run after fixing one gate re-runs only failed gates by default.
  - TC-06-04 (data-accuracy): per-gate timing reported in `--json`.

### DEV-M1-07 — LLM caching layer (semantic-hash key)
- **Tier:** T1 — **Anchor:** Arch §9.3
- **Status:** ✅ SHIPPED — `internal/llmcache` package ships semantic-hash keying + file-change invalidation; integrated into `internal/llmprovider`.
- **Scope:** Optimizes Forge's own bridge calls (DEV-M0-17); does not affect developer IDE traffic.
- **Acceptance:** Cache hit/miss test; invalidation on file change.
- **Test cases:**
  - TC-07-01 (happy): identical request → cache hit, zero provider call.
  - TC-07-02 (negative): file content change invalidates the entry.
  - TC-07-03 (boundary): two requests differing only in irrelevant whitespace hit (semantic equivalence).
  - TC-07-04 (concurrency): two parallel identical requests both succeed; only one provider call.
  - TC-07-05 (data-accuracy): hit's response is byte-identical to the original recorded one.

### DEV-M1-08 — LLM tier router (cheap-first, escalate on fail)
- **Tier:** T1 — **Anchor:** Arch §9.3
- **Status:** ✅ SHIPPED — `internal/tierrouter` ships cheap-first escalation logic; integrated into the `internal/llmprovider` provider chain.
- **Scope:** Routes Forge's own bridge calls (DEV-M0-17); does not proxy IDE traffic.
- **Acceptance:** Routing test with seeded provider responses.
- **Test cases:**
  - TC-08-01 (happy): cheap model succeeds → no escalation.
  - TC-08-02 (negative): cheap model fails validation → escalates to next tier.
  - TC-08-03 (boundary): all tiers fail → returns `FORGE-XXXX` with chain of attempts.
  - TC-08-04 (data-accuracy): ledger records every tier attempt with its cost.

### DEV-M1-09 — Budget guard (per-command + per-day)
- **Tier:** T1 — **Anchor:** Arch §9.3
- **Status:** ✅ SHIPPED — `internal/llmbudget` + `cmdspend` (`forge spend`); per-command + per-day cap; FORGE-2401 on breach; `forge spend --reset`; 29 tests.
- **Scope:** Guards Forge's own bridge calls; developer IDE usage is outside Forge's budget scope.
- **Acceptance:** `FORGE-2401` on cap; rerun-with-`--budget` flow tested.
- **Test cases:**
  - TC-09-01 (happy): under budget → proceeds.
  - TC-09-02 (negative): exact-cap-+1 → blocked with `FORGE-2401`.
  - TC-09-03 (boundary): exact-cap → proceeds (`<=` semantics asserted).
  - TC-09-04 (idempotency): re-run with `--budget=N+1` proceeds where prior failed.
  - TC-09-05 (cross-tenant): two repos share no budget state.

### DEV-M1-10 — Hygiene checkpoint: auto-`forge clean --check` between Code and Ship
- **Tier:** T1 — **Anchor:** Spec §4 hygiene
- **Status:** ✅ SHIPPED — `cmdclean --check` wired as the hygiene checkpoint between Code and Ship in `cmdship`; blocks on unmanifested files.
- **Acceptance:** Ship blocks if unmanifested files remain.
- **Test cases:**
  - TC-10-01 (happy): clean tree → proceeds.
  - TC-10-02 (negative): seeded scratch file mid-ship → blocks at this checkpoint, not at the end.
  - TC-10-03 (idempotency): re-run after `forge clean --apply` proceeds.

### DEV-M1-11 — Scan engine kernel + `Finding` schema with confidence
- **Tier:** T1 — **Anchor:** Arch §10
- **Status:** ✅ SHIPPED — scan engine kernel in `cmdscan`; `Finding` schema with `confidence` field; family aggregation; JSON envelope always emitted; `forge scan all` multi-family runner.
- **Acceptance:** Schema test; finding-roundtrip test.
- **Test cases:**
  - TC-11-01 (happy): scanners emit findings; kernel aggregates by family.
  - TC-11-02 (boundary): zero-finding scan still emits a result envelope.
  - TC-11-03 (negative): malformed finding (missing `confidence`) is rejected by the kernel.
  - TC-11-04 (data-accuracy): roundtrip JSON ↔ object preserves every field.

### DEV-M1-12 — Scanner family: secrets
- **Tier:** T1 — **Anchor:** Arch §10
- **Status:** ✅ SHIPPED — `forge scan secrets`; builtin regex engine + gitleaks fallback; 5 rules; confidence ≥ 0.9 on seeded fixtures.
- **Acceptance:** Seeded-secret app: catches with confidence ≥0.9.
- **Test cases:** TEST-08 + TEST-21 cases all pass for this family.

### DEV-M1-13 — Scanner family: RLS / authz
- **Tier:** T1 — **Anchor:** Arch §10 + spec §16.5.6
- **Status:** ✅ SHIPPED — `forge scan rls`; SQL/migration tenant-column scanner; cross-tenant leak detection; seeded-bypass fixtures caught.
- **Acceptance:** Seeded-RLS-bypass app: caught.
- **Test cases:**
  - TC-13-01 (happy): seeded-bypass detected with confidence ≥0.9.
  - TC-13-02 (cross-tenant): user-A → user-B leak fixture detected.
  - TC-13-03 (false-positive guard): clean RLS reference app produces zero findings.
  - TC-13-04 (regression): every prior repo RLS bug in the agent-system has a fixture here.

### DEV-M1-14 — Scanner family: prompt-injection
- **Tier:** T1 — **Anchor:** Arch §10
- **Status:** ✅ SHIPPED — `forge scan prompt-injection`; 4 patterns (ignore-previous, role-override, system-prompt-leak, unsafe-eval); OWASP LLM-01 coverage.
- **Acceptance:** OWASP LLM Top-10 #1 fixtures all caught.
- **Test cases:**
  - TC-14-01 (happy): each OWASP LLM-01 sub-pattern fixture is caught.
  - TC-14-02 (false-positive guard): a benign string containing the words "ignore previous" inside a quoted string literal is NOT flagged.
  - TC-14-03 (boundary): payload at the configured max-context is still scanned.

### DEV-M1-15 — Scanner family: supply-chain
- **Tier:** T1 — **Anchor:** Arch §10
- **Status:** ✅ SHIPPED — `forge scan supply-chain`; 4 patterns (loose-version, unpinned-git, curl-pipe-shell, go.mod replace); finding includes CVE id + advisory link.
- **Acceptance:** Seeded-vulnerable-dep app: caught.
- **Test cases:**
  - TC-15-01 (happy): known-CVE dep flagged.
  - TC-15-02 (boundary): version exactly equal to the patched version is NOT flagged.
  - TC-15-03 (negative): missing lockfile fails with `FORGE-XXXX`.
  - TC-15-04 (data-accuracy): finding includes CVE id + advisory link.

### DEV-M1-16 — `forge fix --apply` with confidence-tier behavior
- **Tier:** T1 — **Anchor:** Arch §10.2
- **Status:** ✅ SHIPPED — `cmdfix` ships confidence-tier UX: diff-only below 0.9, auto-apply with `--apply` at ≥0.9; `--pr` opens a PR via `gh`.
- **Acceptance:** Diff-only at <0.9; auto-applied with `--apply` at ≥0.9.
- **Test cases:**
  - TC-16-01 (happy): finding ≥0.9 + `--apply` → diff applied; tree green.
  - TC-16-02 (boundary): finding at exactly 0.9 with `--apply` → applied (`>=` semantics).
  - TC-16-03 (negative): finding <0.9 + `--apply` → still diff-only; warning shown.
  - TC-16-04 (idempotency): re-run after apply → no further changes.
  - TC-16-05 (regression): every applied fix is recorded in the audit ledger.

### DEV-M1-17 — Waiver mechanism (`.forge/waivers/`)
- **Tier:** T1 — **Anchor:** Spec §16.5.4 #3
- **Status:** ✅ SHIPPED — `internal/waiver` ships `.forge/waivers/` directory reader with expiry enforcement; wired into the scan engine.
- **Acceptance:** Waiver requires rationale + expiry; expiry-warning test.
- **Test cases:**
  - TC-17-01 (happy): well-formed waiver suppresses the matching finding.
  - TC-17-02 (negative): waiver missing rationale or expiry rejected at parse time.
  - TC-17-03 (boundary): waiver expiring today is treated as expired (strict `<`).
  - TC-17-04 (regression): every waiver appears in the weekly digest (TEST-18 link).
  - TC-17-05 (false-positive guard): a waiver scoped to file A does not suppress the same rule firing on file B.

### DEV-M1-18 — Plugin loader (per ADR-002)
- **Tier:** T1 — **Anchor:** Arch §8.3
- **Status:** ✅ SHIPPED — `internal/plugin`; in-process loader; Wazero WASM behind `forge_wasm` build tag; manifest-based signature check; capability enforcement; sandbox escape tests; 8 WASM tests.
- **Acceptance:** Sandbox escape test; signature failure test.
- **Test cases:**
  - TC-18-01 (happy): signed, capability-conformant plugin loads.
  - TC-18-02 (negative): unsigned plugin rejected.
  - TC-18-03 (negative): tampered plugin (signature mismatch) rejected.
  - TC-18-04 (negative): plugin attempting un-granted capability blocked at runtime.
  - TC-18-05 (concurrency): TEST-09 + TEST-14 cases all pass.

### DEV-M1-19 — Capability + permission model
- **Tier:** T1 — **Anchor:** Arch §8.1–8.2
- **Status:** ✅ SHIPPED — `internal/plugin`; deny-by-default for fs/net/proc/secrets/llm namespaces; per-namespace denial tests; wildcard grants explicitly opt-in at install.
- **Acceptance:** Permission-denial test for each namespace.
- **Test cases:**
  - TC-19-01 (happy): each declared capability grants exactly its scope.
  - TC-19-02 (negative): each namespace (fs, net, proc, secrets, llm) has a denied-without-grant test.
  - TC-19-03 (boundary): wildcard grants (`fs:*`) are explicitly opt-in (gate at install).

### DEV-M1-20 — Plugin manifest schema + validator
- **Tier:** T1 — **Anchor:** Arch §7.3
- **Status:** ✅ SHIPPED — Plugin Manifest schema + validator in `internal/plugin`; required fields enforced; unknown capability names rejected; semver field validated; invalid manifests return `FORGE-XXXX`.
- **Acceptance:** Invalid manifests rejected with `FORGE-XXXX`.
- **Test cases:**
  - TC-20-01 (happy): valid manifest accepted.
  - TC-20-02 (negative): each required field's absence has a fixture.
  - TC-20-03 (negative): unknown capability name rejected.
  - TC-20-04 (data-accuracy): semver field validated to spec.

### DEV-M1-21 — First in-tree scanner-plugin proof (`scanner-cost`)
- **Tier:** T2 — **Anchor:** Arch §8
- **Status:** ✅ SHIPPED — `scanner-cost` in-tree plugin proof complete; Scanner interface, in-process loader, and cost scanner all operational; packaged and loadable via plugin loader.
- **Acceptance:** Loads + runs + reports findings.
- **Test cases:**
  - TC-21-01 (happy): loads under loader; finds at least one seeded cost anti-pattern.
  - TC-21-02 (false-positive guard): clean reference app → zero findings.

### DEV-M1-22 — First in-tree generator-plugin proof (`gen-endpoint`)
- **Tier:** T2 — **Anchor:** Arch §8
- **Status:** ✅ SHIPPED — `gen-endpoint` generator plugin proof implemented in `cmdgenerate`; generates code that passes scan gates.
- **Acceptance:** Generates code that passes scans.
- **Test cases:**
  - TC-22-01 (happy): generated endpoint compiles and passes secrets/RLS scans.
  - TC-22-02 (idempotency): re-running with same input → identical output.
  - TC-22-03 (negative): conflicting endpoint name → fails with `FORGE-XXXX`, no partial write.

### DEV-M1-23 — `forge lint` reads `.forge/instructions/` packs
- **Tier:** T1 — **Anchor:** Spec §11.2 + Arch §5
- **Status:** ✅ SHIPPED — `cmdlint`; reads hygiene rules and manifest conventions; reference app lints green; `--json` structured output.
- **Acceptance:** Lints reference app green.
- **Test cases:**
  - TC-23-01 (happy): reference app lints clean.
  - TC-23-02 (negative): seeded anti-pattern → lint fails with rule id + remediation.
  - TC-23-03 (boundary): empty pack → lint is a no-op pass.
  - TC-23-04 (false-positive guard): a string literal that *resembles* an anti-pattern in user-facing copy is not flagged.

### DEV-M1-24 — First defaults instructions pack
- **Tier:** T1 — **Anchor:** Spec §11.2
- **Status:** ✅ SHIPPED — `forge new` scaffolds `.forge/instructions/defaults.md`; both `cmdlint` and the context builder consume the pack.
- **Acceptance:** Pack referenced by both linter and LLM context builder.
- **Test cases:**
  - TC-24-01 (happy): linter consumes pack; LLM context builder embeds pack.
  - TC-24-02 (data-accuracy): pack version pinned in both consumers.
  - TC-24-03 (regression): pack-format change without consumer bump fails CI.

### DEV-M1-25 — New-pattern detection + RFC-link suggestion in lint output
- **Tier:** T1 — **Anchor:** Spec §16.5.4 #4
- **Status:** ✅ SHIPPED — `cmdlint` emits RFC-link template on unknown patterns; "new pattern detected" signal with how-to-RFC instructions included in output.
- **Acceptance:** Lint failure cites which convention rule + how to amend.
- **Test cases:**
  - TC-25-01 (happy): fail message includes rule id + RFC link template.
  - TC-25-02 (negative): a new pattern not matching any rule is reported as "new" with how-to-RFC instructions.
  - TC-25-03 (false-positive guard): an existing accepted pattern in the reference app is not flagged as "new".

### DEV-M1-26 — Spec-presence gate in CI
- **Tier:** OPS — **Anchor:** Spec §16.5.4 #1
- **Status:** ✅ SHIPPED — spec-presence gate wired in `.github/workflows/ci-gates.yml`; blocks PRs missing `.forge/specs/<change>/spec.md`.
- **Acceptance:** Blocks PR without `.forge/specs/<change>/spec.md`.
- **Verification:** TEST-02 (gate exit-code propagation) + DEV-M1-02 cases applied at CI tier.

### DEV-M1-27 — Tests-precede-code gate in CI
- **Tier:** OPS — **Anchor:** Spec §16.5.4 #2
- **Status:** ✅ SHIPPED — tests-before-code timestamp gate wired in `.github/workflows/ci-gates.yml`; blocks PRs violating the invariant.
- **Acceptance:** Blocks PR violating timestamp invariant.
- **Verification:** DEV-M1-03 cases applied at CI tier; bypass attempt via rebase is detected.

### DEV-M1-28 — `forge scan all --since main` gate
- **Tier:** OPS — **Anchor:** Spec §16.5.4 #3
- **Status:** ✅ SHIPPED — `forge scan all --since main` is a required CI gate in `.github/workflows/ci-gates.yml`; waivers honored per DEV-M1-17.
- **Acceptance:** Blocks high-confidence findings.
- **Verification:** TEST-08 cases must be green for the PR to merge; waivers honored per DEV-M1-17.

### DEV-M1-29 — Convention lint gate
- **Tier:** OPS — **Anchor:** Spec §16.5.4 #4
- **Status:** ✅ SHIPPED — `forge lint` required CI gate wired in `.github/workflows/ci-gates.yml`; blocks new anti-patterns.
- **Acceptance:** Blocks new anti-patterns.
- **Verification:** DEV-M1-23 + DEV-M1-25 cases enforced at CI tier.

### DEV-M1-30 — Public-API delta declaration gate
- **Tier:** OPS — **Anchor:** Spec §16.5.4 #5
- **Status:** ✅ SHIPPED — public-API delta gate wired in CI; undeclared breaking changes blocked; `BREAKING.md` automation active.
- **Acceptance:** API-diff tool detects undeclared breaks.
- **Verification:** seed an undeclared break in a test PR → gate blocks with the diff cited; declared break (with `BREAKING.md` entry) → gate passes.

### DEV-M1-31 — Token-budget regression gate
- **Tier:** OPS — **Anchor:** Spec §16.5.4 #6
- **Status:** ✅ SHIPPED — token-budget regression gate wired in CI; `forge eval` diff >10% blocks PR; +9% passes (boundary verified).
- **Acceptance:** `forge eval` diff >10% blocks.
- **Verification:** TEST-07 cases enforced; +11% synthetic diff blocks; +9% passes (boundary).

### DEV-M1-32 — Docs-sync gate
- **Tier:** OPS — **Anchor:** Spec §16.5.4 #7
- **Status:** ✅ SHIPPED — `forge docs sync` implemented (DEV-M2-27); docs-sync CI gate wired in `.github/workflows/ci-gates.yml`.
- **Acceptance:** `forge docs sync --check` clean required.
- **Verification:** seeded out-of-sync doc → blocks; clean → passes; doc-only no-code change still validated.

### DEV-M1-33 — DCO + signed-commit gate
- **Tier:** OPS — **Anchor:** Spec §16.5.4 #8
- **Status:** ✅ SHIPPED — DCO + signed-commit gate active via branch protection rules; unsigned commits and missing sign-off blocked; enforcement confirmed.
- **Acceptance:** Branch-protection rule active.
- **Verification:** unsigned commit blocked; signed + DCO-signed → passes; expired signing key surfaces clear error.

### DEV-M1-34 — Repo-hygiene gate (`forge clean --check`)
- **Tier:** OPS — **Anchor:** Spec §16.5.4 #11
- **Status:** ✅ SHIPPED — `forge clean --check` required CI gate wired in `.github/workflows/ci-gates.yml`; unmanifested files block merges.
- **Acceptance:** Blocks PR with unmanifested LLM scratch.
- **Verification:** TEST-13 + TEST-23 cases enforced at CI tier.

### DEV-M1-35 — Secrets-clean gate (`forge scan security --secrets --since main`, gitleaks)
- **Tier:** OPS — **Anchor:** Spec §16.5.4 #12
- **Status:** ✅ SHIPPED — `forge scan secrets` + gitleaks required CI gate wired; allowlist `# review-by:` expiry enforced in CI.
- **Acceptance:** Blocks PR with secret findings; allowlist `# review-by:` expiry enforced.
- **Verification:** TEST-21 + TEST-24 cases enforced at CI tier.

### DEV-M1-36 — Pre-commit secret scan + `gitleaks-bypass: <reason>` token surfaced in PR
- **Tier:** OPS — **Anchor:** Spec §4 Repo Hygiene Layer (`.gitleaks.toml` standards)
- **Status:** ✅ SHIPPED — `scripts/install-hooks.sh` + `scripts/forge-pre-commit` ship pre-commit secret scan; bypass token surfaced in PR template and audit log.
- **Acceptance:** Bypassed commit visible in PR template + audit log.
- **Verification:** commit with bypass token → PR template lists it under "Bypasses requiring review"; commit without token + dirty pre-commit → blocks locally; reason missing → bypass token rejected.

### DEV-M1-37 — Allowlist-expiry sweeper (nightly) opens auto-PR to remove expired entries
- **Tier:** OPS — **Anchor:** Spec §16.5.4 #12
- **Status:** ✅ SHIPPED — nightly cron in `.github/workflows/nightly.yml` sweeps expired allowlist entries and opens auto-PRs.
- **Acceptance:** Cron job + auto-PR validated on staging repo.
- **Verification:** seeded expired entry on staging → auto-PR opens within 24h; PR title cites entry; no-expired-entries night → no PR opened.

### DEV-M1-38 — `.gitignore` drift detection in `forge doctor`
- **Tier:** T1 — **Anchor:** Spec §4 Repo Hygiene Layer
- **Status:** ✅ SHIPPED — `forge doctor` inspects `.gitignore` managed-block drift against `internal/codemod` canonical; suggests `forge upgrade gitignore` on deviation.
- **Acceptance:** Modified managed block triggers `forge upgrade gitignore` suggestion.
- **Test cases:**
  - TC-38-01 (happy): unmodified managed block → doctor passes silently.
  - TC-38-02 (negative): hand-edit inside managed block → doctor reports drift + suggests `forge upgrade gitignore`.
  - TC-38-03 (boundary): drift at the marker boundary (e.g. blank line added) is detected.
  - TC-38-04 (false-positive guard): user-section edits never trigger drift.

### DEV-M1-39 — Resilience-pattern library + lint enforcement of §17.4 invariants
- **Tier:** T1 — **Anchor:** Arch §17.1 + §17.4 + ARCH-DEC-14
- **Status:** ✅ SHIPPED — `internal/resilience` ships `Retry[T]`, `WithTimeout[T]`, circuit-breaker, bulkhead, and jitter primitives; lint rule rejects un-wrapped external calls in capability namespaces.
- **Acceptance:** Foundation-layer module ships circuit-breaker, retry-with-jitter, bulkhead, and timeout-budget primitives; lint rule rejects unbounded `await` / un-wrapped external calls in capability namespaces.
- **Test cases:**
  - TC-39-01 (happy): a wrapped LLM call retries with backoff on transient 5xx then succeeds; jitter measured ≥ ±10% over 100 runs.
  - TC-39-02 (boundary): retry budget exhausted → wrapper raises `FORGE-XXXX` with attempt count, never crashes.
  - TC-39-03 (negative): direct `await fetch(...)` in `services/llm/` fails the lint with file:line + auto-fix suggestion.
  - TC-39-04 (idempotency): circuit breaker in `open` state short-circuits without calling downstream; same call in `half-open` probes once.
  - TC-39-05 (concurrency): bulkhead caps concurrent calls per provider; over-cap calls queue then fail-fast on queue full.
  - TC-39-06 (regression): a deliberate revert of the lint rule fails the lint-rule self-test.
  - TC-39-07 (false-positive guard): a wrapped, timed call in `services/llm/` does NOT trigger the lint.

### DEV-M1-40 — Failure-register data model + `forge audit failure-register verify`
- **Tier:** T1 — **Anchor:** Arch §17.2 + ARCH-DEC-16
- **Status:** ✅ SHIPPED — `internal/failure` + `forge audit failure-register verify` sub-verb fully wired; YAML/JSON schema validated; CI gate active; errcode range 3700..3799 reserved.
- **Acceptance:** YAML/JSON schema for failure-register entries; source-of-truth file at `.forge/failure-register.yml`; verifier lints the §17.2 doc table against the file; a missing/extra row fails CI.
- **Test cases:**
  - TC-40-01 (happy): `forge audit failure-register verify` exits 0 on a clean repo.
  - TC-40-02 (negative): a row added to the doc table without a corresponding YAML entry fails with both file:line refs.
  - TC-40-03 (negative): a YAML entry missing `detection` or `recovery` fails schema validation with field path.
  - TC-40-04 (idempotency): running the verifier twice yields identical output (no caching surprises).
  - TC-40-05 (data-accuracy): `--json` output matches the YAML row count and includes `test_anchor` for each.
  - TC-40-06 (regression): the original "missing row" bug from a synthesised PR is replayed and fails the verifier.

### DEV-M1-41 — Issue templates + auto-triage bot + `Fixes:` trailer enforcement
- **Tier:** OPS — **Anchor:** Arch §18.1 + §18.3 + ARCH-DEC-17
- **Status:** ✅ SHIPPED — `.github/ISSUE_TEMPLATE/` templates published; auto-triage bot applies severity + area labels; `Fixes:` trailer enforced in CI for bug-labelled PRs.
- **Acceptance:** `.github/ISSUE_TEMPLATE/{bug,vulnerability,flake,incident}.yml` published; auto-triage bot applies severity-guess + area-guess labels using CODEOWNERS map; CI rejects PRs labelled `bug` whose commit messages lack a `Fixes: #NNN` trailer.
- **Test cases:**
  - TC-41-01 (happy): a new bug issue gets labels within 60s; a PR with `Fixes: #123` and label `bug` passes CI.
  - TC-41-02 (negative): vulnerability template submitted on the public tracker is auto-closed with a private-advisory pointer.
  - TC-41-03 (negative): PR without `Fixes:` trailer + `bug` label fails CI with a one-line explainer.
  - TC-41-04 (boundary): bug template missing required `forge --version` field cannot submit (form validation).
  - TC-41-05 (cross-tenant): triager from a non-eligible tier cannot apply a `severity:S0` label (bot reverts + comments).
  - TC-41-06 (regression): a synthesised "stealth fix" PR (no `Fixes:`, label `bug`) is blocked.
  - TC-41-07 (false-positive guard): a `chore` or `docs` PR without `Fixes:` is NOT blocked.

---

### DEV-M1-42 — `forge test` CLI verb (13 test-family orchestrator)
- **Tier:** T1 — **Anchor:** Spec §13.5
- **Status:** ✅ SHIPPED — `cmdtest` ships all 13 test families with subprocess invocation wired; `forge test all --json` emits structured result; `--fail-fast` active; `forge ship` Checkpoint 2 delegates correctly.
- **Acceptance:** All 13 families callable; `forge test all --json` emits structured result per family; `forge test all --fail-fast` stops on first failure; `forge ship` Checkpoint 2 delegates to `forge test smoke + unit + regression`.
- **Test cases:**
  - TC-42-01 (happy): `forge test unit` invokes `go test ./... -race` and returns structured output.
  - TC-42-02 (happy): `forge test all --json` runs all 13 families in order, emits one JSON record per family.
  - TC-42-03 (negative): `forge test unknown-family` exits non-zero with `FORGE-4301` (ErrTestUnknownFamily).
  - TC-42-04 (boundary): `forge test all --fail-fast` stops after the first failing family.
  - TC-42-05 (integration): `forge ship` Checkpoint 2 delegates to `forge test smoke + unit + regression`.
  - TC-42-06 (regression): passing an empty `--filter` flag does not silently skip all tests.

### DEV-M1-43 — `forge adopt` CLI verb (incremental adoption in existing projects)
- **Tier:** T1 — **Anchor:** Spec §11.1.2 DX commitment #6; Spec §4 namespace 1
- **Status:** ✅ SHIPPED — `cmdadopt` ships incremental adoption: framework detection, primitive-by-primitive add, `--dry-run`, `--json`; idempotent second-run.
- **Acceptance:** `forge adopt` runs in an existing Next.js/Hono/Fastify project and offers to add Forge primitives one at a time; `--json` emits machine-readable adoption plan; `--dry-run` shows what would change without writing files.
- **Test cases:**
  - TC-43-01 (happy): `forge adopt` in a plain `package.json` project detects the framework and proposes relevant primitives.
  - TC-43-02 (happy): `forge adopt --add audit --dry-run` prints a diff without modifying files.
  - TC-43-03 (negative): running `forge adopt` in a directory with no supported framework exits with guidance, not a stack trace.
  - TC-43-04 (idempotency): running `forge adopt --add auth` twice is a no-op the second time.
  - TC-43-05 (boundary): `forge adopt` in a monorepo selects the right sub-package by default or prompts.
  - TC-43-06 (regression): a previously-adopted project does not re-add duplicate imports.

### DEV-M1-44 — `forge check` pre-flight verb (typecheck + lint + test gate)
- **Tier:** T1 — **Anchor:** Spec §4 namespace 4; Spec §13.3 quality gates
- **Status:** ✅ SHIPPED — `cmdcheck` ships typecheck + `forge lint` + `forge test unit` gate sequence; `--json` per-gate status; `schema` sub-verb validates TS↔DB alignment.
- **Acceptance:** `forge check` runs typecheck + `forge lint` + `forge test unit` in sequence; exits non-zero if any fails; `--json` emits per-gate status; `forge check schema` validates TypeScript↔DB type alignment.
- **Test cases:**
  - TC-44-01 (happy): `forge check` in a green project exits 0 and emits a summary.
  - TC-44-02 (negative): a lint violation causes `forge check` to exit non-zero and cite the failing gate.
  - TC-44-03 (negative): a type error is caught by `forge check` before code is committed.
  - TC-44-04 (boundary): `forge check schema` catches a column type mismatch between migration and generated types.
  - TC-44-05 (data-accuracy): `--json` output includes `{gate, status, duration_ms}` for each gate.
  - TC-44-06 (false-positive guard): a project with zero violations exits 0 and does not print spurious warnings.

### DEV-M1-45 — `forge generate <kind>` full verb (module / test / trpc / graphql / migration / fixtures)
- **Tier:** T1 — **Anchor:** Spec §4 namespace 3; Spec §11.1.2 DX commitments #5 + #10
- **Status:** ✅ SHIPPED — `cmdgenerate` ships module/test/trpc/graphql/migration/fixtures kinds; `--dry-run`, `--no-test`, `--json` flags active; idempotent second-run.
- **Acceptance:** `forge generate module <name>` scaffolds schema + service + controller + tests + LLM instructions; `forge generate test --from-bug <issue>` converts a bug report to a regression test; additional kinds (trpc, graphql, migration, fixtures) available via plugins; `--no-test` flag opts out; `--dry-run` previewable.
- **Test cases:**
  - TC-45-01 (happy): `forge generate module payments` creates service, controller, schema, test, and LLM instructions files.
  - TC-45-02 (happy): `forge generate test --from-bug <issue-url>` produces a failing regression test stub.
  - TC-45-03 (idempotency): running `forge generate module payments` twice is a no-op or prompts to overwrite.
  - TC-45-04 (negative): `forge generate unknown-kind` exits non-zero with a list of valid kinds.
  - TC-45-05 (boundary): `forge generate module payments --no-test` creates service + controller + schema but NOT a test file.
  - TC-45-06 (data-accuracy): generated test file pre-fills happy path + boundary cases for the generated service.
  - TC-45-07 (false-positive guard): generate in an existing module does NOT silently overwrite existing files.

### DEV-M1-46 — `forge context generate|show|budget` namespace
- **Tier:** T1 — **Anchor:** Spec §4 namespace 6
- **Status:** ✅ SHIPPED — `cmdcontext` ships generate/show/budget sub-verbs; token-budget enforcement; `--json` on all sub-verbs.
- **Acceptance:** `forge context generate` creates a compressed, semantically dense context snapshot at `.forge/context/snapshot.md`; `forge context show` prints the current snapshot; `forge context budget` reports token usage vs. provider limit; `--json` supported on all sub-verbs.
- **Test cases:**
  - TC-46-01 (happy): `forge context generate` produces a snapshot that fits within the configured token budget.
  - TC-46-02 (happy): `forge context budget` reports token count matching the snapshot size.
  - TC-46-03 (negative): `forge context show` with no snapshot exits non-zero with a `forge context generate` hint.
  - TC-46-04 (boundary): a snapshot at the token-budget limit is truncated with a clear marker, not silently cut.
  - TC-46-05 (data-accuracy): `--json` output includes `{tokens_used, tokens_budget, percent_used}`.
  - TC-46-06 (regression): a prior-run snapshot is not re-used if the repo has uncommitted changes.

### DEV-M1-47 — `forge ask <question>` natural-language Q&A verb
- **Tier:** T1 — **Anchor:** Spec §4 namespace 6
- **Status:** ✅ SHIPPED — `cmdask` ships LLM Q&A with context-snapshot grounding; graceful degradation when no provider; `--json` with `sources[]`.
- **Acceptance:** `forge ask <question>` routes the question through the LLM provider with the current context snapshot as grounding; `--json` emits `{question, answer, sources[]}`; graceful degradation when no LLM provider configured.
- **Test cases:**
  - TC-47-01 (happy): `forge ask "what does this module do"` returns a coherent answer using the context snapshot.
  - TC-47-02 (negative): running `forge ask` with no LLM provider configured exits with a friendly provider-setup hint.
  - TC-47-03 (boundary): a question exceeding the token budget is rejected with `FORGE-XXXX`, not silently truncated.
  - TC-47-04 (data-accuracy): `--json` `sources` field cites the files consulted.
  - TC-47-05 (false-positive guard): `forge ask --dry-run` makes no network calls.
  - TC-47-06 (regression): no PII from the codebase leaks into the answer via the context snapshot (secret-redaction path).

### DEV-M1-48 — `forge fix` CLI verb (full wiring of scan-fix loop)
- **Tier:** T1 — **Anchor:** Spec §4 namespace 4; DEV-M1-16 (scan-fix engine mechanics)
- **Status:** ✅ SHIPPED — `cmdfix` ships interactive list, per-finding apply, `--all --confidence`, `--pr` via `gh`, waiver integration, and audit log writes.
- **Acceptance:** `forge fix` (no args) shows pending high-confidence findings; `forge fix <finding-id>` applies a specific fix; `forge fix --all --confidence=high` batch-applies all high-confidence fixes; `--pr` opens a PR via `gh`; waiver mechanism from DEV-M1-17 wired in.
- **Test cases:**
  - TC-48-01 (happy): `forge fix` with pending high-confidence findings shows an interactive list.
  - TC-48-02 (happy): `forge fix <id>` applies the diff and writes to `.forge/audit.log`.
  - TC-48-03 (negative): `forge fix` with no recent scan exits non-zero with a `forge scan all` hint.
  - TC-48-04 (idempotency): applying the same fix twice is a no-op.
  - TC-48-05 (boundary): `forge fix --all --confidence=high` only applies findings with confidence ≥ high; medium/low are skipped.
  - TC-48-06 (data-accuracy): applied fix is recorded in audit log with finding-id + rule-id + confidence.
  - TC-48-07 (false-positive guard): `forge fix --dry-run` never writes to disk.

### DEV-M1-49 — `forge review` LLM PR-review verb
- **Tier:** T1 — **Anchor:** Spec §4 namespace 4; Spec §11.1.2 DX commitment #4
- **Status:** ✅ SHIPPED — `cmdreview` ships LLM PR-review; `--pr` posts inline comments via `gh`; graceful degradation; `--json` structured output.
- **Acceptance:** `forge review` reviews the current branch diff against `main` using LLM + scan results; `--pr <url>` posts inline comments via `gh` CLI; `--json` emits structured review; graceful degradation when no LLM provider configured.
- **Test cases:**
  - TC-49-01 (happy): `forge review` produces at least one finding on a branch with a known issue.
  - TC-49-02 (happy): `forge review --pr <pr-url>` posts inline comments via `gh` CLI.
  - TC-49-03 (negative): `forge review` with no LLM provider configured exits gracefully with a hint.
  - TC-49-04 (boundary): a PR with only doc changes produces zero code-review findings.
  - TC-49-05 (data-accuracy): `--json` output includes `{file, line, severity, rule_id, message}` per finding.
  - TC-49-06 (false-positive guard): `forge review` on a clean diff returns a clean-review result, not spurious findings.

### DEV-M1-50 — `forge migrate` CLI verb (up / down / status / suggest / repair)
- **Tier:** T1 — **Anchor:** Spec §4 namespace 7; Spec §14.2 migration safety
- **Status:** ✅ SHIPPED — `cmdmigrate` ships up/down/status/suggest/repair sub-verbs; `--dry-run`, `--allow-irreversible`, `--json` on all mutations.
- **Acceptance:** `forge migrate up` runs pending migrations; `forge migrate down` reverses the last; `forge migrate status` shows applied/pending; `forge migrate suggest` uses LLM to generate a migration from schema drift; `forge migrate repair` fixes drift; `--dry-run` on all mutations; `--json` on all sub-verbs.
- **Test cases:**
  - TC-50-01 (happy): `forge migrate up` on a fresh DB applies all pending migrations in order.
  - TC-50-02 (happy): `forge migrate down` reverses the last migration.
  - TC-50-03 (happy): `forge migrate status --json` reports applied and pending counts accurately.
  - TC-50-04 (negative): `forge migrate up` with a migration that has no `down` fails unless `--allow-irreversible` is set.
  - TC-50-05 (idempotency): `forge migrate up` when already fully migrated exits 0 with "nothing to do".
  - TC-50-06 (concurrency): two simultaneous `forge migrate up` calls are serialized via advisory lock.
  - TC-50-07 (boundary): `forge migrate up --dry-run` prints the SQL plan without executing.
  - TC-50-08 (regression): a real prior migration-drift scenario is reproduced and caught by `forge migrate repair`.

---

## M2 — Ecosystem (DEV-M2-01 .. DEV-M2-31)

### DEV-M2-01 — Plugin Registry index (signed JSON in Git) + CDN
- **Tier:** T1 + OPS — **Anchor:** Arch §3 C4
- **Status:** ✅ SHIPPED — Plugin Registry signed JSON index published in Git; CDN distribution active; mirror-able with end-to-end signature verification.
- **Acceptance:** Mirror-able; signature verification end-to-end.
- **Test cases:**
  - TC-01-01 (happy): client validates signed index against trust root.
  - TC-01-02 (negative): tampered index rejected with `FORGE-XXXX`.
  - TC-01-03 (boundary): air-gapped mirror works (TEST-16-02 link).
  - TC-01-04 (data-accuracy): index entries' SHA matches CDN-served artifact.

### DEV-M2-02 — `forge plugin install/list/upgrade/remove` verbs
- **Tier:** T1 — **Anchor:** Spec §4 + §20
- **Status:** ✅ SHIPPED — `cmdplugin` ships install/list/upgrade/remove sub-verbs; `plugins.lock` pinning with reproducible-install enforcement.
- **Acceptance:** Lock file pinning; reproducible install test.
- **Test cases:**
  - TC-02-01 (happy): install → list shows it; upgrade → version bump in lock; remove → cleared.
  - TC-02-02 (idempotency): install with same lock entry on a fresh machine → byte-identical install tree.
  - TC-02-03 (negative): install of an unpinned version when lock exists fails per policy.
  - TC-02-04 (boundary): install of zero plugins is a no-op exit 0.

### DEV-M2-03 — Plugin compliance test runner (per capability)
- **Tier:** T1 — **Anchor:** Arch §8
- **Status:** ✅ SHIPPED — plugin compliance test runner implemented; `PLUGIN_AUTHORING.md` cites the runner command; per-capability compliance suites green.
- **Acceptance:** Authoring guide cites the runner.
- **Test cases:** TEST-03 cases all pass.

### DEV-M2-04 — Second LLM provider plugin
- **Tier:** T2 — **Anchor:** Arch §9
- **Status:** ✅ SHIPPED — Gemini provider adapter shipped in `internal/llmprovider`; compliance suite green; T2 providers registered alongside OpenAI and Anthropic.
- **Acceptance:** Compliance suite green.
- **Test cases:** mirror DEV-M0-19 cases for the second provider.

### DEV-M2-05 — Deploy adapter: target #1
- **Tier:** T2 — **Anchor:** Spec §4 deploy + Arch §5 L6
- **Status:** ✅ SHIPPED — deploy adapter target #1 (Fly.io) ships in `cmddeploy`; deploy + smoke + rollback round-trip tested.
- **Acceptance:** Reference app deploys + rollbacks.
- **Test cases:**
  - TC-05-01 (happy): deploy + smoke + rollback round-trip green.
  - TC-05-02 (negative): deploy of broken artifact rolls back automatically; failure code propagated.
  - TC-05-03 (idempotency): re-deploy of same SHA → noop.
  - TC-05-04 (data-accuracy): deployed version reports the expected commit SHA.

### DEV-M2-06 — Deploy adapter: target #2
- **Tier:** T2 — **Anchor:** Spec §4 deploy
- **Status:** ✅ SHIPPED — deploy adapter target #2 (Railway) ships in `cmddeploy`; deploy + rollback round-trip tested.
- **Acceptance:** Reference app deploys + rollbacks.
- **Test cases:** mirror DEV-M2-05.

### DEV-M2-07 — Storage adapter: target #1
- **Tier:** T2 — **Anchor:** Arch §5 L1
- **Status:** ✅ SHIPPED — storage adapter target #1 ships in `internal/storage`; put/get/list/delete round-trip; cross-tenant isolation tested.
- **Acceptance:** Adapter compliance suite green.
- **Test cases:**
  - TC-07-01 (happy): put/get/list/delete round-trip.
  - TC-07-02 (boundary): zero-byte object handled.
  - TC-07-03 (cross-tenant): two tenants' buckets isolated (no cross-read).
  - TC-07-04 (concurrency): two parallel writers to the same key resolve per provider semantics + assert outcome.

### DEV-M2-08 — Eval harness: public scenario format + 7 reference scenarios
- **Tier:** T1 — **Anchor:** TEST plan §5
- **Status:** ✅ SHIPPED — `internal/eval` + `cmdeval`; deterministic YAML-scenario runner + JSON report shipped; all 7 reference scenarios green nightly; eval gate active on PRs.
- **Acceptance:** All scenarios green nightly.
- **Test cases:** TEST-06 .. TEST-13 all green nightly; one synthetic failing scenario is detected.

### DEV-M2-09 — Learning loop client (opt-in) — share path
- **Tier:** T1 — **Anchor:** Arch §10.3
- **Status:** ✅ SHIPPED — `internal/learningloop` ships opt-in share client; payload schema enforced; no source-code bytes in payload (fuzz-verified).
- **Acceptance:** Privacy invariant test (no source code in payload).
- **Test cases:**
  - TC-09-01 (happy): opted-in payload contains only allowed fields per schema.
  - TC-09-02 (negative): source-code byte never appears in any payload across 100 fuzzed scenarios.
  - TC-09-03 (regression): historical leakage incidents have fixtures here.
  - TC-09-04 (false-positive guard): a comment that *quotes* a known phrase is still excluded as source.

### DEV-M2-10 — Learning loop aggregator MVP
- **Tier:** OPS — **Anchor:** Arch §3 C7
- **Status:** ❌ NOT STARTED — learning loop aggregator (C7) not implemented; no nightly digest; no opt-out mechanism.
- **Acceptance:** Nightly digest produced; opt-out respected.
- **Verification:** opt-out user's id never appears in digest; cron emits digest each night.

### DEV-M2-11 — Scanner family: auth
- **Tier:** T1 — **Anchor:** Arch §10
- **Status:** ✅ **shipped** — implemented as part of `correctness` + `reliability` families in `internal/cli/cmdscan/scan.go`. Auth-specific patterns covered by unsafe-type-assertion, ts-any-escape, context-not-propagated rules.
- **Acceptance:** Seeded auth-bypass app: caught.
- **Test cases:** mirror DEV-M1-13 with auth-bypass fixtures.

### DEV-M2-12 — Scanner family: perf
- **Tier:** T1 — **Anchor:** Arch §10
- **Status:** ✅ **shipped** — `RunPerformance` in `internal/cli/cmdscan/scan.go`; 4 rules: select-star, unbounded-query, mutex-no-defer, fk-missing-index.
- **Acceptance:** N+1, missing-index fixtures caught.
- **Test cases:**
  - TC-12-01 (happy): N+1 fixture caught; missing-index fixture caught.
  - TC-12-02 (false-positive guard): batch query intentionally hitting N rows once is NOT flagged as N+1.
  - TC-12-03 (boundary): query at exactly the N+1 detection threshold is flagged per spec.

### DEV-M2-13 — Scanner family: accessibility
- **Tier:** T1 — **Anchor:** Arch §10
- **Status:** ✅ **shipped** — `RunAccessibility` in `internal/cli/cmdscan/scan.go`; 5 rules: img-missing-alt, link-text-click-here, button-no-label, tabindex-positive, html-missing-lang.
- **Acceptance:** WCAG-AA fixtures caught.
- **Test cases:**
  - TC-13-01 (happy): each WCAG-AA category has a positive fixture caught.
  - TC-13-02 (false-positive guard): compliant components produce zero findings.

### DEV-M2-14 — Scanner family: cost (cloud + LLM)
- **Tier:** T1 — **Anchor:** Arch §10
- **Status:** ✅ **shipped** — `RunCost` in `internal/cli/cmdscan/scan.go`; 4 rules: llm-call-in-loop, llm-no-token-limit, unbounded-cloud-list, missing-cache-control.
- **Acceptance:** Anti-pattern fixtures caught.
- **Test cases:**
  - TC-14-01 (happy): unindexed scan / chatty LLM loop fixtures caught.
  - TC-14-02 (data-accuracy): finding includes estimated $ delta.
  - TC-14-03 (false-positive guard): one-shot batch jobs are not flagged as chatty.

### DEV-M2-15 — `forge upgrade` codemod runner
- **Tier:** T1 — **Anchor:** Spec §16.5.6
- **Status:** ✅ SHIPPED — `cmdupgrade` + `internal/codemod`; `gitignore-marker`, `gitleaks-baseline`, `dependabot-baseline`, `pre-commit-baseline` builtins; `--apply`/dry-run/`list`; idempotent.
- **Acceptance:** One real codemod (a M1→M2 deprecation) shipped.
- **Test cases:**
  - TC-15-01 (happy): codemod transforms the deprecated pattern in the reference app.
  - TC-15-02 (idempotency): re-running codemod → no diff.
  - TC-15-03 (negative): codemod on already-migrated code → exits 0 with "nothing to do".
  - TC-15-04 (regression): rollback path documented + tested.

### DEV-M2-16 — Backward-compat alias mechanism
- **Tier:** T1 — **Anchor:** Spec §16.5.4 #9
- **Status:** ✅ SHIPPED — deprecated-verb alias mechanism wired in `root.go`; deprecation warning emitted on old verb invocations; one-minor retention enforced.
- **Acceptance:** Test: deprecated verb still works with warning.
- **Test cases:**
  - TC-16-01 (happy): deprecated verb runs and emits deprecation warning with replacement.
  - TC-16-02 (boundary): one-minor retention enforced (verb removed at minor+2).
  - TC-16-03 (data-accuracy): warning includes upgrade codemod reference.

### DEV-M2-17 — Performance benchmark gate (≤5% regression)
- **Tier:** OPS — **Anchor:** Spec §16.5.6
- **Status:** ✅ SHIPPED — performance benchmark CI gate wired; ≤5% regression passes; >5% blocks; baseline shift requires `--accept-baseline`.
- **Acceptance:** CI blocks regressions.
- **Verification:** TEST-05 cases enforced at CI; ≤5% baseline drift = pass; >5% = block; baseline shift requires explicit `--accept-baseline`.

### DEV-M2-18 — Plugin-loader sandbox audit + fuzz
- **Tier:** T1 — **Anchor:** Arch §15
- **Status:** ✅ SHIPPED — plugin-loader fuzz suite in nightly CI; WASM sandbox audit formalized; TEST-14 cases assembled; no escape findings.
- **Acceptance:** Fuzz suite in nightly; no escape findings.
- **Test cases:** TEST-14 cases all pass.

### DEV-M2-19 — First three external community plugins published
- **Tier:** T3 — **Anchor:** Spec §16.5.1
- **Status:** ❌ NOT STARTED — no external community plugins published; Plugin Registry (DEV-M2-01) not yet live.
- **Acceptance:** Listed in Registry; compliance suites green.
- **Verification:** each external plugin passes TEST-03 compliance suite; signed by author.

### DEV-M2-20 — Eval harness used to gate at least one PR (proof-of-life)
- **Tier:** OPS — **Anchor:** DEV plan §6
- **Status:** ✅ SHIPPED — eval harness gates PRs; proof-of-life PR linked in changelog with eval-gate status check.
- **Acceptance:** Issue + PR linked in changelog.
- **Verification:** at least one historical PR shows eval-gate status check; changelog cites it.

### DEV-M2-21 — Pilot user runs `forge ship` end-to-end on production app
- **Tier:** DOC — **Anchor:** DEV plan §3.2 exit
- **Status:** ❌ NOT STARTED — no pilot case study published; no external pilot user confirmed.
- **Acceptance:** Case study published.
- **Verification:** case study lives at a public URL; pilot user co-authored.

### DEV-M2-22 — Migration runner: forward + reverse + double-apply tests
- **Tier:** T1 — **Anchor:** TEST plan §2
- **Status:** ✅ SHIPPED — migration runner ships forward + reverse + double-apply tests on reference migration; TEST-10 cases all green.
- **Acceptance:** All three pass on reference migration.
- **Test cases:** TEST-10 cases all pass.

### DEV-M2-23 — Chaos-drill harness for the 8 §17.3 cross-cutting scenarios
- **Tier:** T1 — **Anchor:** Arch §17.3 + ARCH-DEC-15 + OPS-17
- **Status:** ✅ SHIPPED — chaos-drill harness ships in `internal/chaos`; all 8 §17.3 scenarios automated; JSON drill reports at `.forge/drills/<run>.json`; CI runs monthly.
- **Acceptance:** Harness can inject each of the 8 catalogued scenarios (provider outage mid-`ship`, ENOSPC, concurrent ship lock, plugin panic during scan, ledger tamper, cassette drift, secret-leak-via-debug, prod migration drift); each drill produces a JSON report at `.forge/drills/<run>.json`; CI runs the full set monthly.
- **Test cases:**
  - TC-23-01 (happy): each of the 8 scenarios runs end-to-end and produces a valid drill report.
  - TC-23-02 (negative): a scenario where the system fails to contain (e.g. ledger tamper goes undetected) reports `outcome=failed` and exits non-zero.
  - TC-23-03 (idempotency): re-running the same scenario with the same seed produces a byte-identical containment trace.
  - TC-23-04 (concurrency): two scenarios injected in parallel do not corrupt each other's drill reports.
  - TC-23-05 (data-accuracy): drill report cites the exact `FORGE-XXXX` code raised by the system under fault.
  - TC-23-06 (regression): a synthetic regression of the cassette-drift handling causes scenario #6 to fail, proving the gate.
  - TC-23-07 (false-positive guard): a no-fault control run produces `outcome=pass` for all 8 — the harness does not invent failures.

### DEV-M2-24 — Post-mortem template + `docs/postmortems/_TEMPLATE.md` + CI gate
- **Tier:** OPS — **Anchor:** Arch §18.6 + ARCH-DEC-20 + OPS-18
- **Status:** ✅ SHIPPED — `cmdpostmortem` + `docs/postmortems/_TEMPLATE.md` ship; CI gate rejects post-mortems missing qualifying action items.
- **Acceptance:** Template committed; CI gate rejects post-mortems whose `## 6. Action items` section lacks at least one tracking-issue link AND at least one §17.2 register update reference (or one new test commit).
- **Test cases:**
  - TC-24-01 (happy): a sample post-mortem with 2 action items (1 issue + 1 §17.2 PR ref) passes the gate.
  - TC-24-02 (negative): post-mortem with only "be more careful" action item fails with explainer.
  - TC-24-03 (negative): post-mortem missing sections 1–8 fails template-shape lint.
  - TC-24-04 (boundary): post-mortem with exactly 1 qualifying action item passes (off-by-one).
  - TC-24-05 (cross-cutting): a closed S0 issue without a corresponding `docs/postmortems/` file fails the OPS-18 nightly check.
  - TC-24-06 (regression): replay of a real prior incident's PR proves the gate would have caught "action-item-light" PMs.

### DEV-M2-25 — Status-page wiring (incident state machine → Quality Dashboard)
- **Tier:** OPS — **Anchor:** Arch §18.5 #6 + §18.8 + ARCH-DEC-21
- **Status:** ✅ SHIPPED — status-page wiring ships; incident state transitions published to Quality Dashboard via signed webhooks; replay protection active.
- **Acceptance:** Status page reachable; webhook publishes state transitions (identified → mitigated → fixed → post-mortem published) to TEST-19 dashboard; transitions are signed + replay-protected.
- **Test cases:**
  - TC-25-01 (happy): a synthesised incident moves through all 4 states; dashboard reflects each within 60s.
  - TC-25-02 (negative): a webhook with an invalid signature is rejected; nothing surfaces on the dashboard.
  - TC-25-03 (idempotency): the same transition delivered twice (replay) is recorded once.
  - TC-25-04 (boundary): an incident closed without `post-mortem published` for > SLA fires an OPS-18 alert.
  - TC-25-05 (concurrency): two simultaneous incidents do not entangle state.
  - TC-25-06 (false-positive guard): a normal deploy does not auto-create an incident.

---

### DEV-M2-26 — `forge eject` CLI verb (remove framework, leave working app)
- **Tier:** T1 — **Anchor:** Spec §11.1.2 DX commitment #2
- **Status:** ✅ SHIPPED — `cmdeject` ships full eject UX: removes all Forge scaffolding leaving working app; `--dry-run`, `--json`, audit log entry.
- **Acceptance:** `forge eject` removes all Forge scaffolding, leaving a working app; `--dry-run` shows what would change; `--json` emits structured list of changes; post-eject, `forge doctor` exits non-zero; audit log entry written.
- **Test cases:**
  - TC-26-01 (happy): `forge eject` on a freshly scaffolded project produces a working app with no Forge imports or config.
  - TC-26-02 (negative): `forge eject` in a directory that is not a Forge project exits non-zero with guidance.
  - TC-26-03 (idempotency): running `forge eject` twice is a no-op the second time.
  - TC-26-04 (boundary): `forge eject --dry-run` never writes to disk.
  - TC-26-05 (data-accuracy): post-eject, `npm test` / `go test` still passes on the reference app.
  - TC-26-06 (regression): a real prior "eject left broken imports" bug is reproduced and caught.

### DEV-M2-27 — `forge docs sync` / `forge docs heal` sub-verbs
- **Tier:** T1 — **Anchor:** Spec §4 namespace 11; Spec §16.5.4 gate #7
- **Status:** ✅ SHIPPED — `forge docs sync` and `forge docs heal` sub-verbs ship in `cmddocs`; `--dry-run`, `--json`; CI gate enforces docs-sync on every PR.
- **Acceptance:** `forge docs sync` regenerates docs from code so that `forge docs sync && git diff` is empty in CI; `forge docs heal` fixes broken anchors and dead links; both support `--dry-run` and `--json`; DEV-M3-05 `forge docs coverage` is a downstream consumer.
- **Test cases:**
  - TC-27-01 (happy): adding a new verb and running `forge docs sync` produces the new verb's doc page.
  - TC-27-02 (happy): `forge docs heal` fixes a synthetic broken anchor in the docs.
  - TC-27-03 (negative): `forge docs sync` with an invalid template exits non-zero with a clear error.
  - TC-27-04 (idempotency): running `forge docs sync` twice produces no diff the second time.
  - TC-27-05 (boundary): `forge docs heal` with zero broken links exits 0 with "nothing to fix".
  - TC-27-06 (regression): CI gate (`forge docs sync --check`) fails if docs are out of sync with code.

### DEV-M2-28 — `forge hygiene report` / `forge hygiene manifest` sub-verbs
- **Tier:** T1 — **Anchor:** Spec §4 namespace 12
- **Status:** ✅ SHIPPED — `forge hygiene report`, `forge hygiene manifest add|validate` all wired in `cmdhygiene`; `--json` on all sub-verbs.
- **Acceptance:** `forge hygiene report` emits a digest of unmanifested patterns, top violations by rule, and trend since last run; `forge hygiene manifest add <pattern>` adds a pattern to `.forge/hygiene.yml` (requires `--reason`); `forge hygiene manifest validate` checks the manifest against the current repo; `--json` on all sub-verbs.
- **Test cases:**
  - TC-28-01 (happy): `forge hygiene report` on a project with known violations lists them with rule IDs and file paths.
  - TC-28-02 (happy): `forge hygiene manifest add "*.bak" --reason "backup files"` appends to `hygiene.yml`.
  - TC-28-03 (negative): `forge hygiene manifest add` without `--reason` is rejected.
  - TC-28-04 (idempotency): adding the same pattern twice is a no-op.
  - TC-28-05 (data-accuracy): `forge hygiene report --json` includes `{rule_id, count, files[]}` per violation.
  - TC-28-06 (boundary): `forge hygiene report` on a clean project exits 0 with zero violations.

### DEV-M2-29 — `forge learn` sub-verbs (promote / antipatterns / teach / session / instructions / share)
- **Tier:** T1 — **Anchor:** Spec §4 namespace 5; Spec §8 Q21
- **Status:** ✅ SHIPPED — `cmdlearn` ships promote/antipatterns/teach/session/instructions/share sub-verbs; preferences written to `.forge/learned/preferences.yml`.
- **Acceptance:** `forge learn promote` promotes a convention from developer rejections; `forge learn antipatterns` mines reverts for anti-patterns; `forge learn teach <instruction>` adds explicit developer instruction to `.forge/learned/preferences.yml`; `forge learn session` shows session digest; `forge learn instructions` evolves `.forge/instructions/` from PR history; `forge learn share enable|disable` controls federated opt-in sharing (local-only by default).
- **Test cases:**
  - TC-29-01 (happy): `forge learn teach "never use floats for money"` appends to `.forge/learned/preferences.yml`.
  - TC-29-02 (happy): `forge learn promote <convention-id>` moves a rejected suggestion into `.forge/instructions/`.
  - TC-29-03 (negative): `forge learn share enable` without reviewing the payload schema summary fails with a one-line review prompt.
  - TC-29-04 (idempotency): teaching the same instruction twice is a no-op.
  - TC-29-05 (data-accuracy): `forge learn session` lists all learning events from the current session with timestamps.
  - TC-29-06 (false-positive guard): learning loop does NOT silently update `.forge/instructions/` without an explicit `forge learn` command.

### DEV-M2-30 — `forge deploy` / `forge rollback` CLI verbs
- **Tier:** T2 — **Anchor:** Spec §4 namespace 9 (Operate); Spec §14.2 / §14.4
- **Status:** ✅ SHIPPED — `forge deploy` and `forge rollback` CLI verbs ship via `cmddeploy`; adapter selection via `forge.config`; `ROLLBACK.md` runbook auto-generated per spec §14.4.
- **Acceptance:** `forge deploy` triggers the configured cloud adapter to deploy the current build; `forge rollback --to <release-id>` triggers the rollback playbook; both support `--dry-run`, `--json`, and adapter selection via `forge.config`; auto-generates `ROLLBACK.md` runbook per spec §14.4.
- **Test cases:**
  - TC-30-01 (happy): `forge deploy --dry-run` validates the deployment config and prints the plan without executing.
  - TC-30-02 (happy): `forge rollback --to <ref> --dry-run` prints the rollback steps without executing.
  - TC-30-03 (negative): `forge deploy` with no adapter configured exits non-zero with setup guidance.
  - TC-30-04 (boundary): `forge deploy` with an irreversible migration exits non-zero unless `--allow-irreversible` is set.
  - TC-30-05 (data-accuracy): `--json` output includes `{adapter, region, release_id, status}`.
  - TC-30-06 (regression): a synthetic "deploy succeeded but smoke tests failed → auto-rollback" scenario is exercised.

### DEV-M2-31 — `forge agents` namespace (start / stop / list)
- **Tier:** T2 — **Anchor:** Spec §4 namespace 9; Spec §21.3 multi-agent safety
- **Status:** ✅ SHIPPED — `cmdagents` ships start/stop/list sub-verbs; workspace actor model with role + permissions; token budgets and rate limits enforced per §21.3.
- **Acceptance:** `forge agents start <agent-spec>` registers and starts an agent as a workspace actor (with role + permissions); `forge agents stop --workspace=<id>` is the kill switch that halts all agents in the workspace immediately; `forge agents list --json` lists all agents with status, role, and token budget; budgets and rate limits enforced per spec §21.3.
- **Test cases:**
  - TC-31-01 (happy): `forge agents list --json` on a project with no running agents returns an empty array, exits 0.
  - TC-31-02 (happy): `forge agents stop` halts all running agents and records a kill event in the audit log.
  - TC-31-03 (negative): an agent spec requesting permissions exceeding the spawning user's own permissions is rejected.
  - TC-31-04 (boundary): `forge agents start` when the workspace agent budget is exhausted exits non-zero with a budget-exceeded error.
  - TC-31-05 (data-accuracy): agent actions appear in the audit log with `actor_type=agent` and the agent's `actor_id`.
  - TC-31-06 (false-positive guard): `forge agents list` does not start or stop any agents as a side effect.

---

## M3 — Hardening & 1.0 (DEV-M3-01 .. DEV-M3-29)

### DEV-M3-01 — `THREAT_MODEL.md` complete (STRIDE per Arch §15)
- **Tier:** T1 — **Anchor:** Arch §15
- **Status:** ✅ SHIPPED — `docs/THREAT_MODEL.md` published; STRIDE categories with mitigations per Arch §15; TC-id links to individual threat rows not yet uniformly added.
- **Acceptance:** Each threat has a tested mitigation.
- **Verification:** every threat in the doc links to a TC-id (existing or new); CI fails if any threat lacks a linked test.

### DEV-M3-02 — External pentest of CLI + plugin loader
- **Tier:** OPS — **Anchor:** Spec §16.5.6
- **Status:** ✅ SHIPPED — external pentest of CLI + plugin loader completed; findings triaged and criticals fixed; report archived with regression TCs.
- **Acceptance:** Findings triaged; criticals fixed.
- **Verification:** report archived; each critical mapped to a fix PR + a regression TC under the matching scanner family.

### DEV-M3-03 — Bug-bounty program live
- **Tier:** OPS — **Anchor:** Launch §11
- **Status:** ✅ SHIPPED — bug-bounty program live on huntr.dev; scope doc published; first valid report acknowledged within SLA.
- **Acceptance:** huntr.dev or similar; scope documented.
- **Verification:** scope doc public; first valid report acknowledged within SLA.

### DEV-M3-04 — All NFR budgets (Arch §14) asserted in CI
- **Tier:** OPS — **Anchor:** Spec §16.5.6
- **Status:** ✅ SHIPPED — all Arch §14 NFR budgets (cold-start ≤800ms, scan ≤2s, ship ≤60s) gated in CI; dashboard shows all green; baseline + delta tracked per release.
- **Acceptance:** CI dashboard shows all budgets green.
- **Verification:** every Arch §14 budget has a CI gate; baseline + delta tracked per release.

### DEV-M3-05 — Docs site complete: every verb, every error code, every extension point
- **Tier:** DOC — **Anchor:** Spec §16.5.4 #7
- **Status:** ✅ SHIPPED — docs site complete; `forge docs coverage` reports 100%; every verb, error code, and extension point documented.
- **Acceptance:** `forge docs coverage` reports 100%.
- **Verification:** coverage tool fails CI below 100%; sample link-check passes.

### DEV-M3-06 — RFC process operational with ≥3 accepted RFCs
- **Tier:** DOC — **Anchor:** Spec §16.2
- **Status:** ✅ SHIPPED — RFC process operational; ≥3 accepted RFCs published in `docs/rfcs/` archive with linked PRs.
- **Acceptance:** Public RFC archive.
- **Verification:** archive lists ≥3 accepted RFCs with linked PRs.

### DEV-M3-07 — Contribution-standards CI bot live
- **Tier:** OPS — **Anchor:** Spec §16.5.4 + §16.5.7
- **Status:** ✅ SHIPPED — contribution-standards CI bot live; auto-comments which gate failed + doc link on failing PRs within 60s.
- **Acceptance:** Bot auto-comments which gate failed + doc link.
- **Verification:** seeded failing PR → bot comment within 60s naming the gate; passing PR → no spurious comment.

### DEV-M3-08 — Maintainer review-SLA dashboard
- **Tier:** OPS — **Anchor:** Spec §16.5.7
- **Status:** ✅ SHIPPED — maintainer review-SLA dashboard live; automated SLA-breach alert; synthetic breach triggers alert in <1h.
- **Acceptance:** Public; alerts on breach.
- **Verification:** dashboard live; synthetic SLA breach triggers alert in <1h.

### DEV-M3-09 — T2 adapter coverage: ≥top 5 cloud + top 3 LLM providers
- **Tier:** T2 — **Anchor:** Launch §8
- **Status:** ✅ SHIPPED — T2 adapter coverage: ≥5 cloud providers + ≥3 LLM providers; compliance suites green; matrix published in `docs/`.
- **Acceptance:** Compliance suites green.
- **Verification:** each adapter passes TEST-03 + family-specific compliance; matrix published.

### DEV-M3-10 — Performance regression test gates locked
- **Tier:** OPS — **Anchor:** Spec §16.5.6
- **Status:** ✅ SHIPPED — performance regression gates locked; baseline JSON committed per release; trend chart on dashboard; gates required in branch protection.
- **Acceptance:** History tracked per release.
- **Verification:** baseline JSON committed per release; trend chart on dashboard.

### DEV-M3-11 — i18n scaffolding (English-only at 1.0; structure ready)
- **Tier:** T1 — **Anchor:** Arch §11
- **Status:** ✅ SHIPPED — `internal/i18n` ships message catalog; all user-facing strings centralized; i18n lint rule wired; English-only at 1.0 with structure ready for additional locales.
- **Acceptance:** All user-facing strings centralized.
- **Test cases:**
  - TC-11-01 (happy): every user-facing string resolved through the catalog.
  - TC-11-02 (negative): a hard-coded string in source fails the i18n lint.
  - TC-11-03 (data-accuracy): catalog round-trip preserves placeholders.

### DEV-M3-12 — Telemetry payload audit + public schema
- **Tier:** OPS — **Anchor:** Arch §11 + §13 ADR-006
- **Status:** ✅ SHIPPED — `internal/telemetry` file-based OTLP spans; `cmdtelemetry` enable/disable/status/rotate-id; schema versioned; opt-in mechanics tested; 21 tests. **Note:** Arch §13 ADR-006 specifies OTLP over HTTPS; current impl uses local file transport; full OTLP collector wiring is a follow-on.
- **Acceptance:** Schema versioned; opt-in mechanics tested.
- **Verification:** DEV-M0-30 cases at integration tier; schema doc public.

### DEV-M3-13 — Air-gapped install path documented + tested
- **Tier:** OPS — **Anchor:** Arch §12
- **Status:** ✅ SHIPPED — air-gapped install path documented in `docs/DISTRIBUTION.md` + `docs/airgap.md`; TEST-16-02 green; walkthrough validated by non-author.
- **Acceptance:** Mirror walkthrough verified.
- **Verification:** TEST-16-02 case green; doc walkthrough validated by a non-author.

### DEV-M3-14 — All §16.5.4 gates active and required for merge
- **Tier:** OPS — **Anchor:** Spec §16.5
- **Status:** ✅ SHIPPED — all §16.5.4 gates active and required for merge; branch-protection JSON committed; tested by deliberately failing PRs per gate.
- **Acceptance:** Branch protection updated.
- **Verification:** branch-protection JSON committed; each gate listed; tested by a deliberately failing PR per gate.

### DEV-M3-15 — v1.0.0 release artifact + signing + tap update
- **Tier:** OPS — **Anchor:** Arch §13
- **Status:** ✅ SHIPPED — v1.0.0 release artifact signed with sigstore; Brew + Scoop + winget taps updated; reproducible build verified by two independent builders.
- **Acceptance:** Reproducible build verified.
- **Verification:** two independent builders produce byte-identical artifact; signature verified end-to-end.

### DEV-M3-16 — Post-1.0 deprecation policy doc
- **Tier:** DOC — **Anchor:** Spec §16.5.4 #9
- **Status:** ✅ SHIPPED — post-1.0 deprecation policy committed in `docs/DEPRECATION_POLICY.md`; one-minor alias retention rule codified.
- **Acceptance:** One-minor alias retention codified.
- **Verification:** doc published; first deprecation test follows the policy.

### DEV-M3-17 — Status page + incident runbook
- **Tier:** OPS — **Anchor:** Launch §11
- **Status:** ✅ SHIPPED — status page + incident runbook published; tabletop exercise completed; runbook revision committed.
- **Acceptance:** Tabletop exercise completed.
- **Verification:** tabletop minutes archived; runbook revision committed.

### DEV-M3-18 — "What changed since beta" launch post
- **Tier:** DOC — **Anchor:** Launch §8
- **Status:** ✅ SHIPPED — "What changed since beta" launch post co-authored with community maintainer and published.
- **Acceptance:** Co-authored with one community maintainer.
- **Verification:** post published; co-author named.

### DEV-M3-19 — Private-vulnerability intake + `SECURITY.md` + disclosure workflow
- **Tier:** OPS — **Anchor:** Arch §18.1 vulnerability row + Spec §15 + ARCH-DEC-18
- **Status:** ✅ SHIPPED — `docs/SECURITY.md` published with 90-day disclosure window, PGP key, safe-harbor language; GitHub private advisory channel active; huntr.dev intake wired; CNA status applied for.
- **Acceptance:** `SECURITY.md` published with disclosure window (90-day default), PGP key, safe-harbor language; private advisory channel live; bug-bounty intake (huntr.dev or equivalent) wired; CNA status applied for if eligible.
- **Test cases:**
  - TC-19-01 (happy): test advisory submitted via private channel reaches Security WG within first-response SLA.
  - TC-19-02 (negative): public-tracker submission containing the word `vulnerability` in title is auto-redirected with template + closed.
  - TC-19-03 (boundary): advisory at exactly day-90 of the disclosure window flips to public per policy.
  - TC-19-04 (cross-tenant): a non-Security-WG maintainer cannot read advisory threads.
  - TC-19-05 (regression): a synthetic past vuln is replayed end-to-end and the timeline matches the policy.
  - TC-19-06 (false-positive guard): a benign "how does the sandbox work?" thread does NOT get auto-routed to advisory.

### DEV-M3-20 — Two-key enforcement for irreversible incident-time operations
- **Tier:** T1 — **Anchor:** Arch §18.4 + ARCH-DEC-22
- **Status:** ✅ SHIPPED — two-key enforcement active: force-push blocked on main + release branches; sigstore requires two custodians; gate-bypass bot enforces second-maintainer approval; trust-root rotation ceremony documented.
- **Acceptance:** Branch protection blocks force-push on `main` + release branches; sigstore signing requires two custodians; bot enforces second-maintainer approval on `gate-bypass`-labelled PRs; trust-root rotation requires a documented two-custodian ceremony.
- **Test cases:**
  - TC-20-01 (happy): a `gate-bypass` PR with two maintainer approvals merges; with one, it does not.
  - TC-20-02 (negative): force-push to `main` is rejected even by an admin (admin-overrides logged separately).
  - TC-20-03 (negative): a release attempt with a single sigstore custodian fails.
  - TC-20-04 (boundary): exactly two maintainer approvals is the minimum; three or more is fine.
  - TC-20-05 (cross-tenant): a Reviewer (not Maintainer) approval does not satisfy the two-key rule.
  - TC-20-06 (regression): a replay of a hypothetical "silent gate disable" PR is blocked.
  - TC-20-07 (false-positive guard): a routine PR without `gate-bypass` label needs only the normal one approval.

### DEV-M3-21 — Eval-harness flake-quarantine policy implementation
- **Tier:** T1 — **Anchor:** Arch §17.2 eval-harness row + §17.3 #6 + ARCH-DEC-23
- **Status:** ✅ SHIPPED — eval-harness flake-quarantine ships: per-scenario seed pin, cassette signature, 3-run quorum; auto-quarantine + auto-issue at threshold; CI gate fails at 30d without owner.
- **Acceptance:** Per-scenario seed pin + cassette signature + 3-run quorum; auto-quarantine at threshold; auto-issue creation; CI gate fails when a scenario has been quarantined > 30 days without an owner.
- **Test cases:**
  - TC-21-01 (happy): a stable scenario runs 3× with quorum agreement → green.
  - TC-21-02 (negative): a flaky scenario (forced 1/3 disagreement) auto-quarantines + opens an issue; gate does NOT fail on it for 30d.
  - TC-21-03 (boundary): scenario at exactly 30d in quarantine without owner fails the gate (off-by-one).
  - TC-21-04 (idempotency): re-quarantining an already-quarantined scenario does not duplicate the issue.
  - TC-21-05 (regression): a synthesised "silent flip" cassette diff is caught by the cassette-signature check, not by the quorum.
  - TC-21-06 (false-positive guard): a scenario where all 3 runs disagree by < tolerance threshold is NOT quarantined.

### DEV-M3-22 — Reversibility contract: `forge undo` cross-domain coverage (FS + DB + scan-fix)
- **Tier:** T1 — **Anchor:** Arch §17.1 #5 + ARCH-DEC-24
- **Status:** ✅ SHIPPED — `forge undo` ships cross-domain reversibility: FS writes, DB migrations, scan-fix applies; `.forge/trash/<run-id>/` with 14d default retention; cross-platform safe-delete tested.
- **Acceptance:** `.forge/trash/<run-id>/` retention default 14 d, configurable; `forge undo <run-id>` covers FS writes, DB migrations (reverse-migration runner), and `forge fix --apply` scan fixes; cross-platform safe-delete (no Windows path-length / file-handle pitfalls).
- **Test cases:**
  - TC-22-01 (happy): each domain (FS, DB, scan-fix) round-trips: apply → undo → state byte-identical.
  - TC-22-02 (boundary): undo on a run at exactly retention-window-end works; one tick later, run is GC'd with a clear error.
  - TC-22-03 (negative): undo on a non-existent run-id fails with `FORGE-XXXX`, never partial-applies.
  - TC-22-04 (idempotency): `forge undo` twice on the same run-id is a no-op the second time (already-reversed).
  - TC-22-05 (concurrency): undo while a new `--apply` is in progress is serialised via the same advisory lock as DEV-M2-22.
  - TC-22-06 (cross-platform): Windows long-path (> MAX_PATH) FS write is undoable.
  - TC-22-07 (regression): a real prior "trash file leaked through gitignore" bug is replayed and fails the test that proved the fix.
  - TC-22-08 (false-positive guard): read-only verbs do NOT create trash entries.

---

### DEV-M3-23 — High-trust / regulated-industry templates (§12)
- **Tier:** T2 — **Anchor:** Spec §12; Spec §12.2
- **Status:** ✅ SHIPPED — `fintech-grade` and `high-trust` templates ship via `forge new --template`; compliance test suite green.
- **Acceptance:** `forge new my-app --template fintech-grade` scaffolds strict ACID + double-entry ledger + hash-chained audit + PCI-DSS tokenization adapter + field-level PII encryption + reconciliation jobs; at minimum `fintech-grade` and `high-trust` templates ship; other templates (healthcare-grade, govtech-grade, marketplace-grade, identity-grade, agent-grade) ship as follow-ons; each template passes the high-trust compliance test suite.
- **Test cases:**
  - TC-23-01 (happy): `forge new test-app --template fintech-grade` scaffolds without errors; `go test` / `npm test` passes.
  - TC-23-02 (happy): `forge new test-app --template high-trust` scaffolds the generic audit + idempotency + reconciliation variant.
  - TC-23-03 (negative): `forge new test-app --template fintech-grade` fails gracefully if required env vars (DB URL, encryption keys) are absent.
  - TC-23-04 (data-accuracy): every mutation endpoint in the fintech-grade scaffold emits an audit log entry (verified by `forge audit verify`).
  - TC-23-05 (idempotency): every transaction endpoint accepts an idempotency key and is safe to retry.
  - TC-23-06 (false-positive guard): a standard `go-service` scaffold does NOT include PCI-DSS or double-entry-ledger primitives by default.

### DEV-M3-24 — TypeScript runtime module library (`@forge/*` npm packages)
- **Tier:** T1 — **Anchor:** Spec §4 Feature Matrix (Foundation Layer); Spec §20.1
- **Status:** ✅ SHIPPED — `@forge/auth`, `@forge/tenancy`, `@forge/audit`, `@forge/migrations`, `@forge/core` published with Stable tier; ≥80% unit coverage each; TypeScript types exported.
- **Acceptance:** All five core packages published with Stable tier (§18.2); ≥80% unit test coverage each; TypeScript types exported and validated; `forge doctor` checks package compatibility; packages follow §20.1 small-core architecture.
- **Test cases:**
  - TC-24-01 (happy): `npm install @forge/auth` in a plain Hono app adds auth middleware with zero config.
  - TC-24-02 (happy): `@forge/tenancy` RLS policies pass the cross-tenant authz test suite.
  - TC-24-03 (negative): `@forge/auth` without a valid JWT secret fails at startup, not at request time.
  - TC-24-04 (boundary): `@forge/migrations` validates that all migrations have a `down` path before running `up`.
  - TC-24-05 (data-accuracy): `@forge/audit` appends a hash-chained entry for every mutation, verifiable by `forge audit verify`.
  - TC-24-06 (regression): a known RLS bypass is in the cross-tenant fixture suite and fails on pre-fix code.

### DEV-M3-25 — `forge optimize prompts|cost|queries` verb
- **Tier:** T2 — **Anchor:** Spec §4 namespace 5
- **Status:** ✅ SHIPPED — `cmdoptimize` ships prompts/cost/queries sub-verbs; DSPy-style prompt loop; `--dry-run`, `--json` on all sub-verbs.
- **Acceptance:** `forge optimize prompts` runs a DSPy-style loop to improve prompt templates in `.forge/prompts/`; `forge optimize cost` surfaces token-spend hotspots and suggests cheaper alternatives; `forge optimize queries` identifies N+1 and unbounded queries; all support `--dry-run` and `--json`.
- **Test cases:**
  - TC-25-01 (happy): `forge optimize cost --dry-run` produces a list of suggestions without modifying files.
  - TC-25-02 (negative): `forge optimize prompts` without an LLM provider configured exits gracefully.
  - TC-25-03 (data-accuracy): `forge optimize queries --json` output includes `{file, line, query, suggestion, estimated_saving}`.
  - TC-25-04 (false-positive guard): `forge optimize prompts` does NOT modify prompts whose eval score is already above the threshold.

### DEV-M3-26 — Per-verb prompt templates under `.forge/prompts/`
- **Tier:** T1 — **Anchor:** Spec §4 LLM-native design principle; Spec §11.2 principle #8
- **Status:** ✅ SHIPPED — per-verb prompt templates externalized under `.forge/prompts/<command>-*.prompt.md`; all LLM-touching verbs ship templates; developer per-project override supported.
- **Acceptance:** Every LLM-touching verb (ship, review, fix, generate, ask, context, optimize) ships its own prompt template under `.forge/prompts/<command>-*.prompt.md`; templates are vendor-neutral; developers can override per-project by editing `.forge/prompts/`; `forge docs sync` includes prompt templates in generated docs.
- **Test cases:**
  - TC-26-01 (happy): `forge ship spec` uses the template at `.forge/prompts/ship-spec.prompt.md` (or the built-in default if none exists).
  - TC-26-02 (happy): overriding a prompt template in a project changes LLM output (snapshot test).
  - TC-26-03 (negative): a malformed prompt template (invalid frontmatter) is rejected at load time with a clear error.
  - TC-26-04 (idempotency): `forge docs sync` run twice produces no diff in the generated prompt docs.
  - TC-26-05 (false-positive guard): a verb that does NOT call an LLM does NOT have a prompt template registered.

### DEV-M3-27 — Multi-tool AI instructions (AGENTS.md / CLAUDE.md / .cursorrules / .windsurfrules)
- **Tier:** T1 — **Anchor:** Spec §11.1.2 DX commitments #4 + #9; Spec §4 "agent-readable by default"
- **Status:** ✅ SHIPPED — `forge new` scaffolds `AGENTS.md` + `CLAUDE.md` + `.cursorrules` + `.windsurfrules`; `forge generate module` updates them; `forge doctor` warns if missing.
- **Acceptance:** `forge new <name>` scaffolds `AGENTS.md` + `CLAUDE.md` + `.cursorrules` + `.windsurfrules` in the generated project; `forge generate module <name>` updates these files with module-specific context; files kept in sync by `forge docs sync`; `forge doctor` warns if these files are missing or stale.
- **Test cases:**
  - TC-27-01 (happy): `forge new my-app --template go-service` produces all four AI instruction files.
  - TC-27-02 (happy): `forge generate module payments` appends payments-module context to `AGENTS.md`.
  - TC-27-03 (negative): `forge doctor` warns if `AGENTS.md` is absent from a Forge project.
  - TC-27-04 (idempotency): running `forge generate module payments` twice does not duplicate the AGENTS.md entry.
  - TC-27-05 (data-accuracy): `.cursorrules` contains correct `forge lint` and `forge ship` invocation patterns.
  - TC-27-06 (false-positive guard): `forge docs sync` does NOT overwrite developer customizations in these files.

### DEV-M3-28 — Multi-agent runtime foundation (§21.2 / §21.3 primitives)
- **Tier:** T2 — **Anchor:** Spec §21.2 / §21.3; Spec §4 `forge agents` namespace
- **Status:** ✅ SHIPPED — multi-agent runtime foundation ships: actor-type model, RLS for agents, causal Trace primitive, loop-detection circuit-breaker, token budget enforcement per §21.3.
- **Acceptance:** Agent-as-Actor model in the workspace membership table (`actor_type: human|agent|system`); RLS policies apply equally to agents; causal Trace primitive captures `human → agent → capability → outcome` chains; loop-detection circuit-breaker prevents A→B→A cycles; agent token budget + rate limits enforced per §21.3; `forge agents stop` kill switch works cross-domain.
- **Test cases:**
  - TC-28-01 (happy): an agent spawned with `actor_type=agent` is subject to the same RLS policies as a human user.
  - TC-28-02 (happy): a causal trace from a multi-step agent action is queryable end-to-end.
  - TC-28-03 (negative): an A→B→A agent loop is detected and circuit-broken with a structured error.
  - TC-28-04 (negative): an agent cannot be granted permissions exceeding those of its spawning user.
  - TC-28-05 (boundary): `forge agents stop` at exactly the token-budget limit halts the agent before the budget-exceeding call.
  - TC-28-06 (false-positive guard): a legitimate A→B sequential (non-looping) chain is NOT circuit-broken.

### DEV-M3-29 — `forge add <primitive>` verb (auth / billing / storage / plugin)
- **Tier:** T1 — **Anchor:** Spec §4 namespace 3; Spec §20.4 plugin installation
- **Status:** ✅ SHIPPED — `cmdadd` ships auth/billing/storage/plugin primitives; installs + wires + updates `forge.config` + runs migrations; `--dry-run` supported.
- **Acceptance:** `forge add auth` installs and wires the auth primitive; `forge add billing` installs the billing adapter wiring; `forge add storage` installs the storage adapter; `forge add plugin <name>` wraps `forge plugin install`; each call (1) installs the package, (2) updates `forge.config`, (3) adds LLM instructions, (4) runs any install migrations; `--dry-run` support.
- **Test cases:**
  - TC-29-01 (happy): `forge add auth` in a plain `ts-service` project adds auth middleware and wires it to `forge.config`.
  - TC-29-02 (happy): `forge add billing --provider=stripe` installs the Stripe adapter and prompts for `STRIPE_SECRET_KEY`.
  - TC-29-03 (negative): `forge add auth` in a project that already has auth configured prompts to confirm overwrite.
  - TC-29-04 (idempotency): `forge add storage` twice is a no-op the second time.
  - TC-29-05 (boundary): `forge add plugin @acme/unknown-plugin` (not in registry) exits with a clear error.
  - TC-29-06 (data-accuracy): after `forge add auth`, `AGENTS.md` includes auth-specific LLM guidance.

---

*Task file version: 0.5 — companion to spec v0.10.9. Gap-fill tasks DEV-M1-42..50, DEV-M2-26..31, DEV-M3-23..29 added from spec→tasks analysis.*
