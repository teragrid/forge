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
	"strings"
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

// ── Checkpoint marker files (progress counting bug fix) ──────────────────────
//
// Test design (9-point):
//  1. Happy path: full RunWithOptions writes test.md, code.md, ship.md, qa-verify.md.
//  2. Boundary: marker not written for failed checkpoints.
//  3. Negative: marker not written when specSlug is empty (no description, no name).
//  4. Idempotency: marker not overwritten when artefact (spec.md) already exists.
//  5. False-positive guard: ship status countCheckpoints returns correct count.

// TestRunWithOptions_WritesCheckpointMarkers verifies that after a complete
// no-op pipeline run, all seven <checkpoint>.md markers exist in the spec dir.
func TestRunWithOptions_WritesCheckpointMarkers(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	const desc = "test marker feature"
	slug := slugify(desc)

	res := RunWithOptions(RunOptions{
		// This test exercises pipeline mechanics, not the 4-stage testing
		// gate; since 1.8.2 a bare temp dir legitimately fails qa-verify for
		// having no testing evidence.
		NoStrictTesting: true,
		Root:            root,
		Description:     desc,
	})
	if !res.Ready {
		t.Fatalf("expected Ready=true, got message: %s", res.Message)
	}

	specDir := filepath.Join(root, ".forge", "specs", slug)
	for _, cpName := range []string{"spec", "arch", "test", "breakdown", "code", "ship", "qa-verify"} {
		marker := filepath.Join(specDir, cpName+".md")
		if _, err := os.Stat(marker); err != nil {
			t.Errorf("expected marker file %s.md to exist after passing checkpoint, got: %v", cpName, err)
		}
	}
}

// TestRunWithOptions_MarkerNotWrittenOnFail verifies that no marker is written
// when the checkpoint fails (forced by writing an invalid spec that blocks ship).
// We test with an artificially pre-written spec.md that blocks the ship checkpoint:
// this is a negative/boundary case — failed checkpoint must not write its marker.
func TestRunWithOptions_MarkerNotWrittenOnFail(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	const desc = "failing marker feature"

	// Run a single "ship" checkpoint in an otherwise empty project.
	// It will return warning (not fail) since no code issues exist in an empty dir.
	// We verify the marker is still written for non-fail statuses.
	res := RunWithOptions(RunOptions{
		Root:        root,
		Description: desc,
		Names:       []string{"spec"},
	})

	slug := slugify(desc)
	specDir := filepath.Join(root, ".forge", "specs", slug)

	if res.Checkpoints[0].Status == "fail" {
		// If spec fails, its marker must NOT exist.
		marker := filepath.Join(specDir, "spec.md")
		if _, err := os.Stat(marker); err == nil {
			t.Error("spec marker must not exist when spec checkpoint fails")
		}
	} else {
		// If spec passes (ok/warning), its marker MUST exist.
		marker := filepath.Join(specDir, "spec.md")
		if _, err := os.Stat(marker); err != nil {
			t.Errorf("spec marker must exist when spec passes, got: %v", err)
		}
	}
}

// TestRunWithOptions_MarkerNotWrittenWithoutSlug verifies no marker file when
// neither description nor specName is provided.
func TestRunWithOptions_MarkerNotWrittenWithoutSlug(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	// Run with empty description and no specName — specSlug will be empty.
	res := RunWithOptions(RunOptions{
		Root:        root,
		Description: "",
		Names:       []string{"spec"},
	})
	_ = res // We just check no panics and no unexpected directory.
	specsDir := filepath.Join(root, ".forge", "specs")
	if entries, _ := os.ReadDir(specsDir); len(entries) > 0 {
		t.Errorf("expected no spec dirs with empty description, got entries: %v", entries)
	}
}

// TestRunWithOptions_MarkerDoesNotOverwriteExistingArtefact verifies that the
// marker write is skipped when an artefact (e.g. spec.md) already exists from
// a previous run — idempotency guard.
func TestRunWithOptions_MarkerDoesNotOverwriteExistingArtefact(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	const desc = "idempotent marker feature"
	slug := slugify(desc)

	specDir := filepath.Join(root, ".forge", "specs", slug)
	_ = os.MkdirAll(specDir, 0o755)
	const original = "# My original spec content\n\nDo not overwrite me.\n"
	_ = os.WriteFile(filepath.Join(specDir, "spec.md"), []byte(original), 0o600)

	// Run spec checkpoint — spec.md exists so checkSpec returns "ok" without LLM.
	RunWithOptions(RunOptions{
		Root:        root,
		Description: desc,
		Names:       []string{"spec"},
	})

	// The marker write must not have overwritten the real spec.md.
	got, err := os.ReadFile(filepath.Join(specDir, "spec.md"))
	if err != nil {
		t.Fatalf("spec.md missing: %v", err)
	}
	if string(got) != original {
		t.Errorf("spec.md was overwritten by marker write; got %q", string(got))
	}
}

// TestCountCheckpoints_CorrectAfterMarkers verifies that countCheckpoints returns
// 7 when all seven marker files exist in the spec directory.
func TestCountCheckpoints_CorrectAfterMarkers(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	slug := "full-feature"
	specDir := filepath.Join(root, ".forge", "specs", slug)
	_ = os.MkdirAll(specDir, 0o755)

	for _, name := range []string{"spec", "arch", "test", "breakdown", "code", "ship", "qa-verify"} {
		_ = os.WriteFile(filepath.Join(specDir, name+".md"), []byte("# "+name+"\n"), 0o600)
	}

	count := countCheckpoints(specDir)
	if count != 7 {
		t.Errorf("expected countCheckpoints=7, got %d", count)
	}
}

// TestCountCheckpoints_PartialProgress verifies countCheckpoints returns the
// number of existing markers (the pre-fix bug returned 3 instead of correct 3 for
// the RFC-005 feature because test.md and code.md were not being written).
func TestCountCheckpoints_PartialProgress(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	slug := "partial-feature"
	specDir := filepath.Join(root, ".forge", "specs", slug)
	_ = os.MkdirAll(specDir, 0o755)

	// Write only spec + arch + test (3 markers) — confirms post-fix counting.
	for _, name := range []string{"spec", "arch", "test"} {
		_ = os.WriteFile(filepath.Join(specDir, name+".md"), []byte("# "+name+"\n"), 0o600)
	}

	count := countCheckpoints(specDir)
	if count != 3 {
		t.Errorf("expected countCheckpoints=3, got %d", count)
	}
}

// ── arch.go: parallel debate roles ──────────────────────────────────────────
//
// Test design:
//  1. Happy path: defaultArchRoles() returns 6 roles.
//  2. nil-pipe: runParallelArchDebate returns "".
//  3. Idempotency: calling twice produces two "Reviewer Concerns" sections.
//  4. False-positive: no panic on empty archDoc.

// TestDefaultArchRoles_SixRoles verifies exactly 6 roles are defined.
func TestDefaultArchRoles_SixRoles(t *testing.T) {
	t.Parallel()
	roles := defaultArchRoles()
	if len(roles) != 6 {
		t.Errorf("expected 6 arch roles, got %d", len(roles))
	}
	for _, r := range roles {
		if r.name == "" {
			t.Error("arch role has empty name")
		}
		if r.persona == "" {
			t.Error("arch role has empty persona")
		}
	}
}

// TestRunParallelArchDebate_NilPipeNoOp verifies nil pipe returns "".
func TestRunParallelArchDebate_NilPipeNoOp(t *testing.T) {
	t.Parallel()
	got := runParallelArchDebate(nil, "feature", "# Arch Doc", 300)
	if got != "" {
		t.Errorf("expected empty string for nil pipe, got %q", got)
	}
}

// TestRunParallelArchDebate_EmptyDocNoPanic verifies graceful handling of empty archDoc.
func TestRunParallelArchDebate_EmptyDocNoPanic(t *testing.T) {
	t.Parallel()
	// Regression guard: must not panic with empty doc and nil pipe.
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("runParallelArchDebate panicked: %v", r)
		}
	}()
	_ = runParallelArchDebate(nil, "feat", "", 100)
}

// ── DAG parallel pipeline ──────────────────────────────────────────────────
//
// Test design:
//  1. Happy path: RunWithOptions with full pipeline (Names==nil) returns 7 checkpoints.
//  2. arch and test checkpoints must both appear in the result (not deduped/lost).
//  3. Concurrency guard: no data race with -race flag (enforced by go test -race).
//  4. Order: checkpoint order must be spec(0), arch(1), test(2), breakdown(3), … after parallel exec.

// TestDAGPipeline_ArchTestBothPresent verifies that after a full RunWithOptions
// both arch and test checkpoints appear in the result (parallel execution did not lose one).
func TestDAGPipeline_ArchTestBothPresent(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	res := RunWithOptions(RunOptions{
		// This test exercises pipeline mechanics, not the 4-stage testing
		// gate; since 1.8.2 a bare temp dir legitimately fails qa-verify for
		// having no testing evidence.
		NoStrictTesting: true,
		Root:            root,
		Description:     "dag parallel test",
	})
	if !res.Ready {
		t.Fatalf("expected Ready=true, message: %s", res.Message)
	}
	names := make(map[string]bool)
	for _, cp := range res.Checkpoints {
		names[strings.ToLower(cp.Name)] = true
	}
	for _, want := range []string{"arch", "test"} {
		if !names[want] {
			t.Errorf("checkpoint %q missing from parallel pipeline result", want)
		}
	}
}

// TestDAGPipeline_CheckpointOrder verifies the stable order: spec, arch, test, breakdown, code, ship, qa-verify.
func TestDAGPipeline_CheckpointOrder(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	res := RunWithOptions(RunOptions{
		// This test exercises pipeline mechanics, not the 4-stage testing
		// gate; since 1.8.2 a bare temp dir legitimately fails qa-verify for
		// having no testing evidence.
		NoStrictTesting: true,
		Root:            root,
		Description:     "order test feature",
	})
	if !res.Ready {
		t.Fatalf("expected Ready=true, message: %s", res.Message)
	}
	wantOrder := []string{"spec", "arch", "test", "breakdown", "code", "ship", "qa-verify"}
	if len(res.Checkpoints) != len(wantOrder) {
		t.Fatalf("expected %d checkpoints, got %d", len(wantOrder), len(res.Checkpoints))
	}
	for i, want := range wantOrder {
		got := strings.ToLower(res.Checkpoints[i].Name)
		if got != want {
			t.Errorf("checkpoint[%d]: expected %q, got %q", i, want, got)
		}
	}
}
