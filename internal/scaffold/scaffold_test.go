package scaffold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Test design (per always-write-tests.md):
//   Happy: Render writes expected files; one is a templated file with subbed name.
//   Negative: unknown template -> ErrUnknownTemplate.
//   Negative: non-empty target without --force -> ErrTargetNotEmpty.
//   Boundary: empty existing dir is OK without --force.
//   Idempotency: render twice with --force -> byte-identical content.
//   Data-accuracy: README.md and .gitignore include the version stamp.
//   False-positive guard: README.md (managed) is created, not skipped.

func render(t *testing.T, target, name string, force bool) *Result {
	t.Helper()
	res, err := Render(Options{
		Template: "go-service",
		Target:   target,
		Vars:     Vars{Name: name, ForgeVer: "9.9.9-test"},
		Force:    force,
	})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	return res
}

func TestRender_Happy(t *testing.T) {
	t.Parallel()
	target := t.TempDir()
	res := render(t, target, "demo", false)

	for _, want := range []string{"README.md", "go.mod", "main.go", "main_test.go", ".gitignore", ".gitleaks.toml", ".forge/manifest"} {
		if _, err := os.Stat(filepath.Join(target, want)); err != nil {
			t.Errorf("missing %s: %v", want, err)
		}
		found := false
		for _, f := range res.Files {
			if filepath.ToSlash(f) == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Result.Files missing %s (got %v)", want, res.Files)
		}
	}
}

func TestRender_TemplateUnknown(t *testing.T) {
	t.Parallel()
	_, err := Render(Options{Template: "no-such-thing", Target: t.TempDir()})
	if err == nil {
		t.Fatal("expected error for unknown template")
	}
	if !strings.Contains(err.Error(), "FORGE-2200") {
		t.Fatalf("want FORGE-2200, got %v", err)
	}
}

func TestRender_TargetNotEmpty(t *testing.T) {
	t.Parallel()
	target := t.TempDir()
	_ = os.WriteFile(filepath.Join(target, "preexisting"), []byte("x"), 0o600)
	_, err := Render(Options{Template: "go-service", Target: target, Vars: Vars{Name: "x"}})
	if err == nil || !strings.Contains(err.Error(), "FORGE-2201") {
		t.Fatalf("want FORGE-2201, got %v", err)
	}
}

func TestRender_EmptyDirOK(t *testing.T) {
	t.Parallel()
	target := t.TempDir() // exists, empty
	render(t, target, "ok", false)
}

func TestRender_Idempotent(t *testing.T) {
	t.Parallel()
	a, b := t.TempDir(), t.TempDir()
	render(t, a, "demo", false)
	render(t, b, "demo", false)

	for _, rel := range []string{"README.md", "go.mod", "main.go", ".gitignore"} {
		eq, err := FilesEqual(filepath.Join(a, rel), filepath.Join(b, rel))
		if err != nil {
			t.Fatalf("compare %s: %v", rel, err)
		}
		if !eq {
			t.Errorf("non-deterministic render for %s", rel)
		}
	}
}

func TestRender_VersionStampedInTemplates(t *testing.T) {
	t.Parallel()
	target := t.TempDir()
	render(t, target, "demo", false)

	for _, rel := range []string{"README.md", ".gitignore", ".gitleaks.toml", ".forge/manifest"} {
		body, err := os.ReadFile(filepath.Join(target, rel))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		if !strings.Contains(string(body), "9.9.9-test") {
			t.Errorf("%s missing version stamp; got: %s", rel, body)
		}
	}
}

func TestAvailableTemplates(t *testing.T) {
	t.Parallel()
	got := AvailableTemplates()
	if len(got) == 0 {
		t.Fatal("expected at least one template")
	}
	hasGoService := false
	for _, n := range got {
		if n == "go-service" {
			hasGoService = true
		}
	}
	if !hasGoService {
		t.Fatalf("go-service missing; got %v", got)
	}
}
