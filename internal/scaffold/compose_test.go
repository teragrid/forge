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

package scaffold_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/teragrid/forge/internal/scaffold"
)

// ── helpers ──────────────────────────────────────────────────────────────────

// makeModule writes a scaffold directory for a module under root.
func makeModule(t *testing.T, root, id string, files map[string]string) {
	t.Helper()
	dir := filepath.Join(root, id, "scaffold")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

// ── TEST-COMP-01: single module → files copied ───────────────────────────────

func TestCompose_SingleModule(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	makeModule(t, root, "simple-a", map[string]string{
		"main.go": "package main\n",
	})

	result, err := scaffold.Compose([]string{"simple-a"}, scaffold.CompositionOptions{ModulesRoot: root})
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}
	if _, ok := result.Files["main.go"]; !ok {
		t.Errorf("expected main.go in result, got keys: %v", fileKeys(result))
	}
}

// ── TEST-COMP-02: two modules, no conflict → all files included ───────────────

func TestCompose_TwoModulesNoConflict(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	makeModule(t, root, "mod-a", map[string]string{"a.go": "package a\n"})
	makeModule(t, root, "mod-b", map[string]string{"b.go": "package b\n"})

	result, err := scaffold.Compose(
		[]string{"mod-a", "mod-b"},
		scaffold.CompositionOptions{ModulesRoot: root},
	)
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}
	if _, ok := result.Files["a.go"]; !ok {
		t.Errorf("missing a.go in result")
	}
	if _, ok := result.Files["b.go"]; !ok {
		t.Errorf("missing b.go in result")
	}
}

// ── TEST-COMP-03: file conflict → FileConflictError ───────────────────────────

func TestCompose_FileConflict(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	makeModule(t, root, "conflict-a", map[string]string{"main.go": "package a\n"})
	makeModule(t, root, "conflict-b", map[string]string{"main.go": "package b\n"})

	_, err := scaffold.Compose(
		[]string{"conflict-a", "conflict-b"},
		scaffold.CompositionOptions{ModulesRoot: root},
	)
	if err == nil {
		t.Fatal("expected FileConflictError, got nil")
	}
	var fce *scaffold.FileConflictError
	if !isFileConflictError(err, &fce) {
		t.Errorf("want *scaffold.FileConflictError, got %T: %v", err, err)
	}
	if fce.Path != "main.go" {
		t.Errorf("conflict path = %q, want main.go", fce.Path)
	}
}

// ── TEST-COMP-04: .env.example → additive merge ───────────────────────────────

func TestCompose_AdditiveEnvExample(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	makeModule(t, root, "env-a", map[string]string{".env.example": "A=1\nB=2\n"})
	makeModule(t, root, "env-b", map[string]string{".env.example": "B=2\nC=3\n"})

	result, err := scaffold.Compose(
		[]string{"env-a", "env-b"},
		scaffold.CompositionOptions{ModulesRoot: root},
	)
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}
	content := string(result.Files[".env.example"])
	for _, line := range []string{"A=1", "B=2", "C=3"} {
		if !containsLine(content, line) {
			t.Errorf("missing line %q in merged .env.example:\n%s", line, content)
		}
	}
}

// ── TEST-COMP-05: .gitignore → additive merge ────────────────────────────────

func TestCompose_AdditiveGitignore(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	makeModule(t, root, "gi-a", map[string]string{".gitignore": "*.log\n"})
	makeModule(t, root, "gi-b", map[string]string{".gitignore": "node_modules/\n"})

	result, err := scaffold.Compose(
		[]string{"gi-a", "gi-b"},
		scaffold.CompositionOptions{ModulesRoot: root},
	)
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}
	content := string(result.Files[".gitignore"])
	if !containsLine(content, "*.log") {
		t.Errorf("missing *.log in merged .gitignore:\n%s", content)
	}
	if !containsLine(content, "node_modules/") {
		t.Errorf("missing node_modules/ in merged .gitignore:\n%s", content)
	}
}

// ── TEST-COMP-06: .env.example deduplicated ───────────────────────────────────

func TestCompose_DeduplEnvExample(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	makeModule(t, root, "dedup-a", map[string]string{".env.example": "A=1\nB=2\n"})
	makeModule(t, root, "dedup-b", map[string]string{".env.example": "B=2\nC=3\n"})

	result, err := scaffold.Compose(
		[]string{"dedup-a", "dedup-b"},
		scaffold.CompositionOptions{ModulesRoot: root},
	)
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}
	content := string(result.Files[".env.example"])
	// B=2 should appear exactly once.
	count := countOccurrences(content, "B=2")
	if count != 1 {
		t.Errorf("B=2 appears %d times in merged .env.example (want 1):\n%s", count, content)
	}
}

// ── TEST-COMP-07: module not found → ModuleNotFoundError ─────────────────────

func TestCompose_ModuleNotFound(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	_, err := scaffold.Compose(
		[]string{"nonexistent/module"},
		scaffold.CompositionOptions{ModulesRoot: root},
	)
	if err == nil {
		t.Fatal("expected ModuleNotFoundError, got nil")
	}
	var mnfe *scaffold.ModuleNotFoundError
	if !isModuleNotFoundError(err, &mnfe) {
		t.Errorf("want *scaffold.ModuleNotFoundError, got %T: %v", err, err)
	}
	if mnfe.ID != "nonexistent/module" {
		t.Errorf("module ID = %q, want nonexistent/module", mnfe.ID)
	}
}

// ── TEST-COMP-08: empty module list → empty result ────────────────────────────

func TestCompose_EmptyModuleList(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	result, err := scaffold.Compose([]string{}, scaffold.CompositionOptions{ModulesRoot: root})
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}
	if len(result.Files) != 0 {
		t.Errorf("expected empty result, got %d files", len(result.Files))
	}
}

// ── TEST-COMP-09: WriteComposed writes all files ──────────────────────────────

func TestWriteComposed_WritesFiles(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	makeModule(t, root, "write-test", map[string]string{
		"main.go": "package main\n",
		"go.mod":  "module example.com/test\n",
	})

	result, err := scaffold.Compose([]string{"write-test"}, scaffold.CompositionOptions{ModulesRoot: root})
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}

	outDir := t.TempDir()
	written, err := scaffold.WriteComposed(result, outDir)
	if err != nil {
		t.Fatalf("WriteComposed: %v", err)
	}
	if len(written) != 2 {
		t.Errorf("written %d files, want 2", len(written))
	}

	// Verify files exist on disk.
	if _, err := os.Stat(filepath.Join(outDir, "main.go")); err != nil {
		t.Errorf("main.go not found on disk: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outDir, "go.mod")); err != nil {
		t.Errorf("go.mod not found on disk: %v", err)
	}
}

// ── TEST-COMP-10 (integration): see compose_integration_test.go ──────────────

// ── helpers ───────────────────────────────────────────────────────────────────

func fileKeys(r *scaffold.ComposedResult) []string {
	var keys []string
	for k := range r.Files {
		keys = append(keys, k)
	}
	return keys
}

func containsLine(content, line string) bool {
	for _, l := range splitLines(content) {
		if l == line {
			return true
		}
	}
	return false
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func countOccurrences(s, substr string) int {
	count := 0
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			count++
		}
	}
	return count
}

func isFileConflictError(err error, target **scaffold.FileConflictError) bool {
	if err == nil {
		return false
	}
	fce, ok := err.(*scaffold.FileConflictError)
	if ok && target != nil {
		*target = fce
	}
	return ok
}

func isModuleNotFoundError(err error, target **scaffold.ModuleNotFoundError) bool {
	if err == nil {
		return false
	}
	mnfe, ok := err.(*scaffold.ModuleNotFoundError)
	if ok && target != nil {
		*target = mnfe
	}
	return ok
}
