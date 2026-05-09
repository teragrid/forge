package manifest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Test design (per always-write-tests.md):
//   Happy: Load valid file -> sections populated.
//   Negative: parse error on unknown section + on pattern outside section.
//   Boundary: missing file -> Default() with annotated path, no error.
//   Idempotency: parse twice returns equal File.
//   Data-accuracy: matchGlob handles `**`, `*`, `?` correctly.
//   False-positive guard: managed pattern wins over scratch.
//   Regression: README.md never proposed for deletion.

func TestLoad_Happy(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "manifest")
	body := `# comment
[scratch]
**/scratch/**
*.tmp

[managed]
README.md
`
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	f, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(f.Scratch) != 2 || len(f.Managed) != 1 {
		t.Fatalf("unexpected sections: %+v", f)
	}
}

func TestLoad_Missing_ReturnsDefaults(t *testing.T) {
	t.Parallel()
	f, err := Load(filepath.Join(t.TempDir(), "absent"))
	if err != nil {
		t.Fatalf("missing file should not error: %v", err)
	}
	if len(f.Scratch) == 0 {
		t.Fatal("default scratch patterns missing")
	}
	if !strings.Contains(f.Path, "defaults") {
		t.Fatalf("default path should be annotated: %q", f.Path)
	}
}

func TestLoad_UnknownSection(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "manifest")
	_ = os.WriteFile(p, []byte("[bogus]\nfoo\n"), 0o600)
	if _, err := Load(p); err == nil {
		t.Fatal("expected error for unknown section")
	}
}

func TestLoad_PatternOutsideSection(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "manifest")
	_ = os.WriteFile(p, []byte("orphan-pattern\n"), 0o600)
	if _, err := Load(p); err == nil {
		t.Fatal("expected error for orphan pattern")
	}
}

func TestMatchGlob(t *testing.T) {
	t.Parallel()
	cases := []struct {
		pat, name string
		want      bool
	}{
		{"**/scratch/**", "a/b/scratch/c.txt", true},
		{"**/scratch/**", "a/scratch/", true},
		{"**/scratch/**", "scratch/c.txt", true},
		{"**/scratch/**", "src/main.go", false},
		{"*.tmp", "x.tmp", true},
		{"*.tmp", "a/x.tmp", false},
		{"**/*.tmp", "a/b/x.tmp", true},
		{"**", "anything/at/all", true},
		{"a/?/c", "a/b/c", true},
		{"a/?/c", "a/bb/c", false},
		{"README.md", "README.md", true},
	}
	for _, tc := range cases {
		if got := matchGlob(tc.pat, tc.name); got != tc.want {
			t.Errorf("matchGlob(%q,%q)=%v, want %v", tc.pat, tc.name, got, tc.want)
		}
	}
}

func TestIsScratch_ManagedWins(t *testing.T) {
	t.Parallel()
	f := File{
		Scratch: []string{"**/*.md"},
		Managed: []string{"README.md"},
	}
	if f.IsScratch("README.md") {
		t.Fatal("managed pattern must override scratch")
	}
	if !f.IsScratch("docs/notes.md") {
		t.Fatal("non-managed match should be scratch")
	}
}

func TestDefault_ReadmeIsManaged(t *testing.T) {
	t.Parallel()
	d := Default()
	if d.IsScratch("README.md") {
		t.Fatal("README.md must never be a deletion candidate (regression guard)")
	}
}

func TestParse_Idempotent(t *testing.T) {
	t.Parallel()
	body := strings.NewReader("[scratch]\n**/.forge/scratch/**\n[managed]\nLICENSE\n")
	a, err := parse("x", body)
	if err != nil {
		t.Fatal(err)
	}
	b, err := parse("x", strings.NewReader("[scratch]\n**/.forge/scratch/**\n[managed]\nLICENSE\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Scratch) != len(b.Scratch) || a.Scratch[0] != b.Scratch[0] {
		t.Fatalf("not idempotent: %+v vs %+v", a, b)
	}
}
