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

// no_color_gate_test.go — NO_COLOR must not decide whether a human reviews.
//
// NO_COLOR is defined (no-color.org) as a purely presentational signal: it asks
// software not to emit ANSI colour. People set it for accessibility, for
// terminals that render escape codes badly, or simply preference — and they set
// it globally, in a shell profile, once.
//
// forge ship used to read it as "an LLM is driving me" and auto-approve every
// checkpoint. Those users silently lost every approval gate in the pipeline and
// nothing in the output said so. A signal about how text is *displayed* was
// deciding whether a change gets reviewed.
package cmdship

import (
	"bytes"
	"strings"
	"testing"
)

// runShipWithStdin drives the full pipeline with a controlled stdin so the
// interactive gate's behaviour is observable: "n" at the first gate stops the
// run, which only happens if the gate was actually installed.
func runShipWithStdin(t *testing.T, root, stdin string) (string, error) {
	t.Helper()
	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetIn(strings.NewReader(stdin))
	cmd.SetArgs([]string{"--root", root, "--no-strict-testing"})
	err := cmd.Execute()
	return out.String(), err
}

// ── Regression ────────────────────────────────────────────────────────────────

func TestNoColor_DoesNotSuppressApprovalGates(t *testing.T) {
	// Not parallel: mutates process environment.
	t.Setenv("NO_COLOR", "1")
	t.Setenv("FORGE_LLM_MODE", "")

	// "n" at the first gate must stop the pipeline. If NO_COLOR still forced
	// yolo, no gate would be installed, the "n" would go unread, and the run
	// would sail through all seven checkpoints.
	out, err := runShipWithStdin(t, t.TempDir(), "n\n")
	if err == nil {
		t.Fatalf("NO_COLOR=1 suppressed the approval gates — a presentational "+
			"env var must not decide whether a human reviews the change.\n%s", out)
	}
	if !strings.Contains(strings.ToLower(out), "rejected") {
		t.Fatalf("expected the run to stop at a rejected gate:\n%s", out)
	}
}

func TestNoColor_StillAllowsANormalApprovedRun(t *testing.T) {
	// False-positive guard: restoring the gate must not break the ordinary
	// path. Approving every checkpoint still completes the pipeline.
	t.Setenv("NO_COLOR", "1")
	t.Setenv("FORGE_LLM_MODE", "")

	out, err := runShipWithStdin(t, t.TempDir(), "y\ny\ny\ny\ny\ny\n")
	if err != nil {
		t.Fatalf("approving every gate should complete the run: %v\n%s", err, out)
	}
}

// ── The remaining suppression path must announce itself ───────────────────────

func TestForgeLLMMode_SuppressesGatesButSaysSo(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	t.Setenv("FORGE_LLM_MODE", "1")

	// FORGE_LLM_MODE is forge's own explicit signal and does mean "no human is
	// here", so it still suppresses gates. But skipping review must never
	// happen quietly, even when it is correct — otherwise the next person to
	// inherit this env var has no way to discover why nothing ever asks them.
	out, err := runShipWithStdin(t, t.TempDir(), "n\n")
	if err != nil {
		t.Fatalf("FORGE_LLM_MODE=1 should auto-approve, not fail: %v\n%s", err, out)
	}
	if !strings.Contains(out, "FORGE_LLM_MODE=1") {
		t.Fatalf("gate suppression must be announced, not silent:\n%s", out)
	}
}

// ── --human still wins ────────────────────────────────────────────────────────

func TestHumanFlag_OverridesForgeLLMMode(t *testing.T) {
	t.Setenv("FORGE_LLM_MODE", "1")

	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetIn(strings.NewReader("n\n"))
	cmd.SetArgs([]string{"--root", t.TempDir(), "--human", "--no-strict-testing"})
	if err := cmd.Execute(); err == nil {
		t.Fatalf("--human is the explicit opt-out and must reinstate the gates:\n%s", out.String())
	}
}
