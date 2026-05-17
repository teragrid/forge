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
package cmdfix

import (
	"bytes"
	"encoding/json"
	"testing"
)

// TestConfidenceConstants verifies the exported confidence tier constants
// (DEV-M1-16/17).
func TestConfidenceConstants(t *testing.T) {
	t.Parallel()
	if ConfidenceHigh != "high" {
		t.Errorf("ConfidenceHigh: got %q want %q", ConfidenceHigh, "high")
	}
	if ConfidenceMedium != "medium" {
		t.Errorf("ConfidenceMedium: got %q want %q", ConfidenceMedium, "medium")
	}
	if ConfidenceLow != "low" {
		t.Errorf("ConfidenceLow: got %q want %q", ConfidenceLow, "low")
	}
}

// TestRun_DryRun_EmptyRoot verifies that dry-run on an empty root returns a
// result with Mode="dry-run", Applied=0, and no errors.
func TestRun_DryRun_EmptyRoot(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	result := Run(root, "all", "dry-run", false)
	if result.Mode != "dry-run" {
		t.Errorf("Mode: got %q want %q", result.Mode, "dry-run")
	}
	if result.Root != root {
		t.Errorf("Root: got %q want %q", result.Root, root)
	}
	if result.Applied != 0 {
		t.Errorf("Applied must be 0 in dry-run mode, got %d", result.Applied)
	}
}

// TestRun_Apply_EmptyRoot verifies that apply mode on an empty root returns
// Mode="apply" and does not error.
func TestRun_Apply_EmptyRoot(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	result := Run(root, "all", "apply", false)
	if result.Mode != "apply" {
		t.Errorf("Mode: got %q want %q", result.Mode, "apply")
	}
}

// TestRun_Apply_DoesNotApplyLow verifies that low-confidence fixes are never
// applied even with --apply.
func TestRun_Apply_DoesNotApplyLow(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	result := Run(root, "all", "apply", false)
	for _, fx := range result.Fixes {
		if fx.Confidence == ConfidenceLow && fx.Applied {
			t.Errorf("low-confidence fix %q must never be applied", fx.RuleID)
		}
	}
}

// TestNew_JSONOutput verifies that --json emits valid JSON with Mode set.
func TestNew_JSONOutput(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--root", root, "--json"})
	_ = cmd.Execute()
	var result FixResult
	if err := json.NewDecoder(&out).Decode(&result); err != nil {
		t.Fatalf("--json must produce valid JSON: %v\noutput: %s", err, out.String())
	}
	if result.Mode == "" {
		t.Error("expected non-empty Mode in JSON output")
	}
}
