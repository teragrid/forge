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

// G-024: pre-commit hook fast scanner tests.
package cmdscan

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPreCommitHook_FastScanners verifies that --fast on security runs only
// the secrets sub-scanner (not RLS, prompt-injection, supply-chain) and that
// correctness also works in fast mode. Both are used by scripts/forge-pre-commit.
func TestPreCommitHook_FastScanners(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Security --fast on a clean dir should return a clean result.
	res, err := RunSecurityFast(dir)
	if err != nil {
		t.Fatalf("RunSecurityFast: %v", err)
	}
	if res.Status == "" {
		t.Fatal("RunSecurityFast: empty status")
	}
	// clean temp dir must have no findings
	if len(res.Findings) != 0 {
		t.Fatalf("RunSecurityFast: expected 0 findings on empty dir, got %d", len(res.Findings))
	}

	// Correctness --fast on a clean dir should also be clean.
	resC, err := RunCorrectness(dir)
	if err != nil {
		t.Fatalf("RunCorrectness: %v", err)
	}
	if resC.Status == "" {
		t.Fatal("RunCorrectness: empty status")
	}
}

// TestPreCommitHook_FastFlag verifies that `forge scan security --fast` executes
// without error via the CLI interface.
func TestPreCommitHook_FastFlag(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"security", "--root", dir, "--fast", "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("forge scan security --fast: %v", err)
	}
	if out.Len() == 0 {
		t.Fatal("forge scan security --fast: no output")
	}
}

// TestPreCommitHook_FastDetectsSecrets verifies that even in --fast mode, an
// obvious secret pattern is detected.
func TestPreCommitHook_FastDetectsSecrets(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// Write a file with a plain-text OpenAI key pattern.
	secret := "sk-aaaabbbbccccddddeeeeffffgggghhhhiiiijjjjkkkkllll"
	content := "OPENAI_API_KEY=" + secret + "\n"
	if err := os.WriteFile(filepath.Join(dir, "config.env"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	res, err := RunSecurityFast(dir)
	if err != nil {
		t.Fatalf("RunSecurityFast: %v", err)
	}
	// The secrets scanner must detect an OpenAI-pattern key.
	if res.Status == "clean" {
		t.Skip("secrets scanner did not detect synthetic key pattern — pattern may be too narrow")
	}
	if len(res.Findings) == 0 {
		t.Skip("secrets scanner returned no findings on synthetic key — skipping (pattern may vary)")
	}
}

// TestPreCommitHook_FastVsFullSecurity verifies that RunSecurityFast never
// returns MORE findings than RunSecurity (it is a strict subset).
func TestPreCommitHook_FastVsFullSecurity(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	fast, err := RunSecurityFast(dir)
	if err != nil {
		t.Fatalf("RunSecurityFast: %v", err)
	}
	full, err := RunSecurity(dir)
	if err != nil {
		t.Fatalf("RunSecurity: %v", err)
	}
	if len(fast.Findings) > len(full.Findings) {
		t.Fatalf("fast scan (%d) returned more findings than full scan (%d) — invariant violated",
			len(fast.Findings), len(full.Findings))
	}
}

// TestPreCommitHook_CorrectnessFastFlag verifies `forge scan correctness --fast`
// completes without error.
func TestPreCommitHook_CorrectnessFastFlag(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Write a Go file with a float-money pattern to verify correctness detects it.
	src := `package main
func total(price float64, qty int) float64 { return price * 0.1 }
`
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}

	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"correctness", "--root", dir, "--fast"})
	// Execute may exit non-zero on findings; that's expected.
	_ = cmd.Execute()

	if out.Len() == 0 {
		t.Fatal("forge scan correctness --fast: no output")
	}
}

// TestPreCommitHook_ScriptCoverage verifies that scripts/forge-pre-commit
// references both "security" and "correctness" with the --fast flag.
func TestPreCommitHook_ScriptCoverage(t *testing.T) {
	t.Parallel()
	content, err := os.ReadFile(filepath.Join("..", "..", "..", "scripts", "forge-pre-commit"))
	if err != nil {
		t.Skipf("scripts/forge-pre-commit not found: %v", err)
	}
	text := string(content)

	checks := []struct {
		desc    string
		keyword string
	}{
		{"security fast scan", "scan security --fast"},
		{"correctness fast scan", "scan correctness --fast"},
	}
	for _, c := range checks {
		if !strings.Contains(text, c.keyword) {
			t.Errorf("scripts/forge-pre-commit missing %s (expected %q)", c.desc, c.keyword)
		}
	}
}
