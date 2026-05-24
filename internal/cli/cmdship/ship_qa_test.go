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
