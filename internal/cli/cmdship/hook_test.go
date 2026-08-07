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

package cmdship

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ── loadHookConfig: strict-testing parsing ──────────────────────────────────

func TestLoadHookConfig_StrictTestingLine(t *testing.T) {
	root := t.TempDir()
	forgeDir := filepath.Join(root, ".forge")
	if err := os.MkdirAll(forgeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	yaml := "disabled:\n  - some-hook\nstrict-testing: true\n"
	if err := os.WriteFile(filepath.Join(forgeDir, "hooks.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := loadHookConfig(root)
	if !cfg.StrictTesting {
		t.Fatal("expected StrictTesting=true from 'strict-testing: true' line")
	}
	if cfg.Strict {
		t.Fatal("StrictTesting must not also set the unrelated Strict field")
	}
}

func TestLoadHookConfig_StrictAndStrictTestingIndependent(t *testing.T) {
	root := t.TempDir()
	forgeDir := filepath.Join(root, ".forge")
	if err := os.MkdirAll(forgeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Only the plain "strict: true" is set. The two gates are independent, so
	// turning Strict on must not be the thing that turns StrictTesting on —
	// and, since 1.8.2, must not turn it off either: StrictTesting keeps its
	// own default regardless of what Strict says.
	if err := os.WriteFile(filepath.Join(forgeDir, "hooks.yaml"), []byte("strict: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := loadHookConfig(root)
	if !cfg.Strict {
		t.Fatal("expected Strict=true")
	}
	if !cfg.StrictTesting {
		t.Fatal("'strict: true' must not disturb StrictTesting, which defaults on — they are independent gates")
	}
}

// TestLoadHookConfig_StrictTestingCanBeTurnedOff covers the half of the config
// parser that did not exist before 1.8.2. While the default was false the file
// could only ever turn the gate ON, so "strict-testing: false" was never read.
// With the default inverted, failing to read it would leave a project with no
// way to opt out at all — turning a default into a mandate.
func TestLoadHookConfig_StrictTestingCanBeTurnedOff(t *testing.T) {
	root := t.TempDir()
	forgeDir := filepath.Join(root, ".forge")
	if err := os.MkdirAll(forgeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(forgeDir, "hooks.yaml"),
		[]byte("strict-testing: false\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if cfg := loadHookConfig(root); cfg.StrictTesting {
		t.Fatal("'strict-testing: false' must disable the gate; without this a project cannot opt out")
	}
}

func TestLoadHookConfig_MissingFile(t *testing.T) {
	root := t.TempDir() // no .forge/hooks.yaml at all
	cfg := loadHookConfig(root)
	if cfg.Strict {
		t.Fatal("missing hooks.yaml must leave Strict off — it escalates every hook, not just testing")
	}
	// 1.8.2 inverted this. Testing evidence is not a premium feature of forge,
	// it is the product: a pipeline that ships a change while quietly noting
	// nobody verified it is doing the exact thing forge exists to prevent.
	if !cfg.StrictTesting {
		t.Fatal("missing hooks.yaml must default StrictTesting ON — shipping without evidence must be opt-in")
	}
}

// ── missingTestingPipelineStages ────────────────────────────────────────────

func TestMissingTestingPipelineStages(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    int // count of missing stages expected
	}{
		{"all four present", "Local: ran jest.\nPre-push: ci green.\nStaging: retested.\nProduction: smoke checked.", 0},
		{"case-insensitive match", "LOCAL tests ran. PRE-PUSH gate green. STAGING verified. PRODUCTION smoke ok.", 0},
		{"missing production only", "Local: done.\nPre-push: done.\nStaging: done.", 1},
		{"empty file", "", 4},
		{"unrelated content", "This feature does X and Y.", 4},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			missing := missingTestingPipelineStages(tc.content)
			if len(missing) != tc.want {
				t.Fatalf("content %q: got %d missing stages %v, want %d", tc.content, len(missing), missing, tc.want)
			}
		})
	}
}

func TestMissingTestingPipelineStages_CIKeywordNotAFalsePositiveMatch(t *testing.T) {
	// "ci" alone is a substring of common English words (specific, efficient,
	// ...) — the pre-push stage must key off "pre-push", not "ci", or content
	// that never mentions CI at all would incorrectly look documented.
	content := "We were very specific and efficient about local testing and staging and production."
	missing := missingTestingPipelineStages(content)
	found := false
	for _, m := range missing {
		if strings.Contains(m, "Pre-push") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected 'Pre-push / CI' stage to be reported missing (content has no 'pre-push' keyword, only incidental 'ci' substrings), got missing=%v", missing)
	}
}

// ── fourStageTestingGate ─────────────────────────────────────────────────────

func writeTestingPipelineEvidence(t *testing.T, root, description, content string) {
	t.Helper()
	path := testingPipelineEvidencePath(root, description)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

const completeEvidence = "## Local\nran jest + manual click-through\n\n## Pre-push\nCI green\n\n## Staging\nretested the fix live\n\n## Production\nsmoke-checked read-only\n"

func TestFourStageTestingGate_ResultNilIsNoOp(t *testing.T) {
	res := fourStageTestingGate.Handler(HookContext{Result: nil, StrictTesting: true})
	if res.Verdict == VerdictFail {
		t.Fatal("nil Result must never fail — there is nothing to gate yet")
	}
}

func TestFourStageTestingGate_UpstreamFailureIsNoOp(t *testing.T) {
	res := fourStageTestingGate.Handler(HookContext{
		Result:        &Checkpoint{Status: "fail"},
		StrictTesting: true,
	})
	if res.Verdict == VerdictFail {
		t.Fatal("an already-failed checkpoint must not additionally fail on missing testing evidence")
	}
}

func TestFourStageTestingGate_MissingFile_AdvisoryByDefault(t *testing.T) {
	root := t.TempDir()
	res := fourStageTestingGate.Handler(HookContext{
		Root:          root,
		Description:   "some feature",
		Result:        &Checkpoint{Status: "ok"},
		StrictTesting: false, // default
	})
	if res.Verdict == VerdictFail {
		t.Fatalf("StrictTesting=false must never fail on missing testing-pipeline.md, got: %s", res.Message)
	}
}

func TestFourStageTestingGate_MissingFile_BlocksWhenStrict(t *testing.T) {
	root := t.TempDir()
	res := fourStageTestingGate.Handler(HookContext{
		Root:          root,
		Description:   "some feature",
		Result:        &Checkpoint{Status: "ok"},
		StrictTesting: true,
	})
	if res.Verdict != VerdictFail {
		t.Fatalf("StrictTesting=true with no testing-pipeline.md at all must FAIL, got %v", res.Verdict)
	}
	if !strings.Contains(res.Message, "not found") {
		t.Fatalf("expected a 'not found' message, got: %s", res.Message)
	}
}

func TestFourStageTestingGate_IncompleteEvidence_AdvisoryByDefault(t *testing.T) {
	root := t.TempDir()
	writeTestingPipelineEvidence(t, root, "some feature", "## Local\nran jest\n")
	res := fourStageTestingGate.Handler(HookContext{
		Root:          root,
		Description:   "some feature",
		Result:        &Checkpoint{Status: "ok"},
		StrictTesting: false,
	})
	if res.Verdict == VerdictFail {
		t.Fatalf("StrictTesting=false must never fail on incomplete evidence, got: %s", res.Message)
	}
}

func TestFourStageTestingGate_IncompleteEvidence_BlocksWhenStrict(t *testing.T) {
	root := t.TempDir()
	writeTestingPipelineEvidence(t, root, "some feature", "## Local\nran jest\n") // missing 3 stages
	res := fourStageTestingGate.Handler(HookContext{
		Root:          root,
		Description:   "some feature",
		Result:        &Checkpoint{Status: "ok"},
		StrictTesting: true,
	})
	if res.Verdict != VerdictFail {
		t.Fatalf("StrictTesting=true with incomplete evidence must FAIL, got %v", res.Verdict)
	}
	for _, want := range []string{"Pre-push", "Staging", "Production"} {
		if !strings.Contains(res.Message, want) {
			t.Errorf("expected failure message to name missing stage %q, got: %s", want, res.Message)
		}
	}
}

func TestFourStageTestingGate_CompleteEvidence_PassesEvenWhenStrict(t *testing.T) {
	root := t.TempDir()
	writeTestingPipelineEvidence(t, root, "some feature", completeEvidence)
	res := fourStageTestingGate.Handler(HookContext{
		Root:          root,
		Description:   "some feature",
		Result:        &Checkpoint{Status: "ok"},
		StrictTesting: true,
	})
	if res.Verdict != VerdictPass {
		t.Fatalf("complete evidence must PASS even in strict mode, got %v: %s", res.Verdict, res.Message)
	}
}

// ── fourStageTestingReminder ─────────────────────────────────────────────────

func TestFourStageTestingReminder_AlwaysPasses(t *testing.T) {
	// Both strict and non-strict: the reminder is a post-pipeline hook and
	// must never itself fail the run — it only ever prints to stderr.
	for _, strict := range []bool{false, true} {
		res := fourStageTestingReminder.Handler(HookContext{
			Root:          t.TempDir(),
			Description:   "some feature",
			StrictTesting: strict,
		})
		if res.Verdict != VerdictPass {
			t.Fatalf("post-pipeline reminder hook must always pass (strict=%v), got %v", strict, res.Verdict)
		}
	}
}

// ── defaultHooks() wiring ────────────────────────────────────────────────────

func TestDefaultHooks_IncludesFourStageHooks(t *testing.T) {
	var haveGate, haveReminder bool
	for _, h := range defaultHooks() {
		switch h.Name {
		case "four-stage-testing-gate":
			haveGate = true
			if h.Phase != PhasePostCheckpoint || h.Gate != "qa-verify" {
				t.Errorf("four-stage-testing-gate must be PhasePostCheckpoint/qa-verify, got phase=%s gate=%s", h.Phase, h.Gate)
			}
		case "four-stage-testing-reminder":
			haveReminder = true
			if h.Phase != PhasePostPipeline {
				t.Errorf("four-stage-testing-reminder must be PhasePostPipeline, got phase=%s", h.Phase)
			}
		}
	}
	if !haveGate {
		t.Error("defaultHooks() missing four-stage-testing-gate")
	}
	if !haveReminder {
		t.Error("defaultHooks() missing four-stage-testing-reminder")
	}
}

// ── End-to-end: RunOptions.StrictTesting through the real qa-verify checkpoint ──
//
// hook_test.go above proves fourStageTestingGate's own Handler logic in
// isolation. These tests prove the full wiring actually connects:
// RunOptions.StrictTesting → hookCfg.StrictTesting (ship.go) →
// HookContext.StrictTesting → the checkpoint's escalation logic
// (hookCfg.Strict || (hookCfg.StrictTesting && strictTestingFailure)) — the
// exact chain a real `forge ship qa-verify --strict-testing` run exercises.
// Runs the real qa-verify checkpoint with no LLM pipe (nil-pipe-safe, per
// checkQAVerify's own tests) so no network/API key is needed.

func TestEndToEnd_QAVerify_StrictTestingOff_NeverFailsOnMissingEvidence(t *testing.T) {
	root := t.TempDir()
	// Since 1.8.2 `StrictTesting: false` is no longer how the gate is turned
	// off — it is the default-on state, and only an explicit NoStrictTesting
	// waives it. Asserting the old field still disabled the gate would have
	// quietly re-tested nothing.
	res := RunWithOptions(RunOptions{
		Root:            root,
		Description:     "e2e no strict",
		Names:           []string{"qa-verify"},
		NoStrictTesting: true,
	})
	if len(res.Checkpoints) != 1 {
		t.Fatalf("expected 1 checkpoint, got %d", len(res.Checkpoints))
	}
	cp := res.Checkpoints[0]
	if cp.Status == "fail" {
		t.Fatalf("StrictTesting=false must never hard-fail qa-verify over missing testing-pipeline.md; got Detail=%q", cp.Detail)
	}
	if strings.Contains(cp.Detail, "four-stage-testing-gate") {
		t.Fatalf("four-stage-testing-gate must not even appear in Detail when advisory and passing; got Detail=%q", cp.Detail)
	}
}

func TestEndToEnd_QAVerify_StrictTestingOn_MissingEvidence_HardFails(t *testing.T) {
	root := t.TempDir()
	res := RunWithOptions(RunOptions{
		Root:          root,
		Description:   "e2e strict missing",
		Names:         []string{"qa-verify"},
		StrictTesting: true,
	})
	if res.Ready {
		t.Fatal("pipeline must not be Ready when qa-verify hard-fails on missing strict-testing evidence")
	}
	if len(res.Checkpoints) != 1 {
		t.Fatalf("expected 1 checkpoint, got %d", len(res.Checkpoints))
	}
	cp := res.Checkpoints[0]
	if cp.Status != "fail" {
		t.Fatalf("StrictTesting=true with no testing-pipeline.md must hard-fail qa-verify; got Status=%q Detail=%q", cp.Status, cp.Detail)
	}
	if !strings.Contains(cp.Detail, "four-stage-testing-gate") {
		t.Fatalf("expected Detail to name four-stage-testing-gate as the cause, got: %s", cp.Detail)
	}
}

func TestEndToEnd_QAVerify_StrictTestingOn_CompleteEvidence_DoesNotFailOnThisGate(t *testing.T) {
	root := t.TempDir()
	writeTestingPipelineEvidence(t, root, "e2e strict complete", completeEvidence)
	res := RunWithOptions(RunOptions{
		Root:          root,
		Description:   "e2e strict complete",
		Names:         []string{"qa-verify"},
		StrictTesting: true,
	})
	cp := res.Checkpoints[0]
	if strings.Contains(cp.Detail, "four-stage-testing-gate") {
		t.Fatalf("complete testing-pipeline.md must not trigger four-stage-testing-gate, got Detail=%q", cp.Detail)
	}
	// NB: cp.Status can still legitimately be "warning"/"fail" here from the
	// OTHER pre-existing qa-verify hooks (manual-test-plan-gate,
	// qa-coverage-gate) firing on this empty scratch project with no real
	// LLM-generated artefacts — that's expected and out of scope for this
	// test, which only asserts THIS gate's independence.
}

func TestEndToEnd_QAVerify_StrictTestingViaHooksYaml_MatchesFlagBehavior(t *testing.T) {
	root := t.TempDir()
	forgeDir := filepath.Join(root, ".forge")
	if err := os.MkdirAll(forgeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(forgeDir, "hooks.yaml"), []byte("strict-testing: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// No --strict-testing flag (RunOptions.StrictTesting left false) — the
	// project-level .forge/hooks.yaml setting alone must be enough.
	res := RunWithOptions(RunOptions{
		Root:        root,
		Description: "e2e hooks.yaml strict",
		Names:       []string{"qa-verify"},
	})
	cp := res.Checkpoints[0]
	if cp.Status != "fail" {
		t.Fatalf("hooks.yaml 'strict-testing: true' alone (no CLI flag) must hard-fail qa-verify on missing evidence; got Status=%q Detail=%q", cp.Status, cp.Detail)
	}
}
