// Copyright 2024 The Forge Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// testing_pipeline.go — the 4-stage testing pipeline: local, pre-push/CI,
// staging, production. `forge ship`'s own qa-verify checkpoint only proves a
// feature was specced, coded, and unit-tested against the LLM's own test
// suite — it never proves a human (or an agent driving a real browser)
// actually exercised the running app anywhere past localhost. This file adds
// two hooks that close that gap without inventing a new checkpoint:
//
//   - fourStageTestingReminder (post-pipeline): always prints the 4-stage
//     checklist after a successful run. Pure reminder, never blocks — this
//     is the default, guideline-only behavior for every project.
//   - fourStageTestingGate (post-checkpoint, qa-verify): checks for
//     .forge/specs/<slug>/testing-pipeline.md evidence that all 4 stages
//     ran. Advisory (HookResult.Passed always true) unless
//     HookConfig.StrictTesting is set, in which case missing/incomplete
//     evidence fails the qa-verify checkpoint the same way
//     manualTestPlanGate already does for the manual test plan.
//
// StrictTesting is deliberately its own HookConfig field, independent of the
// pre-existing Strict field — a project opting into "enforce the testing
// pipeline" must not silently also turn every other advisory hook
// (self-review-gate, spec-completeness-gate, etc.) into a blocking one.
package cmdship

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// testingPipelineStage names one of the 4 stages, in order, plus the
// case-insensitive keyword fourStageTestingGate looks for in
// testing-pipeline.md to consider that stage documented. Keywords are
// chosen to avoid short/common substrings (e.g. "ci" alone would match
// "specific", "efficient", ...).
type testingPipelineStage struct {
	label   string
	keyword string
}

var testingPipelineStages = []testingPipelineStage{
	{label: "Stage 1 — Local (automated tests + manual run against a live local app)", keyword: "local"},
	{label: "Stage 2 — Pre-push / CI (lint, type-check, unit + integration suites)", keyword: "pre-push"},
	{label: "Stage 3 — Staging (deploy, then manually retest the actual changed behavior)", keyword: "staging"},
	{label: "Stage 4 — Production (read-only smoke check after promotion; no live mutations)", keyword: "production"},
}

// testingPipelineEvidencePath returns where fourStageTestingGate expects
// evidence for the current feature. Mirrors manualTestPlanGate's and
// qaCoverageGate's own artefact path convention exactly
// (.forge/specs/<slug>/<name>.md).
func testingPipelineEvidencePath(root, description string) string {
	return filepath.Join(root, ".forge", "specs", slugify(description), "testing-pipeline.md")
}

// missingTestingPipelineStages reports which stage keywords are absent from
// content (case-insensitive substring match, same style as
// manualTestPlanGate's 6-role section check).
func missingTestingPipelineStages(content string) []string {
	lower := strings.ToLower(content)
	var missing []string
	for _, s := range testingPipelineStages {
		if !strings.Contains(lower, s.keyword) {
			missing = append(missing, s.label)
		}
	}
	return missing
}

// fourStageTestingGate is the qa-verify blocking gate. Advisory by default:
// a missing/incomplete testing-pipeline.md never fails the checkpoint unless
// ctx.StrictTesting is true.
var fourStageTestingGate = Hook{
	Name:  "four-stage-testing-gate",
	Phase: PhasePostCheckpoint,
	Gate:  "qa-verify",
	Handler: func(ctx HookContext) HookResult {
		if ctx.Result == nil || ctx.Result.Status == "fail" {
			return HookResult{Passed: true}
		}

		path := testingPipelineEvidencePath(ctx.Root, ctx.Description)
		data, err := os.ReadFile(path)
		if err != nil {
			if !ctx.StrictTesting {
				return HookResult{Passed: true}
			}
			return HookResult{
				Passed: false,
				Message: fmt.Sprintf(
					"four-stage-testing-gate: %s not found. This gate became blocking by "+
						"default in 1.8.2 — if this run passed on an earlier version, that is why. "+
						"Either document evidence for all 4 stages (local / pre-push / staging / "+
						"production) in %s, or waive the gate: `forge ship --no-strict-testing` for "+
						"one run, or \"strict-testing: false\" in .forge/hooks.yaml for the project",
					filepath.Base(path), filepath.Base(path),
				),
			}
		}

		missing := missingTestingPipelineStages(string(data))
		if len(missing) == 0 {
			return HookResult{Passed: true}
		}
		if !ctx.StrictTesting {
			return HookResult{Passed: true}
		}
		return HookResult{
			Passed: false,
			Message: fmt.Sprintf(
				"four-stage-testing-gate: testing-pipeline.md missing evidence for: %s",
				strings.Join(missing, "; "),
			),
		}
	},
}

// fourStageTestingReminder is the post-pipeline advisory hook. It always
// prints the checklist to stderr after every successful `forge ship` run
// (stderr, not stdout, so it never corrupts `--json` output — see the
// call site's "post-pipeline failures are advisory only" comment in
// ship.go, which already discards this hook's HookResult entirely; the
// print is this hook's real effect, not its return value).
var fourStageTestingReminder = Hook{
	Name:  "four-stage-testing-reminder",
	Phase: PhasePostPipeline,
	Handler: func(ctx HookContext) HookResult {
		var b strings.Builder
		b.WriteString("\n4-stage testing pipeline — before calling this done:\n")
		for i, s := range testingPipelineStages {
			b.WriteString(fmt.Sprintf("  %d. %s\n", i+1, s.label))
		}
		if ctx.StrictTesting {
			b.WriteString("  (--strict-testing is ON: qa-verify already enforced this via testing-pipeline.md)\n")
		} else {
			b.WriteString(fmt.Sprintf("  Advisory only. Document evidence in %s and re-run with\n  --strict-testing to make this a blocking gate.\n",
				filepath.Base(testingPipelineEvidencePath(ctx.Root, ctx.Description))))
		}
		fmt.Fprint(os.Stderr, b.String())
		return HookResult{Passed: true}
	},
}
