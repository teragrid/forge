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

package cmdship

import (
	"os"
	"path/filepath"
	"testing"
)

// ── snapshot.go ──────────────────────────────────────────────────────────────

// TestTakeSnapshot_HappyPath verifies a snapshot is written and meta.txt exists.
func TestTakeSnapshot_HappyPath(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	slug := "my-feature"
	specDir := filepath.Join(root, ".forge", "specs", slug)
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Write a file into the spec dir.
	if err := os.WriteFile(filepath.Join(specDir, "spec.md"), []byte("# Spec\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := TakeSnapshot(root, slug, "arch"); err != nil {
		t.Fatalf("TakeSnapshot: %v", err)
	}

	// meta.txt must exist.
	if !SnapshotExists(root, slug, "arch") {
		t.Fatal("SnapshotExists returned false after TakeSnapshot")
	}

	// spec.md must be copied.
	dst := filepath.Join(root, ".forge", snapshotsBaseDir, slug, "arch", "spec.md")
	if _, err := os.Stat(dst); err != nil {
		t.Fatalf("snapshot file not present: %v", err)
	}
}

// TestTakeSnapshot_NoSpecDir is a no-op when the spec dir does not yet exist.
func TestTakeSnapshot_NoSpecDir(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// Spec dir absent — must not error.
	if err := TakeSnapshot(root, "new-feature", "spec"); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

// TestTakeSnapshot_SubdirExcluded verifies nested dirs (e.g. digests/) are skipped.
func TestTakeSnapshot_SubdirExcluded(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	slug := "feat"
	specDir := filepath.Join(root, ".forge", "specs", slug)
	nestedDir := filepath.Join(specDir, "digests")
	if err := os.MkdirAll(nestedDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Top-level file.
	_ = os.WriteFile(filepath.Join(specDir, "arch.md"), []byte("arch"), 0o600)
	// File inside nested dir — must NOT be copied.
	_ = os.WriteFile(filepath.Join(nestedDir, "arch.digest.yaml"), []byte("x: 1\n"), 0o600)

	if err := TakeSnapshot(root, slug, "arch"); err != nil {
		t.Fatalf("TakeSnapshot: %v", err)
	}

	snapDir := filepath.Join(root, ".forge", snapshotsBaseDir, slug, "arch")
	entries, _ := os.ReadDir(snapDir)
	for _, e := range entries {
		if e.IsDir() {
			t.Errorf("unexpected subdir in snapshot: %s", e.Name())
		}
	}
}

// TestRestoreSnapshot_HappyPath verifies file restoration.
func TestRestoreSnapshot_HappyPath(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	slug := "restore-test"

	// Create a snapshot manually.
	snapDir := filepath.Join(root, ".forge", snapshotsBaseDir, slug, "spec")
	if err := os.MkdirAll(snapDir, 0o755); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(snapDir, "spec.md"), []byte("old spec"), 0o600)
	_ = os.WriteFile(filepath.Join(snapDir, "meta.txt"), []byte("checkpoint: spec\n"), 0o600)

	// Overwrite the spec file in the spec dir.
	specDir := filepath.Join(root, ".forge", "specs", slug)
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(specDir, "spec.md"), []byte("new spec — bad"), 0o600)

	if err := RestoreSnapshot(root, slug, "spec"); err != nil {
		t.Fatalf("RestoreSnapshot: %v", err)
	}

	got, _ := os.ReadFile(filepath.Join(specDir, "spec.md"))
	if string(got) != "old spec" {
		t.Errorf("expected 'old spec', got %q", string(got))
	}
}

// TestRestoreSnapshot_Missing returns error when snapshot does not exist.
func TestRestoreSnapshot_Missing(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	err := RestoreSnapshot(root, "no-such-feat", "spec")
	if err == nil {
		t.Fatal("expected error when snapshot missing, got nil")
	}
}

// TestSnapshotExists_FalseWhenAbsent verifies false return when nothing written.
func TestSnapshotExists_FalseWhenAbsent(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if SnapshotExists(root, "feat", "arch") {
		t.Fatal("expected false for non-existent snapshot")
	}
}

// TestListSnapshots_ReturnsNames verifies multiple snapshots are listed.
func TestListSnapshots_ReturnsNames(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	slug := "list-test"

	for _, cp := range []string{"spec", "arch", "code"} {
		d := filepath.Join(root, ".forge", snapshotsBaseDir, slug, cp)
		_ = os.MkdirAll(d, 0o755)
		_ = os.WriteFile(filepath.Join(d, "meta.txt"), []byte("checkpoint: "+cp+"\n"), 0o600)
	}

	names := ListSnapshots(root, slug)
	if len(names) != 3 {
		t.Fatalf("expected 3 snapshot names, got %d: %v", len(names), names)
	}
}

// TestListSnapshots_NilWhenNone verifies nil returned when dir absent.
func TestListSnapshots_NilWhenNone(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if ListSnapshots(root, "nope") != nil {
		t.Fatal("expected nil for absent snapshot dir")
	}
}

// TestTakeRestoreRoundTrip verifies Take → modify → Restore restores original content.
func TestTakeRestoreRoundTrip(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	slug := "roundtrip"
	specDir := filepath.Join(root, ".forge", "specs", slug)
	_ = os.MkdirAll(specDir, 0o755)
	_ = os.WriteFile(filepath.Join(specDir, "spec.md"), []byte("original"), 0o600)

	// Take snapshot.
	if err := TakeSnapshot(root, slug, "spec"); err != nil {
		t.Fatal(err)
	}

	// Clobber the file.
	_ = os.WriteFile(filepath.Join(specDir, "spec.md"), []byte("clobbered"), 0o600)

	// Restore.
	if err := RestoreSnapshot(root, slug, "spec"); err != nil {
		t.Fatal(err)
	}

	got, _ := os.ReadFile(filepath.Join(specDir, "spec.md"))
	if string(got) != "original" {
		t.Errorf("roundtrip failed: got %q", string(got))
	}
}

// ── domainprofile.go ─────────────────────────────────────────────────────────

// TestLoadDomainProfile_Default verifies the default profile is returned for "".
func TestLoadDomainProfile_Default(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	p := LoadDomainProfile(root, "")
	if p.Profile != "default" {
		t.Errorf("expected profile=default, got %q", p.Profile)
	}
}

// TestLoadDomainProfile_BuiltinBanking verifies banking profile loads.
func TestLoadDomainProfile_BuiltinBanking(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	p := LoadDomainProfile(root, "banking")
	if p.Profile != "banking" {
		t.Errorf("expected profile=banking, got %q", p.Profile)
	}
	if !p.LLM.PIIDetection {
		t.Error("banking profile: expected PIIDetection=true")
	}
	if len(p.LLM.AllowedProviders) == 0 {
		t.Error("banking profile: expected AllowedProviders to be set")
	}
}

// TestLoadDomainProfile_BuiltinHealthcare verifies healthcare profile.
func TestLoadDomainProfile_BuiltinHealthcare(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	p := LoadDomainProfile(root, "healthcare")
	if p.Profile != "healthcare" {
		t.Errorf("expected profile=healthcare, got %q", p.Profile)
	}
	if !p.LLM.PIIDetection {
		t.Error("healthcare profile: expected PIIDetection=true")
	}
}

// TestLoadDomainProfile_UnknownFallsBackToDefault verifies fallback behaviour.
func TestLoadDomainProfile_UnknownFallsBackToDefault(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	p := LoadDomainProfile(root, "totally-unknown-domain")
	if p.Profile != "default" {
		t.Errorf("expected fallback to default, got %q", p.Profile)
	}
}

// TestLoadDomainProfile_ProjectLocalOverride verifies a .forge/domains/ YAML is loaded.
func TestLoadDomainProfile_ProjectLocalOverride(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	domainDir := filepath.Join(root, ".forge", "domains")
	_ = os.MkdirAll(domainDir, 0o755)
	yaml := `profile: custom
description: My custom domain
checkpoints:
  arch:
    budget_multiplier: 3.0
`
	_ = os.WriteFile(filepath.Join(domainDir, "custom.yml"), []byte(yaml), 0o600)

	p := LoadDomainProfile(root, "custom")
	if p.Profile != "custom" {
		t.Errorf("expected profile=custom, got %q", p.Profile)
	}
	mul := p.CheckpointBudgetMultiplier("arch", 1.0)
	if mul != 3.0 {
		t.Errorf("expected budget_multiplier=3.0, got %v", mul)
	}
}

// TestCheckpointBudgetMultiplier_DomainWins verifies domain override > complexity mul.
func TestCheckpointBudgetMultiplier_DomainWins(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	p := LoadDomainProfile(root, "banking")
	mul := p.CheckpointBudgetMultiplier("arch", 1.5 /* complexity */)
	if mul != 2.0 {
		t.Errorf("expected banking arch override=2.0, got %v", mul)
	}
}

// TestCheckpointBudgetMultiplier_ComplexityFallback verifies complexity used when no domain override.
func TestCheckpointBudgetMultiplier_ComplexityFallback(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	p := LoadDomainProfile(root, "default")
	mul := p.CheckpointBudgetMultiplier("arch", 1.75)
	if mul != 1.75 {
		t.Errorf("expected complexity fallback 1.75, got %v", mul)
	}
}

// TestCheckpointSteerings_Present verifies steerings returned when defined.
func TestCheckpointSteerings_Present(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	p := LoadDomainProfile(root, "banking")
	s := p.CheckpointSteerings("arch")
	if len(s) == 0 {
		t.Error("expected banking arch steerings to be non-empty")
	}
}

// TestCheckpointSteerings_Nil verifies nil returned for unknown checkpoint.
func TestCheckpointSteerings_Nil(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	p := LoadDomainProfile(root, "banking")
	s := p.CheckpointSteerings("no-such-checkpoint")
	if s != nil {
		t.Errorf("expected nil steerings for absent checkpoint, got %v", s)
	}
}

// TestAvailableProfiles_ContainsBuiltins verifies all expected profiles are listed.
func TestAvailableProfiles_ContainsBuiltins(t *testing.T) {
	t.Parallel()
	names := AvailableProfiles()
	expected := []string{"banking", "data-heavy", "default", "healthcare", "microservice", "saas-b2b"}
	nameSet := make(map[string]bool, len(names))
	for _, n := range names {
		nameSet[n] = true
	}
	for _, want := range expected {
		if !nameSet[want] {
			t.Errorf("expected profile %q in AvailableProfiles, not found; got: %v", want, names)
		}
	}
}

// ── llmpipe.go ScaledBudget ───────────────────────────────────────────────────

// TestScaledBudget_Multipliers verifies correct multipliers per tier.
func TestScaledBudget_Multipliers(t *testing.T) {
	t.Parallel()
	cases := []struct {
		tier ComplexityTier
		base int
		want int
	}{
		{ComplexityNano, 1000, 700},
		{ComplexityMicro, 1000, 1000},
		{ComplexityStandard, 1000, 1500},
		{ComplexityComplex, 1000, 2000},
	}
	for _, tc := range cases {
		got := ScaledBudget(tc.base, tc.tier)
		if got != tc.want {
			t.Errorf("ScaledBudget(%d, %s) = %d, want %d", tc.base, tc.tier, got, tc.want)
		}
	}
}

// TestScaledBudget_MinimumClamp verifies the 256-token floor.
func TestScaledBudget_MinimumClamp(t *testing.T) {
	t.Parallel()
	// nano × 100 = 70, which is below the 256 minimum.
	got := ScaledBudget(100, ComplexityNano)
	if got != 256 {
		t.Errorf("expected minimum 256, got %d", got)
	}
}

// TestScaledBudget_UnknownTierDefaults1x verifies unknown tier → 1.0×.
func TestScaledBudget_UnknownTierDefaults1x(t *testing.T) {
	t.Parallel()
	const unknownTier ComplexityTier = "super-complex"
	got := ScaledBudget(800, unknownTier)
	if got != 800 {
		t.Errorf("expected 800 for unknown tier, got %d", got)
	}
}

// TestScaledBudget_ZeroBaseClampedToMin verifies 0-base returns minimum.
func TestScaledBudget_ZeroBaseClampedToMin(t *testing.T) {
	t.Parallel()
	got := ScaledBudget(0, ComplexityComplex)
	if got != 256 {
		t.Errorf("expected 256 for zero base, got %d", got)
	}
}
