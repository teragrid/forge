# Forge — Test Plan

> Companion to `FORGE_FRAMEWORK_SPEC.md` v0.10.6, `ARCHITECTURE.md` v0.1, and `DEVELOPMENT_PLAN.md` v0.1.
> Status: **Draft / Pre-RFC**.

This plan codifies the test pyramid, per-change-type test obligations, CI gates, eval scenarios, environments, and the live quality dashboard. It is the testing counterpart to the [DEVELOPMENT_PLAN.md](DEVELOPMENT_PLAN.md) and is enforced by the universal CI gates in spec §16.5.4.

---

## 0. Operating principles (testing-specific)

1. **Test design before code.** A test design (per `always-write-tests.md`) is committed to `.forge/specs/<change>/test-design.md` *before* the failing test is written. CI enforces presence.
2. **Failing test before code.** Spec → red test → green code. Pre-`ship` checklist (M0) enforces it; `forge ship` automates it from M1 (§16.5.4 #2).
3. **Every bug fix = one regression test.** The regression test must FAIL on pre-fix code and PASS on post-fix code. Enforced by PR template + reviewer checklist.
4. **Performance is a test.** All NFR budgets (Arch §14) are CI assertions, not aspirations.
5. **Privacy is a test.** Secret-redaction, learning-loop payload shape, telemetry payload shape — each has a non-skippable test.
6. **Repo hygiene is a test.** `forge clean --check` runs in PR-CI and fails the build if unmanifested LLM scratch is present (spec §16.5.4 #11).
7. **No flake budget.** Flaky tests are quarantined within 24h or deleted. There is no "rerun until green" knob.

---

## 1. Test pyramid

```
                        ┌───────────────────┐
                        │  Eval scenarios   │   ← cost+quality regression gates
                        │  (reference apps) │     (per spec §16.5.6)
                        └───────────────────┘
                       ┌───────────────────────┐
                       │  Integration tests    │   ← real LLM calls (mocked in PR-CI,
                       │  (subprocess `forge`) │     real on nightly + pre-release)
                       └───────────────────────┘
                     ┌───────────────────────────┐
                     │  Contract tests           │   ← every plugin interface
                     │  (provider, scanner, etc.)│     has a compliance suite
                     └───────────────────────────┘
                  ┌───────────────────────────────────┐
                  │  Unit tests (per module)          │
                  └───────────────────────────────────┘
```

---

## 2. Test types per change type

Per `always-write-tests.md` and spec §16.5.4:

| Change type | Required tests |
|-------------|----------------|
| Foundation module (config, fs, errors) | Unit + boundary + negative + concurrency |
| CLI verb addition | Unit (parser) + integration (subprocess) + `--explain` snapshot + `--json` schema test + **user-journey test** (add a compound step to `internal/cli/journey_test.go`) |
| LLM provider adapter | Provider compliance suite + cost-ledger assertion + cache hit/miss test |
| Scanner | Rule unit tests + false-positive guard + confidence-threshold test + perf budget |
| Workflow checkpoint | Integration (happy + each failure mode) + idempotency (re-run) + resume-from-checkpoint test |
| Plugin loader change | Sandbox escape test + signature failure test + permission denial test |
| Migration / codemod | Forward + reverse + double-apply (idempotent) + on-malformed-input |
| Bug fix | Regression test that FAILS on pre-fix code, PASSES on post-fix code |
| Security-sensitive (auth/RLS/secrets/webhooks) | All of above + threat-model assertion (e.g. "secret never appears in logged context") |
| Hygiene rule (`forge clean`) | Manifest match/miss test + dry-run identity + `--apply` rollback + LLM-scratch fixture corpus |

### 2a. User-journey tests (multi-verb compound flows)

Every new verb or significant state-transition change **must** be represented in at least one
compound journey test in `internal/cli/journey_test.go`.  A journey test:

- Chains **two or more verbs** against a shared temp directory so that state
  written by step N is consumed by step N+1.
- Covers the **9-point checklist** (happy path, boundary, negative, idempotency,
  concurrency isolation, cross-journey isolation, regression, data-accuracy,
  false-positive guard) as documented at the top of `journey_test.go`.
- Is named `TestJourney_<WorkflowName>` and uses `t.Parallel()`.

Current journeys (M0):

| Journey | Steps | Guard |
|---------|-------|-------|
| `TestJourney_DeveloperOnboarding` | `new` → `doctor` → `scan secrets` → `ship --dry-run` (×2 idempotency) | fresh scaffold has no secrets |
| `TestJourney_IncidentLifecycle` | `incident new` → `update state` → `update note` → `list --open` → `close` → `list --open` | closed incident absent from `--open` |
| `TestJourney_HygienePipeline` | `new` → `lint` → `scan secrets` → `upgrade list` → `upgrade gitignore-marker` (dry-run) | fresh scaffold has no secrets |
| `TestJourney_BudgetAndObservability` | `spend set` → `spend status` → `audit append` → `audit show` (×2 idempotency) → `insights` | scan verb appears in rollup |
| `TestJourney_TelemetryConsent` | `telemetry status` → `enable` → `status` → `rotate-id` → `disable` → `status` | enabled=false after disable |
| `TestJourney_PluginLifecycle` | `plugin list` → `list --kind scanner` → `show secrets` → `install` → `upgrade` → `remove` → `upgrade` (fails) → `list` (idempotency) | removed plugin cannot be upgraded |

---

## 3. Test-design-first checklist (mandatory before coding any change)

For every change, the contributor produces a brief test design covering:
1. **Happy path** — the intended scenario succeeds.
2. **Boundary** — empty/null, zero, max, min, exactly-at-threshold, off-by-one.
3. **Negative** — invalid input, unauthorized, wrong tier/tenant, expired/cancelled state.
4. **Idempotency / replay** — same operation twice; webhook double-delivery; retry after partial failure.
5. **Concurrency / race** — two writers; out-of-order events; polling mid-state.
6. **Cross-tenant / authz** — workspace A cannot affect workspace B; RLS verifies.
7. **Backward-compat / regression** — exact pre-fix bug reproduction stays in the suite as a guard.
8. **Data-accuracy** — real I/O round-trip; assert numeric/temporal correctness (not just "no error").
9. **False-positive guard** — at least one case where the new check MUST NOT trigger.

Committed to `.forge/specs/<change>/test-design.md`. CI enforces presence.

---

## 4. CI gates (per universal standards §16.5.4)

| # | Gate | Tool | Blocks PR? |
|---|------|------|------------|
| 1 | Spec present + tests-precede-code | `forge ship verify` (manual checklist pre-M1) | yes |
| 2 | Unit + integration tests pass | native runner | yes |
| 3 | `forge scan all --since main` clean | scan engine | yes (high-confidence findings) |
| 4 | `forge lint` (convention) | linter | yes |
| 5 | Public-API delta declared | `BREAKING.md` diff check | yes |
| 6 | Token budget delta ≤ 10% | `forge eval` | yes |
| 7 | Docs in sync | `forge docs sync --check` | yes |
| 8 | DCO + (T1) signed commits | DCO bot | yes |
| 9 | Backward-compat (deprecation alias present) | API diff tool | yes for breaking |
| 10 | Performance budget (≤5% regression) | benchmark harness | yes for T1 |
| 11 | Repo-hygiene clean (`forge clean --check`) | hygiene engine | yes |
| 12 | Security review label (when triggered) | manual + label bot | yes for T1 sensitive |

---

## 5. Eval harness scenarios (run nightly + pre-release)

| Scenario | Asserts |
|----------|---------|
| `new-app` | `forge new` produces running app in ≤60s with all defaults applied |
| `ship-reference` | Reference change ships in ≤5min, ≤$0.20 tokens |
| `scan-seeded-vulns` | All 8 scanner families catch their seeded vulnerabilities |
| `plugin-load` | 10 plugins load + run a verb in ≤500ms total |
| `migration-roundtrip` | Apply + rollback + re-apply produces identical schema |
| `cold-start` | CLI cold-start ≤80ms |
| `secret-redaction` | Seeded secrets never appear in any LLM payload across 100 runs |
| `repo-hygiene` | After 50 simulated `ship` cycles, repo has zero unmanifested files |

---

## 6. Test environments

| Env | Purpose | Trigger | LLM calls |
|-----|---------|---------|-----------|
| Local dev | Contributor inner loop | manual | mocked or live (their own key) |
| PR-CI | Per-push gates | every push | mocked (recorded fixtures) |
| Nightly-CI | Eval + integration | cron 02:00 UTC | live, against pinned models |
| Pre-release | Full eval matrix + perf benchmarks | release branch | live, every supported model |
| Canary | Released binary tested by 5 internal projects for 1 week | post-release | live |

LLM fixture management: `cassettes/` directory of recorded interactions, refreshed weekly on nightly-CI; fixture diffs reviewed.

---

## 7. Quality dashboard (visible from M1 onward)

Tracked weekly:

| Metric | Target | Source |
|--------|--------|--------|
| Test count | grows monotonically | runner |
| Coverage (line) | ≥85% core, ≥70% plugins | runner |
| Mean time to merge (T1/T2/T3 PRs) | ≤14/21/30 days | GH API |
| Eval scenario pass rate | 100% on main | nightly-CI |
| Token cost per `forge ship` reference | trending down | eval harness |
| Open security findings (high) | 0 | scan engine |
| Open security findings (med) | ≤5 | scan engine |
| Bug-regression coverage (% bug fixes with regression test) | 100% | PR template enforcement |
| Flaky-test count (open >24h) | 0 | runner + tracker |
| Hygiene violations on PRs | 0 | `forge clean --check` |

---

## 8. Test-investment milestones (mirrors `DEVELOPMENT_PLAN.md` §3)

| Milestone | Test-investment headline |
|-----------|--------------------------|
| **M0** | Unit + integration harnesses; `new-app` + `secret-redaction` + `cold-start` evals; provider contract suite v0; hygiene fixture corpus seeded |
| **M1** | Workflow-checkpoint integration suite; 4 scanner families with false-positive guards; `ship-reference` + `scan-seeded-vulns` evals live; hygiene gate live |
| **M2** | Plugin compliance suites for each tier; learning-loop privacy invariant tests; `plugin-load` + `migration-roundtrip` evals; `repo-hygiene` eval at scale |
| **M3** | Threat-model assertion tests; full NFR budget gates; flake budget = 0 enforced; eval coverage ≥95% of public verbs |

---

*Plan version: 0.1 — companion to spec v0.10.6, architecture v0.1, development plan v0.1.*
