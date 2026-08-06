// Copyright 2024 The Forge Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// gate_mutation_test.go — tests for the gates themselves.
//
// # What this file is for
//
// Every other test in this package asks "does the pipeline behave correctly?"
// This one asks a different question: **do the quality gates actually check
// anything?**
//
// That is not paranoia. A gate can pass its own review, ship, run in CI for
// months, and check nothing at all. It happened in 1.8.1: the test-reachability
// checker compiled `\\.` — "a literal backslash then any character" — from a
// config source that meant the regex `\.`. The pattern matched nothing in any
// real path, so every ignore rule looked inert, nothing appeared excluded, and
// the checker reported the dead zone it was written to catch as perfectly fine.
// It was green, it was wrong, and nothing in the suite would have noticed,
// because every test asked whether good input passed.
//
// A gate that cannot fail is not a gate. So each one is run against input that
// is *known to be bad*, and must reject it.
//
// # The completeness guard matters more than the fixtures
//
// TestGateMutation_EveryDefaultHookIsCovered fails the build when a hook is
// added to defaultHooks() without a mutation entry here. Without it this file
// silently decays: the gates written next year get no coverage, and the suite
// keeps reporting green over an ever-larger blind spot. A safety net with a
// growing hole in it is worse than none, because it is still trusted.
//
// # Gates that cannot fail
//
// Some hooks are advisory by design and can never return VerdictFail. Those
// are declared with alwaysPasses and a written reason, rather than being left
// out of the table. Omission and "deliberately cannot fail" look identical in
// a diff; one is a decision and the other is an oversight.
package cmdship

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// gateMutation is one gate plus the two inputs that prove it discriminates.
type gateMutation struct {
	// hook is the gate under test.
	hook Hook
	// checkpoint is the CheckpointName the gate fires for.
	checkpoint string
	// description drives the slug the gate resolves its artefact paths from.
	description string
	// good is a set of artefact files (relative to .forge/specs/<slug>/) that
	// the gate must accept.
	good map[string]string
	// bad is a set the gate must reject. The whole point of the file.
	bad map[string]string
	// selfReports is true when the gate returns a verdict on an EMPTY project
	// — i.e. it notices its own artefact is missing and says so, rather than
	// reporting clean. Gates that read a file must set this; a gate that reads
	// nothing (there are none today) would not.
	//
	// This is the M3 half of the mutation table. `bad` proves the gate can
	// fail on wrong content; this proves it does not silently pass on no
	// content at all — the more common and much quieter defect.
	selfReports bool
	// alwaysPasses marks an advisory hook that cannot fail by design. reason
	// is required with it — an assertion-free entry has to justify itself.
	alwaysPasses bool
	reason       string
}

const mutationFeature = "gate mutation feature"

// mutationTable is the mutation coverage for every hook in defaultHooks().
//
// The `bad` fixtures are deliberately the *realistic* failure, not an absurd
// one: an empty file trips almost any check and proves almost nothing. Each
// one here is a plausible artefact that a real run could produce and that the
// gate exists to reject.
var mutationTable = []gateMutation{
	{
		hook:        specCompletenessGate,
		checkpoint:  "spec",
		description: mutationFeature,
		good: map[string]string{
			"spec.md": "# Spec\n\n## Acceptance Criteria\n\n- Given a signed-in user\n  When they exceed the rate limit\n  Then the API returns 429\n",
		},
		// Plausible-looking spec prose with no testable criteria — exactly what
		// an LLM produces when it drifts into summary mode.
		selfReports: true,
		bad: map[string]string{
			"spec.md": "# Spec\n\nThis feature adds rate limiting to the public API so the service stays responsive under load.\n",
		},
	},
	{
		hook:        adrQualityGate,
		checkpoint:  "arch",
		description: mutationFeature,
		good: map[string]string{
			"adr.md": "# ADR-001\n\n## Alternatives\n\nAlternative A: token bucket.\nOption B: leaky bucket.\n\n## Consequences\n\nHigher memory per key.\n",
		},
		// A decision recorded with no alternatives weighed and no cost stated:
		// an ADR in name only.
		selfReports: true,
		bad: map[string]string{
			"adr.md": "# ADR-001\n\nWe will use a token bucket.\n",
		},
	},
	{
		hook:        archFileLint,
		checkpoint:  "arch",
		description: mutationFeature,
		good: map[string]string{
			"arch.md": "# Architecture\n\n## Components\n\nA limiter middleware in front of the router.\n",
		},
		// A heading immediately followed by another heading — the shape a
		// truncated or skeleton-only generation leaves behind.
		selfReports: true,
		bad: map[string]string{
			"arch.md": "# Architecture\n## Components\n### Limiter\n",
		},
	},
	{
		hook:        tddGate,
		checkpoint:  "test",
		description: mutationFeature,
		good: map[string]string{
			"tests.md": "# Tests\n\nScenario: over the limit\nGiven 100 requests\nWhen the 101st arrives\nThen it is rejected\n",
		},
		// The single most dangerous artefact in the whole pipeline: a test file
		// that runs, passes, and asserts nothing.
		selfReports: true,
		bad: map[string]string{
			"tests.md": "# Tests\n\nScenario: over the limit\nGiven 100 requests\n\n```ts\nexpect(true).toBe(true)\n```\n",
		},
	},
	{
		hook:        breakdownCompletenessGate,
		checkpoint:  "breakdown",
		description: mutationFeature,
		good: map[string]string{
			"tasks.md": "# Tasks\n\n- [ ] Add the limiter middleware\n- [ ] Add the 429 response path\n",
		},
		// Checkboxes with nothing after them: a breakdown that counts as work
		// done while describing no work at all.
		selfReports: true,
		bad: map[string]string{
			"tasks.md": "# Tasks\n\n- [ ]\n- [ ]\n",
		},
	},
	{
		hook:        taskCompletionGate,
		checkpoint:  "code",
		description: mutationFeature,
		good: map[string]string{
			"tasks.md": "# Tasks\n\n- [x] Add the limiter middleware\n- [x] Add the 429 response path\n",
		},
		selfReports: true,
		bad: map[string]string{
			"tasks.md": "# Tasks\n\n- [x] Add the limiter middleware\n- [ ] Add the 429 response path\n",
		},
	},
	{
		hook:        securityHygieneGate,
		checkpoint:  "code",
		description: mutationFeature,
		good: map[string]string{
			"impl-notes.md": "# Notes\n\nThe limiter reads its config from the environment via the settings loader.\n",
		},
		selfReports: true,
		bad: map[string]string{
			"impl-notes.md": "# Notes\n\nFor local testing we set api_key = \"sk-live-not-a-real-key\" in the handler.\n",
		},
	},
	{
		hook:        qaCoverageGate,
		checkpoint:  "qa-verify",
		description: mutationFeature,
		good: map[string]string{
			"spec.md":             "# Spec\n\nGiven a signed-in user over the limit\n",
			"qa-report.md":        "# QA\n\nVerified: Given a signed-in user over the limit — returns 429.\n",
			"manual-test-plan.md": manualTestPlanAllRoles,
		},
		// A QA report that reviews something other than the acceptance criteria
		// it is supposed to be evidence for.
		selfReports: true,
		bad: map[string]string{
			"spec.md":             "# Spec\n\nGiven a signed-in user over the limit\n",
			"qa-report.md":        "# QA\n\nRan the suite. Everything looked fine.\n",
			"manual-test-plan.md": manualTestPlanAllRoles,
		},
	},
	{
		hook:        manualTestPlanGate,
		checkpoint:  "qa-verify",
		description: mutationFeature,
		good: map[string]string{
			"manual-test-plan.md": manualTestPlanAllRoles,
		},
		// Half the review roles missing: a plan that looks complete at a glance
		// but leaves security and compliance unexamined.
		selfReports: true,
		bad: map[string]string{
			"manual-test-plan.md": "# Manual Test Plan\n\n## Product Owner\n- Check the happy path.\n\n## Business Analyst\n- Check the edge cases.\n",
		},
	},
	{
		hook:        fourStageTestingGate,
		checkpoint:  "qa-verify",
		description: mutationFeature,
		good: map[string]string{
			"testing-pipeline.md": testingPipelineAllStages,
		},
		// Evidence for the two cheap stages and silence on the two that
		// actually exercise a deployed system.
		selfReports: true,
		bad: map[string]string{
			"testing-pipeline.md": "# Testing Pipeline\n\n## Stage 1 — Local\nRan the suite locally.\n\n## Stage 2 — Pre-push\nCI is green.\n",
		},
	},
	{
		hook:        specCodeAlignmentGateCode,
		checkpoint:  "code",
		description: mutationFeature,
		// spec.md is required in both fixtures, not decoration: auditSlug()
		// returns early when it is absent, so a tasks-only fixture would have
		// exercised nothing and passed vacuously. It did, on the first run of
		// this table — see TestSpecCodeAlignment_SilentlyPassesWithoutSpecMD.
		good: map[string]string{
			"spec.md":  "# Spec\n\nGiven a user over the limit\n",
			"tasks.md": "# Tasks\n\n- [x] Add the limiter middleware\n",
		},
		selfReports: true,
		bad: map[string]string{
			"spec.md":  "# Spec\n\nGiven a user over the limit\n",
			"tasks.md": "# Tasks\n\n- [ ] Add the limiter middleware\n",
		},
	},
	{
		hook:        specCodeAlignmentGateQA,
		checkpoint:  "qa-verify",
		description: mutationFeature,
		// spec.md is required in both fixtures, not decoration: auditSlug()
		// returns early when it is absent, so a tasks-only fixture would have
		// exercised nothing and passed vacuously. It did, on the first run of
		// this table — see TestSpecCodeAlignment_SilentlyPassesWithoutSpecMD.
		good: map[string]string{
			"spec.md":  "# Spec\n\nGiven a user over the limit\n",
			"tasks.md": "# Tasks\n\n- [x] Add the limiter middleware\n",
		},
		selfReports: true,
		bad: map[string]string{
			"spec.md":  "# Spec\n\nGiven a user over the limit\n",
			"tasks.md": "# Tasks\n\n- [ ] Add the limiter middleware\n",
		},
	},
	{
		hook:        selfReviewGate,
		checkpoint:  "spec",
		description: mutationFeature,
		good: map[string]string{
			"spec.md": "# Spec\n\n## Acceptance Criteria\n\nGiven a user\nWhen over the limit\nThen 429\n",
		},
		selfReports: true,
		bad: map[string]string{
			"spec.md": "# Spec\n\n## Acceptance Criteria\n\nTODO: write the acceptance criteria\n",
		},
	},
	{
		hook:         fourStageTestingReminder,
		checkpoint:   "",
		alwaysPasses: true,
		reason: "post-pipeline reminder: it prints the 4-stage checklist to stderr after a " +
			"successful run and is documented as never blocking. The blocking half of the " +
			"same concern is fourStageTestingGate, which is covered above.",
	},
}

const manualTestPlanAllRoles = `# Manual Test Plan

## Product Owner (UAT)
- Confirm the limit matches what was requested.

## Business Analyst
- Confirm the edge cases are enumerated.

## Quality Engineer
- Confirm negative cases are covered.

## Security Reviewer
- Confirm the limit cannot be bypassed by header spoofing.

## DevOps / SRE
- Confirm the limiter degrades safely when the store is unreachable.

## Compliance / CPO
- Confirm no personal data is written to the limiter log.
`

const testingPipelineAllStages = `# Testing Pipeline

## Stage 1 — Local
Ran the suite locally against a live local app.

## Stage 2 — Pre-push / CI
Lint, type-check, unit and integration suites all green.

## Stage 3 — Staging
Deployed to staging and retested the changed behaviour by hand.

## Stage 4 — Production
Read-only smoke check after promotion; no live mutations.
`

// ── The mutation test itself ──────────────────────────────────────────────────

// TestGateMutation_EveryGateRejectsKnownBadInput is the heart of this file.
//
// A gate that does not return VerdictFail for its known-bad fixture is broken,
// whatever its
// own unit tests say — those assert that good input passes, which a function
// returning `true` unconditionally also does.
func TestGateMutation_EveryGateRejectsKnownBadInput(t *testing.T) {
	t.Parallel()
	for _, m := range mutationTable {
		if m.alwaysPasses {
			continue
		}
		t.Run(m.hook.Name+"/"+m.checkpoint+"/rejects-bad", func(t *testing.T) {
			t.Parallel()
			res := runGateAgainst(t, m, m.bad)
			if res.Verdict != VerdictFail {
				t.Fatalf("%s PASSED its known-bad fixture — this gate does not check what it claims to.\n"+
					"Bad fixture: %v", m.hook.Name, keysOf(m.bad))
			}
			if strings.TrimSpace(res.Message) == "" {
				t.Errorf("%s rejected the bad fixture but gave no message — a failure the user "+
					"cannot act on is barely better than no gate", m.hook.Name)
			}
		})
	}
}

// TestGateMutation_EveryGateAcceptsKnownGoodInput is the false-positive guard.
// A gate that rejects everything blocks every pipeline, gets disabled, and then
// protects nothing at all — the same end state as a gate that checks nothing,
// reached from the other direction.
func TestGateMutation_EveryGateAcceptsKnownGoodInput(t *testing.T) {
	t.Parallel()
	for _, m := range mutationTable {
		if m.alwaysPasses {
			continue
		}
		t.Run(m.hook.Name+"/"+m.checkpoint+"/accepts-good", func(t *testing.T) {
			t.Parallel()
			res := runGateAgainst(t, m, m.good)
			if res.Verdict != VerdictPass {
				t.Fatalf("%s REJECTED its known-good fixture: %s\n"+
					"A gate that cries wolf gets switched off, and then it protects nothing.",
					m.hook.Name, res.Message)
			}
		})
	}
}

// TestGateMutation_NoGateReportsCleanOnAnEmptyProject is the M3 half.
//
// The `bad` fixtures above prove a gate can fail on *wrong* content. This
// proves it does not report clean on *no* content — a quieter and far more
// common defect, because the code path that produces it reads "the artefact
// isn't there yet, nothing to complain about" and looks entirely reasonable.
//
// spec-code-alignment-gate did exactly that: on a project with unfinished
// tasks and no spec.md it returned pass, having skipped every check. Nothing
// distinguished that from a genuinely clean project until Verdict gained a
// third state.
func TestGateMutation_NoGateReportsCleanOnAnEmptyProject(t *testing.T) {
	t.Parallel()
	for _, m := range mutationTable {
		if m.alwaysPasses || !m.selfReports {
			continue
		}
		t.Run(m.hook.Name+"/"+m.checkpoint+"/empty-project", func(t *testing.T) {
			t.Parallel()
			res := runGateAgainst(t, m, nil) // no artefacts at all
			if res.Verdict == VerdictPass {
				t.Fatalf("%s reported PASS on a project with none of its artefacts present. "+
					"It verified nothing and said everything was fine — return VerdictUnknown "+
					"with a reason instead.", m.hook.Name)
			}
			if strings.TrimSpace(res.Message) == "" {
				t.Errorf("%s gave a non-pass verdict with no message; the user cannot act on that",
					m.hook.Name)
			}
		})
	}
}

// TestVerdict_UnknownIsTheZeroValue pins the property the whole type rests on.
//
// A handler that forgets to set a verdict must yield "unverified", not a false
// pass. If VerdictPass ever becomes iota's first value, every incomplete
// handler in the codebase silently starts reporting success — which is the
// exact failure this type was introduced to make impossible.
func TestVerdict_UnknownIsTheZeroValue(t *testing.T) {
	t.Parallel()
	var zero Verdict
	if zero != VerdictUnknown {
		t.Fatal("VerdictUnknown must be the zero value: a forgotten verdict has to mean " +
			"'unverified', never 'fine'")
	}
	if (HookResult{}).Verdict != VerdictUnknown {
		t.Fatal("a zero HookResult must be unverified")
	}
	if VerdictUnknown.String() != "unverified" {
		t.Errorf("Verdict.String() must not soften the unknown case: %q", VerdictUnknown.String())
	}
}

// TestPartitionResults_KeepsFailuresAndUnverifiedApart guards the distinction
// at the point it is consumed. Merging the two buckets would restore the old
// behaviour without touching the type at all.
func TestPartitionResults_KeepsFailuresAndUnverifiedApart(t *testing.T) {
	t.Parallel()
	failures, unverified := partitionResults([]HookResult{
		{Verdict: VerdictFail, Message: "real problem", HookName: "a"},
		{Verdict: VerdictUnknown, Message: "could not check", HookName: "b"},
		{Verdict: VerdictPass, HookName: "c"},
	})
	if len(failures) != 1 || failures[0].HookName != "a" {
		t.Fatalf("failures: %+v", failures)
	}
	if len(unverified) != 1 || unverified[0].HookName != "b" {
		t.Fatalf("unverified: %+v", unverified)
	}
}

// TestGateMutation_EveryDefaultHookIsCovered stops this file from decaying.
//
// Without it, a hook added next year gets no mutation coverage and the suite
// still reports green — a safety net with a growing hole, which is worse than
// no net because it is still trusted.
func TestGateMutation_EveryDefaultHookIsCovered(t *testing.T) {
	t.Parallel()
	// Keyed on the hook's own Gate, not the fixture's checkpoint: a hook with
	// Gate=="" fires for every checkpoint, so the fixture picks one to exercise
	// it through while the hook itself is still a single entry to cover.
	covered := map[string]bool{}
	for _, m := range mutationTable {
		covered[mutationKey(m.hook.Name, m.hook.Gate)] = true
	}
	for _, h := range defaultHooks() {
		if !covered[mutationKey(h.Name, h.Gate)] {
			t.Errorf("hook %q (gate %q) has no entry in mutationTable.\n"+
				"Add one with a known-bad fixture it must reject — or, if it genuinely "+
				"cannot fail, an alwaysPasses entry stating why. Silence is not an option: "+
				"an uncovered gate and a gate that checks nothing look identical from here.",
				h.Name, h.Gate)
		}
	}
}

// TestGateMutation_AlwaysPassesEntriesAreJustified keeps the escape hatch from
// becoming the easy way to silence the completeness guard.
func TestGateMutation_AlwaysPassesEntriesAreJustified(t *testing.T) {
	t.Parallel()
	for _, m := range mutationTable {
		if !m.alwaysPasses {
			continue
		}
		if len(strings.TrimSpace(m.reason)) < 40 {
			t.Errorf("hook %q is marked alwaysPasses with no real justification. "+
				"Declaring a gate unable to fail is a design claim and has to be argued, "+
				"not asserted.", m.hook.Name)
		}
	}
}

// ── What the first run of this table found ────────────────────────────────────

// TestSpecCodeAlignment_ReportsUnverifiedWithoutSpecMD is the mutation table's
// first find, now fixed.
//
// auditSlug() returns early when spec.md is absent, so every gap check in
// spec-code-alignment-gate is skipped. That used to fall through to PASS: on a
// project with unfinished tasks the gate reported clean, having never looked.
// Not reachable by accident in a normal pipeline, but very reachable on
// purpose — `forge ship --from=code` on a project whose spec was never
// written, or one where spec.md was deleted.
//
// It is now VerdictUnknown. This is exactly what the third verdict is for: the
// gate did not find the project acceptable, it found it unexaminable, and
// those are different facts. The checkpoint is annotated UNVERIFIED rather
// than silently green, and the run is not blocked over something that may well
// be intentional.
func TestSpecCodeAlignment_ReportsUnverifiedWithoutSpecMD(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	slug := slugify(mutationFeature)
	specDir := filepath.Join(root, ".forge", "specs", slug)
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Unfinished work, and no spec to audit it against.
	if err := os.WriteFile(filepath.Join(specDir, "tasks.md"),
		[]byte("# Tasks\n\n- [ ] Add the limiter middleware\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	res := specCodeAlignmentHandler(HookContext{
		CheckpointName: "code",
		Root:           root,
		Description:    mutationFeature,
		Result:         &Checkpoint{Name: "code", Status: "ok"},
	})
	if res.Verdict == VerdictPass {
		t.Fatal("the gate reported PASS without a spec to audit against — it verified nothing")
	}
	if res.Verdict != VerdictUnknown {
		t.Fatalf("want VerdictUnknown (unexaminable, not broken), got %v: %s", res.Verdict, res.Message)
	}
	if !strings.Contains(res.Message, "spec.md") {
		t.Errorf("the message must name what was missing: %q", res.Message)
	}
}

// TestPreCheckpointHooks_AreRegisteredButNeverRun records the largest gap the
// mutation work turned up, and fails the moment it is closed so the change is
// noticed rather than absorbed.
//
// self-review-gate is declared PhasePreCheckpoint, listed in defaultHooks(),
// documented in this package's header comment, and covered by tests. It has
// never executed. runWithOptions calls runHooks for PhasePostCheckpoint and
// PhasePostPipeline only — there is no PhasePreCheckpoint call site anywhere
// in the package.
//
// This is the failure mode one level up from a gate that checks nothing: a
// gate that never runs at all. Everything *about* it is correct — the handler
// works, the tests pass, the docs describe it — and none of that was ever
// reachable. Counting it among forge's quality gates has been inaccurate since
// it was written.
//
// Not wired in here on purpose. Turning on a gate that has never fired will
// flag artefacts in projects that have been shipping happily, and that belongs
// in a release with a changelog entry, not in a test file. Recorded so it is a
// decision someone makes rather than a fact nobody knows.
func TestPreCheckpointHooks_AreRegisteredButNeverRun(t *testing.T) {
	t.Parallel()

	var preCheckpoint []string
	for _, h := range defaultHooks() {
		if h.Phase == PhasePreCheckpoint {
			preCheckpoint = append(preCheckpoint, h.Name)
		}
	}
	if len(preCheckpoint) == 0 {
		t.Skip("no pre-checkpoint hooks registered; nothing to record")
	}

	// A full run with an artefact that the gate would reject outright. If
	// pre-checkpoint hooks were wired in, this would surface in the output.
	root := t.TempDir()
	slug := slugify(mutationFeature)
	specDir := filepath.Join(root, ".forge", "specs", slug)
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "spec.md"),
		[]byte("# Spec\n\n## Acceptance Criteria\n\nTODO: write these\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	res := RunWithOptions(RunOptions{
		Root:            root,
		Description:     mutationFeature,
		Names:           []string{"spec"},
		NoStrictTesting: true,
	})

	var detail string
	for _, cp := range res.Checkpoints {
		detail += cp.Detail
	}
	if strings.Contains(detail, "self-review-gate") {
		t.Fatalf("pre-checkpoint hooks now run — %v fire in the pipeline.\n"+
			"That is the better behaviour. Update this test, and give the change a "+
			"changelog entry: projects that have been shipping clean will start seeing "+
			"findings from a gate that never fired before.", preCheckpoint)
	}
}

// ── harness ───────────────────────────────────────────────────────────────────

// runGateAgainst materialises the fixture into a temp project and runs the
// gate's handler against it.
func runGateAgainst(t *testing.T, m gateMutation, files map[string]string) HookResult {
	t.Helper()
	root := t.TempDir()
	slug := slugify(m.description)
	specDir := filepath.Join(root, ".forge", "specs", slug)
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(specDir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	// Result must be non-fail: every post-checkpoint gate short-circuits to
	// not-applicable when the checkpoint already failed, so a "fail" here would make
	// the whole table pass vacuously.
	result := &Checkpoint{Name: m.checkpoint, Status: "ok"}
	return m.hook.Handler(HookContext{
		Phase:          m.hook.Phase,
		CheckpointName: m.checkpoint,
		Root:           root,
		Description:    m.description,
		Result:         result,
		StrictTesting:  true,
	})
}

func mutationKey(name, gate string) string { return name + "\x00" + gate }

func keysOf(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
