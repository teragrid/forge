package cmdclean

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	mustWrite := func(rel, body string) {
		full := filepath.Join(root, rel)
		_ = os.MkdirAll(filepath.Dir(full), 0o755)
		if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	// Scratch candidates (default patterns).
	mustWrite(".forge/scratch/note.txt", "x")
	mustWrite("_scratch_temp", "x")
	mustWrite("draft.tmp.md", "x")
	// Legitimate (managed) files.
	mustWrite("README.md", "readme")
	mustWrite("LICENSE", "lic")
	mustWrite("src/main.go", "package main")
	return root
}

func TestRun_Check_FindsScratch(t *testing.T) {
	t.Parallel()
	root := setupTree(t)
	res, err := Run(root, false)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Mode != "check" {
		t.Fatalf("mode = %q", res.Mode)
	}
	expectIncludes(t, res.Candidates, ".forge/scratch", "_scratch_temp", "draft.tmp.md")
	expectExcludes(t, res.Candidates, "README.md", "LICENSE", "src/main.go")
}

func TestRun_Apply_DeletesAndIdempotent(t *testing.T) {
	t.Parallel()
	root := setupTree(t)
	res, err := Run(root, true)
	if err != nil {
		t.Fatalf("Run apply: %v", err)
	}
	if len(res.Deleted) == 0 {
		t.Fatal("expected deletions")
	}
	for _, d := range res.Deleted {
		if _, err := os.Stat(filepath.Join(root, d)); err == nil {
			t.Errorf("%s still exists after apply", d)
		}
	}
	// Idempotency: second run should find nothing.
	res2, err := Run(root, false)
	if err != nil {
		t.Fatalf("Run again: %v", err)
	}
	if len(res2.Candidates) != 0 {
		t.Fatalf("expected 0 candidates after apply, got %v", res2.Candidates)
	}
	// README must survive.
	if _, err := os.Stat(filepath.Join(root, "README.md")); err != nil {
		t.Fatalf("README.md must survive clean: %v", err)
	}
}

func TestRun_EmptyTree(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	res, err := Run(root, false)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(res.Candidates) != 0 {
		t.Fatalf("empty tree should have no candidates: %v", res.Candidates)
	}
}

func TestRun_SkipsDotGit(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	_ = os.MkdirAll(filepath.Join(root, ".git", "objects"), 0o755)
	_ = os.WriteFile(filepath.Join(root, ".git", "HEAD"), []byte("ref"), 0o600)
	_ = os.WriteFile(filepath.Join(root, "_scratch_x"), []byte("x"), 0o600)
	res, err := Run(root, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range res.Candidates {
		if strings.HasPrefix(c, ".git") || strings.HasPrefix(c, ".git/") {
			t.Fatalf(".git must not appear in candidates: %v", c)
		}
	}
}

// ── DEV-M0-36 secret guard tests ───────────────────────────────────────────

// TC-36-01: checkTrackedSecrets in a non-git tree → graceful skip (no panic, empty result).
func TestCheckTrackedSecrets_NonGitTree(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// No .git directory; git ls-files should fail → err returned, caller skips.
	secrets, err := checkTrackedSecrets(dir)
	if err == nil && len(secrets) > 0 {
		t.Errorf("expected no secrets in non-git tree, got %v", secrets)
	}
	// err != nil is expected and fine — caller suppresses it.
}

// TC-36-02: secretPatterns coverage — patterns that SHOULD match.
func TestSecretPatterns_Match(t *testing.T) {
	t.Parallel()
	shouldMatch := []string{
		".env", ".env.local", ".env.production.local",
		"server.pem", "private.key", "id_rsa", "id_ed25519", "id_ecdsa", "id_dsa",
		"secrets.json", "credentials.json", ".netrc",
	}
	for _, name := range shouldMatch {
		matched := false
		for _, pat := range secretPatterns {
			if ok, _ := filepath.Match(pat, name); ok {
				matched = true
				break
			}
		}
		if !matched {
			t.Errorf("expected %q to match a secret pattern but it did not", name)
		}
	}
}

// TC-36-03 false-positive guard: names that MUST NOT match.
func TestSecretPatterns_NoFalsePositive(t *testing.T) {
	t.Parallel()
	shouldNotMatch := []string{
		".env.local.example",
		".env.example",
		"environment.go",
		"key_handler.go",
		"secrets_test.go",
		"README.md",
		"main.go",
		".envrc",
	}
	for _, name := range shouldNotMatch {
		for _, pat := range secretPatterns {
			if ok, _ := filepath.Match(pat, name); ok {
				t.Errorf("false positive: %q matched secret pattern %q — should NOT match", name, pat)
			}
		}
	}
}

// TC-36-04: Run() with git unavailable does NOT return an error (graceful degradation).
func TestRun_SecretGuard_GracefulSkipWhenNoGit(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// Write .env inside the tree — no git repo.
	_ = os.WriteFile(filepath.Join(root, ".env"), []byte("SECRET=x"), 0o600)
	// Run should succeed (no git = graceful skip).
	res, err := Run(root, false)
	if err != nil {
		t.Fatalf("Run should not error when git is absent: %v", err)
	}
	// TrackedSecrets should be nil or empty since git is unavailable.
	if len(res.TrackedSecrets) > 0 {
		t.Errorf("expected no TrackedSecrets without git repo, got %v", res.TrackedSecrets)
	}
}

func expectIncludes(t *testing.T, got []string, wants ...string) {
	t.Helper()
	for _, w := range wants {
		found := false
		for _, g := range got {
			if g == w || strings.HasPrefix(g, w+"/") || strings.HasPrefix(g, w) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected candidate %q in %v", w, got)
		}
	}
}

func expectExcludes(t *testing.T, got []string, wants ...string) {
	t.Helper()
	for _, w := range wants {
		for _, g := range got {
			if g == w {
				t.Errorf("did NOT expect candidate %q (in %v)", w, got)
			}
		}
	}
}
