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

package cmdtest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// ── GenerateSpec ──────────────────────────────────────────────────────────────

// Test design:
//   happy   — dry-run returns 9 cases, no file written
//   happy   — non-dry-run writes spec.yml with correct content
//   boundary — no families defaults to unit/integration/regression
//   negative — empty feature returns not-ready
//   idempotency — generate twice, second result matches first
//   data accuracy — 9 caseTypes → exactly 9 cases in the spec
//   false-positive guard — non-empty feature always returns Ready=true in dry-run

func TestGenerateSpec_DryRun(t *testing.T) {
	res := GenerateSpec(SpecGenerateOptions{
		Feature: "rate-limiter",
		DryRun:  true,
		Root:    t.TempDir(),
	})
	if !res.Ready {
		t.Fatalf("expected Ready=true, got message: %s", res.Message)
	}
	if res.DryRun != true {
		t.Errorf("expected DryRun=true")
	}
	if res.CaseCount != len(caseTypes) {
		t.Errorf("expected %d cases, got %d", len(caseTypes), res.CaseCount)
	}
	if res.Spec == nil {
		t.Fatal("expected Spec to be populated in dry-run")
	}
	// Verify no file was written.
	if _, err := os.Stat(res.SpecPath); err == nil {
		t.Error("dry-run should not write a file, but file exists")
	}
}

func TestGenerateSpec_WritesFile(t *testing.T) {
	dir := t.TempDir()
	res := GenerateSpec(SpecGenerateOptions{
		Feature: "checkout",
		DryRun:  false,
		Root:    dir,
	})
	if !res.Ready {
		t.Fatalf("expected Ready=true, got: %s", res.Message)
	}
	if _, err := os.Stat(res.SpecPath); err != nil {
		t.Fatalf("expected file at %s: %v", res.SpecPath, err)
	}
	data, err := os.ReadFile(res.SpecPath)
	if err != nil {
		t.Fatalf("read spec: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "feature: checkout") {
		t.Errorf("spec YAML missing 'feature: checkout':\n%s", content)
	}
	if !strings.Contains(content, "happy_path") {
		t.Errorf("spec YAML missing 'happy_path' case")
	}
}

func TestGenerateSpec_EmptyFeature_Error(t *testing.T) {
	res := GenerateSpec(SpecGenerateOptions{
		Feature: "",
		DryRun:  true,
		Root:    t.TempDir(),
	})
	if res.Ready {
		t.Error("expected Ready=false for empty feature")
	}
	if !strings.Contains(res.Message, "feature name required") {
		t.Errorf("unexpected message: %s", res.Message)
	}
}

func TestGenerateSpec_DefaultFamilies(t *testing.T) {
	res := GenerateSpec(SpecGenerateOptions{
		Feature:  "auth",
		Families: nil, // should default
		DryRun:   true,
		Root:     t.TempDir(),
	})
	if res.Spec == nil {
		t.Fatal("expected non-nil spec")
	}
	if len(res.Spec.Families) != len(defaultSpecFamilies) {
		t.Errorf("expected %d default families, got %d", len(defaultSpecFamilies), len(res.Spec.Families))
	}
	for i, f := range defaultSpecFamilies {
		if res.Spec.Families[i] != f {
			t.Errorf("family[%d] = %q, want %q", i, res.Spec.Families[i], f)
		}
	}
}

func TestGenerateSpec_9Cases(t *testing.T) {
	res := GenerateSpec(SpecGenerateOptions{Feature: "payments", DryRun: true, Root: t.TempDir()})
	if res.Spec == nil {
		t.Fatal("expected non-nil spec")
	}
	if len(res.Spec.Cases) != 9 {
		t.Errorf("expected 9 case types, got %d", len(res.Spec.Cases))
	}
	seen := make(map[string]bool)
	for _, c := range res.Spec.Cases {
		seen[c.Type] = true
	}
	for _, ct := range caseTypes {
		if !seen[ct] {
			t.Errorf("missing case type %q", ct)
		}
	}
}

func TestGenerateSpec_FalsePositiveGuard(t *testing.T) {
	// A valid feature must always succeed in dry-run.
	for _, feat := range []string{"auth", "rate-limiter", "billing", "x"} {
		res := GenerateSpec(SpecGenerateOptions{Feature: feat, DryRun: true, Root: t.TempDir()})
		if !res.Ready {
			t.Errorf("feature %q: expected Ready=true, got: %s", feat, res.Message)
		}
	}
}

func TestGenerateSpec_Idempotent(t *testing.T) {
	dir := t.TempDir()
	opts := SpecGenerateOptions{Feature: "notifications", DryRun: false, Root: dir}
	r1 := GenerateSpec(opts)
	if !r1.Ready {
		t.Fatalf("first run failed: %s", r1.Message)
	}
	r2 := GenerateSpec(opts)
	if !r2.Ready {
		t.Fatalf("second run failed: %s", r2.Message)
	}
	// Both runs produce the same case count and spec path.
	if r1.CaseCount != r2.CaseCount {
		t.Errorf("case count changed between runs: %d vs %d", r1.CaseCount, r2.CaseCount)
	}
	if r1.SpecPath != r2.SpecPath {
		t.Errorf("spec path changed: %q vs %q", r1.SpecPath, r2.SpecPath)
	}
}

// ── loadSpec ──────────────────────────────────────────────────────────────────

func TestLoadSpec_ValidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "spec.yml")
	content := `feature: auth
version: 1
description: auth feature
families:
  - unit
cases:
  - id: TC-01
    name: happy path
    family: unit
    type: happy_path
    arrange: create user
    act: call login
    assert: returns token
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	spec, err := loadSpec(path)
	if err != nil {
		t.Fatalf("loadSpec: %v", err)
	}
	if spec.Feature != "auth" {
		t.Errorf("feature = %q, want %q", spec.Feature, "auth")
	}
	if len(spec.Cases) != 1 {
		t.Errorf("cases = %d, want 1", len(spec.Cases))
	}
	if spec.Cases[0].Type != "happy_path" {
		t.Errorf("case type = %q, want happy_path", spec.Cases[0].Type)
	}
}

func TestLoadSpec_StripComments(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "spec.yml")
	content := `# forge test spec — auth
# Generated by forge test spec auth
# Edit the cases below

feature: auth
version: 1
description: auth
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	spec, err := loadSpec(path)
	if err != nil {
		t.Fatalf("loadSpec with comments: %v", err)
	}
	if spec.Feature != "auth" {
		t.Errorf("feature = %q, want auth", spec.Feature)
	}
}

func TestLoadSpec_MissingFeature_Error(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "spec.yml")
	// YAML with no feature field.
	if err := os.WriteFile(path, []byte("version: 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := loadSpec(path)
	if err == nil {
		t.Fatal("expected error for spec with no feature field")
	}
	if !strings.Contains(err.Error(), "spec.feature is required") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLoadSpec_NonexistentFile(t *testing.T) {
	_, err := loadSpec("/nonexistent/spec.yml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

// ── deduplicateFamilies ───────────────────────────────────────────────────────

func TestDeduplicateFamilies_SpecAndCases(t *testing.T) {
	specFamilies := []Family{FamilyUnit}
	cases := []SpecCase{
		{Family: FamilyUnit},        // duplicate
		{Family: FamilyIntegration}, // new
		{Family: FamilyRegression},  // new
	}
	result := deduplicateFamilies(specFamilies, cases)
	if len(result) != 3 {
		t.Errorf("expected 3 unique families, got %d: %v", len(result), result)
	}
	if result[0] != FamilyUnit {
		t.Errorf("first family should be unit, got %q", result[0])
	}
}

func TestDeduplicateFamilies_NoDuplicates(t *testing.T) {
	// False-positive guard: families with no duplicates must not be collapsed.
	specFamilies := []Family{FamilyUnit, FamilyIntegration, FamilyRegression}
	result := deduplicateFamilies(specFamilies, nil)
	if len(result) != 3 {
		t.Errorf("expected 3 families, got %d", len(result))
	}
}

func TestDeduplicateFamilies_Empty_DefaultsToFallback(t *testing.T) {
	result := deduplicateFamilies(nil, nil)
	if len(result) == 0 {
		t.Error("expected defaultSpecFamilies when no families given")
	}
}

// ── RunFromSpec ───────────────────────────────────────────────────────────────

func TestRunFromSpec_MissingPath_Error(t *testing.T) {
	res := RunFromSpec(SpecRunOptions{
		SpecPath: "",
		Feature:  "",
	})
	if res.Ready {
		t.Error("expected Ready=false when no spec path or feature")
	}
	if !strings.Contains(res.Message, "--spec") {
		t.Errorf("expected hint about --spec in message: %s", res.Message)
	}
}

func TestRunFromSpec_NonexistentSpec_Error(t *testing.T) {
	res := RunFromSpec(SpecRunOptions{
		SpecPath: "/nonexistent/spec.yml",
		RunOptions: RunOptions{
			Root:   t.TempDir(),
			DryRun: true,
		},
	})
	if res.Ready {
		t.Error("expected Ready=false for missing spec file")
	}
}

func TestRunFromSpec_DryRun(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "spec.yml")
	content := `feature: auth
version: 1
description: auth
families:
  - unit
  - regression
cases: []
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	res := RunFromSpec(SpecRunOptions{
		SpecPath: path,
		RunOptions: RunOptions{
			Root:   dir,
			DryRun: true,
		},
	})
	if !res.Ready {
		t.Fatalf("expected Ready=true: %s", res.Message)
	}
	if res.Feature != "auth" {
		t.Errorf("feature = %q, want auth", res.Feature)
	}
	if len(res.Families) < 1 {
		t.Errorf("expected at least 1 family, got %d", len(res.Families))
	}
}

func TestRunFromSpec_Idempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "spec.yml")
	content := `feature: payments
version: 1
description: payment flow
families:
  - unit
cases: []
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	opts := SpecRunOptions{
		SpecPath:   path,
		RunOptions: RunOptions{Root: dir, DryRun: true},
	}
	r1 := RunFromSpec(opts)
	r2 := RunFromSpec(opts)
	if r1.Feature != r2.Feature {
		t.Errorf("feature changed between runs: %q vs %q", r1.Feature, r2.Feature)
	}
	if len(r1.Families) != len(r2.Families) {
		t.Errorf("families changed between runs: %d vs %d", len(r1.Families), len(r2.Families))
	}
}

// ── newSpecCmd ────────────────────────────────────────────────────────────────

func TestNewSpecCmd_FlagsRegistered(t *testing.T) {
	cmd := newSpecCmd()
	if cmd.Name() != "spec" {
		t.Errorf("expected command name 'spec', got %q", cmd.Name())
	}

	// spec is now a leaf command — it must NOT have a 'generate' subcommand.
	for _, sub := range cmd.Commands() {
		if sub.Name() == "generate" {
			t.Error("'generate' subcommand should be removed; use 'forge test spec <feature>' directly")
		}
		if sub.Name() == "run" {
			t.Error("'run' must not be a child of 'spec'; it belongs to 'forge test run'")
		}
	}

	// Key flags must exist directly on the spec command.
	if f := cmd.Flags().Lookup("dry-run"); f == nil {
		t.Error("spec command missing --dry-run flag")
	} else if f.DefValue != "false" {
		t.Errorf("--dry-run default should be false (write by default), got %q", f.DefValue)
	}
	if f := cmd.Flags().Lookup("spec"); f == nil {
		t.Error("spec command missing --spec flag")
	}
	if f := cmd.Flags().Lookup("description"); f == nil {
		t.Error("spec command missing --description flag")
	}
}

// TestRunCmd_SpecFlags verifies that the forge test run command exposes --spec
// and --feature flags for spec-driven runs.
func TestRunCmd_SpecFlags(t *testing.T) {
	// New() returns the top-level forge test command.
	parent := New()
	var runCmd *cobra.Command
	for _, sub := range parent.Commands() {
		if sub.Name() == "run" {
			runCmd = sub
			break
		}
	}
	if runCmd == nil {
		t.Fatal("'run' subcommand not found on forge test")
	}
	if f := runCmd.Flags().Lookup("spec"); f == nil {
		t.Error("forge test run missing --spec flag")
	}
	if f := runCmd.Flags().Lookup("feature"); f == nil {
		t.Error("forge test run missing --feature flag")
	}
}
