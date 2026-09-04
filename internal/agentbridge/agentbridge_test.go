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
//  1. Happy path           — miss defers a turn, submit records it, replay hits.
//  2. Boundary             — empty answer rejected; invalid session name rejected.
//  3. Negative             — Fulfil with no pending turn; nil-receiver safety.
//  4. Idempotency          — Lookup after Fulfil never re-asks; repeated Open is stable.
//  5. Concurrency          — sessions are isolated; each test owns a TempDir.
//  6. Cross-cutting        — pause latches, so one run yields exactly one turn.
//  7. Regression           — the drift fallback exists precisely to stop the
//     re-ask loop that pure content addressing causes
//     once the host agent's own writes change a prompt.
//  8. Data-accuracy        — recorded content round-trips byte-for-byte.
//  9. False-positive guard — a genuinely different operation is not served a
//     neighbouring answer.
package agentbridge

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func mustOpen(t *testing.T, root, session string) *Bridge {
	t.Helper()
	b, err := Open(root, session)
	if err != nil {
		t.Fatalf("Open(%q, %q): %v", root, session, err)
	}
	return b
}

// ── Happy path ────────────────────────────────────────────────────────────────

func TestLookup_MissDefersTurn_ThenReplays(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	b := mustOpen(t, root, DefaultSession)

	if _, err := b.Lookup("spec.generate", "spec", "", "SYS", "USR", 8000); !errors.Is(err, ErrTurnRequired) {
		t.Fatalf("first Lookup: want ErrTurnRequired, got %v", err)
	}
	turn, ok := b.Pending()
	if !ok {
		t.Fatal("expected a pending turn after a miss")
	}
	if turn.Operation != "spec.generate" || turn.Checkpoint != "spec" {
		t.Fatalf("turn metadata wrong: %+v", turn)
	}
	if turn.System != "SYS" || turn.User != "USR" || turn.MaxTokens != 8000 {
		t.Fatalf("turn did not carry the prompt verbatim: %+v", turn)
	}

	const answer = "# Spec\n\nGiven a user\nWhen they log in\nThen a session starts\n"
	if _, err := b.Fulfil(answer); err != nil {
		t.Fatalf("Fulfil: %v", err)
	}
	if _, ok := b.Pending(); ok {
		t.Fatal("pending turn should be cleared after Fulfil")
	}

	// A fresh process (fresh Bridge) must replay the recorded answer rather
	// than asking again — that resumability is what lets a chat window whose
	// context was compacted keep driving the same run.
	b2 := mustOpen(t, root, DefaultSession)
	got, err := b2.Lookup("spec.generate", "spec", "", "SYS", "USR", 8000)
	if err != nil {
		t.Fatalf("replay Lookup: %v", err)
	}
	if got != answer {
		t.Fatalf("replayed content differs:\n got %q\nwant %q", got, answer)
	}
	if _, ok := b2.Pending(); ok {
		t.Fatal("a replayed hit must not create a pending turn")
	}
}

// ── Cross-cutting: one run, one turn ──────────────────────────────────────────

func TestLookup_PauseLatches_OneTurnPerRun(t *testing.T) {
	t.Parallel()
	b := mustOpen(t, t.TempDir(), DefaultSession)

	if _, err := b.Lookup("spec.generate", "spec", "", "S1", "U1", 100); !errors.Is(err, ErrTurnRequired) {
		t.Fatalf("want ErrTurnRequired, got %v", err)
	}
	first, _ := b.Pending()

	// Later checkpoints in the same process must not overwrite the question
	// already put to the host agent — the driver answers one at a time.
	for _, op := range []string{"arch.generate", "breakdown.generate"} {
		if _, err := b.Lookup(op, "arch", "", "S2", "U2", 100); !errors.Is(err, ErrTurnRequired) {
			t.Fatalf("%s: want ErrTurnRequired, got %v", op, err)
		}
	}
	after, _ := b.Pending()
	if after.Operation != first.Operation || after.Hash != first.Hash {
		t.Fatalf("pending turn was overwritten: %q → %q", first.Operation, after.Operation)
	}
	if !b.Paused() {
		t.Fatal("bridge should report paused")
	}
}

// ── Regression: an unanswered turn is never replaced ──────────────────────────

func TestLookup_RestoredPendingTurnSurvivesADifferentQuestion(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	b := mustOpen(t, root, DefaultSession)
	if _, err := b.Lookup("spec.generate", "spec", "", "SYS", "USR", 100); !errors.Is(err, ErrTurnRequired) {
		t.Fatalf("want ErrTurnRequired, got %v", err)
	}
	original, _ := b.Pending()

	// The host agent has been shown "generate a spec" and has not answered.
	// A re-run can legitimately reach a *different* prompt — the failed
	// checkpoint may still have scaffolded a stub, putting the next run on
	// the review path. Overwriting pending here would misfile the answer the
	// agent is about to submit, so the original question must stand.
	b2 := mustOpen(t, root, DefaultSession)
	if _, err := b2.Lookup("spec.review", "spec", "", "OTHER-SYS", "OTHER-USR", 100); !errors.Is(err, ErrTurnRequired) {
		t.Fatalf("want ErrTurnRequired, got %v", err)
	}
	still, ok := b2.Pending()
	if !ok {
		t.Fatal("the unanswered turn disappeared")
	}
	if still.Operation != original.Operation || still.Hash != original.Hash {
		t.Fatalf("unanswered turn was replaced: %q → %q", original.Operation, still.Operation)
	}

	// Answering it files against the question actually shown, and clears the
	// latch so the next run can move on.
	if _, err := b2.Fulfil("# Spec\n"); err != nil {
		t.Fatalf("Fulfil: %v", err)
	}
	b3 := mustOpen(t, root, DefaultSession)
	got, err := b3.Lookup("spec.generate", "spec", "", "SYS", "USR", 100)
	if err != nil || got != "# Spec\n" {
		t.Fatalf("answer was filed against the wrong turn: got %q, err %v", got, err)
	}
}

func TestLookup_RestoredTurnStillReplaysEarlierAnswers(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	// Answer turn 1, then leave turn 2 unanswered.
	b := mustOpen(t, root, DefaultSession)
	_, _ = b.Lookup("spec.generate", "spec", "", "S1", "U1", 100)
	if _, err := b.Fulfil("# Spec\n"); err != nil {
		t.Fatalf("Fulfil: %v", err)
	}
	_, _ = b.Lookup("arch.generate", "arch", "", "S2", "U2", 100)

	// The next run must replay turn 1 rather than latching on it — otherwise
	// the pipeline could never reach the pending question again.
	b2 := mustOpen(t, root, DefaultSession)
	got, err := b2.Lookup("spec.generate", "spec", "", "S1", "U1", 100)
	if err != nil {
		t.Fatalf("a restored bridge must still replay answered turns: %v", err)
	}
	if got != "# Spec\n" {
		t.Fatalf("want the recorded answer, got %q", got)
	}
	if _, err := b2.Lookup("arch.generate", "arch", "", "S2", "U2", 100); !errors.Is(err, ErrTurnRequired) {
		t.Fatalf("the unanswered turn should still be owed, got %v", err)
	}
}

// ── Regression: the drift fallback stops the re-ask loop ──────────────────────

func TestLookup_OrdinalFallback_ReplaysWhenPromptDrifts(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	b := mustOpen(t, root, DefaultSession)

	_, _ = b.Lookup("spec.generate", "spec", "", "SYS-v1", "USR", 100)
	if _, err := b.Fulfil("# Spec v1\n"); err != nil {
		t.Fatalf("Fulfil: %v", err)
	}

	// The host agent's own artefact landed on disk, so the next run compiles a
	// prompt that embeds it and no longer hashes the same. Content addressing
	// alone would re-ask this forever; the ordinal key breaks the loop.
	b2 := mustOpen(t, root, DefaultSession)
	got, err := b2.Lookup("spec.generate", "spec", "", "SYS-v2-now-includes-the-file", "USR", 100)
	if err != nil {
		t.Fatalf("drifted Lookup should replay, got %v", err)
	}
	if got != "# Spec v1\n" {
		t.Fatalf("want the recorded answer, got %q", got)
	}
	if b2.Stats().Drifted != 1 {
		t.Fatalf("drift should be counted and surfaced, got %d", b2.Stats().Drifted)
	}
}

func TestLookup_StrictReplay_ReasksInsteadOfDrifting(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	b := mustOpen(t, root, DefaultSession)
	_, _ = b.Lookup("spec.generate", "spec", "", "SYS-v1", "USR", 100)
	if _, err := b.Fulfil("# Spec v1\n"); err != nil {
		t.Fatalf("Fulfil: %v", err)
	}

	b2 := mustOpen(t, root, DefaultSession)
	b2.StrictReplay = true
	if _, err := b2.Lookup("spec.generate", "spec", "", "SYS-v2", "USR", 100); !errors.Is(err, ErrTurnRequired) {
		t.Fatalf("strict replay must re-ask a changed prompt, got %v", err)
	}
}

// ── False-positive guard ──────────────────────────────────────────────────────

func TestLookup_DoesNotServeAnotherOperationsAnswer(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	b := mustOpen(t, root, DefaultSession)
	_, _ = b.Lookup("spec.generate", "spec", "", "SYS", "USR", 100)
	if _, err := b.Fulfil("# Spec\n"); err != nil {
		t.Fatalf("Fulfil: %v", err)
	}

	// Same prompt text, different operation: the ordinal key is namespaced by
	// operation, so arch must get its own turn rather than inheriting spec's.
	b2 := mustOpen(t, root, DefaultSession)
	if _, err := b2.Lookup("arch.generate", "arch", "", "SYS", "USR", 100); !errors.Is(err, ErrTurnRequired) {
		t.Fatalf("arch must ask its own question, got %v", err)
	}
}

// ── Negative + boundary ───────────────────────────────────────────────────────

func TestFulfil_NoPendingTurn(t *testing.T) {
	t.Parallel()
	b := mustOpen(t, t.TempDir(), DefaultSession)
	if _, err := b.Fulfil("anything"); !errors.Is(err, ErrNoPendingTurn) {
		t.Fatalf("want ErrNoPendingTurn, got %v", err)
	}
}

func TestFulfil_RejectsEmptyAnswer(t *testing.T) {
	t.Parallel()
	b := mustOpen(t, t.TempDir(), DefaultSession)
	_, _ = b.Lookup("spec.generate", "spec", "", "SYS", "USR", 100)
	if _, err := b.Fulfil("   \n\t "); err == nil {
		t.Fatal("a whitespace-only answer must be rejected, not recorded")
	}
	if _, ok := b.Pending(); !ok {
		t.Fatal("a rejected answer must leave the turn pending")
	}
}

func TestOpen_RejectsUnsafeSessionName(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for _, name := range []string{"../escape", "a/b", "with space", strings.Repeat("x", 129)} {
		if _, err := Open(root, name); err == nil {
			t.Fatalf("session name %q should be rejected", name)
		}
	}
}

func TestNilBridge_IsSafe(t *testing.T) {
	t.Parallel()
	var b *Bridge
	if b.Paused() {
		t.Fatal("nil bridge must not report paused")
	}
	if _, ok := b.Pending(); ok {
		t.Fatal("nil bridge must have no pending turn")
	}
	if _, err := b.Lookup("op", "cp", "", "s", "u", 1); !errors.Is(err, ErrTurnRequired) {
		t.Fatalf("nil bridge Lookup: want ErrTurnRequired, got %v", err)
	}
	if err := b.Reset(); err != nil {
		t.Fatalf("nil bridge Reset: %v", err)
	}
}

// ── Concurrency / isolation ───────────────────────────────────────────────────

func TestSessions_AreIsolated(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	a := mustOpen(t, root, "feature-a")
	_, _ = a.Lookup("spec.generate", "spec", "", "SYS", "USR", 100)
	if _, err := a.Fulfil("# A\n"); err != nil {
		t.Fatalf("Fulfil: %v", err)
	}

	// Identical prompt in a different session must still be asked: two
	// concurrent conversations must never answer each other's turns.
	bSess := mustOpen(t, root, "feature-b")
	if _, err := bSess.Lookup("spec.generate", "spec", "", "SYS", "USR", 100); !errors.Is(err, ErrTurnRequired) {
		t.Fatalf("session b must ask its own question, got %v", err)
	}

	names, err := ListSessions(root)
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(names) != 2 || names[0] != "feature-a" || names[1] != "feature-b" {
		t.Fatalf("want [feature-a feature-b], got %v", names)
	}
}

// ── Idempotency + reset ───────────────────────────────────────────────────────

func TestReset_DiscardsRecordedAnswers(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	b := mustOpen(t, root, DefaultSession)
	_, _ = b.Lookup("spec.generate", "spec", "", "SYS", "USR", 100)
	if _, err := b.Fulfil("# Spec\n"); err != nil {
		t.Fatalf("Fulfil: %v", err)
	}
	if err := b.Reset(); err != nil {
		t.Fatalf("Reset: %v", err)
	}

	b2 := mustOpen(t, root, DefaultSession)
	if _, err := b2.Lookup("spec.generate", "spec", "", "SYS", "USR", 100); !errors.Is(err, ErrTurnRequired) {
		t.Fatalf("after reset the run must start over, got %v", err)
	}
	for _, f := range []string{responsesFile, sessionFile, pendingFile} {
		if _, err := os.Stat(filepath.Join(root, DefaultDir, DefaultSession, f)); err == nil && f == responsesFile {
			t.Fatalf("%s should have been removed by Reset", f)
		}
	}
}

// ── Data accuracy ─────────────────────────────────────────────────────────────

func TestFulfil_RoundTripsContentExactly(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// Fenced blocks, trailing whitespace and CRLF all survive: a spec answer
	// is Markdown with embedded Gherkin/YAML, and JSONL storage must not
	// reinterpret any of it.
	const answer = "# Spec\r\n\n```yaml\nkey: \"value\"\n```\n\n- [ ] task  \n"
	b := mustOpen(t, root, DefaultSession)
	_, _ = b.Lookup("spec.generate", "spec", "", "SYS", "USR", 100)
	if _, err := b.Fulfil(answer); err != nil {
		t.Fatalf("Fulfil: %v", err)
	}
	b2 := mustOpen(t, root, DefaultSession)
	got, err := b2.Lookup("spec.generate", "spec", "", "SYS", "USR", 100)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if got != answer {
		t.Fatalf("content did not round-trip:\n got %q\nwant %q", got, answer)
	}
}

func TestHash_IsStableAndSeparated(t *testing.T) {
	t.Parallel()
	// Determinism across separate calls is what makes replay work at all:
	// the same question asked in a later process must address the same slot.
	first, second := Hash("op", "sys", "usr"), Hash("op", "sys", "usr")
	if first != second {
		t.Fatalf("hash must be deterministic: %q vs %q", first, second)
	}
	// NUL separation: "a"+"b" and "ab" must not collide across field
	// boundaries, or two different prompts could share one recorded answer.
	if Hash("op", "a", "b") == Hash("op", "ab", "") {
		t.Fatal("field boundaries must not collide")
	}
	if Hash("spec", "s", "u") == Hash("arch", "s", "u") {
		t.Fatal("operation must participate in the hash")
	}
}

// ── Rendering ─────────────────────────────────────────────────────────────────

func TestRenderTurn_CarriesEverythingNeededToAnswer(t *testing.T) {
	t.Parallel()
	turn := Turn{
		Operation: "spec.generate", Checkpoint: "spec", MaxTokens: 8000,
		Hash: "0123456789abcdef0123456789abcdef", Ordinal: "spec.generate#1",
		System: "SYSTEM-TEXT", User: "USER-TEXT",
	}
	out := RenderTurn(turn, DefaultSession)
	for _, want := range []string{
		"SYSTEM-TEXT", "USER-TEXT", "spec.generate", "8000",
		"forge agent submit", "forge ship --agent-mode",
		"You do NOT decide whether a checkpoint passed",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("rendered turn missing %q:\n%s", want, out)
		}
	}
	// The default session must not clutter the commands with a redundant flag.
	if strings.Contains(out, "--session "+DefaultSession) {
		t.Fatal("default session should not be spelled out in the commands")
	}
}

func TestRenderTurn_NamedSessionAppearsInEveryCommand(t *testing.T) {
	t.Parallel()
	out := RenderTurn(Turn{Operation: "op", Hash: "0123456789abcdef", Ordinal: "op#1"}, "feature-b")
	if !strings.Contains(out, "forge agent submit --file <path> --session feature-b") {
		t.Fatalf("submit command must carry the session:\n%s", out)
	}
	if !strings.Contains(out, "forge ship --agent-mode --session feature-b") {
		t.Fatalf("resume command must carry the session:\n%s", out)
	}
}

func TestFence_SurvivesFencedContentInsideThePrompt(t *testing.T) {
	t.Parallel()
	// Specs routinely contain ``` blocks. If the delimiter bracketing the
	// payload were an ordinary fence, the first embedded block would close it
	// and the host agent would read a truncated prompt.
	out := RenderTurn(Turn{
		Operation: "spec.generate", Hash: "0123456789abcdef", Ordinal: "spec#1",
		System: "s", User: "```go\nfunc main() {}\n```",
	}, DefaultSession)
	if strings.Count(out, Fence) != 4 {
		t.Fatalf("expected exactly 4 payload delimiters, got %d:\n%s",
			strings.Count(out, Fence), out)
	}
}

// TestSetFeature_SwitchingFeatureResetsStaleSession is a regression test for
// cross-feature state contamination: SetFeature recorded the new
// feature/slug but never checked whether the session already belonged to a
// different feature. The ordinal fallback in Lookup is keyed only on
// "operation#N" with no feature scoping, so replaying default-session state
// into a second, unrelated feature could silently serve the first feature's
// recorded answer for the second feature's Nth call to the same operation —
// e.g. its qa-verify tasks — instead of asking a fresh question.
func TestSetFeature_SwitchingFeatureResetsStaleSession(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	b := mustOpen(t, root, DefaultSession)
	if switched := b.SetFeature("agency master-calendar", "agency-master-calendar"); switched {
		t.Fatal("first SetFeature call on a fresh session must not report a switch")
	}
	if _, err := b.Lookup("ship:qa-verify:generate", "qa-verify", "", "sys", "usr", 100); !errors.Is(err, ErrTurnRequired) {
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
	b2 := mustOpen(t, root, DefaultSession)
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
	if !errors.Is(lookupErr, ErrTurnRequired) {
		t.Fatalf("expected a fresh pause for the new feature, got content=%q err=%v", content, lookupErr)
	}
}

// TestSetFeature_BareContinuationPreservesIdentity is a regression test for
// the FORGE_SHIP_ISSUES_2026-09-04.md ISSUE 4 root cause: a bare re-run of
// the hint forge itself prints after a submit — `forge ship --agent-mode`,
// with no --name and no description — called SetFeature("", "") on every
// continuation. The unconditional write at the end of SetFeature blanked the
// session's recorded Feature/Slug even though nothing about the call
// intended a switch, so the very next lookup of "what feature is this
// session driving" (the ship.go call site) found nothing to resume against.
func TestSetFeature_BareContinuationPreservesIdentity(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	b := mustOpen(t, root, DefaultSession)
	if switched := b.SetFeature("blog inbound hub", "blog-inbound-hub"); switched {
		t.Fatal("first SetFeature call on a fresh session must not report a switch")
	}

	// Simulate the bare continuation call forge ship makes when neither
	// --name nor a description was passed.
	if switched := b.SetFeature("", ""); switched {
		t.Fatal("a bare SetFeature(\"\", \"\") must never report a switch")
	}
	if feature, slug := b.Feature(); feature != "blog inbound hub" || slug != "blog-inbound-hub" {
		t.Fatalf("bare continuation must preserve the session's prior identity, got feature=%q slug=%q", feature, slug)
	}

	// A fresh process reopening the session must see the same preserved identity.
	b2 := mustOpen(t, root, DefaultSession)
	if feature, slug := b2.Feature(); feature != "blog inbound hub" || slug != "blog-inbound-hub" {
		t.Fatalf("reopened session must preserve identity across processes, got feature=%q slug=%q", feature, slug)
	}
}
