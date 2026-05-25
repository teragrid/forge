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
package cmdship

import (
	"os"
	"path/filepath"
	"testing"
)

// TestCheckQAVerify_Idempotent verifies two consecutive calls return identical status.
func TestCheckQAVerify_Idempotent(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	cp1 := checkQAVerify(root, "test feature")
	cp2 := checkQAVerify(root, "test feature")
	if cp1.Status != cp2.Status {
		t.Errorf("Idempotency: first=%q second=%q", cp1.Status, cp2.Status)
	}
}

// TestCheckQAVerify_NameIsAlwaysSet verifies the Checkpoint Name field is always populated.
func TestCheckQAVerify_NameIsAlwaysSet(t *testing.T) {
	t.Parallel()
	for _, desc := range []string{"", "  ", "some feature"} {
		cp := checkQAVerify(t.TempDir(), desc)
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
	cp := checkQAVerify(root, "test feature")
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
	cp := checkQAVerify(root, "test feature")
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
	cp := checkQAVerify(root, "test feature")
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
	cp := checkQAVerify(t.TempDir(), "any feature")
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

	cp := checkQAVerify(root, "test feature")

	if cp.GapAudit == nil {
		t.Fatal("GapAudit must be set when a spec directory exists")
	}
	if !cp.GapAudit.HasBlockingGaps() {
		t.Error("expected at least one blocking gap from incomplete tasks.md")
	}
	if cp.Status != "fail" {
		t.Errorf("expected Status=fail when blocking spec gaps exist; got %q (detail: %s)",
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

	cp := checkQAVerify(root, "warn feature")

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

	cp := checkQAVerify(root, "unspecced feature")

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
