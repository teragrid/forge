// Copyright 2024 The Forge Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Test design (per always-write-tests.md):
//
//	Happy: init in empty dir with ts-service (default) creates package.json + forge.config.ts
//	Happy: init with --template go-service creates go.mod
//	Auto-detect: package.json in cwd → ts-service chosen
//	Auto-detect: go.mod in cwd → go-service chosen
//	Auto-detect: empty dir → ts-service (default)
//	Negative: unknown --template → error with FORGE-5101
//	Negative: non-empty dir without --force → error with FORGE-2201
//	Force: non-empty dir + --force → succeeds
//	Idempotency: init twice with --force → consistent output
//	False-positive guard: ts-service init must NOT create go.mod
//	False-positive guard: go-service init must NOT create forge.config.ts
package cmdinit

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runInit runs `forge init <dir>` with the given extra args.
// Passes dir as a positional argument — no os.Chdir needed (safe for t.Parallel).
func runInit(t *testing.T, dir string, extraArgs ...string) (string, error) {
	t.Helper()
	cmd := New("9.9.9-test")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	// Prepend dir as the positional path argument.
	args := append([]string{dir}, extraArgs...)
	cmd.SetArgs(args)
	execErr := cmd.Execute()
	return out.String(), execErr
}

// ─── Auto-detection ───────────────────────────────────────────────────────────

func TestInit_AutoDetect_EmptyDir_DefaultsToTSService(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	out, err := runInit(t, dir)
	if err != nil {
		t.Fatalf("run: %v\nout: %s", err, out)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "forge.config.ts")); statErr != nil {
		t.Errorf("expected forge.config.ts (ts-service default); got error: %v", statErr)
	}
}

func TestInit_AutoDetect_PackageJSON_PicksTSService(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// Pre-create package.json to simulate an existing Node project.
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := runInit(t, dir, "--force") // --force because dir is non-empty
	if err != nil {
		t.Fatalf("run: %v\nout: %s", err, out)
	}
	// ts-service should have been applied: AGENTS.md is a ts-service file
	if _, statErr := os.Stat(filepath.Join(dir, "AGENTS.md")); statErr != nil {
		t.Errorf("expected AGENTS.md from ts-service detection: %v", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil {
		t.Error("ts-service init must NOT create go.mod")
	}
}

func TestInit_AutoDetect_GoMod_PicksGoService(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// Pre-create go.mod to simulate an existing Go project.
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/test\n\ngo 1.24\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := runInit(t, dir, "--force")
	if err != nil {
		t.Fatalf("run: %v\nout: %s", err, out)
	}
	// go-service should have been applied: main.go is a go-service file
	if _, statErr := os.Stat(filepath.Join(dir, "main.go")); statErr != nil {
		t.Errorf("expected main.go from go-service detection: %v", statErr)
	}
}

// ─── Happy path ───────────────────────────────────────────────────────────────

func TestInit_TSService_CreatesExpectedFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	out, err := runInit(t, dir, "--template", "ts-service")
	if err != nil {
		t.Fatalf("run: %v\nout: %s", err, out)
	}
	for _, rel := range []string{
		"package.json", "tsconfig.json", "forge.config.ts",
		"src/modules/auth/auth.service.ts",
		"migrations/20260101000000_init.sql",
		".github/workflows/ci.yml",
		"README.md",
	} {
		if _, statErr := os.Stat(filepath.Join(dir, rel)); statErr != nil {
			t.Errorf("missing %s: %v", rel, statErr)
		}
	}
}

func TestInit_GoService_CreatesExpectedFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	out, err := runInit(t, dir, "--template", "go-service")
	if err != nil {
		t.Fatalf("run: %v\nout: %s", err, out)
	}
	for _, rel := range []string{"go.mod", "main.go", "main_test.go", "README.md"} {
		if _, statErr := os.Stat(filepath.Join(dir, rel)); statErr != nil {
			t.Errorf("missing %s: %v", rel, statErr)
		}
	}
}

// ─── Negative cases ───────────────────────────────────────────────────────────

func TestInit_UnknownTemplate_ReturnsError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_, err := runInit(t, dir, "--template", "no-such-template")
	if err == nil {
		t.Fatal("expected error for unknown template")
	}
	if !strings.Contains(err.Error(), "FORGE-5101") {
		t.Fatalf("expected FORGE-5101 in error, got: %v", err)
	}
}

func TestInit_NonEmptyDirWithoutForce_ReturnsError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "existing.txt"), []byte("hi"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := runInit(t, dir, "--template", "ts-service")
	if err == nil {
		t.Fatal("expected error for non-empty dir without --force")
	}
	// Should be FORGE-2201 from the scaffold engine
	if !strings.Contains(err.Error(), "FORGE-2201") {
		t.Fatalf("expected FORGE-2201, got: %v", err)
	}
}

// ─── Force flag ───────────────────────────────────────────────────────────────

func TestInit_Force_SucceedsInNonEmptyDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "existing.txt"), []byte("hi"), 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := runInit(t, dir, "--template", "ts-service", "--force")
	if err != nil {
		t.Fatalf("run with --force: %v\nout: %s", err, out)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "package.json")); statErr != nil {
		t.Errorf("package.json missing after --force init: %v", statErr)
	}
}

// ─── Idempotency ──────────────────────────────────────────────────────────────

func TestInit_ForceIdempotent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if _, err := runInit(t, dir, "--template", "ts-service"); err != nil {
		t.Fatalf("first init: %v", err)
	}
	if _, err := runInit(t, dir, "--template", "ts-service", "--force"); err != nil {
		t.Fatalf("second init (force): %v", err)
	}
	// AGENTS.md must still exist and be valid after second run.
	if _, statErr := os.Stat(filepath.Join(dir, "AGENTS.md")); statErr != nil {
		t.Errorf("AGENTS.md missing after second init: %v", statErr)
	}
}

// ─── False-positive guards ────────────────────────────────────────────────────

func TestInit_TSService_DoesNotCreateGoMod(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if _, err := runInit(t, dir, "--template", "ts-service"); err != nil {
		t.Fatalf("run: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil {
		t.Error("ts-service init must NOT create go.mod")
	}
}

func TestInit_GoService_DoesNotCreateForgeConfigTS(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if _, err := runInit(t, dir, "--template", "go-service"); err != nil {
		t.Fatalf("run: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "forge.config.ts")); statErr == nil {
		t.Error("go-service init must NOT create forge.config.ts")
	}
}

// ─── Name substitution ────────────────────────────────────────────────────────

func TestInit_CustomName_AppearsInReadme(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	out, err := runInit(t, dir, "--template", "ts-service", "--name", "custom-project")
	if err != nil {
		t.Fatalf("run: %v\nout: %s", err, out)
	}
	body, err := os.ReadFile(filepath.Join(dir, "README.md"))
	if err != nil {
		t.Fatalf("read README.md: %v", err)
	}
	if !strings.Contains(string(body), "custom-project") {
		t.Errorf("README.md: custom-project name not found; snippet: %.200s", string(body))
	}
}

// ─── Output format ────────────────────────────────────────────────────────────

func TestInit_OutputMentionsTemplate(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	out, err := runInit(t, dir, "--template", "ts-service")
	if err != nil {
		t.Fatalf("run: %v\nout: %s", err, out)
	}
	if !strings.Contains(out, "ts-service") {
		t.Errorf("output does not mention template; got: %s", out)
	}
	if !strings.Contains(out, "npm install") {
		t.Errorf("output missing npm next-steps; got: %s", out)
	}
}

func TestInit_GoService_OutputMentionsGoModTidy(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	out, err := runInit(t, dir, "--template", "go-service")
	if err != nil {
		t.Fatalf("run: %v\nout: %s", err, out)
	}
	if !strings.Contains(out, "go mod tidy") {
		t.Errorf("output missing go mod tidy next-step; got: %s", out)
	}
}

// ─── --minimal flag ───────────────────────────────────────────────────────────

// Test design (always-write-tests.md):
//
// Happy:        --minimal in empty dir creates all 7 expected files.
// Happy:        --minimal in non-empty dir succeeds (force is implied).
// Happy:        --name is respected in forge.config.yml content.
// Happy:        --json emits valid JSON with files array.
// Boundary:     knowledge-index.json is valid JSON (even when index is empty/stub).
// Boundary:     global.instructions.md is non-empty.
// False-positive: --minimal must NOT create go.mod, package.json, or main.go.
// Idempotency:  running --minimal twice produces the same 7 files.
// Regression:   forge ship next-steps appear in --minimal output.

// minimalFiles is the expected set of files written by forge init --minimal.
var minimalFiles = []string{
	".forge/conventions.json",
	".forge/hygiene.yml",
	".forge/instructions/global.instructions.md",
	".forge/knowledge-index.json",
	".forge/manifest",
	"AGENTS.md",
	"forge.config.yml",
}

func TestInit_Minimal_HappyPath_CreatesAllFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	out, err := runInit(t, dir, "--minimal")
	if err != nil {
		t.Fatalf("run: %v\nout: %s", err, out)
	}
	for _, rel := range minimalFiles {
		if _, statErr := os.Stat(filepath.Join(dir, rel)); statErr != nil {
			t.Errorf("missing expected file %s: %v", rel, statErr)
		}
	}
}

func TestInit_Minimal_NonEmptyDir_Succeeds(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// Simulate an existing project with its own files.
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/ai-agent\n\ngo 1.24\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := runInit(t, dir, "--minimal")
	if err != nil {
		t.Fatalf("--minimal on non-empty dir should succeed (force implied): %v\nout: %s", err, out)
	}
	for _, rel := range minimalFiles {
		if _, statErr := os.Stat(filepath.Join(dir, rel)); statErr != nil {
			t.Errorf("missing %s: %v", rel, statErr)
		}
	}
	// Existing project files must be untouched.
	if _, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr != nil {
		t.Error("go.mod was unexpectedly removed by --minimal")
	}
	if _, statErr := os.Stat(filepath.Join(dir, "main.go")); statErr != nil {
		t.Error("main.go was unexpectedly removed by --minimal")
	}
}

func TestInit_Minimal_NameAppearsInConfig(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	out, err := runInit(t, dir, "--minimal", "--name", "ai-marketing-platform")
	if err != nil {
		t.Fatalf("run: %v\nout: %s", err, out)
	}
	body, err := os.ReadFile(filepath.Join(dir, "forge.config.yml"))
	if err != nil {
		t.Fatalf("read forge.config.yml: %v", err)
	}
	if !strings.Contains(string(body), "ai-marketing-platform") {
		t.Errorf("forge.config.yml: project name not found; snippet: %.200s", string(body))
	}
	agentsBody, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if err != nil {
		t.Fatalf("read AGENTS.md: %v", err)
	}
	if !strings.Contains(string(agentsBody), "ai-marketing-platform") {
		t.Errorf("AGENTS.md: project name not found; snippet: %.200s", string(agentsBody))
	}
}

func TestInit_Minimal_KnowledgeIndexIsValidJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if _, err := runInit(t, dir, "--minimal"); err != nil {
		t.Fatalf("run: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, ".forge/knowledge-index.json"))
	if err != nil {
		t.Fatalf("read knowledge-index.json: %v", err)
	}
	var v map[string]any
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Errorf("knowledge-index.json is not valid JSON: %v\ncontent: %.200s", err, raw)
	}
	if _, ok := v["version"]; !ok {
		t.Error("knowledge-index.json missing required 'version' field")
	}
	if _, ok := v["entries"]; !ok {
		t.Error("knowledge-index.json missing required 'entries' field")
	}
}

func TestInit_Minimal_GlobalInstructionsIsNonEmpty(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if _, err := runInit(t, dir, "--minimal"); err != nil {
		t.Fatalf("run: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, ".forge/instructions/global.instructions.md"))
	if err != nil {
		t.Fatalf("read global.instructions.md: %v", err)
	}
	if len(raw) == 0 {
		t.Error("global.instructions.md is empty")
	}
	if !strings.Contains(string(raw), "forge ship") {
		t.Errorf("global.instructions.md missing forge ship reference; got: %.200s", raw)
	}
}

func TestInit_Minimal_DoesNotCreateLanguageFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if _, err := runInit(t, dir, "--minimal"); err != nil {
		t.Fatalf("run: %v", err)
	}
	// False-positive guard: minimal must NOT write any language-specific files.
	for _, unwanted := range []string{"go.mod", "package.json", "main.go", "tsconfig.json", "docker-compose.yml"} {
		if _, statErr := os.Stat(filepath.Join(dir, unwanted)); statErr == nil {
			t.Errorf("--minimal must NOT create %s", unwanted)
		}
	}
}

func TestInit_Minimal_Idempotent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if _, err := runInit(t, dir, "--minimal"); err != nil {
		t.Fatalf("first run: %v", err)
	}
	if _, err := runInit(t, dir, "--minimal"); err != nil {
		t.Fatalf("second run (idempotency): %v", err)
	}
	// All files still present after second run.
	for _, rel := range minimalFiles {
		if _, statErr := os.Stat(filepath.Join(dir, rel)); statErr != nil {
			t.Errorf("missing %s after second --minimal run: %v", rel, statErr)
		}
	}
}

func TestInit_Minimal_OutputMentionsForgeShip(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	out, err := runInit(t, dir, "--minimal")
	if err != nil {
		t.Fatalf("run: %v\nout: %s", err, out)
	}
	if !strings.Contains(out, "forge ship") {
		t.Errorf("--minimal output must mention forge ship next-step; got: %s", out)
	}
}

func TestInit_Minimal_JSONOutput(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	out, err := runInit(t, dir, "--minimal", "--json")
	if err != nil {
		t.Fatalf("run: %v\nout: %s", err, out)
	}
	var res struct {
		Template string   `json:"Template"`
		Target   string   `json:"Target"`
		Files    []string `json:"Files"`
	}
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("--json output is not valid JSON: %v\nraw: %s", err, out)
	}
	if res.Template != "minimal" {
		t.Errorf("Template field: want %q, got %q", "minimal", res.Template)
	}
	if len(res.Files) != len(minimalFiles) {
		t.Errorf("Files count: want %d, got %d; files: %v", len(minimalFiles), len(res.Files), res.Files)
	}
}
