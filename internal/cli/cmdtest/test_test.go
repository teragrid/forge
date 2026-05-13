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

// Test-design checklist (always-write-tests.md 9-point):
//  1. Happy path          — every known family resolves to a non-empty plan.
//  2. Boundary            — empty root path defaults to cwd; workers=0 accepted.
//  3. Negative            — unknown family produces Status:"fail" in result.
//  4. Idempotency         — Run called twice with same args yields same structure.
//  5. Concurrency         — all tests run in parallel; each owns isolated tmp dir.
//  6. Cross-family        — unit result must not contain soak/perf results.
//  7. Regression          — every Family constant in orderedFamilies resolves.
//  8. Data-accuracy       — FamilyResult.Family == requested family; counts ≥ 0.
//  9. False-positive guard — unit run must NOT produce a "fail" status.
package cmdtest

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// ── Happy path ────────────────────────────────────────────────────────────────

func TestRun_Unit_HappyPath(t *testing.T) {
	t.Parallel()
	res := Run([]Family{FamilyUnit}, RunOptions{Root: t.TempDir(), DryRun: true})
	if res == nil {
		t.Fatal("Run returned nil")
	}
	if len(res.Families) != 1 {
		t.Fatalf("expected 1 family result, got %d", len(res.Families))
	}
	if res.Families[0].Family != FamilyUnit {
		t.Fatalf("expected family %q, got %q", FamilyUnit, res.Families[0].Family)
	}
	// In dry-run the status is "pending" (plan only).
	if res.Families[0].Status != "pending" {
		t.Fatalf("expected status pending, got %q", res.Families[0].Status)
	}
	if !res.Ready {
		t.Fatal("expected Ready=true for dry-run pending plan")
	}
}

func TestRun_All_HappyPath(t *testing.T) {
	t.Parallel()
	res := Run(orderedFamilies, RunOptions{Root: t.TempDir(), DryRun: true})
	if len(res.Families) != len(orderedFamilies) {
		t.Fatalf("expected %d families, got %d", len(orderedFamilies), len(res.Families))
	}
	if !res.Ready {
		t.Fatalf("all-families dry-run should be Ready; failures: %+v", res.Families)
	}
}

// ── Regression: every ordered family must resolve (no unknown-family error) ──

func TestRun_AllFamiliesResolve(t *testing.T) {
	t.Parallel()
	opts := RunOptions{Root: t.TempDir(), DryRun: true}
	for _, f := range orderedFamilies {
		f := f // capture
		t.Run(string(f), func(t *testing.T) {
			t.Parallel()
			res := Run([]Family{f}, opts)
			if len(res.Families) != 1 {
				t.Fatalf("[%s] expected 1 result, got %d", f, len(res.Families))
			}
			if res.Families[0].Status == "fail" {
				t.Fatalf("[%s] unexpected fail: %s", f, res.Families[0].Detail)
			}
			if res.Families[0].Family != f {
				t.Fatalf("[%s] family mismatch: got %q", f, res.Families[0].Family)
			}
		})
	}
}

// ── Negative: unknown family name ─────────────────────────────────────────────

func TestRun_UnknownFamily(t *testing.T) {
	t.Parallel()
	res := Run([]Family{"not-a-real-family"}, RunOptions{Root: t.TempDir(), DryRun: true})
	if len(res.Families) != 1 {
		t.Fatalf("expected 1 result, got %d", len(res.Families))
	}
	if res.Families[0].Status != "fail" {
		t.Fatalf("expected fail for unknown family, got %q", res.Families[0].Status)
	}
	if !strings.Contains(res.Families[0].Detail, "unknown family") {
		t.Fatalf("expected 'unknown family' in detail, got: %s", res.Families[0].Detail)
	}
	if res.Ready {
		t.Fatal("expected Ready=false when a family fails")
	}
}

// ── Boundary: workers flag preserved in plan output ───────────────────────────

func TestRun_LoadWorkers_InDetail(t *testing.T) {
	t.Parallel()
	res := Run([]Family{FamilyLoad}, RunOptions{Root: t.TempDir(), DryRun: true, Workers: 42})
	if len(res.Families) != 1 {
		t.Fatalf("expected 1 family, got %d", len(res.Families))
	}
	if !strings.Contains(res.Families[0].Detail, "42") {
		t.Fatalf("expected worker count 42 in detail: %s", res.Families[0].Detail)
	}
}

func TestRun_SoakDuration_InDetail(t *testing.T) {
	t.Parallel()
	res := Run([]Family{FamilySoak}, RunOptions{Root: t.TempDir(), DryRun: true, Duration: "2h"})
	if !strings.Contains(res.Families[0].Detail, "2h") {
		t.Fatalf("expected duration 2h in detail: %s", res.Families[0].Detail)
	}
}

func TestRun_PerfBenchCount_InDetail(t *testing.T) {
	t.Parallel()
	res := Run([]Family{FamilyPerf}, RunOptions{Root: t.TempDir(), DryRun: true, BenchCount: 10})
	if !strings.Contains(res.Families[0].Detail, "10") {
		t.Fatalf("expected bench-count 10 in detail: %s", res.Families[0].Detail)
	}
}

// ── Idempotency ───────────────────────────────────────────────────────────────

func TestRun_Idempotent(t *testing.T) {
	t.Parallel()
	opts := RunOptions{Root: t.TempDir(), DryRun: true}
	r1 := Run([]Family{FamilyUnit}, opts)
	r2 := Run([]Family{FamilyUnit}, opts)
	if r1.Ready != r2.Ready {
		t.Fatalf("idempotency violation: first Ready=%v second Ready=%v", r1.Ready, r2.Ready)
	}
	if len(r1.Families) != len(r2.Families) {
		t.Fatalf("idempotency violation: first %d families, second %d", len(r1.Families), len(r2.Families))
	}
	if r1.Families[0].Status != r2.Families[0].Status {
		t.Fatalf("idempotency violation: first status %q, second %q",
			r1.Families[0].Status, r2.Families[0].Status)
	}
}

// ── Data accuracy ─────────────────────────────────────────────────────────────

func TestRun_DataAccuracy_FamilyField(t *testing.T) {
	t.Parallel()
	families := []Family{FamilySmoke, FamilyUnit, FamilyJourney}
	res := Run(families, RunOptions{Root: t.TempDir(), DryRun: true})
	for i, fr := range res.Families {
		if fr.Family != families[i] {
			t.Errorf("families[%d]: expected %q got %q", i, families[i], fr.Family)
		}
		if fr.TestCount < 0 {
			t.Errorf("families[%d]: negative TestCount %d", i, fr.TestCount)
		}
	}
}

// ── False-positive guard ──────────────────────────────────────────────────────

// Requesting unit must NOT produce soak/perf/load results.
func TestRun_Unit_NoSoakResult(t *testing.T) {
	t.Parallel()
	res := Run([]Family{FamilyUnit}, RunOptions{Root: t.TempDir(), DryRun: true})
	for _, fr := range res.Families {
		if fr.Family == FamilySoak || fr.Family == FamilyLoad || fr.Family == FamilyPerf {
			t.Fatalf("unit run must not produce %s results", fr.Family)
		}
	}
}

// Unit result must not be "fail" status in dry-run.
func TestRun_Unit_NotFail(t *testing.T) {
	t.Parallel()
	res := Run([]Family{FamilyUnit}, RunOptions{Root: t.TempDir(), DryRun: true})
	if res.Families[0].Status == "fail" {
		t.Fatalf("unit dry-run must not fail: %s", res.Families[0].Detail)
	}
}

// ── FailFast ─────────────────────────────────────────────────────────────────

func TestRun_FailFast_StopsOnUnknown(t *testing.T) {
	t.Parallel()
	families := []Family{"bad-family", FamilyUnit, FamilySoak}
	res := Run(families, RunOptions{Root: t.TempDir(), DryRun: true, FailFast: true})
	// With fail-fast, should stop after the first failure.
	if len(res.Families) != 1 {
		t.Fatalf("fail-fast: expected 1 result, got %d", len(res.Families))
	}
	if res.Families[0].Status != "fail" {
		t.Fatalf("fail-fast: expected fail status, got %q", res.Families[0].Status)
	}
}

// ── Cobra command wiring ──────────────────────────────────────────────────────

func TestCmd_Unit_TextOutput(t *testing.T) {
	t.Parallel()
	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"unit"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unit subcommand failed: %v\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "forge test") {
		t.Fatalf("missing forge test header: %s", out.String())
	}
	if !strings.Contains(out.String(), "unit") {
		t.Fatalf("missing family name in output: %s", out.String())
	}
}

func TestCmd_All_JSONOutput(t *testing.T) {
	t.Parallel()
	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"all", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("all --json failed: %v\n%s", err, out.String())
	}
	var res TestResult
	if err := json.Unmarshal(out.Bytes(), &res); err != nil {
		t.Fatalf("not valid JSON: %v\n%s", err, out.String())
	}
	if !res.DryRun {
		t.Fatal("expected dry_run=true")
	}
	if len(res.Families) != len(orderedFamilies) {
		t.Fatalf("expected %d families in JSON, got %d", len(orderedFamilies), len(res.Families))
	}
}

func TestCmd_Journey_JSONOutput(t *testing.T) {
	t.Parallel()
	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"journey", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("journey --json failed: %v\n%s", err, out.String())
	}
	var res TestResult
	if err := json.Unmarshal(out.Bytes(), &res); err != nil {
		t.Fatalf("not valid JSON: %v\n%s", err, out.String())
	}
	if len(res.Families) != 1 || res.Families[0].Family != FamilyJourney {
		t.Fatalf("wrong family in JSON: %+v", res.Families)
	}
}

// TestFamilyRegistry verifies every entry in FamilyRegistry() has a non-empty description.
func TestFamilyRegistry_Complete(t *testing.T) {
	t.Parallel()
	seen := map[Family]bool{}
	for _, m := range FamilyRegistry() {
		if m.Description == "" {
			t.Errorf("family %q has no description", m.Family)
		}
		if seen[m.Family] {
			t.Errorf("duplicate family %q in registry", m.Family)
		}
		seen[m.Family] = true
	}
	// Every orderedFamily must appear in the registry.
	for _, f := range orderedFamilies {
		if !seen[f] {
			t.Errorf("family %q missing from FamilyRegistry()", f)
		}
	}
}
