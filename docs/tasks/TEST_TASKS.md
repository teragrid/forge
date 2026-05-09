# Forge — Test Tasks

> Companion to `../TEST_PLAN.md`.
> Tracker for the Section-C tasks (TEST-NN) from the master breakdown.

ID and conventions follow `ARCHITECTURE_TASKS.md`. Each task lists explicit **test cases** (TC-IDs) following the 9-point checklist from `always-write-tests.md` (happy / boundary / negative / idempotency / concurrency / cross-tenant / regression / data-accuracy / false-positive guard). TC-IDs are stable and never reused.

---

## TEST-01 — Unit-test framework selection + conventions doc

- **Anchor:** TEST plan §1, ADR-001
- **Stack baseline (per ADR-001):** `go test` + `gotestsum` + golden-file snapshots; `-race` mandatory in CI; `testify` permitted only where assertion verbosity hurts readability.
- **Acceptance:** First module's test passes; conventions doc committed; `go test -race ./...` is wired into the PR gate.
- **Test cases:**
  - TC-01-01 (happy): a sample passing test runs under the chosen runner.
  - TC-01-02 (boundary): empty test file produces a clear "no tests" exit code, not a crash.
  - TC-01-03 (negative): syntactically broken test file fails fast with line:col error.
  - TC-01-04 (data-accuracy): sample test asserting a known numeric (`2+2==4`) round-trips through the runner reporter unchanged.
  - TC-01-05 (false-positive guard): a deliberately failing test in the same suite is reported failing — proves the runner is not silently skipping.

## TEST-02 — Integration-test harness (`forge` subprocess fixture)

- **Anchor:** TEST plan §1
- **Acceptance:** Reusable fixture; first E2E green.
- **Test cases:**
  - TC-02-01 (happy): `forge --version` via subprocess returns expected version + exit 0.
  - TC-02-02 (boundary): zero-byte stdin to a piped `forge` verb does not deadlock.
  - TC-02-03 (negative): unknown verb exits non-zero with `FORGE-XXXX` code on stderr.
  - TC-02-04 (idempotency): running the same read-only verb twice produces byte-identical `--json` output.
  - TC-02-05 (concurrency): two parallel subprocess invocations of an idempotent verb both succeed (no global lock contention).
  - TC-02-06 (regression): kill -9 mid-run leaves no orphan child processes.
  - TC-02-07 (false-positive guard): verb that *should* exit non-zero (e.g. invoked outside a project) does so, proving the harness propagates real failures.

## TEST-03 — Contract-test framework for plugin interfaces

- **Anchor:** TEST plan §1; spec §20
- **Acceptance:** Each tier (`ILlmProvider`, `IScanner`, `IDeployTarget`) has a compliance suite.
- **Test cases:**
  - TC-03-01 (happy): reference impl of each interface passes its compliance suite.
  - TC-03-02 (boundary): interface methods receiving empty inputs return spec-defined empty results, not nulls.
  - TC-03-03 (negative): non-conforming impl (missing required method) is rejected by the loader with a clear `FORGE-XXXX` code.
  - TC-03-04 (negative): impl that misrepresents capabilities (declares `streaming: true` but doesn't stream) fails the contract test.
  - TC-03-05 (idempotency): contract-suite re-run on the same impl produces the same pass/fail set.
  - TC-03-06 (false-positive guard): a deliberately broken impl flips the suite to fail — proves the suite is actually exercising the interface.

## TEST-04 — LLM cassette/recording library + refresh policy

- **Anchor:** TEST plan §6
- **Acceptance:** PR-CI uses cassettes; nightly refreshes.
- **Test cases:**
  - TC-04-01 (happy): record-then-replay produces the same observable output, no live call made on replay.
  - TC-04-02 (boundary): cassette with zero interactions still loads and behaves as "miss → live call" (or strict-mode error, depending on flag).
  - TC-04-03 (negative): replay against a stale cassette (request signature changed) fails loudly with diff between recorded and current request.
  - TC-04-04 (idempotency): replay twice → identical outputs both times; no cassette mutation.
  - TC-04-05 (data-accuracy): recorded token counts and latencies are surfaced unchanged; cost ledger asserts equality on replay.
  - TC-04-06 (false-positive guard): a request not present in the cassette + strict-mode raises, proving the harness is not silently passing.

## TEST-05 — NFR benchmark suite (cold-start, RSS, scan throughput)

- **Anchor:** Arch §14
- **Acceptance:** Baseline numbers committed.
- **Test cases:**
  - TC-05-01 (happy): each benchmark runs to completion and emits a structured result.
  - TC-05-02 (boundary): benchmark with `--iterations=1` still produces valid stats (no division-by-zero).
  - TC-05-03 (regression): cold-start regression beyond +5% from baseline fails the suite.
  - TC-05-04 (data-accuracy): two consecutive runs of the same bench are within ±10% (proves measurement is stable).
  - TC-05-05 (false-positive guard): a deliberately slow patch causes the regression gate to trip, then reverting unblocks it.

## TEST-06 — Eval scenario: `new-app`

- **Anchor:** TEST plan §5
- **Acceptance:** Nightly green.
- **Test cases:**
  - TC-06-01 (happy): `forge new sample-saas` produces a running app in ≤60 s.
  - TC-06-02 (data-accuracy): the scaffolded app's first `forge ship` reference change succeeds end-to-end.
  - TC-06-03 (boundary): scaffold with the minimum supported template variant still passes.
  - TC-06-04 (regression): a known-bad commit (kept on a branch) fails this eval — proves the scenario is meaningful.
  - TC-06-05 (false-positive guard): a passing eval on a deliberately-broken scaffold (e.g. missing dep) is impossible by construction (the eval imports the run script, not a snapshot).

## TEST-07 — Eval scenario: `ship-reference`

- **Anchor:** TEST plan §5
- **Acceptance:** Nightly green; ≤5 min, ≤$0.20 tokens.
- **Test cases:**
  - TC-07-01 (happy): reference change ships within the budget on the canonical reference app.
  - TC-07-02 (boundary): change adjacent to the budget (e.g. wall-time exactly at budget) is accepted; +1% over fails.
  - TC-07-03 (data-accuracy): per-checkpoint timing breakdown sums to total ±2%.
  - TC-07-04 (idempotency): second run on the same change reuses cache; cost drops, behavior identical.
  - TC-07-05 (regression): a synthetic prompt-template regression that doubles tokens flips the gate.

## TEST-08 — Eval scenario: `scan-seeded-vulns`

- **Anchor:** TEST plan §5
- **Acceptance:** All 8 families catch their seeds.
- **Test cases:**
  - TC-08-01 (happy): each scanner family reports ≥1 finding with confidence ≥0.9 against its seed app.
  - TC-08-02 (negative / false-positive guard): clean reference app produces zero findings across all 8 families.
  - TC-08-03 (boundary): finding exactly at confidence threshold is reported per spec (`>=` not `>`).
  - TC-08-04 (regression): mutating a seed file *away* from the vuln pattern silently passes — proves rules are pattern-aware, not snapshot-aware.

## TEST-09 — Eval scenario: `plugin-load`

- **Anchor:** TEST plan §5
- **Acceptance:** 10 plugins ≤500 ms total.
- **Test cases:**
  - TC-09-01 (happy): loading the canonical 10-plugin set + invoking one verb each completes ≤500 ms p95.
  - TC-09-02 (boundary): zero-plugin baseline measured separately (proves overhead = total − baseline ≤ budget).
  - TC-09-03 (negative): loading 11th plugin still works; budget breach is reported, not crashed.
  - TC-09-04 (concurrency): parallel load of all 10 plugins respects sandbox isolation (no cross-plugin state leak).

## TEST-10 — Eval scenario: `migration-roundtrip`

- **Anchor:** TEST plan §5
- **Acceptance:** Apply + rollback + re-apply produces identical schema.
- **Test cases:**
  - TC-10-01 (happy): forward → reverse → forward yields byte-identical schema dump.
  - TC-10-02 (boundary): empty migration is a no-op end-to-end.
  - TC-10-03 (negative): malformed SQL fails forward; reverse is not attempted; ledger records the failure.
  - TC-10-04 (idempotency): forward applied twice (without reverse) is rejected as already-applied (no double-apply).
  - TC-10-05 (data-accuracy): row counts in seeded tables are preserved across the roundtrip.

## TEST-11 — Eval scenario: `cold-start`

- **Anchor:** TEST plan §5
- **Acceptance:** ≤80 ms.
- **Test cases:**
  - TC-11-01 (happy): `forge --version` cold-start p95 ≤80 ms on the reference machine class.
  - TC-11-02 (boundary): first-ever invocation after install (no cache) is measured separately and reported (may exceed 80 ms; not the gate).
  - TC-11-03 (regression): +5% over baseline blocks merge.

## TEST-12 — Eval scenario: `secret-redaction` (100-run zero-leak)

- **Anchor:** TEST plan §5; Arch §15
- **Acceptance:** Zero leakage in 100 runs.
- **Test cases:**
  - TC-12-01 (happy): seeded secrets in source files never appear verbatim in any LLM payload across 100 runs.
  - TC-12-02 (boundary): secret at exactly the minimum-length threshold of the redactor is still redacted.
  - TC-12-03 (false-positive guard): a *non-secret* string resembling a key shape is NOT redacted.
  - TC-12-04 (data-accuracy): every redacted occurrence is replaced by a stable placeholder of equal length (proves no length-leak).
  - TC-12-05 (regression): bypass attempt (e.g. base64-encoded secret) is caught by the entropy fallback.

## TEST-13 — Eval scenario: `repo-hygiene` (50 ship cycles, zero unmanifested files)

- **Anchor:** TEST plan §5; spec §4 hygiene
- **Acceptance:** Within budget.
- **Test cases:**
  - TC-13-01 (happy): after 50 simulated `forge ship` cycles on the reference app, `git status --porcelain` is empty and `forge clean --check` is green.
  - TC-13-02 (boundary): one cycle that legitimately produces a new generated artefact passes only when the manifest is updated in the same cycle.
  - TC-13-03 (negative): seeded scratch file appearing mid-run causes `forge clean --check` to fail at the next checkpoint, not at the end.
  - TC-13-04 (idempotency): re-running the eval on the resulting tree is a no-op.

## TEST-14 — Fuzz suite for plugin sandbox

- **Anchor:** Arch §15
- **Acceptance:** Nightly; reports zero escapes.
- **Test cases:**
  - TC-14-01 (happy): canonical fuzz corpus runs to completion within wall-time budget.
  - TC-14-02 (negative): seeded malicious plugin (attempts FS write outside grant) is blocked + reported.
  - TC-14-03 (negative): seeded plugin attempting outbound network without `network:*` capability is blocked.
  - TC-14-04 (concurrency): two malicious plugins loaded in parallel cannot collude across sandbox boundary.
  - TC-14-05 (regression): every historical sandbox-escape CVE has a corpus entry; suite proves none re-emerge.

## TEST-15 — Property-based tests for config layering

- **Anchor:** Arch §11
- **Acceptance:** Layering invariant proven.
- **Test cases:**
  - TC-15-01 (happy): for any random (defaults, file, env, flags) tuple, precedence flags > env > file > defaults holds.
  - TC-15-02 (boundary): missing layer (e.g. no env, no file) still resolves to defaults.
  - TC-15-03 (negative): conflicting types between layers (string vs int) raise a typed `FORGE-XXXX` error, not a panic.
  - TC-15-04 (data-accuracy): `forge config explain --key K` reports the winning layer correctly for every tuple.

## TEST-16 — Cross-OS install matrix

- **Anchor:** Arch §12
- **Acceptance:** Matrix green pre-release across mac/linux/win × x64/arm64.
- **Test cases:**
  - TC-16-01 (happy): brew/scoop/winget/curl install + `forge --version` succeeds on each (OS, arch) cell.
  - TC-16-02 (boundary): air-gapped install path (mirror-only, no internet) succeeds on linux/x64.
  - TC-16-03 (negative): unsupported (OS, arch) cell exits with a clear "unsupported platform" message, not a stack trace.
  - TC-16-04 (idempotency): re-install over an existing install preserves user config.

## TEST-17 — Bug-regression checklist enforcement

- **Anchor:** `always-write-tests.md`
- **Acceptance:** PR template requires linked regression test.
- **Test cases:**
  - TC-17-01 (happy): a "fix" PR with a linked regression test that fails on parent SHA and passes on PR SHA is accepted.
  - TC-17-02 (negative): a "fix" PR without a linked regression test is blocked by the bot.
  - TC-17-03 (negative): a regression test that *also* passes on the parent SHA is rejected ("test does not actually catch the bug").
  - TC-17-04 (false-positive guard): a non-fix PR (e.g. docs-only) is not blocked by the regression-test rule.

## TEST-18 — False-positive review weekly cadence

- **Anchor:** DEV plan §4 risk row
- **Acceptance:** Weekly issue summarizes false positives + actions.
- **Test cases:**
  - TC-18-01 (happy): cron emits one digest issue per week with sections for scan / lint / hygiene / secrets.
  - TC-18-02 (boundary): a week with zero false positives still emits a digest (states "0 across all categories").
  - TC-18-03 (data-accuracy): each false-positive entry links to the rule + the suppressed PR, and the suppression has an expiry.

## TEST-19 — Quality dashboard live + auto-updating

- **Anchor:** TEST plan §7
- **Acceptance:** Public URL; updates daily.
- **Test cases:**
  - TC-19-01 (happy): dashboard fetch returns the current week's metrics for every row in TEST plan §7 table.
  - TC-19-02 (boundary): metrics with no data this week show `n/a`, not `0` (avoids misleading "perfect" state).
  - TC-19-03 (data-accuracy): published metric values match the underlying source query (snapshot test against the dashboard JSON).
  - TC-19-04 (regression): a deliberately-introduced flaky test surfaces in the "Flaky-test count (open >24h)" widget within 25 h.

## TEST-20 — Hygiene fixture corpus maintenance

- **Anchor:** Spec §4 hygiene
- **Acceptance:** Corpus grows; PR template prompts contributor.
- **Test cases:**
  - TC-20-01 (happy): every entry in the corpus is matched by a hygiene rule.
  - TC-20-02 (boundary): empty corpus directory is a hard-fail of the corpus-integrity test.
  - TC-20-03 (negative): adding a corpus entry without a matching rule fails CI ("orphan fixture").
  - TC-20-04 (regression): historic OPS-11 weekly additions are never deleted.

## TEST-21 — Secrets fixture corpus

- **Anchor:** Spec §4 Repo Hygiene Layer (`.gitleaks.toml` standards)
- **Acceptance:** Every Forge-aware rule has ≥1 positive + 1 negative fixture; gitleaks allowlist excludes the directory.
- **Test cases:**
  - TC-21-01 (happy): each Forge-aware rule (Supabase JWT, Stripe live, PayPal live, OpenAI/Anthropic/Google, Twilio/SendGrid, social, PEM, generic high-entropy) flags its positive fixture with confidence ≥0.9.
  - TC-21-02 (false-positive guard): each rule does NOT flag its negative fixture (e.g. test key, public anon key, `pk_test_*`).
  - TC-21-03 (boundary): fixture key at the minimum-length boundary of the rule is still detected.
  - TC-21-04 (regression): every historical false-positive that was waived in `.gitleaks.toml` has a negative fixture committed alongside the waiver.
  - TC-21-05 (data-accuracy): fixture filenames embed `FORGE_FAKE_` prefix; suite asserts no real-key shape escapes the fixture directory.

## TEST-22 — `.gitignore` mandatory-block contract test

- **Anchor:** Spec §4 Repo Hygiene Layer (`.gitignore` standards) + §16.5.4 #11
- **Acceptance:** Every required entry + every required negation present in rendered `.gitignore`.
- **Test cases:**
  - TC-22-01 (happy): rendered `.gitignore` from `forge new` contains every entry in the mandatory-block manifest.
  - TC-22-02 (negative): a template fragment that omits a mandatory entry causes `forge new` itself to fail.
  - TC-22-03 (boundary): negation lines (`!*.example`, `!*.template`) are present and not shadowed by any later broader pattern.
  - TC-22-04 (regression): a known-bad pattern (broader `.env*` without `.example` re-include) fails the contract test.
  - TC-22-05 (false-positive guard): a project-specific user entry below the managed marker does not trip the contract test.

## TEST-23 — Secret-file guard test (tracked-file detector)

- **Anchor:** Spec §16.5.4 #11
- **Acceptance:** Tracked secret files fail; tracked example files pass.
- **Test cases:**
  - TC-23-01 (negative): `git add .env.local && forge clean --check` exits non-zero with a path-specific message.
  - TC-23-02 (happy): `git add .env.local.example && forge clean --check` exits zero.
  - TC-23-03 (boundary): file matched by the guard list but already untracked (only present in working tree) is reported as a warning, not a failure.
  - TC-23-04 (negative): each guard-list entry (`*.pem`, `*.key`, `*.pfx`, service-account JSON shape) has its own positive test.
  - TC-23-05 (idempotency): re-running `forge clean --check` on a clean tree is a no-op.

## TEST-24 — Allowlist-expiry regression test

- **Anchor:** Spec §16.5.4 #12
- **Acceptance:** Frozen-clock test asserts expiry behavior.
- **Test cases:**
  - TC-24-01 (happy): allowlist entry with `# review-by:` in the future passes the gate.
  - TC-24-02 (negative): entry with `# review-by:` in the past fails the gate.
  - TC-24-03 (negative): entry missing `# review-by:` is rejected at parse time (does not reach scan time).
  - TC-24-04 (boundary): entry with `# review-by:` exactly equal to today is treated as expired (strict `<` semantics).
  - TC-24-05 (data-accuracy): each expired entry's failure message includes the entry's `description` and original commit SHA.

## TEST-25 — Secret-redaction privacy invariant

- **Anchor:** Spec §4 Repo Hygiene Layer (`.gitleaks.toml` standards)
- **Acceptance:** Raw match never appears in stdout / log / telemetry / LLM context.
- **Test cases:**
  - TC-25-01 (happy): on a finding, stdout/log payload contains only path, line, rule ID, and a redacted preview (first 4 + last 4 chars).
  - TC-25-02 (negative): a synthetic test that *tries* to log the raw match (via `--explain` or debug flag) still gets the redacted form.
  - TC-25-03 (data-accuracy): redacted preview length is equal to `min(8, raw_len)`; never reveals total length beyond that.
  - TC-25-04 (regression): every prior leakage CVE / incident has a guard test here.
  - TC-25-05 (false-positive guard): a non-secret string that happens to be flagged is also redacted in the preview (consistent behavior).

## TEST-26 — `forge upgrade gitignore`/`gitleaks` idempotency + user-section preservation

- **Anchor:** Spec §4 Repo Hygiene Layer
- **Acceptance:** Round-trip is a noop; user section byte-identical across version bumps.
- **Test cases:**
  - TC-26-01 (happy): upgrade twice → `git diff` is empty.
  - TC-26-02 (data-accuracy): user-owned section bytes are identical before/after upgrade across two version bumps.
  - TC-26-03 (negative): upgrade run on a file that has been hand-edited inside the managed block reports the drift and refuses to clobber without `--force`.
  - TC-26-04 (boundary): upgrade on a brand-new project (no prior managed block) writes the markers + content correctly.
  - TC-26-05 (regression): the v0.10.7 → v0.10.8 upgrade is exercised end-to-end as a fixture.

## TEST-27 — Chaos-drill harness regression suite

- **Anchor:** Arch §17.3 + DEV-M2-23 + ARCH-DEC-15
- **Acceptance:** All 8 cross-cutting scenarios from §17.3 each have at least one passing drill in CI; harness self-tests pass.
- **Test cases:**
  - TC-27-01 (happy): each of the 8 scenarios runs and reports `outcome=pass` on a clean tree.
  - TC-27-02 (boundary): drill on an empty repo no-ops without panic.
  - TC-27-03 (negative): drill attempted on a protected branch is refused with `FORGE-XXXX`.
  - TC-27-04 (idempotency): re-running with the same seed yields a byte-identical containment trace.
  - TC-27-05 (concurrency): two scenarios injected in parallel produce non-entangled reports.
  - TC-27-06 (cross-tenant): a non-maintainer cannot launch the chaos harness against shared CI infra.
  - TC-27-07 (regression): the 8 catalogued scenarios are pinned by ID; removing one fails the suite.
  - TC-27-08 (data-accuracy): drill report's cited `FORGE-XXXX` matches the system's actual emitted code.
  - TC-27-09 (false-positive guard): a no-fault control run produces `outcome=pass` for all 8 — never `clean`-by-mistake.

## TEST-28 — Post-mortem CI gate enforcement

- **Anchor:** Arch §18.6 + DEV-M2-24 + ARCH-DEC-20 + OPS-18
- **Acceptance:** PRs touching `docs/postmortems/` are validated against the template; closed S0/S1 issues without a post-mortem fail the OPS-18 nightly check.
- **Test cases:**
  - TC-28-01 (happy): a complete post-mortem with 1 issue link + 1 §17.2 register update passes.
  - TC-28-02 (boundary): exactly 1 qualifying action item passes (off-by-one).
  - TC-28-03 (negative): post-mortem with only "be more careful" action items fails with explainer.
  - TC-28-04 (negative): missing one of the 8 mandatory sections fails template-shape lint.
  - TC-28-05 (idempotency): re-running the gate on an unchanged PM yields identical exit code + output.
  - TC-28-06 (regression): a closed S0 issue from > SLA ago without a PM file fires the OPS-18 alert in test mode.
  - TC-28-07 (false-positive guard): a non-incident PR (e.g. typo fix) does not trigger the PM gate.

## TEST-29 — Bug-intake SLA + reporter-feedback test

- **Anchor:** Arch §18.2 + §18.7 + DEV-M1-41 + ARCH-DEC-17
- **Acceptance:** Auto-triage labels a new bug within 60s; severity-based first-response SLA is measured; reporter is credited on close (`Reported-by:` trailer).
- **Test cases:**
  - TC-29-01 (happy): a synthetic bug issue is labelled within 60s; first human response within S2 SLA.
  - TC-29-02 (boundary): an issue at exactly the SLA boundary is counted as on-time (off-by-one).
  - TC-29-03 (negative): an issue past SLA without first response surfaces in OPS-19 dashboard as "breach".
  - TC-29-04 (idempotency): re-running the triage bot on a labelled issue does not duplicate labels.
  - TC-29-05 (cross-tenant): community vs core-team reporters get the same SLA labels.
  - TC-29-06 (data-accuracy): close commit's `Reported-by:` trailer matches the original reporter login.
  - TC-29-07 (regression): a synthetic "stealth fix" PR (no `Fixes:` trailer) is blocked, tying back to DEV-M1-41.
  - TC-29-08 (false-positive guard): a `question` or `discussion` issue is NOT held to the bug SLA.

## TEST-30 — Failure-register sync linter

- **Anchor:** Arch §17.2 + DEV-M1-40 + ARCH-DEC-16
- **Acceptance:** `.forge/failure-register.yml` and the §17.2 doc table are kept in lock-step by `forge audit failure-register verify`; PRs touching either must update both.
- **Test cases:**
  - TC-30-01 (happy): clean repo passes verifier with exit 0.
  - TC-30-02 (negative): doc-table row added without YAML entry fails with both file:line refs.
  - TC-30-03 (negative): YAML entry missing `detection`/`recovery`/`test_anchor` fails schema check.
  - TC-30-04 (data-accuracy): each YAML row's `test_anchor` resolves to a real test that exists in the repo.
  - TC-30-05 (regression): a synthesised PR that silently removes a row fails the verifier.
  - TC-30-06 (false-positive guard): a doc-only cosmetic edit (whitespace/formatting) does not fail the linter.

## TEST-31 — Resilience invariants lint coverage

- **Anchor:** Arch §17.4 + DEV-M1-39 + ARCH-DEC-14
- **Acceptance:** Each of the 7 §17.4 CI invariants has at least one positive fixture (passes) and one negative fixture (fails with file:line); lint runs in CI on every PR.
- **Test cases:**
  - TC-31-01 (happy): all 7 positive fixtures pass under the lint.
  - TC-31-02 (negative): all 7 negative fixtures fail with the expected file:line + invariant ID.
  - TC-31-03 (boundary): a fixture sitting exactly at a configured threshold (e.g. retry count == max) is classed correctly.
  - TC-31-04 (idempotency): re-running the lint without source changes yields identical output.
  - TC-31-05 (regression): reverting the wrapper around a known external call re-fires the original invariant.
  - TC-31-06 (cross-cutting): the lint runs across both core (`packages/`) and adapter (`adapters/`) trees.
  - TC-31-07 (false-positive guard): a properly wrapped, timed external call in a capability namespace does NOT fire any invariant.

---

*Task file version: 0.4 — companion to spec v0.10.9.*
