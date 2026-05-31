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
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/teragrid/forge/internal/llmprovider"
)

// ── Test helpers ──────────────────────────────────────────────────────────────

// mockProvider is a fake LLM provider for unit tests. It captures the request
// so tests can assert on what was sent to the LLM.
type mockProvider struct {
	capturedReq *llmprovider.Request
	resp        *llmprovider.Response
	err         error
}

func (m *mockProvider) Name() string { return "mock" }

func (m *mockProvider) Complete(_ context.Context, req *llmprovider.Request) (*llmprovider.Response, error) {
	m.capturedReq = req
	if m.err != nil {
		return nil, m.err
	}
	return m.resp, nil
}

func (m *mockProvider) Capabilities() llmprovider.Capabilities {
	return llmprovider.Capabilities{}
}

// goodLLMResponse returns a well-formed LLM JSON response for use in tests.
func goodLLMResponse() *llmprovider.Response {
	return &llmprovider.Response{Content: `{
		"root_cause": "nil pointer dereference in the payment handler",
		"fix": {"file": "payment.go", "patch": "- if p == nil {\n+ if p == nil { return }", "confidence": "high"},
		"regression_test": {"file": "payment_test.go", "code": "func TestPaymentNilGuard(t *testing.T) {}"},
		"summary": "added nil guard to payment handler"
	}`}
}

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

// TestRun_NoLLMProvider_ReturnsError verifies that Run returns a non-nil error
// when no LLM provider is configured. The partial result still has Source,
// Input, Mode, and Root set (they are resolved before the LLM call).
func TestRun_NoLLMProvider_ReturnsError(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// No testProvider set — llmprovider.Detect() will fail in test environments
	// that have no real API keys configured (the normal case).
	result, err := Run(root, "dry-run", "login fails when email has a +", "", "")
	if err == nil {
		// A real LLM is present in this environment; verify the happy path.
		if result.Source != SourceBug {
			t.Errorf("Source: got %q want %q", result.Source, SourceBug)
		}
		return
	}
	// No LLM — partial result still has Source, Input, Mode and Root set.
	if result.Source != SourceBug {
		t.Errorf("Source: got %q want %q", result.Source, SourceBug)
	}
	if result.Input != "login fails when email has a +" {
		t.Errorf("Input: got %q", result.Input)
	}
	if result.Mode != "dry-run" {
		t.Errorf("Mode: got %q want %q", result.Mode, "dry-run")
	}
	if result.Root != root {
		t.Errorf("Root: got %q want %q", result.Root, root)
	}
}

// TestRun_Test_NoLLMProvider verifies that Source and Input are populated even
// when Run fails with ErrNoLLMProvider (the partial result is always set before
// the LLM call).
func TestRun_Test_NoLLMProvider(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	result, _ := Run(root, "dry-run", "", "", "TestLoginHandler_PlusSign")
	// Source and Input are resolved before the LLM call.
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

	result, err := Run(root, "dry-run", "", "SEC-001", "", RunContext{
		testProvider: &mockProvider{resp: goodLLMResponse()},
	})
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
// Uses testProviderHook to inject a mock LLM so no real API calls are made.
func TestNew_JSONOutput_Bug(t *testing.T) {
	// Not t.Parallel() — uses package-level testProviderHook.
	testProviderHook = &mockProvider{resp: goodLLMResponse()}
	defer func() { testProviderHook = nil }()

	root := t.TempDir()
	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--root", root, "--bug", "button click does nothing", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

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
// Uses testProviderHook to inject a mock LLM so no real API calls are made.
func TestNew_JSONOutput_Test(t *testing.T) {
	// Not t.Parallel() — uses package-level testProviderHook.
	testProviderHook = &mockProvider{resp: goodLLMResponse()}
	defer func() { testProviderHook = nil }()

	root := t.TempDir()
	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--root", root, "--test", "TestCheckout_NilCart", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

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
	mock := &mockProvider{resp: goodLLMResponse()}
	rc := RunContext{testProvider: mock}
	r1, err := Run(root, "dry-run", bugDesc, "", "", rc)
	if err != nil {
		t.Fatalf("first run: %v", err)
	}
	r2, err := Run(root, "dry-run", bugDesc, "", "", rc)
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

// TestNew_ApplyFlag_SetsMode verifies that --apply changes the mode to "apply".
func TestNew_ApplyFlag_SetsMode(t *testing.T) {
	// Not t.Parallel() — uses package-level testProviderHook.
	testProviderHook = &mockProvider{resp: goodLLMResponse()}
	defer func() { testProviderHook = nil }()

	root := t.TempDir()
	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--root", root, "--bug", "search returns wrong results", "--apply", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute with --apply: %v", err)
	}

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
// that Source is set in the partial result even when Run fails with
// ErrNoLLMProvider (regression guard for the variadic-arg change).
func TestRun_BackwardCompat(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// 5-arg form: RunContext is optional. Source is resolved before the LLM
	// call, so result.Source is populated regardless of whether err is nil.
	result, _ := Run(root, "dry-run", "crash on startup", "", "")
	if result.Source != SourceBug {
		t.Errorf("Source: got %q want %q", result.Source, SourceBug)
	}
}

// TestRun_RunContext_Stack passes a RunContext with a stack trace and verifies
// the stack trace is included in the LLM prompt.
func TestRun_RunContext_Stack(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mock := &mockProvider{resp: goodLLMResponse()}
	rc := RunContext{
		Stack:        "goroutine 1 [running]:\nmain.main()\n\t/main.go:42 +0x80",
		testProvider: mock,
	}
	result, err := Run(root, "dry-run", "nil pointer dereference", "", "", rc)
	if err != nil {
		t.Fatalf("Run with stack: %v", err)
	}
	if result.Input != "nil pointer dereference" {
		t.Errorf("Input: got %q", result.Input)
	}
	// The stack trace must be included in the LLM prompt.
	if mock.capturedReq == nil {
		t.Fatal("LLM was not called")
	}
	if !strings.Contains(mock.capturedReq.UserPrompt, "goroutine 1 [running]") {
		t.Errorf("stack trace not found in LLM prompt:\n%s", mock.capturedReq.UserPrompt)
	}
}

// TestRun_RunContext_Files includes a source file that doesn't exist — Run must
// not return an error; the missing file is surfaced gracefully in the LLM prompt.
func TestRun_RunContext_Files(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mock := &mockProvider{resp: goodLLMResponse()}
	rc := RunContext{
		Files:        []string{filepath.Join(root, "nonexistent.go")},
		testProvider: mock,
	}
	result, err := Run(root, "dry-run", "timeout on checkout", "", "", rc)
	if err != nil {
		t.Fatalf("Run with missing file: %v", err)
	}
	if result.Source != SourceBug {
		t.Errorf("Source: got %q want %q", result.Source, SourceBug)
	}
	// The (missing) file must still be mentioned in the LLM prompt (graceful fallback).
	if mock.capturedReq != nil && !strings.Contains(mock.capturedReq.UserPrompt, "nonexistent.go") {
		t.Errorf("missing file not mentioned in LLM prompt:\n%s", mock.capturedReq.UserPrompt)
	}
}

// TestRun_RunContext_ExtraCtx passes free-form context; verifies no error and
// that the extra context is included in the LLM prompt.
func TestRun_RunContext_ExtraCtx(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mock := &mockProvider{resp: goodLLMResponse()}
	rc := RunContext{
		ExtraCtx:     "This only happens on the EU cluster, not US.",
		testProvider: mock,
	}
	result, err := Run(root, "dry-run", "payment gateway timeout", "", "", rc)
	if err != nil {
		t.Fatalf("Run with extra context: %v", err)
	}
	if result.Mode != "dry-run" {
		t.Errorf("Mode: got %q want dry-run", result.Mode)
	}
	// Extra context must appear in the LLM prompt.
	if mock.capturedReq != nil && !strings.Contains(mock.capturedReq.UserPrompt, "EU cluster") {
		t.Errorf("extra context not found in LLM prompt:\n%s", mock.capturedReq.UserPrompt)
	}
}

// TestNew_StackFlag verifies the --stack CLI flag is accepted and forwarded
// to the LLM request.
func TestNew_StackFlag(t *testing.T) {
	// Not t.Parallel() — uses package-level testProviderHook.
	mock := &mockProvider{resp: goodLLMResponse()}
	testProviderHook = mock
	defer func() { testProviderHook = nil }()

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
	// Verify the stack trace was forwarded to the LLM.
	if mock.capturedReq == nil {
		t.Fatal("LLM was not called")
	}
	if !strings.Contains(mock.capturedReq.UserPrompt, "goroutine 1 [running]") {
		t.Errorf("--stack value not found in LLM prompt")
	}
}

// TestNew_FileFlag verifies the --file CLI flag is accepted and the file
// content is included in the LLM prompt.
func TestNew_FileFlag(t *testing.T) {
	// Not t.Parallel() — uses package-level testProviderHook.
	mock := &mockProvider{resp: goodLLMResponse()}
	testProviderHook = mock
	defer func() { testProviderHook = nil }()

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
	// Verify the file content was forwarded to the LLM.
	if mock.capturedReq == nil {
		t.Fatal("LLM was not called")
	}
	if !strings.Contains(mock.capturedReq.UserPrompt, "auth.go") {
		t.Errorf("--file path not found in LLM prompt")
	}
}

// TestNew_ModelFlag verifies the --model CLI flag is accepted and forwarded
// to the LLM request.
func TestNew_ModelFlag(t *testing.T) {
	// Not t.Parallel() — uses package-level testProviderHook.
	mock := &mockProvider{resp: goodLLMResponse()}
	testProviderHook = mock
	defer func() { testProviderHook = nil }()

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
	// Verify the model override was forwarded to the LLM request.
	if mock.capturedReq == nil {
		t.Fatal("LLM was not called")
	}
	if mock.capturedReq.Model != "gpt-4o" {
		t.Errorf("Model in LLM request: got %q want %q", mock.capturedReq.Model, "gpt-4o")
	}
}

// ── Regression tests for issue #18 ───────────────────────────────────────────

// TestRun_LLMCallFailed_ExitsNonZero is the primary regression guard for
// issue #18. Before the fix, a failing provider.Complete returned (result, nil),
// giving forge bugfix a silent exit-0 and bypassing all quality gates.
// The fix must return a non-nil error on ANY LLM failure.
func TestRun_LLMCallFailed_ExitsNonZero(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mock := &mockProvider{err: fmt.Errorf("429 Too Many Requests — retry after 30s")}
	_, err := Run(root, "dry-run", "login panic", "", "", RunContext{testProvider: mock})
	if err == nil {
		t.Fatal("LLM call failure must return a non-nil error (regression guard for issue #18): " +
			"before the fix, Run returned (result, nil) silently, causing exit-0 and bypassed quality gates")
	}
}

// TestRun_LLMResponse_ValidJSON_HappyPath verifies the happy path: when the LLM
// returns valid JSON, Run succeeds and the result contains RootCause, Fix, Summary.
func TestRun_LLMResponse_ValidJSON_HappyPath(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mock := &mockProvider{resp: goodLLMResponse()}
	result, err := Run(root, "dry-run", "nil pointer dereference in payment handler", "", "",
		RunContext{testProvider: mock})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.RootCause == "" {
		t.Error("RootCause must be populated on success")
	}
	if result.Fix == nil {
		t.Error("Fix must be populated on success")
	}
	if result.Summary == "" {
		t.Error("Summary must be populated on success")
	}
}

// TestRun_LLMResponse_InvalidJSON_ExitsNonZero verifies that a non-JSON LLM
// response returns a non-nil error (boundary: corrupted or non-JSON LLM output).
func TestRun_LLMResponse_InvalidJSON_ExitsNonZero(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mock := &mockProvider{resp: &llmprovider.Response{
		Content: "I cannot help with that request.",
	}}
	_, err := Run(root, "dry-run", "login panic", "", "", RunContext{testProvider: mock})
	if err == nil {
		t.Fatal("non-JSON LLM response must return a non-nil error")
	}
}

// TestRun_LLMCallFailed_FalsePositiveGuard is a false-positive guard: when the
// LLM succeeds with valid JSON, Run must NOT return an error.
func TestRun_LLMCallFailed_FalsePositiveGuard(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mock := &mockProvider{resp: goodLLMResponse()}
	_, err := Run(root, "dry-run", "button does nothing", "", "", RunContext{testProvider: mock})
	if err != nil {
		t.Errorf("valid LLM response must not produce an error, got: %v", err)
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
