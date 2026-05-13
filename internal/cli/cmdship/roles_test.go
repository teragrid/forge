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
	"strings"
	"sync"
	"testing"
)

// ── DefaultRoles ─────────────────────────────────────────────────────────────

func TestDefaultRoles_EightRoles(t *testing.T) {
	t.Parallel()
	roles := DefaultRoles()
	if len(roles) != 8 {
		t.Fatalf("expected 8 roles, got %d", len(roles))
	}
}

func TestDefaultRoles_UniqueIDs(t *testing.T) {
	t.Parallel()
	seen := map[RoleID]struct{}{}
	for _, r := range DefaultRoles() {
		if _, dup := seen[r.ID]; dup {
			t.Fatalf("duplicate role ID: %q", r.ID)
		}
		seen[r.ID] = struct{}{}
	}
}

func TestDefaultRoles_FocusAreasNonEmpty(t *testing.T) {
	t.Parallel()
	for _, r := range DefaultRoles() {
		if len(r.FocusAreas) == 0 {
			t.Errorf("role %q has no focus areas", r.ID)
		}
	}
}

func TestDefaultRoles_AllIDsPresent(t *testing.T) {
	t.Parallel()
	want := map[RoleID]bool{
		RolePO: false, RoleBA: false, RoleSA: false,
		RoleDL: false, RoleQE: false, RoleSec: false,
		RoleOps: false, RoleCPO: false,
	}
	for _, r := range DefaultRoles() {
		want[r.ID] = true
	}
	for id, found := range want {
		if !found {
			t.Errorf("missing role ID: %q", id)
		}
	}
}

// ── SelfDebate — happy-path ────────────────────────────────────────────────

func TestSelfDebate_Spec_3Rounds_Consensus(t *testing.T) {
	t.Parallel()
	res := SelfDebate(DebateOptions{Deliverable: "spec", MaxRounds: 3, DryRun: true})
	if !res.Consensus {
		t.Fatal("expected consensus=true in dry-run")
	}
	if len(res.Rounds) != 3 {
		t.Fatalf("expected 3 rounds, got %d", len(res.Rounds))
	}
}

func TestSelfDebate_Spec_ImprovementsNonEmpty(t *testing.T) {
	t.Parallel()
	res := SelfDebate(DebateOptions{Deliverable: "spec", MaxRounds: 3, DryRun: true})
	if len(res.Improvements) == 0 {
		t.Fatal("expected at least one improvement for spec deliverable")
	}
}

func TestSelfDebate_Breakdown_Works(t *testing.T) {
	t.Parallel()
	res := SelfDebate(DebateOptions{Deliverable: "breakdown", MaxRounds: 3, DryRun: true})
	if !res.Consensus {
		t.Fatal("expected consensus for breakdown deliverable")
	}
	if len(res.Improvements) == 0 {
		t.Fatal("expected improvements for breakdown deliverable")
	}
}

func TestSelfDebate_Code_Works(t *testing.T) {
	t.Parallel()
	res := SelfDebate(DebateOptions{Deliverable: "code", MaxRounds: 3, DryRun: true})
	if !res.Consensus {
		t.Fatal("expected consensus for code deliverable")
	}
}

func TestSelfDebate_Verify_Works(t *testing.T) {
	t.Parallel()
	res := SelfDebate(DebateOptions{Deliverable: "verify", MaxRounds: 3, DryRun: true})
	if !res.Consensus {
		t.Fatal("expected consensus for verify deliverable")
	}
}

// ── SelfDebate — boundary cases ────────────────────────────────────────────

func TestSelfDebate_MaxRounds1_OneRound(t *testing.T) {
	t.Parallel()
	res := SelfDebate(DebateOptions{Deliverable: "spec", MaxRounds: 1, DryRun: true})
	if len(res.Rounds) != 1 {
		t.Fatalf("expected 1 round, got %d", len(res.Rounds))
	}
	if !res.Consensus {
		t.Fatal("expected consensus even with 1 round (dry-run)")
	}
}

func TestSelfDebate_MaxRoundsZero_DefaultsTo3(t *testing.T) {
	t.Parallel()
	res := SelfDebate(DebateOptions{Deliverable: "spec", MaxRounds: 0, DryRun: true})
	if len(res.Rounds) != 3 {
		t.Fatalf("MaxRounds=0 should default to 3 rounds, got %d", len(res.Rounds))
	}
}

func TestSelfDebate_MaxRoundsCap_3(t *testing.T) {
	t.Parallel()
	res := SelfDebate(DebateOptions{Deliverable: "spec", MaxRounds: 99, DryRun: true})
	if len(res.Rounds) != 3 {
		t.Fatalf("MaxRounds>3 should be capped at 3, got %d", len(res.Rounds))
	}
}

func TestSelfDebate_EmptyDeliverable_NoNilPanic(t *testing.T) {
	t.Parallel()
	// Unknown deliverable → generic catalog. Must not panic.
	res := SelfDebate(DebateOptions{Deliverable: "", DryRun: true})
	if res == nil {
		t.Fatal("unexpected nil result")
	}
	if res.Deliverable != "" {
		t.Errorf("deliverable mismatch: got %q", res.Deliverable)
	}
}

func TestSelfDebate_UnknownDeliverable_Generic(t *testing.T) {
	t.Parallel()
	res := SelfDebate(DebateOptions{Deliverable: "notadeliverable", DryRun: true})
	if res == nil {
		t.Fatal("unexpected nil result")
	}
	// Generic catalog has one concern per role, 6 roles → at least 1 round concern.
	if len(res.Rounds) == 0 {
		t.Fatal("expected at least one round")
	}
	if len(res.Rounds[0].Concerns) == 0 {
		t.Fatal("expected concerns from generic catalog")
	}
}

// ── SelfDebate — negative / nil ────────────────────────────────────────────

func TestSelfDebate_NilRoles_UsesDefaultRoles(t *testing.T) {
	t.Parallel()
	// Passing nil Roles must not panic and must default to 8 roles.
	res := SelfDebate(DebateOptions{Deliverable: "spec", Roles: nil, DryRun: true})
	if len(res.Roles) != 8 {
		t.Fatalf("expected 8 roles, got %d", len(res.Roles))
	}
}

func TestSelfDebate_EmptyRoles_NoNilPanic(t *testing.T) {
	t.Parallel()
	// An explicit empty slice: no roles → no concerns; must not panic.
	res := SelfDebate(DebateOptions{Deliverable: "spec", Roles: []Role{}, DryRun: true})
	if res == nil {
		t.Fatal("unexpected nil result")
	}
}

// ── SelfDebate — data accuracy ─────────────────────────────────────────────

func TestSelfDebate_AllRolesInResult(t *testing.T) {
	t.Parallel()
	res := SelfDebate(DebateOptions{Deliverable: "spec", DryRun: true})
	wantIDs := map[RoleID]bool{
		RolePO: false, RoleBA: false, RoleSA: false,
		RoleDL: false, RoleQE: false, RoleSec: false,
		RoleOps: false, RoleCPO: false,
	}
	for _, id := range res.Roles {
		wantIDs[id] = true
	}
	for id, found := range wantIDs {
		if !found {
			t.Errorf("role %q absent from DebateResult.Roles", id)
		}
	}
}

func TestSelfDebate_Round1AllRolesHaveConcerns(t *testing.T) {
	t.Parallel()
	res := SelfDebate(DebateOptions{Deliverable: "spec", DryRun: true})
	if len(res.Rounds) == 0 {
		t.Fatal("no rounds")
	}
	rolesPresent := map[RoleID]bool{}
	for _, c := range res.Rounds[0].Concerns {
		rolesPresent[c.Role] = true
	}
	for _, id := range []RoleID{RolePO, RoleBA, RoleSA, RoleDL, RoleQE, RoleSec, RoleOps, RoleCPO} {
		if !rolesPresent[id] {
			t.Errorf("role %q raised no concerns in round 1 (spec)", id)
		}
	}
}

func TestSelfDebate_PolishedSummaryNonEmpty(t *testing.T) {
	t.Parallel()
	res := SelfDebate(DebateOptions{Deliverable: "spec", Feature: "payments", DryRun: true})
	if res.PolishedSummary == "" {
		t.Fatal("PolishedSummary should not be empty")
	}
}

func TestSelfDebate_PolishedSummary_ContainsFeature(t *testing.T) {
	t.Parallel()
	res := SelfDebate(DebateOptions{Deliverable: "spec", Feature: "payments", DryRun: true})
	if !contains(res.PolishedSummary, "payments") {
		t.Errorf("PolishedSummary should mention feature name; got: %s", res.PolishedSummary)
	}
}

func TestSelfDebate_Round1Summary_MentionsCounts(t *testing.T) {
	t.Parallel()
	res := SelfDebate(DebateOptions{Deliverable: "spec", DryRun: true})
	if len(res.Rounds[0].Summary) == 0 {
		t.Fatal("round 1 summary should not be empty")
	}
}

// ── SelfDebate — idempotency ────────────────────────────────────────────────

func TestSelfDebate_Idempotent(t *testing.T) {
	t.Parallel()
	opts := DebateOptions{Deliverable: "spec", Feature: "idempotency-test", DryRun: true}
	r1 := SelfDebate(opts)
	r2 := SelfDebate(opts)
	if len(r1.Improvements) != len(r2.Improvements) {
		t.Fatalf("same opts returned different improvement counts: %d vs %d",
			len(r1.Improvements), len(r2.Improvements))
	}
}

// ── SelfDebate — concurrency ────────────────────────────────────────────────

func TestSelfDebate_Concurrent_NoRace(t *testing.T) {
	t.Parallel()
	const workers = 8
	var wg sync.WaitGroup
	for i := range workers {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			deliverables := []string{"spec", "breakdown", "code", "verify"}
			d := deliverables[n%len(deliverables)]
			res := SelfDebate(DebateOptions{Deliverable: d, DryRun: true})
			if res == nil {
				t.Errorf("goroutine %d: nil result", n)
			}
		}(i)
	}
	wg.Wait()
}

// ── SelfDebate — false-positive guard ─────────────────────────────────────

// Code catalog must not contain NFR concerns (those belong in spec catalog).
func TestSelfDebate_Code_NoNFRConcerns(t *testing.T) {
	t.Parallel()
	res := SelfDebate(DebateOptions{Deliverable: "code", DryRun: true})
	for _, round := range res.Rounds {
		for _, c := range round.Concerns {
			if c.Area == "NFRs" {
				t.Errorf("NFR concern should not appear in code deliverable; got: %+v", c)
			}
		}
	}
}

// Spec catalog must not contain branch-coverage concerns (those belong in code catalog).
func TestSelfDebate_Spec_NoBranchCoverageConcerns(t *testing.T) {
	t.Parallel()
	res := SelfDebate(DebateOptions{Deliverable: "spec", DryRun: true})
	for _, round := range res.Rounds {
		for _, c := range round.Concerns {
			if c.Area == "branch coverage" {
				t.Errorf("branch coverage concern should not appear in spec deliverable; got: %+v", c)
			}
		}
	}
}

// Code catalog must not contain deployment-impact concerns (those belong in spec/breakdown catalogs).
func TestSelfDebate_Code_NoDeploymentImpactConcerns(t *testing.T) {
	t.Parallel()
	res := SelfDebate(DebateOptions{Deliverable: "code", DryRun: true})
	for _, round := range res.Rounds {
		for _, c := range round.Concerns {
			if c.Area == "deployment impact" {
				t.Errorf("deployment impact concern should not appear in code deliverable; got: %+v", c)
			}
		}
	}
}

// Spec catalog must not contain compliance-scan concerns (those belong in verify catalog).
func TestSelfDebate_Spec_NoComplianceScanConcerns(t *testing.T) {
	t.Parallel()
	res := SelfDebate(DebateOptions{Deliverable: "spec", DryRun: true})
	for _, round := range res.Rounds {
		for _, c := range round.Concerns {
			if c.Area == "compliance scan" {
				t.Errorf("compliance scan concern should not appear in spec deliverable; got: %+v", c)
			}
		}
	}
}

// ── RunWithOptions debate integration ─────────────────────────────────────

func TestRunWithOptions_DebateOpts_AllCheckpointsHaveDebate(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	res := RunWithOptions(RunOptions{
		Root: root,
		DebateOpts: &DebateOptions{
			Feature:   "payments",
			MaxRounds: 3,
			DryRun:    true,
		},
	})
	if len(res.Checkpoints) == 0 {
		t.Fatal("expected checkpoints")
	}
	for _, cp := range res.Checkpoints {
		if cp.Debate == nil {
			t.Errorf("checkpoint %q has nil Debate, expected self-debate result", cp.Name)
		}
	}
}

func TestRunWithOptions_NoDebateOpts_CheckpointsHaveNilDebate(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	res := RunWithOptions(RunOptions{Root: root})
	for _, cp := range res.Checkpoints {
		if cp.Debate != nil {
			t.Errorf("checkpoint %q should have nil Debate when DebateOpts not set", cp.Name)
		}
	}
}

func TestRunWithOptions_DebateDeliverableMatchesCheckpointName(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	res := RunWithOptions(RunOptions{
		Root: root,
		DebateOpts: &DebateOptions{
			DryRun: true,
		},
	})
	for _, cp := range res.Checkpoints {
		if cp.Debate == nil {
			continue
		}
		wantDel := strings.ToLower(cp.Name)
		if cp.Debate.Deliverable != wantDel {
			t.Errorf("cp %q: debate.Deliverable=%q, want %q", cp.Name, cp.Debate.Deliverable, wantDel)
		}
	}
}

// ── Helper ────────────────────────────────────────────────────────────────

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsRune(s, sub))
}

func containsRune(s, sub string) bool {
	for i := range s {
		if i+len(sub) <= len(s) && s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
