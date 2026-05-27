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

package cmdship

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCollectWorkspaceContext_HappyPath verifies that a project root with a
// go.mod, Makefile, AGENTS.md, and an existing spec produces a correct snapshot.
func TestCollectWorkspaceContext_HappyPath(t *testing.T) {
	root := t.TempDir()
	slug := "my-feature"

	// Set up a minimal Go project.
	writeFile(t, root, "go.mod", "module github.com/example/app\n\ngo 1.24\n")
	writeFile(t, root, "Makefile", "build:\n\tgo build ./...\n")
	writeFile(t, root, "AGENTS.md", "# AGENTS\nDo not use CGO. Always write tests.\n")
	if err := os.MkdirAll(filepath.Join(root, "internal"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".forge", "specs", "existing-feature"), 0o755); err != nil {
		t.Fatal(err)
	}

	res := collectWorkspaceContext(root, slug)

	// Content must be non-empty.
	if res.Content == "" {
		t.Fatal("Content is empty")
	}

	// Tech stack should include both Go module and Make.
	if !strings.Contains(res.Content, "Go module") {
		t.Error("expected Go module in tech stack")
	}
	if !strings.Contains(res.Content, "Make") {
		t.Error("expected Make in tech stack")
	}

	// Module path and Go version should appear in the Go module label.
	if !strings.Contains(res.Content, "github.com/example/app") {
		t.Error("expected module path in tech stack label")
	}
	if !strings.Contains(res.Content, "go 1.24") {
		t.Error("expected Go version in tech stack label")
	}

	// Project structure: internal/ dir should appear.
	if !strings.Contains(res.Content, "internal") {
		t.Error("expected 'internal' in project structure")
	}
	// Hidden dirs (.forge) must NOT appear in structure section.
	if strings.Contains(res.Content, "## Project Structure\n- .forge") {
		t.Error("hidden .forge dir must not appear in project structure")
	}

	// Existing specs section should mention the pre-created feature.
	if !strings.Contains(res.Content, "existing-feature") {
		t.Error("expected existing-feature in existing specs")
	}

	// Conventions from AGENTS.md must appear.
	if !strings.Contains(res.Content, "Do not use CGO") {
		t.Error("expected AGENTS.md convention text")
	}

	// Snapshot file must have been written.
	if res.SnapshotPath == "" {
		t.Fatal("SnapshotPath is empty — file not written")
	}
	data, err := os.ReadFile(res.SnapshotPath)
	if err != nil {
		t.Fatalf("reading snapshot: %v", err)
	}
	if string(data) != res.Content {
		t.Error("on-disk snapshot does not match returned Content")
	}
}

// TestCollectWorkspaceContext_EmptyRoot verifies that an empty directory returns
// a non-empty snapshot (just a header) and does not panic.
func TestCollectWorkspaceContext_EmptyRoot(t *testing.T) {
	root := t.TempDir()
	res := collectWorkspaceContext(root, "my-feature")
	if res.Content == "" {
		t.Fatal("Content is empty for empty root")
	}
	// No tech stack detected.
	if len(res.TechStack) != 0 {
		t.Errorf("expected no tech stack, got %v", res.TechStack)
	}
	// HasGit should be false (no git repo).
	if res.HasGit {
		t.Error("expected HasGit=false for non-git directory")
	}
}

// TestCollectWorkspaceContext_ExistingSlugExcluded verifies that the slug being
// planned is NOT listed in the existing specs section (it doesn't exist yet).
func TestCollectWorkspaceContext_ExistingSlugExcluded(t *testing.T) {
	root := t.TempDir()
	slug := "new-feature"
	// Create a different existing spec.
	if err := os.MkdirAll(filepath.Join(root, ".forge", "specs", "old-feature"), 0o755); err != nil {
		t.Fatal(err)
	}

	res := collectWorkspaceContext(root, slug)

	// old-feature must appear.
	if !strings.Contains(res.Content, "old-feature") {
		t.Error("expected old-feature in existing specs")
	}
	// new-feature must NOT appear (it's the one being planned, does not exist yet).
	// We verify by checking the Existing Feature Specs section specifically.
	if strings.Contains(res.Content, "- new-feature") {
		t.Error("the target slug must not appear in existing specs section before it is created")
	}
}

// TestCollectWorkspaceContext_ConventionTruncation verifies that a very large
// AGENTS.md is truncated to ~500 chars and does not exceed a safe bound.
func TestCollectWorkspaceContext_ConventionTruncation(t *testing.T) {
	root := t.TempDir()
	// Write a large AGENTS.md (5 KB).
	largeContent := strings.Repeat("Do not import C. Write table-driven tests. ", 120)
	writeFile(t, root, "AGENTS.md", largeContent)

	res := collectWorkspaceContext(root, "feat")

	if !strings.Contains(res.Content, "AGENTS.md") {
		t.Error("expected AGENTS.md label in conventions section")
	}
	// The convention summary must be capped (500 chars + label overhead ≈ 600 max).
	convStart := strings.Index(res.Content, "## Project Conventions")
	if convStart < 0 {
		t.Fatal("Project Conventions section missing")
	}
	convSection := res.Content[convStart:]
	if len(convSection) > 800 {
		t.Errorf("conventions section too long (%d chars); should be truncated", len(convSection))
	}
	if !strings.Contains(convSection, "[truncated]") {
		t.Error("expected [truncated] marker for large AGENTS.md")
	}
}

// TestCollectWorkspaceContext_SnapshotWriteFailure verifies that when the
// snapshot cannot be written the function still returns non-empty Content
// and only SnapshotPath is empty — it does not panic.
// The collision is created by placing a regular file where the spec slug
// directory would be, which prevents MkdirAll from succeeding on all OSes.
func TestCollectWorkspaceContext_SnapshotWriteFailure(t *testing.T) {
	root := t.TempDir()
	slug := "feat"
	// Place a regular FILE at .forge/specs/feat so MkdirAll cannot create
	// .forge/specs/feat/ (works cross-platform including Windows).
	specsDir := filepath.Join(root, ".forge", "specs")
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Write a file with the slug name — this blocks directory creation.
	if err := os.WriteFile(filepath.Join(specsDir, slug), []byte("collision"), 0o600); err != nil {
		t.Fatal(err)
	}

	res := collectWorkspaceContext(root, slug)

	if res.Content == "" {
		t.Error("Content must be populated even when write fails")
	}
	if res.SnapshotPath != "" {
		t.Error("SnapshotPath must be empty when write fails")
	}
}

// TestDetectTechStack verifies individual indicators.
func TestDetectTechStack(t *testing.T) {
	root := t.TempDir()

	// No indicators yet.
	if stack := detectTechStack(root); len(stack) != 0 {
		t.Errorf("expected empty stack, got %v", stack)
	}

	// Add go.mod.
	writeFile(t, root, "go.mod", "module m\n\ngo 1.24\n")
	stack := detectTechStack(root)
	found := false
	for _, s := range stack {
		if s == "Go module" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'Go module' in stack %v", stack)
	}

	// Add Dockerfile.
	writeFile(t, root, "Dockerfile", "FROM scratch\n")
	stack = detectTechStack(root)
	foundDocker := false
	for _, s := range stack {
		if s == "Docker" {
			foundDocker = true
		}
	}
	if !foundDocker {
		t.Errorf("expected 'Docker' in stack %v", stack)
	}
}

// TestReadGoModSummary verifies module path + go version extraction.
func TestReadGoModSummary(t *testing.T) {
	root := t.TempDir()

	// Missing file → empty.
	if got := readGoModSummary(root); got != "" {
		t.Errorf("expected empty, got %q", got)
	}

	// Valid go.mod.
	writeFile(t, root, "go.mod", "module github.com/acme/svc\n\ngo 1.22.1\n\nrequire foo v1.0.0\n")
	got := readGoModSummary(root)
	if !strings.Contains(got, "github.com/acme/svc") {
		t.Errorf("expected module path in %q", got)
	}
	if !strings.Contains(got, "1.22.1") {
		t.Errorf("expected Go version in %q", got)
	}

	// go.mod without go directive.
	writeFile(t, root, "go.mod", "module github.com/acme/other\n")
	got = readGoModSummary(root)
	if got != "github.com/acme/other" {
		t.Errorf("expected module-only result, got %q", got)
	}
}

// ── helpers ────────────────────────────────────────────────────────────────────

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", name, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}
