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
