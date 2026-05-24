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

package cmdbugfix

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ── Source constants ──────────────────────────────────────────────────────────

func TestSourceConstants(t *testing.T) {
	t.Parallel()
	if SourceBug != "bug" {
		t.Errorf("SourceBug: got %q want %q", SourceBug, "bug")
	}
	if SourceFinding != "finding" {
		t.Errorf("SourceFinding: got %q want %q", SourceFinding, "finding")
	}
	if SourceTest != "test" {
		t.Errorf("SourceTest: got %q want %q", SourceTest, "test")
	}
}

// ── Run: no-LLM fallback ──────────────────────────────────────────────────────

// TestRun_Bug_NoLLM verifies that --bug succeeds without an LLM provider,
// returning a structured result with a helpful placeholder message.
func TestRun_Bug_NoLLM(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	result, err := Run(root, "dry-run", "login fails when email has a +", "", "")
	if err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}
	if result.Source != SourceBug {
		t.Errorf("Source: got %q want %q", result.Source, SourceBug)
	}
	if result.Input == "" {
		t.Error("Input must not be empty")
	}
	if result.Mode != "dry-run" {
		t.Errorf("Mode: got %q want %q", result.Mode, "dry-run")
	}
	if result.Root != root {
		t.Errorf("Root: got %q want %q", result.Root, root)
	}
	// Without an LLM, we expect the RootCause to explain the missing provider.
	if !strings.Contains(result.RootCause, "LLM provider not configured") &&
		!strings.Contains(result.RootCause, "LLM call failed") &&
		result.RootCause == "" {
		t.Errorf("unexpected empty RootCause without LLM")
	}
}

// TestRun_Test_NoLLM verifies that --test also succeeds without an LLM provider.
func TestRun_Test_NoLLM(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	result, err := Run(root, "dry-run", "", "", "TestLoginHandler_PlusSign")
	if err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}
	if result.Source != SourceTest {
		t.Errorf("Source: got %q want %q", result.Source, SourceTest)
	}
	if result.Input != "TestLoginHandler_PlusSign" {
		t.Errorf("Input: got %q want %q", result.Input, "TestLoginHandler_PlusSign")
	}
}

// ── Finding lookup ────────────────────────────────────────────────────────────

// TestRun_Finding_NotFound returns ErrFindingNotFound when the ID is absent.
func TestRun_Finding_NotFound(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	_, err := Run(root, "dry-run", "", "SEC-999", "")
	if err == nil {
		t.Fatal("expected error for missing finding, got nil")
	}
}

// TestRun_Finding_Found resolves a finding from review-results.json.
func TestRun_Finding_Found(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	// Write a minimal review-results.json.
	forgeDir := filepath.Join(root, ".forge")
	if err := os.MkdirAll(forgeDir, 0o750); err != nil {
		t.Fatal(err)
	}
	reviewData := `{
		"findings": [
			{"rule_id": "SEC-001", "file": "auth.go", "line": 42, "severity": "error", "message": "SQL injection risk"}
		]
	}`
	if err := os.WriteFile(filepath.Join(forgeDir, "review-results.json"), []byte(reviewData), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := Run(root, "dry-run", "", "SEC-001", "")
	if err != nil {
		t.Fatalf("Run returned unexpected error for existing finding: %v", err)
	}
	if result.Source != SourceFinding {
		t.Errorf("Source: got %q want %q", result.Source, SourceFinding)
	}
	if !strings.Contains(result.Input, "SQL injection risk") {
		t.Errorf("Input should contain finding message, got: %s", result.Input)
	}
}

// ── Boundary: no source specified ────────────────────────────────────────────

// TestNew_NoFlags_ReturnsError verifies that running without --bug/--finding/--test
// returns FORGE-6301 (no bug source specified).
func TestNew_NoFlags_ReturnsError(t *testing.T) {
	t.Parallel()
	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when no source flag is provided")
	}
}

// ── JSON output ───────────────────────────────────────────────────────────────

// TestNew_JSONOutput_Bug verifies that --json emits valid JSON with the correct
// Source and Mode fields when --bug is provided.
func TestNew_JSONOutput_Bug(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--root", root, "--bug", "button click does nothing", "--json"})
	_ = cmd.Execute()

	var result BugfixResult
	if err := json.NewDecoder(&out).Decode(&result); err != nil {
		t.Fatalf("--json must produce valid JSON: %v\noutput: %s", err, out.String())
	}
	if result.Source != SourceBug {
		t.Errorf("Source: got %q want %q", result.Source, SourceBug)
	}
	if result.Mode != "dry-run" {
		t.Errorf("Mode should default to dry-run, got %q", result.Mode)
	}
	if result.Root != root {
		t.Errorf("Root: got %q want %q", result.Root, root)
	}
}

// TestNew_JSONOutput_Test verifies JSON output when --test is provided.
func TestNew_JSONOutput_Test(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--root", root, "--test", "TestCheckout_NilCart", "--json"})
	_ = cmd.Execute()

	var result BugfixResult
	if err := json.NewDecoder(&out).Decode(&result); err != nil {
		t.Fatalf("--json must produce valid JSON: %v\noutput: %s", err, out.String())
	}
	if result.Source != SourceTest {
		t.Errorf("Source: got %q want %q", result.Source, SourceTest)
	}
}

// ── Idempotency ───────────────────────────────────────────────────────────────

// TestRun_Idempotent verifies that running forge bugfix twice for the same bug
// does not return an error on either call.
func TestRun_Idempotent(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	bugDesc := "panic on nil pointer in payment handler"
	r1, err := Run(root, "dry-run", bugDesc, "", "")
	if err != nil {
		t.Fatalf("first run: %v", err)
	}
	r2, err := Run(root, "dry-run", bugDesc, "", "")
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	// Both should agree on source and input.
	if r1.Source != r2.Source {
		t.Errorf("source mismatch: %q vs %q", r1.Source, r2.Source)
	}
	if r1.Input != r2.Input {
		t.Errorf("input mismatch: %q vs %q", r1.Input, r2.Input)
	}
}

// ── Apply mode: dry-run guard ─────────────────────────────────────────────────

// TestNew_ApplyFlag_Sets mode verifies that --apply changes the mode to "apply".
func TestNew_ApplyFlag_SetsMode(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--root", root, "--bug", "search returns wrong results", "--apply", "--json"})
	_ = cmd.Execute()

	var result BugfixResult
	if err := json.NewDecoder(&out).Decode(&result); err != nil {
		t.Fatalf("--json must produce valid JSON: %v\noutput: %s", err, out.String())
	}
	if result.Mode != "apply" {
		t.Errorf("Mode: got %q want %q", result.Mode, "apply")
	}
}

// ── Regression guard ─────────────────────────────────────────────────────────

// TestRun_Bug_NoBugTextNoInput is a false-positive guard: an empty bug string
// is treated as "no source" and must fail, not return a zero-value result.
func TestRun_Bug_NoBugTextNoInput(t *testing.T) {
	t.Parallel()
	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--bug", ""})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when --bug is empty string")
	}
}

// ── RunContext / new real-world flags ─────────────────────────────────────────

// TestRun_BackwardCompat verifies the 5-arg Run signature still compiles and
// works (no RunContext supplied). This is the regression guard for the variadic
// change.
func TestRun_BackwardCompat(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// Must compile and not panic; no LLM → placeholder result.
	result, err := Run(root, "dry-run", "crash on startup", "", "")
	if err != nil {
		t.Fatalf("5-arg Run: %v", err)
	}
	if result.Source != SourceBug {
		t.Errorf("Source: got %q want %q", result.Source, SourceBug)
	}
}

// TestRun_RunContext_Stack passes a RunContext with a stack trace and verifies
// the call does not error out (no LLM in test env).
func TestRun_RunContext_Stack(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	rc := RunContext{
		Stack: "goroutine 1 [running]:\nmain.main()\n\t/main.go:42 +0x80",
	}
	result, err := Run(root, "dry-run", "nil pointer dereference", "", "", rc)
	if err != nil {
		t.Fatalf("Run with stack: %v", err)
	}
	if result.Input != "nil pointer dereference" {
		t.Errorf("Input: got %q", result.Input)
	}
}

// TestRun_RunContext_Files includes a source file that doesn't exist — Run must
// not return an error; the missing file is surfaced gracefully in the LLM prompt.
func TestRun_RunContext_Files(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	rc := RunContext{
		Files: []string{filepath.Join(root, "nonexistent.go")},
	}
	result, err := Run(root, "dry-run", "timeout on checkout", "", "", rc)
	if err != nil {
		t.Fatalf("Run with missing file: %v", err)
	}
	if result.Source != SourceBug {
		t.Errorf("Source: got %q want %q", result.Source, SourceBug)
	}
}

// TestRun_RunContext_ExtraCtx passes free-form context; verifies no error.
func TestRun_RunContext_ExtraCtx(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	rc := RunContext{ExtraCtx: "This only happens on the EU cluster, not US."}
	result, err := Run(root, "dry-run", "payment gateway timeout", "", "", rc)
	if err != nil {
		t.Fatalf("Run with extra context: %v", err)
	}
	if result.Mode != "dry-run" {
		t.Errorf("Mode: got %q want dry-run", result.Mode)
	}
}

// TestNew_StackFlag verifies the --stack CLI flag is accepted.
func TestNew_StackFlag(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{
		"--root", root,
		"--bug", "nil pointer in auth",
		"--stack", "goroutine 1 [running]:\nmain()",
		"--json",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute with --stack: %v", err)
	}
	var result BugfixResult
	if err := json.NewDecoder(&out).Decode(&result); err != nil {
		t.Fatalf("JSON parse: %v\n%s", err, out.String())
	}
	if result.Source != SourceBug {
		t.Errorf("Source: got %q", result.Source)
	}
}

// TestNew_FileFlag verifies the --file CLI flag is accepted (repeatable).
func TestNew_FileFlag(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// Write a real file to include.
	srcFile := filepath.Join(root, "auth.go")
	if err := os.WriteFile(srcFile, []byte("package main\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{
		"--root", root,
		"--bug", "wrong auth redirect",
		"--file", srcFile,
		"--json",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute with --file: %v", err)
	}
}

// TestNew_ModelFlag verifies the --model CLI flag is accepted.
func TestNew_ModelFlag(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{
		"--root", root,
		"--bug", "crash on logout",
		"--model", "gpt-4o",
		"--json",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute with --model: %v", err)
	}
}

// ── applyPatch ────────────────────────────────────────────────────────────────

// TestApplyPatch_SavesPatchFile verifies that applyPatch saves the patch
// to .forge/patches/ even when git apply is unavailable.
func TestApplyPatch_SavesPatchFile(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	fp := &FixPatch{
		File:       "cmd/server.go",
		Patch:      "func fixMe() {}",
		Confidence: "high",
	}
	patchFile, _ := applyPatch(root, fp)
	if patchFile == "" {
		t.Fatal("applyPatch must return a patch file path")
	}
	if _, err := os.Stat(patchFile); err != nil {
		t.Errorf("patch file not created on disk: %v", err)
	}
}

// TestApplyPatch_Nil verifies applyPatch handles nil gracefully.
func TestApplyPatch_Nil(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	patchFile, err := applyPatch(root, nil)
	if err != nil {
		t.Errorf("applyPatch(nil): unexpected error: %v", err)
	}
	if patchFile != "" {
		t.Errorf("applyPatch(nil): expected empty patchFile, got %q", patchFile)
	}
}

// TestBugfixResult_PatchFileField verifies that BugfixResult has the PatchFile
// field and it round-trips through JSON correctly.
func TestBugfixResult_PatchFileField(t *testing.T) {
	t.Parallel()
	r := BugfixResult{PatchFile: "/tmp/x.patch", Applied: true}
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var r2 BugfixResult
	if err := json.Unmarshal(b, &r2); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if r2.PatchFile != "/tmp/x.patch" {
		t.Errorf("PatchFile: got %q want %q", r2.PatchFile, "/tmp/x.patch")
	}
}
