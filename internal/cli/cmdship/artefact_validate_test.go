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

// Test design for J8/J9 (fix-checkpoint-llm-quality-and-observability):
// stripPreamble/looksComplete/generateWithValidation guard the two confirmed
// incidents — a raw conversational preamble written verbatim as the first
// line of a generated artefact, and a response truncated mid-section with no
// error surfaced.

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStripPreamble_RemovesConversationalLeadIn(t *testing.T) {
	t.Parallel()
	raw := "I'll review this feature specification and provide comprehensive improvements.\n\n# Spec: my feature\n\n## What\nDoes a thing.\n"
	got := stripPreamble(raw)
	want := "# Spec: my feature\n\n## What\nDoes a thing.\n"
	if got != want {
		t.Errorf("stripPreamble = %q, want %q", got, want)
	}
}

func TestStripPreamble_NoPreamble_Unchanged(t *testing.T) {
	t.Parallel()
	raw := "# Spec: my feature\n\n## What\nDoes a thing.\n"
	if got := stripPreamble(raw); got != raw {
		t.Errorf("stripPreamble should not alter content already starting with a heading; got %q", got)
	}
}

func TestLooksComplete_UnbalancedCodeFence_Truncated(t *testing.T) {
	t.Parallel()
	// Regression: cut off mid-Gherkin-block inside an unclosed fence.
	truncated := "# Spec\n\n```gherkin\nGiven a user\nWhen they submit"
	if looksComplete(truncated) {
		t.Error("content with an odd number of ``` fences should be flagged incomplete")
	}
}

func TestLooksComplete_EndsOnHeading_Complete(t *testing.T) {
	t.Parallel()
	if !looksComplete("# Spec\n\n## Out of Scope") {
		t.Error("content ending on a heading should be treated as complete")
	}
}

// TestLooksComplete_EndsOnListItem_NotTruncated is the regression this spec's
// implementation surfaced: a Markdown document legitimately ending on a
// bullet list item (e.g. "- happy path") was previously misclassified as
// truncated, because only terminal punctuation/headings/closing fences were
// accepted — a normal document ending on a list item is not a truncation
// signal.
func TestLooksComplete_EndsOnListItem_NotTruncated(t *testing.T) {
	t.Parallel()
	cases := []string{
		"# Generated from YAML\n## Acceptance Criteria\n- happy path",
		"# Spec\n\n## Out of Scope\n* nothing else for v1",
		"# Spec\n\n## Tasks\n1. write the migration",
		"# Spec\n\n## Tasks\n2) implement the RPC",
	}
	for _, c := range cases {
		if !looksComplete(c) {
			t.Errorf("content ending on a complete list item should not be flagged truncated: %q", c)
		}
	}
}

// Root cause 2026-07-23 (dogfooding on ai-marketing-platfrom, forge ship
// breakdown): a numbered list item cut off mid-word inside an unclosed
// inline code span — "2. Return: `{platform, external_account_id, oauth_"
// — was accepted as complete because isListItem only checks the line's
// prefix, not whether an inline code span opened on that line was ever
// closed. breakdown.md and tasks.md were both silently truncated as a
// result, with the checkpoint reporting success.
func TestLooksComplete_ListItemWithUnclosedInlineCode_Truncated(t *testing.T) {
	t.Parallel()
	cases := []string{
		"# Breakdown\n\n2. Return: `{platform, external_account_id, oauth_",
		"# Breakdown\n\n- See `src/lib/foo",
	}
	for _, c := range cases {
		if looksComplete(c) {
			t.Errorf("list item with an unclosed inline code span should be flagged truncated: %q", c)
		}
	}
}

func TestLooksComplete_EndsMidSentence_Truncated(t *testing.T) {
	t.Parallel()
	// No terminal punctuation, not a heading, not a list item, not a closing fence.
	if looksComplete("# Spec\n\nThe acceptance criteria for this feature are as follows and") {
		t.Error("content ending mid-sentence with no terminal shape should be flagged incomplete")
	}
}

func TestLooksComplete_Empty_Incomplete(t *testing.T) {
	t.Parallel()
	if looksComplete("") || looksComplete("   \n\n  ") {
		t.Error("empty/whitespace-only content should never be treated as complete")
	}
}

func TestGenerateWithValidation_RetriesOnceOnIncomplete(t *testing.T) {
	t.Parallel()
	calls := 0
	invoke := func() (string, bool, error) {
		calls++
		if calls == 1 {
			return "# Spec\n\nThe rest was cut off and", false, nil // incomplete (heuristic)
		}
		return "# Spec\n\n## Acceptance Criteria\n- happy path", false, nil // complete on retry
	}
	content, complete, err := generateWithValidation(invoke)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !complete {
		t.Error("expected complete=true after a successful retry")
	}
	if calls != 2 {
		t.Errorf("expected exactly one retry (2 calls total), got %d", calls)
	}
	if content == "" {
		t.Error("expected non-empty content from the retry")
	}
}

func TestGenerateWithValidation_LLMError_ReturnsErrImmediately(t *testing.T) {
	t.Parallel()
	calls := 0
	boom := errors.New("boom")
	invoke := func() (string, bool, error) {
		calls++
		return "", false, boom
	}
	_, complete, err := generateWithValidation(invoke)
	if err == nil {
		t.Fatal("expected the LLM error to propagate")
	}
	if complete {
		t.Error("complete should be false on error")
	}
	if calls != 1 {
		t.Errorf("an LLM/transport error should not be retried; got %d calls", calls)
	}
}

// TestGenerateWithValidation_APITruncatedListItem_OverridesHeuristic is the
// regression test for the confirmed production incident (2026-07-17
// dogfooding on the Copilot provider): a generated spec.md was cut off
// mid-word inside a list item —
// "- **Cost awareness**: ...verification should be run minimally (e.g., 1-2
// runs for posit" — and looksComplete's list-item branch waved it through as
// a normal short list item (its documented, accepted blind spot: it cannot
// tell "- happy path" from a genuinely truncated item without a dictionary).
// The checkpoint still reported success and wrote the broken file.
//
// The fix: when the provider's own stop/finish reason says the completion
// was cut off by MaxTokens, that signal is authoritative and must force
// complete=false even when looksComplete's text-shape heuristic alone would
// have accepted it.
func TestGenerateWithValidation_APITruncatedListItem_OverridesHeuristic(t *testing.T) {
	t.Parallel()
	calls := 0
	truncatedListItem := "# Spec\n\n## Non-Functional Requirements\n" +
		"- **Cost awareness**: Real LLM calls incur usage cost/quota against " +
		"the Copilot provider; verification should be run minimally (e.g., 1-2 runs for posit"
	invoke := func() (string, bool, error) {
		calls++
		// Every attempt returns the same truncated-mid-word list item with
		// apiTruncated=true, simulating a provider that keeps hitting MaxTokens.
		return truncatedListItem, true, nil
	}
	if looksComplete(truncatedListItem) != true {
		t.Fatal("test setup invariant broken: this exact string must be the " +
			"documented looksComplete false-negative (starts with '- ', heuristic alone accepts it) " +
			"for this test to actually exercise the apiTruncated override")
	}
	_, complete, err := generateWithValidation(invoke)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if complete {
		t.Error("apiTruncated=true must force complete=false even though the " +
			"text-shape heuristic alone would accept this content")
	}
	if calls != 2 {
		t.Errorf("expected exactly one retry (2 calls total), got %d", calls)
	}
}

// findUnverifiedFileReferences / appendUnverifiedPathsWarning — root cause:
// LLM-generated spec/arch content invented plausible-looking but nonexistent
// file paths 3 separate times in one session (e.g. a real
// src/app/api/integrations/facebook/callback/route.ts rendered as
// src/app/api/auth/facebook/callback/route.ts) and was trusted as fact.
//
// Test Design:
//  1. Happy path — a path that exists on disk is not flagged
//  2. Negative — a path that does not exist is flagged
//  3. Boundary — content with no backtick-wrapped paths at all → no findings
//  4. False-positive guard — prose text that merely contains "/" (not
//     backtick-wrapped) is never flagged
//  5. False-positive guard — a `.forge/...` path (an artefact this or a
//     sibling checkpoint is about to create) is never flagged
//  6. Idempotency — calling twice on the same content returns the same result
//  7. Data-accuracy — appendUnverifiedPathsWarning lists exactly the
//     unverified paths, deduplicated, and leaves verified content untouched
//     when nothing is unverified

func TestFindUnverifiedFileReferences_ExistingPathNotFlagged(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mustMkdirAll(t, filepath.Join(root, "src", "lib"))
	mustWriteFile(t, filepath.Join(root, "src", "lib", "real.go"), "package lib\n")

	got := findUnverifiedFileReferences(root, "See `src/lib/real.go` for details.")
	if len(got) != 0 {
		t.Errorf("expected no unverified paths, got %v", got)
	}
}

func TestFindUnverifiedFileReferences_NonexistentPathFlagged(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	got := findUnverifiedFileReferences(root, "Edit `src/app/api/auth/facebook/callback/route.ts` to fix this.")
	if len(got) != 1 || got[0] != "src/app/api/auth/facebook/callback/route.ts" {
		t.Errorf("expected exactly the nonexistent path flagged, got %v", got)
	}
}

func TestFindUnverifiedFileReferences_NoPathsInContent(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	got := findUnverifiedFileReferences(root, "# Spec\n\nJust prose, no code references at all.\n")
	if len(got) != 0 {
		t.Errorf("expected no findings for path-free content, got %v", got)
	}
}

func TestFindUnverifiedFileReferences_ProseSlashesNotFlagged(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	got := findUnverifiedFileReferences(root, "This is a fix/enhancement for read/write access, not a file path.")
	if len(got) != 0 {
		t.Errorf("prose containing '/' outside backticks must never be flagged, got %v", got)
	}
}

func TestFindUnverifiedFileReferences_ForgeSpecsPathNeverFlagged(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	got := findUnverifiedFileReferences(root, "Written to `.forge/specs/my-feature/spec.md` by this checkpoint.")
	if len(got) != 0 {
		t.Errorf(".forge/ artefact paths must never be flagged, got %v", got)
	}
}

func TestFindUnverifiedFileReferences_Idempotent(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	content := "See `src/does/not/exist.ts` and `src/also/missing.go`."

	r1 := findUnverifiedFileReferences(root, content)
	r2 := findUnverifiedFileReferences(root, content)
	if len(r1) != len(r2) || (len(r1) > 0 && r1[0] != r2[0]) {
		t.Errorf("expected identical results across calls, got %v then %v", r1, r2)
	}
}

func TestAppendUnverifiedPathsWarning_ListsDedupedUnverifiedPaths(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	content := "Touches `src/missing/a.ts` and again `src/missing/a.ts`, plus `src/missing/b.ts`."

	out := appendUnverifiedPathsWarning(root, content)
	if !strings.Contains(out, content) {
		t.Error("original content must be preserved")
	}
	// content mentions src/missing/a.ts twice already (2 occurrences); the
	// warning list must add exactly one more (deduplicated), not two.
	if strings.Count(out, "src/missing/a.ts") != 3 {
		t.Errorf("expected the warning list to add exactly 1 deduplicated occurrence (2 in body + 1 in list = 3 total): %s", out)
	}
	if strings.Count(out, "> - `src/missing/a.ts`") != 1 {
		t.Errorf("expected exactly one bullet line for the deduplicated path in the warning list: %s", out)
	}
	if !strings.Contains(out, "src/missing/b.ts") {
		t.Error("expected the second unverified path in the warning list")
	}
	if !strings.Contains(out, "Unverified file references") {
		t.Error("expected the warning callout heading")
	}
}

func TestAppendUnverifiedPathsWarning_NoOpWhenAllPathsVerified(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mustMkdirAll(t, filepath.Join(root, "src"))
	mustWriteFile(t, filepath.Join(root, "src", "real.go"), "package src\n")
	content := "See `src/real.go`."

	out := appendUnverifiedPathsWarning(root, content)
	if out != content {
		t.Errorf("content with only verified paths must be returned unchanged, got: %s", out)
	}
}

func mustMkdirAll(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustWriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
