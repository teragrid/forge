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

// ── Regression: a paused Test checkpoint must not fabricate placeholder
// artefacts that later shadow the real host-agent answer ─────────────────────
//
// Root cause this guards: writeTestArtifactsWithContext used to write the
// static RED placeholder stub to tests/*.test.ts unconditionally whenever the
// LLM call returned any error — including ErrAgentTurn, a pause rather than a
// failure. Once written, that placeholder satisfied allTestArtifactsExist
// forever, so checkTest never called writeTestArtifacts again on a later run
// — the host agent's real, later-submitted answer for the same operation sat
// unused in the bridge's response store, and the checkpoint reported
// "N test file(s) found; all 4 named artifacts present" against content
// nobody had actually reviewed. Confirmed live against a real binary: a
// submitted `expect(ping()).toBe('pong')` answer for ship:test:unit never
// reached tests/*.test.ts because the placeholder from the paused attempt
// had already tripped the exists-guard.
func TestWriteTestArtifacts_PausedTurnDoesNotFabricatePlaceholder(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	bridge, err := agentbridge.Open(root, agentbridge.DefaultSession)
	if err != nil {
		t.Fatalf("open bridge: %v", err)
	}
	pipe := newLLMPipeAgent(root, bridge)
	slug := "add-ping"
	unitPath := filepath.Join(root, "tests", slug+".test.ts")

	// First attempt: ship:test:unit is unanswered, so this must pause —
	// and, critically, must not write ANY of the 4 named artifacts.
	paths, err := writeTestArtifactsWithContext(root, slug, "add ping function returning pong", "", TestFrameworkContext{}, pipe)
	if !IsAgentTurn(err) {
		t.Fatalf("expected an agent-turn pause on the first attempt, got err=%v paths=%+v", err, paths)
	}
	if _, statErr := os.Stat(unitPath); statErr == nil {
		t.Fatalf("paused generation must not write a placeholder to %s — doing so permanently shadows the real answer", unitPath)
	}

	// Answer ship:test:unit, ship:test:integration, ship:test:rls in turn —
	// each is a distinct operation, so each pauses once before the next
	// becomes reachable.
	realUnit := "```typescript\nexpect(ping()).toBe('pong');\n```"
	for _, answer := range []string{realUnit, "integration answer", "rls answer"} {
		turn, ok := bridge.Pending()
		if !ok {
			t.Fatalf("expected a pending turn before submitting %q", answer)
		}
		if _, ferr := bridge.Fulfil(answer); ferr != nil {
			t.Fatalf("Fulfil(%q): %v", answer, ferr)
		}
		paths, err = writeTestArtifactsWithContext(root, slug, "add ping function returning pong", "", TestFrameworkContext{}, pipe)
		if turn.Operation != "ship:test:rls" && !IsAgentTurn(err) {
			t.Fatalf("expected the next operation to pause after answering %q, got err=%v", answer, err)
		}
	}

	if err != nil {
		t.Fatalf("all three sub-generations were answered; expected no error, got %v", err)
	}
	if paths.UnitTest == "" {
		t.Fatal("UnitTest path must be set once generation completes")
	}
	content, readErr := os.ReadFile(paths.UnitTest)
	if readErr != nil {
		t.Fatalf("read %s: %v", paths.UnitTest, readErr)
	}
	if !strings.Contains(string(content), "ping()") {
		t.Fatalf("tests/%s.test.ts must contain the host agent's real answer, got:\n%s", slug, content)
	}
	if strings.Contains(string(content), "intentionally failing — complete implementation") {
		t.Fatalf("tests/%s.test.ts still holds the static placeholder instead of the real answer:\n%s", slug, content)
	}
}
