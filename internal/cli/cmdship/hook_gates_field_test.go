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

package cmdship

import (
	"os"
	"path/filepath"
	"testing"
)

// Field reports from a Next.js/Supabase project running `forge ship --agent-mode`
// (ISSUE 6/17, 14 and 15 in its forge issue log). Each case reproduced on 1.10.10.

func writeSpecFile(t *testing.T, root, slug, name, body string) {
	t.Helper()
	dir := filepath.Join(root, ".forge", "specs", slug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func gateCtx(root, slug string) HookContext {
	return HookContext{Root: root, SpecName: slug, Result: &Checkpoint{Status: "ok"}}
}

func TestSpecCompletenessGate_AcceptsNaturalGherkinProse(t *testing.T) {
	cases := map[string]string{
		"lowercase when/then": "## Acceptance Criteria\n- AC-1: Given a draft post, when it is scheduled, then it appears in the queue.\n",
		"bold markers":        "## Acceptance Criteria\n- AC-1 **Given** a draft **When** scheduled **Then** queued.\n",
		"all lowercase":       "## Acceptance criteria\n1. given a draft, when scheduled, then queued\n",
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			writeSpecFile(t, root, "f", "spec.md", body)
			if res := specCompletenessGate.Handler(gateCtx(root, "f")); res.Verdict != VerdictPass {
				t.Fatalf("valid Given/When/Then AC rejected: %s", res.Message)
			}
		})
	}
}

func TestSpecCompletenessGate_StillFailsWithoutGWT(t *testing.T) {
	root := t.TempDir()
	// "whenever" and "thenceforth" must not satisfy the word-boundary match.
	writeSpecFile(t, root, "f", "spec.md", "## Acceptance Criteria\n- AC-1: Given X, whenever Y, thenceforth Z.\n")
	if res := specCompletenessGate.Handler(gateCtx(root, "f")); res.Verdict != VerdictFail {
		t.Fatalf("spec without a when/then clause must fail, got %v", res.Verdict)
	}
}

func TestQACoverageGate_MatchesACIdentifiers(t *testing.T) {
	root := t.TempDir()
	writeSpecFile(t, root, "f", "spec.md", "## Acceptance Criteria\n"+
		"- AC-1: Given a ready draft, when the arm is A, then SP01 renders.\n"+
		"- AC-2: Given a suppressed contact, when sending, then it is skipped.\n"+
		"- AC-10: Given no sender, when sending, then 422.\n")
	writeSpecFile(t, root, "f", "qa-report.md", "| AC | Evidence | Result |\n|---|---|---|\n"+
		"| AC-1 arm a ready draft | test x | PASS |\n| AC-2 suppressed | test y | PASS |\n| AC-10 no sender | test z | PASS |\n")
	if res := qaCoverageGate.Handler(gateCtx(root, "f")); res.Verdict != VerdictPass {
		t.Fatalf("qa-report referencing every AC id must pass: %s", res.Message)
	}
}

func TestQACoverageGate_ACIdIsNotAPrefixMatch(t *testing.T) {
	root := t.TempDir()
	writeSpecFile(t, root, "f", "spec.md", "## Acceptance Criteria\n- AC-1: Given a, when b, then c.\n- AC-2: Given d, when e, then f.\n")
	// AC-10 / AC-21 must not count as AC-1 / AC-2.
	writeSpecFile(t, root, "f", "qa-report.md", "| AC-10 | x | PASS |\n| AC-21 | y | PASS |\n")
	res := qaCoverageGate.Handler(gateCtx(root, "f"))
	if res.Verdict != VerdictFail {
		t.Fatalf("AC-10/AC-21 must not cover AC-1/AC-2, got %v", res.Verdict)
	}
}

func TestQACoverageGate_UnnumberedGivenLinesStillUsePrefix(t *testing.T) {
	root := t.TempDir()
	writeSpecFile(t, root, "f", "spec.md", "## Acceptance Criteria\nGiven a logged-in owner on the posts page\n")
	writeSpecFile(t, root, "f", "qa-report.md", "Covered: Given a logged-in owner on the posts page — PASS\n")
	if res := qaCoverageGate.Handler(gateCtx(root, "f")); res.Verdict != VerdictPass {
		t.Fatalf("verbatim unnumbered Given line must still count: %s", res.Message)
	}
}

func TestTaskCompletionGate_SkipsPostPushTasks(t *testing.T) {
	root := t.TempDir()
	writeSpecFile(t, root, "f", "tasks.md", "- [x] T1 migration\n- [x] T2 route\n- [ ] T3 [post-push] retest on staging\n")
	if res := taskCompletionGate.Handler(gateCtx(root, "f")); res.Verdict != VerdictPass {
		t.Fatalf("a [post-push] task cannot be done before push and must not block the code gate: %s", res.Message)
	}
}

func TestTaskCompletionGate_StillCountsOpenTasks(t *testing.T) {
	root := t.TempDir()
	writeSpecFile(t, root, "f", "tasks.md", "- [x] T1\n- [ ] T2 route\n- [ ] T3 [post-push] staging\n")
	res := taskCompletionGate.Handler(gateCtx(root, "f"))
	if res.Verdict != VerdictFail {
		t.Fatalf("an open pre-push task must still fail, got %v", res.Verdict)
	}
}

// ISSUE 18: tests cited by tasks.md / qa-report.md belong to the feature even
// when they do not use forge's scaffold filenames.
func TestFeatureTestFiles_CountsTestsCitedByTasks(t *testing.T) {
	root := t.TempDir()
	existing := filepath.Join(root, "tests", "unit", "api", "mcp-internal-post-crud.test.ts")
	other := filepath.Join(root, "tests", "unit", "unrelated.test.ts")
	for _, p := range []string{existing, other} {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("test('x',()=>{})"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeSpecFile(t, root, "mcp-unschedule-post", "tasks.md", "- [x] T1 cases in tests/unit/api/mcp-internal-post-crud.test.ts\n")
	got := featureTestFiles(root, "mcp-unschedule-post", []string{existing, other})
	if len(got) != 1 || got[0] != existing {
		t.Fatalf("want only the cited test, got %v", got)
	}
}

func TestWarningCheckpointNames(t *testing.T) {
	got := warningCheckpointNames([]Checkpoint{{Name: "Spec", Status: "warning"}, {Name: "Arch", Status: "ok"}, {Name: "Test", Status: "warning"}})
	if len(got) != 2 || got[0] != "Spec" || got[1] != "Test" {
		t.Fatalf("got %v", got)
	}
}
