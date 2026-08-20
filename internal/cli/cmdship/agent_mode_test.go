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
//  1. Happy path           — a real `forge ship --agent-mode` run pauses,
//     takes an answer, and advances on re-run.
//  2. Boundary             — --json emits the machine-readable turn payload.
//  3. Negative             — a pause is not reported as a checkpoint failure.
//  4. Idempotency          — re-running while paused re-presents the same turn.
//  5. Concurrency          — agent mode serialises the arch/test DAG.
//  6. Cross-cutting        — exit code 78 distinguishes "your turn" from
//     "the change is broken".
//  7. Regression           — an API key in the environment must NOT be used
//     when the user asked for agent mode; being billed
//     after opting out is the exact failure this
//     feature exists to prevent.
//  8. Data-accuracy        — the rendered turn carries the compiled prompt.
//  9. False-positive guard — a run with no bridge is completely unaffected.
package cmdship

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/teragrid/forge/internal/agentbridge"
	"github.com/teragrid/forge/internal/errcode"
	"github.com/teragrid/forge/internal/llmprovider"
)

// runShipAgent executes `forge ship spec <desc> --agent-mode` against root and
// returns combined output plus the error (which is the pause sentinel when a
// turn is owed).
func runShipAgent(t *testing.T, root string, extra ...string) (string, error) {
	t.Helper()
	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	args := append([]string{"spec", "add rate limiting", "--root", root, "--agent-mode"}, extra...)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

// ── Happy path: the full driver loop ──────────────────────────────────────────

func TestAgentMode_PausesThenAdvancesOnSubmittedAnswer(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	out, err := runShipAgent(t, root)
	if err == nil {
		t.Fatalf("expected the run to pause for a turn; output:\n%s", out)
	}
	if !IsAgentTurn(err) {
		t.Fatalf("pause must be reported as an agent turn, not a failure: %v", err)
	}
	if !strings.Contains(out, "FORGE AGENT TURN") {
		t.Fatalf("expected the turn block on stdout:\n%s", out)
	}
	if !strings.Contains(out, "forge agent submit") {
		t.Fatalf("the turn must tell the host agent how to answer:\n%s", out)
	}

	// Answer it exactly as `forge agent submit` would.
	b, err := agentbridge.Open(root, agentbridge.DefaultSession)
	if err != nil {
		t.Fatalf("open bridge: %v", err)
	}
	turn, ok := b.Pending()
	if !ok {
		t.Fatal("the paused run should have persisted a pending turn")
	}
	if turn.System == "" || turn.User == "" {
		t.Fatalf("the persisted turn must carry the compiled prompt: %+v", turn)
	}
	const answer = "# Rate limiting\n\n## Acceptance criteria\n\n- [ ] Requests over the limit get 429\n"
	if _, err := b.Fulfil(answer); err != nil {
		t.Fatalf("Fulfil: %v", err)
	}

	// Re-run: the recorded answer is replayed, so this operation no longer
	// pauses. The run either completes or pauses on a *later* operation —
	// either way it must have moved past the one just answered.
	out2, err2 := runShipAgent(t, root)
	b2, _ := agentbridge.Open(root, agentbridge.DefaultSession)
	if next, paused := b2.Pending(); paused && next.Ordinal == turn.Ordinal {
		t.Fatalf("the answered turn was asked again — the pipeline did not advance\nerr=%v\n%s", err2, out2)
	}
}

// ── Idempotency ───────────────────────────────────────────────────────────────

func TestAgentMode_RerunWhilePausedRepresentsTheSameTurn(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	if _, err := runShipAgent(t, root); !IsAgentTurn(err) {
		t.Fatalf("first run should pause, got %v", err)
	}
	b, _ := agentbridge.Open(root, agentbridge.DefaultSession)
	first, _ := b.Pending()

	// Re-running without answering must be safe and must not silently
	// advance, skip, or fabricate the missing artefact.
	if _, err := runShipAgent(t, root); !IsAgentTurn(err) {
		t.Fatalf("second run should pause identically, got %v", err)
	}
	b2, _ := agentbridge.Open(root, agentbridge.DefaultSession)
	second, ok := b2.Pending()
	if !ok || second.Hash != first.Hash {
		t.Fatalf("re-run changed the pending turn: %q → %q", first.Ordinal, second.Ordinal)
	}
}

// ── Boundary: machine-readable output ─────────────────────────────────────────

func TestAgentMode_JSONEmitsTurnPayload(t *testing.T) {
	t.Parallel()
	out, err := runShipAgent(t, t.TempDir(), "--json")
	if !IsAgentTurn(err) {
		t.Fatalf("expected a pause, got %v", err)
	}
	for _, want := range []string{
		`"status": "agent_turn_required"`,
		`"submit_cmd"`,
		`"resume_cmd"`,
		`"exit_code": 78`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("JSON payload missing %s:\n%s", want, out)
		}
	}
}

// ── Cross-cutting: the exit code ──────────────────────────────────────────────

func TestIsAgentTurn_DistinguishesPauseFromFailure(t *testing.T) {
	t.Parallel()
	if ExitAgentTurn == 1 || ExitAgentTurn == 0 {
		t.Fatalf("the pause exit code must be distinguishable from success and failure, got %d", ExitAgentTurn)
	}
	_, err := runShipAgent(t, t.TempDir())
	if !IsAgentTurn(err) {
		t.Fatalf("a paused run must map to ExitAgentTurn, got %v", err)
	}
	// A genuine ship failure must not be mistaken for a turn.
	if IsAgentTurn(errcode.New(ErrShipFailed, "a real failure", nil)) {
		t.Fatal("a real checkpoint failure must not be treated as a pause")
	}
}

// ── Regression: agent mode must not bill a detected API key ───────────────────

func TestAgentMode_IgnoresAnInjectedProvider(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	// A MockProvider stands in for a real key present in the environment.
	// Agent mode must route around it entirely: a user who opted out of
	// paying must not be charged because forge found a key lying about.
	mock := &llmprovider.MockProvider{}
	bridge, err := agentbridge.Open(root, agentbridge.DefaultSession)
	if err != nil {
		t.Fatalf("open bridge: %v", err)
	}
	res := RunWithOptions(RunOptions{
		Root:        root,
		Description: "add rate limiting",
		Names:       []string{"spec"},
		LLMPipe:     newLLMPipeWithProvider(mock, root),
		AgentBridge: bridge,
	})
	if res == nil {
		t.Fatal("expected a result")
	}
	if mock.Calls() != 0 {
		t.Fatalf("agent mode dialled the provider %d time(s); it must never do so", mock.Calls())
	}
	if !bridge.Paused() {
		t.Fatal("the run should have deferred a turn to the host agent")
	}
}

// ── Regression: a pause must not be treated as an LLM failure ─────────────────
//
// Root cause fixed here: checkSpec/checkArch/checkBreakdown/checkCode/checkTest
// all funnelled generateWithValidation's error straight into their generic
// "LLM failed, write a stub" branch. IsAgentTurn(err) was never checked, so a
// bridge miss (a *pause*, not a failure) was indistinguishable from a real
// provider error: the checkpoint clobbered whatever artefact was on disk with
// a stub template, logged a false failure, and — because the bridge's pause
// latch is set inside Lookup regardless — every checkpoint gated by
// agentPaused() correctly stopped running in *that* invocation, but the
// artefact files were already corrupted by the time it did. On the next
// invocation, the stubbed file made the checkpoint look "already done" and
// the pipeline moved on to the next checkpoint, which repeated the same
// mistake — net effect: a single run silently stamped broken stubs across
// spec/arch/test/breakdown/code instead of pausing once per checkpoint for a
// real answer.
func TestAgentMode_ArchPauseDoesNotStubTheArtefact(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	// Turn 1: spec pauses.
	if _, err := runShipAgent(t, root); !IsAgentTurn(err) {
		t.Fatalf("expected spec to pause, got %v", err)
	}
	b, _ := agentbridge.Open(root, agentbridge.DefaultSession)
	if _, err := b.Fulfil("# Spec\n\n## Acceptance Criteria\n- [ ] works\n"); err != nil {
		t.Fatalf("Fulfil spec: %v", err)
	}

	slug := "add-rate-limiting"
	assertNoStubsYet := func(except ...string) {
		t.Helper()
		skip := map[string]bool{}
		for _, s := range except {
			skip[s] = true
		}
		for _, artefact := range []string{"arch.md", "test-stubs.md", "breakdown.md", "code-plan.md"} {
			if skip[artefact] {
				continue
			}
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
		cmd := New()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{"add rate limiting", "--root", root, "--agent-mode"})
		err := cmd.Execute()
		if !IsAgentTurn(err) {
			t.Fatalf("turn %d: expected a pause, got %v\n%s", i, err, out.String())
		}

		bn, _ := agentbridge.Open(root, agentbridge.DefaultSession)
		pending, ok := bn.Pending()
		if !ok {
			t.Fatalf("turn %d: expected a pending turn", i)
		}
		if pending.Operation == "ship:arch:generate" {
			assertNoStubsYet()
			reachedArch = true
			break
		}
		assertNoStubsYet()
		if _, err := bn.Fulfil("placeholder host-agent answer for " + pending.Operation); err != nil {
			t.Fatalf("turn %d: Fulfil %s: %v", i, pending.Operation, err)
		}
	}
	if !reachedArch {
		t.Fatalf("never reached the arch generation turn within %d turns", maxTurns)
	}
}

// ── Regression: reusing a session across features must not cross-contaminate ──
//
// Root cause fixed here: Bridge.SetFeature recorded the new feature/slug but
// never checked whether the session already belonged to a different feature.
// The ordinal fallback in Lookup is keyed only on "operation#N" with no
// feature scoping, so replaying "default" session state into a second,
// unrelated feature could silently serve the first feature's recorded answer
// for the second feature's Nth call to the same operation — e.g. its
// qa-verify tasks — instead of asking a fresh question.
func TestAgentMode_SwitchingFeatureResetsStaleSession(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	b, err := agentbridge.Open(root, agentbridge.DefaultSession)
	if err != nil {
		t.Fatalf("open bridge: %v", err)
	}
	if switched := b.SetFeature("agency master-calendar", "agency-master-calendar"); switched {
		t.Fatal("first SetFeature call on a fresh session must not report a switch")
	}
	if _, err := b.Lookup("ship:qa-verify:generate", "qa-verify", "", "sys", "usr", 100); !errors.Is(err, agentbridge.ErrTurnRequired) {
		t.Fatalf("expected a pause, got %v", err)
	}
	if _, err := b.Fulfil("master-calendar tasks answer"); err != nil {
		t.Fatalf("Fulfil: %v", err)
	}
	if got := b.Stats().Responses; got != 1 {
		t.Fatalf("expected 1 recorded response before the switch, got %d", got)
	}

	// Reopen as a fresh process would, then point the same default session at
	// an unrelated feature.
	b2, err := agentbridge.Open(root, agentbridge.DefaultSession)
	if err != nil {
		t.Fatalf("reopen bridge: %v", err)
	}
	if switched := b2.SetFeature("checkout redesign", "checkout-redesign"); !switched {
		t.Fatal("SetFeature must report a switch when the session already belonged to a different feature")
	}
	if got := b2.Stats().Responses; got != 0 {
		t.Fatalf("switching features must discard the prior feature's recorded answers, %d remain", got)
	}
	// The Nth call (N=1) to the same operation for the new feature must ask a
	// fresh question rather than replaying the old feature's answer via the
	// ordinal fallback.
	content, lookupErr := b2.Lookup("ship:qa-verify:generate", "qa-verify", "", "sys", "usr", 100)
	if !errors.Is(lookupErr, agentbridge.ErrTurnRequired) {
		t.Fatalf("expected a fresh pause for the new feature, got content=%q err=%v", content, lookupErr)
	}
}

// ── False-positive guard: no bridge, no behaviour change ──────────────────────

func TestNoBridge_PipelineUnaffected(t *testing.T) {
	t.Parallel()
	res := RunWithOptions(RunOptions{
		Root:        t.TempDir(),
		Description: "add rate limiting",
		Names:       []string{"spec"},
	})
	if res == nil || len(res.Checkpoints) == 0 {
		t.Fatal("a run without agent mode must behave exactly as before")
	}
}
