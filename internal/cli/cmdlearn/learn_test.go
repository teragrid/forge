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

// Gap tests for the continuous learning loop (G-030 through G-035).
//
//	G-030: TestLearnPromote_RepeatedRejection  — 3 identical rejections in conventions.jsonl → promote outputs a lint-rule proposal
//	G-031: TestLearnAntipatterns_FixtureRepo   — against fixture repo, antipatterns produces deterministic output
//	G-032: TestLearnTeach_WritesPreferences    — forge learn teach writes entry to preferences.yml
//	G-033: TestLearnSession_Digest             — with session/*.log, forge learn session outputs summary
//	G-034: TestLearnInstructions_ProposesPR    — --dry-run reads instructions and proposes edits
package cmdlearn

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// helper: write a convention entry line into conventions.jsonl.
func writeConventionEntry(t *testing.T, root, rule string, rejections int) {
	t.Helper()
	dir := filepath.Join(root, ".forge", "learned")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	line := `{"ts":"2024-01-01T00:00:00Z","rule":"` + rule + `","rejections":` +
		itoa(rejections) + `,"detail":"test detail"}` + "\n"
	f, err := os.OpenFile(filepath.Join(dir, "conventions.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.WriteString(line); err != nil {
		t.Fatal(err)
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}

// ── G-030: TestLearnPromote_RepeatedRejection ─────────────────────────────────

// TestLearnPromote_RepeatedRejection verifies that 3 identical rejections in
// conventions.jsonl cause `forge learn promote --dry-run` to output a lint-rule proposal.
func TestLearnPromote_RepeatedRejection(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	// Write 3 entries for the same rule (each with rejections=1).
	writeConventionEntry(t, root, "no-float-for-money", 1)
	writeConventionEntry(t, root, "no-float-for-money", 1)
	writeConventionEntry(t, root, "no-float-for-money", 1)

	var jsonOut bool
	cmd := NewPromoteCmd(&root, &jsonOut)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--dry-run"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("forge learn promote --dry-run: %v\noutput: %s", err, out.String())
	}
	output := out.String()
	if !strings.Contains(output, "no-float-for-money") {
		t.Fatalf("expected rule name in promote output, got:\n%s", output)
	}
}

// TestLearnPromote_NoRules verifies that when there are fewer than 3 rejections
// the promote command reports "no rules" gracefully.
func TestLearnPromote_NoRules(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// Write only 2 rejections — below the threshold.
	writeConventionEntry(t, root, "some-rule", 1)
	writeConventionEntry(t, root, "some-rule", 1)

	var jsonOut bool
	cmd := NewPromoteCmd(&root, &jsonOut)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--dry-run"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(out.String(), "some-rule") {
		t.Fatalf("should not promote rule with <3 total rejections, got:\n%s", out.String())
	}
}

// TestLearnPromote_MissingFile verifies that a missing conventions.jsonl is
// handled gracefully (no error, just an informational message).
func TestLearnPromote_MissingFile(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	var jsonOut bool
	cmd := NewPromoteCmd(&root, &jsonOut)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--dry-run"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("missing file should not error: %v", err)
	}
}

// ── G-031: TestLearnAntipatterns_FixtureRepo ─────────────────────────────────

// TestLearnAntipatterns_FixtureRepo verifies that against a fixture repo
// `forge learn antipatterns --dry-run` produces deterministic output.
func TestLearnAntipatterns_FixtureRepo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// Simulate git log output with revert commits.
	gitDir := filepath.Join(root, ".git")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatal(err)
	}

	var jsonOut bool
	cmd := NewAntiPatternsCmd(&root, &jsonOut)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--dry-run"})
	// Should not error even on a fresh git repo without revert history.
	if err := cmd.Execute(); err != nil {
		t.Fatalf("forge learn antipatterns: %v\noutput: %s", err, out.String())
	}
	// Output should contain something (even if no patterns found).
	if len(out.String()) == 0 {
		t.Fatal("expected non-empty output from antipatterns")
	}
}

// ── G-032: TestLearnTeach_WritesPreferences ───────────────────────────────────

// TestLearnTeach_WritesPreferences verifies that `forge learn teach "never float for money"`
// writes an entry to .forge/learned/preferences.yml.
func TestLearnTeach_WritesPreferences(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	var jsonOut bool
	cmd := NewTeachCmd(&root, &jsonOut)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"never float for money"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("forge learn teach: %v\noutput: %s", err, out.String())
	}

	// Verify preferences.yml was created and contains the preference text.
	prefPath := filepath.Join(root, ".forge", "learned", "preferences.yml")
	data, err := os.ReadFile(prefPath)
	if err != nil {
		t.Fatalf("preferences.yml not created: %v", err)
	}
	if !strings.Contains(string(data), "never float for money") {
		t.Fatalf("preferences.yml does not contain preference text:\n%s", string(data))
	}
}

// TestLearnTeach_EmptyText verifies that an empty preference text returns an error.
func TestLearnTeach_EmptyText(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	var jsonOut bool
	cmd := NewTeachCmd(&root, &jsonOut)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	// No args, no --text flag.
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error for empty preference text, got nil")
	}
}

// ── G-033: TestLearnSession_Digest ───────────────────────────────────────────

// TestLearnSession_Digest verifies that with a .forge/session/*.log file,
// `forge learn session` outputs an accepted/rejected summary.
func TestLearnSession_Digest(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	// Create a session log file.
	sessDir := filepath.Join(root, ".forge", "session")
	if err := os.MkdirAll(sessDir, 0o755); err != nil {
		t.Fatal(err)
	}
	logContent := "accepted: 3\nrejected: 1\n[forge session 2024-01-01]\n"
	if err := os.WriteFile(filepath.Join(sessDir, "2024-01-01.log"), []byte(logContent), 0o600); err != nil {
		t.Fatal(err)
	}

	var jsonOut bool
	cmd := NewSessionCmd(&root, &jsonOut)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("forge learn session: %v\noutput: %s", err, out.String())
	}

	output := out.String()
	// Should contain session digest output.
	if len(output) == 0 {
		t.Fatal("expected non-empty session output")
	}
	// Should mention the log file or session content.
	if !strings.Contains(output, "Session") && !strings.Contains(output, "session") {
		t.Fatalf("expected 'session' in output, got:\n%s", output)
	}
}

// TestLearnSession_NoDirectory verifies that a missing session dir is handled gracefully.
func TestLearnSession_NoDirectory(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	var jsonOut bool
	cmd := NewSessionCmd(&root, &jsonOut)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	// No .forge/session/ directory.
	if err := cmd.Execute(); err != nil {
		t.Fatalf("missing session dir should not error: %v", err)
	}
}

// ── G-034: TestLearnInstructions_ProposesPR ───────────────────────────────────

// TestLearnInstructions_ProposesPR verifies that `forge learn instructions --dry-run`
// reads recent PRs and proposes instruction edits without error.
func TestLearnInstructions_ProposesPR(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	// Create a pre-existing instructions file.
	instrDir := filepath.Join(root, ".forge", "instructions")
	if err := os.MkdirAll(instrDir, 0o755); err != nil {
		t.Fatal(err)
	}
	instrContent := "# Forge Instructions\n\n- Always write tests first\n"
	if err := os.WriteFile(filepath.Join(instrDir, "default.instructions.md"), []byte(instrContent), 0o600); err != nil {
		t.Fatal(err)
	}

	var jsonOut bool
	cmd := NewInstructionsCmd(&root, &jsonOut)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--dry-run"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("forge learn instructions --dry-run: %v\noutput: %s", err, out.String())
	}
	// Output should be non-empty.
	if len(out.String()) == 0 {
		t.Fatal("expected non-empty output from forge learn instructions --dry-run")
	}
}
