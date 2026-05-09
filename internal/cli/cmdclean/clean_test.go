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
