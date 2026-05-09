package codemod

// hygiene_test.go — tests for the "gitignore" and "gitleaks" codemods (DEV-M0-35).
//
// Test design (per §always-write-tests):
//  1. Happy path: fresh file → managed block inserted.
//  2. Idempotency: same apply twice → second is no-op.
//  3. Drift without --force → ErrManagedBlockDrift returned, file unchanged.
//  4. Drift with ApplyForce → managed block overwritten, user content preserved.
//  5. Dry-run: nothing written to disk.
//  6. False-positive guard: user content outside managed block is never flagged as drift.
//  7. gitleaks: existing legacy file (no markers) without force → ErrManagedBlockDrift.
//  8. gitleaks: user rules after end marker preserved after ApplyForce.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ── gitignore codemod ──────────────────────────────────────────────────────

// TC-35-01: happy path — fresh repo with no .gitignore.
func TestGitignoreCM_FreshInsert(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	c := gitignoreCM{}
	rep, err := c.Apply(dir, false)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if rep.Changed != 1 {
		t.Fatalf("expected Changed=1, got %d", rep.Changed)
	}
	body, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if !strings.Contains(string(body), giStart) || !strings.Contains(string(body), giEnd) {
		t.Fatalf("markers not found in result: %s", body)
	}
}

// TC-35-01b: existing file without markers — block appended, existing content preserved.
func TestGitignoreCM_AppendToExisting(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	existing := "node_modules/\ndist/\n"
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(existing), 0o600); err != nil {
		t.Fatal(err)
	}
	c := gitignoreCM{}
	if _, err := c.Apply(dir, false); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	body, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if !strings.Contains(string(body), "node_modules/") {
		t.Error("pre-existing content lost")
	}
	if !strings.Contains(string(body), giStart) {
		t.Error("managed block not inserted")
	}
}

// TC-35-02 idempotency — apply twice → second run is no-op.
func TestGitignoreCM_Idempotent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	c := gitignoreCM{}
	if _, err := c.Apply(dir, false); err != nil {
		t.Fatalf("first apply: %v", err)
	}
	rep, err := c.Apply(dir, false)
	if err != nil {
		t.Fatalf("second apply: %v", err)
	}
	if rep.Changed != 0 {
		t.Fatalf("second apply should be no-op, got Changed=%d", rep.Changed)
	}
	if rep.Detail != "no change" {
		t.Errorf("expected detail='no change', got %q", rep.Detail)
	}
}

// TC-35-03 drift without force → ErrManagedBlockDrift, file unchanged.
func TestGitignoreCM_DriftNoForce(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	drifted := giStart + "\n# HAND-EDITED INSIDE MANAGED BLOCK\n" + giEnd + "\n"
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(drifted), 0o600); err != nil {
		t.Fatal(err)
	}
	c := gitignoreCM{}
	_, err := c.Apply(dir, false)
	if err != ErrManagedBlockDrift {
		t.Fatalf("expected ErrManagedBlockDrift, got %v", err)
	}
	// File must be unchanged.
	body, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if string(body) != drifted {
		t.Error("file was modified despite drift guard")
	}
}

// TC-35-04 force — drifted managed block overwritten; user content outside preserved.
func TestGitignoreCM_ForceOverwritesDriftedBlock(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	userBefore := "# user rule before\n"
	driftedBlock := giStart + "\n# HAND-EDITED\n" + giEnd + "\n"
	userAfter := "# user rule after\n"
	content := userBefore + driftedBlock + userAfter
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	c := gitignoreCM{}
	rep, err := c.ApplyForce(dir, false)
	if err != nil {
		t.Fatalf("ApplyForce: %v", err)
	}
	if rep.Changed != 1 {
		t.Fatalf("expected Changed=1, got %d", rep.Changed)
	}
	body, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))
	s := string(body)
	if !strings.Contains(s, userBefore) {
		t.Error("user content before managed block was lost")
	}
	if !strings.Contains(s, userAfter) {
		t.Error("user content after managed block was lost")
	}
	if strings.Contains(s, "# HAND-EDITED") {
		t.Error("drifted content survived force overwrite")
	}
	if !strings.Contains(s, canonicalGitignoreBlock[:50]) {
		t.Error("canonical block not present after force")
	}
}

// TC-35-05 dry-run — nothing written to disk.
func TestGitignoreCM_DryRun(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	c := gitignoreCM{}
	rep, err := c.Apply(dir, true) // dryRun=true
	if err != nil {
		t.Fatalf("DryRun Apply: %v", err)
	}
	if rep.Changed != 1 {
		t.Fatalf("dry run should report Changed=1, got %d", rep.Changed)
	}
	if _, err := os.Stat(filepath.Join(dir, ".gitignore")); !os.IsNotExist(err) {
		t.Error("dry run must not create the file")
	}
}

// TC-35-06 false-positive guard: user section outside markers is never drift.
func TestGitignoreCM_UserSectionOutsideMarkersNotDrift(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// First apply — creates canonical block.
	c := gitignoreCM{}
	if _, err := c.Apply(dir, false); err != nil {
		t.Fatalf("apply: %v", err)
	}
	// User appends their own line after the managed block.
	f, err := os.OpenFile(filepath.Join(dir, ".gitignore"), os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteString("# my custom rule\n")
	_ = f.Close()
	// Second apply must succeed (no-op) — not flagged as drift.
	rep, err := c.Apply(dir, false)
	if err != nil {
		t.Fatalf("unexpected error after user append: %v", err)
	}
	if rep.Changed != 0 {
		t.Fatalf("user content outside markers should not trigger Changed, got %d", rep.Changed)
	}
}

// ── gitleaks codemod ───────────────────────────────────────────────────────

// TC-35-10: happy path — fresh gitleaks.toml.
func TestGitleaksCM_FreshInsert(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	c := gitleaksCM{}
	rep, err := c.Apply(dir, false)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if rep.Changed != 1 {
		t.Fatalf("expected Changed=1, got %d", rep.Changed)
	}
	body, _ := os.ReadFile(filepath.Join(dir, ".gitleaks.toml"))
	if !strings.Contains(string(body), glStart) || !strings.Contains(string(body), glEnd) {
		t.Fatalf("markers not found: %s", body)
	}
}

// TC-35-11: idempotency.
func TestGitleaksCM_Idempotent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	c := gitleaksCM{}
	if _, err := c.Apply(dir, false); err != nil {
		t.Fatalf("first: %v", err)
	}
	rep, err := c.Apply(dir, false)
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if rep.Changed != 0 {
		t.Fatalf("second run must be no-op, got Changed=%d", rep.Changed)
	}
}

// TC-35-12: legacy file (no markers) — ErrManagedBlockDrift without force.
func TestGitleaksCM_LegacyFileNoForce(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	legacy := "title = \"old\"\n[[rules]]\nid = \"custom\"\n"
	if err := os.WriteFile(filepath.Join(dir, ".gitleaks.toml"), []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}
	c := gitleaksCM{}
	_, err := c.Apply(dir, false)
	if err != ErrManagedBlockDrift {
		t.Fatalf("expected ErrManagedBlockDrift, got %v", err)
	}
}

// TC-35-13: user rules after end marker are preserved after ApplyForce.
func TestGitleaksCM_ForcePreservesUserRules(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// First fresh install.
	c := gitleaksCM{}
	if _, err := c.Apply(dir, false); err != nil {
		t.Fatalf("setup: %v", err)
	}
	// User appends custom rules after the managed block.
	f, err := os.OpenFile(filepath.Join(dir, ".gitleaks.toml"), os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteString("\n[[rules]]\nid = \"my-secret\"\nregex = \"MY_SECRET\"\n")
	_ = f.Close()
	// Drift: manually edit managed block.
	body, _ := os.ReadFile(filepath.Join(dir, ".gitleaks.toml"))
	drifted := strings.Replace(string(body), "Generic API key", "CHANGED_BY_USER", 1)
	if err := os.WriteFile(filepath.Join(dir, ".gitleaks.toml"), []byte(drifted), 0o600); err != nil {
		t.Fatal(err)
	}
	// Apply without force should fail.
	if _, err := c.Apply(dir, false); err != ErrManagedBlockDrift {
		t.Fatalf("expected ErrManagedBlockDrift without force, got %v", err)
	}
	// Apply with force should succeed and preserve user rules.
	if _, err := c.ApplyForce(dir, false); err != nil {
		t.Fatalf("ApplyForce: %v", err)
	}
	result, _ := os.ReadFile(filepath.Join(dir, ".gitleaks.toml"))
	if !strings.Contains(string(result), "MY_SECRET") {
		t.Error("user custom rule was lost after ApplyForce")
	}
	if !strings.Contains(string(result), "Generic API key") {
		t.Error("canonical block not restored after ApplyForce")
	}
}
