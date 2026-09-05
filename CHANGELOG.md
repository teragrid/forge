# Changelog

All notable changes to forge will be documented in this file. Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/); the project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.10.3] — 2026-09-05 — Agent mode: fabrication, wrong-feature resume, concurrency, and dry-run leaks fixed

### Fixed

- **`forge ship --agent-mode` could fabricate a passing Test checkpoint while silently discarding the host agent's real answer.** `writeTestArtifactsWithContext` wrote the static RED placeholder stub to `tests/*.test.ts` whenever an LLM/bridge call returned *any* error — including `ErrAgentTurn`, a pause rather than a failure. Once written, that placeholder permanently satisfied `allTestArtifactsExist`, so `checkTest` never called the generator again on a later run: a host agent's real, later-submitted answer for the same operation sat unused in the bridge's response store forever, while the checkpoint reported "N test file(s) found; all 4 named artifacts present" against content nobody had reviewed. Confirmed live against a real binary: submitting `expect(ping()).toBe('pong')` for `ship:test:unit` never reached `tests/*.test.ts` because the placeholder from the paused attempt had already tripped the exists-guard. `writeTestArtifactsWithContext` (and its `writeTestArtifacts`/`llmGoStub` callers) now return the agent-turn error before writing anything for the branch that paused, so the exists-guard stays false and the real answer lands on the next run.
- **A still-pending host-agent turn from an earlier run could go undetected, letting the pipeline fabricate results past it.** `agentbridge.Bridge.loadPending` restored a pending turn from disk without latching `paused`, deferring that to a `Lookup` call that would only happen if some checkpoint this run coincidentally re-asked the same operation. A checkpoint that never called `Lookup` at all this run (an "artefact already exists" skip, a language branch with no LLM call) left `paused` false for the whole process — which meant both the run loop's per-checkpoint gate and the final "render the pending turn" check in `forge ship` (both keyed on `Paused()`, not `Pending()` alone) silently failed to fire while an earlier turn (e.g. an `arch-parallel-debate` role) sat unanswered on disk the entire time. `loadPending` now latches `paused` immediately; replay is unaffected since `Lookup` checks its hash/ordinal indexes before it ever checks `paused`.
- **No description/`--name` with more than one existing spec directory silently ran the whole pipeline against an undefined slug.** `checkSpec` treated this as a soft "ok, pass a description" and let every later checkpoint fall back to its own "no description" branch independently, producing a full pipeline's worth of fabricated, spec-less output instead of one clear error. Now hard-fails, names every ambiguous candidate, and demands `--name`. A single unambiguous spec directory is unaffected.
- **The managed `.gitignore` block never listed forge's own agent-mode/scratch state**, so `.forge/agent/` (bridge session/pending/response files), `.forge/.snapshots/`, `.forge/learned/`, `.forge/trash/`, and `.forge/token-ledger.jsonl` all showed up as untracked in `git status`. Confirmed live: an agent-mode run left `git status --porcelain` reporting the bridge's own bookkeeping as changes, which the Code checkpoint's `countChangedFiles` then counted as evidence of real code changes — "N modified file(s)" for a run where the only real source file was untouched. Fixed in all three copies of the managed block (`codemod.canonicalGitignoreBlock`, `codemod.defaultMarkerBody`, `cmddoctor.canonicalGiSnippet`).
- **CI: the `M1-27` tests-precede-code gate flagged any test-only PR touching a mature file as a TDD violation.** It compared each file's all-time-latest-touch commit timestamp across the entire repo history rather than this PR's own commits — true for nearly every established file the moment its test gets a maintenance update with no accompanying production change. Both `git log` calls are now scoped to `origin/$BASE..HEAD` and take the oldest commit in that range, so an untouched production file correctly produces no commits (skipped) while a genuine same-PR "production code first, test added later" violation is still caught.
- **CI: the `M2-17` perf benchmark gate's `benchstat` install failed on `ci-gates.yml`'s Go 1.25 pin** once `golang.org/x/perf@latest` started requiring Go ≥ 1.26. `ci.yml` and `nightly.yml` were already on Go 1.26, matching `go.mod`'s `toolchain go1.26.6` (bumped 2026-08-20 for stdlib CVE fixes) — `ci-gates.yml` alone had drifted. Bumped to match.
- **`forge ship` hard-failed every LLM checkpoint when the configured provider was unusable (e.g. Anthropic credit balance too low) instead of falling back to `--agent-mode`.** `--agent-mode` was already the documented escape hatch for exactly this — "drive the pipeline from your own AI chat instead of an API key" — but it required the operator to notice the failure and manually re-invoke with the flag. `forge ship` now runs a cheap live probe (mirroring `forge doctor --llm`'s `checkLLMProviderLive`) before starting a non-agent-mode run: when a provider *is* configured but a minimal completion call against it fails for a permanent reason (an invalid/expired key, or a hard `invalid_request_error` — Anthropic's shape for "credit balance too low"), the run automatically switches to agent mode and prints a note explaining why, instead of proceeding to fail every checkpoint one at a time. No provider configured at all is left untouched — that already has its own stub/hint UX and is not this failure. Set `FORGE_NO_AGENT_FALLBACK=1` to keep the old hard-failure behavior (e.g. for CI jobs that should error rather than pause on a host-agent turn).
- **CRITICAL: `forge ship --agent-mode` without `--name`/description could silently resume a different, unrelated feature's pending checkpoint.** `Bridge.SetFeature` unconditionally wrote its `feature`/`slug` arguments into the session on every call, including the bare continuation forge itself tells you to run after a submit (`next: forge ship --agent-mode`, no flags). That call passed `SetFeature("", "")`, which blanked the session's previously-recorded feature identity — so the next resolution of "which spec is this session driving" had nothing to resume against and could fall through to an unrelated feature's incomplete checkpoint instead, corrupting its pipeline state with the wrong artefact if the answer were submitted. `SetFeature("", "")` is now a no-op when the session already has an identity, and the bare-continuation path in `forge ship --agent-mode` resolves the missing `--name`/description from the session's own recorded `Feature()` before doing anything else, so a bare re-run of forge's own printed hint now reliably continues the same feature.
- **CRITICAL: `forge ship --agent-mode` could hang for minutes with zero output mid-architecture-debate, or panic with a "concurrent map" error.** `checkArch`'s parallel role debate (`runParallelArchDebate`) fires one goroutine per reviewer role — six by default — and every one calls `LLMPipe.Invoke`, which in agent mode resolves through `Bridge.Lookup`. `Bridge` was documented "not safe for concurrent use" but nothing enforced that: six goroutines mutating the same `seen`/`byHash`/`byOrdinal` maps and `pending`/`paused` fields with no synchronization is a data race the Go runtime can surface as an outright panic, or — plausibly what was actually observed — as a hang, if the race corrupts a map's internal structure rather than tripping the concurrent-access detector cleanly. `Bridge` now serializes every exported method that touches shared state behind a mutex; a new regression test (`TestLookup_ConcurrentCallsAreSafe`) reproduces the exact six-goroutines-one-operation shape and is checked under `go test -race` in the nightly workflow.
- **`--dry-run` made real LLM calls and wrote files to disk despite its own help text ("preview what would happen without making LLM calls or git operations").** `newLLMPipeInteractive`'s dry-run branch called `newLLMPipe(root)`, which returns a live, billable pipe whenever a provider is configured — its own doc comment already (incorrectly) claimed dry-run "falls back to nil silently" while the code did the opposite. Separately, `checkSpec` and `checkArch` had no `dryRun` parameter at all: a preview of a not-yet-generated feature unconditionally created `.forge/specs/<slug>/`, wrote `workspace-context.md`, and wrote a `spec.md`/`arch.md` stub (or the real LLM output, if a working key was configured) — exactly the stray-directory behavior observed running exploratory `--dry-run` probes. Dry-run now always gets a nil pipe (no live provider is ever dialled), and `checkSpec`/`checkArch` report what they *would* generate without touching disk when the target artefact doesn't already exist yet.

## [1.10.2] — 2026-08-21 — Agent mode stopped pausing: a bridge miss was treated as an LLM failure

### Fixed

- **`forge ship --agent-mode` silently stubbed spec/arch/test/breakdown/code instead of pausing for a real turn.** Every checkpoint that generates an artefact via `LLMPipe` funnelled `generateWithValidation`'s error straight into its generic "LLM failed → write a stub, log a failure" branch, without ever checking whether that error was actually `ErrAgentTurn` — a *pause*, not a failure. In agent mode a bridge miss on, say, the arch checkpoint was therefore treated exactly like a real provider error: the checkpoint overwrote whatever was on disk (or nothing, pre-emptively) with a stub template and recorded a false failure in the learned-failures file used as future prompt context. A second, independent instance of the same root cause lived in the per-checkpoint post-processing loop: because the pause was reported as `cp.Status == "ok"`, the completion-marker writer — which writes `<checkpoint>.md` whenever a checkpoint didn't fail — ran anyway and wrote placeholder "Status: warning / Evidence: none" content into the checkpoint's own primary artefact file (e.g. `arch.md`) before the host agent had answered anything. Once that file existed, the next `forge ship --agent-mode` invocation saw it as "already done" and moved straight to the next checkpoint, repeating the mistake — the net effect being a single run that could stamp broken stubs across several checkpoints in a row while only ever showing the user the first turn.

  Every checkpoint (`spec`, `arch`, `test`, `breakdown`, `code`) now checks `IsAgentTurn` before falling back to a stub, and a paused checkpoint is marked `AgentPaused` so the post-checkpoint hooks, evidence policy, digest, and completion-marker steps are all skipped for it rather than run against an artefact that doesn't exist yet.

- **Reusing the same agent-mode session across unrelated features could replay one feature's answers into another's.** The bridge's ordinal-fallback replay (used when a prompt's hash has drifted but its position in the run has not) is keyed only on `operation#N`, with no feature scoping. Driving a second feature through the same `default` session — the common case, since `--session` is opt-in — meant its Nth call to a given operation (e.g. `ship:qa-verify:generate`) could silently hit the *first* feature's recorded answer for that same position instead of asking a fresh question. `Bridge.SetFeature` now detects when a session already belongs to a different feature/slug and resets the session's recorded responses before adopting the new one; `forge ship --agent-mode` prints a note when this happens so a reused session isn't a silent trap.

## [1.10.1] — 2026-08-08 — Scanner false positives, and a QA scenario that had been red since 1.9.0

### Fixed

- **`forge scan security` reported hardening as exposure.** The `select-without-where-tenant` rule matched line-locally with `select\s+.+\s+from\s+\w+\s*;`, which flagged `REVOKE SELECT ON auth.users FROM authenticated;` — a statement that *removes* read access — as an unscoped tenant read. A rule that inverts its own meaning is worse than an absent rule: it teaches the reader to distrust the output, and real findings then hide in the noise.

  The same regex also flagged `SELECT count(*) INTO v FROM inserted;` where `inserted` is a CTE the same statement had just populated. Tenant scoping belongs on the writing arm of a `WITH inserted AS (INSERT … RETURNING …)`; the read cannot be scoped and is not a leak. Deciding that needs context beyond the current line, which a line-local scanner does not have.

  `RunRLS` now scans whole files in two passes — pass 1 collects CTE names (`WITH x AS (` and continuation `, x AS (`), pass 2 evaluates the rules with that context and skips lines whose leading keyword is `GRANT` or `REVOKE`. The exemption is deliberately narrow, and pinned that way: `TC-SCAN-RLS-07` asserts that a genuine unscoped read of a physical table *in a file that also contains a CTE* is still flagged, so the fix cannot quietly become an off switch for the rule.

  Found by dogfooding on a real repo where all 8 findings from `forge scan security` were false positives, 7 of them from this rule. That repo now reports `findings: 0, clean`, and a real unscoped `SELECT` still trips.

- **QA-24 and QA-26 had failed on a clean `main` since 1.9.0.** Both run the full ship pipeline against a scratch project from `forge init --minimal` — no `go.mod`, no `package.json`, no `testing-pipeline.md` — and both assert exit 0. When the four-stage testing gate became blocking by default (1.8.2, re-released as 1.9.0), QA-Verify started correctly failing such a project, so the pipeline correctly exited 1 and the expectations were simply stale.

  That left stage [13/13] of the pre-push hook permanently red — precisely the state in which a real failure goes unnoticed. What these two scenarios actually cover is that `--json` bypasses the *interactive* gate and emits the documented `checkpoints` / `dry_run` keys, not whether an empty directory can satisfy a testing audit; both now pass `--no-strict-testing`, the documented waiver for exactly this case, with a comment recording why it is required rather than incidental. `SHIP_QA_ONLY=1 FORGE_NO_LLM=1 bash scripts/forge-qa-real.sh` goes from 2 of 12 failing to 12 of 12 passing.

## [1.10.0] — 2026-08-07 — Quality gates that can fail, admit ignorance, and cite their evidence

### Added

- **M1 — a checkpoint may no longer report green on forge's own say-so.** `Checkpoint.Status` is a plain string that any of twenty-odd code paths could set to `"ok"`, and nothing ever required the code setting it to say *why*. Because forge is the actor in all of those paths, the thing being implicitly asserted was always true — "I wrote the file", "I ran the generator", "I completed the step". Those are facts about forge's behaviour, not about whether the change is sound. Every bug in this family has had the same shape:

  | forge verified | what mattered |
  |---|---|
  | `spec.md` written | `spec.md` is complete |
  | test file written | the test will ever run |
  | the gate returned | the gate examined anything |
  | the checkpoint ran | the checkpoint verified anything |

  Checkpoints now carry `Evidence`, tagged by source. `SourceExternalTool` (a scanner, test runner, linter or git was asked and answered) and `SourceReadBack` (forge re-read the artefact from disk and re-validated it, judging it as it would judge a stranger's) count as independent. `SourceForgeClaim` — forge asserting its own success — is recorded, reported, and never sufficient on its own.

  Enforcement is at the reporting boundary, not via a private field or a mandatory setter: routing every assignment through `Pass(evidence)` would just invite `SourceForgeClaim` boilerplate that satisfies the compiler and nothing else. A checkpoint reaching `"ok"` with no independent evidence is downgraded to `"warning"` and annotated `UNVERIFIED[…]`.

  **This cannot break a working pipeline.** The claim a downgrade makes is "nobody checked" — a reason to withhold confidence, not to block a release. `res.Ready` keys on `"fail"`, which this policy never produces (`TestEvidencePolicy_NeverBlocksARun`).

- **The gates turned out to be the evidence system already.** A hook returning `VerdictPass` has read an artefact off disk and re-validated it — that *is* read-back evidence, it was simply never recorded as the basis for the status. Wiring it up gave most checkpoints real evidence without touching a single `Status = "ok"` line. Only `VerdictUnknown` contributes nothing, which is exactly what M3 was for.

- **Checkpoint marker files now record the basis, not just the outcome.** `.forge/specs/<slug>/<checkpoint>.md` gains an `Evidence:` line. The marker is the durable record — what `forge ship status` reads and what someone opens months later to ask "was this actually checked?" — and recording a status without its basis left that question unanswerable. Evidence is also emitted in `forge ship --json` so CI can audit a green run instead of taking the word `ok` for it.

- **`PhasePreCheckpoint` hooks now actually run.** `self-review-gate` was declared, listed in `defaultHooks()`, documented in the package header, covered by tests — and had never executed, because `runWithOptions` only ever called `runHooks` for the two later phases. Counting it among forge's quality gates was inaccurate from the day it was written.

  It is wired in as **advisory**: findings annotate the checkpoint and downgrade `ok` to `warning`, but do not fail it. Every project using forge has been shipping without this gate, so switching it on as a blocker would break builds over artefacts that were acceptable yesterday. `HookConfig.Strict` is the opt-in for making it stop a run, exactly as with every other hook. All three pre-checkpoint calls are now routed through one `beforeCheckpoint()` helper alongside the snapshot and the agent-mode checkpoint marker, so a future checkpoint cannot pick up two of the three and silently miss the third.

- **`Verdict` — quality gates now have three outcomes, not two.** `HookResult.Passed bool` is replaced by `Verdict` (`VerdictUnknown` / `VerdictPass` / `VerdictFail`).

  A bool forced every gate that *could not check* — artefact missing, tool not installed, config it cannot parse — to answer either "pass" or "fail". Gate authors almost always picked pass, because failing a build over something that is not the user's fault is obviously wrong. So "I did not verify this" and "I verified this and it is fine" became the same value, and the caller could not tell them apart. Every instance of that in forge has been the same bug: a green checkpoint standing on a check that never ran.

  `VerdictUnknown` is deliberately the **zero value** — a handler that forgets to set a verdict yields "unverified", which is honest, rather than falling into a false pass. Pinned by `TestVerdict_UnknownIsTheZeroValue`, because if `VerdictPass` ever became iota's first value, every incomplete handler in the codebase would silently start reporting success.

  Unverified gates annotate the checkpoint `UNVERIFIED[…]` and never escalate it. They are suppressed on an already-failed checkpoint, which has a real error to show.

- **Every "could not check" path now says so.** Nine gates returned `Passed: true` when their artefact was missing (`// no spec file yet`, `// no ADR file → nothing to check`, …). All now return `VerdictUnknown` with a reason naming the missing file. Enforced going forward by `TestGateMutation_NoGateReportsCleanOnAnEmptyProject`: no gate may report clean on a project where none of its artefacts exist.

### Fixed

- **`spec-code-alignment-gate` reported PASS on projects it had never examined** — the gap the M2 mutation table found on its first run. `auditSlug()` returns early when `spec.md` is absent, skipping every check, and the gate fell through to pass. `forge ship --from=code` on a project whose spec was never written got a green alignment gate that verified nothing. It now returns `VerdictUnknown`: the gate did not find the project acceptable, it found it *unexaminable*, and those are different facts.

- **`self-review-gate` reported PASS after scanning zero files.** Same shape, found by the same test.

### Added

- **Gate mutation testing (`gate_mutation_test.go`) — tests for the quality gates themselves.** Every other test in the suite asks "does the pipeline behave correctly?"; these ask whether the gates *check anything at all*. Each of the 13 hooks in `defaultHooks()` is now run against a **known-bad** fixture it must reject, and a known-good one it must accept. A gate that cannot fail is not a gate — and that is not hypothetical: 1.8.1's reachability checker compiled `\\.` from config source meaning `\.`, matched nothing in any real path, and reported the dead zone it was written to catch as fine. It was green, it was wrong, and nothing in the suite would have noticed, because every existing test asked only whether *good* input passed.

  `TestGateMutation_EveryDefaultHookIsCovered` fails the build when a hook is added without a mutation entry. Without that guard the file decays silently: gates written later get no coverage while the suite still reports green — a safety net with a growing hole, which is worse than none because it is still trusted. Hooks that genuinely cannot fail (the post-pipeline reminder) must declare `alwaysPasses` with a written justification rather than being omitted, since omission and "deliberately cannot fail" are indistinguishable in a diff.

- **A real gap the mutation table found on its first run**, now pinned by `TestSpecCodeAlignment_SilentlyPassesWithoutSpecMD`: `auditSlug()` returns early when `spec.md` is absent, so `spec-code-alignment-gate` reports **pass** on a project with unfinished tasks — it never looked. Not reachable by accident in a normal pipeline, but very reachable on purpose: `forge ship --from=code` on a project whose spec was never written gets a green alignment gate that verified nothing. Left as-is rather than flipped to a failure, because failing every spec-less run is a behaviour change that belongs in its own release; recorded so it is a known gap with a name instead of an unexamined green.

### Fixed

- **`NO_COLOR=1` silently disabled every interactive approval gate in `forge ship`.** [NO_COLOR](https://no-color.org) is defined as a purely *presentational* signal — it asks software not to emit ANSI colour. People set it for accessibility, for terminals that render escape codes badly, or simply preference, and they set it globally in a shell profile once. Forge read it as "an LLM is driving me" and auto-approved all seven checkpoints without ever prompting, with nothing in the output saying so. A signal about how text is *displayed* was deciding whether a human reviews the change. `NO_COLOR` now only suppresses colour, which is all it ever asked for.

- **Gate suppression is no longer silent.** `FORGE_LLM_MODE=1` still auto-approves — it is forge's own explicit signal and does mean no human is present — but it now announces itself on stderr. Skipping review must never happen quietly even when it is the correct behaviour, or the next person to inherit that environment variable has no way to discover why nothing ever asks them.

### Changed

- **`BREAKING.md` now has a `Default-behaviour changes` tier.** The versioning table alone read as "any default change is MAJOR", a bar high enough that the realistic alternative became shipping as a patch and hoping — which is precisely what happened in 1.8.2. The new tier permits `MINOR` under four explicit conditions (documented opt-out; opt-out named *in the failure message*, not just the changelog; loud and specific failure; `Breaking Changes` section plus a `!` commit), and keeps capability removal at `MAJOR` regardless. The 1.8.2 → 1.9.0 correction is written up as the worked example, including what the correction could **not** fix. An unfollowed policy is worse than no policy, because it still gets cited as binding.

## [1.9.0] — 2026-08-05 — Re-release of 1.8.2 under a correct version signal

No code changes from 1.8.2. This release exists to fix the version number, which was wrong in a way that matters to consumers.

1.8.2 changed a default so that `four-stage-testing-gate` blocks by default. That fails pipelines which previously passed, and [BREAKING.md](BREAKING.md) classifies *"changing the default value of a config key in a way that silently alters behaviour"* as a breaking change. Shipping it as a **patch** told every `^1.8.0` / `~1.8.1` consumer it was a safe automatic upgrade — the opposite of what it was. A version number is a machine-readable promise, and that one was false.

Re-releasing as a **minor** restores the signal. Note what this does and does not fix:

- **Fixed going forward:** consumers pinned to `~1.8.1` (patch-only) no longer pick this up automatically.
- **Not fixed:** anyone on `^1.8.0` who already upgraded to 1.8.2. npm publishes are permanent and 1.8.2 cannot be recalled. If your pipeline went red at `qa-verify`, see the 1.8.2 notes below — the failure message names both opt-outs.

The behaviour, the opt-outs, and the reasoning are unchanged and documented under [1.8.2](#182--2026-08-05--the-4-stage-testing-gate-is-on-by-default) below. `1.8.2` is left in this changelog rather than renamed, because it was published and rewriting released history would be a second false signal on top of the first.

## [1.8.2] — 2026-08-05 — The 4-stage testing gate is on by default

> **Read this before upgrading.** This release changes a default in a way that will fail pipelines that previously passed. It ships as a patch version, so `^1.8.0` / `~1.8.1` consumers receive it automatically. If a green `forge ship` suddenly fails at qa-verify with `four-stage-testing-gate`, that is this change, and the failure message names both ways to opt out.

### Breaking Changes

- **`four-stage-testing-gate` is now blocking by default.** Previously it only ran when opted into with `forge ship --strict-testing` or `strict-testing: true` in `.forge/hooks.yaml`; missing `testing-pipeline.md` evidence was reported and then ignored. It now fails `qa-verify` unless the evidence is present.

  The old default meant the gate's own finding — *"there is no evidence this change was tested"* — was itself recorded as an acceptable outcome. Testing evidence is not a premium feature of forge, it is the product: a pipeline that ships a change while quietly noting nobody verified it is doing the exact thing forge exists to prevent.

  **To restore the previous behaviour**, pick either:
  - per run: `forge ship --no-strict-testing`
  - per project: add `strict-testing: false` to `.forge/hooks.yaml`

- **`--strict-testing` is now a no-op** in the common case, since the gate it enabled is on by default. It is deliberately **not** removed and does **not** error — scripts and CI jobs in the wild pass it, and breaking them would serve no purpose. It retains one real effect: forcing the gate on over a project file that set `strict-testing: false`.

- **`.forge/hooks.yaml` now reads `strict-testing: false`.** While the default was off, the file could only ever turn the gate *on*, so the `false` case was never parsed. With the default inverted, leaving it unread would have meant a project had no way to opt out at all — turning a default into a mandate. `strict: false` is now honoured for the same reason.

### Fixed

- **`forge ship --resume` silently discarded every flag on the command line.** `runResumeFlag` called `RunCheckpoints`, which takes no options struct, so a resumed run ignored the switches it was given: `--resume --no-strict-testing` re-enabled the very gate the user had just waived, and `--resume --agent-mode` would have dialled a paid provider despite the user opting out of one. Resume now routes through `RunWithOptions` and honours the same flags as a fresh run — a resumed run is the same pipeline, and must not be a way around a gate.

## [1.8.1] — 2026-08-05 — The Test checkpoint no longer calls tests green that no runner will ever execute

### Fixed

- **`forge ship test` wrote test files into a path no runner collects, counted them, and reported the checkpoint green.** Writing a test and running a test are different things, and on a real project they came apart. Given the common split-config Jest setup:

  ```js
  // jest.config.js
  testPathIgnorePatterns: ['/node_modules/', '\\.(integration|e2e)\\.']
  // jest.integration.config.js
  testMatch: ['<rootDir>/tests/**/integration/*.test.ts']
  ```

  forge writes `tests/<slug>.integration.test.ts`. The default config **ignores** it for having `.integration.` in the name; the integration config **does not match** it for not living under an `integration/` directory. The file exists, the checkpoint is green, and the test has never run once — not in CI, not locally, not ever. It does not even appear as uncovered in a coverage report, because a test no runner collects is not a gap in the report; it is absent from the report's universe. That is worse than having no test at all: a missing test is visibly missing, while a test that silently never runs is indistinguishable from a passing one. Confirmed live on a project where an entire generated integration suite had zero executions and nobody noticed.

- **New reachability check (`internal/cli/cmdship/test_reachability.go`)**, run at the Test checkpoint against every JS/TS test file on disk, in three tiers of descending trust — because the answer is only worth having if forge is honest about where it came from:
  1. **Ask the runner.** `jest --listTests` / `vitest list` enumerate exactly what will be collected. This is the resolver that actually runs in CI, not a reimplementation of it. Only a runner already installed in `node_modules/.bin` is used — forge never resolves a package from the network to answer a read-only question about the working tree, and never installs one as a side effect.
  2. **Read the configs.** With no runner installed, `testPathIgnorePatterns` / `testMatch` / `testRegex` are parsed from every `jest.*.config.*`, `vitest.*.config.*`, and any inline `"jest"` block in `package.json`. The union of all configs decides reachability, since a split unit/integration setup is the normal case rather than an edge case.
  3. **Say so.** When neither works, the checkpoint reports the artefacts as *unverified* instead of implying they are wired up. `ReachabilityReport.OK()` returns false for an undetermined report, so "unknown" can never be rendered as "fine" at a call site.

  Unreachable files downgrade an `ok` Test checkpoint to `warning` with the offending paths named. It is deliberately **advisory, never blocking**: making it fail the pipeline would strand every project whose config forge cannot parse, and 1.7.12 already set the precedent that a new testing gate lands advisory first. Go and Python projects are skipped entirely — `go test ./...` and pytest collect by convention rather than configuration, so there is no equivalent dead zone to fall into and a check would only produce noise.

- **A false green inside the new check itself, caught by its own tests before release.** These config fields hold *regexes*, so at the source level they are double-escaped: `'\\.(integration|e2e)\\.'` is the JS spelling of `\.(integration|e2e)\.`. Compiling the raw source text yields `\\.` — "a literal backslash, then any character" — which matches nothing in a real path. Every ignore rule would have looked inert, nothing would have appeared excluded, and the dead zone would have been reported as reachable: a false green produced by the very check written to prevent false greens. String literals are now unescaped before compilation, with single-backslash regex escapes (`\d`, `\.`) left intact.

## [1.8.0] — 2026-08-04 — Two planes: run the whole ship pipeline from your own AI chat, with no API key

### Added

- **`forge ship --agent-mode` — the deterministic half of forge no longer depends on buying a second LLM subscription.** Until now `forge ship` needed `ANTHROPIC_API_KEY` / `OPENAI_API_KEY` / a Copilot token to do anything at all, which meant a user already paying for a model inside a chat window (Claude Code, Copilot Chat, Cursor) had to buy a redundant one to use forge. But only part of forge ever needed a model. Forge is really two planes fused into one binary:
  - the **deterministic plane** — the checkpoint state machine, quality gates, `validateArtefact`, spec manifest, traceability, scan/lint, and the audit + token ledgers. All of it runs locally in Go, needs no network, and is the only thing in forge that can truthfully say whether a checkpoint passed;
  - the **reasoning plane** — writing a spec, debating an architecture, decomposing a breakdown. Only this half needs a model.

  Agent mode points the second half at the AI chat the user is already in, and leaves the first half exactly where it was. `forge ship "<feature>" --agent-mode` runs every deterministic step it can, and when it reaches a step that needs reasoning it prints the compiled prompt as a `FORGE AGENT TURN` block and exits **78** (`cmdship.ExitAgentTurn`). The host agent answers, runs `forge agent submit --file <path>`, and re-runs `forge ship --agent-mode`; forge replays the answer at the point it was asked for, runs the same gates against it, and either advances or hands over the next turn.

- **The host agent supplies text, never verdicts.** This is what separates agent mode from a prose "skill" that merely describes the forge workflow — an imitated quality gate is one the model grades itself on. A submitted answer re-enters the pipeline through the same path a provider response takes, so it gets the same preamble stripping, the same truncation detection, and the same checkpoint hooks. An artefact that would have been rejected coming from Anthropic is rejected coming from your chat window.

- **New `internal/agentbridge` package** — the plane boundary itself, wired in at forge's single LLM choke point (`LLMPipe.InvokeChecked`), so every checkpoint that can call a provider is covered by construction rather than one at a time. Prompts are scrubbed by `secretrewriter` *before* the bridge sees them: a deferred turn is written to disk and read back by a human-facing chat window, which is at least as exposed as an API request.

- **New `forge agent` verb** — the host agent's side of the loop: `status`, `prompt`, `submit --file <p>` / `submit -`, `sessions`, `reset --yes`, and `loop` (prints the driver protocol for pasting into a chat). `forge agent prompt` is deliberately idempotent and stateless — a chat whose context was compacted mid-run can recover the full question, budget and submit command without remembering anything from the previous turn.

- **`--session <name>`** on both `forge ship --agent-mode` and `forge agent`, so two conversations driving two features concurrently can never answer each other's turns.

- **Replay is content-addressed, with a positional fallback.** Answers are keyed primarily by a sha256 over the operation name plus both prompt halves, so an unchanged question always replays. A pure content address is not sufficient on its own, and the reason is specific: prompts embed repository state, and the host agent writes files *between* turns, so the prompt for turn N legitimately changes once turn N's own artefact lands on disk — keying on the hash alone would re-ask that question forever. The store therefore also keeps an ordinal key (the Nth call to a given operation) and falls back to it, counting the event as drift so it is visible in `forge agent status` rather than silent. `--strict-replay` turns the fallback off for callers that would rather re-ask than replay a stale answer.

- **`forge skill install` now embeds the agent-mode driver loop** at the top of every generated persona file (Copilot instructions, `CLAUDE.md`, `.cursor/rules`, `.windsurfrules`), pointing the assistant at the enforced path first and keeping the existing prose methodology below it as the fallback for when the binary is not installed — with an explicit instruction to *say* which of the two it used, because a gate the model evaluated itself is weaker evidence than one forge evaluated. The protocol body is embedded verbatim from `cmdagent.DriverProtocol()` rather than restated, and a test fails the build if a future edit hand-writes it instead — two copies of a protocol drift, and the copy on disk is the one that outlives the binary that enforces it.

### Fixed

- **An unanswered agent turn could be silently replaced by a different question, misfiling the answer.** Re-running `forge ship --agent-mode` without answering is normal — it is how a driver that lost its place recovers — but the pipeline can legitimately arrive at a *different* prompt the second time, because a checkpoint that failed to generate its artefact may still have scaffolded a stub, moving the next run from the generate path onto the review path. Since `forge agent submit` records against whatever is pending at submit time, that would file a freshly generated spec as if it were a review. A pending turn now stands until it is answered or the session is reset, while already-answered turns still replay normally so the run can reach it.

### Changed

- **Agent mode ignores a detected API key rather than quietly using it.** `RunOptions.AgentBridge` takes priority over both `RunOptions.LLMPipe` and provider auto-detection. A user who asked not to be billed must not be billed because forge found a key lying around in the environment.
- **Agent mode runs the arch/test DAG serially.** Those two checkpoints normally run in parallel, but the bridge is driven from one goroutine and there is nothing to compute past a pause anyway — continuing would only queue up work to be redone on the next replay.
- **`forge ship`'s no-provider tip now names agent mode** instead of only telling users to go buy a key.
- **`main()` maps errors to exit status through the new `cli.ExitCode`.** Everything is still exit 1 except a paused agent-mode run: a driver loop that cannot tell "your turn" from "the change is broken" will either abort a healthy run or retry a broken one.

## [1.7.14] — 2026-08-02 — Copilot stops silently drifting to a dead default model; `forge llm` for one-command provider switching

### Fixed

- **`CopilotProvider`'s hardcoded default model (`claude-sonnet-4-5-20250514`) had gone dead** on GitHub's Copilot backend. GitHub's Copilot API does not error on an unknown model id — it silently substitutes a different one (observed live: a request for the dead snapshot came back HTTP 200 from `gpt-4.1` instead), so the existing HTTP-400 model-unavailable fallback never even triggered. The practical symptom: `gpt-4.1`'s non-streaming output through this proxy got cut short (`finish_reason: "length"`) well under the requested token budget on real prompts, and 1.7.13's `generateWithValidation` correctly flagged the result as truncated — the checkpoint failure was real, but the actual defect was one layer upstream of the validator, in what model got requested in the first place.
- `resolveModel()` now resolves the default model dynamically from the live `GET /models` catalog on first use — preferring an enabled, picker-enabled Anthropic "sonnet"-family model, tie-broken by highest version number (the API's response order is not sorted by version, e.g. `claude-sonnet-4.6` can appear before `claude-sonnet-5`) — instead of trusting a hardcoded constant that inevitably drifts stale as GitHub rotates model ids. Falls back to a refreshed static list only when the `/models` endpoint itself is unreachable (true offline/air-gap case). Explicit overrides (`FORGE_COPILOT_MODEL` env var, or `forge.yml`'s `llm.model`) always take priority over this resolution and are never second-guessed.
- Added `"stream": false` explicitly to the Copilot chat-completion request body, matching the provider's own declared `Capabilities().Streaming = false` (`Complete()` only ever parses a single JSON response object, never SSE frames).

### Added

- **New `forge llm` command** — one place to discover and switch which LLM backend `forge ship`/`spec`/`arch`/etc. use, instead of three separate `forge config set llm.provider` + `forge config set llm.model` + `forge doctor --llm` steps and guessing model ids from memory:
  - `forge llm list` — every known provider (anthropic, openai, gemini, azure, bedrock, ollama, copilot), whether each is currently usable given present credentials, which one `forge ship` would actually pick right now, and — for Copilot specifically — the full live model catalog (vendor, enabled/picker-enabled state) with the resolved default marked.
  - `forge llm use <provider> [model]` — writes `forge.yml`'s `llm.provider` (and `llm.model`, or clears it when omitted so the provider resolves its own default) and immediately makes one real, minimal completion call to confirm the new configuration actually works — surfacing a bad/stale model id or missing credentials at switch time with a clear, actionable error, rather than discovering it on the next real `forge ship` run. For Copilot, an explicitly-given model is validated against the live catalog before anything is written, since (per the Fixed section above) the API itself won't reject an unknown id.

## [1.7.13] — 2026-07-29 — Every checkpoint's generated content is now verified, not just spec/arch

### Fixed

- **`test`/`breakdown` checkpoints, and the TS/Go test-stub generators, wrote truncated or preamble-leaked LLM output straight to disk with no check at all.** 1.7.10/1.7.11 added truncation/preamble detection (`generateWithValidation`) to `spec.md`/`arch.md` only; `generateTestStubs`, `generateBreakdown`, and the Jest/Go stub generators (`writeTestArtifactsWithContext`, `llmGoStub`) still called the provider directly and trusted whatever came back. All five now route through `generateWithValidation` and fail the checkpoint (logging to `.forge/learned/*-failures.jsonl`) instead of persisting a broken file.
- **Every one of those generation budgets was undersized enough that the retry inside `generateWithValidation` failed identically, every time.** Token-ledger evidence: the breakdown checkpoint landed at *exactly* its configured budget on every real run observed (3000 on 2026-07-13/14, 6000 on 2026-07-23, twice in a row including the retry) — i.e. it has never once finished under its own budget. Raised: spec generate/review 2000→8000, arch generate 3000→6000, test-stub generate 3000→5000, breakdown generate 3000(→6000)→12000, TS unit/integration stubs 2000→6000, RLS stubs 1500→4000, Go stubs 2000→6000.
- **A cut-off response was discarded and fully re-asked from scratch on retry, roughly doubling both input and output cost per truncation** — the expensive side of Claude pricing thrown away on the exact runs that were already failing. `AnthropicAdapter` now continues a `Truncated` response (1.7.11's authoritative stop-reason signal) via the standard assistant-prefill technique (replaying the truncated text as the assistant's prior turn) instead of re-asking, keeping 100% of the already-paid output.
- **RLS test stubs were generated with no test-framework instruction at all**, and the unit/integration prompts hardcoded "Jest" regardless of the target project — a real run on this (Jest) repo got Vitest-flavored `rls.test.ts` back. All three stub prompts now name the actually-detected test runner explicitly.

### Added

- **Generated `spec.md`/`arch.md` now flag their own hallucinated file-path references.** `findUnverifiedFileReferences` scans backtick-wrapped, path-shaped tokens (the convention every spec/arch prompt already uses for file paths) and checks each against the real filesystem; `appendUnverifiedPathsWarning` appends a visible `⚠ Unverified file references` callout listing any that don't exist, so a reviewer sees the flag inline instead of needing to separately grep the repo. Scoped to file paths only (not DB columns/tables or invented infra) — a cheap structural check, not a second LLM call.

### Fixed (dev tooling)

- **The pre-push hook's own QA gate (stage 12) ran the P8 `forge ship` scenarios (QA-22–33) twice, once for real.** Stage 13 already re-runs the identical cases with `FORGE_NO_LLM=1` to stub out the provider; stage 12 ran them first with no such guard, and `forge ship --dry-run` does **not** actually skip LLM calls when a provider is detected (`--dry-run` only skips the interactive credential prompt) — so on any machine where a provider is auto-detected, stage 12 fired a real, chained multi-checkpoint LLM run that was slow and intermittently flaky for reasons unrelated to the code being pushed. `scripts/forge-qa-real.sh` gained a `SKIP_SHIP_QA` flag to skip P8 entirely; the pre-push hook now sets it for stage 12, leaving P8 coverage solely to stage 13's stubbed run. Note: the underlying `--dry-run` behavior (documented as "no LLM calls" but not enforced as such) is a separate, real gap, tracked but not fixed here.

## [1.7.12] — 2026-07-25 — The 4-stage testing pipeline: advisory by default, `--strict-testing` to enforce

### Added

- **New `--strict-testing` flag and the 4-stage testing pipeline reminder.** `forge ship`'s checkpoints (including `qa-verify`) prove a feature was specced, coded, and passed its own LLM-generated test suite — none of that proves a human, or an agent driving a real browser, actually exercised the running app anywhere past the generated stubs. After every successful run, `forge ship` now prints a reminder (to stderr, so `--json` output stays uncorrupted) covering 4 stages: local (automated + manual run against the live app), pre-push/CI, staging (manual retest of the actual changed behavior after deploy), and production (read-only smoke check after promotion). This is advisory-only by default across every project — it never blocks the pipeline. Pass `--strict-testing` (or set `strict-testing: true` in `.forge/hooks.yaml` to require it project-wide) to turn it into a real gate: the `qa-verify` checkpoint then fails unless `.forge/specs/<slug>/testing-pipeline.md` documents all 4 stages. Deliberately independent of the pre-existing `strict: true` hook setting — enabling `--strict-testing` does not also turn unrelated hooks like `manual-test-plan-gate` into blocking gates. See `docs/verbs/ship.md`.

## [1.7.11] — 2026-07-17 — Single-checkpoint runs stop paying for the whole pipeline

### Fixed

- **J10: single-checkpoint subcommands (`forge ship spec|arch|test|breakdown|code|ship|qa-verify "<desc>"`) no longer silently execute the entire pipeline.** Every `check*` function used to run unconditionally in `runWithOptions` regardless of `opts.Names`; the requested-checkpoint filter was applied only to what got *reported*, after all the real LLM calls and file writes for every other checkpoint had already happened. Confirmed via dogfooding on the Copilot provider (2026-07-17): `forge ship spec "<desc>"` reported `[1/1] ✓ Spec` while actually generating `arch.md`, `breakdown.md`, `code-plan.md`, `test-stubs.md`, and `tasks.md` too — roughly 6x the LLM cost of what the command should have made, with no indication in the output. Each checkpoint now only runs when it's in `opts.Names` (or on a full-pipeline run).
- **J11: truncated LLM artefacts could pass the J9 completeness check and get written to disk as if they succeeded.** `looksComplete()`'s Markdown-list-item branch accepts any line starting with `- `/`* `/a numbered prefix as a complete, intentional document ending — including a line that was actually cut off mid-word by hitting `MaxTokens` (e.g. `- **Cost awareness**: ...1-2 runs for posit`), since a bullet point that's merely short (`- happy path`) is structurally indistinguishable from one that's truncated without a dictionary. `llmprovider.Response` now carries a `Truncated` field populated from the provider's own stop/finish reason (Anthropic `stop_reason == "max_tokens"`, Copilot/OpenAI-compatible `finish_reason == "length"`) and propagated through `tierrouter.RouteResult`. `generateWithValidation` treats this as authoritative and forces `complete = false` regardless of what the text-shape heuristic alone would have concluded.
- **`MockProvider.calls` (test helper) had an unsynchronized counter**, racy under any test exercising concurrent checkpoints (e.g. `runParallelArchDebate`'s 6-role parallel debate). Found while verifying the two fixes above under `-race`; CI's self-hosted runner doesn't run with `-race` so this was never caught. Now guarded by a mutex.

## [1.7.10] — 2026-07-15 — Checkpoints stop hallucinating and self-heal a dead default model

### Fixed

- **`forge ship arch` could invent infrastructure the target project doesn't use.** `checkArch`'s prompt only forwarded the raw `spec.md` text, never the workspace-context tech-stack summary `checkSpec` already collects — confirmed live on a real project (Next.js + Supabase, no cache layer, no APM vendor), where a generated `arch.md` invented Redis, DataDog, NextAuth.js, and S3/Glacier. `checkArch` now forwards that same context.
- **`forge ship test` hardcoded Go regardless of the target project's language.** `generateTestStubs` always prompted "You are a senior Go QA engineer... write idiomatic Go tests," producing Go/`net-http` stubs on a TypeScript/Jest project. It now uses the detected language/framework, matching the pattern `writeTestArtifactsWithContext`/`detectTestFramework` already used elsewhere in the same checkpoint. The expected-artifact-filename list (previously hardcoded to TypeScript/Supabase-RLS conventions like `.rls.test.ts`/`.scan.baseline.json`, confirmed nonsensical running against this Go repo itself) is now convention-aware too.
- **Checkpoint digests were computed from the wrong input.** The digest writer passed `cp.Detail` (the one-line checkpoint status message) into `makeDigestFromArtefact` instead of the real generated artefact, so `digests/*.digest.yaml` always showed a near-zero `token_estimate` and empty `decisions`/`constraints`/`risks_accepted` regardless of what actually happened — useless for auditing whether a checkpoint engaged the LLM meaningfully. Digests now read the real `spec.md`/`arch.md`/etc. content.
- **`arch`/`spec` checkpoint failures left no audit trail.** `appendFailure` was already wired for `test`/`breakdown`/`qa-verify` but not these two, so an LLM error on arch/spec was undiscoverable except by reading the generated markdown by hand. Both now log to `.forge/learned/*-failures.jsonl` on failure, matching the existing checkpoints.
- **LLM responses could be written to disk incomplete or with leaked preamble, with the checkpoint still reporting success.** Confirmed twice: a generated `spec.md` had a raw conversational sentence ("I'll review this feature specification and provide comprehensive improvements...") as its literal first line, and a separate `spec.md` was truncated mid-Gherkin-block with no error surfaced. Checkpoints now strip leaked preamble and detect truncated responses (unbalanced code fences / no terminal heading-or-punctuation) before writing, retrying once and falling through to the failure log rather than silently persisting a broken file.
- **Anthropic's error handling couldn't distinguish a dead model id from a rate limit.** `Complete()`'s error switch branched on bare HTTP status codes only (401/429/default), with no JSON body parsing — a 404 (deprecated model) produced the exact same generic error string as every other failure. Errors are now classified from Anthropic's `{"type":"error","error":{"type":...}}` envelope into typed, actionable errors (`not_found`/`rate_limit`/`invalid_request`/`auth`).
- **A dead hardcoded default Anthropic model id (`claude-sonnet-4-5-20250514`, added in 1.7.1) broke every checkpoint on any project without an explicit `model:` pin, with no recovery.** Root incident: a project's `forge.yml` set `llm.provider: anthropic` with no model override and inherited this now-deprecated id, 404ing on every call until manually diagnosed via `forge doctor --llm`. Anthropic now gets the same live model discovery Copilot already had (`loadModels`, cached 24h under `.forge/cache/models/anthropic.json`) plus automatic one-shot fallback to a known-good model on a `not_found_error` — unless the user explicitly pinned a model, in which case forge fails loudly naming the dead id instead of silently substituting.

### Added

- **Bonus regression fix found while adding test coverage for the above**: `checkSpec`'s `looksComplete` heuristic misclassified a document ending on a normal Markdown bullet (e.g. `- happy path`) as truncated. Fixed, with regression coverage.

## [1.7.9] — 2026-07-13 — `--name`/`-n` no longer fragments a spec across two directories

### Fixed

- **`forge ship "<description>" -n custom-slug` silently split one feature's artefacts across two `.forge/specs/` directories.** `checkSpec`/`checkArch` correctly resolve `--name` before doing anything, but `generateTestStubs`, `generateBreakdown`, and `generateCodePlan` (the functions that actually write `test-stubs.md`, `breakdown.md`, `tasks.md`, and `code-plan.md`) each independently re-derived `slugify(description)` with no `specName` parameter at all — so those four files landed in a second directory auto-slugified from the raw description, while `spec.md`/`arch.md` correctly used `--name`. `checkCode`'s own success message claimed the file was written under the `--name` directory even while `generateCodePlan` silently wrote it to the wrong one. The same bug existed in the qa-verify/ship gap-remediation path (`auditSpecVsCode`, `remediateIncompleteTasks`, `remediateAuthzGap`) and in `checkPR`/`buildPRBody`, so the final QA audit could silently check an empty, nonexistent directory whenever `--name` didn't match the description's natural slug. Found via 3 reproductions in a single real project session (2 LLM providers, 100% reproducible). Fixed by threading the caller-resolved slug into every one of these functions instead of letting them recompute it.
- **The Code/Ship checkpoint reported a nonsensical "N modified file(s)" count.** `countChangedFiles` never checked git status — it recursively walked the entire working tree counting every `.go`/`.ts`/`.js`/`.py`/`.sql` file that existed on disk, regardless of whether it was actually changed. On a real ~1700-source-file project this reported "1693 modified file(s)" when `git status --short` showed single digits, across two separate runs with different providers. Now uses `gitservice.Status()` (`git status --porcelain`), which respects `.gitignore` and counts only genuinely modified/staged/untracked paths.

## [1.7.8] — 2026-07-13 — Stale config no longer permanently defeats the Copilot model fallback

### Fixed

- **A stale `forge.yml` `llm.model` value could permanently break every `forge ship` LLM call, with the root cause hidden behind `...` truncation.** `profileProvider.Complete` fills `Request.Model` from `forge.yml`'s `llm.model` as a soft default whenever a caller (e.g. the tier router, which deliberately leaves `Model` empty for Copilot so the provider can pick its own default) left it unset. `CopilotProvider.Complete`'s automatic fallback to `copilotKnownModels` on an HTTP 400 "model unavailable" response existed specifically to recover from an unavailable/deprecated model, but was gated on `req.Model != ""` — intended to mean "the caller explicitly pinned this model, honour it" — which could never distinguish a genuine pin from a config-file default that merely happened to be broken, so it suppressed recovery every time. Found via a real incident: a project's `forge.yml` named a model GitHub Copilot's `/models` endpoint listed but its `/chat/completions` endpoint rejected, failing every checkpoint with the actual reason truncated away. Fixed by adding `Request.ModelPinned bool`, distinct from `Request.Model` — `profileProvider` never sets it when filling in a config default, so the fallback now correctly reaches configured-but-broken models while still honouring a genuinely pinned model (e.g. a future `--model` flag) with no silent substitution.
- **LLM error messages were truncated at 77 characters, frequently cutting off exactly before the useful part** (the API's own `"message"` field, which sits inside a JSON body after a longer error-type prefix). `llmErrNote` now extracts that field directly when present, and the fallback truncation limit was raised to 160 characters. The full, untruncated error is now also persisted to `.forge/learned/breakdown-failures.jsonl` (previously only the truncated summary was recorded), so a later diagnosis — by a human or an LLM driving forge — doesn't lose the root cause to the same truncation twice.

### Added

- **`forge doctor --llm`** detects the active LLM provider the same way `forge ship` does, then sends one minimal real completion call and reports success or failure with the full error text — collapsing a class of incident that previously took a custom throwaway program and ad-hoc tracing into a single command. Opt-in (makes a real, token-consuming network call) and non-required, so it never blocks `forge doctor`'s overall health verdict on its own.

## [1.7.7] — 2026-07-10 — `forge clean --apply` no longer re-nests its own trash directory

### Fixed

- **`forge clean --apply` re-nested `.forge/trash` deeper on every run instead of converging.** The scan never excluded `.forge/trash` (its own destination directory) from the walk. Manifest `[scratch]` patterns are matched gitignore-style — a root-level pattern like `_*` matches by basename at any depth — so a file just moved into `.forge/trash/<run1>/...` matched the same pattern again on the very next `--apply` and got moved into `.forge/trash/<run2>/.forge/trash/<run1>/...`, nesting one level deeper every run without ever reaching zero candidates. Found live on a real project: two consecutive `forge clean --apply` runs produced `.forge/trash/<run2>/.forge/trash/<run1>/supabase/.branches/_current_branch`; the workaround at the time was deleting `.forge/trash` directly rather than retrying `--apply`, since retrying only nested it further. Fixed by excluding `.forge/trash` (and everything inside it) from the scan in `Run`, `RunDryRun`, and `RunWithTrash`, the same way `.git` and `autoSkipDirs` already are. Added `TestRunWithTrash_DoesNotReNestOwnTrash`, which runs `RunWithTrash` twice in a row and asserts the second pass finds zero candidates and creates no new trash directory.

## [1.7.6] — 2026-07-05 — Fix npm test passed-count parsing in QA-Verify

### Fixed

- **QA-Verify's npm test detail always reported "0 case(s) passed"** even when the real Jest run passed thousands of tests. Root cause, found immediately after shipping 1.7.5's Node detection: Jest always prints a `Test Suites: N passed, N total` line *before* the `Tests: ...` line, so the original unanchored `(\d+)\s+passed` regex matched the suite count (e.g. 296) instead of the individual-test count (e.g. 4118) — or matched nothing at all once the parsing was naively anchored to `Tests:` immediately followed by digits, since Jest's `Tests:` line commonly has a `N todo,` or `N failed,` group before the passed count (`Tests: 6 todo, 4118 passed, 4124 total`). Fixed with `(?m)^Tests:.*?(\d+)\s+passed`, anchored to a line starting with `Tests:` specifically (not `Test Suites:`), taking the first `N passed` within that line.
- Added a regression test reproducing the exact multi-line Jest output shape (`Test Suites:` line followed by a `Tests:` line with a `todo` prefix) that exposed both bugs at once.

## [1.7.5] — 2026-07-05 — QA-Verify recognizes Node/TypeScript projects

### Added

- **`runQATestSuite` now has a native-fallback case for Node/TypeScript projects** — a `package.json` declaring a non-empty `"test"` script is run via `npm test --silent` and its Jest-style `Tests: N passed` output is parsed into the checkpoint detail, matching the existing Go (`go test ./...`) and Python (`pytest`) native fallbacks. Previously QA-Verify only recognized Go (`go.mod`, `cmd/mcp/main.go`) and Python (`pyproject.toml`/`pytest.ini`/`setup.cfg`, `mcp_server.py`) projects — every Node/TypeScript project (including forge's own `next-app`/`ts-service` scaffold templates) always fell through to "no MCP server or test runner found" regardless of how many real tests it had. Found while dogfooding on a downstream Next.js project with 300+ passing Jest suites that QA-Verify reported as having no test runner at all.
- A `package.json` present but with no `"test"` script (or an empty one) correctly falls through to the "no runner found" warning rather than treating a missing-script `npm test` failure (exit 1, no tests actually ran) as a real test failure.

## [1.7.4] — 2026-07-04 — Test checkpoint no longer clobbers existing tests

### Fixed

- **`forge ship test` (checkpoint 3) overwrote hand-written test content with RED placeholder stubs on every re-run** — `checkTest` called `writeTestArtifacts` unconditionally whenever a feature description was present, with no check for whether the 4 named artifacts (`<slug>.test.ts`, `.integration.test.ts`, `.rls.test.ts`, `.scan.baseline.json`) already held real, developer-written content. Re-running `forge ship test`, a full `forge ship`, or even `forge ship -d` (dry-run) after tests were completed would silently reset them to `expect(false).toBe(true)` stubs. `checkTest` now skips the write entirely once `allTestArtifactsExist` reports all 4 present — the checkpoint still reports coverage status, it just never touches files that are already there.
- **`--dry-run` was not side-effect-free for the Test checkpoint** — despite being documented as "preview what would happen without making LLM calls or git operations," `forge ship -d` still wrote the 4 named test artifacts to disk. `checkTest` now takes an explicit `dryRun` parameter (threaded from `RunPipeline`'s `opts.DryRun`) and never writes to disk when set.
- Added `TestCheckTest_DoesNotClobberExistingNamedArtifacts` and `TestCheckTest_DryRunNeverWritesArtifacts` regression tests in `internal/cli/cmdship/ship_test.go`.

## [1.7.2] — 2026-06-14 — Credential auto-detection & interactive key prompt

### Added

- **Anthropic Claude auto-detection from Claude Code CLI** — forge now reads `~/.claude/config.json` (the `primaryApiKey` field) so users who have Claude Code installed can run forge commands without setting `ANTHROPIC_API_KEY` separately. The same credential that powers their Claude Code session is reused automatically. Priority: `ANTHROPIC_API_KEY` env var → `~/.claude/config.json` → next provider.
- **`llmprovider.DetectOrPrompt()`** — new function that wraps `Detect()` and, when no provider is found, interactively prompts the user to paste an Anthropic API key. `forge ship` calls this in non-dry-run mode so first-time users are guided rather than silently dropped into dry-run.
- **`"claude"` and `"claude-code"` provider aliases** — `forge config set llm.provider claude-code` and `forge config set llm.provider claude` are now accepted as aliases for the Anthropic provider.

### Fixed

- **Test hermetics** — detection tests that expected `ErrNoProvider` or a non-Anthropic provider when `ANTHROPIC_API_KEY=""` were sensitive to the presence of `~/.claude/config.json` on the developer machine. All such tests now redirect `HOME`/`USERPROFILE` to an empty temp directory so the new auto-detection path does not interfere.

## [1.7.1] — 2026-06-14 — Multi-LLM provider fixes

### Fixed

- **TierRouter model selection** — Gemini, Azure OpenAI, Bedrock, Copilot, and Ollama providers were all sent Anthropic model IDs by the tier escalation ladder. Each provider family now receives the correct model names: Gemini gets Gemini IDs, Azure/OpenAI get OpenAI IDs, Bedrock gets Anthropic IDs, and Copilot/Ollama defer to their own configured defaults.
- **Gemini `Complete()` ignored `req.Model`** — the Gemini adapter always used the provider's initialisation-time default model even when the tier router or caller set `Request.Model`. It now respects `req.Model` and reports the actual model used in the response.
- **Stale Anthropic model list** — capabilities advertised only Claude 3.5/3-Opus. Now includes the full Claude 4.x family (`claude-opus-4-8-20250514`, `claude-sonnet-4-5-20250514`, `claude-haiku-4-5-20251001`) and Claude 3.7 (`claude-3-7-sonnet-20250219`).
- **Stale OpenAI model list** — capabilities only listed `gpt-4o`, `gpt-4o-mini`, `gpt-4-turbo`. Now includes `gpt-4.1`, `gpt-4.1-mini`, `o4-mini`, `o3`, `o1`.
- **Stale Gemini model list and default** — default was `gemini-1.5-flash`; updated to `gemini-2.0-flash`. Capabilities now list the full Gemini 2.5 and 2.0 families.
- **Copilot fallback models missing version suffixes** — `claude-3-7-sonnet` and `claude-3-5-sonnet` lacked date suffixes, causing ambiguous model resolution. All fallback entries now use fully-versioned IDs. Added `claude-opus-4-8-20250514`, `gpt-4.1`, and `o4-mini` to the fallback list.
- **TierRouter `DefaultTiers` using outdated models** — T0/T1/T2 ladders updated to current model generations across all three provider families (Anthropic, OpenAI, Gemini).

## [1.7.0] — 2026-06-08 — LLM-first rearchitecture

### Added

- **`internal/llmresponse` package** — standard JSON envelope emitted by all forge commands when running in LLM mode. Envelope fields: `ok`, `checkpoint`, `status`, `context_summary`, `next_actions`, `llm_tokens_used`, `cost_usd`, `duration_ms`, `error` (with `code`, `message`, `remedy`). See `docs/mcp/tools.json` for schema.
- **LLM mode auto-detection** (`llmresponse.DetectMode`) — priority chain: `--human` opt-out > `--json` flag > `FORGE_LLM_MODE=1` > `NO_COLOR=1` > non-TTY stdout.
- **`FORGE_LLM_MODE=1` environment variable** — enables JSON envelopes and suppresses all interactive `y/N` gates in `forge ship` (AC-3). LLM agents set this once; no per-command flags needed.
- **`forge ship --human` flag** — explicit opt-out of LLM mode auto-detection; always produces human-readable output even when stdout is piped (AC-9).
- **`forge ship` gate suppression** — when in LLM mode, `y/N` approval gates are automatically skipped (equivalent to `--yes`), so Claude/GPT-4o/Copilot can drive the pipeline without blocking.
- **`context_summary` generator** (`llmresponse.GenerateSummary`) — deterministic ≤2000-char UTF-8 string giving an LLM complete situational awareness after one forge command. Includes verb/checkpoint, test results, spend, changed files, and error info (AC-7).
- **`next_actions` generator** (`llmresponse.NextActions`) — ordered list of concrete copy-pasteable commands for the next pipeline step.
- **`errcode.RegisterWithRemedy`** — new registration function that stores a copy-pasteable remedy alongside the description. `errcode.Remedy(c)` returns it. `*Error.ForgeCode()` and `*Error.ForgeRemedy()` implement the `llmresponse` interface for structured error envelopes.
- **`llmresponse.BudgetExceededError`** — `FORGE-2001` error with remedy `set FORGE_BUDGET_USD=<amount>` for budget cap hits (AC-8). `llmresponse.CheckBudget(root)` reads the token ledger.
- **10 MCP tools** (up from 4) — added `forge_ship_checkpoint`, `forge_get_errors`, `forge_set_budget`, `forge_list_specs`, `forge_get_spec`, `forge_check_health` to the MCP server (AC-4). Static schema at `docs/mcp/tools.json` (AC-5).
- **`forge doctor` LLM-mode advisory** — new `llm-mode` check surfaces whether `FORGE_LLM_MODE` is set and what it does (T-018).

### Breaking Changes

- **`forge ship` piped output format** — when stdout is not a TTY (or `--json` / `FORGE_LLM_MODE=1`), output is now a JSON envelope instead of ANSI text. Use `--human` to opt out. See `BREAKING.md` for migration guide.
- **`errcode.Register` signature unchanged** — backward-compatible; all existing `Register` calls continue to work. `RegisterWithRemedy` is additive.

## [1.5.0] — 2026-05-28

### Added

- **P1: Model tier routing** — `LLMPipe` now uses `tierrouter` for complexity-driven model selection. `nano`/`micro` → T0 (cheap), `standard` → T1 (balanced), `complex` → T2 (powerful). Controlled via `SetComplexityTier`.
- **P1: 3-layer knowledge base** (`internal/knowledge/layered.go`) — KB loader merges embedded global KB, user-scoped `~/.forge/kb/`, and project-scoped `.forge/kb/` in priority order. Project KB entries override global ones.
- **P2: OpenTelemetry checkpoint spans** — `telemetry.StartPipelineSpan` and `telemetry.EmitCheckpointSpan` emit per-checkpoint OTEL spans to `.forge/telemetry.jsonl`. Wired into the `forge ship` pipeline.
- **P2: Prometheus metrics export** — `forge metrics` command reads `.forge/token-ledger.jsonl` and outputs `forge_tokens_total` and `forge_cost_usd_total` counter series in Prometheus text format, labelled by model.
- **P2: forge undo integration** — `writeShipTrashManifest` records every ship run to `.forge/trash/<runID>/manifest.json`; `snapOnFail` takes a best-effort snapshot on checkpoint failure. Both wired into `forge ship`.
- **`forge companion`** — zero-setup AI pairing command with four subcommands: `install` (writes expert persona files for VS Code Copilot / Claude / Cursor / Windsurf), `update` (force-refreshes skill files), `status` (shows per-platform install state), `guide` (prints the vibe-coding quick-start cheatsheet with the top-10 daily prompts).
- **`forge init` companion hint** — after scaffolding completes, `forge init` prints a `forge companion install` hint so new projects are prompted to set up AI pairing immediately.
- **Command groups in `forge --help`** — 50 commands now displayed in 7 named groups: Core, Build & Ship, Analysis & Quality, Operations, AI & Automation, Config & Tools, Advanced.
- **Vibe-coding workflows in skill templates** — all platform templates (Copilot, Claude, Cursor, Windsurf) now include a "Daily Vibe-Coding Patterns" section with feature/bugfix/security/standup/review workflow examples.
- **Error code ranges** — `FORGE-6600..6649` reserved for `cli/metrics`; `FORGE-6650..6699` reserved for `cli/companion`.

### Fixed

- `companion.go` raw-string backtick syntax error — guide string switched to concatenation to allow embedded backtick characters.
- Duplicate `FORGE-6800` registration — `cmdmetrics` moved from 6800 to 6600; `cmdcompanion` moved from 6900 (unregistered) to 6650.
- `captureProvider` in `llmpipe_tier_test.go` — added missing `Capabilities()` method to satisfy `llmprovider.Provider` interface.
- `TestInvoke_FallbackToDirectWhenRouterNil` — test now uses `newLLMPipeWithProvider` (initialises `rewriter`) and then nils the router, avoiding the nil-pointer dereference.

## [1.3.0] — 2026-05-26

### Added

- **Ship pipeline hooks** (`hook.go`) — lifecycle hook framework for `forge ship`. Hooks fire at `pre-checkpoint`, `post-checkpoint`, and `post-pipeline` phases. Configured via `.forge/hooks.yml`; any hook can be disabled per-repo. Ships with `defaultHooks()`: tdd-gate, lint-gate, build-gate, and security-scan-gate.
- **`runWithOptions` hooks wiring** — `RunOptions.Hooks` (optional override) and `RunOptions.EnableLearning` fields added to `RunOptions`. When `Hooks` is nil, `defaultHooks()` is used automatically. Post-checkpoint and post-pipeline hooks are executed inside the ship pipeline.
- **Learning loop** (`EnableLearning`) — when `RunOptions.EnableLearning` is true, `extractAndLearnFromFeature` runs after a successful pipeline and writes learned patterns to two destinations: `.forge/learned/patterns-<slug>.jsonl` (JSONL append log) and `forge-knowledge/knowledge-base/patterns/workflow/learned/<slug>-<ts>.md` (KB markdown with YAML frontmatter for `forge kb search`).
- **Steering layer** (`steering.go`) — pipeline steering helpers that inspect checkpoint results and decide whether to continue, pause, or abort the run based on configurable policies.
- **Sub-workflow coordinator** (`subworkflow.go`) — orchestrates nested ship sub-workflows so a parent spec can delegate sections to child pipelines and merge their results back into the parent `ShipResult`.

### Changed

- **`checkQAVerify` refactored** — split into three phases: (1) spec-vs-code gap audit + LLM-driven remediation loop, (2) `runQATestSuite()` (Go MCP / Python MCP / `go test` / pytest auto-detection), (3) `generateManualTestPlan()` only when tests pass. Eliminates the previous monolithic implementation.
- **`extractAndLearnFromFeature` KB output** — learned-pattern markdown files now include a YAML frontmatter block with `id`, `category: patterns/workflow`, and a `forge_integration` map (checkpoint list, scan families, tags) for structured KB indexing.

## [1.2.0] — 2026-05-24

### Added

- **`forge mcp serve`** — JSON-RPC 2.0 stdio server that exposes Forge to any MCP-compatible AI chat tool (VS Code Copilot, Claude Desktop, Cursor, Windsurf). Four tools: `forge_kb_search`, `forge_get_workflow`, `forge_get_standards`, `forge_run`. The `forge_run` deny-list prevents unsafe verbs (`mcp`, `serve`, `remove`, `eject`, `clean`) from being invoked remotely.
- **`forge mcp info`** — prints ready-to-paste config snippets for VS Code (`settings.json`), Claude Desktop (`claude_desktop_config.json`), Cursor (`.cursor/mcp.json`), and Windsurf (`windsurf_mcp.json`).
- **`forge skill install --for <platform>`** — installs the Forge expert role into a project and wires it to the target AI tool. Platforms: `copilot`, `claude`, `cursor`, `windsurf`, `all`. Respects `--dry-run` to preview without writing files.
- **`forge skill list`** — lists all Forge-managed skill files in the current project.
- **`forge skill remove`** — removes all Forge-managed skill files. Accepts `--force` / `-f` (alias for `--yes`) to skip the confirmation prompt.
- **`forge explain mcp`** — plain-English description of the MCP server tools and usage.
- **`forge explain skill`** — plain-English description of the skill install command and `--for` platforms.
- **`.vscode/settings.json` MCP entry** — the repo now ships a `.vscode/settings.json` that wires `forge mcp serve` as the `forge` MCP server so any developer who opens the repo in VS Code gets Forge tools in Copilot Chat automatically.

### Fixed

- **`forge skill remove --force` flag missing** — `--force` / `-f` was accepted by `install` but silently rejected by `remove` with "unknown flag". Added as an alias for `--yes` on the remove sub-command.

## [1.1.11] — 2026-05-24

### Added

- **`forge config set llm.provider <name>`** — configuring a provider in `forge.yml` is now sufficient to enable all LLM-powered commands without setting any environment variable. Supported values: `copilot`, `ollama`, `openai`, `anthropic`, `gemini`, `azure`, `bedrock`.
- **`forge config set llm.model <model>`** — the configured model is automatically applied to every LLM request. Set once; applies to `forge ship`, `forge bugfix`, `forge scan`, and every other command.

### Changed

- **`llmprovider.Detect()` reads `forge.yml` first** — the forge.yml `llm.provider` setting now takes priority over the environment-variable detection order. If the configured provider's credentials are absent, detection falls back to env-var order automatically.
- **LLM error hints updated** — all "no provider" error hints now read `"run 'forge config set llm.provider <name>' or set ANTHROPIC_API_KEY / OPENAI_API_KEY"` instead of pointing only at Anthropic.

## [1.1.10] — 2026-05-24

### Added

- **Short flags on all key commands** — every frequently-used flag now has a single-character shorthand:
  - `forge ship`: `-d/--dry-run`, `-j/--json`, `-y/--yes`, `-Y/--yolo`, `-Q/--quick`, `-f/--from`, `-s/--skip-checkpoint`, `-p/--pr`, `-r/--root`, `-R/--resume`, `-B/--no-branch`
  - `forge ship spec`: `-n/--name` (renames `--spec` to `--name`; canonical usage: `forge ship spec "description" -n "name"`)
  - `forge scan`: `-r/--root`, `-j/--json`, `-m/--mode`, `-s/--since`, `-f/--fast`
  - `forge bugfix`: `-b/--bug`, `-f/--finding`, `-t/--test`, `-s/--stack`, `-c/--context`, `-m/--model`, `-a/--apply`, `-j/--json`, `-r/--root`
  - `forge init`: `-t/--template`, `-n/--name`, `-m/--minimal`, `-f/--force`, `-j/--json`
  - `forge new`: `-t/--tsd`, `-n/--name`, `-f/--force`, `-j/--json`, `-l/--list`
  - `forge audit`: `-r/--root`, `-j/--json`; `export`: `-o/--output`; `erase`: `-d/--dry-run`

### Changed

- **`forge ship spec --spec` renamed to `--name/-n`** — the flag that overrides the spec directory slug is now `--name` (with shorthand `-n`). The old `--spec` name is removed.

## [1.1.9] — 2026-05-25

### Added

- **`forge ship spec "description"` positional arg** — `--description` flag is no longer required; pass the feature description as the first positional argument (e.g. `forge ship spec "add rate limiting"`). `--description` is still accepted but prints a deprecation tip.
- **All checkpoint subcommands accept positional description** — `forge ship arch|test|breakdown|code|ship "description"` all now accept a positional arg.
- **`forge ship` spec checkpoint detects pre-generated `spec.yml`** — when `forge test spec` has already produced `.forge/specs/<slug>/spec.yml`, the spec checkpoint loads it and uses `InvokeWithKnowledge` for KB-enriched LLM review. Case count and families appear in the checkpoint detail line (e.g. `spec reviewed via KB — spec.yml: 9 cases, families: unit, integration`).
- **`forge ship` spec checkpoint generates `spec.md` from `spec.yml`** — when `spec.yml` is present but `spec.md` is absent, `spec.md` is generated from the YAML spec content (KB-enriched when an LLM provider is configured).
- **`cmdtest.ReadSpec(path)`** — exported function for reading `.forge/specs/<slug>/spec.yml` files from other packages.

### Changed

- **`--dry-run` defaults to `false`** — previously `forge ship` ran in dry-run mode by default; now it runs live. Pass `--dry-run` explicitly to preview without LLM calls or git operations.
- **Single-checkpoint output simplified** — running a single checkpoint (e.g. `forge ship spec`) shows focused output with a "next:" hint instead of the full 6-checkpoint pipeline header.
- **Checkpoint progress format** — full pipeline now shows `[1/6]`, `[2/6]` etc.
- **LLM tip message improved** — removed stale "M1 HTTP transport pending" reference.
- **No-description hint updated** — suggests `forge ship spec "<your feature>"` instead of referencing the deprecated `--description` flag.

### Fixed

- `forge ship` journey test now explicitly passes `--dry-run` when testing the dry-run flag (aligns with new default of `false`).

## [1.1.8] — 2026-05-24

### Fixed

- **`forge test spec <feature>` now writes by default** — `--dry-run` flag defaulted to `true`, so running `forge test spec <feature>` without any flags was silently doing a preview instead of writing the file. Default changed to `false`; use `--dry-run` explicitly to preview.
- **`forge test spec` is now a direct command (no `generate` subcommand)** — previously `forge test spec generate <feature>`; now just `forge test spec <feature>`. The `generate` subcommand was removed to match the documented short form.

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
