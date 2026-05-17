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
package cmdhygiene_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/teragrid/forge/internal/cli/cmdhygiene"
)

// ── G-062: forge-owner ownership tags ────────────────────────────────────────

func TestOwnerFor_DetectsGoComment(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	content := "// forge-owner: ship\npackage main\n"
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	owner, ok := cmdhygiene.OwnerFor(root, "main.go")
	if !ok {
		t.Fatal("expected to find forge-owner tag")
	}
	if owner != "ship" {
		t.Errorf("owner: got %q want %q", owner, "ship")
	}
}

func TestOwnerFor_DetectsHashComment(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	content := "# forge-owner: scan\nsome_setting = true\n"
	if err := os.WriteFile(filepath.Join(root, "config.toml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	owner, ok := cmdhygiene.OwnerFor(root, "config.toml")
	if !ok {
		t.Fatal("expected to find forge-owner tag")
	}
	if owner != "scan" {
		t.Errorf("owner: got %q want %q", owner, "scan")
	}
}

func TestOwnerFor_NilWhenMissing(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "plain.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, ok := cmdhygiene.OwnerFor(root, "plain.go")
	if ok {
		t.Error("expected ok=false for file without forge-owner tag")
	}
}

func TestStampOwnership_AddsHeader(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	original := "package main\n\nfunc main() {}\n"
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := cmdhygiene.StampOwnership(root, "main.go", "ship"); err != nil {
		t.Fatalf("StampOwnership: %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(root, "main.go"))
	content := string(data)
	if !strings.Contains(content, "forge-owner: ship") {
		t.Errorf("expected forge-owner stamp in file:\n%s", content)
	}
	if !strings.Contains(content, "package main") {
		t.Error("original content should be preserved after stamp")
	}
}

func TestStampOwnership_IdempotentSameOwner(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	content := "// forge-owner: ship\npackage main\n"
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := cmdhygiene.StampOwnership(root, "main.go", "ship"); err != nil {
		t.Fatalf("StampOwnership: %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(root, "main.go"))
	if strings.Count(string(data), "forge-owner:") != 1 {
		t.Error("StampOwnership must not duplicate the forge-owner tag")
	}
}

// ── G-066: Mandatory hygiene block ───────────────────────────────────────────

func TestValidateMandatoryBlock_MissingFile(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	missing, err := cmdhygiene.ValidateMandatoryBlock(root)
	if err != nil {
		t.Fatalf("ValidateMandatoryBlock: %v", err)
	}
	if len(missing) == 0 {
		t.Error("expected missing entries when .gitignore does not exist")
	}
	// .forge/llm-scratch/ must be in the required list.
	found := false
	for _, m := range missing {
		if strings.Contains(m, "llm-scratch") {
			found = true
		}
	}
	if !found {
		t.Error("expected .forge/llm-scratch/ in mandatory block list")
	}
}

func TestValidateMandatoryBlock_AllPresent(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	content := ".env\n.env.local\n.env.*.local\n" +
		".forge/cache/\n.forge/llm-scratch/\n.forge/trash/\n" +
		".forge/session/\n.forge/learned/\n.forge/scan-history/\n.forge/eval-runs/\n"
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	missing, err := cmdhygiene.ValidateMandatoryBlock(root)
	if err != nil {
		t.Fatalf("ValidateMandatoryBlock: %v", err)
	}
	if len(missing) != 0 {
		t.Errorf("expected 0 missing entries when all present, got %d: %v", len(missing), missing)
	}
}

func TestValidateMandatoryBlock_PartiallyPresent(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// Only .env is present; everything else missing.
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".env\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	missing, err := cmdhygiene.ValidateMandatoryBlock(root)
	if err != nil {
		t.Fatalf("ValidateMandatoryBlock: %v", err)
	}
	if len(missing) == 0 {
		t.Error("expected missing entries when only .env is present")
	}
}

// ── G-067: Negation discipline ────────────────────────────────────────────────

func TestValidateNegationDiscipline_CoveredByNegation(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// .gitignore negates *.example.
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("*.bak\n!*.example\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "env.example"), []byte("KEY=value"), 0o644); err != nil {
		t.Fatal(err)
	}
	uncovered, err := cmdhygiene.ValidateNegationDiscipline(root)
	if err != nil {
		t.Fatalf("ValidateNegationDiscipline: %v", err)
	}
	if len(uncovered) != 0 {
		t.Errorf("expected 0 uncovered files, got %d: %v", len(uncovered), uncovered)
	}
}

func TestValidateNegationDiscipline_UncoveredTemplate(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// No negation in .gitignore.
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("*.bak\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "config.template"), []byte("x=y"), 0o644); err != nil {
		t.Fatal(err)
	}
	uncovered, err := cmdhygiene.ValidateNegationDiscipline(root)
	if err != nil {
		t.Fatalf("ValidateNegationDiscipline: %v", err)
	}
	if len(uncovered) == 0 {
		t.Error("expected uncovered .template file")
	}
}

// ── G-068: .gitleaks.toml framework-managed ───────────────────────────────────

func TestEnsureGitleaksConfig_CreatesFile(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := cmdhygiene.EnsureGitleaksConfig(root); err != nil {
		t.Fatalf("EnsureGitleaksConfig: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, ".gitleaks.toml"))
	if err != nil {
		t.Fatalf("read .gitleaks.toml: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "forge-managed: true") {
		t.Error(".gitleaks.toml missing forge-managed marker")
	}
	for _, rule := range []string{"forge-openai-key", "forge-anthropic-key", "forge-stripe-live-key"} {
		if !strings.Contains(content, rule) {
			t.Errorf(".gitleaks.toml missing rule %q", rule)
		}
	}
}

func TestEnsureGitleaksConfig_DoesNotOverwrite(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	custom := "# custom gitleaks config\n"
	if err := os.WriteFile(filepath.Join(root, ".gitleaks.toml"), []byte(custom), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := cmdhygiene.EnsureGitleaksConfig(root); err != nil {
		t.Fatalf("EnsureGitleaksConfig: %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(root, ".gitleaks.toml"))
	if string(data) != custom {
		t.Error("EnsureGitleaksConfig must not overwrite existing .gitleaks.toml")
	}
}

// ── G-069: Allowlist expiry gate ─────────────────────────────────────────────

func TestCheckAllowlistExpiry_StaleEntry(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	past := time.Now().AddDate(-1, 0, 0).Format("2006-01-02")
	content := "[allowlist]\npaths = [\"secrets/old-key.txt\"] # review-by: " + past + "\n"
	if err := os.WriteFile(filepath.Join(root, ".gitleaks.toml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	expired, err := cmdhygiene.CheckAllowlistExpiry(root)
	if err != nil {
		t.Fatalf("CheckAllowlistExpiry: %v", err)
	}
	if len(expired) == 0 {
		t.Error("expected at least one expired allowlist entry")
	}
	if expired[0].DaysStale <= 0 {
		t.Errorf("expected DaysStale > 0, got %d", expired[0].DaysStale)
	}
}

func TestCheckAllowlistExpiry_FutureEntry(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	future := time.Now().AddDate(0, 6, 0).Format("2006-01-02")
	content := "[allowlist]\npaths = [\"ok/path\"] # review-by: " + future + "\n"
	if err := os.WriteFile(filepath.Join(root, ".gitleaks.toml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	expired, err := cmdhygiene.CheckAllowlistExpiry(root)
	if err != nil {
		t.Fatalf("CheckAllowlistExpiry: %v", err)
	}
	if len(expired) != 0 {
		t.Errorf("expected 0 expired entries for future review-by date, got %d", len(expired))
	}
}

func TestCheckAllowlistExpiry_NoFile(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	expired, err := cmdhygiene.CheckAllowlistExpiry(root)
	if err != nil {
		t.Fatalf("unexpected error when .gitleaks.toml absent: %v", err)
	}
	if len(expired) != 0 {
		t.Errorf("expected empty slice when no .gitleaks.toml, got %d entries", len(expired))
	}
}

// TestCheckAllowlistExpiry_MalformedDate verifies that a malformed review-by
// date is silently skipped (no error returned). G-069.
func TestCheckAllowlistExpiry_MalformedDate(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	content := "[allowlist]\npaths = [\"path/to/file\"] # review-by: not-a-valid-date\n"
	if err := os.WriteFile(filepath.Join(root, ".gitleaks.toml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	expired, err := cmdhygiene.CheckAllowlistExpiry(root)
	if err != nil {
		t.Fatalf("CheckAllowlistExpiry must not error on malformed review-by date: %v", err)
	}
	if len(expired) != 0 {
		t.Errorf("expected 0 entries for malformed date, got %d", len(expired))
	}
}

// TestValidateNegationDiscipline_NoGitignore verifies that a missing .gitignore
// returns nil, nil: no violations, no error. G-067.
func TestValidateNegationDiscipline_NoGitignore(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// No .gitignore present in the temp directory.
	uncovered, err := cmdhygiene.ValidateNegationDiscipline(root)
	if err != nil {
		t.Fatalf("ValidateNegationDiscipline must not error when .gitignore is absent: %v", err)
	}
	if len(uncovered) != 0 {
		t.Errorf("expected 0 uncovered files when .gitignore absent, got %v", uncovered)
	}
}
