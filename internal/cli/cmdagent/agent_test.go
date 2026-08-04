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
//  1. Happy path           — status → prompt → submit round-trip.
//  2. Boundary             — stdin submit; --json on every read command.
//  3. Negative             — submit/prompt with nothing pending; no answer given.
//  4. Idempotency          — prompt is repeatable and holds no state.
//  5. Concurrency          — --session isolates two conversations.
//  6. Cross-cutting        — reset refuses to destroy answers without --yes.
//  7. Regression           — the driver protocol keeps the "you don't decide
//     gate outcomes" rule, which is the whole reason
//     agent mode is safe to ship.
//  8. Data-accuracy        — submitted content is recorded byte-for-byte.
//  9. False-positive guard — sessions listing is empty, not fabricated.
package cmdagent

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/teragrid/forge/internal/agentbridge"
)

// run executes `forge agent <args...>` and returns combined output.
func run(t *testing.T, stdin string, args ...string) (string, error) {
	t.Helper()
	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetIn(strings.NewReader(stdin))
	cmd.SetArgs(args)
	return outAndErr(&out, cmd.Execute())
}

func outAndErr(buf *bytes.Buffer, err error) (string, error) { return buf.String(), err }

// seedPendingTurn puts root into the state a paused `forge ship --agent-mode`
// leaves behind.
func seedPendingTurn(t *testing.T, root, session, op string) agentbridge.Turn {
	t.Helper()
	b, err := agentbridge.Open(root, session)
	if err != nil {
		t.Fatalf("open bridge: %v", err)
	}
	if _, err := b.Lookup(op, "spec", "", "SYSTEM-TEXT", "USER-TEXT", 8000); err == nil {
		t.Fatal("expected the seeded lookup to defer a turn")
	}
	turn, _ := b.Pending()
	return turn
}

// ── Happy path ────────────────────────────────────────────────────────────────

func TestSubmit_RecordsAnswerAndPointsAtTheResume(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	seedPendingTurn(t, root, agentbridge.DefaultSession, "spec.generate")

	answer := filepath.Join(root, "answer.md")
	const content = "# Rate limiting\n\n- [ ] 429 over the limit\n"
	if err := os.WriteFile(answer, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := run(t, "", "submit", "--root", root, "--file", answer)
	if err != nil {
		t.Fatalf("submit: %v\n%s", err, out)
	}
	if !strings.Contains(out, "forge ship --agent-mode") {
		t.Fatalf("submit must tell the driver how to continue:\n%s", out)
	}

	// Data accuracy: the recorded answer replays byte-for-byte.
	b, _ := agentbridge.Open(root, agentbridge.DefaultSession)
	got, err := b.Lookup("spec.generate", "spec", "", "SYSTEM-TEXT", "USER-TEXT", 8000)
	if err != nil {
		t.Fatalf("recorded answer did not replay: %v", err)
	}
	if got != content {
		t.Fatalf("content mismatch:\n got %q\nwant %q", got, content)
	}
}

func TestSubmit_FromStdin(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	seedPendingTurn(t, root, agentbridge.DefaultSession, "spec.generate")

	if _, err := run(t, "# Spec from stdin\n", "submit", "-", "--root", root); err != nil {
		t.Fatalf("stdin submit: %v", err)
	}
	b, _ := agentbridge.Open(root, agentbridge.DefaultSession)
	got, err := b.Lookup("spec.generate", "spec", "", "SYSTEM-TEXT", "USER-TEXT", 8000)
	if err != nil || got != "# Spec from stdin\n" {
		t.Fatalf("stdin answer not recorded: got %q err %v", got, err)
	}
}

// ── prompt: idempotent, self-contained ────────────────────────────────────────

func TestPrompt_IsRepeatableAndCarriesTheWholeQuestion(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	seedPendingTurn(t, root, agentbridge.DefaultSession, "spec.generate")

	first, err := run(t, "", "prompt", "--root", root)
	if err != nil {
		t.Fatalf("prompt: %v", err)
	}
	second, err := run(t, "", "prompt", "--root", root)
	if err != nil {
		t.Fatalf("second prompt: %v", err)
	}
	if first != second {
		t.Fatal("prompt must be idempotent — a compacted chat re-reads it freely")
	}
	// Everything needed to answer without remembering the previous turn.
	for _, want := range []string{"SYSTEM-TEXT", "USER-TEXT", "forge agent submit", "8000"} {
		if !strings.Contains(first, want) {
			t.Fatalf("prompt missing %q:\n%s", want, first)
		}
	}
}

func TestPrompt_JSON(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	seedPendingTurn(t, root, agentbridge.DefaultSession, "spec.generate")
	out, err := run(t, "", "prompt", "--root", root, "--json")
	if err != nil {
		t.Fatalf("prompt --json: %v", err)
	}
	for _, want := range []string{`"operation"`, `"system"`, `"user"`, `"max_tokens"`} {
		if !strings.Contains(out, want) {
			t.Fatalf("JSON turn missing %s:\n%s", want, out)
		}
	}
}

// ── Negative ──────────────────────────────────────────────────────────────────

func TestPrompt_NothingPending(t *testing.T) {
	t.Parallel()
	if _, err := run(t, "", "prompt", "--root", t.TempDir()); err == nil {
		t.Fatal("prompt with no pending turn must fail, not print an empty block")
	}
}

func TestSubmit_NothingPending(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	answer := filepath.Join(root, "a.md")
	if err := os.WriteFile(answer, []byte("# x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := run(t, "", "submit", "--root", root, "--file", answer); err == nil {
		t.Fatal("submitting with nothing pending must fail rather than record a stray answer")
	}
}

func TestSubmit_NoAnswerSupplied(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	seedPendingTurn(t, root, agentbridge.DefaultSession, "spec.generate")
	// Neither --file nor "-": stdin is never read implicitly, because a chat
	// tool that leaves stdin attached but silent would hang the command.
	if _, err := run(t, "ignored", "submit", "--root", root); err == nil {
		t.Fatal("submit with no source must fail fast")
	}
}

// ── Cross-cutting: reset guards recorded work ─────────────────────────────────

func TestReset_RequiresConfirmation(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	seedPendingTurn(t, root, agentbridge.DefaultSession, "spec.generate")
	answer := filepath.Join(root, "a.md")
	if err := os.WriteFile(answer, []byte("# Spec\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := run(t, "", "submit", "--root", root, "--file", answer); err != nil {
		t.Fatalf("submit: %v", err)
	}

	if _, err := run(t, "", "reset", "--root", root); err == nil {
		t.Fatal("reset must refuse to discard recorded answers without --yes")
	}
	// The answer survived the refusal.
	b, _ := agentbridge.Open(root, agentbridge.DefaultSession)
	if b.Stats().Responses != 1 {
		t.Fatalf("a refused reset must not delete anything, got %d responses", b.Stats().Responses)
	}

	if _, err := run(t, "", "reset", "--root", root, "--yes"); err != nil {
		t.Fatalf("confirmed reset: %v", err)
	}
	b2, _ := agentbridge.Open(root, agentbridge.DefaultSession)
	if b2.Stats().Responses != 0 {
		t.Fatalf("confirmed reset should have cleared answers, got %d", b2.Stats().Responses)
	}
}

// ── Concurrency: sessions ─────────────────────────────────────────────────────

func TestSessions_AreIsolatedAndListed(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	seedPendingTurn(t, root, "feature-a", "spec.generate")
	seedPendingTurn(t, root, "feature-b", "spec.generate")

	answer := filepath.Join(root, "a.md")
	if err := os.WriteFile(answer, []byte("# A\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := run(t, "", "submit", "--root", root, "--session", "feature-a", "--file", answer); err != nil {
		t.Fatalf("submit to feature-a: %v", err)
	}

	// feature-b must still be owed its turn: answering one conversation must
	// never silently answer the other.
	bB, _ := agentbridge.Open(root, "feature-b")
	if _, ok := bB.Pending(); !ok {
		t.Fatal("feature-b's turn was consumed by feature-a's answer")
	}

	out, err := run(t, "", "sessions", "--root", root)
	if err != nil {
		t.Fatalf("sessions: %v", err)
	}
	if !strings.Contains(out, "feature-a") || !strings.Contains(out, "feature-b") {
		t.Fatalf("both sessions should be listed:\n%s", out)
	}
}

// ── False-positive guard ──────────────────────────────────────────────────────

func TestSessions_EmptyProjectSaysSo(t *testing.T) {
	t.Parallel()
	out, err := run(t, "", "sessions", "--root", t.TempDir())
	if err != nil {
		t.Fatalf("sessions: %v", err)
	}
	if !strings.Contains(out, "no agent-mode sessions") {
		t.Fatalf("an empty project should say so plainly:\n%s", out)
	}
}

func TestStatus_ReportsPendingAndClean(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	clean, err := run(t, "", "status", "--root", root)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if !strings.Contains(clean, "pending      none") {
		t.Fatalf("a fresh project has nothing pending:\n%s", clean)
	}

	seedPendingTurn(t, root, agentbridge.DefaultSession, "spec.generate")
	busy, err := run(t, "", "status", "--root", root)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if !strings.Contains(busy, "spec.generate#1") {
		t.Fatalf("status should name the pending turn:\n%s", busy)
	}
}

// ── Regression: the safety rule must stay in the protocol ─────────────────────

func TestDriverProtocol_KeepsTheGateAuthorityRule(t *testing.T) {
	t.Parallel()
	p := DriverProtocol()
	// Agent mode is only safe because the host agent supplies text and never
	// verdicts. If this instruction is ever dropped from the protocol, a
	// driver will start self-reporting green gates — the exact failure the
	// two-plane split exists to prevent.
	if !strings.Contains(p, "Never report a gate as passing") {
		t.Fatalf("the gate-authority rule is missing from the driver protocol:\n%s", p)
	}
	for _, want := range []string{
		"forge ship \"<feature>\" --agent-mode",
		"forge agent submit --file",
		"forge agent status",
		"exits with code 78",
	} {
		if !strings.Contains(p, want) {
			t.Fatalf("driver protocol missing %q", want)
		}
	}
}

func TestLoop_PrintsTheProtocol(t *testing.T) {
	t.Parallel()
	out, err := run(t, "", "loop")
	if err != nil {
		t.Fatalf("loop: %v", err)
	}
	if out != DriverProtocol() {
		t.Fatal("forge agent loop must print the protocol verbatim")
	}
}
