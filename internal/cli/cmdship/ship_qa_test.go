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

// Tests for checkQAVerify (checkpoint 7).
//
// Test-design checklist (always-write-tests.md 9-point):
//  1. Happy path     — no MCP or go.mod in tempdir → Status="warning" (no test runner)
//  2. Happy path     — go.mod present (fallback path) → Status "ok" or "fail" (real go binary)
//  3. Happy path     — cmd/mcp/main.go present (Go MCP path) → Status "ok" or "fail"
//  4. Happy path     — mcp_server.py present (Python MCP path) → Status "ok" or "fail"
//  5. Boundary       — empty description → Name still "QA-Verify"
//  6. Idempotency    — call twice same dir → identical Status
//  7. Regression     — no blocking failure when no runner found (no panic)
//  8. Data-accuracy  — cp.Name is always "QA-Verify"
//  9. False-positive — "warning" status must NOT be "fail"
//
// 10. Remediation    — LLM pipe clears blocking gap; loop capped at maxRemediationRounds
package cmdship

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/teragrid/forge/internal/llmprovider"
)

// TestCheckQAVerify_Idempotent verifies two consecutive calls return identical status.
func TestCheckQAVerify_Idempotent(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	cp1 := checkQAVerify(root, "test feature", "", nil)
	cp2 := checkQAVerify(root, "test feature", "", nil)
	if cp1.Status != cp2.Status {
		t.Errorf("Idempotency: first=%q second=%q", cp1.Status, cp2.Status)
	}
}

// TestCheckQAVerify_NameIsAlwaysSet verifies the Checkpoint Name field is always populated.
func TestCheckQAVerify_NameIsAlwaysSet(t *testing.T) {
	t.Parallel()
	for _, desc := range []string{"", "  ", "some feature"} {
		cp := checkQAVerify(t.TempDir(), desc, "", nil)
		if cp.Name != "QA-Verify" {
			t.Errorf("desc=%q: Name: want %q, got %q", desc, "QA-Verify", cp.Name)
		}
	}
}

// TestCheckQAVerify_GoMCPEntry checks that presence of cmd/mcp/main.go triggers
// the Go MCP path gracefully (even if the mcpserver package does not exist).
func TestCheckQAVerify_GoMCPEntry(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	entryDir := filepath.Join(root, "cmd", "mcp")
	if err := os.MkdirAll(entryDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(entryDir, "main.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatal(err)
	}
	cp := checkQAVerify(root, "test feature", "", nil)
	if cp.Name != "QA-Verify" {
		t.Errorf("Name: want %q, got %q", "QA-Verify", cp.Name)
	}
	// On CI the go binary tries to run tests but ./internal/mcpserver doesn't exist → "fail".
	// Status must be one of: ok, fail, warning — never an empty string or panic.
	if cp.Status != "ok" && cp.Status != "fail" && cp.Status != "warning" {
		t.Errorf("unexpected Status %q: detail: %s", cp.Status, cp.Detail)
	}
}

// TestCheckQAVerify_PythonMCPEntry checks that presence of mcp_server.py triggers
// the Python MCP path without panicking.
func TestCheckQAVerify_PythonMCPEntry(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "mcp_server.py"), []byte("# mcp\n"), 0644); err != nil {
		t.Fatal(err)
	}
	cp := checkQAVerify(root, "test feature", "", nil)
	if cp.Name != "QA-Verify" {
		t.Errorf("Name: want %q, got %q", "QA-Verify", cp.Name)
	}
	if cp.Status != "ok" && cp.Status != "fail" && cp.Status != "warning" {
		t.Errorf("unexpected Status %q: detail: %s", cp.Status, cp.Detail)
	}
}

// TestCheckQAVerify_GoModFallback checks that go.mod (without an MCP server)
// triggers the go test ./... fallback path without panicking.
func TestCheckQAVerify_GoModFallback(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"),
		[]byte("module example.com/test\ngo 1.21\n"), 0644); err != nil {
		t.Fatal(err)
	}
	cp := checkQAVerify(root, "test feature", "", nil)
	if cp.Name != "QA-Verify" {
		t.Errorf("Name: want %q, got %q", "QA-Verify", cp.Name)
	}
	if cp.Status != "ok" && cp.Status != "fail" && cp.Status != "warning" {
		t.Errorf("unexpected Status %q: detail: %s", cp.Status, cp.Detail)
	}
}

// ── Spec-gap tests (TG-38/TG-39 wiring in checkpoint 7) ──────────────────────

// TestCheckQAVerify_GapAuditIsAlwaysSet verifies that GapAudit is always
// populated after checkQAVerify, regardless of whether a spec exists.
func TestCheckQAVerify_GapAuditIsAlwaysSet(t *testing.T) {
	t.Parallel()
	cp := checkQAVerify(t.TempDir(), "any feature", "", nil)
	if cp.GapAudit == nil {
		t.Error("GapAudit must never be nil after checkQAVerify")
	}
}

// TestCheckQAVerify_BlockingGapFailsCheckpoint verifies that an incomplete task
// in tasks.md causes checkQAVerify to return Status="fail" even when no test
// runner is present.
//
// This test FAILS on the pre-fix code and PASSES on the post-fix code — it
// serves as a regression guard for the spec-audit wiring in checkpoint 7.
func TestCheckQAVerify_BlockingGapFailsCheckpoint(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	slug := "test-feature"
	specDir := filepath.Join(root, ".forge", "specs", slug)
	if err := os.MkdirAll(specDir, 0755); err != nil {
		t.Fatal(err)
	}
	// spec.md presence triggers the audit engine.
	if err := os.WriteFile(filepath.Join(specDir, "spec.md"),
		[]byte("# Test Feature spec\n"), 0644); err != nil {
		t.Fatal(err)
	}
	// One unchecked task == one blocking gap.
	if err := os.WriteFile(filepath.Join(specDir, "tasks.md"),
		[]byte("# Tasks\n- [x] Done task\n- [ ] Incomplete task\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cp := checkQAVerify(root, "test feature", "", nil)

	if cp.GapAudit == nil {
		t.Fatal("GapAudit must be set when a spec directory exists")
	}
	if !cp.GapAudit.HasBlockingGaps() {
		t.Error("expected at least one blocking gap from incomplete tasks.md")
	}
	if cp.Status != "fail" {
		t.Errorf("expected Status=fail when blocking spec gaps exist (no LLM); got %q (detail: %s)",
			cp.Status, cp.Detail)
	}
}

// TestCheckQAVerify_WarningGapDoesNotFail verifies that a spec with no blocking
// gaps does NOT fail the checkpoint (false-positive guard).
func TestCheckQAVerify_WarningGapDoesNotFail(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	slug := "warn-feature"
	specDir := filepath.Join(root, ".forge", "specs", slug)
	if err := os.MkdirAll(specDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "spec.md"),
		[]byte("# Warn Feature spec\n"), 0644); err != nil {
		t.Fatal(err)
	}
	// All tasks complete — no blocking gaps.
	if err := os.WriteFile(filepath.Join(specDir, "tasks.md"),
		[]byte("# Tasks\n- [x] All tasks done\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cp := checkQAVerify(root, "warn feature", "", nil)

	if cp.GapAudit == nil {
		t.Fatal("GapAudit must be set when a spec directory exists")
	}
	if cp.GapAudit.HasBlockingGaps() {
		t.Error("expected no blocking gaps when all tasks are checked off")
	}
	if cp.Status == "fail" {
		t.Errorf("status must not be fail when no blocking gaps exist; got %q (detail: %s)",
			cp.Status, cp.Detail)
	}
}

// TestCheckQAVerify_NoSpecMeansNoBlockingGaps verifies that a project without a
// .forge/specs/ directory does not accumulate blocking gaps.
func TestCheckQAVerify_NoSpecMeansNoBlockingGaps(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// No .forge/specs/ — auditSpecVsCode returns an empty result.

	cp := checkQAVerify(root, "unspecced feature", "", nil)

	if cp.GapAudit == nil {
		t.Error("GapAudit must always be non-nil after checkQAVerify")
	}
	if cp.GapAudit != nil && cp.GapAudit.HasBlockingGaps() {
		t.Error("no blocking gaps expected when no spec directory exists")
	}
	if cp.Status == "fail" {
		t.Errorf("must not fail when no spec exists; got Status=%q Detail=%q",
			cp.Status, cp.Detail)
	}
}

// ── Remediation loop tests ────────────────────────────────────────────────────

// TestCheckQAVerify_RemediationClearsBlockingGap verifies that when a blocking
// gap exists and an LLM pipe is available, the remediation loop implements the
// missing tasks and clears the gap before returning — so the checkpoint does NOT
// fail even though it started with a blocking gap.
//
// Happy path 10 from the test-design checklist.
func TestCheckQAVerify_RemediationClearsBlockingGap(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	// Create a spec with one incomplete task (blocking gap).
	slug := "remediation-feature"
	specDir := filepath.Join(root, ".forge", "specs", slug)
	if err := os.MkdirAll(specDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "spec.md"),
		[]byte("# Remediation Feature\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "tasks.md"),
		[]byte("# Tasks\n- [ ] Implement handler\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Mock LLM returns a canned implementation plan string (any non-empty response).
	mock := &llmprovider.MockProvider{
		Response: &llmprovider.Response{Content: "## Implementation plan\n\n```go\nfunc Handler() {}\n```"},
	}
	pipe := mockPipe(root, mock)

	cp := checkQAVerify(root, "remediation feature", "", pipe)

	// After remediation, tasks.md should have all boxes checked.
	tasksData, err := os.ReadFile(filepath.Join(specDir, "tasks.md"))
	if err != nil {
		t.Fatalf("tasks.md disappeared: %v", err)
	}
	if string(tasksData) == "# Tasks\n- [ ] Implement handler\n" {
		t.Error("tasks.md was not updated by remediation — all tasks should be marked done")
	}

	// After remediation the gap is cleared — checkpoint must not fail.
	if cp.Status == "fail" {
		t.Errorf("checkpoint must not fail after successful remediation; got Status=%q Detail=%q",
			cp.Status, cp.Detail)
	}
	if cp.RemediationRounds < 1 {
		t.Errorf("expected at least 1 remediation round; got %d", cp.RemediationRounds)
	}
	if cp.GapAudit == nil {
		t.Error("GapAudit must be set")
	}
}

// TestCheckQAVerify_RemediationMaxRoundsReached verifies that when the LLM
// provider errors on every call (remediation never succeeds), the loop exits
// after maxRemediationRounds and the checkpoint fails with a meaningful message.
func TestCheckQAVerify_RemediationMaxRoundsReached(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	slug := "stuck-feature"
	specDir := filepath.Join(root, ".forge", "specs", slug)
	if err := os.MkdirAll(specDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "spec.md"),
		[]byte("# Stuck Feature\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "tasks.md"),
		[]byte("# Tasks\n- [ ] Task that will never be fixed\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Mock LLM always errors — remediation dispatches will fail, gap persists.
	mock := &llmprovider.MockProvider{Err: fmt.Errorf("provider unavailable")}
	pipe := mockPipe(root, mock)

	cp := checkQAVerify(root, "stuck feature", "", pipe)

	if cp.Status != "fail" {
		t.Errorf("expected fail when gap persists after max rounds; got Status=%q", cp.Status)
	}
	if cp.RemediationRounds != maxRemediationRounds {
		t.Errorf("expected RemediationRounds=%d; got %d", maxRemediationRounds, cp.RemediationRounds)
	}
	if cp.Detail == "" {
		t.Error("Detail must be non-empty when remediation fails")
	}
}

// TestRemediateGaps_NilPipe verifies that remediateGaps is a no-op when pipe
// is nil — no panics, returns 0 dispatched.
func TestRemediateGaps_NilPipe(t *testing.T) {
	t.Parallel()
	gaps := []AuditGap{
		{Type: "incomplete-tasks", Severity: "blocking", Description: "1 task", File: "/nonexistent/tasks.md"},
	}
	n := remediateGaps(t.TempDir(), "any feature", "", gaps, nil)
	if n != 0 {
		t.Errorf("nil pipe must return 0 dispatched; got %d", n)
	}
}

// TestRemediateIncompleteTasks_MarksTasksDone verifies that
// remediateIncompleteTasks rewrites tasks.md replacing "- [ ]" with "- [x]"
// when the LLM returns a valid response.
func TestRemediateIncompleteTasks_MarksTasksDone(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	slug := "mark-done-feature"
	specDir := filepath.Join(root, ".forge", "specs", slug)
	if err := os.MkdirAll(specDir, 0755); err != nil {
		t.Fatal(err)
	}
	tasksPath := filepath.Join(specDir, "tasks.md")
	if err := os.WriteFile(tasksPath,
		[]byte("- [x] Done\n- [ ] Pending A\n* [ ] Pending B\n"), 0644); err != nil {
		t.Fatal(err)
	}

	mock := &llmprovider.MockProvider{
		Response: &llmprovider.Response{Content: "# Code plan\n\nImplement everything."},
	}
	pipe := mockPipe(root, mock)

	gap := AuditGap{
		Type:     "incomplete-tasks",
		Severity: "blocking",
		File:     tasksPath,
	}
	if err := remediateIncompleteTasks(root, "mark done feature", "", gap, pipe, nil); err != nil {
		t.Fatalf("remediateIncompleteTasks returned error: %v", err)
	}

	updated, err := os.ReadFile(tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	content := string(updated)
	if content == "- [x] Done\n- [ ] Pending A\n* [ ] Pending B\n" {
		t.Error("tasks.md was not modified: unchecked tasks must be marked done")
	}
	// Positive check: no unchecked boxes remain.
	if contains(content, "- [ ]") || contains(content, "* [ ]") {
		t.Errorf("unchecked tasks remain in tasks.md after remediation:\n%s", content)
	}
}

// TestRemediateIncompleteTasks_HonorsSpecNameOverride — regression test: when
// gap.File is empty (the caller didn't resolve one), remediateIncompleteTasks
// used to fall back to slugify(description) unconditionally, ignoring the
// --name/-n override — so it could neither find the real tasks.md nor write
// code-plan.md to the directory the rest of the pipeline uses.
func TestRemediateIncompleteTasks_HonorsSpecNameOverride(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	description := "a very long feature description whose slugified form is " +
		"nothing like the custom name below"

	specDir := filepath.Join(root, ".forge", "specs", "custom-slug")
	if err := os.MkdirAll(specDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "tasks.md"), []byte("- [ ] Pending\n"), 0644); err != nil {
		t.Fatal(err)
	}

	mock := &llmprovider.MockProvider{
		Response: &llmprovider.Response{Content: "# Code plan\n\nImplement the pending task."},
	}
	pipe := mockPipe(root, mock)

	// gap.File deliberately empty — forces the tasksPath fallback that must
	// honor specName.
	gap := AuditGap{Type: "incomplete-tasks", Severity: "blocking"}
	if err := remediateIncompleteTasks(root, description, "custom-slug", gap, pipe, nil); err != nil {
		t.Fatalf("remediateIncompleteTasks returned error: %v", err)
	}

	updated, err := os.ReadFile(filepath.Join(specDir, "tasks.md"))
	if err != nil {
		t.Fatalf("tasks.md under the --name override directory was not touched: %v", err)
	}
	if contains(string(updated), "- [ ]") {
		t.Errorf("unchecked tasks remain after remediation:\n%s", string(updated))
	}
	if _, err := os.ReadFile(filepath.Join(specDir, "code-plan.md")); err != nil {
		t.Fatalf("code-plan.md not written under the --name override directory: %v", err)
	}
	wrongDir := filepath.Join(root, ".forge", "specs", slugify(description))
	if _, err := os.Stat(wrongDir); err == nil {
		t.Fatalf("remediation artefacts leaked into the description-derived slug directory %q — specName was not honored", wrongDir)
	}
}

// ── P1-L4 Shrinking-context tests ───────────────────────────────────────────

// TestRemediationState_Round1_UsesFullContext verifies that on round 1 the
// full spec + breakdown context path is taken (state.PriorAttempt == "").
func TestRemediationState_Round1_UsesFullContext(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	slug := "full-ctx-feature"
	specDir := filepath.Join(root, ".forge", "specs", slug)
	if err := os.MkdirAll(specDir, 0755); err != nil {
		t.Fatal(err)
	}
	tasksPath := filepath.Join(specDir, "tasks.md")
	if err := os.WriteFile(tasksPath, []byte("- [ ] Task one\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "spec.md"), []byte("# Spec\nDo the thing."), 0644); err != nil {
		t.Fatal(err)
	}

	var capturedUser string
	mock := &llmprovider.MockProvider{
		Fn: func(req *llmprovider.Request) (*llmprovider.Response, error) {
			capturedUser = req.UserPrompt
			return &llmprovider.Response{Content: "plan v1"}, nil
		},
	}
	pipe := mockPipe(root, mock)

	state := &RemediationState{GapItem: "Task one", Round: 1}
	gap := AuditGap{Type: "incomplete-tasks", Severity: "blocking", File: tasksPath}
	if err := remediateIncompleteTasks(root, "full ctx feature", "", gap, pipe, state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Round 1 → PriorAttempt now set to LLM response.
	if state.PriorAttempt != "plan v1" {
		t.Errorf("PriorAttempt: want %q, got %q", "plan v1", state.PriorAttempt)
	}
	// The user prompt must NOT contain "Prior attempt" on round 1.
	if contains(capturedUser, "Prior attempt") {
		t.Errorf("round 1 should not include 'Prior attempt' in prompt; got:\n%s", capturedUser)
	}
}

// TestRemediationState_Round2_UsesShrinkingContext verifies round 2+ sends
// only the gap + truncated prior attempt, not the full spec.
func TestRemediationState_Round2_UsesShrinkingContext(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	slug := "shrink-ctx-feature"
	specDir := filepath.Join(root, ".forge", "specs", slug)
	if err := os.MkdirAll(specDir, 0755); err != nil {
		t.Fatal(err)
	}
	tasksPath := filepath.Join(specDir, "tasks.md")
	if err := os.WriteFile(tasksPath, []byte("- [ ] Remaining task\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var capturedUser string
	mock := &llmprovider.MockProvider{
		Fn: func(req *llmprovider.Request) (*llmprovider.Response, error) {
			capturedUser = req.UserPrompt
			return &llmprovider.Response{Content: "plan v2"}, nil
		},
	}
	pipe := mockPipe(root, mock)

	state := &RemediationState{
		GapItem:      "Remaining task",
		PriorAttempt: "plan v1 from round 1",
		FailureNote:  "still unchecked",
		Round:        2,
	}
	gap := AuditGap{Type: "incomplete-tasks", Severity: "blocking", File: tasksPath}
	if err := remediateIncompleteTasks(root, "shrink ctx feature", "", gap, pipe, state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Round 2 → prompt must include "Prior attempt".
	if !contains(capturedUser, "Prior attempt") {
		t.Errorf("round 2 prompt must contain 'Prior attempt'; got:\n%s", capturedUser)
	}
	// The full spec.md content must NOT appear (shrinking context).
	if contains(capturedUser, "# Spec") {
		t.Errorf("round 2 prompt must not include full spec content; got:\n%s", capturedUser)
	}
	// State updated with new attempt.
	if state.PriorAttempt != "plan v2" {
		t.Errorf("PriorAttempt after round 2: want %q, got %q", "plan v2", state.PriorAttempt)
	}
}

// TestRemediateGapsRound_StateMapPropagated verifies that remediateGapsRound
// populates and threads the state map across calls (idempotency / replay).
func TestRemediateGapsRound_StateMapPropagated(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	slug := "statemap-feature"
	specDir := filepath.Join(root, ".forge", "specs", slug)
	if err := os.MkdirAll(specDir, 0755); err != nil {
		t.Fatal(err)
	}
	tasksPath := filepath.Join(specDir, "tasks.md")
	if err := os.WriteFile(tasksPath, []byte("- [ ] Task A\n"), 0644); err != nil {
		t.Fatal(err)
	}

	round1Response := "plan A round 1"
	mock := &llmprovider.MockProvider{
		Response: &llmprovider.Response{Content: round1Response},
	}
	pipe := mockPipe(root, mock)

	gaps := []AuditGap{{Type: "incomplete-tasks", Severity: "blocking", File: tasksPath, Description: "Task A"}}
	stateMap := make(map[string]*RemediationState)

	// Round 1 — state map is empty → creates state entry.
	remediateGapsRound(root, "statemap feature", "", gaps, pipe, 1, stateMap)
	if stateMap["Task A"] == nil {
		t.Fatal("stateMap must contain an entry for 'Task A' after round 1")
	}
	if stateMap["Task A"].PriorAttempt != round1Response {
		t.Errorf("PriorAttempt after round 1: want %q, got %q", round1Response, stateMap["Task A"].PriorAttempt)
	}
}

// TestTruncateTokens_LongString verifies truncation occurs at 4×maxTokens chars.
func TestTruncateTokens_LongString(t *testing.T) {
	t.Parallel()
	s := strings.Repeat("a", 1300)
	result := truncateTokens(s, 300) // 300×4 = 1200 chars
	if len(result) > 1220 {          // 1200 + len("[truncated]")
		t.Errorf("truncated string too long: %d chars", len(result))
	}
	if !contains(result, "[truncated]") {
		t.Error("truncated string must end with '[truncated]'")
	}
}

// TestTruncateTokens_ShortString verifies short strings are returned unchanged.
func TestTruncateTokens_ShortString(t *testing.T) {
	t.Parallel()
	s := "hello world"
	if got := truncateTokens(s, 300); got != s {
		t.Errorf("short string must be unchanged; got %q", got)
	}
}

// TestRemediateGaps_NilPipeRoundVariant is a regression guard: nil pipe with
// remediateGapsRound must be a no-op (returns 0, no panic).
func TestRemediateGaps_NilPipeRoundVariant(t *testing.T) {
	t.Parallel()
	gaps := []AuditGap{
		{Type: "incomplete-tasks", Severity: "blocking", Description: "task", File: "/nonexistent"},
	}
	n := remediateGapsRound(t.TempDir(), "feat", "", gaps, nil, 2, nil)
	if n != 0 {
		t.Errorf("nil pipe must return 0; got %d", n)
	}
}
