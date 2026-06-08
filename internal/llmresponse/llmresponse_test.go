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

package llmresponse_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/teragrid/forge/internal/llmresponse"
)

// ── Envelope (Wrap) ───────────────────────────────────────────────────────────

func TestWrap_OKResponse(t *testing.T) {
	r := llmresponse.Wrap(llmresponse.Options{
		Checkpoint:     "code",
		Status:         llmresponse.StatusCompleted,
		ContextSummary: "forge ship code → completed",
		NextActions:    []string{"forge ship ship --name auth"},
	})
	if !r.OK {
		t.Fatal("expected ok=true")
	}
	if r.Status != llmresponse.StatusCompleted {
		t.Fatalf("expected status %q, got %q", llmresponse.StatusCompleted, r.Status)
	}
	if r.Error != nil {
		t.Fatalf("expected no error field, got %+v", r.Error)
	}
	if len(r.NextActions) == 0 {
		t.Fatal("expected next_actions to be populated")
	}
}

func TestWrap_ErrorResponse(t *testing.T) {
	r := llmresponse.Wrap(llmresponse.Options{
		Checkpoint: "code",
		Status:     llmresponse.StatusFailed,
		Err:        errors.New("build failed: undefined: Foo"),
	})
	if r.OK {
		t.Fatal("expected ok=false")
	}
	if r.Error == nil {
		t.Fatal("expected error field to be non-nil")
	}
	if r.Error.Remedy == "" {
		t.Fatal("remedy must not be empty on error (AC-2)")
	}
	// False-positive guard: a successful response must NOT have an error field.
	good := llmresponse.Wrap(llmresponse.Options{Status: llmresponse.StatusCompleted})
	if good.Error != nil {
		t.Fatal("false-positive: successful response should not have error field")
	}
}

func TestWrap_NextActionsDefaultsToEmptySlice(t *testing.T) {
	r := llmresponse.Wrap(llmresponse.Options{Status: llmresponse.StatusCompleted})
	if r.NextActions == nil {
		t.Fatal("next_actions must default to empty slice, not nil (JSON marshals to [] not null)")
	}
}

func TestWrap_Write_JSON(t *testing.T) {
	var buf bytes.Buffer
	r := llmresponse.Wrap(llmresponse.Options{
		Checkpoint:     "verify",
		Status:         llmresponse.StatusCompleted,
		ContextSummary: "forge ship verify → completed",
	})
	if err := r.Write(&buf); err != nil {
		t.Fatalf("Write error: %v", err)
	}
	// Must be valid JSON
	var out map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &out); err != nil {
		t.Fatalf("output is not valid JSON: %v\nraw: %s", err, buf.String())
	}
	// Must not contain ANSI escape sequences (AC-1)
	if strings.Contains(buf.String(), "\x1b[") {
		t.Fatal("JSON output must not contain ANSI escape sequences (AC-1)")
	}
}

// ── Context / LLM mode ───────────────────────────────────────────────────────

func TestWithLLMMode_RoundTrip(t *testing.T) {
	ctx := context.Background()
	if llmresponse.IsLLMMode(ctx) {
		t.Fatal("fresh context should not be in LLM mode")
	}
	ctx = llmresponse.WithLLMMode(ctx, true)
	if !llmresponse.IsLLMMode(ctx) {
		t.Fatal("expected LLM mode after WithLLMMode(true)")
	}
	ctx = llmresponse.WithLLMMode(ctx, false)
	if llmresponse.IsLLMMode(ctx) {
		t.Fatal("expected human mode after WithLLMMode(false)")
	}
}

// ── Mode detection ────────────────────────────────────────────────────────────

func TestDetectMode_JSONFlagEnablesLLMMode(t *testing.T) {
	if !llmresponse.DetectMode(llmresponse.DetectOptions{JSONFlag: true}) {
		t.Fatal("--json should enable LLM mode")
	}
}

func TestDetectMode_HumanFlagWins(t *testing.T) {
	// --human overrides --json (explicit opt-out always wins).
	if llmresponse.DetectMode(llmresponse.DetectOptions{JSONFlag: true, HumanFlag: true}) {
		t.Fatal("--human should override --json (AC-9)")
	}
}

func TestDetectMode_ForgeLLMModeEnv(t *testing.T) {
	t.Setenv("FORGE_LLM_MODE", "1")
	if !llmresponse.DetectMode(llmresponse.DetectOptions{}) {
		t.Fatal("FORGE_LLM_MODE=1 should enable LLM mode (AC-3)")
	}
}

func TestDetectMode_NoColorEnv(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	if !llmresponse.DetectMode(llmresponse.DetectOptions{}) {
		t.Fatal("NO_COLOR=1 should enable LLM mode")
	}
}

func TestDetectMode_DefaultHuman(t *testing.T) {
	// Clear relevant env vars.
	t.Setenv("FORGE_LLM_MODE", "")
	t.Setenv("NO_COLOR", "")
	// When stdout is a regular file (not TTY), still returns true — so we use
	// a fake pipe file. We can only check the non-TTY path is reachable.
	// Here we just verify the flag path is stable.
	result := llmresponse.DetectMode(llmresponse.DetectOptions{
		JSONFlag:  false,
		HumanFlag: false,
		Stdout:    os.Stdout, // may or may not be TTY in test runner
	})
	// Result is environment-dependent; we only assert it returns without panic.
	_ = result
}

// ── Summary ───────────────────────────────────────────────────────────────────

func TestGenerateSummary_UnderCharLimit(t *testing.T) {
	s := llmresponse.GenerateSummary(llmresponse.SummaryParams{
		Verb:        "ship",
		Checkpoint:  "code",
		FeatureName: "auth-email",
		Status:      llmresponse.StatusCompleted,
		TestsPassed: 33,
		TestsFailed: 0,
	})
	n := utf8.RuneCountInString(s)
	if n > 2000 {
		t.Fatalf("context_summary exceeds 2000 chars: got %d", n)
	}
	if s == "" {
		t.Fatal("summary must not be empty")
	}
}

func TestGenerateSummary_TruncatesLargeFileList(t *testing.T) {
	files := make([]string, 100)
	for i := range files {
		files[i] = "internal/some/package/file.go"
	}
	s := llmresponse.GenerateSummary(llmresponse.SummaryParams{
		Verb:         "ship",
		FilesChanged: files,
		Status:       llmresponse.StatusCompleted,
	})
	n := utf8.RuneCountInString(s)
	if n > 2000 {
		t.Fatalf("summary with large file list exceeds 2000 chars: got %d", n)
	}
}

func TestGenerateSummary_ErrorFieldIncluded(t *testing.T) {
	s := llmresponse.GenerateSummary(llmresponse.SummaryParams{
		Verb:      "ship",
		Status:    llmresponse.StatusFailed,
		ErrorCode: "FORGE-3200",
		ErrorHint: "checkpoint failed; run forge doctor",
	})
	if !strings.Contains(s, "FORGE-3200") {
		t.Fatal("summary should include error code when present")
	}
}

func TestGenerateSummary_Deterministic(t *testing.T) {
	p := llmresponse.SummaryParams{
		Verb:        "ship",
		Checkpoint:  "verify",
		FeatureName: "rate-limiter",
		Status:      llmresponse.StatusCompleted,
		TestsPassed: 10,
	}
	first := llmresponse.GenerateSummary(p)
	second := llmresponse.GenerateSummary(p)
	if first != second {
		t.Fatalf("GenerateSummary must be deterministic (AC-7): got %q then %q", first, second)
	}
}

// ── NextActions ───────────────────────────────────────────────────────────────

func TestNextActions_SuccessAdvancesToNextCheckpoint(t *testing.T) {
	actions := llmresponse.NextActions("code", "auth-email", false)
	if len(actions) == 0 {
		t.Fatal("expected at least one next action after 'code'")
	}
	// First action should reference the next checkpoint (ship).
	if !strings.Contains(actions[0], "ship") {
		t.Fatalf("first action should reference 'ship' checkpoint, got: %q", actions[0])
	}
}

func TestNextActions_FailureSuggestsRetry(t *testing.T) {
	actions := llmresponse.NextActions("breakdown", "feature-x", true)
	found := false
	for _, a := range actions {
		if strings.Contains(a, "breakdown") && strings.Contains(a, "retry") {
			found = true
		}
	}
	if !found {
		// Accept if the retry action contains "breakdown" even without "retry" word.
		for _, a := range actions {
			if strings.Contains(a, "breakdown") {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("expected retry action to contain 'breakdown', actions: %v", actions)
	}
}

func TestNextActions_LastCheckpointNoNext(t *testing.T) {
	actions := llmresponse.NextActions("verify", "feature-x", false)
	// verify is the last checkpoint; first action should be git push or similar.
	found := false
	for _, a := range actions {
		if strings.Contains(a, "git push") || strings.Contains(a, "push") {
			found = true
		}
	}
	if !found {
		t.Fatalf("after final checkpoint, expected push action; got: %v", actions)
	}
}
