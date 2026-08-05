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
// Some hooks are advisory by design and can never return Passed=false. Those
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
// A gate that returns Passed for its known-bad fixture is broken, whatever its
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
			if res.Passed {
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
			if !res.Passed {
				t.Fatalf("%s REJECTED its known-good fixture: %s\n"+
					"A gate that cries wolf gets switched off, and then it protects nothing.",
					m.hook.Name, res.Message)
			}
		})
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

// TestSpecCodeAlignment_SilentlyPassesWithoutSpecMD documents a real gap that
// the mutation table exposed on its very first run, and pins the current
// behaviour so a future change to it is deliberate.
//
// auditSlug() returns early when spec.md is absent, so spec-code-alignment-gate
// reports PASS on a project with unfinished tasks — it never looked. In a
// normal pipeline spec.md exists by the time the code checkpoint runs, so this
// is not reachable by accident. It is reachable on purpose: `forge ship
// --from=code` on a project whose spec was never written, or one where spec.md
// was deleted, gets a green alignment gate that verified nothing.
//
// This is the M3 problem in miniature — "could not check" and "checked, fine"
// are the same value in a bool. It is left as-is here rather than flipped to a
// failure, because failing every spec-less run is a behaviour change that
// belongs in its own release, not smuggled in with a test file. The point of
// recording it is that it is now a known gap with a name, instead of an
// unexamined green.
func TestSpecCodeAlignment_SilentlyPassesWithoutSpecMD(t *testing.T) {
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
	if !res.Passed {
		t.Fatal("behaviour changed: the gate now fails without spec.md. That is arguably " +
			"the better behaviour — update this test and note it in the changelog as a " +
			"deliberate change, rather than deleting the test.")
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
	// Passed when the checkpoint already failed, so a "fail" here would make
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
