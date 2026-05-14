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

package cmdfixtures

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TC-FIXTURES-01: dry-run produces expected stdout without writing files.
func TestNew_DryRun(t *testing.T) {
	cmd := New()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"user"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "dry-run") {
		t.Errorf("expected dry-run indicator in output, got: %q", out)
	}
	if !strings.Contains(out, "user.json") {
		t.Errorf("expected fixture path in output, got: %q", out)
	}
}

// TC-FIXTURES-02: --apply writes the fixture file to disk.
func TestNew_Apply(t *testing.T) {
	dir := t.TempDir()
	cmd := New()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"order", "--apply", "--output-dir", dir})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := filepath.Join(dir, "order.json")
	data, err := os.ReadFile(expected)
	if err != nil {
		t.Fatalf("fixture file not created: %v", err)
	}
	if !strings.Contains(string(data), `"_fixture"`) {
		t.Errorf("fixture file missing _fixture key: %s", string(data))
	}
}

// TC-FIXTURES-03: --apply on existing file skips without error.
func TestNew_Apply_Idempotent(t *testing.T) {
	dir := t.TempDir()
	existing := filepath.Join(dir, "product.json")
	if err := os.WriteFile(existing, []byte(`{"existing":true}`), 0o600); err != nil {
		t.Fatal(err)
	}

	cmd := New()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"product", "--apply", "--output-dir", dir})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Original file should be unchanged.
	data, _ := os.ReadFile(existing)
	if !strings.Contains(string(data), `"existing":true`) {
		t.Errorf("existing file was overwritten: %s", string(data))
	}
}

// TC-FIXTURES-04: --json emits valid JSON with correct fields.
func TestNew_JSONOutput(t *testing.T) {
	dir := t.TempDir()
	cmd := New()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"invoice", "--apply", "--output-dir", dir, "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result FixtureResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON output: %v\noutput: %s", err, buf.String())
	}
	if result.Name != "invoice" {
		t.Errorf("want name=invoice, got %q", result.Name)
	}
	if result.Mode != "apply" {
		t.Errorf("want mode=apply, got %q", result.Mode)
	}
	if !result.Created {
		t.Errorf("want created=true")
	}
}

// TC-FIXTURES-05: missing argument returns an error.
func TestNew_MissingArg(t *testing.T) {
	cmd := New()
	cmd.SetArgs([]string{})
	// Silence usage output in test.
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error for missing argument, got nil")
	}
}
