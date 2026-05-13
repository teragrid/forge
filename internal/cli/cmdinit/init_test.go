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
//   Happy: init in empty dir with ts-service (default) creates package.json + forge.config.ts
//   Happy: init with --template go-service creates go.mod
//   Auto-detect: package.json in cwd → ts-service chosen
//   Auto-detect: go.mod in cwd → go-service chosen
//   Auto-detect: empty dir → ts-service (default)
//   Negative: unknown --template → error with FORGE-5101
//   Negative: non-empty dir without --force → error with FORGE-2201
//   Force: non-empty dir + --force → succeeds
//   Idempotency: init twice with --force → consistent output
//   False-positive guard: ts-service init must NOT create go.mod
//   False-positive guard: go-service init must NOT create forge.config.ts
package cmdinit

import (
	"bytes"
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
