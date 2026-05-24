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

// ── TEST-COMP-08b: SkipMissing=true → no error, missing IDs collected ─────────

func TestCompose_SkipMissing_NoError(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// Only one module has a scaffold dir; the other two are absent.
	makeModule(t, root, "present/module", map[string]string{
		"main.go": "package main\n",
	})

	result, err := scaffold.Compose(
		[]string{"absent/alpha", "present/module", "absent/beta"},
		scaffold.CompositionOptions{ModulesRoot: root, SkipMissing: true},
	)
	if err != nil {
		t.Fatalf("SkipMissing=true must not error on missing modules, got: %v", err)
	}
	// The present module's file must be included.
	if _, ok := result.Files["main.go"]; !ok {
		t.Error("main.go from present module not in result")
	}
	// Both absent modules must appear in MissingModules.
	if len(result.MissingModules) != 2 {
		t.Errorf("MissingModules = %v, want [absent/alpha absent/beta]", result.MissingModules)
	}
	for _, want := range []string{"absent/alpha", "absent/beta"} {
		found := false
		for _, m := range result.MissingModules {
			if m == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("MissingModules does not contain %q; got %v", want, result.MissingModules)
		}
	}
}

// ── TEST-COMP-08c: SkipMissing=false (default) still errors ───────────────────

func TestCompose_SkipMissing_DefaultFalse(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	_, err := scaffold.Compose(
		[]string{"absent/module"},
		scaffold.CompositionOptions{ModulesRoot: root, SkipMissing: false},
	)
	if err == nil {
		t.Fatal("expected ModuleNotFoundError when SkipMissing=false, got nil")
	}
	var mnfe *scaffold.ModuleNotFoundError
	if !isModuleNotFoundError(err, &mnfe) {
		t.Errorf("want *scaffold.ModuleNotFoundError, got %T: %v", err, err)
	}
}

// ── TEST-COMP-08d: all modules missing + SkipMissing → empty files, all listed ─

func TestCompose_SkipMissing_AllMissing(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	result, err := scaffold.Compose(
		[]string{"x/a", "x/b", "x/c"},
		scaffold.CompositionOptions{ModulesRoot: root, SkipMissing: true},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Files) != 0 {
		t.Errorf("expected zero files, got %d", len(result.Files))
	}
	if len(result.MissingModules) != 3 {
		t.Errorf("MissingModules = %v, want 3 entries", result.MissingModules)
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

// ── TEST-COMP-11: language-variant scaffold-python/ preferred (TG-14) ─────────

func TestCompose_LanguageVariant_Python(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	// Module has both scaffold/ (Go) and scaffold-python/ (Python).
	makeModule(t, root, "core/rbac", map[string]string{
		"internal/middleware/rbac.go.tmpl": "package middleware\n",
	})
	// Create scaffold-python/ with a Python-specific file.
	langDir := filepath.Join(root, "core/rbac", "scaffold-python")
	if err := os.MkdirAll(filepath.Join(langDir, "backend/src/core"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(langDir, "backend/src/core", "permissions.py.tmpl"), []byte("# permissions\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := scaffold.Compose(
		[]string{"core/rbac"},
		scaffold.CompositionOptions{ModulesRoot: root, Language: "python"},
	)
	if err != nil {
		t.Fatalf("Compose with Language=python: %v", err)
	}
	// Should pick scaffold-python/ → Python file present.
	if _, ok := result.Files["backend/src/core/permissions.py.tmpl"]; !ok {
		t.Errorf("expected Python permissions file; got keys: %v", fileKeys(result))
	}
	// Should NOT include Go file from scaffold/.
	if _, ok := result.Files["internal/middleware/rbac.go.tmpl"]; ok {
		t.Errorf("Go file should not be present when scaffold-python/ was selected")
	}
}

// ── TEST-COMP-12: no language-variant → falls back to scaffold/ (TG-14) ───────

func TestCompose_LanguageVariant_Go(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	// Module has scaffold/ with a Go file and scaffold-python/ with a Python file.
	makeModule(t, root, "core/rbac", map[string]string{
		"internal/middleware/rbac.go.tmpl": "package middleware\n",
	})
	langDir := filepath.Join(root, "core/rbac", "scaffold-python")
	if err := os.MkdirAll(filepath.Join(langDir, "backend/src/core"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(langDir, "backend/src/core", "permissions.py.tmpl"), []byte("# permissions\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// No Language set → should use default scaffold/.
	result, err := scaffold.Compose(
		[]string{"core/rbac"},
		scaffold.CompositionOptions{ModulesRoot: root},
	)
	if err != nil {
		t.Fatalf("Compose without Language: %v", err)
	}
	if _, ok := result.Files["internal/middleware/rbac.go.tmpl"]; !ok {
		t.Errorf("expected Go rbac file from default scaffold/; got keys: %v", fileKeys(result))
	}
	if _, ok := result.Files["backend/src/core/permissions.py.tmpl"]; ok {
		t.Errorf("Python file should not be present when Language is empty")
	}
}

// ── TEST-COMP-13: language set but no variant exists → falls back to scaffold/ ─

func TestCompose_LanguageVariant_FallsBackWhenMissing(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	// Module has only scaffold/ (no scaffold-go/).
	makeModule(t, root, "core/rbac", map[string]string{
		"internal/middleware/rbac.go.tmpl": "package middleware\n",
	})

	// Language=go but no scaffold-go/ exists → fall back to scaffold/.
	result, err := scaffold.Compose(
		[]string{"core/rbac"},
		scaffold.CompositionOptions{ModulesRoot: root, Language: "go"},
	)
	if err != nil {
		t.Fatalf("Compose with Language=go (no variant): %v", err)
	}
	if _, ok := result.Files["internal/middleware/rbac.go.tmpl"]; !ok {
		t.Errorf("expected fallback to scaffold/ when scaffold-go/ absent; got keys: %v", fileKeys(result))
	}
}

// ── TEST-COMP-14: core/mcp-server Go scaffold composes without error ──────────

func TestCompose_MCPServer_GoScaffold(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	makeModule(t, root, "core/mcp-server", map[string]string{
		"cmd/mcp/main.go.tmpl":                        "package main\n",
		"internal/mcpserver/server.go.tmpl":            "package mcpserver\n",
		"internal/mcpserver/tools.go.tmpl":             "package mcpserver\n",
		"internal/mcpserver/tools_test.go.tmpl":        "package mcpserver_test\n",
		".vscode/settings.json.tmpl":                   `{"mcp":{"servers":{}}}`,
		".env.example":                                 "MCP_SERVER_NAME=myapp\n",
	})

	result, err := scaffold.Compose(
		[]string{"core/mcp-server"},
		scaffold.CompositionOptions{ModulesRoot: root},
	)
	if err != nil {
		t.Fatalf("Compose core/mcp-server (Go): %v", err)
	}
	for _, want := range []string{
		"cmd/mcp/main.go.tmpl",
		"internal/mcpserver/server.go.tmpl",
		"internal/mcpserver/tools.go.tmpl",
		"internal/mcpserver/tools_test.go.tmpl",
		".vscode/settings.json.tmpl",
		".env.example",
	} {
		if _, ok := result.Files[want]; !ok {
			t.Errorf("expected %q in mcp-server Go scaffold; got keys: %v", want, fileKeys(result))
		}
	}
}

// ── TEST-COMP-15: core/mcp-server Python scaffold selected when Language=python

func TestCompose_MCPServer_PythonScaffold(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// Default Go scaffold.
	makeModule(t, root, "core/mcp-server", map[string]string{
		"cmd/mcp/main.go.tmpl": "package main\n",
	})
	// Python variant.
	pyDir := filepath.Join(root, "core/mcp-server", "scaffold-python")
	for _, f := range []string{
		"mcp_server.py.tmpl",
		"tools/__init__.py.tmpl",
		"requirements-mcp.txt",
		".env.example",
		".vscode/settings.json.tmpl",
	} {
		full := filepath.Join(pyDir, filepath.FromSlash(f))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("# mcp\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	result, err := scaffold.Compose(
		[]string{"core/mcp-server"},
		scaffold.CompositionOptions{ModulesRoot: root, Language: "python"},
	)
	if err != nil {
		t.Fatalf("Compose core/mcp-server (Python): %v", err)
	}
	if _, ok := result.Files["mcp_server.py.tmpl"]; !ok {
		t.Errorf("expected Python mcp_server.py.tmpl; got keys: %v", fileKeys(result))
	}
	if _, ok := result.Files["cmd/mcp/main.go.tmpl"]; ok {
		t.Errorf("Go entry point must not be present when Language=python")
	}
}

// ── TEST-COMP-16: core/mcp-server composed with core/rbac (no file conflict) ─

func TestCompose_MCPServer_ComposedWithRBAC(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	makeModule(t, root, "core/mcp-server", map[string]string{
		"cmd/mcp/main.go.tmpl":               "package main\n",
		"internal/mcpserver/server.go.tmpl":   "package mcpserver\n",
		".env.example":                        "MCP_SERVER_NAME=\n",
	})
	makeModule(t, root, "core/rbac", map[string]string{
		"internal/middleware/rbac.go.tmpl": "package middleware\n",
		".env.example":                     "RBAC_KEY=\n",
	})

	result, err := scaffold.Compose(
		[]string{"core/mcp-server", "core/rbac"},
		scaffold.CompositionOptions{ModulesRoot: root},
	)
	if err != nil {
		t.Fatalf("Compose mcp-server+rbac: %v", err)
	}
	for _, want := range []string{
		"cmd/mcp/main.go.tmpl",
		"internal/mcpserver/server.go.tmpl",
		"internal/middleware/rbac.go.tmpl",
		".env.example", // merged additive
	} {
		if _, ok := result.Files[want]; !ok {
			t.Errorf("expected %q after composition; got keys: %v", want, fileKeys(result))
		}
	}
}
