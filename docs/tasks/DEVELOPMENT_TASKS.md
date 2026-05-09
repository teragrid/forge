# Forge — Development Tasks

> Companion to `../docs/DEVELOPMENT_PLAN.md`.
> Tracker for the Section-B tasks (DEV-M{0..3}-NN) from the master breakdown.

## Release status — what shipped where

> Latest tag: **`v0.2.0-m2-preview`** at commit `768367b` (CI green). HEAD adds `forge audit query`, `forge spend` (#15), `forge incident` (#16), `forge telemetry` (#17), and WASM plugin runtime stub (DEV-M2-09/M3-03/M3-06/M3-01/M2-05).

### `v0.1.0-mvp` — community-launch slice (M0 + M1 partial)

| Task | Title | Status |
|------|-------|--------|
| DEV-M0-01 | Repo skeleton + DCO + cross-compile | ✅ (CI gates, 6-triple matrix, CODEOWNERS) |
| DEV-M0-02 | Config loader (layered) | 🟡 partial — flags+env via cobra; viper layering deferred to M0.3 |
| DEV-M0-03 | Error-code framework `FORGE-XXXX` | ✅ `internal/errcode` (reserved-range registry, panic on dup, tests) |
| DEV-M0-04 | Structured logger | ✅ `internal/logobs` (slog wrapper, secret redaction, `--explain` bypass) |
| DEV-M0-11 | CLI verb router | ✅ `internal/cli` + `internal/cli/cmd<verb>/` subpackages, `verbmeta` registry |
| DEV-M0-12 | `forge explain` | ✅ `--json` supported; lists all verbs or one manifest |
| DEV-M0-13 | `forge new` | ✅ `go-service` template; `--name`/`--module`/`--force`/`--json` |
| DEV-M0-14 | `forge doctor` | ✅ checks git/go/temp; `--json` supported |
| DEV-M0-15 | `forge clean` | ✅ `--check`/`--apply`; manifest-driven |
| DEV-M0-22 | Bundled `.gitignore` template | ✅ ships in `go-service` template with marker block |
| DEV-M0-23 | Bundled `.gitleaks.toml` | ✅ ships in `go-service` template (4 baseline rules) |
| DEV-M0-27 | `.forge/manifest` reader | ✅ `internal/manifest` (scratch/managed sections, glob matcher) |
| DEV-M0-33 | CI workflow (lint+test+build matrix) | ✅ `.github/workflows/ci.yml` (Go 1.25, race+cgo) |
| DEV-M0-34 | Release workflow stub | ✅ tag-driven, GoReleaser-ready |

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
| DEV-M2-01 | `forge upgrade` codemod runner | ✅ 2 builtins (`gitignore-marker`, `gitleaks-baseline`); `--apply`/dry-run/`list` |
| DEV-M2-02 | Audit ledger | ✅ `internal/audit` SHA-256 hash-chained JSONL at `.forge/audit.log`; `forge audit show/verify/append` |
| DEV-M2-03 | Plugin discovery (in-tree) | ✅ scanners + codemods auto-register to `plugin.Default()`; `forge plugin list/show` (verb #11) |
| DEV-M3-S1 | NFR benchmarks (spike) | ✅ `BenchmarkScanSecrets_500Files`, `BenchmarkScaffold_GoService`; `make bench` |
| DEV-M3-S2 | Error-code doc generator | ✅ `cmd/gen-errors`; `make docs` / `make docs-check`; 20 codes |

### Remaining for `v0.3.0` and beyond

**M2.x — plugin ecosystem**
- DEV-M2-04 — `forge eval` scenario harness (YAML scenarios, deterministic runner, JSON report; codes 3600..3699). ✅ **shipped**
- DEV-M2-05 — Wazero WASM plugin runtime behind `forge_wasm` build tag ✅ **shipped** (`wasm_stub.go` + `wasm.go` + `wasm_stub_test.go`; 8 tests; codes 4200..4299)
- DEV-M2-06 — `.forge/plugins.json` discovery + dynamic registration ✅ **shipped**
- DEV-M2-07 — Codemod: `dependabot-baseline` ✅ **shipped**
- DEV-M2-08 — Codemod: `pre-commit-baseline` ✅ **shipped**
- DEV-M2-09 — Audit ledger: `forge audit query` sub-subcommand ✅ **shipped** (AND-filter, `--since`, `--limit`, `--json`; code 3402; 11 tests)

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

Items still deferred from M0: DEV-M0-05/06/07/08/09/16/17/18/19/20/21/24/25/26/28/29/30/31/32/35/36 (see detail rows below).

---

ID and conventions follow `ARCHITECTURE_TASKS.md`. Each implementation task (T1/T2/T3) lists explicit **test cases** (TC-IDs) following the 9-point checklist from `always-write-tests.md` (happy / boundary / negative / idempotency / concurrency / cross-tenant / regression / data-accuracy / false-positive guard) — only points that meaningfully apply are included, never invented. OPS/DOC tasks list **verification checks** instead of unit-style cases.

---

## M0 — Bootstrap (DEV-M0-01 .. DEV-M0-36)

### DEV-M0-01 — Repo skeleton: monorepo layout, license, CODEOWNERS, DCO bot
- **Tier:** T1 — **Anchor:** Arch §3 C1, ADR-001
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
- **Acceptance:** Unit + boundary tests; `--explain` shows source per key.
- **Test cases:**
  - TC-02-01 (happy): each layer overrides the layer below it for a known key.
  - TC-02-02 (boundary): missing file / empty env still resolves to defaults; no crash.
  - TC-02-03 (negative): malformed config file fails with `FORGE-XXXX`, not a stack trace.
  - TC-02-04 (data-accuracy): `--explain --key K` reports the winning layer correctly under every combination tested by TEST-15.

### DEV-M0-03 — Error-code framework (`FORGE-XXXX`) + reserved-range registry + lint rule
- **Tier:** T1 — **Anchor:** Arch §11
- **Acceptance:** Lint blocks duplicate codes; doc auto-generated.
- **Test cases:**
  - TC-03-01 (happy): a new error code in an unused range passes lint.
  - TC-03-02 (negative): duplicate code anywhere in the tree fails lint with both source locations cited.
  - TC-03-03 (negative): code outside any reserved range fails lint.
  - TC-03-04 (data-accuracy): generated error-code doc lists every code with its description.
  - TC-03-05 (false-positive guard): the same code referenced (not declared) twice does not trip the duplicate check.

### DEV-M0-04 — Structured logger (JSON + TTY formatter)
- **Tier:** T1 — **Anchor:** Arch §11
- **Acceptance:** Unit + integration; never logs prompts unless `--explain`.
- **Test cases:**
  - TC-04-01 (happy): JSON mode emits one event per log call with required fields.
  - TC-04-02 (boundary): log of a 1 MB payload is truncated with a marker, not OOM.
  - TC-04-03 (negative): attempting to log a value flagged "secret" emits redacted form.
  - TC-04-04 (regression): default mode never includes raw LLM prompt content (snapshot test).
  - TC-04-05 (data-accuracy): `--explain` mode includes prompt content under a labeled field, byte-for-byte.

### DEV-M0-05 — Filesystem service with sandbox-aware glob
- **Tier:** T1 — **Anchor:** Arch §8.2
- **Acceptance:** Permission-denial test; cross-OS path test.
- **Test cases:**
  - TC-05-01 (happy): glob inside granted root returns expected files on linux/mac/win.
  - TC-05-02 (negative): glob escaping the grant (`../../etc/passwd`) is denied with `FORGE-XXXX`.
  - TC-05-03 (boundary): symlink at the grant boundary is not followed outward.
  - TC-05-04 (cross-OS): backslash vs forward-slash handling identical across platforms.

### DEV-M0-06 — Git service (read-only ops: status, diff-since, log)
- **Tier:** T1 — **Anchor:** Arch §6
- **Acceptance:** Integration test against real repo fixtures.
- **Test cases:**
  - TC-06-01 (happy): `status` / `diff-since` / `log` return correct shapes against a fixture repo.
  - TC-06-02 (boundary): empty repo + first-commit edge cases handled.
  - TC-06-03 (negative): operating outside a repo exits with `FORGE-XXXX`.
  - TC-06-04 (false-positive guard): the service has zero write paths (audit grep test).

### DEV-M0-07 — Process service (proc spawn with allow-list)
- **Tier:** T1 — **Anchor:** Arch §8.2
- **Acceptance:** Sandbox escape test; deny-by-default verified.
- **Test cases:**
  - TC-07-01 (happy): allow-listed binary runs and stdout is captured.
  - TC-07-02 (negative): non-allow-listed binary is denied.
  - TC-07-03 (negative): allow-listed binary invoked with arg trying to spawn a child outside allow-list is blocked.
  - TC-07-04 (concurrency): two parallel spawns share no process group leakage.
  - TC-07-05 (regression): every prior sandbox-escape ticket has a fixture here.

### DEV-M0-08 — Audit ledger (append-only, hash-chained)
- **Tier:** T1 — **Anchor:** Arch §15
- **Acceptance:** Tamper-detection test; replay test.
- **Test cases:**
  - TC-08-01 (happy): N appends + verify chain is valid.
  - TC-08-02 (negative): mutating any byte of any prior entry breaks `verify`.
  - TC-08-03 (idempotency): replay of the ledger reconstructs the same end state.
  - TC-08-04 (boundary): empty ledger verifies clean.
  - TC-08-05 (data-accuracy): each entry's hash equals `H(prev || payload)` (snapshot vector).

### DEV-M0-09 — Secrets handling (placeholder rewriter for prompts)
- **Tier:** T1 — **Anchor:** Arch §11 + §15
- **Acceptance:** Seeded-secret test asserts zero leakage in 100 runs (TEST-12).
- **Test cases:**
  - TC-09-01 (happy): seeded secret never appears in 100 outbound LLM payloads.
  - TC-09-02 (negative): non-secret string of similar shape is NOT redacted.
  - TC-09-03 (data-accuracy): redacted placeholder length is `min(8, raw_len)`.
  - TC-09-04 (regression): historical leakage fixtures all redacted.
  - TC-09-05 (false-positive guard): a string the user explicitly marks `--no-redact` (in `--explain`) flows through unchanged.

### DEV-M0-10 — CLI verb router (`forge <ns> <verb>`) with 3 namespaces (`new/doctor/explain`)
- **Tier:** T1 — **Anchor:** Spec §4 Command Surface
- **Acceptance:** Unit + `--help` snapshot test.
- **Test cases:**
  - TC-10-01 (happy): each (ns, verb) dispatches correctly.
  - TC-10-02 (negative): unknown verb under known ns shows ns-scoped help.
  - TC-10-03 (negative): unknown ns returns global help with `FORGE-XXXX`.
  - TC-10-04 (data-accuracy): `--help` snapshot stable across runs.

### DEV-M0-11 — `--json` schema framework + per-verb schema tests
- **Tier:** T1 — **Anchor:** Spec §4 universal flags
- **Acceptance:** Schema test harness; every `--json` output validated.
- **Test cases:**
  - TC-11-01 (happy): each verb's `--json` output validates against its schema.
  - TC-11-02 (negative): a verb that drifts (adds field without schema bump) fails CI.
  - TC-11-03 (boundary): empty result set still validates.
  - TC-11-04 (data-accuracy): schema version field equals declared verb version.

### DEV-M0-12 — `--explain` introspection surface (per verb manifest emission)
- **Tier:** T1 — **Anchor:** Spec §4 + Arch §1
- **Acceptance:** Snapshot test per verb.
- **Test cases:**
  - TC-12-01 (happy): each verb under `--explain` emits manifest with inputs, outputs, side-effects, gates touched.
  - TC-12-02 (regression): a side-effect added without manifest update fails CI.
  - TC-12-03 (false-positive guard): no-op verbs still emit a manifest (do not crash on empty side-effect set).

### DEV-M0-13 — `forge new <template>` happy-path
- **Tier:** T1 — **Anchor:** Spec §4
- **Acceptance:** `new-app` eval scenario passes (TEST-06).
- **Test cases:**
  - TC-13-01 (happy): scaffold runs and the resulting app builds.
  - TC-13-02 (negative): scaffold into non-empty dir fails unless `--force`.
  - TC-13-03 (idempotency): re-running with `--force` reproduces byte-identical output (excluding timestamps).
  - TC-13-04 (data-accuracy): rendered `.gitignore` and `.gitleaks.toml` pass TEST-22 / TEST-21 contracts.

### DEV-M0-14 — `forge doctor` (env health check)
- **Tier:** T1 — **Anchor:** Spec §4
- **Acceptance:** Detects missing deps with actionable `FORGE-XXXX`.
- **Test cases:**
  - TC-14-01 (happy): healthy environment exits 0 with summary.
  - TC-14-02 (negative): each detectable failure mode exits non-zero with one specific `FORGE-XXXX`.
  - TC-14-03 (data-accuracy): `--json` output enumerates every check with status + remediation hint.
  - TC-14-04 (false-positive guard): a benign warning (e.g. optional tool missing) does NOT fail the gate.

### DEV-M0-15 — `forge clean` MVP (manifest-based dry-run + `--apply`)
- **Tier:** T1 — **Anchor:** Spec §4 hygiene
- **Acceptance:** Hygiene fixture corpus passes; dry-run identity test.
- **Test cases:**
  - TC-15-01 (happy): every entry in the hygiene corpus is detected.
  - TC-15-02 (idempotency): dry-run twice → identical report; tree unchanged.
  - TC-15-03 (negative): `--apply` deleting a file outside the manifested patterns is impossible (allow-list semantics).
  - TC-15-04 (boundary): empty repo → zero findings, exit 0.
  - TC-15-05 (false-positive guard): an unmanifested but legitimate file (e.g. `README.md`) is never proposed for deletion.

### DEV-M0-16 — `ILlmProvider` interface + mock provider
- **Tier:** T1 — **Anchor:** Arch §9.1
- **Acceptance:** Provider compliance suite (v0) defined.
- **Test cases:**
  - TC-16-01 (happy): mock provider passes the v0 compliance suite.
  - TC-16-02 (negative): a deliberately broken mock (omits `streaming`) fails the suite.
  - TC-16-03 (boundary): empty prompt input returns spec-defined empty completion, not error.

### DEV-M0-17 — LLM gateway (single provider, no caching/routing yet)
- **Tier:** T1 — **Anchor:** Arch §9
- **Acceptance:** Integration test with mock; live-test gated by env var.
- **Test cases:**
  - TC-17-01 (happy): gateway proxies request → response unchanged.
  - TC-17-02 (negative): provider error surfaces as `FORGE-XXXX` (no raw provider error leaks).
  - TC-17-03 (data-accuracy): token counts from provider land in the ledger unchanged.
  - TC-17-04 (regression): live-test only runs when `FORGE_LIVE_LLM=1`.

### DEV-M0-18 — Token ledger (append-only)
- **Tier:** T1 — **Anchor:** Arch §9.2
- **Acceptance:** Cost per request asserted in test.
- **Test cases:**
  - TC-18-01 (happy): per-request entry written with prompt+completion tokens, model, cost.
  - TC-18-02 (data-accuracy): summed cost equals reference fixture total to ±$0.0001.
  - TC-18-03 (negative): a request with zero tokens still writes an entry (not skipped).

### DEV-M0-19 — First real provider plugin
- **Tier:** T2 — **Anchor:** Arch §9
- **Acceptance:** Provider compliance suite passes against live API (nightly only).
- **Test cases:**
  - TC-19-01 (happy): nightly compliance run green.
  - TC-19-02 (negative): provider 5xx is retried per policy and surfaces as `FORGE-XXXX` after exhaustion.
  - TC-19-03 (data-accuracy): provider's reported tokens match the ledger entry.
  - TC-19-04 (regression): API-version pin in the plugin manifest is asserted at startup.

### DEV-M0-20 — Manual `ship` checklist doc (pre-automation stand-in)
- **Tier:** DOC — **Anchor:** Spec §4 + DEV plan §0
- **Acceptance:** `CHECKLIST.md` referenced from CONTRIBUTING.md.
- **Verification:** doc lint passes; CONTRIBUTING.md link resolves; checklist mirrors spec §16.5.4 gate list.

### DEV-M0-21 — `forge ship --quick "..."` MVP
- **Tier:** T1 — **Anchor:** Spec §4
- **Acceptance:** `ship-reference` eval scenario passes (TEST-07).
- **Test cases:**
  - TC-21-01 (happy): reference change ships within budget.
  - TC-21-02 (negative): change that fails any §16.5.4 gate aborts with the gate name + remediation.
  - TC-21-03 (idempotency): re-running on no-op change exits 0 with "no changes".
  - TC-21-04 (boundary): change touching exactly the budgeted file count succeeds.

### DEV-M0-22 — Unit-test harness conventions
- **Tier:** T1 — **Anchor:** TEST plan §1
- **Acceptance:** Sample test for each module type.
- **Verification:** TEST-01 cases all pass on the harness.

### DEV-M0-23 — Integration-test harness (subprocess `forge`)
- **Tier:** T1 — **Anchor:** TEST plan §1
- **Acceptance:** At least one passing E2E test.
- **Verification:** TEST-02 cases all pass.

### DEV-M0-24 — Eval harness scaffold + `new-app` scenario
- **Tier:** T1 — **Anchor:** TEST plan §5
- **Acceptance:** Nightly run reports green.
- **Verification:** TEST-06 cases all pass.

### DEV-M0-25 — NFR benchmark suite scaffold
- **Tier:** T1 — **Anchor:** Arch §14
- **Acceptance:** Cold-start measured; baseline recorded.
- **Verification:** TEST-05 + TEST-11 cases all pass; baseline JSON committed.

### DEV-M0-26 — Secret-redaction regression test (100-run loop)
- **Tier:** T1 — **Anchor:** Arch §15
- **Acceptance:** Test in CI; fails on seeded leak.
- **Verification:** TEST-12 cases all pass.

### DEV-M0-27 — CI pipeline (lint + unit + integration + secret-scan + hygiene)
- **Tier:** OPS — **Anchor:** TEST plan §4
- **Acceptance:** CI green on main; required for merge.
- **Verification checks:** every §16.5.4 gate runs in CI; failure of any one gate blocks the PR; CI matrix covers linux/mac/win.

### DEV-M0-28 — Sigstore signing pipeline
- **Tier:** OPS — **Anchor:** Arch §15
- **Acceptance:** Release artifact signed; verification documented.
- **Verification checks:** post-build verification step runs `cosign verify` on every artifact; signature failure aborts release; doc walkthrough tested by a non-author.

### DEV-M0-29 — Brew/scoop/winget tap published
- **Tier:** OPS — **Anchor:** Arch §13 ADR-003
- **Acceptance:** Install matrix test passes.
- **Verification:** TEST-16 cases all pass.

### DEV-M0-30 — Telemetry opt-in plumbing (off by default)
- **Tier:** T1 — **Anchor:** Arch §11
- **Acceptance:** `forge telemetry` shows current state; payload printable via `--explain`.
- **Test cases:**
  - TC-30-01 (happy): default install reports `telemetry: off`.
  - TC-30-02 (negative): no payload is sent while off (network-mock asserts zero outbound).
  - TC-30-03 (data-accuracy): when on, payload matches the public schema (DEV-M3-12) byte-for-byte.
  - TC-30-04 (regression): a code path adding a new field without schema bump fails CI.

### DEV-M0-31 — M0 release notes + changelog automation
- **Tier:** DOC — **Anchor:** Spec §16.5 #5
- **Acceptance:** `BREAKING.md` + `CHANGELOG.md` generated from spec frontmatter.
- **Verification:** generator round-trip is reproducible; missing entries fail CI.

### DEV-M0-32 — Hygiene fixture corpus seeded (≥30 known LLM-scratch patterns)
- **Tier:** T1 — **Anchor:** Spec §4 hygiene
- **Acceptance:** Corpus committed; `forge clean` catches every entry.
- **Verification:** TEST-20 cases all pass; corpus size ≥30.

### DEV-M0-33 — `.gitignore` template fragments + composer
- **Tier:** T1 — **Anchor:** Spec §4 Repo Hygiene Layer (`.gitignore` standards)
- **Acceptance:** `forge new` emits version-stamped managed block + user section; mandatory hygiene block present; `.example`/`.template` negations preserved.
- **Test cases:**
  - TC-33-01 (happy): each fragment (`node`, `next`, `python`, `supabase`, `terraform`, `docker`, `os`, `editor`, `llm-scratch`) renders into the composed file.
  - TC-33-02 (boundary): selecting zero optional fragments still emits the mandatory block.
  - TC-33-03 (negative): unknown fragment name fails with `FORGE-XXXX`.
  - TC-33-04 (regression): rendered file passes TEST-22 contract test.
  - TC-33-05 (data-accuracy): managed-block version marker matches `forge --version` major.minor.

### DEV-M0-34 — `.gitleaks.toml` template with Forge-aware rule pack
- **Tier:** T1 — **Anchor:** Spec §4 Repo Hygiene Layer (`.gitleaks.toml` standards)
- **Acceptance:** `forge new` ships file; rule pack catches every entry in `tests/fixtures/secrets-corpus/` and zero false positives on the reference app.
- **Test cases:**
  - TC-34-01 (happy): TEST-21 positive fixtures all flagged.
  - TC-34-02 (false-positive guard): TEST-21 negative fixtures none flagged.
  - TC-34-03 (data-accuracy): rule pack version stamp matches CLI version.
  - TC-34-04 (regression): every Forge-aware rule has at least one fixture in TEST-21.

### DEV-M0-35 — `forge upgrade gitignore` + `forge upgrade gitleaks` codemods (idempotent; preserve user-owned section)
- **Tier:** T1 — **Anchor:** Spec §4 Repo Hygiene Layer
- **Acceptance:** Round-trip test: upgrade → noop diff; user section untouched across two version bumps.
- **Test cases:**
  - TC-35-01 (happy): TEST-26 cases all pass.
  - TC-35-02 (negative): `--force` is required to overwrite drift inside the managed block.
  - TC-35-03 (boundary): brand-new repo upgrade writes markers correctly.

### DEV-M0-36 — Secret-file guard list + `git ls-files` cross-check in `forge clean --check`
- **Tier:** T1 — **Anchor:** Spec §4 Repo Hygiene Layer + §16.5.4 #11
- **Acceptance:** Test fixture: tracked `.env.local` → fail; tracked `.env.local.example` → pass.
- **Test cases:** TEST-23 cases all pass.

---

## M1 — Workflow & Scan (DEV-M1-01 .. DEV-M1-41)

### DEV-M1-01 — `ship` workflow orchestrator (5 checkpoints, resumable)
- **Tier:** T1 — **Anchor:** Arch §6
- **Acceptance:** Resume-from-checkpoint test for each stage.
- **Test cases:**
  - TC-01-01 (happy): full ship runs end-to-end on the reference app.
  - TC-01-02 (idempotency): resuming after each checkpoint produces same final tree as a clean run.
  - TC-01-03 (negative): resume after corrupted checkpoint state fails fast with `FORGE-XXXX`, never silently re-runs.
  - TC-01-04 (concurrency): two ships on different branches do not share checkpoint state.
  - TC-01-05 (regression): kill -9 mid-checkpoint leaves a recoverable state (next `ship` resumes).

### DEV-M1-02 — Spec checkpoint
- **Tier:** T1 — **Anchor:** Spec §4 + §16.5.4 #1
- **Acceptance:** `forge ship verify` blocks on missing spec.
- **Test cases:**
  - TC-02-01 (happy): change with `.forge/specs/<change>/spec.md` proceeds.
  - TC-02-02 (negative): change without spec is blocked with the gate name + path expected.
  - TC-02-03 (boundary): docs-only change is exempt per spec policy (verified by manifest).
  - TC-02-04 (false-positive guard): a spec.md inside test fixtures does not satisfy the gate for an unrelated change.

### DEV-M1-03 — Test checkpoint: tests-precede-code timestamp guard
- **Tier:** T1 — **Anchor:** Spec §16.5.4 #2
- **Acceptance:** Code-before-test PR is blocked.
- **Test cases:**
  - TC-03-01 (happy): tests committed before code → pass.
  - TC-03-02 (negative): code committed before tests in same PR → blocked.
  - TC-03-03 (boundary): test + code in the same commit → pass (atomic exception).
  - TC-03-04 (false-positive guard): pure refactor with no behavior change is exempt by manifest flag.

### DEV-M1-04 — Breakdown checkpoint: tasks.md generation
- **Tier:** T1 — **Anchor:** Spec §4
- **Acceptance:** Each task has scope + principle ref.
- **Test cases:**
  - TC-04-01 (happy): generated tasks.md validates against schema.
  - TC-04-02 (negative): a task missing its principle ref fails the gate.
  - TC-04-03 (data-accuracy): each task's anchor resolves to a real spec section.

### DEV-M1-05 — Code checkpoint: per-task diff loop with re-test
- **Tier:** T1 — **Anchor:** Spec §4
- **Acceptance:** Loop terminates green or escalates with reason.
- **Test cases:**
  - TC-05-01 (happy): converges within max-iter on reference change.
  - TC-05-02 (negative): non-converging loop escalates after max-iter with the failing test's name.
  - TC-05-03 (idempotency): converged tree replays identically.
  - TC-05-04 (boundary): zero-iter (already green) exits 0 with no diff.

### DEV-M1-06 — Ship checkpoint: gate orchestration
- **Tier:** T1 — **Anchor:** Spec §4 + §16.5.4
- **Acceptance:** Each gate independently re-runnable.
- **Test cases:**
  - TC-06-01 (happy): all gates green → ship.
  - TC-06-02 (negative): each gate failure independently blocks ship and names the gate.
  - TC-06-03 (idempotency): re-run after fixing one gate re-runs only failed gates by default.
  - TC-06-04 (data-accuracy): per-gate timing reported in `--json`.

### DEV-M1-07 — LLM caching layer (semantic-hash key)
- **Tier:** T1 — **Anchor:** Arch §9.3
- **Acceptance:** Cache hit/miss test; invalidation on file change.
- **Test cases:**
  - TC-07-01 (happy): identical request → cache hit, zero provider call.
  - TC-07-02 (negative): file content change invalidates the entry.
  - TC-07-03 (boundary): two requests differing only in irrelevant whitespace hit (semantic equivalence).
  - TC-07-04 (concurrency): two parallel identical requests both succeed; only one provider call.
  - TC-07-05 (data-accuracy): hit's response is byte-identical to the original recorded one.

### DEV-M1-08 — LLM tier router (cheap-first, escalate on fail)
- **Tier:** T1 — **Anchor:** Arch §9.2
- **Acceptance:** Routing test with seeded provider responses.
- **Test cases:**
  - TC-08-01 (happy): cheap model succeeds → no escalation.
  - TC-08-02 (negative): cheap model fails validation → escalates to next tier.
  - TC-08-03 (boundary): all tiers fail → returns `FORGE-XXXX` with chain of attempts.
  - TC-08-04 (data-accuracy): ledger records every tier attempt with its cost.

### DEV-M1-09 — Budget guard (per-command + per-day)
- **Tier:** T1 — **Anchor:** Arch §9.2
- **Acceptance:** `FORGE-2401` on cap; rerun-with-`--budget` flow tested.
- **Test cases:**
  - TC-09-01 (happy): under budget → proceeds.
  - TC-09-02 (negative): exact-cap-+1 → blocked with `FORGE-2401`.
  - TC-09-03 (boundary): exact-cap → proceeds (`<=` semantics asserted).
  - TC-09-04 (idempotency): re-run with `--budget=N+1` proceeds where prior failed.
  - TC-09-05 (cross-tenant): two repos share no budget state.

### DEV-M1-10 — Hygiene checkpoint: auto-`forge clean --check` between Code and Ship
- **Tier:** T1 — **Anchor:** Spec §4 hygiene
- **Acceptance:** Ship blocks if unmanifested files remain.
- **Test cases:**
  - TC-10-01 (happy): clean tree → proceeds.
  - TC-10-02 (negative): seeded scratch file mid-ship → blocks at this checkpoint, not at the end.
  - TC-10-03 (idempotency): re-run after `forge clean --apply` proceeds.

### DEV-M1-11 — Scan engine kernel + `Finding` schema with confidence
- **Tier:** T1 — **Anchor:** Arch §10
- **Acceptance:** Schema test; finding-roundtrip test.
- **Test cases:**
  - TC-11-01 (happy): scanners emit findings; kernel aggregates by family.
  - TC-11-02 (boundary): zero-finding scan still emits a result envelope.
  - TC-11-03 (negative): malformed finding (missing `confidence`) is rejected by the kernel.
  - TC-11-04 (data-accuracy): roundtrip JSON ↔ object preserves every field.

### DEV-M1-12 — Scanner family: secrets
- **Tier:** T1 — **Anchor:** Arch §10
- **Acceptance:** Seeded-secret app: catches with confidence ≥0.9.
- **Test cases:** TEST-08 + TEST-21 cases all pass for this family.

### DEV-M1-13 — Scanner family: RLS / authz
- **Tier:** T1 — **Anchor:** Arch §10 + spec §16.5.6
- **Acceptance:** Seeded-RLS-bypass app: caught.
- **Test cases:**
  - TC-13-01 (happy): seeded-bypass detected with confidence ≥0.9.
  - TC-13-02 (cross-tenant): user-A → user-B leak fixture detected.
  - TC-13-03 (false-positive guard): clean RLS reference app produces zero findings.
  - TC-13-04 (regression): every prior repo RLS bug in the agent-system has a fixture here.

### DEV-M1-14 — Scanner family: prompt-injection
- **Tier:** T1 — **Anchor:** Arch §10
- **Acceptance:** OWASP LLM Top-10 #1 fixtures all caught.
- **Test cases:**
  - TC-14-01 (happy): each OWASP LLM-01 sub-pattern fixture is caught.
  - TC-14-02 (false-positive guard): a benign string containing the words "ignore previous" inside a quoted string literal is NOT flagged.
  - TC-14-03 (boundary): payload at the configured max-context is still scanned.

### DEV-M1-15 — Scanner family: supply-chain
- **Tier:** T1 — **Anchor:** Arch §10
- **Acceptance:** Seeded-vulnerable-dep app: caught.
- **Test cases:**
  - TC-15-01 (happy): known-CVE dep flagged.
  - TC-15-02 (boundary): version exactly equal to the patched version is NOT flagged.
  - TC-15-03 (negative): missing lockfile fails with `FORGE-XXXX`.
  - TC-15-04 (data-accuracy): finding includes CVE id + advisory link.

### DEV-M1-16 — `forge fix --apply` with confidence-tier behavior
- **Tier:** T1 — **Anchor:** Arch §10.2
- **Acceptance:** Diff-only at <0.9; auto-applied with `--apply` at ≥0.9.
- **Test cases:**
  - TC-16-01 (happy): finding ≥0.9 + `--apply` → diff applied; tree green.
  - TC-16-02 (boundary): finding at exactly 0.9 with `--apply` → applied (`>=` semantics).
  - TC-16-03 (negative): finding <0.9 + `--apply` → still diff-only; warning shown.
  - TC-16-04 (idempotency): re-run after apply → no further changes.
  - TC-16-05 (regression): every applied fix is recorded in the audit ledger.

### DEV-M1-17 — Waiver mechanism (`.forge/waivers/`)
- **Tier:** T1 — **Anchor:** Spec §16.5.4 #3
- **Acceptance:** Waiver requires rationale + expiry; expiry-warning test.
- **Test cases:**
  - TC-17-01 (happy): well-formed waiver suppresses the matching finding.
  - TC-17-02 (negative): waiver missing rationale or expiry rejected at parse time.
  - TC-17-03 (boundary): waiver expiring today is treated as expired (strict `<`).
  - TC-17-04 (regression): every waiver appears in the weekly digest (TEST-18 link).
  - TC-17-05 (false-positive guard): a waiver scoped to file A does not suppress the same rule firing on file B.

### DEV-M1-18 — Plugin loader (per ADR-002)
- **Tier:** T1 — **Anchor:** Arch §8.3
- **Acceptance:** Sandbox escape test; signature failure test.
- **Test cases:**
  - TC-18-01 (happy): signed, capability-conformant plugin loads.
  - TC-18-02 (negative): unsigned plugin rejected.
  - TC-18-03 (negative): tampered plugin (signature mismatch) rejected.
  - TC-18-04 (negative): plugin attempting un-granted capability blocked at runtime.
  - TC-18-05 (concurrency): TEST-09 + TEST-14 cases all pass.

### DEV-M1-19 — Capability + permission model
- **Tier:** T1 — **Anchor:** Arch §8.1–8.2
- **Acceptance:** Permission-denial test for each namespace.
- **Test cases:**
  - TC-19-01 (happy): each declared capability grants exactly its scope.
  - TC-19-02 (negative): each namespace (fs, net, proc, secrets, llm) has a denied-without-grant test.
  - TC-19-03 (boundary): wildcard grants (`fs:*`) are explicitly opt-in (gate at install).

### DEV-M1-20 — Plugin manifest schema + validator
- **Tier:** T1 — **Anchor:** Arch §7.3
- **Acceptance:** Invalid manifests rejected with `FORGE-XXXX`.
- **Test cases:**
  - TC-20-01 (happy): valid manifest accepted.
  - TC-20-02 (negative): each required field's absence has a fixture.
  - TC-20-03 (negative): unknown capability name rejected.
  - TC-20-04 (data-accuracy): semver field validated to spec.

### DEV-M1-21 — First in-tree scanner-plugin proof (`scanner-cost`)
- **Tier:** T2 — **Anchor:** Arch §8
- **Acceptance:** Loads + runs + reports findings.
- **Test cases:**
  - TC-21-01 (happy): loads under loader; finds at least one seeded cost anti-pattern.
  - TC-21-02 (false-positive guard): clean reference app → zero findings.

### DEV-M1-22 — First in-tree generator-plugin proof (`gen-endpoint`)
- **Tier:** T2 — **Anchor:** Arch §8
- **Acceptance:** Generates code that passes scans.
- **Test cases:**
  - TC-22-01 (happy): generated endpoint compiles and passes secrets/RLS scans.
  - TC-22-02 (idempotency): re-running with same input → identical output.
  - TC-22-03 (negative): conflicting endpoint name → fails with `FORGE-XXXX`, no partial write.

### DEV-M1-23 — `forge lint` reads `.forge/instructions/` packs
- **Tier:** T1 — **Anchor:** Spec §11.2 + Arch §5
- **Acceptance:** Lints reference app green.
- **Test cases:**
  - TC-23-01 (happy): reference app lints clean.
  - TC-23-02 (negative): seeded anti-pattern → lint fails with rule id + remediation.
  - TC-23-03 (boundary): empty pack → lint is a no-op pass.
  - TC-23-04 (false-positive guard): a string literal that *resembles* an anti-pattern in user-facing copy is not flagged.

### DEV-M1-24 — First defaults instructions pack
- **Tier:** T1 — **Anchor:** Spec §11.2
- **Acceptance:** Pack referenced by both linter and LLM context builder.
- **Test cases:**
  - TC-24-01 (happy): linter consumes pack; LLM context builder embeds pack.
  - TC-24-02 (data-accuracy): pack version pinned in both consumers.
  - TC-24-03 (regression): pack-format change without consumer bump fails CI.

### DEV-M1-25 — New-pattern detection + RFC-link suggestion in lint output
- **Tier:** T1 — **Anchor:** Spec §16.5.4 #4
- **Acceptance:** Lint failure cites which convention rule + how to amend.
- **Test cases:**
  - TC-25-01 (happy): fail message includes rule id + RFC link template.
  - TC-25-02 (negative): a new pattern not matching any rule is reported as "new" with how-to-RFC instructions.
  - TC-25-03 (false-positive guard): an existing accepted pattern in the reference app is not flagged as "new".

### DEV-M1-26 — Spec-presence gate in CI
- **Tier:** OPS — **Anchor:** Spec §16.5.4 #1
- **Acceptance:** Blocks PR without `.forge/specs/<change>/spec.md`.
- **Verification:** TEST-02 (gate exit-code propagation) + DEV-M1-02 cases applied at CI tier.

### DEV-M1-27 — Tests-precede-code gate in CI
- **Tier:** OPS — **Anchor:** Spec §16.5.4 #2
- **Acceptance:** Blocks PR violating timestamp invariant.
- **Verification:** DEV-M1-03 cases applied at CI tier; bypass attempt via rebase is detected.

### DEV-M1-28 — `forge scan all --since main` gate
- **Tier:** OPS — **Anchor:** Spec §16.5.4 #3
- **Acceptance:** Blocks high-confidence findings.
- **Verification:** TEST-08 cases must be green for the PR to merge; waivers honored per DEV-M1-17.

### DEV-M1-29 — Convention lint gate
- **Tier:** OPS — **Anchor:** Spec §16.5.4 #4
- **Acceptance:** Blocks new anti-patterns.
- **Verification:** DEV-M1-23 + DEV-M1-25 cases enforced at CI tier.

### DEV-M1-30 — Public-API delta declaration gate
- **Tier:** OPS — **Anchor:** Spec §16.5.4 #5
- **Acceptance:** API-diff tool detects undeclared breaks.
- **Verification:** seed an undeclared break in a test PR → gate blocks with the diff cited; declared break (with `BREAKING.md` entry) → gate passes.

### DEV-M1-31 — Token-budget regression gate
- **Tier:** OPS — **Anchor:** Spec §16.5.4 #6
- **Acceptance:** `forge eval` diff >10% blocks.
- **Verification:** TEST-07 cases enforced; +11% synthetic diff blocks; +9% passes (boundary).

### DEV-M1-32 — Docs-sync gate
- **Tier:** OPS — **Anchor:** Spec §16.5.4 #7
- **Acceptance:** `forge docs sync --check` clean required.
- **Verification:** seeded out-of-sync doc → blocks; clean → passes; doc-only no-code change still validated.

### DEV-M1-33 — DCO + signed-commit gate
- **Tier:** OPS — **Anchor:** Spec §16.5.4 #8
- **Acceptance:** Branch-protection rule active.
- **Verification:** unsigned commit blocked; signed + DCO-signed → passes; expired signing key surfaces clear error.

### DEV-M1-34 — Repo-hygiene gate (`forge clean --check`)
- **Tier:** OPS — **Anchor:** Spec §16.5.4 #11
- **Acceptance:** Blocks PR with unmanifested LLM scratch.
- **Verification:** TEST-13 + TEST-23 cases enforced at CI tier.

### DEV-M1-35 — Secrets-clean gate (`forge scan security --secrets --since main`, gitleaks)
- **Tier:** OPS — **Anchor:** Spec §16.5.4 #12
- **Acceptance:** Blocks PR with secret findings; allowlist `# review-by:` expiry enforced.
- **Verification:** TEST-21 + TEST-24 cases enforced at CI tier.

### DEV-M1-36 — Pre-commit secret scan + `gitleaks-bypass: <reason>` token surfaced in PR
- **Tier:** OPS — **Anchor:** Spec §4 Repo Hygiene Layer (`.gitleaks.toml` standards)
- **Acceptance:** Bypassed commit visible in PR template + audit log.
- **Verification:** commit with bypass token → PR template lists it under "Bypasses requiring review"; commit without token + dirty pre-commit → blocks locally; reason missing → bypass token rejected.

### DEV-M1-37 — Allowlist-expiry sweeper (nightly) opens auto-PR to remove expired entries
- **Tier:** OPS — **Anchor:** Spec §16.5.4 #12
- **Acceptance:** Cron job + auto-PR validated on staging repo.
- **Verification:** seeded expired entry on staging → auto-PR opens within 24h; PR title cites entry; no-expired-entries night → no PR opened.

### DEV-M1-38 — `.gitignore` drift detection in `forge doctor`
- **Tier:** T1 — **Anchor:** Spec §4 Repo Hygiene Layer
- **Acceptance:** Modified managed block triggers `forge upgrade gitignore` suggestion.
- **Test cases:**
  - TC-38-01 (happy): unmodified managed block → doctor passes silently.
  - TC-38-02 (negative): hand-edit inside managed block → doctor reports drift + suggests `forge upgrade gitignore`.
  - TC-38-03 (boundary): drift at the marker boundary (e.g. blank line added) is detected.
  - TC-38-04 (false-positive guard): user-section edits never trigger drift.

### DEV-M1-39 — Resilience-pattern library + lint enforcement of §17.4 invariants
- **Tier:** T1 — **Anchor:** Arch §17.1 + §17.4 + ARCH-DEC-14
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

## M2 — Ecosystem (DEV-M2-01 .. DEV-M2-25)

### DEV-M2-01 — Plugin Registry index (signed JSON in Git) + CDN
- **Tier:** T1 + OPS — **Anchor:** Arch §3 C4
- **Acceptance:** Mirror-able; signature verification end-to-end.
- **Test cases:**
  - TC-01-01 (happy): client validates signed index against trust root.
  - TC-01-02 (negative): tampered index rejected with `FORGE-XXXX`.
  - TC-01-03 (boundary): air-gapped mirror works (TEST-16-02 link).
  - TC-01-04 (data-accuracy): index entries' SHA matches CDN-served artifact.

### DEV-M2-02 — `forge plugin install/list/upgrade/remove` verbs
- **Tier:** T1 — **Anchor:** Spec §4 + §20
- **Acceptance:** Lock file pinning; reproducible install test.
- **Test cases:**
  - TC-02-01 (happy): install → list shows it; upgrade → version bump in lock; remove → cleared.
  - TC-02-02 (idempotency): install with same lock entry on a fresh machine → byte-identical install tree.
  - TC-02-03 (negative): install of an unpinned version when lock exists fails per policy.
  - TC-02-04 (boundary): install of zero plugins is a no-op exit 0.

### DEV-M2-03 — Plugin compliance test runner (per capability)
- **Tier:** T1 — **Anchor:** Arch §8
- **Acceptance:** Authoring guide cites the runner.
- **Test cases:** TEST-03 cases all pass.

### DEV-M2-04 — Second LLM provider plugin
- **Tier:** T2 — **Anchor:** Arch §9
- **Acceptance:** Compliance suite green.
- **Test cases:** mirror DEV-M0-19 cases for the second provider.

### DEV-M2-05 — Deploy adapter: target #1
- **Tier:** T2 — **Anchor:** Spec §4 deploy + Arch §5 L6
- **Acceptance:** Reference app deploys + rollbacks.
- **Test cases:**
  - TC-05-01 (happy): deploy + smoke + rollback round-trip green.
  - TC-05-02 (negative): deploy of broken artifact rolls back automatically; failure code propagated.
  - TC-05-03 (idempotency): re-deploy of same SHA → noop.
  - TC-05-04 (data-accuracy): deployed version reports the expected commit SHA.

### DEV-M2-06 — Deploy adapter: target #2
- **Tier:** T2 — **Anchor:** Spec §4 deploy
- **Acceptance:** Reference app deploys + rollbacks.
- **Test cases:** mirror DEV-M2-05.

### DEV-M2-07 — Storage adapter: target #1
- **Tier:** T2 — **Anchor:** Arch §5 L1
- **Acceptance:** Adapter compliance suite green.
- **Test cases:**
  - TC-07-01 (happy): put/get/list/delete round-trip.
  - TC-07-02 (boundary): zero-byte object handled.
  - TC-07-03 (cross-tenant): two tenants' buckets isolated (no cross-read).
  - TC-07-04 (concurrency): two parallel writers to the same key resolve per provider semantics + assert outcome.

### DEV-M2-08 — Eval harness: public scenario format + 7 reference scenarios
- **Tier:** T1 — **Anchor:** TEST plan §5
- **Acceptance:** All scenarios green nightly.
- **Test cases:** TEST-06 .. TEST-13 all green nightly; one synthetic failing scenario is detected.

### DEV-M2-09 — Learning loop client (opt-in) — share path
- **Tier:** T1 — **Anchor:** Arch §10.3
- **Acceptance:** Privacy invariant test (no source code in payload).
- **Test cases:**
  - TC-09-01 (happy): opted-in payload contains only allowed fields per schema.
  - TC-09-02 (negative): source-code byte never appears in any payload across 100 fuzzed scenarios.
  - TC-09-03 (regression): historical leakage incidents have fixtures here.
  - TC-09-04 (false-positive guard): a comment that *quotes* a known phrase is still excluded as source.

### DEV-M2-10 — Learning loop aggregator MVP
- **Tier:** OPS — **Anchor:** Arch §3 C7
- **Acceptance:** Nightly digest produced; opt-out respected.
- **Verification:** opt-out user's id never appears in digest; cron emits digest each night.

### DEV-M2-11 — Scanner family: auth
- **Tier:** T1 — **Anchor:** Arch §10
- **Acceptance:** Seeded auth-bypass app: caught.
- **Test cases:** mirror DEV-M1-13 with auth-bypass fixtures.

### DEV-M2-12 — Scanner family: perf
- **Tier:** T1 — **Anchor:** Arch §10
- **Acceptance:** N+1, missing-index fixtures caught.
- **Test cases:**
  - TC-12-01 (happy): N+1 fixture caught; missing-index fixture caught.
  - TC-12-02 (false-positive guard): batch query intentionally hitting N rows once is NOT flagged as N+1.
  - TC-12-03 (boundary): query at exactly the N+1 detection threshold is flagged per spec.

### DEV-M2-13 — Scanner family: accessibility
- **Tier:** T1 — **Anchor:** Arch §10
- **Acceptance:** WCAG-AA fixtures caught.
- **Test cases:**
  - TC-13-01 (happy): each WCAG-AA category has a positive fixture caught.
  - TC-13-02 (false-positive guard): compliant components produce zero findings.

### DEV-M2-14 — Scanner family: cost (cloud + LLM)
- **Tier:** T1 — **Anchor:** Arch §10
- **Acceptance:** Anti-pattern fixtures caught.
- **Test cases:**
  - TC-14-01 (happy): unindexed scan / chatty LLM loop fixtures caught.
  - TC-14-02 (data-accuracy): finding includes estimated $ delta.
  - TC-14-03 (false-positive guard): one-shot batch jobs are not flagged as chatty.

### DEV-M2-15 — `forge upgrade` codemod runner
- **Tier:** T1 — **Anchor:** Spec §16.5.6
- **Acceptance:** One real codemod (a M1→M2 deprecation) shipped.
- **Test cases:**
  - TC-15-01 (happy): codemod transforms the deprecated pattern in the reference app.
  - TC-15-02 (idempotency): re-running codemod → no diff.
  - TC-15-03 (negative): codemod on already-migrated code → exits 0 with "nothing to do".
  - TC-15-04 (regression): rollback path documented + tested.

### DEV-M2-16 — Backward-compat alias mechanism
- **Tier:** T1 — **Anchor:** Spec §16.5.4 #9
- **Acceptance:** Test: deprecated verb still works with warning.
- **Test cases:**
  - TC-16-01 (happy): deprecated verb runs and emits deprecation warning with replacement.
  - TC-16-02 (boundary): one-minor retention enforced (verb removed at minor+2).
  - TC-16-03 (data-accuracy): warning includes upgrade codemod reference.

### DEV-M2-17 — Performance benchmark gate (≤5% regression)
- **Tier:** OPS — **Anchor:** Spec §16.5.6
- **Acceptance:** CI blocks regressions.
- **Verification:** TEST-05 cases enforced at CI; ≤5% baseline drift = pass; >5% = block; baseline shift requires explicit `--accept-baseline`.

### DEV-M2-18 — Plugin-loader sandbox audit + fuzz
- **Tier:** T1 — **Anchor:** Arch §15
- **Acceptance:** Fuzz suite in nightly; no escape findings.
- **Test cases:** TEST-14 cases all pass.

### DEV-M2-19 — First three external community plugins published
- **Tier:** T3 — **Anchor:** Spec §16.5.1
- **Acceptance:** Listed in Registry; compliance suites green.
- **Verification:** each external plugin passes TEST-03 compliance suite; signed by author.

### DEV-M2-20 — Eval harness used to gate at least one PR (proof-of-life)
- **Tier:** OPS — **Anchor:** DEV plan §6
- **Acceptance:** Issue + PR linked in changelog.
- **Verification:** at least one historical PR shows eval-gate status check; changelog cites it.

### DEV-M2-21 — Pilot user runs `forge ship` end-to-end on production app
- **Tier:** DOC — **Anchor:** DEV plan §3.2 exit
- **Acceptance:** Case study published.
- **Verification:** case study lives at a public URL; pilot user co-authored.

### DEV-M2-22 — Migration runner: forward + reverse + double-apply tests
- **Tier:** T1 — **Anchor:** TEST plan §2
- **Acceptance:** All three pass on reference migration.
- **Test cases:** TEST-10 cases all pass.

### DEV-M2-23 — Chaos-drill harness for the 8 §17.3 cross-cutting scenarios
- **Tier:** T1 — **Anchor:** Arch §17.3 + ARCH-DEC-15 + OPS-17
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
- **Acceptance:** Status page reachable; webhook publishes state transitions (identified → mitigated → fixed → post-mortem published) to TEST-19 dashboard; transitions are signed + replay-protected.
- **Test cases:**
  - TC-25-01 (happy): a synthesised incident moves through all 4 states; dashboard reflects each within 60s.
  - TC-25-02 (negative): a webhook with an invalid signature is rejected; nothing surfaces on the dashboard.
  - TC-25-03 (idempotency): the same transition delivered twice (replay) is recorded once.
  - TC-25-04 (boundary): an incident closed without `post-mortem published` for > SLA fires an OPS-18 alert.
  - TC-25-05 (concurrency): two simultaneous incidents do not entangle state.
  - TC-25-06 (false-positive guard): a normal deploy does not auto-create an incident.

---

## M3 — Hardening & 1.0 (DEV-M3-01 .. DEV-M3-22)

### DEV-M3-01 — `THREAT_MODEL.md` complete (STRIDE per Arch §15)
- **Tier:** T1 — **Anchor:** Arch §15
- **Acceptance:** Each threat has a tested mitigation.
- **Verification:** every threat in the doc links to a TC-id (existing or new); CI fails if any threat lacks a linked test.

### DEV-M3-02 — External pentest of CLI + plugin loader
- **Tier:** OPS — **Anchor:** Spec §16.5.6
- **Acceptance:** Findings triaged; criticals fixed.
- **Verification:** report archived; each critical mapped to a fix PR + a regression TC under the matching scanner family.

### DEV-M3-03 — Bug-bounty program live
- **Tier:** OPS — **Anchor:** Launch §11
- **Acceptance:** huntr.dev or similar; scope documented.
- **Verification:** scope doc public; first valid report acknowledged within SLA.

### DEV-M3-04 — All NFR budgets (Arch §14) asserted in CI
- **Tier:** OPS — **Anchor:** Spec §16.5.6
- **Acceptance:** CI dashboard shows all budgets green.
- **Verification:** every Arch §14 budget has a CI gate; baseline + delta tracked per release.

### DEV-M3-05 — Docs site complete: every verb, every error code, every extension point
- **Tier:** DOC — **Anchor:** Spec §16.5.4 #7
- **Acceptance:** `forge docs coverage` reports 100%.
- **Verification:** coverage tool fails CI below 100%; sample link-check passes.

### DEV-M3-06 — RFC process operational with ≥3 accepted RFCs
- **Tier:** DOC — **Anchor:** Spec §16.2
- **Acceptance:** Public RFC archive.
- **Verification:** archive lists ≥3 accepted RFCs with linked PRs.

### DEV-M3-07 — Contribution-standards CI bot live
- **Tier:** OPS — **Anchor:** Spec §16.5.4 + §16.5.7
- **Acceptance:** Bot auto-comments which gate failed + doc link.
- **Verification:** seeded failing PR → bot comment within 60s naming the gate; passing PR → no spurious comment.

### DEV-M3-08 — Maintainer review-SLA dashboard
- **Tier:** OPS — **Anchor:** Spec §16.5.7
- **Acceptance:** Public; alerts on breach.
- **Verification:** dashboard live; synthetic SLA breach triggers alert in <1h.

### DEV-M3-09 — T2 adapter coverage: ≥top 5 cloud + top 3 LLM providers
- **Tier:** T2 — **Anchor:** Launch §8
- **Acceptance:** Compliance suites green.
- **Verification:** each adapter passes TEST-03 + family-specific compliance; matrix published.

### DEV-M3-10 — Performance regression test gates locked
- **Tier:** OPS — **Anchor:** Spec §16.5.6
- **Acceptance:** History tracked per release.
- **Verification:** baseline JSON committed per release; trend chart on dashboard.

### DEV-M3-11 — i18n scaffolding (English-only at 1.0; structure ready)
- **Tier:** T1 — **Anchor:** Arch §11
- **Acceptance:** All user-facing strings centralized.
- **Test cases:**
  - TC-11-01 (happy): every user-facing string resolved through the catalog.
  - TC-11-02 (negative): a hard-coded string in source fails the i18n lint.
  - TC-11-03 (data-accuracy): catalog round-trip preserves placeholders.

### DEV-M3-12 — Telemetry payload audit + public schema
- **Tier:** OPS — **Anchor:** Arch §11 + §13 ADR-006
- **Acceptance:** Schema versioned; opt-in mechanics tested.
- **Verification:** DEV-M0-30 cases at integration tier; schema doc public.

### DEV-M3-13 — Air-gapped install path documented + tested
- **Tier:** OPS — **Anchor:** Arch §12
- **Acceptance:** Mirror walkthrough verified.
- **Verification:** TEST-16-02 case green; doc walkthrough validated by a non-author.

### DEV-M3-14 — All §16.5.4 gates active and required for merge
- **Tier:** OPS — **Anchor:** Spec §16.5
- **Acceptance:** Branch protection updated.
- **Verification:** branch-protection JSON committed; each gate listed; tested by a deliberately failing PR per gate.

### DEV-M3-15 — v1.0.0 release artifact + signing + tap update
- **Tier:** OPS — **Anchor:** Arch §13
- **Acceptance:** Reproducible build verified.
- **Verification:** two independent builders produce byte-identical artifact; signature verified end-to-end.

### DEV-M3-16 — Post-1.0 deprecation policy doc
- **Tier:** DOC — **Anchor:** Spec §16.5.4 #9
- **Acceptance:** One-minor alias retention codified.
- **Verification:** doc published; first deprecation test follows the policy.

### DEV-M3-17 — Status page + incident runbook
- **Tier:** OPS — **Anchor:** Launch §11
- **Acceptance:** Tabletop exercise completed.
- **Verification:** tabletop minutes archived; runbook revision committed.

### DEV-M3-18 — "What changed since beta" launch post
- **Tier:** DOC — **Anchor:** Launch §8
- **Acceptance:** Co-authored with one community maintainer.
- **Verification:** post published; co-author named.

### DEV-M3-19 — Private-vulnerability intake + `SECURITY.md` + disclosure workflow
- **Tier:** OPS — **Anchor:** Arch §18.1 vulnerability row + Spec §15 + ARCH-DEC-18
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

*Task file version: 0.4 — companion to spec v0.10.9.*
