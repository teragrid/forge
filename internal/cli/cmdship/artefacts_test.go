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
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/teragrid/forge/internal/errcode"
	"github.com/teragrid/forge/internal/llmprovider"
)

// ── G-005: spec.yml written alongside spec.md ────────────────────────────────

func TestCheckSpec_WritesSpecYML(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	cp := checkSpec(root, "add login", "", nil)
	if cp.Status != "ok" {
		t.Fatalf("expected status ok, got %q (detail: %s)", cp.Status, cp.Detail)
	}

	slug := slugify("add login")
	ymlPath := filepath.Join(root, ".forge", "specs", slug, "spec.yml")
	if _, err := os.Stat(ymlPath); err != nil {
		t.Errorf("spec.yml not written to %s: %v", ymlPath, err)
	}
}

// ── G-006: four named test artifacts written at Test checkpoint ───────────────

func TestCheckTest_FourArtefacts(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	slug := slugify("add login")
	paths := writeTestArtifacts(root, slug, "add login", "", nil)

	if _, err := os.Stat(paths.UnitTest); err != nil {
		t.Errorf("unit test artifact missing (%s): %v", paths.UnitTest, err)
	}
	if _, err := os.Stat(paths.IntegrationTest); err != nil {
		t.Errorf("integration test artifact missing (%s): %v", paths.IntegrationTest, err)
	}
	if _, err := os.Stat(paths.RLSTest); err != nil {
		t.Errorf("RLS test artifact missing (%s): %v", paths.RLSTest, err)
	}
	if _, err := os.Stat(paths.ScanBaseline); err != nil {
		t.Errorf("scan baseline artifact missing (%s): %v", paths.ScanBaseline, err)
	}
}

// ── G-007: breakdown checkpoint writes tasks.md ──────────────────────────────

func TestCheckBreakdown_TasksMD(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	// Pre-create the spec directory (generateBreakdown reads spec.md from there).
	slug := slugify("add login")
	specsDir := filepath.Join(root, ".forge", "specs", slug)
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Mock LLM returns numbered tasks so extractTasksMD can parse them.
	mock := &llmprovider.MockProvider{
		Response: mockResponse("1. Set up route handler\n2. Write database migration\n3. Add auth middleware\n"),
	}
	cp := checkBreakdown(root, "add login", mockPipe(root, mock))
	if cp.Status != "ok" {
		t.Fatalf("expected status ok, got %q (detail: %s)", cp.Status, cp.Detail)
	}

	tasksMDPath := filepath.Join(specsDir, "tasks.md")
	if _, err := os.Stat(tasksMDPath); err != nil {
		t.Errorf("tasks.md not written to %s: %v", tasksMDPath, err)
	}

	data, err := os.ReadFile(tasksMDPath)
	if err != nil {
		t.Fatalf("cannot read tasks.md: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "- [ ] T-") {
		t.Errorf("tasks.md does not contain checkbox tasks, got:\n%s", content)
	}
}

// ── G-008: ErrContextBudgetExceeded when bundle exceeds token budget ──────────

func TestContextBundle_OverBudget(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	slug := "budget-feature"
	specsDir := filepath.Join(root, ".forge", "specs", slug)
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Write a tasks.md with one unchecked task.
	tasksMD := "# Tasks: budget feature\n\n- [ ] T-001: Implement core functionality\n"
	if err := os.WriteFile(filepath.Join(specsDir, "tasks.md"), []byte(tasksMD), 0o600); err != nil {
		t.Fatal(err)
	}

	// tokenBudget=1 forces any non-trivial bundle to exceed the limit.
	err := writeTaskContextBundles(root, slug, 1)
	if err == nil {
		t.Fatal("expected ErrContextBudgetExceeded, got nil")
	}
	sentinel := errcode.New(ErrContextBudgetExceeded, "", nil)
	if !errors.Is(err, sentinel) {
		t.Errorf("expected errors.Is(err, ErrContextBudgetExceeded sentinel), got: %v", err)
	}
}

// ── G-009: AutoAdvance=true when all tasks are complete ──────────────────────

func TestAutoAdvanceToShip(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	feature := "test feature"
	slug := slugify(feature)
	specsDir := filepath.Join(root, ".forge", "specs", slug)
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// All tasks checked [x].
	tasksMD := "# Tasks: test feature\n\n- [x] T-001: Implement handler\n- [x] T-002: Write tests\n"
	if err := os.WriteFile(filepath.Join(specsDir, "tasks.md"), []byte(tasksMD), 0o600); err != nil {
		t.Fatal(err)
	}

	cp := checkCode(root, feature, nil)
	if !cp.AutoAdvance {
		t.Errorf("expected AutoAdvance=true when all tasks complete, got false (status=%s detail=%s)",
			cp.Status, cp.Detail)
	}
}

// ── G-010: project-local prompt overrides ────────────────────────────────────

func TestPromptTemplate_ProjectOverride(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	// Write a custom prompt override.
	promptDir := filepath.Join(root, ".forge", "prompts")
	if err := os.MkdirAll(promptDir, 0o755); err != nil {
		t.Fatal(err)
	}
	customContent := "You are a custom spec engineer.\n"
	if err := os.WriteFile(
		filepath.Join(promptDir, "ship-spec.prompt.md"),
		[]byte(customContent),
		0o600,
	); err != nil {
		t.Fatal(err)
	}

	got := loadProjectPrompt(root, "ship-spec")
	if got != customContent {
		t.Errorf("loadProjectPrompt returned %q, want %q", got, customContent)
	}
}

func TestPromptTemplate_MissingReturnsEmpty(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	got := loadProjectPrompt(root, "ship-spec")
	if got != "" {
		t.Errorf("expected empty string for missing prompt, got %q", got)
	}
}

// ── G-011: learning loop — appendFailure / loadRecentFailures ─────────────────

func TestLearningLoop_AppendAndRead(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	appendFailure(root, "spec", "my-feature", "acceptance criteria missing")
	appendFailure(root, "spec", "my-feature", "second failure detail")

	got := loadRecentFailures(root, "spec", 3)
	if got == "" {
		t.Fatal("loadRecentFailures returned empty string after appendFailure")
	}
	if !strings.Contains(got, "my-feature") {
		t.Errorf("expected output to mention feature name, got:\n%s", got)
	}
	if !strings.Contains(got, "second failure detail") {
		t.Errorf("expected output to contain most recent failure detail, got:\n%s", got)
	}
}

func TestLearningLoop_NoFileReturnsEmpty(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	got := loadRecentFailures(root, "spec", 3)
	if got != "" {
		t.Errorf("expected empty string when no failure file exists, got %q", got)
	}
}

func TestLearningLoop_RespectsLimit(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	for i := 0; i < 5; i++ {
		appendFailure(root, "test", "feature-x", "failure")
	}

	// Request only 2 recent failures.
	got := loadRecentFailures(root, "test", 2)
	// The header mentions "last 2".
	if !strings.Contains(got, "last 2") {
		t.Errorf("expected \"last 2\" in output, got:\n%s", got)
	}
}
