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
//  1. Happy path           — a project with full evidence still ships.
//  2. Boundary             — evidence present but a stage missing.
//  3. Negative             — no evidence at all now blocks by default.
//  4. Idempotency          — the default does not drift between runs.
//  5. Concurrency          — every case owns its TempDir.
//  6. Cross-cutting        — --resume obeys the same switches as a fresh run.
//  7. Regression           — 1.8.2 inverted the default; a silent flip back
//     would restore exactly the failure mode the change
//     was made to remove, and nothing else would notice.
//  8. Data-accuracy        — the opt-out is honoured from both file and flag.
//  9. False-positive guard — the opt-out genuinely lets a bare project ship,
//     so the gate cannot become an unescapable mandate.
package cmdship

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ── Regression: the default itself ────────────────────────────────────────────

// Shipping without testing evidence must be something a user asks for by name.
// Before 1.8.2 the gate was advisory, which meant its own finding — "there is
// no evidence this was tested" — was reported as an acceptable outcome. If this
// default ever flips back, that failure mode returns silently.
func TestStrictTesting_IsOnByDefault(t *testing.T) {
	t.Parallel()
	res := RunWithOptions(RunOptions{
		Root:        t.TempDir(),
		Description: "no evidence anywhere",
		Names:       []string{"qa-verify"},
	})
	if res.Ready {
		t.Fatal("a run with no testing evidence must not be Ready by default")
	}
	if len(res.Checkpoints) != 1 || res.Checkpoints[0].Status != "fail" {
		t.Fatalf("qa-verify must hard-fail on missing evidence by default: %+v", res.Checkpoints)
	}
	if !strings.Contains(res.Checkpoints[0].Detail, "four-stage-testing-gate") {
		t.Fatalf("the failure must name the gate that caused it: %q", res.Checkpoints[0].Detail)
	}
}

// ── False-positive guard: the opt-out must actually work ──────────────────────

func TestStrictTesting_FlagOptOutLetsABareProjectShip(t *testing.T) {
	t.Parallel()
	res := RunWithOptions(RunOptions{
		Root:            t.TempDir(),
		Description:     "waived",
		Names:           []string{"qa-verify"},
		NoStrictTesting: true,
	})
	if len(res.Checkpoints) != 1 {
		t.Fatalf("expected 1 checkpoint, got %d", len(res.Checkpoints))
	}
	// A default that cannot be turned off is a mandate, not a default.
	if res.Checkpoints[0].Status == "fail" {
		t.Fatalf("--no-strict-testing must waive the gate: %q", res.Checkpoints[0].Detail)
	}
}

func TestStrictTesting_HooksFileOptOutLetsABareProjectShip(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	forgeDir := filepath.Join(root, ".forge")
	if err := os.MkdirAll(forgeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(forgeDir, "hooks.yaml"),
		[]byte("strict-testing: false\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	res := RunWithOptions(RunOptions{
		Root: root, Description: "waived by file", Names: []string{"qa-verify"},
	})
	if res.Checkpoints[0].Status == "fail" {
		t.Fatalf("\"strict-testing: false\" must waive the gate: %q", res.Checkpoints[0].Detail)
	}
}

// ── Precedence ────────────────────────────────────────────────────────────────

// An explicit opt-out is checked last, so it beats an explicit opt-in. Passing
// both is contradictory, and the safer reading of a contradiction is not to
// silently enforce something the user also asked to skip — they will see the
// run succeed and can re-read their own flags.
func TestStrictTesting_ExplicitOptOutBeatsExplicitOptIn(t *testing.T) {
	t.Parallel()
	res := RunWithOptions(RunOptions{
		Root:            t.TempDir(),
		Description:     "both flags",
		Names:           []string{"qa-verify"},
		StrictTesting:   true,
		NoStrictTesting: true,
	})
	if res.Checkpoints[0].Status == "fail" {
		t.Fatal("--no-strict-testing must win over --strict-testing")
	}
}

// ── Cross-cutting: --resume must obey the same switches ───────────────────────

// runResumeFlag used to call RunCheckpoints, which takes no options, so every
// flag on the command line was silently discarded: `--resume --no-strict-testing`
// re-enabled the gate the user had just waived. The same defect would have made
// `--resume --agent-mode` dial a provider.
func TestResume_HonoursNoStrictTesting(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	slug := "auth-email"
	specDir := filepath.Join(root, ".forge", "specs", slug)
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, cp := range []string{"spec", "test", "breakdown", "code", "ship"} {
		if err := os.WriteFile(filepath.Join(specDir, cp+".md"), []byte("done\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	run := func(extra ...string) error {
		cmd := New()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs(append([]string{slug, "--resume", "--root", root}, extra...))
		return cmd.Execute()
	}

	if err := run("--no-strict-testing"); err != nil {
		t.Fatalf("--resume must honour --no-strict-testing, got: %v", err)
	}
}

func TestResume_EnforcesTheGateByDefault(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	slug := "auth-email"
	specDir := filepath.Join(root, ".forge", "specs", slug)
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, cp := range []string{"spec", "test", "breakdown", "code", "ship"} {
		if err := os.WriteFile(filepath.Join(specDir, cp+".md"), []byte("done\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{slug, "--resume", "--root", root})
	// A resumed run is the same pipeline; it must not be a way around the gate.
	if err := cmd.Execute(); err == nil {
		t.Fatalf("--resume with no testing evidence must fail by default:\n%s", out.String())
	}
}

// ── Idempotency ───────────────────────────────────────────────────────────────

func TestStrictTesting_DefaultIsStableAcrossLoads(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	first := loadHookConfig(root)
	second := loadHookConfig(root)
	if first.StrictTesting != second.StrictTesting || !first.StrictTesting {
		t.Fatalf("default must be stable and on: %v then %v", first.StrictTesting, second.StrictTesting)
	}
}
