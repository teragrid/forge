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
//  1. Happy path          â€” full pipeline and each single-checkpoint subcommand exit 0.
//  2. Boundary            â€” empty description; single-checkpoint on "verify" (manifest check).
//  3. Negative            â€” RunCheckpoints with unknown checkpoint name â†’ Ready=false.
//  4. Idempotency         â€” Run called twice yields identical checkpoint count.
//  5. Concurrency         â€” all tests are parallel with isolated TempDirs.
//  6. Cross-checkpoint    â€” "spec" subcommand must NOT return verify/code results.
//  7. Regression          â€” Run must always return 5 checkpoints (original contract).
//  8. Data-accuracy       â€” each checkpoint name matches the requested name.
//  9. False-positive guard â€” "verify" must not fail on a fresh temp dir.
package cmdship

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/teragrid/forge/internal/agentbridge"
	"github.com/teragrid/forge/internal/cli/cmdtest"
	"github.com/teragrid/forge/internal/llmprovider"
)

// â”€â”€ Happy path: full pipeline â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

func TestRun_DryRun(t *testing.T) {
	t.Parallel()
	res := Run(t.TempDir(), "test change")
	if !res.DryRun {
		t.Fatal("expected dry_run=true")
	}
	if len(res.Checkpoints) != 7 {
		t.Fatalf("expected 7 checkpoints, got %d", len(res.Checkpoints))
	}
}

func TestCmd_Text(t *testing.T) {
	t.Parallel()
	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--root", t.TempDir(), "--no-strict-testing"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "7-checkpoint") {
		t.Fatalf("missing pipeline output: %s", out.String())
	}
}

func TestCmd_JSON(t *testing.T) {
	t.Parallel()
	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--json", "--root", t.TempDir(), "--no-strict-testing"})
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

// â”€â”€ Checkpoint subcommands â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

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

// â”€â”€ Negative: unknown checkpoint name â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

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

// â”€â”€ Idempotency â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

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

// â”€â”€ Cross-checkpoint isolation â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

func TestRunCheckpoints_Spec_NoVerifyResult(t *testing.T) {
	t.Parallel()
	res := RunCheckpoints(t.TempDir(), "", []string{"spec"})
	for _, cp := range res.Checkpoints {
		if strings.EqualFold(cp.Name, "verify") || strings.EqualFold(cp.Name, "code") {
			t.Fatalf("spec subcommand must not return %q checkpoint", cp.Name)
		}
	}
}

// â”€â”€ False-positive guard: verify on empty dir must not fail â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

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

// â”€â”€ YOLO mode â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€
//
// Test-design coverage (always-write-tests.md 9-point):
//  1. Happy path    â€” --yolo: all 5 checkpoints, Ready=true, Approved=nil
//  2. Happy path    â€” interactive, all "y": 4 gates fired, Approved=true on first 4
//  3. Boundary      â€” single subcommand: gate never called, no Approved field
//  4. Negative      â€” "n" at first gate: stops at 1, Ready=false
//  5. Negative      â€” "y\nn": stops at 2nd, 2 checkpoints, Ready=false
//  6. Idempotency   â€” RunCheckpointsGated(nil gate) twice â†’ same count
//  7. Concurrency   â€” all tests t.Parallel() with isolated TempDirs
//  8. Data-accuracy â€” Approved=true/false matches gate return value
//  9. False-positive â€” --json disables gate: all 6 run, Approved=nil

// TestRunCheckpointsGated_NilGate_YOLO â€” nil gate runs all 6, no Approved fields.
func TestRunCheckpointsGated_NilGate_YOLO(t *testing.T) {
	t.Parallel()
	// RunCheckpointsGated has no options struct to waive the 1.8.2 testing
	// gate through, and this test is about gate *approval* plumbing, not
	// testing evidence — so drive the same path via RunWithOptions.
	res := RunWithOptions(RunOptions{Root: t.TempDir(), NoStrictTesting: true})
	if len(res.Checkpoints) != 7 {
		t.Fatalf("yolo: expected 7 checkpoints, got %d", len(res.Checkpoints))
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

// TestRunCheckpointsGated_AllApproved â€” gate always returns true; 4 gates called
// (never for the last checkpoint), first 4 have Approved=true.
func TestRunCheckpointsGated_AllApproved(t *testing.T) {
	t.Parallel()
	gateCalls := 0
	gate := Gate(func(_, _ int, _ Checkpoint) bool {
		gateCalls++
		return true
	})
	// See TestRunCheckpointsGated_NilGate_YOLO: this asserts gate-approval
	// plumbing, so the 1.8.2 testing gate is waived to keep a bare temp dir
	// from failing qa-verify for a reason this test is not about.
	res := RunWithOptions(RunOptions{Root: t.TempDir(), Gate: gate, NoStrictTesting: true})
	if len(res.Checkpoints) != 7 {
		t.Fatalf("all-approved: expected 7 checkpoints, got %d", len(res.Checkpoints))
	}
	if !res.Ready {
		t.Fatal("all-approved: expected Ready=true")
	}
	if gateCalls != 6 {
		t.Fatalf("all-approved: gate must be called 6 times (not for last), got %d", gateCalls)
	}
	for i, cp := range res.Checkpoints[:6] {
		if cp.Approved == nil || !*cp.Approved {
			t.Fatalf("all-approved: checkpoint %d (%s) Approved must be true", i+1, cp.Name)
		}
	}
	// Last checkpoint has no Approved field.
	if res.Checkpoints[6].Approved != nil {
		t.Fatal("all-approved: last checkpoint must not have Approved set")
	}
}

// TestRunCheckpointsGated_RejectAtFirst â€” gate returns false at idx=0.
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

// TestRunCheckpointsGated_RejectAtSecond â€” approve first, reject second.
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

// TestRunCheckpointsGated_SingleCheckpoint_GateNotCalled â€” a single-checkpoint
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

// TestRunCheckpointsGated_Idempotent â€” calling with nil gate twice yields the
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

// TestCmd_Yolo_JSON â€” forge ship --yolo --json: G-004 NDJSON stream with 6 events.
func TestCmd_Yolo_JSON(t *testing.T) {
	t.Parallel()
	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--yolo", "--json", "--root", t.TempDir(), "--no-strict-testing"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("--yolo --json: %v\n%s", err, out.String())
	}
	// G-004: --yolo + --json now emits NDJSON event stream (one line per checkpoint).
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 7 {
		t.Fatalf("--yolo --json: expected 7 NDJSON lines, got %d\n%s", len(lines), out.String())
	}
	// Decode all 7 events.
	for i, line := range lines {
		var ev ShipEvent
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			t.Fatalf("--yolo --json: line %d not valid ShipEvent: %v\n%s", i+1, err, line)
		}
		if ev.SchemaVersion != "1" {
			t.Errorf("--yolo --json: line %d: schema_version must be \"1\", got %q", i+1, ev.SchemaVersion)
		}
	}
	// Second-to-last event must be ship.passed or ship.failed; last is qa.passed or qa.failed.
	var shipEv ShipEvent
	_ = json.Unmarshal([]byte(lines[5]), &shipEv)
	if shipEv.Event != "ship.passed" && shipEv.Event != "ship.failed" {
		t.Errorf("--yolo --json: checkpoint 6 event must be ship.passed|ship.failed, got %q", shipEv.Event)
	}
	var lastEv ShipEvent
	_ = json.Unmarshal([]byte(lines[6]), &lastEv)
	if lastEv.Event != "qa.passed" && lastEv.Event != "qa.failed" {
		t.Errorf("--yolo --json: last event must be qa.passed|qa.failed, got %q", lastEv.Event)
	}
}

// TestCmd_Yolo_Text â€” forge ship --yolo: text output contains "YOLO" badge.
func TestCmd_Yolo_Text(t *testing.T) {
	t.Parallel()
	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--yolo", "--root", t.TempDir(), "--no-strict-testing"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("--yolo text: %v\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "YOLO") {
		t.Fatalf("--yolo text: expected YOLO badge in output:\n%s", out.String())
	}
}

// TestCmd_JSON_DisablesGate â€” --json mode never prompts; all 6 checkpoints run
// without reading stdin. Approved=nil on all (false-positive guard: the gate
// must NOT fire in --json mode).
func TestCmd_JSON_DisablesGate(t *testing.T) {
	t.Parallel()
	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	// Provide no stdin data â€” if the gate fired it would block / return false.
	cmd.SetIn(strings.NewReader(""))
	cmd.SetArgs([]string{"--json", "--root", t.TempDir(), "--no-strict-testing"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("--json gate-disabled: %v\n%s", err, out.String())
	}
	var res ShipResult
	if err := json.Unmarshal(out.Bytes(), &res); err != nil {
		t.Fatalf("--json gate-disabled: not JSON: %v\n%s", err, out.String())
	}
	if len(res.Checkpoints) != 7 {
		t.Fatalf("--json gate-disabled: expected 7 checkpoints, got %d", len(res.Checkpoints))
	}
	if !res.Ready {
		t.Fatal("--json gate-disabled: expected Ready=true")
	}
}

// TestCmd_Interactive_AllApproved â€” inject "y" for each of the 4 gates.
// Pipeline completes with Ready=true and text output contains "approved".
func TestCmd_Interactive_AllApproved(t *testing.T) {
	t.Parallel()
	// 4 gates between 5 checkpoints; provide 4 "y" answers.
	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetIn(strings.NewReader("y\ny\ny\ny\ny\n"))
	cmd.SetArgs([]string{"--root", t.TempDir(), "--no-strict-testing"}) // full pipeline, no --yolo, no --json â†’ interactive
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

// TestCmd_Interactive_RejectFirst â€” inject "n" at the first gate.
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

// TestCmd_Interactive_RejectMiddle â€” approve first two, reject third.
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

// TestCmd_Subcommand_Spec_NoApproval â€” single-checkpoint subcommand needs no
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

// TestRenderText_YoloBadge â€” ShipResult with Yolo=true produces "YOLO" in text.
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

// TestRenderText_RejectedAnnotation â€” Approved=false produces "[rejected]" in text.
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

// TestRenderText_ApprovedAnnotation â€” Approved=true produces "[approved]" in text.
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

// â”€â”€ Self-debate integration (ship_test additions) â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

// TestCmd_Yolo_JSON_IncludesDebate â€” RunWithOptions with DebateOpts produces a ShipResult
// where DebateEnabled=true and each checkpoint has a non-nil Debate object.
func TestCmd_Yolo_JSON_IncludesDebate(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	res := RunWithOptions(RunOptions{
		Root: root,
		// Debate plumbing, not the 1.8.2 four-stage testing gate.
		NoStrictTesting: true,
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

// TestCmd_Yolo_Text_ShowsDebate â€” RunWithOptions with DebateOpts populates Debate.Roles.
func TestCmd_Yolo_Text_ShowsDebate(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	res := RunWithOptions(RunOptions{
		Root: root,
		// Debate plumbing, not the 1.8.2 four-stage testing gate.
		NoStrictTesting: true,
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

// TestCmd_Yolo_Text_ShowsImprovements â€” RunWithOptions with DebateOpts populates Improvements.
func TestCmd_Yolo_Text_ShowsImprovements(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	res := RunWithOptions(RunOptions{
		Root: root,
		// Debate plumbing, not the 1.8.2 four-stage testing gate.
		NoStrictTesting: true,
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

// TestRunWithOptions_DebateEnabled â€” RunWithOptions with DebateOpts sets Debate on
// every checkpoint, consensus is always reached in dry-run, improvements non-empty.
func TestRunWithOptions_DebateEnabled(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	res := RunWithOptions(RunOptions{
		Root: root,
		// Debate plumbing, not the 1.8.2 four-stage testing gate.
		NoStrictTesting: true,
		DebateOpts: &DebateOptions{
			Feature:   "order-fulfillment",
			MaxRounds: 3,
			DryRun:    true,
		},
	})

	if !res.Ready {
		t.Fatalf("expected Ready=true, got message: %s", res.Message)
	}
	if len(res.Checkpoints) != 7 {
		t.Fatalf("expected 7 checkpoints, got %d", len(res.Checkpoints))
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

// TestRenderText_DebateSummary â€” renderText shows the self-debate summary line when
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

// â”€â”€ LLM-driven checkpoint tests (MockProvider injection) â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€
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

// â”€â”€ Spec checkpoint (LLM-driven) â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

// TestCheckSpec_LLM_GeneratesNewSpec â€” when no spec exists the LLM generates one
// and the file is written under .forge/specs/<slug>/spec.md.
func TestCheckSpec_LLM_GeneratesNewSpec(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mock := &llmprovider.MockProvider{
		Response: mockResponse("# Spec: add login\n\n## What\nAdd a login form.\n"),
	}
	cp := checkSpec(root, "add login", "", mockPipe(root, mock), false)

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

// TestCheckSpec_LLM_ReviewsExistingSpec â€” when a spec already exists the LLM
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
	cp := checkSpec(root, "review feature", "", mockPipe(root, mock), false)

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

// TestCheckSpec_LLM_ProviderFails_GracefulDegradation â€” when the LLM returns an
// error during spec generation, the checkpoint is still "ok" with a stub file.
func TestCheckSpec_LLM_ProviderFails_GracefulDegradation(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mock := &llmprovider.MockProvider{Err: fmt.Errorf("FORGE-4051 transport not implemented")}
	cp := checkSpec(root, "failing feature", "", mockPipe(root, mock), false)

	if cp.Status != "ok" {
		t.Fatalf("provider error must not fail the spec checkpoint; got %q: %s", cp.Status, cp.Detail)
	}
}

// TestCheckSpec_LLM_NoDescription_Warning â€” no description and no existing specs
// results in "warning" (provider name should appear in detail).
func TestCheckSpec_LLM_NoDescription_Warning(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mock := &llmprovider.MockProvider{Response: mockResponse("ignored")}
	cp := checkSpec(root, "", "", mockPipe(root, mock), false)

	if cp.Status != "warning" {
		t.Fatalf("expected warning with no description, got %q: %s", cp.Status, cp.Detail)
	}
	if !strings.Contains(cp.Detail, "mock") {
		t.Errorf("detail should mention provider name: %s", cp.Detail)
	}
}

// â”€â”€ YAML spec (spec.yml) integration tests â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€
//
// Test-design checklist (always-write-tests.md 9-point):
//  1. Happy path (LLM)     â€” spec.yml + spec.md exist; LLM KB-enriched review; detail shows case count.
//  2. Happy path (no LLM)  â€” spec.yml + spec.md exist; nil pipe; detail shows case count.
//  3. Boundary             â€” spec.yml with 0 cases; checkpoint still "ok".
//  4. Negative             â€” corrupt spec.yml; falls back to plain spec.md behavior.
//  5. Idempotency          â€” call checkSpec twice; identical "ok" result.
//  6. Regression           â€” spec.md only (no spec.yml); original plain Invoke behavior unchanged.
//  7. Data-accuracy        â€” detail has exact case count and family list.
//  8. False-positive guard â€” no spec.yml; plain spec.md must not fail.
//  9. New spec from YAML   â€” spec.yml present, spec.md absent; spec.md generated from YAML.

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

// TestCheckSpec_YAML_WithLLM_KBEnrichedReview â€” happy path: spec.yml + spec.md both
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
	cp := checkSpec(root, feature, "", mockPipe(root, mock), false)

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

// TestCheckSpec_YAML_NoLLM_DetailShowsCaseCount â€” happy path: spec.yml + spec.md,
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

	cp := checkSpec(root, feature, "", nil, false)

	if cp.Status != "ok" {
		t.Fatalf("expected ok, got %q: %s", cp.Status, cp.Detail)
	}
	if !strings.Contains(cp.Detail, "2") {
		t.Errorf("detail should contain case count 2; got: %s", cp.Detail)
	}
}

// TestCheckSpec_YAML_ZeroCases_StillOK â€” boundary: spec.yml with 0 cases;
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

	cp := checkSpec(root, feature, "", nil, false)

	if cp.Status != "ok" {
		t.Fatalf("spec with 0 cases must still be ok; got %q: %s", cp.Status, cp.Detail)
	}
}

// TestCheckSpec_YAML_CorruptYAML_FallsBackToSpecMD â€” negative: corrupt spec.yml
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

	cp := checkSpec(root, feature, "", nil, false)

	if cp.Status != "ok" {
		t.Fatalf("corrupt spec.yml must not fail checkpoint; got %q: %s", cp.Status, cp.Detail)
	}
}

// TestCheckSpec_YAML_Idempotency â€” calling checkSpec twice on the same dir
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

	cp1 := checkSpec(root, feature, "", nil, false)
	cp2 := checkSpec(root, feature, "", nil, false)

	if cp1.Status != "ok" || cp2.Status != "ok" {
		t.Fatalf("both calls must be ok; got %q, %q", cp1.Status, cp2.Status)
	}
	if cp1.Detail != cp2.Detail {
		t.Errorf("idempotency: detail changed between calls\n  first:  %s\n  second: %s",
			cp1.Detail, cp2.Detail)
	}
}

// TestCheckSpec_YAML_Regression_SpecMDOnly â€” regression: spec.md only (no spec.yml)
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
	cp := checkSpec(root, feature, "", mockPipe(root, mock), false)

	if cp.Status != "ok" {
		t.Fatalf("spec.md-only path must be ok; got %q: %s", cp.Status, cp.Detail)
	}
	if mock.Calls() == 0 {
		t.Error("expected LLM call for spec.md review; got none")
	}
}

// TestCheckSpec_YAML_DataAccuracy_DetailHasCaseCountAndFamilies â€” data-accuracy:
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

	cp := checkSpec(root, feature, "", nil, false)

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

// TestCheckSpec_YAML_FalsePositiveGuard_NoYAML_NoFailure â€” false-positive: absent
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

	cp := checkSpec(root, feature, "", nil, false)

	if cp.Status != "ok" {
		t.Fatalf("absent spec.yml must not fail; got %q: %s", cp.Status, cp.Detail)
	}
}

// TestCheckSpec_YAML_OnlyYAML_GeneratesSpecMD â€” when spec.yml is present but
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
	// No spec.md â€” intentionally absent.

	mock := &llmprovider.MockProvider{
		Response: mockResponse("# Generated from YAML\n## Acceptance Criteria\n- happy path\n"),
	}
	cp := checkSpec(root, feature, "", mockPipe(root, mock), false)

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

// â”€â”€ --name/-n flag (spec-name override) â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

// TestCheckSpec_SpecName_HappyPath â€” when --name login is given the checkpoint
// targets .forge/specs/login/ regardless of the description text.
func TestCheckSpec_SpecName_HappyPath(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// Spec lives under "login", not under slugify("add login feature").
	dir := filepath.Join(root, ".forge", "specs", "login")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Login Spec\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cp := checkSpec(root, "add login feature", "login", nil, false)

	if cp.Status != "ok" {
		t.Fatalf("expected ok with --name login override, got %q: %s", cp.Status, cp.Detail)
	}
	if !strings.Contains(cp.Detail, "login") {
		t.Errorf("detail should reference spec dir 'login'; got: %s", cp.Detail)
	}
}

// TestCheckSpec_SpecName_WithYAML_KBEnriched â€” happy path: --name with spec.yml
// present; KB-enriched LLM call must be made.
func TestCheckSpec_SpecName_WithYAML_KBEnriched(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dir := filepath.Join(root, ".forge", "specs", "auth")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestSpecYAML(t, dir, minTestSpec("auth"))
	if err := os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Auth Spec\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mock := &llmprovider.MockProvider{
		Response: mockResponse("# Auth Spec Enhanced\n"),
	}

	cp := checkSpec(root, "authentication flow", "auth", mockPipe(root, mock), false)

	if cp.Status != "ok" {
		t.Fatalf("expected ok, got %q: %s", cp.Status, cp.Detail)
	}
	if mock.Calls() == 0 {
		t.Error("expected KB-enriched LLM call via --name; got none")
	}
}

// TestCheckSpec_SpecName_Empty_FallsBackToSlug â€” boundary: empty --name must
// fall back to the slug derived from description (regression guard).
func TestCheckSpec_SpecName_Empty_FallsBackToSlug(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	feature := "rate-limiting"
	slug := slugify(feature)
	dir := filepath.Join(root, ".forge", "specs", slug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Rate Limiting\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// specName="" â†’ must resolve via slugify(feature).
	cp := checkSpec(root, feature, "", nil, false)

	if cp.Status != "ok" {
		t.Fatalf("empty specName must use derived slug %q; got %q: %s", slug, cp.Status, cp.Detail)
	}
	if !strings.Contains(cp.Detail, slug) {
		t.Errorf("detail should contain slug %q; got: %s", slug, cp.Detail)
	}
}

// TestCheckSpec_SpecName_NoSuchDir_GeneratesStub â€” negative: --name unknown
// where the dir does not exist; a stub spec.md must be created there.
func TestCheckSpec_SpecName_NoSuchDir_GeneratesStub(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	cp := checkSpec(root, "my feature", "unknown-spec", nil, false)

	if cp.Status != "ok" {
		t.Fatalf("missing spec dir should produce stub, not fail; got %q: %s", cp.Status, cp.Detail)
	}
	stubPath := filepath.Join(root, ".forge", "specs", "unknown-spec", "spec.md")
	if _, err := os.Stat(stubPath); os.IsNotExist(err) {
		t.Errorf("stub spec.md must be created under --name directory; not found at %s", stubPath)
	}
}

// TestCheckSpec_SpecName_Idempotency â€” calling checkSpec twice with the same
// --name must produce identical "ok" results.
func TestCheckSpec_SpecName_Idempotency(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dir := filepath.Join(root, ".forge", "specs", "my-feature")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# My Feature\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cp1 := checkSpec(root, "some description", "my-feature", nil, false)
	cp2 := checkSpec(root, "some description", "my-feature", nil, false)

	if cp1.Status != "ok" || cp2.Status != "ok" {
		t.Fatalf("both calls must be ok; got %q, %q", cp1.Status, cp2.Status)
	}
	if cp1.Detail != cp2.Detail {
		t.Errorf("idempotency: detail changed\n  first:  %s\n  second: %s", cp1.Detail, cp2.Detail)
	}
}

// TestCheckSpec_SpecName_Regression_NoFlag_DescriptionSlugWorks â€” regression:
// existing callers that pass no specName must continue to derive the slug from
// description exactly as before.
func TestCheckSpec_SpecName_Regression_NoFlag_DescriptionSlugWorks(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	desc := "add payment gateway"
	slug := slugify(desc)
	dir := filepath.Join(root, ".forge", "specs", slug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Payment Gateway\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cp := checkSpec(root, desc, "", nil, false)

	if cp.Status != "ok" {
		t.Fatalf("description-derived slug path must still work; got %q: %s", cp.Status, cp.Detail)
	}
}

// TestCheckSpec_SpecName_DataAccuracy_DetailContainsSpecName â€” data-accuracy:
// detail string must reference the exact --name value used.
func TestCheckSpec_SpecName_DataAccuracy_DetailContainsSpecName(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dir := filepath.Join(root, ".forge", "specs", "dashboard")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Dashboard\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cp := checkSpec(root, "admin dashboard feature", "dashboard", nil, false)

	if cp.Status != "ok" {
		t.Fatalf("expected ok, got %q: %s", cp.Status, cp.Detail)
	}
	if !strings.Contains(cp.Detail, "dashboard") {
		t.Errorf("detail must reference the spec name 'dashboard'; got: %s", cp.Detail)
	}
}

// TestCheckSpec_SpecName_FalsePositiveGuard_OtherCheckpoints_Unaffected â€” false-
// positive guard: the --name flag on other checkpoints (arch, test, etc.) must
// have no observable effect; those functions do not accept a specName parameter.
// This test uses RunOptions.SpecName only in RunWithOptions (which only routes it
// to checkSpec), confirming no unintended side-effects on other checkpoints.
func TestCheckSpec_SpecName_FalsePositiveGuard_OtherCheckpoints_Unaffected(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// Run only the "arch" checkpoint; SpecName set to something non-empty.
	res := RunWithOptions(RunOptions{
		Root:        root,
		Description: "my feature",
		SpecName:    "my-feature", // must not affect arch checkpoint
		Names:       []string{"arch"},
	})
	if len(res.Checkpoints) != 1 {
		t.Fatalf("expected 1 checkpoint (arch), got %d", len(res.Checkpoints))
	}
	if !strings.EqualFold(res.Checkpoints[0].Name, "arch") {
		t.Fatalf("expected arch checkpoint, got %q", res.Checkpoints[0].Name)
	}
}

// TestCheckSpec_SpecName_OnlySpecName_NoDescription_UsesSpecNameAsContext â€” when
// only --name is set and description is empty, the spec name is used as the LLM
// feature context and the spec directory is found by the exact name.
func TestCheckSpec_SpecName_OnlySpecName_NoDescription(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dir := filepath.Join(root, ".forge", "specs", "login")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Login\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cp := checkSpec(root, "", "login", nil, false)

	if cp.Status != "ok" {
		t.Fatalf("spec-name-only (no description) must be ok; got %q: %s", cp.Status, cp.Detail)
	}
	if !strings.Contains(cp.Detail, "login") {
		t.Errorf("detail must reference spec name 'login'; got: %s", cp.Detail)
	}
}

// TestCheckSpec_SpecName_CobraFlag_ExposedOnSpecSubcmd â€” verifies that the
// `forge ship spec` subcommand exposes `--name`/`-n` flag and not other subcommands.
func TestCheckSpec_SpecName_CobraFlag_ExposedOnSpecSubcmd(t *testing.T) {
	t.Parallel()
	cmd := New()

	var specSubCmd, archSubCmd *cobra.Command
	for _, sub := range cmd.Commands() {
		switch sub.Name() {
		case "spec":
			specSubCmd = sub
		case "arch":
			archSubCmd = sub
		}
	}
	if specSubCmd == nil {
		t.Fatal("spec subcommand not found")
	}
	if specSubCmd.Flags().Lookup("name") == nil {
		t.Error("forge ship spec must expose --name/-n flag")
	}
	if specSubCmd.Flags().ShorthandLookup("n") == nil {
		t.Error("forge ship spec must expose -n shorthand")
	}
	if archSubCmd == nil {
		t.Fatal("arch subcommand not found")
	}
	// --name/-n is now exposed on ALL checkpoint subcommands (moved to bindFlags).
	if archSubCmd.Flags().Lookup("name") == nil {
		t.Error("forge ship arch must expose --name/-n flag (bindFlags global registration)")
	}
}

// â”€â”€ Test-generation checkpoint â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

// TestCheckTest_LLM_GeneratesStubs â€” when no test files exist the LLM generates
// stubs and writes them to .forge/specs/<slug>/test-stubs.md.
func TestCheckTest_LLM_GeneratesStubs(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mock := &llmprovider.MockProvider{
		Response: mockResponse("```go\nfunc TestLogin(t *testing.T) {}\n```"),
	}
	cp := checkTest(root, "add login", "", mockPipe(root, mock), false)

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

// TestCheckTest_LLM_HonorsSpecNameOverride — regression test: generateTestStubs
// must write to the --name/-n override directory, not a second directory
// auto-slugified from the raw description. See
// TestCheckBreakdown_LLM_HonorsSpecNameOverride for the full incident context.
func TestCheckTest_LLM_HonorsSpecNameOverride(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	description := "Master content calendar: a long feature description whose " +
		"slugified form differs from the --name override below"
	mock := &llmprovider.MockProvider{
		Response: mockResponse("```go\nfunc TestCalendar(t *testing.T) {}\n```"),
	}
	cp := checkTest(root, description, "custom-slug", mockPipe(root, mock), false)

	if cp.Status != "ok" {
		t.Fatalf("expected ok, got %q: %s", cp.Status, cp.Detail)
	}
	if _, err := os.ReadFile(filepath.Join(root, ".forge", "specs", "custom-slug", "test-stubs.md")); err != nil {
		t.Fatalf("test-stubs.md not written under the --name override directory: %v", err)
	}
	wrongDir := filepath.Join(root, ".forge", "specs", slugify(description))
	if _, err := os.Stat(wrongDir); err == nil {
		t.Fatalf("test-stubs.md leaked into the description-derived slug directory %q — specName was not honored", wrongDir)
	}
}

// TestCheckTest_LLM_ProviderFails_Warning â€” when the LLM returns an error and no
// test files exist, the checkpoint is "warning" (not "fail").
func TestCheckTest_LLM_ProviderFails_Warning(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mock := &llmprovider.MockProvider{Err: fmt.Errorf("FORGE-4051 transport not implemented")}
	cp := checkTest(root, "broken feature", "", mockPipe(root, mock), false)

	if cp.Status != "warning" {
		t.Fatalf("expected warning on provider error, got %q: %s", cp.Status, cp.Detail)
	}
}

// TestCheckTest_LLM_ExistingTestFiles â€” when test files exist the checkpoint is "ok"
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
	cp := checkTest(root, "extend feature", "", mockPipe(root, mock), false)

	if cp.Status != "ok" {
		t.Fatalf("existing test files should give ok, got %q: %s", cp.Status, cp.Detail)
	}
}

// TestCheckTest_DoesNotClobberExistingNamedArtifacts — regression test for a
// real bug: checkTest used to call writeTestArtifacts unconditionally, so
// re-running `forge ship test` (or a full `forge ship`/`forge ship -d`) after
// a developer had already written real content into the 4 named artifacts
// (<slug>.test.ts, .integration.test.ts, .rls.test.ts, .scan.baseline.json)
// would silently overwrite that content with fresh RED placeholder stubs.
// Once all 4 artifacts exist, checkTest must leave them untouched.
func TestCheckTest_DoesNotClobberExistingNamedArtifacts(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	slug := slugify("add login")
	testsDir := filepath.Join(root, "tests")
	if err := os.MkdirAll(testsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	const realContent = "// hand-written, real assertions\nexpect(true).toBe(true);\n"
	names := []string{
		slug + ".test.ts",
		slug + ".integration.test.ts",
		slug + ".rls.test.ts",
		slug + ".scan.baseline.json",
	}
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(testsDir, name), []byte(realContent), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	mock := &llmprovider.MockProvider{
		Response: mockResponse("```go\nfunc TestLogin(t *testing.T) {}\n```"),
	}
	cp := checkTest(root, "add login", "", mockPipe(root, mock), false)

	if cp.Status != "ok" {
		t.Fatalf("expected ok, got %q: %s", cp.Status, cp.Detail)
	}
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(testsDir, name))
		if err != nil {
			t.Fatalf("artifact %s missing after checkTest: %v", name, err)
		}
		if string(data) != realContent {
			t.Errorf("artifact %s was overwritten; got:\n%s", name, data)
		}
	}
}

// TestCheckTest_DryRunNeverWritesArtifacts — --dry-run must be side-effect-free
// (per its documented contract: "preview what would happen without making LLM
// calls or git operations"). checkTest must not scaffold the 4 named artifacts
// on disk when dryRun is true, even when none exist yet.
func TestCheckTest_DryRunNeverWritesArtifacts(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	slug := slugify("add login")
	mock := &llmprovider.MockProvider{
		Response: mockResponse("```go\nfunc TestLogin(t *testing.T) {}\n```"),
	}

	_ = checkTest(root, "add login", "", mockPipe(root, mock), true)

	testsDir := filepath.Join(root, "tests")
	for _, name := range []string{
		slug + ".test.ts",
		slug + ".integration.test.ts",
		slug + ".rls.test.ts",
		slug + ".scan.baseline.json",
	} {
		if _, err := os.Stat(filepath.Join(testsDir, name)); err == nil {
			t.Errorf("dry-run must not write %s to disk", name)
		}
	}
}

// ── Breakdown checkpoint ──────────────────────────────────────────────────

// TestCheckBreakdown_LLM_GeneratesBreakdown â€” when no breakdown.md exists the LLM
// produces one and writes it under .forge/specs/<slug>/breakdown.md.
func TestCheckBreakdown_LLM_GeneratesBreakdown(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mock := &llmprovider.MockProvider{
		Response: mockResponse("## Task 1\nImplement route handler.\n\n## Task 2\nWrite tests.\n"),
	}
	cp := checkBreakdown(root, "new endpoint", "", mockPipe(root, mock))

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

// TestCheckBreakdown_LLM_HonorsSpecNameOverride — regression test for a real
// incident: generateBreakdown used to independently re-derive slugify(description)
// instead of honoring the --name/-n override the caller already resolved, so
// breakdown.md/tasks.md landed in a second, wrong .forge/specs/<slug>/ directory
// (auto-slugified from the raw description) while spec.md/arch.md correctly used
// the --name override. Reproduced live across 3 real forge ship runs (2 providers)
// before the fix threaded specName through generateBreakdown.
func TestCheckBreakdown_LLM_HonorsSpecNameOverride(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	description := "Master content calendar: a long feature description whose " +
		"slugified form differs from the --name override below"
	mock := &llmprovider.MockProvider{
		Response: mockResponse("## Task 1\nDo the thing.\n"),
	}
	cp := checkBreakdown(root, description, "custom-slug", mockPipe(root, mock))

	if cp.Status != "ok" {
		t.Fatalf("expected ok, got %q: %s", cp.Status, cp.Detail)
	}
	if _, err := os.ReadFile(filepath.Join(root, ".forge", "specs", "custom-slug", "breakdown.md")); err != nil {
		t.Fatalf("breakdown.md not written under the --name override directory: %v", err)
	}
	if _, err := os.ReadFile(filepath.Join(root, ".forge", "specs", "custom-slug", "tasks.md")); err != nil {
		t.Fatalf("tasks.md not written under the --name override directory: %v", err)
	}
	wrongDir := filepath.Join(root, ".forge", "specs", slugify(description))
	if _, err := os.Stat(wrongDir); err == nil {
		t.Fatalf("breakdown artefacts leaked into the description-derived slug directory %q — specName was not honored", wrongDir)
	}
}

// TestCheckBreakdown_LLM_ExistingBreakdown â€” when breakdown.md already exists the
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
	cp := checkBreakdown(root, "existing breakdown", "", mockPipe(root, mock))

	if cp.Status != "ok" {
		t.Fatalf("existing breakdown.md must be ok, got %q: %s", cp.Status, cp.Detail)
	}
	if mock.Calls() != 0 {
		t.Error("MockProvider should not be called when breakdown.md already exists")
	}
}

// TestCheckBreakdown_LLM_ProviderFails_Warning â€” LLM error + no breakdown â†’ "warning".
func TestCheckBreakdown_LLM_ProviderFails_Warning(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mock := &llmprovider.MockProvider{Err: fmt.Errorf("FORGE-4051 transport not implemented")}
	cp := checkBreakdown(root, "breakdown fail", "", mockPipe(root, mock))

	if cp.Status != "warning" {
		t.Fatalf("expected warning on provider error, got %q: %s", cp.Status, cp.Detail)
	}
}

// â”€â”€ Code+scan checkpoint (full loop) â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

// TestCheckCode_LLM_GeneratesCodePlan â€” when spec and breakdown exist the LLM
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
	cp := checkCode(root, "code plan feature", "", mockPipe(root, mock))

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

// TestCheckCode_LLM_HonorsSpecNameOverride — regression test: generateCodePlan
// must read spec.md/breakdown.md from, and write code-plan.md to, the --name/-n
// override directory, not a second directory auto-slugified from the raw
// description. See TestCheckBreakdown_LLM_HonorsSpecNameOverride for the full
// incident context — this one matters most in practice because the checkCode
// log line ("code plan written by %s (see .forge/specs/%s/code-plan.md)")
// reported the *correct* --name directory even while generateCodePlan silently
// wrote the real file to the wrong one, making the bug invisible from the CLI
// output alone.
func TestCheckCode_LLM_HonorsSpecNameOverride(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	description := "Master content calendar: a long feature description whose " +
		"slugified form differs from the --name override below"
	dir := filepath.Join(root, ".forge", "specs", "custom-slug")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "breakdown.md"), []byte("## Task 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mock := &llmprovider.MockProvider{
		Response: mockResponse("## Step 1\nCreate handler.\n"),
	}
	cp := checkCode(root, description, "custom-slug", mockPipe(root, mock))

	if cp.Status != "ok" {
		t.Fatalf("expected ok, got %q: %s", cp.Status, cp.Detail)
	}
	if mock.Calls() == 0 {
		t.Fatal("MockProvider.Complete was not called — generateCodePlan did not find spec.md/breakdown.md under the --name override directory")
	}
	if _, err := os.ReadFile(filepath.Join(dir, "code-plan.md")); err != nil {
		t.Fatalf("code-plan.md not written under the --name override directory: %v", err)
	}
	wrongDir := filepath.Join(root, ".forge", "specs", slugify(description))
	if _, err := os.Stat(wrongDir); err == nil {
		t.Fatalf("code-plan.md leaked into the description-derived slug directory %q — specName was not honored", wrongDir)
	}
}

// TestCheckCode_LLM_NoContext_NoCallMade â€” when neither spec.md nor breakdown.md
// exist, generateCodePlan returns early and the LLM is NOT called.
func TestCheckCode_LLM_NoContext_NoCallMade(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mock := &llmprovider.MockProvider{Response: mockResponse("should not be called")}
	cp := checkCode(root, "empty feature", "", mockPipe(root, mock))

	// Without context files, code plan can't be generated; structural fallback.
	// Status depends on working-tree state; either warning or ok both acceptable.
	_ = cp // outcome checked by not panicking

	if mock.Calls() != 0 {
		t.Error("MockProvider should not be called when no spec/breakdown context exists")
	}
}

// TestCheckCode_LLM_ProviderFails_GracefulFallback â€” provider error with changed files
// still results in "ok" (falls back to file-count structural check).
func TestCheckCode_LLM_ProviderFails_GracefulFallback(t *testing.T) {
	t.Parallel()
	skipIfNoGitShip(t)
	// countChangedFiles now shells out to `git status --porcelain` (it used to
	// just check for a .git/index file's existence, which any fixture could
	// fake without a real repo). A genuine repo + an untracked file is needed
	// so countChangedFiles legitimately returns > 0.
	root := initShipRepo(t)
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
	cp := checkCode(root, "provider fail code", "", mockPipe(root, mock))

	// With changed files but provider error, status is "ok" (structural fallback).
	if cp.Status != "ok" {
		t.Fatalf("expected ok (structural fallback), got %q: %s", cp.Status, cp.Detail)
	}
}

// TestCountChangedFiles_MatchesRealGitStatus — regression test for a real
// incident: countChangedFiles used to walk the entire working tree counting
// every .go/.ts/.js/.py/.sql file that existed on disk, regardless of whether
// it was actually changed. On a real ~1700-source-file project this reported
// "1693 modified file(s)" in the Code/Ship checkpoint output when the real
// change count (per `git status --short`) was in the single digits —
// reproduced live across 2 separate forge ship runs (different LLM providers,
// same misleading count both times). It must count exactly what
// `git status --porcelain` reports: untracked + modified + staged paths,
// honoring .gitignore.
func TestCountChangedFiles_MatchesRealGitStatus(t *testing.T) {
	t.Parallel()
	skipIfNoGitShip(t)
	root := initShipRepo(t)

	// Happy path: a clean checkout reports zero changes.
	if n := countChangedFiles(root); n != 0 {
		t.Fatalf("expected 0 changed files on a clean checkout, got %d", n)
	}

	// Two untracked source files + one gitignored file. The gitignored file is
	// the false-positive guard: the old file-extension walk would have counted
	// it (it's a real .go file on disk); git status --porcelain must not.
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("ignored.go\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "a.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "b.ts"), []byte("export {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "ignored.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// .gitignore itself is also untracked, so the real count is 3 (.gitignore,
	// a.go, b.ts) — ignored.go must NOT be among them.
	if n := countChangedFiles(root); n != 3 {
		t.Fatalf("expected 3 changed files (.gitignore, a.go, b.ts; ignored.go must be excluded), got %d", n)
	}
}

// â”€â”€ PR creation â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

// TestCheckPR_GhNotFound_Warning â€” when gh CLI is not in PATH, checkPR returns
// Status="warning" (never "fail") so the pipeline is never hard-blocked.
func TestCheckPR_GhNotFound_Warning(t *testing.T) {
	t.Parallel()
	// Use a temp dir that doesn't have gh; check by examining the result.
	root := t.TempDir()
	cp := checkPR(root, "test PR", "")

	// In a typical CI environment, gh may or may not be installed.
	// The invariant is: status must never be "fail".
	if cp.Status == "fail" {
		t.Fatalf("checkPR must never return 'fail', got %q: %s", cp.Status, cp.Detail)
	}
	if cp.Name != "PR" {
		t.Errorf("checkpoint name must be 'PR', got %q", cp.Name)
	}
}

// TestRunWithOptions_CreatePR_GhAbsent â€” full pipeline with CreatePR=true appends
// a PR checkpoint that is either "ok" or "warning" (depends on gh availability).
func TestRunWithOptions_CreatePR_GhAbsent(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	res := RunWithOptions(RunOptions{
		// This test exercises pipeline mechanics, not the 4-stage testing
		// gate; since 1.8.2 a bare temp dir legitimately fails qa-verify for
		// having no testing evidence.
		NoStrictTesting: true,
		Root:            root,
		CreatePR:        true,
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

// â”€â”€ Full pipeline with MockProvider â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

// TestRunWithOptions_MockLLM_FullPipeline â€” inject a MockProvider for the entire
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
		// Pipeline mechanics, not the 1.8.2 four-stage testing gate.
		NoStrictTesting: true,
		Root:            root,
		Description:     "full pipeline mock feature",
		LLMPipe:         mockPipe(root, mock),
	})

	if len(res.Checkpoints) != 7 {
		t.Fatalf("expected 7 checkpoints, got %d", len(res.Checkpoints))
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

// TestRunWithOptions_SingleCheckpoint_DoesNotRunOthers is the regression test
// for a confirmed production incident (2026-07-17 dogfooding): running a
// single-checkpoint subcommand — e.g. `forge ship spec "<desc>"`, which sets
// opts.Names = []string{"spec"} — silently executed every other checkpoint's
// real LLM calls and file writes too (arch.md, breakdown.md, code-plan.md,
// test-stubs.md, tasks.md all appeared on disk), while the CLI only reported
// the one requested checkpoint ("[1/1] âœ“ Spec"). The bug: every check*
// function used to be called unconditionally before opts.Names was ever
// consulted — the filtering only trimmed what got *displayed*, not what
// actually ran.
//
// This asserts both sides of the fix: only the requested checkpoint appears
// in the result, and the mock provider is invoked the number of times a
// single spec-only run should need — not once per checkpoint in the
// full pipeline.
func TestRunWithOptions_SingleCheckpoint_DoesNotRunOthers(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mock := &llmprovider.MockProvider{
		Response: mockResponse("# Spec: mock feature\n\n## What\nDoes a thing."),
	}
	res := RunWithOptions(RunOptions{
		Root:        root,
		Description: "single checkpoint mock feature",
		Names:       []string{"spec"},
		LLMPipe:     mockPipe(root, mock),
	})

	if len(res.Checkpoints) != 1 {
		t.Fatalf("expected exactly 1 checkpoint in the result for Names=[spec], got %d: %+v",
			len(res.Checkpoints), res.Checkpoints)
	}
	if got := strings.ToLower(res.Checkpoints[0].Name); got != "" && got != "spec" {
		t.Errorf("expected the single checkpoint to be spec, got %q", res.Checkpoints[0].Name)
	}

	slug := slugify("single checkpoint mock feature")
	specsDir := filepath.Join(root, ".forge", "specs", slug)
	for _, unexpected := range []string{"arch.md", "openapi.yaml", "breakdown.md", "code-plan.md", "test-stubs.md", "tasks.md"} {
		if _, err := os.Stat(filepath.Join(specsDir, unexpected)); err == nil {
			t.Errorf("Names=[spec] must not generate %s — only the spec checkpoint was requested", unexpected)
		}
	}
	if _, err := os.Stat(filepath.Join(specsDir, "spec.md")); err != nil {
		t.Errorf("expected spec.md to be written: %v", err)
	}

	// A spec-only run makes exactly one real Complete() call (spec generation
	// via generateWithValidation's genFn; no retry needed since the mock
	// response is structurally complete). Full pipeline needs many more.
	if calls := mock.Calls(); calls != 1 {
		t.Errorf("expected exactly 1 provider call for a spec-only run, got %d "+
			"(a higher count means other checkpoints ran too)", calls)
	}
}

// TestRunWithOptions_MockLLM_Idempotent â€” running the full pipeline twice with the
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

// Single-checkpoint runs must not execute other checkpoints.
//
// Root cause (2026-07-20): runWithOptions used to build ALL 7 checkpoints
// unconditionally regardless of opts.Names, filtering down to the requested
// one only when selecting which result to *display*. A bare `forge ship arch`
// or `forge ship ship` therefore silently ran checkSpec's LLM-backed spec.md
// review pass (among others) too - paying for up to 7 checkpoints per single
// requested one, and (observed directly) letting checkSpec's review
// overwrite an existing spec.md with regenerated, sometimes-hallucinated
// content even when the caller only asked for `arch` or `ship`. It also
// explains the `ship` checkpoint's own hygiene gate failing on cache files
// written by checkTest/checkBreakdown/checkQAVerify - checkpoints nobody
// requested - moments before checkVerify's clean-check ran.

// 1. Happy path - requesting only "arch" must not touch an existing spec.md.
func TestRunWithOptions_SingleCheckpoint_DoesNotRewriteUnrequestedSpec(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	slug := "single-cp-guard"
	specDir := filepath.Join(root, ".forge", "specs", slug)
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatal(err)
	}
	const original = "# Original Spec\n\nThis must not change.\n"
	if err := os.WriteFile(filepath.Join(specDir, "spec.md"), []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	// If checkSpec were (incorrectly) invoked, its review pass would
	// overwrite spec.md with this distinctive mock content.
	mock := &llmprovider.MockProvider{
		Response: mockResponse("# MOCK REGENERATED CONTENT - checkSpec should never have run\n"),
	}
	res := RunWithOptions(RunOptions{
		Root:     root,
		SpecName: slug,
		Names:    []string{"arch"},
		LLMPipe:  mockPipe(root, mock),
	})

	if len(res.Checkpoints) != 1 || !strings.EqualFold(res.Checkpoints[0].Name, "arch") {
		t.Fatalf("expected exactly 1 checkpoint (arch), got %+v", res.Checkpoints)
	}

	got, err := os.ReadFile(filepath.Join(specDir, "spec.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != original {
		t.Errorf("spec.md was rewritten by an unrequested checkpoint (checkSpec ran despite Names=[\"arch\"]):\n%s", got)
	}

	// arch checkpoint itself must have actually run for real.
	if _, err := os.Stat(filepath.Join(specDir, "arch.md")); err != nil {
		t.Errorf("expected arch.md to be created by the requested arch checkpoint: %v", err)
	}
}

// 2. Cost - requesting only "ship" must not invoke checkTest/checkBreakdown/
// checkQAVerify (each of which would call the mock LLM and create its own
// artefact if it ran).
func TestRunWithOptions_SingleCheckpoint_ShipDoesNotRunUnrequestedCheckpoints(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	slug := "single-cp-ship-guard"
	specDir := filepath.Join(root, ".forge", "specs", slug)
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "spec.md"), []byte("# Spec\n\nAcceptance criteria here.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	mock := &llmprovider.MockProvider{Response: mockResponse("mock content that would create sibling artefacts")}
	res := RunWithOptions(RunOptions{
		Root:     root,
		SpecName: slug,
		Names:    []string{"ship"},
		LLMPipe:  mockPipe(root, mock),
	})

	if len(res.Checkpoints) != 1 || !strings.EqualFold(res.Checkpoints[0].Name, "ship") {
		t.Fatalf("expected exactly 1 checkpoint (ship), got %+v", res.Checkpoints)
	}

	// None of these are artefacts the "ship" checkpoint itself writes - their
	// presence would mean the corresponding checkpoint ran unrequested.
	for _, unexpected := range []string{"arch.md", "breakdown.md", "test-stubs.md"} {
		if _, err := os.Stat(filepath.Join(specDir, unexpected)); err == nil {
			t.Errorf("%s exists - an unrequested checkpoint ran during a Names=[\"ship\"] invocation", unexpected)
		}
	}
}

// 3. Regression guard - a full-pipeline run (Names empty) must still execute
// and report all 7 checkpoints, unaffected by the single-checkpoint guard.
func TestRunWithOptions_FullPipeline_StillRunsAllCheckpoints(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mock := &llmprovider.MockProvider{Response: mockResponse("")}
	res := RunWithOptions(RunOptions{
		Root:        root,
		Description: "full pipeline still works",
		LLMPipe:     mockPipe(root, mock),
	})
	if len(res.Checkpoints) != 7 {
		t.Fatalf("expected 7 checkpoints on a full-pipeline run, got %d", len(res.Checkpoints))
	}
}

// â”€â”€ testTimestampGuard unit tests â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€
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

// TC-TTG-01: non-git directory â†’ guard returns nil immediately.
func TestTimestampGuard_NotGitRepo_ReturnsNil(t *testing.T) {
	t.Parallel()
	got := testTimestampGuard(t.TempDir())
	if got != nil {
		t.Fatalf("expected nil for non-git dir, got %v", got)
	}
}

// TC-TTG-02: clean working tree â†’ no dirty files â†’ guard returns nil.
func TestTimestampGuard_CleanWorkingTree_ReturnsNil(t *testing.T) {
	skipIfNoGitShip(t)
	t.Parallel()
	root := initShipRepo(t)
	got := testTimestampGuard(root)
	if got != nil {
		t.Fatalf("expected nil for clean working tree, got %v", got)
	}
}

// TC-TTG-03: production file dirty, corresponding test file NOT dirty â†’ violation.
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

// TC-TTG-04: both production and test files are dirty â†’ no violation.
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

// TC-TTG-05: dirty production file has no corresponding test file â†’ not a violation.
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
			t.Fatalf("baz.go has no test file â€” should not be a violation; got %v", got)
		}
	}
}

// TC-TTG-06: only the test file is dirty â†’ no violation (updating tests alone is fine).
func TestTimestampGuard_TestFileOnlyDirty_NoViolation(t *testing.T) {
	skipIfNoGitShip(t)
	t.Parallel()
	root := initShipRepo(t)
	commitFiles(t, root, map[string]string{
		"qux.go":      "package main\nfunc Qux() {}\n",
		"qux_test.go": "package main\nimport \"testing\"\nfunc TestQux(t *testing.T) {}\n",
	})
	// Only the test file is modified â€” this is fine, no TDD violation.
	os.WriteFile(filepath.Join(root, "qux_test.go"), []byte("package main\nimport \"testing\"\nfunc TestQux(t *testing.T) { t.Skip() }\n"), 0o644) //nolint:errcheck
	got := testTimestampGuard(root)
	if len(got) != 0 {
		t.Fatalf("expected no violations when only test file is dirty, got %v", got)
	}
}

// TC-RENDER-01: warning checkpoint uses â–³ (U+25B3) marker, not garbled bytes.
// Regression for the Consolas-font mojibake bug where âš  (U+26A0) was
// double-encoded and rendered as "Ã¢Ëœ"" in PowerShell / cmd.exe.
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
	// Must contain the â–³ triangle (U+25B3), not any garbled multi-byte sequence.
	if !strings.Contains(got, "\u25b3") {
		t.Fatalf("warning marker must be â–³ (U+25B3); got output:\n%s", got)
	}
	for _, garbled := range []string{"\u00e2\u0160\u02dc", "\u00e2\u0161 ", "Ã¢Ëœ"} {
		if strings.Contains(got, garbled) {
			t.Fatalf("warning marker must not contain garbled bytes %q; got output:\n%s", garbled, got)
		}
	}
}

// TC-RENDER-02: interactive gate warning marker is also â–³ (U+25B3).
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
		t.Fatalf("interactive gate warning marker must be â–³ (U+25B3); got output:\n%s", got)
	}
}

// -- checkQAVerify tests
func TestCheckQAVerify_NoRunner(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	cp := checkQAVerify(root, "test feature", "", nil)
	if cp.Status != "warning" {
		t.Errorf("Status: want warning, got %q detail: %s", cp.Status, cp.Detail)
	}
	if cp.Status == "fail" {
		t.Error("False-positive guard: must not be fail")
	}
}

// TestRunQATestSuite_NodeProject_RunsNpmTest — regression test for a real gap:
// runQATestSuite had no case for Node/TypeScript projects at all (only Go and
// Python), so any package.json-based project always fell through to "no MCP
// server or test runner found" regardless of how many real tests it had.
// A package.json with a non-empty "test" script must now be picked up and
// run via `npm test --silent`.
func TestRunQATestSuite_NodeProject_RunsNpmTest(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	pkgJSON := `{"name":"fixture","version":"1.0.0","scripts":{"test":"node -e \"console.log('Tests: 3 passed, 3 total')\""}}`
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(pkgJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	status, detail := runQATestSuite(root)

	if status != "ok" {
		t.Fatalf("status: want ok, got %q detail: %s", status, detail)
	}
	if !strings.Contains(detail, "npm test") {
		t.Errorf("detail should mention npm test: %s", detail)
	}
	if !strings.Contains(detail, "3 case(s) passed") {
		t.Errorf("detail should report the parsed passed-count: %s", detail)
	}
}

// TestRunQATestSuite_NodeProject_ParsesPassedCountWithTodoPrefix — regression
// test for a real bug found while dogfooding: real Jest output always prints
// a "Test Suites: N passed, N total" line BEFORE the "Tests: ..." line, and
// the "Tests:" line itself isn't always "Tests: N passed" — a preceding
// "N todo," or "N failed," group is common (e.g. "Tests: 6 todo, 4118
// passed, 4124 total"). Two bugs compounded: an unanchored "(\d+)\s+passed"
// search matches the Test Suites count (e.g. 296) instead of the real
// per-test count (4118), and a "Tests:" anchor without accounting for the
// "todo"/"failed" prefix matched nothing at all, reporting 0. Verified
// against a fixture that reproduces the exact multi-line shape Jest emits.
func TestRunQATestSuite_NodeProject_ParsesPassedCountWithTodoPrefix(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	// Reproduce the multi-line shape via a script FILE rather than an
	// inline `node -e "..."` command — hand-quoting a multi-line string
	// through npm's script -> shell -> node argv chain is fragile and
	// platform-dependent (cmd.exe's quoting rules differ from POSIX
	// shells), and isn't what this test is trying to verify anyway.
	jestLikeOutput := "Test Suites: 296 passed, 296 total\nTests:       6 todo, 4118 passed, 4124 total\n"
	fakeJestScript := "console.log(" + strconv.Quote(jestLikeOutput) + ");"
	if err := os.WriteFile(filepath.Join(root, "fake-jest-output.js"), []byte(fakeJestScript), 0o644); err != nil {
		t.Fatal(err)
	}

	pkg := map[string]any{
		"name":    "fixture",
		"version": "1.0.0",
		"scripts": map[string]string{"test": "node fake-jest-output.js"},
	}
	pkgJSON, err := json.Marshal(pkg)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "package.json"), pkgJSON, 0o644); err != nil {
		t.Fatal(err)
	}

	status, detail := runQATestSuite(root)

	if status != "ok" {
		t.Fatalf("status: want ok, got %q detail: %s", status, detail)
	}
	if !strings.Contains(detail, "4118 case(s) passed") {
		t.Errorf("detail should report the Tests: line's 4118, not the Test Suites: line's 296: %s", detail)
	}
}

// TestRunQATestSuite_NodeProject_NoTestScript_FallsThrough — false-positive
// guard: a package.json with no "test" script (common for pure libraries or
// mid-scaffold projects) must NOT be treated as a Node test runner — `npm
// test` with no script defined just prints an npm error and exits 1, which
// would be indistinguishable from a real failing suite if not guarded against
// up front.
func TestRunQATestSuite_NodeProject_NoTestScript_FallsThrough(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	pkgJSON := `{"name":"fixture","version":"1.0.0","scripts":{"build":"tsc"}}`
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(pkgJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	status, detail := runQATestSuite(root)

	if status != "warning" || detail != "" {
		t.Fatalf("expected the no-runner-found fallback (warning, \"\"), got status=%q detail=%q", status, detail)
	}
}

// ── Regression: an agent-mode pause must not be treated as an LLM failure ─────
//
// Root cause: checkSpec/checkArch/checkBreakdown/checkCode/checkTest all
// funnelled generateWithValidation's error straight into their generic "LLM
// failed, write a stub" branch. IsAgentTurn(err) was never checked, so a
// bridge miss (a *pause*, not a failure) was indistinguishable from a real
// provider error, and the checkpoint clobbered whatever artefact was on disk
// with a stub template. A second, independent bug in this file's own
// per-checkpoint post-processing loop compounded it: because the pause
// reported cp.Status == "ok", the completion-marker writer ran anyway and
// wrote placeholder content into the checkpoint's own primary artefact file
// (e.g. arch.md) before the host agent had answered anything — so the next
// invocation saw that file as "already done" and moved on, repeating the
// mistake on the next checkpoint. Net effect: a single run could silently
// stamp broken stubs across spec/arch/test/breakdown/code instead of pausing
// once per checkpoint for a real answer.
func TestAgentMode_ArchPauseDoesNotStubTheArtefact(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	runShipAgentOnce := func() error {
		cmd := New()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{"add rate limiting", "--root", root, "--agent-mode"})
		err := cmd.Execute()
		if !IsAgentTurn(err) {
			t.Fatalf("expected a pause, got %v\n%s", err, out.String())
		}
		return err
	}

	// Turn 1: spec pauses.
	_ = runShipAgentOnce()
	b, _ := agentbridge.Open(root, agentbridge.DefaultSession)
	if _, err := b.Fulfil("# Spec\n\n## Acceptance Criteria\n- [ ] works\n"); err != nil {
		t.Fatalf("Fulfil spec: %v", err)
	}

	slug := "add-rate-limiting"
	assertNoStubsYet := func() {
		t.Helper()
		for _, artefact := range []string{"arch.md", "test-stubs.md", "breakdown.md", "code-plan.md"} {
			p := filepath.Join(root, ".forge", "specs", slug, artefact)
			if fi, statErr := os.Stat(p); statErr == nil {
				data, _ := os.ReadFile(p)
				t.Fatalf("%s must not exist yet (size=%d) — the pipeline should have paused for a real "+
					"answer instead of writing a stub while a turn was still owed:\n%s", artefact, fi.Size(), string(data))
			}
		}
	}

	// Drive the run forward, answering whatever turn comes up (spec.md
	// existing now routes through a review turn before arch), until arch's
	// own generation turn is reached. At every step, no *later* checkpoint's
	// artefact must have been stubbed out while a prior turn was still owed.
	const maxTurns = 5
	reachedArch := false
	for i := 0; i < maxTurns; i++ {
		_ = runShipAgentOnce()

		bn, _ := agentbridge.Open(root, agentbridge.DefaultSession)
		pending, ok := bn.Pending()
		if !ok {
			t.Fatalf("turn %d: expected a pending turn", i)
		}
		assertNoStubsYet()
		if pending.Operation == "ship:arch:generate" {
			reachedArch = true
			break
		}
		if _, err := bn.Fulfil("placeholder host-agent answer for " + pending.Operation); err != nil {
			t.Fatalf("turn %d: Fulfil %s: %v", i, pending.Operation, err)
		}
	}
	if !reachedArch {
		t.Fatalf("never reached the arch generation turn within %d turns", maxTurns)
	}
}
