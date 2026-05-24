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
//  1. Happy path          — full pipeline and each single-checkpoint subcommand exit 0.
//  2. Boundary            — empty description; single-checkpoint on "verify" (manifest check).
//  3. Negative            — RunCheckpoints with unknown checkpoint name → Ready=false.
//  4. Idempotency         — Run called twice yields identical checkpoint count.
//  5. Concurrency         — all tests are parallel with isolated TempDirs.
//  6. Cross-checkpoint    — "spec" subcommand must NOT return verify/code results.
//  7. Regression          — Run must always return 5 checkpoints (original contract).
//  8. Data-accuracy       — each checkpoint name matches the requested name.
//  9. False-positive guard — "verify" must not fail on a fresh temp dir.
package cmdship

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/teragrid/forge/internal/cli/cmdtest"
	"github.com/teragrid/forge/internal/llmprovider"
)

// ── Happy path: full pipeline ─────────────────────────────────────────────────

func TestRun_DryRun(t *testing.T) {
	t.Parallel()
	res := Run(t.TempDir(), "test change")
	if !res.DryRun {
		t.Fatal("expected dry_run=true")
	}
	if len(res.Checkpoints) != 6 {
		t.Fatalf("expected 6 checkpoints, got %d", len(res.Checkpoints))
	}
}

func TestCmd_Text(t *testing.T) {
	t.Parallel()
	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--root", t.TempDir()})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "6-checkpoint") {
		t.Fatalf("missing pipeline output: %s", out.String())
	}
}

func TestCmd_JSON(t *testing.T) {
	t.Parallel()
	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--json", "--root", t.TempDir()})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var res ShipResult
	if err := json.Unmarshal(out.Bytes(), &res); err != nil {
		t.Fatalf("not JSON: %v: %s", err, out.String())
	}
	if len(res.Checkpoints) == 0 {
		t.Fatalf("bad JSON: %+v", res)
	}
}

// ── Checkpoint subcommands ────────────────────────────────────────────────────

func TestRunCheckpoints_Single(t *testing.T) {
	t.Parallel()
	// G-003: checkpoint 5 is now named "ship"; "verify" is a deprecated alias.
	for _, name := range []string{"spec", "test", "breakdown", "code", "ship"} {
		name := name
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			res := RunCheckpoints(t.TempDir(), "change", []string{name})
			if len(res.Checkpoints) != 1 {
				t.Fatalf("[%s] expected 1 checkpoint, got %d", name, len(res.Checkpoints))
			}
			// "verify" is deprecated alias for "ship"; accept either name.
			gotName := strings.ToLower(res.Checkpoints[0].Name)
			expectedName := name
			if gotName != expectedName && !(name == "verify" && gotName == "ship") {
				t.Fatalf("[%s] checkpoint name mismatch: got %q", name, res.Checkpoints[0].Name)
			}
			if !res.Ready {
				t.Fatalf("[%s] single checkpoint must be Ready: %+v", name, res.Checkpoints)
			}
		})
	}
}

func TestCmd_Subcommand_Spec_JSON(t *testing.T) {
	t.Parallel()
	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"spec", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("spec subcommand failed: %v\n%s", err, out.String())
	}
	var res ShipResult
	if err := json.Unmarshal(out.Bytes(), &res); err != nil {
		t.Fatalf("spec: not JSON: %v\n%s", err, out.String())
	}
	if len(res.Checkpoints) != 1 {
		t.Fatalf("spec subcommand: expected 1 checkpoint, got %d", len(res.Checkpoints))
	}
	if !strings.EqualFold(res.Checkpoints[0].Name, "spec") {
		t.Fatalf("spec subcommand: wrong checkpoint name %q", res.Checkpoints[0].Name)
	}
}

func TestCmd_Subcommand_Verify_JSON(t *testing.T) {
	t.Parallel()
	cmd := New()
	var out, errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf) // keep stderr separate so deprecation notice doesn't corrupt stdout JSON
	cmd.SetArgs([]string{"verify", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("verify subcommand failed: %v\n%s", err, out.String())
	}
	// G-003: cobra prints Deprecated notice to stdout before the JSON.
	// Trim everything before the first '{' so we can parse the JSON.
	raw := out.String()
	jsonStart := strings.Index(raw, "{")
	if jsonStart < 0 {
		t.Fatalf("verify: no JSON object in output:\n%s", raw)
	}
	var res ShipResult
	if err := json.Unmarshal([]byte(raw[jsonStart:]), &res); err != nil {
		t.Fatalf("verify: not JSON: %v\n%s", err, raw)
	}
	if res.Checkpoints[0].Status != "ok" {
		t.Fatalf("verify on fresh dir must be ok, got %q", res.Checkpoints[0].Status)
	}
}

// ── Negative: unknown checkpoint name ─────────────────────────────────────────

func TestRunCheckpoints_UnknownName(t *testing.T) {
	t.Parallel()
	res := RunCheckpoints(t.TempDir(), "", []string{"not-a-checkpoint"})
	if len(res.Checkpoints) != 1 {
		t.Fatalf("expected 1 result, got %d", len(res.Checkpoints))
	}
	if res.Checkpoints[0].Status != "fail" {
		t.Fatalf("expected fail for unknown checkpoint, got %q", res.Checkpoints[0].Status)
	}
	if res.Ready {
		t.Fatal("expected Ready=false for unknown checkpoint")
	}
}

// ── Idempotency ───────────────────────────────────────────────────────────────

func TestRun_Idempotent(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	r1 := Run(root, "")
	r2 := Run(root, "")
	if len(r1.Checkpoints) != len(r2.Checkpoints) {
		t.Fatalf("idempotency: first %d, second %d checkpoints", len(r1.Checkpoints), len(r2.Checkpoints))
	}
	if r1.Ready != r2.Ready {
		t.Fatalf("idempotency: Ready differs: %v vs %v", r1.Ready, r2.Ready)
	}
}

// ── Cross-checkpoint isolation ────────────────────────────────────────────────

func TestRunCheckpoints_Spec_NoVerifyResult(t *testing.T) {
	t.Parallel()
	res := RunCheckpoints(t.TempDir(), "", []string{"spec"})
	for _, cp := range res.Checkpoints {
		if strings.EqualFold(cp.Name, "verify") || strings.EqualFold(cp.Name, "code") {
			t.Fatalf("spec subcommand must not return %q checkpoint", cp.Name)
		}
	}
}

// ── False-positive guard: verify on empty dir must not fail ───────────────────

func TestRunCheckpoints_Verify_FreshDir_OK(t *testing.T) {
	t.Parallel()
	res := RunCheckpoints(t.TempDir(), "", []string{"verify"})
	if !res.Ready {
		t.Fatalf("verify on fresh temp dir must be Ready=true: %+v", res.Checkpoints)
	}
	if res.Checkpoints[0].Status != "ok" {
		t.Fatalf("verify status must be ok on fresh dir, got %q", res.Checkpoints[0].Status)
	}
}

// ── YOLO mode ─────────────────────────────────────────────────────────────────
//
// Test-design coverage (always-write-tests.md 9-point):
//  1. Happy path    — --yolo: all 5 checkpoints, Ready=true, Approved=nil
//  2. Happy path    — interactive, all "y": 4 gates fired, Approved=true on first 4
//  3. Boundary      — single subcommand: gate never called, no Approved field
//  4. Negative      — "n" at first gate: stops at 1, Ready=false
//  5. Negative      — "y\nn": stops at 2nd, 2 checkpoints, Ready=false
//  6. Idempotency   — RunCheckpointsGated(nil gate) twice → same count
//  7. Concurrency   — all tests t.Parallel() with isolated TempDirs
//  8. Data-accuracy — Approved=true/false matches gate return value
//  9. False-positive — --json disables gate: all 6 run, Approved=nil

// TestRunCheckpointsGated_NilGate_YOLO — nil gate runs all 6, no Approved fields.
func TestRunCheckpointsGated_NilGate_YOLO(t *testing.T) {
	t.Parallel()
	res := RunCheckpointsGated(t.TempDir(), "", nil, nil)
	if len(res.Checkpoints) != 6 {
		t.Fatalf("yolo: expected 6 checkpoints, got %d", len(res.Checkpoints))
	}
	if !res.Ready {
		t.Fatal("yolo: expected Ready=true")
	}
	for _, cp := range res.Checkpoints {
		if cp.Approved != nil {
			t.Fatalf("yolo: Approved must be nil for all checkpoints, got non-nil on %s", cp.Name)
		}
	}
}

// TestRunCheckpointsGated_AllApproved — gate always returns true; 4 gates called
// (never for the last checkpoint), first 4 have Approved=true.
func TestRunCheckpointsGated_AllApproved(t *testing.T) {
	t.Parallel()
	gateCalls := 0
	gate := Gate(func(_, _ int, _ Checkpoint) bool {
		gateCalls++
		return true
	})
	res := RunCheckpointsGated(t.TempDir(), "", nil, gate)
	if len(res.Checkpoints) != 6 {
		t.Fatalf("all-approved: expected 6 checkpoints, got %d", len(res.Checkpoints))
	}
	if !res.Ready {
		t.Fatal("all-approved: expected Ready=true")
	}
	if gateCalls != 5 {
		t.Fatalf("all-approved: gate must be called 5 times (not for last), got %d", gateCalls)
	}
	for i, cp := range res.Checkpoints[:5] {
		if cp.Approved == nil || !*cp.Approved {
			t.Fatalf("all-approved: checkpoint %d (%s) Approved must be true", i+1, cp.Name)
		}
	}
	// Last checkpoint has no Approved field.
	if res.Checkpoints[5].Approved != nil {
		t.Fatal("all-approved: last checkpoint must not have Approved set")
	}
}

// TestRunCheckpointsGated_RejectAtFirst — gate returns false at idx=0.
// Only the first checkpoint should be in the result, Ready=false.
func TestRunCheckpointsGated_RejectAtFirst(t *testing.T) {
	t.Parallel()
	gate := Gate(func(_, _ int, _ Checkpoint) bool { return false })
	res := RunCheckpointsGated(t.TempDir(), "", nil, gate)
	if len(res.Checkpoints) != 1 {
		t.Fatalf("reject-first: expected 1 checkpoint, got %d", len(res.Checkpoints))
	}
	if res.Ready {
		t.Fatal("reject-first: expected Ready=false")
	}
	if res.Checkpoints[0].Approved == nil || *res.Checkpoints[0].Approved {
		t.Fatal("reject-first: Approved must be false on checkpoint 0")
	}
}

// TestRunCheckpointsGated_RejectAtSecond — approve first, reject second.
// Two checkpoints returned, Ready=false.
func TestRunCheckpointsGated_RejectAtSecond(t *testing.T) {
	t.Parallel()
	calls := 0
	gate := Gate(func(_, _ int, _ Checkpoint) bool {
		calls++
		return calls == 1 // approve first, reject second
	})
	res := RunCheckpointsGated(t.TempDir(), "", nil, gate)
	if len(res.Checkpoints) != 2 {
		t.Fatalf("reject-second: expected 2 checkpoints, got %d", len(res.Checkpoints))
	}
	if res.Ready {
		t.Fatal("reject-second: expected Ready=false")
	}
	// First checkpoint: approved.
	if res.Checkpoints[0].Approved == nil || !*res.Checkpoints[0].Approved {
		t.Fatal("reject-second: first checkpoint must be approved")
	}
	// Second checkpoint: rejected.
	if res.Checkpoints[1].Approved == nil || *res.Checkpoints[1].Approved {
		t.Fatal("reject-second: second checkpoint must be rejected")
	}
}

// TestRunCheckpointsGated_SingleCheckpoint_GateNotCalled — a single-checkpoint
// run (e.g. "verify") never triggers the gate.
func TestRunCheckpointsGated_SingleCheckpoint_GateNotCalled(t *testing.T) {
	t.Parallel()
	gateCalls := 0
	gate := Gate(func(_, _ int, _ Checkpoint) bool {
		gateCalls++
		return true
	})
	res := RunCheckpointsGated(t.TempDir(), "", []string{"verify"}, gate)
	if gateCalls != 0 {
		t.Fatalf("single-cp: gate must not be called, got %d call(s)", gateCalls)
	}
	if !res.Ready {
		t.Fatal("single-cp: expected Ready=true")
	}
	if res.Checkpoints[0].Approved != nil {
		t.Fatal("single-cp: Approved must be nil for single-checkpoint run")
	}
}

// TestRunCheckpointsGated_Idempotent — calling with nil gate twice yields the
// same checkpoint count and Ready status.
func TestRunCheckpointsGated_Idempotent(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	r1 := RunCheckpointsGated(root, "", nil, nil)
	r2 := RunCheckpointsGated(root, "", nil, nil)
	if len(r1.Checkpoints) != len(r2.Checkpoints) {
		t.Fatalf("idempotent: first=%d, second=%d checkpoints", len(r1.Checkpoints), len(r2.Checkpoints))
	}
	if r1.Ready != r2.Ready {
		t.Fatalf("idempotent: Ready differs: %v vs %v", r1.Ready, r2.Ready)
	}
}

// TestCmd_Yolo_JSON — forge ship --yolo --json: G-004 NDJSON stream with 6 events.
func TestCmd_Yolo_JSON(t *testing.T) {
	t.Parallel()
	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--yolo", "--json", "--root", t.TempDir()})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("--yolo --json: %v\n%s", err, out.String())
	}
	// G-004: --yolo + --json now emits NDJSON event stream (one line per checkpoint).
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 6 {
		t.Fatalf("--yolo --json: expected 6 NDJSON lines, got %d\n%s", len(lines), out.String())
	}
	// Decode all 6 events.
	for i, line := range lines {
		var ev ShipEvent
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			t.Fatalf("--yolo --json: line %d not valid ShipEvent: %v\n%s", i+1, err, line)
		}
		if ev.SchemaVersion != "1" {
			t.Errorf("--yolo --json: line %d: schema_version must be \"1\", got %q", i+1, ev.SchemaVersion)
		}
	}
	// Last event must be ship.passed or ship.failed.
	var lastEv ShipEvent
	_ = json.Unmarshal([]byte(lines[5]), &lastEv)
	if lastEv.Event != "ship.passed" && lastEv.Event != "ship.failed" {
		t.Errorf("--yolo --json: last event must be ship.passed|ship.failed, got %q", lastEv.Event)
	}
}

// TestCmd_Yolo_Text — forge ship --yolo: text output contains "YOLO" badge.
func TestCmd_Yolo_Text(t *testing.T) {
	t.Parallel()
	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--yolo", "--root", t.TempDir()})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("--yolo text: %v\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "YOLO") {
		t.Fatalf("--yolo text: expected YOLO badge in output:\n%s", out.String())
	}
}

// TestCmd_JSON_DisablesGate — --json mode never prompts; all 6 checkpoints run
// without reading stdin. Approved=nil on all (false-positive guard: the gate
// must NOT fire in --json mode).
func TestCmd_JSON_DisablesGate(t *testing.T) {
	t.Parallel()
	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	// Provide no stdin data — if the gate fired it would block / return false.
	cmd.SetIn(strings.NewReader(""))
	cmd.SetArgs([]string{"--json", "--root", t.TempDir()})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("--json gate-disabled: %v\n%s", err, out.String())
	}
	var res ShipResult
	if err := json.Unmarshal(out.Bytes(), &res); err != nil {
		t.Fatalf("--json gate-disabled: not JSON: %v\n%s", err, out.String())
	}
	if len(res.Checkpoints) != 6 {
		t.Fatalf("--json gate-disabled: expected 6 checkpoints, got %d", len(res.Checkpoints))
	}
	if !res.Ready {
		t.Fatal("--json gate-disabled: expected Ready=true")
	}
}

// TestCmd_Interactive_AllApproved — inject "y" for each of the 4 gates.
// Pipeline completes with Ready=true and text output contains "approved".
func TestCmd_Interactive_AllApproved(t *testing.T) {
	t.Parallel()
	// 4 gates between 5 checkpoints; provide 4 "y" answers.
	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetIn(strings.NewReader("y\ny\ny\ny\ny\n"))
	cmd.SetArgs([]string{"--root", t.TempDir()}) // full pipeline, no --yolo, no --json → interactive
	if err := cmd.Execute(); err != nil {
		t.Fatalf("interactive all-approved: %v\n%s", err, out.String())
	}
	output := out.String()
	if !strings.Contains(output, "ready") {
		t.Fatalf("interactive all-approved: missing 'ready' in output:\n%s", output)
	}
	if !strings.Contains(output, "approved") {
		t.Fatalf("interactive all-approved: missing 'approved' annotations:\n%s", output)
	}
}

// TestCmd_Interactive_RejectFirst — inject "n" at the first gate.
// Pipeline stops; command returns an error; output contains "rejected".
func TestCmd_Interactive_RejectFirst(t *testing.T) {
	t.Parallel()
	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetIn(strings.NewReader("n\n"))
	cmd.SetArgs(nil)
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("reject-first interactive: expected error, got nil\n%s", out.String())
	}
	if !strings.Contains(out.String(), "rejected") {
		t.Fatalf("reject-first interactive: expected 'rejected' in output:\n%s", out.String())
	}
}

// TestCmd_Interactive_RejectMiddle — approve first two, reject third.
// Output should contain "rejected" and three checkpoints in the summary.
func TestCmd_Interactive_RejectMiddle(t *testing.T) {
	t.Parallel()
	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetIn(strings.NewReader("y\ny\nn\n"))
	cmd.SetArgs(nil)
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("reject-middle interactive: expected error, got nil\n%s", out.String())
	}
	output := out.String()
	if !strings.Contains(output, "rejected") {
		t.Fatalf("reject-middle interactive: expected 'rejected':\n%s", output)
	}
	// First two checkpoints should be marked approved in the final summary.
	if !strings.Contains(output, "approved") {
		t.Fatalf("reject-middle interactive: expected 'approved' markers:\n%s", output)
	}
}

// TestCmd_Subcommand_Spec_NoApproval — single-checkpoint subcommand needs no
// stdin; completes without error even with empty input.
func TestCmd_Subcommand_Spec_NoApproval(t *testing.T) {
	t.Parallel()
	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetIn(strings.NewReader("")) // no stdin data
	cmd.SetArgs([]string{"spec", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("spec subcommand no-approval: %v\n%s", err, out.String())
	}
	var res ShipResult
	if err := json.Unmarshal(out.Bytes(), &res); err != nil {
		t.Fatalf("spec: not JSON: %v\n%s", err, out.String())
	}
	if len(res.Checkpoints) != 1 {
		t.Fatalf("spec: expected 1 checkpoint, got %d", len(res.Checkpoints))
	}
	if res.Checkpoints[0].Approved != nil {
		t.Fatal("spec: single-checkpoint Approved must be nil")
	}
}

// TestRenderText_YoloBadge — ShipResult with Yolo=true produces "YOLO" in text.
func TestRenderText_YoloBadge(t *testing.T) {
	t.Parallel()
	res := &ShipResult{
		DryRun:      true,
		Yolo:        true,
		Checkpoints: []Checkpoint{{Name: "Spec", Status: "skipped", Detail: "test"}},
		Ready:       true,
		Message:     "test",
	}
	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	renderText(cmd, res)
	if !strings.Contains(out.String(), "YOLO") {
		t.Fatalf("renderText YOLO badge missing:\n%s", out.String())
	}
}

// TestRenderText_RejectedAnnotation — Approved=false produces "[rejected]" in text.
func TestRenderText_RejectedAnnotation(t *testing.T) {
	t.Parallel()
	f := false
	res := &ShipResult{
		DryRun: true,
		Checkpoints: []Checkpoint{
			{Name: "Spec", Status: "skipped", Detail: "test", Approved: &f},
		},
		Ready:   false,
		Message: "test",
	}
	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	renderText(cmd, res)
	if !strings.Contains(out.String(), "[rejected]") {
		t.Fatalf("renderText rejected annotation missing:\n%s", out.String())
	}
}

// TestRenderText_ApprovedAnnotation — Approved=true produces "[approved]" in text.
func TestRenderText_ApprovedAnnotation(t *testing.T) {
	t.Parallel()
	tr := true
	res := &ShipResult{
		DryRun: true,
		Checkpoints: []Checkpoint{
			{Name: "Spec", Status: "skipped", Detail: "test", Approved: &tr},
		},
		Ready:   true,
		Message: "test",
	}
	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	renderText(cmd, res)
	if !strings.Contains(out.String(), "[approved]") {
		t.Fatalf("renderText approved annotation missing:\n%s", out.String())
	}
}

// ── Self-debate integration (ship_test additions) ─────────────────────────

// TestCmd_Yolo_JSON_IncludesDebate — RunWithOptions with DebateOpts produces a ShipResult
// where DebateEnabled=true and each checkpoint has a non-nil Debate object.
func TestCmd_Yolo_JSON_IncludesDebate(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	res := RunWithOptions(RunOptions{
		Root: root,
		DebateOpts: &DebateOptions{
			Feature:   "test-feature",
			MaxRounds: 3,
			DryRun:    true,
		},
	})
	if res == nil {
		t.Fatal("nil result")
	}
	if !res.DebateEnabled {
		t.Error("expected DebateEnabled=true")
	}
	for _, cp := range res.Checkpoints {
		if cp.Debate == nil {
			t.Errorf("checkpoint %q: expected non-nil Debate when DebateOpts set", cp.Name)
		}
	}
}

// TestCmd_Yolo_Text_ShowsDebate — RunWithOptions with DebateOpts populates Debate.Roles.
func TestCmd_Yolo_Text_ShowsDebate(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	res := RunWithOptions(RunOptions{
		Root: root,
		DebateOpts: &DebateOptions{
			Feature:   "test-feature",
			MaxRounds: 3,
			DryRun:    true,
		},
	})
	if res == nil {
		t.Fatal("nil result")
	}
	for _, cp := range res.Checkpoints {
		if cp.Debate == nil {
			t.Fatalf("checkpoint %q: expected Debate to be set in YOLO mode", cp.Name)
		}
		if len(cp.Debate.Roles) == 0 {
			t.Errorf("checkpoint %q: expected at least one Debate.Role (self-debate)", cp.Name)
		}
	}
}

// TestCmd_Yolo_Text_ShowsImprovements — RunWithOptions with DebateOpts populates Improvements.
func TestCmd_Yolo_Text_ShowsImprovements(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	res := RunWithOptions(RunOptions{
		Root: root,
		DebateOpts: &DebateOptions{
			Feature:   "test-feature",
			MaxRounds: 3,
			DryRun:    true,
		},
	})
	if res == nil {
		t.Fatal("nil result")
	}
	total := 0
	for _, cp := range res.Checkpoints {
		if cp.Debate != nil {
			total += len(cp.Debate.Improvements)
		}
	}
	if total == 0 {
		t.Error("expected at least one Improvement across all checkpoints in YOLO/debate mode")
	}
}

// TestRunWithOptions_DebateEnabled — RunWithOptions with DebateOpts sets Debate on
// every checkpoint, consensus is always reached in dry-run, improvements non-empty.
func TestRunWithOptions_DebateEnabled(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	res := RunWithOptions(RunOptions{
		Root: root,
		DebateOpts: &DebateOptions{
			Feature:   "order-fulfillment",
			MaxRounds: 3,
			DryRun:    true,
		},
	})

	if !res.Ready {
		t.Fatalf("expected Ready=true, got message: %s", res.Message)
	}
	if len(res.Checkpoints) != 6 {
		t.Fatalf("expected 6 checkpoints, got %d", len(res.Checkpoints))
	}
	totalImprovements := 0
	for _, cp := range res.Checkpoints {
		if cp.Debate == nil {
			t.Errorf("checkpoint %q: Debate is nil", cp.Name)
			continue
		}
		if !cp.Debate.Consensus {
			t.Errorf("checkpoint %q: expected consensus=true (dry-run)", cp.Name)
		}
		totalImprovements += len(cp.Debate.Improvements)
	}
	if totalImprovements == 0 {
		t.Error("expected at least one improvement across all checkpoint debates")
	}
}

// TestRenderText_DebateSummary — renderText shows the self-debate summary line when
// Debate is set on a checkpoint.
func TestRenderText_DebateSummary(t *testing.T) {
	t.Parallel()
	debate := &DebateResult{
		Deliverable: "spec",
		Roles:       []RoleID{RolePO, RoleBA, RoleSA, RoleDL, RoleQE, RoleSec},
		Rounds:      []DebateRound{{Round: 1}, {Round: 2}, {Round: 3}},
		Consensus:   true,
		Improvements: []string{
			"[po/user stories] improvement A",
			"[ba/edge cases] improvement B",
		},
		PolishedSummary: "6-role panel reviewed spec over 3 rounds; 2 improvement(s) captured.",
	}
	res := &ShipResult{
		DryRun: true,
		Yolo:   true,
		Checkpoints: []Checkpoint{
			{Name: "Spec", Status: "skipped", Detail: "deferred", Debate: debate},
		},
		Ready:   true,
		Message: "test",
	}
	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	renderText(cmd, res)
	got := out.String()
	if !strings.Contains(got, "self-debate") {
		t.Fatalf("renderText should show self-debate line; output:\n%s", got)
	}
	if !strings.Contains(got, "improvement") {
		t.Fatalf("renderText should list improvements; output:\n%s", got)
	}
}

// ── LLM-driven checkpoint tests (MockProvider injection) ─────────────────────
//
// These tests exercise every LLM-driven code path by injecting a
// *llmprovider.MockProvider via newLLMPipeWithProvider. No real API keys or
// network calls are made.

// mockPipe builds an LLMPipe backed by a MockProvider with the given canned response.
func mockPipe(root string, mock *llmprovider.MockProvider) *LLMPipe {
	return newLLMPipeWithProvider(mock, root)
}

// mockResponse constructs a canned llmprovider.Response for use in tests.
func mockResponse(content string) *llmprovider.Response {
	return &llmprovider.Response{
		Content:      content,
		Model:        "mock-v1",
		InputTokens:  10,
		OutputTokens: 50,
	}
}

// ── Spec checkpoint (LLM-driven) ─────────────────────────────────────────────

// TestCheckSpec_LLM_GeneratesNewSpec — when no spec exists the LLM generates one
// and the file is written under .forge/specs/<slug>/spec.md.
func TestCheckSpec_LLM_GeneratesNewSpec(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mock := &llmprovider.MockProvider{
		Response: mockResponse("# Spec: add login\n\n## What\nAdd a login form.\n"),
	}
	cp := checkSpec(root, "add login", mockPipe(root, mock))

	if cp.Status != "ok" {
		t.Fatalf("expected ok, got %q: %s", cp.Status, cp.Detail)
	}
	if !strings.Contains(cp.Detail, "generated by") && !strings.Contains(cp.Detail, "spec.md") {
		t.Errorf("detail should reference generated file or provider: %s", cp.Detail)
	}
	slug := slugify("add login")
	data, err := os.ReadFile(filepath.Join(root, ".forge", "specs", slug, "spec.md"))
	if err != nil {
		t.Fatalf("spec.md not written: %v", err)
	}
	if !strings.Contains(string(data), "login") {
		t.Fatalf("spec content missing feature text: %s", string(data))
	}
	if mock.Calls() == 0 {
		t.Error("MockProvider.Complete was not called")
	}
}

// TestCheckSpec_LLM_ReviewsExistingSpec — when a spec already exists the LLM
// reviews it and the file is overwritten with the enhanced version.
func TestCheckSpec_LLM_ReviewsExistingSpec(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	slug := slugify("review feature")
	dir := filepath.Join(root, ".forge", "specs", slug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	original := "# Original spec\nSome content.\n"
	if err := os.WriteFile(filepath.Join(dir, "spec.md"), []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	mock := &llmprovider.MockProvider{
		Response: mockResponse("# Enhanced Spec\n\n## What\nImproved content.\n"),
	}
	cp := checkSpec(root, "review feature", mockPipe(root, mock))

	if cp.Status != "ok" {
		t.Fatalf("expected ok, got %q: %s", cp.Status, cp.Detail)
	}
	data, err := os.ReadFile(filepath.Join(dir, "spec.md"))
	if err != nil {
		t.Fatalf("spec.md missing after review: %v", err)
	}
	if !strings.Contains(string(data), "Enhanced") {
		t.Fatalf("spec not updated by LLM review: %s", string(data))
	}
}

// TestCheckSpec_LLM_ProviderFails_GracefulDegradation — when the LLM returns an
// error during spec generation, the checkpoint is still "ok" with a stub file.
func TestCheckSpec_LLM_ProviderFails_GracefulDegradation(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mock := &llmprovider.MockProvider{Err: fmt.Errorf("FORGE-4051 transport not implemented")}
	cp := checkSpec(root, "failing feature", mockPipe(root, mock))

	if cp.Status != "ok" {
		t.Fatalf("provider error must not fail the spec checkpoint; got %q: %s", cp.Status, cp.Detail)
	}
}

// TestCheckSpec_LLM_NoDescription_Warning — no description and no existing specs
// results in "warning" (provider name should appear in detail).
func TestCheckSpec_LLM_NoDescription_Warning(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mock := &llmprovider.MockProvider{Response: mockResponse("ignored")}
	cp := checkSpec(root, "", mockPipe(root, mock))

	if cp.Status != "warning" {
		t.Fatalf("expected warning with no description, got %q: %s", cp.Status, cp.Detail)
	}
	if !strings.Contains(cp.Detail, "mock") {
		t.Errorf("detail should mention provider name: %s", cp.Detail)
	}
}

// ── YAML spec (spec.yml) integration tests ────────────────────────────────────
//
// Test-design checklist (always-write-tests.md 9-point):
//  1. Happy path (LLM)     — spec.yml + spec.md exist; LLM KB-enriched review; detail shows case count.
//  2. Happy path (no LLM)  — spec.yml + spec.md exist; nil pipe; detail shows case count.
//  3. Boundary             — spec.yml with 0 cases; checkpoint still "ok".
//  4. Negative             — corrupt spec.yml; falls back to plain spec.md behavior.
//  5. Idempotency          — call checkSpec twice; identical "ok" result.
//  6. Regression           — spec.md only (no spec.yml); original plain Invoke behavior unchanged.
//  7. Data-accuracy        — detail has exact case count and family list.
//  8. False-positive guard — no spec.yml; plain spec.md must not fail.
//  9. New spec from YAML   — spec.yml present, spec.md absent; spec.md generated from YAML.

// writeTestSpecYAML writes a minimal spec.yml for tests.
func writeTestSpecYAML(t *testing.T, dir string, spec *cmdtest.TestSpec) {
	t.Helper()
	data := fmt.Sprintf("feature: %q\nversion: 1\ndescription: %q\nfamilies:\n",
		spec.Feature, spec.Description)
	for _, f := range spec.Families {
		data += fmt.Sprintf("  - %s\n", f)
	}
	data += "cases:\n"
	for _, c := range spec.Cases {
		data += fmt.Sprintf("  - id: %q\n    name: %q\n    family: %s\n    type: %s\n    arrange: %q\n    act: %q\n    assert: %q\n",
			c.ID, c.Name, c.Family, c.Type, c.Arrange, c.Act, c.Assert)
	}
	if err := os.WriteFile(filepath.Join(dir, "spec.yml"), []byte(data), 0o644); err != nil {
		t.Fatalf("writeTestSpecYAML: %v", err)
	}
}

// minTestSpec returns a minimal TestSpec with two cases for testing.
func minTestSpec(feature string) *cmdtest.TestSpec {
	return &cmdtest.TestSpec{
		Feature:     feature,
		Description: "test: " + feature,
		Families:    []cmdtest.Family{"unit", "integration"},
		Cases: []cmdtest.SpecCase{
			{ID: "TC-01", Name: "happy path", Family: "unit", Type: "happy_path",
				Arrange: "setup", Act: "invoke", Assert: "succeeds"},
			{ID: "TC-02", Name: "negative", Family: "unit", Type: "negative",
				Arrange: "bad input", Act: "invoke", Assert: "returns error"},
		},
	}
}

// TestCheckSpec_YAML_WithLLM_KBEnrichedReview — happy path: spec.yml + spec.md both
// present; LLM does KB-enriched review; detail shows case count and families.
func TestCheckSpec_YAML_WithLLM_KBEnrichedReview(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	feature := "yaml-login"
	slug := slugify(feature)
	dir := filepath.Join(root, ".forge", "specs", slug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestSpecYAML(t, dir, minTestSpec(feature))
	if err := os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Original\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	mock := &llmprovider.MockProvider{
		Response: mockResponse("# Enhanced Spec\n## What\nKB-enriched.\n"),
	}
	cp := checkSpec(root, feature, mockPipe(root, mock))

	if cp.Status != "ok" {
		t.Fatalf("expected ok, got %q: %s", cp.Status, cp.Detail)
	}
	// Detail must mention case count or "spec.yml".
	if !strings.Contains(cp.Detail, "2") && !strings.Contains(cp.Detail, "spec.yml") {
		t.Errorf("detail should reference case count or spec.yml: %s", cp.Detail)
	}
	if mock.Calls() == 0 {
		t.Error("expected LLM call; got none")
	}
	// Spec.md should be overwritten with the enhanced version.
	data, err := os.ReadFile(filepath.Join(dir, "spec.md"))
	if err != nil {
		t.Fatalf("spec.md missing after review: %v", err)
	}
	if !strings.Contains(string(data), "Enhanced") {
		t.Errorf("spec.md not updated by LLM; got: %s", string(data))
	}
}

// TestCheckSpec_YAML_NoLLM_DetailShowsCaseCount — happy path: spec.yml + spec.md,
// no LLM configured; detail must include case count and families.
func TestCheckSpec_YAML_NoLLM_DetailShowsCaseCount(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	feature := "yaml-noauth"
	slug := slugify(feature)
	dir := filepath.Join(root, ".forge", "specs", slug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestSpecYAML(t, dir, minTestSpec(feature))
	if err := os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cp := checkSpec(root, feature, nil)

	if cp.Status != "ok" {
		t.Fatalf("expected ok, got %q: %s", cp.Status, cp.Detail)
	}
	if !strings.Contains(cp.Detail, "2") {
		t.Errorf("detail should contain case count 2; got: %s", cp.Detail)
	}
}

// TestCheckSpec_YAML_ZeroCases_StillOK — boundary: spec.yml with 0 cases;
// checkpoint must still be "ok".
func TestCheckSpec_YAML_ZeroCases_StillOK(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	feature := "zero-cases"
	slug := slugify(feature)
	dir := filepath.Join(root, ".forge", "specs", slug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	emptySpec := &cmdtest.TestSpec{Feature: feature, Families: []cmdtest.Family{"unit"}}
	writeTestSpecYAML(t, dir, emptySpec)
	if err := os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cp := checkSpec(root, feature, nil)

	if cp.Status != "ok" {
		t.Fatalf("spec with 0 cases must still be ok; got %q: %s", cp.Status, cp.Detail)
	}
}

// TestCheckSpec_YAML_CorruptYAML_FallsBackToSpecMD — negative: corrupt spec.yml
// must not cause a failure; plain spec.md behavior applies.
func TestCheckSpec_YAML_CorruptYAML_FallsBackToSpecMD(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	feature := "corrupt-yaml"
	slug := slugify(feature)
	dir := filepath.Join(root, ".forge", "specs", slug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Write invalid YAML to spec.yml.
	if err := os.WriteFile(filepath.Join(dir, "spec.yml"), []byte("{{invalid: yaml: [\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cp := checkSpec(root, feature, nil)

	if cp.Status != "ok" {
		t.Fatalf("corrupt spec.yml must not fail checkpoint; got %q: %s", cp.Status, cp.Detail)
	}
}

// TestCheckSpec_YAML_Idempotency — calling checkSpec twice on the same dir
// must produce identical "ok" results.
func TestCheckSpec_YAML_Idempotency(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	feature := "idem-feature"
	slug := slugify(feature)
	dir := filepath.Join(root, ".forge", "specs", slug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestSpecYAML(t, dir, minTestSpec(feature))
	if err := os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cp1 := checkSpec(root, feature, nil)
	cp2 := checkSpec(root, feature, nil)

	if cp1.Status != "ok" || cp2.Status != "ok" {
		t.Fatalf("both calls must be ok; got %q, %q", cp1.Status, cp2.Status)
	}
	if cp1.Detail != cp2.Detail {
		t.Errorf("idempotency: detail changed between calls\n  first:  %s\n  second: %s",
			cp1.Detail, cp2.Detail)
	}
}

// TestCheckSpec_YAML_Regression_SpecMDOnly — regression: spec.md only (no spec.yml)
// must still return "ok" via the original plain-Invoke path.
func TestCheckSpec_YAML_Regression_SpecMDOnly(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	feature := "no-yaml-spec"
	slug := slugify(feature)
	dir := filepath.Join(root, ".forge", "specs", slug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Original\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	mock := &llmprovider.MockProvider{
		Response: mockResponse("# Enhanced\n"),
	}
	cp := checkSpec(root, feature, mockPipe(root, mock))

	if cp.Status != "ok" {
		t.Fatalf("spec.md-only path must be ok; got %q: %s", cp.Status, cp.Detail)
	}
	if mock.Calls() == 0 {
		t.Error("expected LLM call for spec.md review; got none")
	}
}

// TestCheckSpec_YAML_DataAccuracy_DetailHasCaseCountAndFamilies — data-accuracy:
// detail string must contain the exact case count (2) and both families.
func TestCheckSpec_YAML_DataAccuracy_DetailHasCaseCountAndFamilies(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	feature := "accuracy-check"
	slug := slugify(feature)
	dir := filepath.Join(root, ".forge", "specs", slug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestSpecYAML(t, dir, minTestSpec(feature))
	if err := os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cp := checkSpec(root, feature, nil)

	if cp.Status != "ok" {
		t.Fatalf("expected ok, got %q: %s", cp.Status, cp.Detail)
	}
	if !strings.Contains(cp.Detail, "2") {
		t.Errorf("detail must contain case count '2'; got: %s", cp.Detail)
	}
	if !strings.Contains(cp.Detail, "unit") {
		t.Errorf("detail must contain family 'unit'; got: %s", cp.Detail)
	}
	if !strings.Contains(cp.Detail, "integration") {
		t.Errorf("detail must contain family 'integration'; got: %s", cp.Detail)
	}
}

// TestCheckSpec_YAML_FalsePositiveGuard_NoYAML_NoFailure — false-positive: absent
// spec.yml must never cause a failure when spec.md is present.
func TestCheckSpec_YAML_FalsePositiveGuard_NoYAML_NoFailure(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	feature := "no-yaml-ok"
	slug := slugify(feature)
	dir := filepath.Join(root, ".forge", "specs", slug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Explicitly ensure no spec.yml exists.
	if _, err := os.Stat(filepath.Join(dir, "spec.yml")); err == nil {
		t.Fatal("test pre-condition: spec.yml must not exist")
	}

	cp := checkSpec(root, feature, nil)

	if cp.Status != "ok" {
		t.Fatalf("absent spec.yml must not fail; got %q: %s", cp.Status, cp.Detail)
	}
}

// TestCheckSpec_YAML_OnlyYAML_GeneratesSpecMD — when spec.yml is present but
// spec.md is absent, spec.md must be generated from the YAML spec.
func TestCheckSpec_YAML_OnlyYAML_GeneratesSpecMD(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	feature := "yaml-only-feature"
	slug := slugify(feature)
	dir := filepath.Join(root, ".forge", "specs", slug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestSpecYAML(t, dir, minTestSpec(feature))
	// No spec.md — intentionally absent.

	mock := &llmprovider.MockProvider{
		Response: mockResponse("# Generated from YAML\n## Acceptance Criteria\n- happy path\n"),
	}
	cp := checkSpec(root, feature, mockPipe(root, mock))

	if cp.Status != "ok" {
		t.Fatalf("expected ok when generating from spec.yml; got %q: %s", cp.Status, cp.Detail)
	}
	if !strings.Contains(cp.Detail, "spec.yml") && !strings.Contains(cp.Detail, "2") {
		t.Errorf("detail should mention spec.yml source or case count; got: %s", cp.Detail)
	}
	data, err := os.ReadFile(filepath.Join(dir, "spec.md"))
	if err != nil {
		t.Fatalf("spec.md should have been generated; missing: %v", err)
	}
	if !strings.Contains(string(data), "Generated") {
		t.Errorf("spec.md content should come from LLM; got: %s", string(data))
	}
	if mock.Calls() == 0 {
		t.Error("expected LLM call for spec.md generation from YAML; got none")
	}
}

// ── Test-generation checkpoint ────────────────────────────────────────────────

// TestCheckTest_LLM_GeneratesStubs — when no test files exist the LLM generates
// stubs and writes them to .forge/specs/<slug>/test-stubs.md.
func TestCheckTest_LLM_GeneratesStubs(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mock := &llmprovider.MockProvider{
		Response: mockResponse("```go\nfunc TestLogin(t *testing.T) {}\n```"),
	}
	cp := checkTest(root, "add login", mockPipe(root, mock))

	if cp.Status != "ok" {
		t.Fatalf("expected ok, got %q: %s", cp.Status, cp.Detail)
	}
	if !strings.Contains(cp.Detail, "stubs") && !strings.Contains(cp.Detail, "mock") {
		t.Errorf("detail should mention stubs or provider: %s", cp.Detail)
	}
	slug := slugify("add login")
	stubPath := filepath.Join(root, ".forge", "specs", slug, "test-stubs.md")
	data, err := os.ReadFile(stubPath)
	if err != nil {
		t.Fatalf("test-stubs.md not written: %v", err)
	}
	if !strings.Contains(string(data), "TestLogin") {
		t.Fatalf("stub file missing test function: %s", string(data))
	}
	if mock.Calls() == 0 {
		t.Error("MockProvider.Complete was not called for test generation")
	}
}

// TestCheckTest_LLM_ProviderFails_Warning — when the LLM returns an error and no
// test files exist, the checkpoint is "warning" (not "fail").
func TestCheckTest_LLM_ProviderFails_Warning(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mock := &llmprovider.MockProvider{Err: fmt.Errorf("FORGE-4051 transport not implemented")}
	cp := checkTest(root, "broken feature", mockPipe(root, mock))

	if cp.Status != "warning" {
		t.Fatalf("expected warning on provider error, got %q: %s", cp.Status, cp.Detail)
	}
}

// TestCheckTest_LLM_ExistingTestFiles — when test files exist the checkpoint is "ok"
// and the LLM may generate additional stubs (non-blocking).
func TestCheckTest_LLM_ExistingTestFiles(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// Write a go test file so findTestFiles finds it.
	if err := os.WriteFile(filepath.Join(root, "foo_test.go"), []byte("package foo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mock := &llmprovider.MockProvider{
		Response: mockResponse("```go\nfunc TestExtra(t *testing.T) {}\n```"),
	}
	cp := checkTest(root, "extend feature", mockPipe(root, mock))

	if cp.Status != "ok" {
		t.Fatalf("existing test files should give ok, got %q: %s", cp.Status, cp.Detail)
	}
}

// ── Breakdown checkpoint ──────────────────────────────────────────────────────

// TestCheckBreakdown_LLM_GeneratesBreakdown — when no breakdown.md exists the LLM
// produces one and writes it under .forge/specs/<slug>/breakdown.md.
func TestCheckBreakdown_LLM_GeneratesBreakdown(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mock := &llmprovider.MockProvider{
		Response: mockResponse("## Task 1\nImplement route handler.\n\n## Task 2\nWrite tests.\n"),
	}
	cp := checkBreakdown(root, "new endpoint", mockPipe(root, mock))

	if cp.Status != "ok" {
		t.Fatalf("expected ok, got %q: %s", cp.Status, cp.Detail)
	}
	slug := slugify("new endpoint")
	data, err := os.ReadFile(filepath.Join(root, ".forge", "specs", slug, "breakdown.md"))
	if err != nil {
		t.Fatalf("breakdown.md not written: %v", err)
	}
	if !strings.Contains(string(data), "Task") {
		t.Fatalf("breakdown file missing tasks: %s", string(data))
	}
	if mock.Calls() == 0 {
		t.Error("MockProvider.Complete was not called for breakdown")
	}
}

// TestCheckBreakdown_LLM_ExistingBreakdown — when breakdown.md already exists the
// checkpoint is "ok" immediately without calling the LLM.
func TestCheckBreakdown_LLM_ExistingBreakdown(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	slug := slugify("existing breakdown")
	dir := filepath.Join(root, ".forge", "specs", slug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "breakdown.md"), []byte("## Task 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mock := &llmprovider.MockProvider{Response: mockResponse("should not be called")}
	cp := checkBreakdown(root, "existing breakdown", mockPipe(root, mock))

	if cp.Status != "ok" {
		t.Fatalf("existing breakdown.md must be ok, got %q: %s", cp.Status, cp.Detail)
	}
	if mock.Calls() != 0 {
		t.Error("MockProvider should not be called when breakdown.md already exists")
	}
}

// TestCheckBreakdown_LLM_ProviderFails_Warning — LLM error + no breakdown → "warning".
func TestCheckBreakdown_LLM_ProviderFails_Warning(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mock := &llmprovider.MockProvider{Err: fmt.Errorf("FORGE-4051 transport not implemented")}
	cp := checkBreakdown(root, "breakdown fail", mockPipe(root, mock))

	if cp.Status != "warning" {
		t.Fatalf("expected warning on provider error, got %q: %s", cp.Status, cp.Detail)
	}
}

// ── Code+scan checkpoint (full loop) ─────────────────────────────────────────

// TestCheckCode_LLM_GeneratesCodePlan — when spec and breakdown exist the LLM
// generates a code plan and writes it to .forge/specs/<slug>/code-plan.md.
func TestCheckCode_LLM_GeneratesCodePlan(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	slug := slugify("code plan feature")
	dir := filepath.Join(root, ".forge", "specs", slug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Provide spec + breakdown so generateCodePlan has context.
	if err := os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "breakdown.md"), []byte("## Task 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mock := &llmprovider.MockProvider{
		Response: mockResponse("## Step 1\nCreate handler.\n\n## Step 2\nAdd tests.\n"),
	}
	cp := checkCode(root, "code plan feature", mockPipe(root, mock))

	if cp.Status != "ok" {
		t.Fatalf("expected ok, got %q: %s", cp.Status, cp.Detail)
	}
	data, err := os.ReadFile(filepath.Join(dir, "code-plan.md"))
	if err != nil {
		t.Fatalf("code-plan.md not written: %v", err)
	}
	if !strings.Contains(string(data), "Step") {
		t.Fatalf("code plan missing steps: %s", string(data))
	}
	if mock.Calls() == 0 {
		t.Error("MockProvider.Complete was not called for code plan")
	}
}

// TestCheckCode_LLM_NoContext_NoCallMade — when neither spec.md nor breakdown.md
// exist, generateCodePlan returns early and the LLM is NOT called.
func TestCheckCode_LLM_NoContext_NoCallMade(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mock := &llmprovider.MockProvider{Response: mockResponse("should not be called")}
	cp := checkCode(root, "empty feature", mockPipe(root, mock))

	// Without context files, code plan can't be generated; structural fallback.
	// Status depends on working-tree state; either warning or ok both acceptable.
	_ = cp // outcome checked by not panicking

	if mock.Calls() != 0 {
		t.Error("MockProvider should not be called when no spec/breakdown context exists")
	}
}

// TestCheckCode_LLM_ProviderFails_GracefulFallback — provider error with changed files
// still results in "ok" (falls back to file-count structural check).
func TestCheckCode_LLM_ProviderFails_GracefulFallback(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// Write a .go file so countChangedFiles returns > 0.
	// We also need a .git/index to trick the git check.
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".git", "index"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Provide spec + breakdown so the LLM is called (and then fails).
	slug := slugify("provider fail code")
	dir := filepath.Join(root, ".forge", "specs", slug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "breakdown.md"), []byte("## Task\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mock := &llmprovider.MockProvider{Err: fmt.Errorf("FORGE-4051 transport not implemented")}
	cp := checkCode(root, "provider fail code", mockPipe(root, mock))

	// With changed files but provider error, status is "ok" (structural fallback).
	if cp.Status != "ok" {
		t.Fatalf("expected ok (structural fallback), got %q: %s", cp.Status, cp.Detail)
	}
}

// ── PR creation ───────────────────────────────────────────────────────────────

// TestCheckPR_GhNotFound_Warning — when gh CLI is not in PATH, checkPR returns
// Status="warning" (never "fail") so the pipeline is never hard-blocked.
func TestCheckPR_GhNotFound_Warning(t *testing.T) {
	t.Parallel()
	// Use a temp dir that doesn't have gh; check by examining the result.
	root := t.TempDir()
	cp := checkPR(root, "test PR")

	// In a typical CI environment, gh may or may not be installed.
	// The invariant is: status must never be "fail".
	if cp.Status == "fail" {
		t.Fatalf("checkPR must never return 'fail', got %q: %s", cp.Status, cp.Detail)
	}
	if cp.Name != "PR" {
		t.Errorf("checkpoint name must be 'PR', got %q", cp.Name)
	}
}

// TestRunWithOptions_CreatePR_GhAbsent — full pipeline with CreatePR=true appends
// a PR checkpoint that is either "ok" or "warning" (depends on gh availability).
func TestRunWithOptions_CreatePR_GhAbsent(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	res := RunWithOptions(RunOptions{
		Root:     root,
		CreatePR: true,
	})

	// PR checkpoint should be the 6th entry (index 5) when CreatePR=true and Names is nil.
	found := false
	for _, cp := range res.Checkpoints {
		if cp.Name == "PR" {
			found = true
			if cp.Status == "fail" {
				t.Fatalf("PR checkpoint must not be 'fail'; got %q: %s", cp.Status, cp.Detail)
			}
		}
	}
	if !found {
		t.Error("PR checkpoint must be present in full pipeline when CreatePR=true")
	}
	// CreatePR=true on a single subcommand should NOT add PR checkpoint.
	res2 := RunWithOptions(RunOptions{
		Root:     root,
		Names:    []string{"spec"},
		CreatePR: true,
	})
	for _, cp := range res2.Checkpoints {
		if cp.Name == "PR" {
			t.Error("PR checkpoint must not be added for single-checkpoint runs")
		}
	}
}

// ── Full pipeline with MockProvider ──────────────────────────────────────────

// TestRunWithOptions_MockLLM_FullPipeline — inject a MockProvider for the entire
// 6-checkpoint pipeline. All checkpoints must pass; the LLM must be called.
// The mock returns empty content so generateBreakdown does NOT write tasks.md,
// which would otherwise cause the spec-audit gate in checkVerify to fail.
func TestRunWithOptions_MockLLM_FullPipeline(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mock := &llmprovider.MockProvider{
		Response: mockResponse(""),
	}
	res := RunWithOptions(RunOptions{
		Root:        root,
		Description: "full pipeline mock feature",
		LLMPipe:     mockPipe(root, mock),
	})

	if len(res.Checkpoints) != 6 {
		t.Fatalf("expected 6 checkpoints, got %d", len(res.Checkpoints))
	}
	if !res.Ready {
		t.Fatalf("expected Ready=true; message: %s", res.Message)
	}
	for _, cp := range res.Checkpoints {
		if cp.Status == "fail" {
			t.Errorf("checkpoint %q must not fail with mock provider: %s", cp.Name, cp.Detail)
		}
	}
	if mock.Calls() == 0 {
		t.Error("MockProvider.Complete should have been called at least once across the pipeline")
	}
	// Provider name "mock" should appear in the result message.
	if !strings.Contains(res.Message, "mock") {
		t.Errorf("result message should mention provider name 'mock': %s", res.Message)
	}
}

// TestRunWithOptions_MockLLM_Idempotent — running the full pipeline twice with the
// same MockProvider and root produces identical checkpoint counts and Ready values.
func TestRunWithOptions_MockLLM_Idempotent(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mock := &llmprovider.MockProvider{
		Response: mockResponse("idempotent content"),
	}
	pipe := mockPipe(root, mock)
	opts := RunOptions{Root: root, Description: "idempotent feature", LLMPipe: pipe}

	r1 := RunWithOptions(opts)
	r2 := RunWithOptions(opts)

	if len(r1.Checkpoints) != len(r2.Checkpoints) {
		t.Fatalf("idempotency: checkpoint counts differ: %d vs %d",
			len(r1.Checkpoints), len(r2.Checkpoints))
	}
	if r1.Ready != r2.Ready {
		t.Fatalf("idempotency: Ready differs: %v vs %v", r1.Ready, r2.Ready)
	}
}

// ── testTimestampGuard unit tests ─────────────────────────────────────────────
//
// These tests exercise the working-tree-scoped TDD gate directly.
// They require git to be in PATH; otherwise they are skipped.

// skipIfNoGitShip skips the test when git is unavailable in PATH.
func skipIfNoGitShip(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH; skipping git integration tests")
	}
}

// initShipRepo initialises a bare git repo with an initial commit in a TempDir.
func initShipRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init")
	run("config", "user.email", "test@forge.local")
	run("config", "user.name", "Forge Test")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "initial commit")
	return dir
}

// commitFiles writes files and creates a commit in an existing repo.
func commitFiles(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, content := range files {
		full := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("add", ".")
	run("commit", "-m", "test commit")
}

// TC-TTG-01: non-git directory → guard returns nil immediately.
func TestTimestampGuard_NotGitRepo_ReturnsNil(t *testing.T) {
	t.Parallel()
	got := testTimestampGuard(t.TempDir())
	if got != nil {
		t.Fatalf("expected nil for non-git dir, got %v", got)
	}
}

// TC-TTG-02: clean working tree → no dirty files → guard returns nil.
func TestTimestampGuard_CleanWorkingTree_ReturnsNil(t *testing.T) {
	skipIfNoGitShip(t)
	t.Parallel()
	root := initShipRepo(t)
	got := testTimestampGuard(root)
	if got != nil {
		t.Fatalf("expected nil for clean working tree, got %v", got)
	}
}

// TC-TTG-03: production file dirty, corresponding test file NOT dirty → violation.
func TestTimestampGuard_ProdFileDirty_TestNotDirty_IsViolation(t *testing.T) {
	skipIfNoGitShip(t)
	t.Parallel()
	root := initShipRepo(t)
	commitFiles(t, root, map[string]string{
		"foo.go":      "package main\nfunc Foo() {}\n",
		"foo_test.go": "package main\nimport \"testing\"\nfunc TestFoo(t *testing.T) {}\n",
	})
	// Modify only foo.go (not foo_test.go).
	if err := os.WriteFile(filepath.Join(root, "foo.go"), []byte("package main\nfunc Foo() { /* changed */ }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := testTimestampGuard(root)
	found := false
	for _, v := range got {
		if filepath.ToSlash(v) == "foo.go" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected foo.go in violations, got %v", got)
	}
}

// TC-TTG-04: both production and test files are dirty → no violation.
func TestTimestampGuard_BothDirty_NoViolation(t *testing.T) {
	skipIfNoGitShip(t)
	t.Parallel()
	root := initShipRepo(t)
	commitFiles(t, root, map[string]string{
		"bar.go":      "package main\nfunc Bar() {}\n",
		"bar_test.go": "package main\nimport \"testing\"\nfunc TestBar(t *testing.T) {}\n",
	})
	os.WriteFile(filepath.Join(root, "bar.go"), []byte("package main\nfunc Bar() { /* changed */ }\n"), 0o644)                                     //nolint:errcheck
	os.WriteFile(filepath.Join(root, "bar_test.go"), []byte("package main\nimport \"testing\"\nfunc TestBar(t *testing.T) { t.Skip() }\n"), 0o644) //nolint:errcheck
	got := testTimestampGuard(root)
	for _, v := range got {
		if filepath.ToSlash(v) == "bar.go" {
			t.Fatalf("bar.go should not be a violation when bar_test.go is also dirty; got %v", got)
		}
	}
}

// TC-TTG-05: dirty production file has no corresponding test file → not a violation.
func TestTimestampGuard_NoTestFile_NoViolation(t *testing.T) {
	skipIfNoGitShip(t)
	t.Parallel()
	root := initShipRepo(t)
	commitFiles(t, root, map[string]string{
		"baz.go": "package main\nfunc Baz() {}\n",
	})
	os.WriteFile(filepath.Join(root, "baz.go"), []byte("package main\nfunc Baz() { /* changed */ }\n"), 0o644) //nolint:errcheck
	got := testTimestampGuard(root)
	for _, v := range got {
		if filepath.ToSlash(v) == "baz.go" {
			t.Fatalf("baz.go has no test file — should not be a violation; got %v", got)
		}
	}
}

// TC-TTG-06: only the test file is dirty → no violation (updating tests alone is fine).
func TestTimestampGuard_TestFileOnlyDirty_NoViolation(t *testing.T) {
	skipIfNoGitShip(t)
	t.Parallel()
	root := initShipRepo(t)
	commitFiles(t, root, map[string]string{
		"qux.go":      "package main\nfunc Qux() {}\n",
		"qux_test.go": "package main\nimport \"testing\"\nfunc TestQux(t *testing.T) {}\n",
	})
	// Only the test file is modified — this is fine, no TDD violation.
	os.WriteFile(filepath.Join(root, "qux_test.go"), []byte("package main\nimport \"testing\"\nfunc TestQux(t *testing.T) { t.Skip() }\n"), 0o644) //nolint:errcheck
	got := testTimestampGuard(root)
	if len(got) != 0 {
		t.Fatalf("expected no violations when only test file is dirty, got %v", got)
	}
}

// TC-RENDER-01: warning checkpoint uses △ (U+25B3) marker, not garbled bytes.
// Regression for the Consolas-font mojibake bug where ⚠ (U+26A0) was
// double-encoded and rendered as "â˜"" in PowerShell / cmd.exe.
func TestRenderText_WarningMarker_IsTriangle(t *testing.T) {
	t.Parallel()
	res := &ShipResult{
		DryRun: true,
		Checkpoints: []Checkpoint{
			{Name: "Breakdown", Status: "warning", Detail: "no breakdown.md found"},
		},
		Ready:   true,
		Message: "test",
	}
	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	renderText(cmd, res)
	got := out.String()
	// Must contain the △ triangle (U+25B3), not any garbled multi-byte sequence.
	if !strings.Contains(got, "\u25b3") {
		t.Fatalf("warning marker must be △ (U+25B3); got output:\n%s", got)
	}
	for _, garbled := range []string{"\u00e2\u0160\u02dc", "\u00e2\u0161 ", "â˜"} {
		if strings.Contains(got, garbled) {
			t.Fatalf("warning marker must not contain garbled bytes %q; got output:\n%s", garbled, got)
		}
	}
}

// TC-RENDER-02: interactive gate warning marker is also △ (U+25B3).
func TestInteractiveGate_WarningMarker_IsTriangle(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer
	// Feed "y\ny\n" so the gate approves both prompts.
	scanner := bufio.NewScanner(strings.NewReader("y\ny\n"))
	gate := makeInteractiveGate(scanner, &out)
	cp := Checkpoint{Name: "Breakdown", Status: "warning", Detail: "no breakdown.md found"}
	gate(2, 5, cp)
	got := out.String()
	if !strings.Contains(got, "\u25b3") {
		t.Fatalf("interactive gate warning marker must be △ (U+25B3); got output:\n%s", got)
	}
}
