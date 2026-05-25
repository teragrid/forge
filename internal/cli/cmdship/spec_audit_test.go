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

// Test design (always-write-tests.md 9-point):
//  1. Happy path  — spec dir with all tasks [x] + tests present → 0 gaps.
//  2. Boundary    — empty spec dir, partial tasks, missing spec.yml.
//  3. Negative    — unchecked tasks, authz role missing from RLS test.
//  4. Idempotency — two calls with same dir yield identical result.
//  5. Concurrency — all tests parallel with isolated TempDirs.
//  6. Authz       — roles with commas in value parsed correctly.
//  7. Regression  — FORGE-3201 registered before tests run (init order).
//  8. Data-accuracy — Severity and Type values match constants.
//  9. False-positive — valid project with complete tasks → HasBlockingGaps false.
package cmdship

import (
	"os"
	"path/filepath"
	"testing"
)

// ── helpers ───────────────────────────────────────────────────────────────────

// makeSpecDir creates a .forge/specs/<slug>/ directory under root with the
// provided content files (map of relative-to-specDir path → content).
func makeSpecDir(t *testing.T, root, slug string, files map[string]string) string {
	t.Helper()
	specDir := filepath.Join(root, ".forge", "specs", slug)
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatalf("makeSpecDir MkdirAll: %v", err)
	}
	for rel, content := range files {
		full := filepath.Join(specDir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("makeSpecDir sub-dir: %v", err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("makeSpecDir write %s: %v", rel, err)
		}
	}
	return specDir
}

// countBySeverity counts gaps with the given severity string.
func countBySeverity(gaps []AuditGap, severity string) int {
	n := 0
	for _, g := range gaps {
		if g.Severity == severity {
			n++
		}
	}
	return n
}

// ── AUDIT-01: no .forge/specs/ directory → empty result ──────────────────────

func TestAuditSpecVsCode_NoSpecDir(t *testing.T) {
	t.Parallel()
	res := auditSpecVsCode(t.TempDir(), "")
	if res.SpecFound {
		t.Error("expected SpecFound=false when no .forge/specs/ dir")
	}
	if len(res.Gaps) != 0 {
		t.Errorf("expected 0 gaps, got %d", len(res.Gaps))
	}
}

// ── AUDIT-02: spec dir present but spec.md absent → skipped ──────────────────

func TestAuditSpecVsCode_SpecMDAbsent(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// Create dir but no spec.md.
	if err := os.MkdirAll(filepath.Join(root, ".forge", "specs", "my-feature"), 0o755); err != nil {
		t.Fatal(err)
	}
	res := auditSpecVsCode(root, "my feature")
	if res.SpecFound {
		t.Error("expected SpecFound=false when spec.md absent")
	}
	if len(res.Gaps) != 0 {
		t.Errorf("expected 0 gaps, got %d", len(res.Gaps))
	}
}

// ── AUDIT-03: all tasks complete + no spec.yml → 0 gaps (happy path) ─────────

func TestAuditSpecVsCode_AllTasksComplete(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	makeSpecDir(t, root, "add-auth", map[string]string{
		"spec.md":  "# Spec\n",
		"tasks.md": "- [x] Implement login\n- [x] Add JWT middleware\n",
	})
	res := auditSpecVsCode(root, "add auth")
	if !res.SpecFound {
		t.Fatal("expected SpecFound=true")
	}
	if res.HasBlockingGaps() {
		t.Errorf("expected no blocking gaps; got: %v", res.Gaps)
	}
}

// ── AUDIT-04: incomplete tasks → blocking gap (regression for FORGE-3201) ────

func TestAuditSpecVsCode_IncompleteTasks(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	makeSpecDir(t, root, "add-auth", map[string]string{
		"spec.md": "# Spec\n",
		"tasks.md": "- [x] Done task\n" +
			"- [ ] Pending task 1\n" +
			"- [ ] Pending task 2\n",
	})
	res := auditSpecVsCode(root, "add auth")
	if !res.HasBlockingGaps() {
		t.Fatal("expected blocking gap for incomplete tasks")
	}
	blocking := countBySeverity(res.Gaps, "blocking")
	if blocking == 0 {
		t.Error("expected at least 1 blocking gap")
	}
	// Type must be correct.
	found := false
	for _, g := range res.Gaps {
		if g.Type == "incomplete-tasks" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected gap of type 'incomplete-tasks'; got: %v", res.Gaps)
	}
}

// ── AUDIT-05: all tasks complete → HasBlockingGaps false (false-positive guard) ─

func TestAuditSpecVsCode_NoFalsePositive_AllComplete(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	makeSpecDir(t, root, "feature-x", map[string]string{
		"spec.md":  "# Spec\n",
		"tasks.md": "- [x] Step one\n- [x] Step two\n- [x] Step three\n",
	})
	res := auditSpecVsCode(root, "feature x")
	if res.HasBlockingGaps() {
		t.Errorf("no blocking gaps expected; got: %v", res.Gaps)
	}
}

// ── AUDIT-06: authz role missing from RLS tests → blocking gap ───────────────

func TestAuditSpecVsCode_AuthzRoleMissingFromRLSTest(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	specYML := `authz_model: "role: admin\nrole: viewer\n"
events_emitted: []
`
	makeSpecDir(t, root, "billing", map[string]string{
		"spec.md":  "# Spec\n",
		"spec.yml": specYML,
	})
	// RLS test file that only covers "admin", not "viewer".
	if err := os.MkdirAll(filepath.Join(root, "tests"), 0o755); err != nil {
		t.Fatal(err)
	}
	rlsContent := `describe("admin policy", () => { it("allows admin", () => {}) })`
	if err := os.WriteFile(filepath.Join(root, "tests", "billing.rls.test.ts"), []byte(rlsContent), 0o644); err != nil {
		t.Fatal(err)
	}
	res := auditSpecVsCode(root, "billing")
	if !res.HasBlockingGaps() {
		t.Error("expected blocking gap for untested authz role")
	}
	found := false
	for _, g := range res.Gaps {
		if g.Type == "authz-role-untested" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected gap of type 'authz-role-untested'; got: %v", res.Gaps)
	}
}

// ── AUDIT-07: authz roles all covered → 0 blocking gaps ──────────────────────

func TestAuditSpecVsCode_AuthzRolesCovered(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	specYML := `authz_model: "roles: admin, viewer\n"
events_emitted: []
`
	makeSpecDir(t, root, "billing", map[string]string{
		"spec.md":  "# Spec\n",
		"spec.yml": specYML,
	})
	if err := os.MkdirAll(filepath.Join(root, "tests"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Cover both roles in the RLS test.
	rlsContent := `describe("policies", () => {
  it("allows admin", () => {})
  it("denies viewer from private data", () => {})
})`
	if err := os.WriteFile(filepath.Join(root, "tests", "billing.rls.test.ts"), []byte(rlsContent), 0o644); err != nil {
		t.Fatal(err)
	}
	res := auditSpecVsCode(root, "billing")
	for _, g := range res.Gaps {
		if g.Type == "authz-role-untested" && g.Severity == "blocking" {
			t.Errorf("unexpected blocking authz gap: %+v", g)
		}
	}
}

// ── AUDIT-08: missing event assertion in test file → warning gap ──────────────

func TestAuditSpecVsCode_MissingEventTest(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	specYML := `authz_model: ""
events_emitted:
  - user.signed_up
  - payment.succeeded
`
	makeSpecDir(t, root, "onboarding", map[string]string{
		"spec.md":  "# Spec\n",
		"spec.yml": specYML,
	})
	// Test file only covers one event.
	if err := os.MkdirAll(filepath.Join(root, "tests"), 0o755); err != nil {
		t.Fatal(err)
	}
	testContent := `it('emits user.signed_up', () => expect(events).toContain('user.signed_up'))`
	if err := os.WriteFile(filepath.Join(root, "tests", "onboarding.test.ts"), []byte(testContent), 0o644); err != nil {
		t.Fatal(err)
	}
	res := auditSpecVsCode(root, "onboarding")
	// Missing event should be a warning, not blocking.
	found := false
	for _, g := range res.Gaps {
		if g.Type == "missing-event-test" {
			found = true
			if g.Severity == "blocking" {
				t.Errorf("missing-event-test should be 'warning', got 'blocking'")
			}
		}
	}
	if !found {
		t.Errorf("expected gap of type 'missing-event-test'; got: %v", res.Gaps)
	}
}

// ── AUDIT-09: no events in spec → no event gap ────────────────────────────────

func TestAuditSpecVsCode_NoEvents_NoGap(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	specYML := `authz_model: ""
events_emitted: []
`
	makeSpecDir(t, root, "widget", map[string]string{
		"spec.md":  "# Spec\n",
		"spec.yml": specYML,
	})
	res := auditSpecVsCode(root, "widget")
	for _, g := range res.Gaps {
		if g.Type == "missing-event-test" {
			t.Errorf("unexpected event gap when events_emitted is empty: %+v", g)
		}
	}
}

// ── AUDIT-10: idempotency — calling twice yields same gap count ───────────────

func TestAuditSpecVsCode_Idempotent(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	makeSpecDir(t, root, "feature", map[string]string{
		"spec.md":  "# Spec\n",
		"tasks.md": "- [ ] Task A\n- [x] Task B\n",
	})
	r1 := auditSpecVsCode(root, "feature")
	r2 := auditSpecVsCode(root, "feature")
	if len(r1.Gaps) != len(r2.Gaps) {
		t.Errorf("idempotency: first call got %d gaps, second got %d", len(r1.Gaps), len(r2.Gaps))
	}
}

// ── AUDIT-11: extractAuthzRoles handles comma-separated values ────────────────

func TestExtractAuthzRoles_CommaSeparated(t *testing.T) {
	t.Parallel()
	roles := extractAuthzRoles("roles: admin, user, viewer")
	if len(roles) != 3 {
		t.Fatalf("expected 3 roles, got %d: %v", len(roles), roles)
	}
}

// ── AUDIT-12: extractAuthzRoles deduplicates ──────────────────────────────────

func TestExtractAuthzRoles_Dedup(t *testing.T) {
	t.Parallel()
	roles := extractAuthzRoles("role: admin\nrole: admin\nrole: user")
	seen := map[string]int{}
	for _, r := range roles {
		seen[r]++
	}
	for r, n := range seen {
		if n > 1 {
			t.Errorf("role %q appears %d times; expected deduplication", r, n)
		}
	}
}

// ── AUDIT-13: extractAuthzRoles empty input → nil slice ──────────────────────

func TestExtractAuthzRoles_Empty(t *testing.T) {
	t.Parallel()
	if roles := extractAuthzRoles(""); roles != nil {
		t.Errorf("expected nil for empty input, got %v", roles)
	}
}

// ── AUDIT-14: no authz_model in spec.yml → 0 authz gaps ─────────────────────

func TestAuditSpecVsCode_NoAuthzModel(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	makeSpecDir(t, root, "simple", map[string]string{
		"spec.md":  "# Spec\n",
		"spec.yml": "authz_model: \"\"\nevents_emitted: []\n",
	})
	res := auditSpecVsCode(root, "simple")
	for _, g := range res.Gaps {
		if g.Type == "authz-role-untested" {
			t.Errorf("unexpected authz gap when authz_model is empty: %+v", g)
		}
	}
}

// ── AUDIT-15: asterisk bullet tasks also detected ────────────────────────────

func TestCheckIncompleteTasks_AsteriskBullets(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	makeSpecDir(t, root, "feat", map[string]string{
		"spec.md":  "# Spec\n",
		"tasks.md": "* [x] Done\n* [ ] Not done\n",
	})
	res := auditSpecVsCode(root, "feat")
	if !res.HasBlockingGaps() {
		t.Error("expected blocking gap for asterisk-bullet unchecked task")
	}
}

// ── EVT-GAP-01: checkVerify with gap-laden spec dir emits blocking fail ───────

func TestCheckVerify_WithBlockingGap_Fails(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	makeSpecDir(t, root, "add-auth", map[string]string{
		"spec.md":  "# Spec\n",
		"tasks.md": "- [ ] Still pending\n",
	})
	cp := checkVerify(root, "add auth", nil)
	if cp.Status != "fail" {
		t.Errorf("expected status 'fail' with blocking gaps, got %q", cp.Status)
	}
	if cp.GapAudit == nil {
		t.Fatal("expected GapAudit to be populated")
	}
	if !cp.GapAudit.HasBlockingGaps() {
		t.Error("expected GapAudit.HasBlockingGaps() == true")
	}
}

// ── EVT-GAP-02: checkVerify with clean project → ok ──────────────────────────

func TestCheckVerify_CleanProject_Passes(t *testing.T) {
	t.Parallel()
	// Fresh temp dir: no .forge/specs/ at all → audit returns empty result.
	cp := checkVerify(t.TempDir(), "", nil)
	if cp.Status == "fail" {
		t.Errorf("expected checkVerify to pass on clean project, got fail: %s", cp.Detail)
	}
}

// ── EVT-GAP-03: gap.detected events emitted before ship event ─────────────────

func TestGapDetectedEventsEmittedBeforeShipEvent(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	makeSpecDir(t, root, "add-auth", map[string]string{
		"spec.md":  "# Spec\n",
		"tasks.md": "- [ ] Pending task\n",
	})

	var buf writerFunc
	var events []string
	buf = func(p []byte) (int, error) {
		events = append(events, string(p))
		return len(p), nil
	}

	opts := RunOptions{
		Root:        root,
		Description: "add auth",
		EventWriter: buf,
	}
	runWithOptions(opts)

	// Find gap.detected and ship.* events and verify ordering.
	gapIdx := -1
	shipIdx := -1
	for i, e := range events {
		if contains(e, "gap.detected") {
			gapIdx = i
		}
		if contains(e, "ship.failed") || contains(e, "ship.passed") {
			shipIdx = i
		}
	}
	if gapIdx == -1 {
		t.Error("expected at least one gap.detected event")
	}
	if shipIdx == -1 {
		t.Error("expected a ship.failed or ship.passed event")
	}
	if gapIdx != -1 && shipIdx != -1 && gapIdx > shipIdx {
		t.Errorf("gap.detected (idx %d) must come before ship event (idx %d)", gapIdx, shipIdx)
	}
}

// writerFunc implements io.Writer for test event capture.
type writerFunc func(p []byte) (int, error)

func (f writerFunc) Write(p []byte) (int, error) { return f(p) }
