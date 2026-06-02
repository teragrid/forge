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

package cmdship

// RFC-005 P3 test suite.
// Covers P3 features (async approval deferred to P4):
//   1. PII detection (pii_filter.go)
//   2. A/B steering (ab_steering.go)
//   3. Drift detection (drift_detect.go)
//   4. Compound checkpoints (compound_checkpoint.go)
//   5. Immutable audit trail (immutable_audit.go)
//   6. Incremental re-run (incremental_rerun.go)

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// ══════════════════════════════════════════════════════════════════════════════
// 1. PII Filter
// ══════════════════════════════════════════════════════════════════════════════

func TestPIIFilter_Scan_NoMatches(t *testing.T) {
	f := NewPIIFilter(PIIPolicyRedact)
	matches := f.Scan("No PII here. Just a normal sentence.")
	if len(matches) != 0 {
		t.Errorf("expected 0 matches, got %d: %+v", len(matches), matches)
	}
}

func TestPIIFilter_Scan_Email(t *testing.T) {
	f := NewPIIFilter(PIIPolicyRedact)
	text := "Contact alice@example.com for support."
	matches := f.Scan(text)
	found := false
	for _, m := range matches {
		if m.Category == "email" {
			found = true
		}
	}
	if !found {
		t.Error("expected email match, got none")
	}
}

func TestPIIFilter_Scan_SSN(t *testing.T) {
	f := NewPIIFilter(PIIPolicyRedact)
	matches := f.Scan("SSN: 123-45-6789 is sensitive.")
	found := false
	for _, m := range matches {
		if m.Category == "ssn" {
			found = true
		}
	}
	if !found {
		t.Error("expected ssn match")
	}
}

func TestPIIFilter_Apply_Redact(t *testing.T) {
	f := NewPIIFilter(PIIPolicyRedact)
	out, err := f.Apply("Send bill to bob@corp.org")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out == "Send bill to bob@corp.org" {
		t.Error("expected PII to be redacted")
	}
}

func TestPIIFilter_Apply_Block_WithPII(t *testing.T) {
	f := NewPIIFilter(PIIPolicyBlock)
	_, err := f.Apply("Call 555.123.4567 now")
	if !errors.Is(err, ErrPIIDetected) {
		t.Errorf("expected ErrPIIDetected, got %v", err)
	}
}

func TestPIIFilter_Apply_Block_NoMatch(t *testing.T) {
	f := NewPIIFilter(PIIPolicyBlock)
	out, err := f.Apply("No PII in this prompt.")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if out != "No PII in this prompt." {
		t.Errorf("expected unchanged text, got %q", out)
	}
}

func TestPIIFilter_Apply_Warn_RedactsButNoError(t *testing.T) {
	f := NewPIIFilter(PIIPolicyWarn)
	out, err := f.Apply("Patient: John Doe is admitted.")
	if err != nil {
		t.Fatalf("warn policy must not return error, got %v", err)
	}
	// Text should be redacted even in warn mode.
	if out == "Patient: John Doe is admitted." {
		t.Error("expected PII redacted in warn mode")
	}
}

func TestPIIFilter_Categories_Dedup(t *testing.T) {
	f := NewPIIFilter(PIIPolicyRedact)
	// Two emails → only one category entry.
	cats := f.Categories("a@b.com and c@d.com")
	count := 0
	for _, c := range cats {
		if c == "email" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected 1 deduplicated email category, got %d", count)
	}
}

// FalsePositive guard: private IPs must NOT be flagged as public ip PII.
func TestPIIFilter_FalsePositive_PrivateIPv4(t *testing.T) {
	f := NewPIIFilter(PIIPolicyBlock)
	_, err := f.Apply("Server at 192.168.1.1 is healthy.")
	if errors.Is(err, ErrPIIDetected) {
		t.Error("private IP 192.168.1.1 should not be flagged as public PII")
	}
}

// ══════════════════════════════════════════════════════════════════════════════
// 2. A/B Steering
// ══════════════════════════════════════════════════════════════════════════════

func TestRecordABRun_WritesJSONL(t *testing.T) {
	root := t.TempDir()
	rec := ABRunRecord{
		ExperimentName: "arch-style",
		Checkpoint:     "arch",
		RunAt:          time.Now().UTC(),
		Winner:         ABVariantA,
		ScoreA:         8.5,
		ScoreB:         7.0,
	}
	if err := RecordABRun(root, rec); err != nil {
		t.Fatalf("RecordABRun: %v", err)
	}
	// File must exist.
	path := abExperimentPath(root, "arch-style")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected JSONL file at %s: %v", path, err)
	}
}

func TestGetABReport_Empty(t *testing.T) {
	root := t.TempDir()
	rep, err := GetABReport(root, "no-such-experiment")
	if err != nil {
		t.Fatalf("GetABReport: %v", err)
	}
	if rep.TotalRuns != 0 {
		t.Errorf("expected 0 runs, got %d", rep.TotalRuns)
	}
}

func TestGetABReport_Aggregate(t *testing.T) {
	root := t.TempDir()
	name := "test-aggr"
	runs := []ABRunRecord{
		{ExperimentName: name, Winner: ABVariantA, ScoreA: 9, ScoreB: 7, RunAt: time.Now()},
		{ExperimentName: name, Winner: ABVariantB, ScoreA: 6, ScoreB: 8, RunAt: time.Now()},
		{ExperimentName: name, Winner: "tie", ScoreA: 7, ScoreB: 7, RunAt: time.Now()},
		{ExperimentName: name, Winner: "inconclusive", ScoreA: 0, ScoreB: 0, RunAt: time.Now()},
	}
	for _, r := range runs {
		if err := RecordABRun(root, r); err != nil {
			t.Fatalf("RecordABRun: %v", err)
		}
	}
	rep, err := GetABReport(root, name)
	if err != nil {
		t.Fatalf("GetABReport: %v", err)
	}
	if rep.TotalRuns != 4 {
		t.Errorf("expected 4 total runs, got %d", rep.TotalRuns)
	}
	if rep.WinsA != 1 {
		t.Errorf("expected 1 win for A, got %d", rep.WinsA)
	}
	if rep.WinsB != 1 {
		t.Errorf("expected 1 win for B, got %d", rep.WinsB)
	}
	if rep.Ties != 1 {
		t.Errorf("expected 1 tie, got %d", rep.Ties)
	}
	if rep.Inconclusive != 1 {
		t.Errorf("expected 1 inconclusive, got %d", rep.Inconclusive)
	}
}

func TestRunABExperiment_NilPipe_Inconclusive(t *testing.T) {
	root := t.TempDir()
	def := ABExperimentDef{Name: "nil-test", Checkpoint: "arch", VariantA: "A", VariantB: "B"}
	winner, err := RunABExperiment(root, def, nil, "base", "prompt", 100,
		func(_ string) float64 { return 5.0 })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if winner != "inconclusive" {
		t.Errorf("expected inconclusive with nil pipe, got %s", winner)
	}
	rep, _ := GetABReport(root, "nil-test")
	if rep.TotalRuns != 1 {
		t.Errorf("expected run to be recorded")
	}
}

// ══════════════════════════════════════════════════════════════════════════════
// 4. Drift Detection
// ══════════════════════════════════════════════════════════════════════════════

func TestDetectDrift_NoSpecMD_Clean(t *testing.T) {
	root := t.TempDir()
	report, err := DetectDrift(root, "no-such-slug", false)
	if err != nil {
		t.Fatalf("DetectDrift on absent spec: %v", err)
	}
	if !report.Clean {
		t.Error("expected clean report when spec.md absent")
	}
}

func TestDetectDrift_CleanSpec_NoSignals(t *testing.T) {
	root := t.TempDir()
	slug := "clean-feat"
	specDir := filepath.Join(root, ".forge", "specs", slug)
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "# Spec\n- [ ] AC-001: thing works\n- [ ] AC-002: other\n"
	if err := os.WriteFile(filepath.Join(specDir, "spec.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	report, err := DetectDrift(root, slug, false)
	if err != nil {
		t.Fatalf("DetectDrift: %v", err)
	}
	// No tasks.md, no baseline → only clean if no blocking signals.
	if report.BlockCount > 0 {
		t.Errorf("unexpected blocking signals: %+v", report.Signals)
	}
}

func TestDetectDrift_TaskCompleteness_Blocking(t *testing.T) {
	root := t.TempDir()
	slug := "tasks-feat"
	specDir := filepath.Join(root, ".forge", "specs", slug)
	snapDir := filepath.Join(root, ".forge", snapshotsBaseDir, slug, "code")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(snapDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Write tasks.md with unchecked item.
	if err := os.WriteFile(filepath.Join(specDir, "tasks.md"),
		[]byte("- [ ] T-001 implement thing\n- [x] T-002 done\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Write spec.md modified AFTER the snapshot dir (simulate drift).
	time.Sleep(5 * time.Millisecond)
	if err := os.WriteFile(filepath.Join(specDir, "spec.md"),
		[]byte("# Spec\n- [ ] AC-001: works\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	report, err := DetectDrift(root, slug, false)
	if err != nil {
		t.Fatalf("DetectDrift: %v", err)
	}
	found := false
	for _, s := range report.Signals {
		if s.Kind == "task-completeness" && s.Severity == "blocking" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected blocking task-completeness signal, signals: %+v", report.Signals)
	}
}

func TestDetectDrift_FailOnBlock(t *testing.T) {
	root := t.TempDir()
	slug := "block-feat"
	specDir := filepath.Join(root, ".forge", "specs", slug)
	snapDir := filepath.Join(root, ".forge", snapshotsBaseDir, slug, "code")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(snapDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "tasks.md"),
		[]byte("- [ ] T-001 thing\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	time.Sleep(5 * time.Millisecond)
	if err := os.WriteFile(filepath.Join(specDir, "spec.md"),
		[]byte("# Spec\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := DetectDrift(root, slug, true)
	if !errors.Is(err, ErrDriftDetected) {
		t.Errorf("expected ErrDriftDetected, got %v", err)
	}
}

func TestDetectDrift_ACCountChange(t *testing.T) {
	root := t.TempDir()
	slug := "ac-change"
	specDir := filepath.Join(root, ".forge", "specs", slug)
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Write baseline with 2 ACs.
	baseline := DriftBaseline{
		Slug: slug, RecordedAt: time.Now().UTC(), ACCount: 2,
	}
	if err := writeJSON(filepath.Join(specDir, "drift-baseline.json"), baseline); err != nil {
		t.Fatal(err)
	}
	// Write spec.md with 3 ACs (drift).
	if err := os.WriteFile(filepath.Join(specDir, "spec.md"),
		[]byte("- [ ] AC-001\n- [ ] AC-002\n- [ ] AC-003\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	report, err := DetectDrift(root, slug, false)
	if err != nil {
		t.Fatalf("DetectDrift: %v", err)
	}
	found := false
	for _, s := range report.Signals {
		if s.Kind == "ac-count" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected ac-count signal, signals: %+v", report.Signals)
	}
}

func TestSaveDriftBaseline(t *testing.T) {
	root := t.TempDir()
	slug := "baseline-feat"
	specDir := filepath.Join(root, ".forge", "specs", slug)
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "spec.md"),
		[]byte("- [ ] AC-001\n- [x] AC-002\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := SaveDriftBaseline(root, slug); err != nil {
		t.Fatalf("SaveDriftBaseline: %v", err)
	}
	bl, err := readDriftBaseline(specDir)
	if err != nil {
		t.Fatalf("readDriftBaseline: %v", err)
	}
	if bl.ACCount != 2 {
		t.Errorf("expected ACCount=2, got %d", bl.ACCount)
	}
}

// ══════════════════════════════════════════════════════════════════════════════
// 5. Compound Checkpoints
// ══════════════════════════════════════════════════════════════════════════════

func TestCompoundRegistry_BuiltinQuick(t *testing.T) {
	r := NewCompoundRegistry()
	cps, err := r.Expand("quick")
	if err != nil {
		t.Fatalf("Expand quick: %v", err)
	}
	if len(cps) != 2 || cps[0] != "spec" || cps[1] != "arch" {
		t.Errorf("expected [spec arch], got %v", cps)
	}
}

func TestCompoundRegistry_Canonical_Passthrough(t *testing.T) {
	r := NewCompoundRegistry()
	cps, err := r.Expand("test")
	if err != nil {
		t.Fatalf("Expand test: %v", err)
	}
	if len(cps) != 1 || cps[0] != "test" {
		t.Errorf("expected [test], got %v", cps)
	}
}

func TestCompoundRegistry_Register_UserDefined(t *testing.T) {
	r := NewCompoundRegistry()
	if err := r.Register("my-compound", []string{"spec", "test"}); err != nil {
		t.Fatalf("Register: %v", err)
	}
	cps, err := r.Expand("my-compound")
	if err != nil {
		t.Fatalf("Expand my-compound: %v", err)
	}
	if len(cps) != 2 {
		t.Errorf("expected 2 checkpoints, got %v", cps)
	}
}

func TestCompoundRegistry_ExpandCheckpoints_Dedup(t *testing.T) {
	r := NewCompoundRegistry()
	// "quick" = [spec, arch]; "spec" is already in quick → spec appears once.
	cps, err := r.ExpandCheckpoints([]string{"quick", "spec"})
	if err != nil {
		t.Fatalf("ExpandCheckpoints: %v", err)
	}
	count := 0
	for _, c := range cps {
		if c == "spec" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected spec once, got %d times in %v", count, cps)
	}
}

func TestCompoundRegistry_CycleDetection(t *testing.T) {
	r := NewCompoundRegistry()
	// Register A → B, then B → A (cycle).
	_ = r.Register("A", []string{"spec", "arch"})
	// Direct self-reference cycle via name expansion.
	err := r.Register("B", []string{"A", "B"})
	if err == nil {
		t.Error("expected cycle detection error")
	}
}

func TestCompoundRegistry_UnknownCheckpoint(t *testing.T) {
	r := NewCompoundRegistry()
	_, err := r.Expand("nonexistent-checkpoint")
	if err == nil {
		t.Error("expected error for unknown checkpoint")
	}
}

func TestCompoundRegistry_BuiltinFull(t *testing.T) {
	r := NewCompoundRegistry()
	cps, err := r.Expand("full")
	if err != nil {
		t.Fatalf("Expand full: %v", err)
	}
	if len(cps) != len(canonicalCheckpoints) {
		t.Errorf("expected %d checkpoints, got %d: %v", len(canonicalCheckpoints), len(cps), cps)
	}
}

// ══════════════════════════════════════════════════════════════════════════════
// 6. Immutable Audit Trail
// ══════════════════════════════════════════════════════════════════════════════

func TestVerifyLedger_EmptyFile(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "audit.log")
	result, err := VerifyLedger(path)
	if err != nil {
		t.Fatalf("VerifyLedger on nonexistent file: %v", err)
	}
	if result.Tampered {
		t.Error("empty/absent ledger must not be tampered")
	}
	if result.TotalEntries != 0 {
		t.Errorf("expected 0 entries, got %d", result.TotalEntries)
	}
}

func TestVerifyLedger_ValidChain(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "audit.log")
	writeAuditChain(t, path, 3)
	result, err := VerifyLedger(path)
	if err != nil {
		t.Fatalf("VerifyLedger: %v", err)
	}
	if result.Tampered {
		t.Errorf("valid chain reported tampered; breaks: %v", result.Breaks)
	}
	if result.TotalEntries != 3 {
		t.Errorf("expected 3 entries, got %d", result.TotalEntries)
	}
}

func TestVerifyLedger_TamperedEntry(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "audit.log")
	writeAuditChain(t, path, 2)

	// Corrupt the file by appending a bad entry.
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	// Write a valid-looking entry but with wrong hash/prevhash.
	_, _ = f.WriteString(`{"ts":"2026-01-01T00:00:00Z","verb":"hack","action":"tamper","prev_hash":"bad","hash":"bad"}` + "\n")
	f.Close()

	result, err := VerifyLedger(path)
	if err != nil {
		t.Fatalf("VerifyLedger: %v", err)
	}
	if !result.Tampered {
		t.Error("expected tampered=true after injecting bad entry")
	}
}

func TestSealAndVerify_RoundTrip(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "audit.log")
	writeAuditChain(t, path, 2)

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	if err := SealLedger(path, priv); err != nil {
		t.Fatalf("SealLedger: %v", err)
	}
	if err := VerifySeal(path, pub); err != nil {
		t.Errorf("VerifySeal: %v", err)
	}
}

func TestVerifySeal_BadKey(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "audit.log")
	writeAuditChain(t, path, 1)

	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	wrongPub, _, _ := ed25519.GenerateKey(rand.Reader)

	_ = SealLedger(path, priv)
	if err := VerifySeal(path, wrongPub); err == nil {
		t.Error("expected signature verification failure with wrong public key")
	}
}

// writeAuditChain writes n properly chained audit entries to path.
func writeAuditChain(t *testing.T, path string, n int) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("writeAuditChain create: %v", err)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	prev := ""
	for i := 0; i < n; i++ {
		e := auditEntry{
			Timestamp: time.Now().UTC().Add(time.Duration(i) * time.Second),
			Verb:      "ship",
			Action:    "checkpoint",
			Actor:     "test",
			PrevHash:  prev,
		}
		e.Hash = computeAuditHash(e)
		if err := enc.Encode(e); err != nil {
			t.Fatalf("writeAuditChain encode %d: %v", i, err)
		}
		prev = e.Hash
	}
}

// ══════════════════════════════════════════════════════════════════════════════
// 7. Incremental Re-run
// ══════════════════════════════════════════════════════════════════════════════

func TestIncrementalPlan_NoBaseline_FullRun(t *testing.T) {
	root := t.TempDir()
	plan, err := IncrementalPlan(root, "feat", false)
	if err != nil {
		t.Fatalf("IncrementalPlan: %v", err)
	}
	if plan.BaselineFound {
		t.Error("expected no baseline found")
	}
	if len(plan.Rerun) != len(canonicalCheckpoints) {
		t.Errorf("expected all %d checkpoints, got %d", len(canonicalCheckpoints), len(plan.Rerun))
	}
}

func TestIncrementalPlan_NoBaseline_StrictMode(t *testing.T) {
	root := t.TempDir()
	_, err := IncrementalPlan(root, "feat", true)
	if !errors.Is(err, ErrIncrementalNoBaseline) {
		t.Errorf("expected ErrIncrementalNoBaseline, got %v", err)
	}
}

func TestIncrementalPlan_AllSnapshotsPresent_NoChanges(t *testing.T) {
	root := t.TempDir()
	slug := "stable"
	specDir := filepath.Join(root, ".forge", "specs", slug)
	snapBase := filepath.Join(root, ".forge", snapshotsBaseDir, slug)

	// Create snapshot directories for all checkpoints (simulate a prior full run).
	for _, cp := range canonicalCheckpoints {
		if err := os.MkdirAll(filepath.Join(snapBase, cp), 0o755); err != nil {
			t.Fatal(err)
		}
		// Snapshot dir newer than any spec file → no drift.
		// Touch spec.md before snapshot dirs.
	}
	// Write spec.md BEFORE creating snapshots (snapshots are newer).
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "spec.md"),
		[]byte("# spec"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Re-touch snapshots to ensure they are newer.
	for _, cp := range canonicalCheckpoints {
		d := filepath.Join(snapBase, cp)
		now := time.Now().Add(time.Second)
		if err := os.Chtimes(d, now, now); err != nil {
			t.Fatal(err)
		}
	}

	plan, err := IncrementalPlan(root, slug, false)
	if err != nil {
		t.Fatalf("IncrementalPlan: %v", err)
	}
	if !plan.BaselineFound {
		t.Error("expected baseline found")
	}
	// With no file changes, all checkpoints should be skippable.
	if len(plan.Rerun) != 0 {
		t.Errorf("expected 0 checkpoints to re-run, got %v", plan.Rerun)
	}
}

func TestIncrementalPlan_SpecChanged_PropagatesDown(t *testing.T) {
	root := t.TempDir()
	slug := "drift"
	specDir := filepath.Join(root, ".forge", "specs", slug)
	snapBase := filepath.Join(root, ".forge", snapshotsBaseDir, slug)

	// Create all snapshot directories first (old timestamp).
	past := time.Now().Add(-time.Hour)
	for _, cp := range canonicalCheckpoints {
		d := filepath.Join(snapBase, cp)
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(d, past, past); err != nil {
			t.Fatal(err)
		}
	}

	// Write spec.md NOW (newer than snapshots).
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "spec.md"),
		[]byte("# updated spec"), 0o644); err != nil {
		t.Fatal(err)
	}

	plan, err := IncrementalPlan(root, slug, false)
	if err != nil {
		t.Fatalf("IncrementalPlan: %v", err)
	}
	// spec changed → spec, arch, test, breakdown, code, ship, qa-verify all dirty.
	if len(plan.Rerun) == 0 {
		t.Error("expected spec change to dirty downstream checkpoints")
	}
	// spec must be in Rerun.
	found := false
	for _, cp := range plan.Rerun {
		if cp == "spec" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected spec in rerun list, got %v", plan.Rerun)
	}
}

func TestCheckpointNeedsRerun_NoBaseline(t *testing.T) {
	root := t.TempDir()
	needs, err := CheckpointNeedsRerun(root, "feat", "arch")
	if err != nil {
		t.Fatalf("CheckpointNeedsRerun: %v", err)
	}
	if !needs {
		t.Error("expected needs re-run when no baseline")
	}
}
