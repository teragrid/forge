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

	// Core Go-service files
	coreFiles := []string{
		"README.md", "go.mod", "main.go", "main_test.go",
		".gitignore", ".gitleaks.toml", ".forge/manifest",
	}
	// Developer Promise #4 — AI-context files (spec §8 Q13 + §11.1.2 #4)
	aiContextFiles := []string{
		"AGENTS.md", "CLAUDE.md", ".cursorrules", ".windsurfrules",
		".forge/instructions/global.instructions.md",
	}
	// Forge framework files (spec §5 scaffold structure)
	forgeFiles := []string{
		".forge/hygiene.yml", ".forge/conventions.json", ".forge-conventions",
		"forge.config.yml",
	}
	// DevOps / deployment files (spec §5 scaffold + §14 Deployment)
	devopsFiles := []string{
		".github/workflows/ci.yml", "docker-compose.yml", "ROLLBACK.md",
	}

	for _, want := range append(append(append(coreFiles, aiContextFiles...), forgeFiles...), devopsFiles...) {
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

// TestRender_AIContextFilesContainProjectName verifies that AI-context files
// have the project name substituted (data-accuracy for Developer Promise #4).
func TestRender_AIContextFilesContainProjectName(t *testing.T) {
	t.Parallel()
	target := t.TempDir()
	render(t, target, "my-saas", false)

	for _, rel := range []string{"AGENTS.md", "CLAUDE.md", ".cursorrules", ".windsurfrules",
		".forge/instructions/global.instructions.md"} {
		body, err := os.ReadFile(filepath.Join(target, rel))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		s := string(body)
		if !strings.Contains(s, "my-saas") {
			t.Errorf("%s: project name 'my-saas' not substituted; snippet: %.120s", rel, s)
		}
		if !strings.Contains(s, "9.9.9-test") {
			t.Errorf("%s: forge version '9.9.9-test' not substituted; snippet: %.120s", rel, s)
		}
	}
}

// TestRender_ForgeConfigValid verifies forge.config.yml is rendered with name substitution.
func TestRender_ForgeConfigValid(t *testing.T) {
	t.Parallel()
	target := t.TempDir()
	render(t, target, "acme-api", false)

	body, err := os.ReadFile(filepath.Join(target, "forge.config.yml"))
	if err != nil {
		t.Fatalf("read forge.config.yml: %v", err)
	}
	s := string(body)
	if !strings.Contains(s, `project: "acme-api"`) {
		t.Errorf("forge.config.yml: project name not substituted; got snippet: %.200s", s)
	}
	// False-positive guard: no unresolved template markers
	if strings.Contains(s, "{{") || strings.Contains(s, "}}") {
		t.Errorf("forge.config.yml: unresolved template marker found in: %.200s", s)
	}
}

// TestRender_CIWorkflowContainsProjectName verifies the CI workflow is generated correctly.
func TestRender_CIWorkflowContainsProjectName(t *testing.T) {
	t.Parallel()
	target := t.TempDir()
	render(t, target, "hello-service", false)

	body, err := os.ReadFile(filepath.Join(target, ".github/workflows/ci.yml"))
	if err != nil {
		t.Fatalf("read ci.yml: %v", err)
	}
	s := string(body)
	if !strings.Contains(s, "hello-service") {
		t.Errorf("ci.yml: project name 'hello-service' not substituted; snippet: %.200s", s)
	}
}

// TestRender_ROLLBACKContainsProjectName verifies ROLLBACK.md is generated with the name.
func TestRender_ROLLBACKContainsProjectName(t *testing.T) {
	t.Parallel()
	target := t.TempDir()
	render(t, target, "rollout-svc", false)

	body, err := os.ReadFile(filepath.Join(target, "ROLLBACK.md"))
	if err != nil {
		t.Fatalf("read ROLLBACK.md: %v", err)
	}
	if !strings.Contains(string(body), "rollout-svc") {
		t.Errorf("ROLLBACK.md: project name 'rollout-svc' not substituted")
	}
}

// TestRender_ConventionsJSONValid verifies .forge/conventions.json has no unresolved markers.
func TestRender_ConventionsJSONValid(t *testing.T) {
	t.Parallel()
	target := t.TempDir()
	render(t, target, "check-app", false)

	body, err := os.ReadFile(filepath.Join(target, ".forge/conventions.json"))
	if err != nil {
		t.Fatalf("read conventions.json: %v", err)
	}
	s := string(body)
	// False-positive guard: no unresolved Go template markers remain
	if strings.Contains(s, "{{") || strings.Contains(s, "}}") {
		t.Errorf("conventions.json: unresolved template marker; snippet: %.200s", s)
	}
	// Data accuracy: project name present
	if !strings.Contains(s, "check-app") {
		t.Errorf("conventions.json: project name 'check-app' not found; snippet: %.200s", s)
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

	for _, rel := range []string{
		"README.md", ".gitignore", ".gitleaks.toml", ".forge/manifest",
		"AGENTS.md", "CLAUDE.md", ".cursorrules", ".windsurfrules",
		".forge/hygiene.yml", "forge.config.yml", "ROLLBACK.md",
		".forge/instructions/global.instructions.md",
	} {
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

// ─── ts-service template tests ───────────────────────────────────────────────

func renderTS(t *testing.T, target, name string, force bool) *Result {
	t.Helper()
	res, err := Render(Options{
		Template: "ts-service",
		Target:   target,
		Vars:     Vars{Name: name, ForgeVer: "9.9.9-test"},
		Force:    force,
	})
	if err != nil {
		t.Fatalf("Render ts-service: %v", err)
	}
	return res
}

func TestRenderTSService_AvailableTemplatesIncludesIt(t *testing.T) {
	t.Parallel()
	found := false
	for _, n := range AvailableTemplates() {
		if n == "ts-service" {
			found = true
		}
	}
	if !found {
		t.Fatal("ts-service not in AvailableTemplates()")
	}
}

// TestRenderTSService_Happy verifies all expected files are created.
func TestRenderTSService_Happy(t *testing.T) {
	t.Parallel()
	target := t.TempDir()
	res := renderTS(t, target, "my-api", false)

	wantFiles := []string{
		// AI context
		"AGENTS.md", "CLAUDE.md", ".cursorrules", ".windsurfrules",
		// Config & package
		"forge.config.ts", "package.json", "tsconfig.json",
		"vitest.config.ts", "eslint.config.js",
		// Forge conventions
		".forge/conventions.json", ".forge/hygiene.yml", ".forge-conventions",
		// Instructions
		".forge/instructions/global.instructions.md",
		".forge/instructions/auth.instructions.md",
		// Entry point + shared utilities
		"src/main.ts",
		"src/shared/logger.ts",
		// Shared errors, types, middleware, guards
		"src/shared/errors.ts",
		"src/shared/types.ts",
		"src/shared/middleware/auth.middleware.ts",
		"src/shared/guards/workspace.guard.ts",
		// Infrastructure
		"src/infrastructure/database/client.ts",
		"src/infrastructure/queue/client.ts",
		"src/infrastructure/storage/client.ts",
		// Auth module
		"src/modules/auth/auth.service.ts",
		"src/modules/auth/auth.controller.ts",
		"src/modules/auth/auth.types.ts",
		"src/modules/auth/auth.test.ts",
		"src/modules/auth/auth.routes.ts",
		"src/modules/auth/auth.stubs.ts",
		// Migration
		"migrations/20260101000000_init.sql",
		// GitHub Actions
		".github/workflows/ci.yml",
		".github/workflows/deploy-staging.yml",
		".github/workflows/deploy-production.yml",
		// Docs
		"README.md", "ROLLBACK.md",
		// Security
		".gitignore", ".gitleaks.toml",
		// Security tests
		"tests/security/auth.security.test.ts",
	}

	for _, want := range wantFiles {
		if _, err := os.Stat(filepath.Join(target, want)); err != nil {
			t.Errorf("missing file %s: %v", want, err)
		}
		found := false
		for _, f := range res.Files {
			if filepath.ToSlash(f) == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Result.Files missing %s", want)
		}
	}
}

// TestRenderTSService_NameSubstitution verifies {{.Name}} is replaced throughout.
func TestRenderTSService_NameSubstitution(t *testing.T) {
	t.Parallel()
	target := t.TempDir()
	renderTS(t, target, "my-saas", false)

	for _, rel := range []string{
		"AGENTS.md", "CLAUDE.md", "README.md",
		"forge.config.ts", "package.json",
		".forge/instructions/global.instructions.md",
	} {
		body, err := os.ReadFile(filepath.Join(target, rel))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		s := string(body)
		if !strings.Contains(s, "my-saas") {
			t.Errorf("%s: project name 'my-saas' not substituted; snippet: %.120s", rel, s)
		}
		if strings.Contains(s, "{{") || strings.Contains(s, "}}") {
			t.Errorf("%s: unresolved template marker found; snippet: %.120s", rel, s)
		}
	}
}

// TestRenderTSService_NoGoFiles verifies ts-service does NOT generate Go files.
// This is a false-positive guard: the template must not contain go.mod or main.go.
func TestRenderTSService_NoGoFiles(t *testing.T) {
	t.Parallel()
	target := t.TempDir()
	res := renderTS(t, target, "no-go-files", false)

	for _, f := range res.Files {
		if f == "go.mod" || f == "main.go" || strings.HasSuffix(f, ".go") {
			t.Errorf("ts-service must not generate Go file %s", f)
		}
	}
	if _, err := os.Stat(filepath.Join(target, "go.mod")); err == nil {
		t.Error("ts-service: go.mod must not exist")
	}
}

// TestRenderTSService_PackageJSONHasScripts verifies package.json contains forge scripts.
func TestRenderTSService_PackageJSONHasScripts(t *testing.T) {
	t.Parallel()
	target := t.TempDir()
	renderTS(t, target, "pkg-check", false)

	body, err := os.ReadFile(filepath.Join(target, "package.json"))
	if err != nil {
		t.Fatalf("read package.json: %v", err)
	}
	s := string(body)
	for _, want := range []string{`"test"`, `"build"`, `"lint"`, `"migrate:up"`} {
		if !strings.Contains(s, want) {
			t.Errorf("package.json missing script %s", want)
		}
	}
	if !strings.Contains(s, "pkg-check") {
		t.Errorf("package.json: project name not substituted")
	}
}

// TestRenderTSService_MigrationHasUpDown verifies the migration has both up and down sections.
func TestRenderTSService_MigrationHasUpDown(t *testing.T) {
	t.Parallel()
	target := t.TempDir()
	renderTS(t, target, "db-check", false)

	body, err := os.ReadFile(filepath.Join(target, "migrations/20260101000000_init.sql"))
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	s := string(body)
	if !strings.Contains(s, "migrate:up") {
		t.Errorf("migration missing -- migrate:up section")
	}
	if !strings.Contains(s, "migrate:down") {
		t.Errorf("migration missing -- migrate:down section")
	}
}

// TestRenderTSService_ForgeConfigTSNoUnresolvedMarkers validates forge.config.ts template.
func TestRenderTSService_ForgeConfigTSNoUnresolvedMarkers(t *testing.T) {
	t.Parallel()
	target := t.TempDir()
	renderTS(t, target, "config-test", false)

	body, err := os.ReadFile(filepath.Join(target, "forge.config.ts"))
	if err != nil {
		t.Fatalf("read forge.config.ts: %v", err)
	}
	s := string(body)
	if strings.Contains(s, "{{") || strings.Contains(s, "}}") {
		t.Errorf("forge.config.ts: unresolved template marker: %.200s", s)
	}
	if !strings.Contains(s, "config-test") {
		t.Errorf("forge.config.ts: project name not substituted")
	}
}

// TestRenderTSService_EmptyDirsExist verifies placeholder directories are created.
func TestRenderTSService_EmptyDirsExist(t *testing.T) {
	t.Parallel()
	target := t.TempDir()
	renderTS(t, target, "dirs-check", false)

	for _, dir := range []string{
		"src/shared/guards",
		"src/shared/decorators",
		"src/infrastructure/database",
		"src/infrastructure/queue",
		"src/infrastructure/storage",
		"tests/unit",
		"tests/integration",
		"tests/security",
		"src/modules/workspace",
		"src/modules/billing",
		".forge/context-bundles",
		".forge/lint-rules",
	} {
		if info, err := os.Stat(filepath.Join(target, dir)); err != nil || !info.IsDir() {
			t.Errorf("expected directory %s to exist", dir)
		}
	}
}

// TestRenderTSService_EntryPointExists verifies src/main.ts is present and references the project name.
// Regression: before this was added, `npm run dev` would fail on a fresh scaffold.
func TestRenderTSService_EntryPointExists(t *testing.T) {
	t.Parallel()
	target := t.TempDir()
	renderTS(t, target, "entry-check", false)

	body, err := os.ReadFile(filepath.Join(target, "src/main.ts"))
	if err != nil {
		t.Fatalf("src/main.ts missing — npm run dev would fail: %v", err)
	}
	s := string(body)
	if !strings.Contains(s, "entry-check") {
		t.Errorf("src/main.ts: project name 'entry-check' not substituted; snippet: %.200s", s)
	}
	// Must export the server for integration tests
	if !strings.Contains(s, "export") {
		t.Errorf("src/main.ts: must export server for testing; snippet: %.200s", s)
	}
	// Must have health endpoints
	if !strings.Contains(s, "/healthz") {
		t.Errorf("src/main.ts: missing /healthz endpoint; snippet: %.200s", s)
	}
	if strings.Contains(s, "{{") || strings.Contains(s, "}}") {
		t.Errorf("src/main.ts: unresolved template marker; snippet: %.200s", s)
	}
}

// TestRenderTSService_VitestConfigExists verifies vitest.config.ts is present and valid.
func TestRenderTSService_VitestConfigExists(t *testing.T) {
	t.Parallel()
	target := t.TempDir()
	renderTS(t, target, "vitest-check", false)

	body, err := os.ReadFile(filepath.Join(target, "vitest.config.ts"))
	if err != nil {
		t.Fatalf("vitest.config.ts missing — npm test may fail: %v", err)
	}
	s := string(body)
	if !strings.Contains(s, "defineConfig") {
		t.Errorf("vitest.config.ts: expected defineConfig call")
	}
	if strings.Contains(s, "{{") || strings.Contains(s, "}}") {
		t.Errorf("vitest.config.ts: unresolved template marker; snippet: %.200s", s)
	}
}

// TestRenderTSService_ESLintConfigExists verifies eslint.config.js is present.
func TestRenderTSService_ESLintConfigExists(t *testing.T) {
	t.Parallel()
	target := t.TempDir()
	renderTS(t, target, "eslint-check", false)

	body, err := os.ReadFile(filepath.Join(target, "eslint.config.js"))
	if err != nil {
		t.Fatalf("eslint.config.js missing — npm run lint would fail: %v", err)
	}
	s := string(body)
	if !strings.Contains(s, "@typescript-eslint") {
		t.Errorf("eslint.config.js: expected @typescript-eslint plugin")
	}
	if strings.Contains(s, "{{") || strings.Contains(s, "}}") {
		t.Errorf("eslint.config.js: unresolved template marker; snippet: %.200s", s)
	}
}

// TestRenderTSService_SharedLoggerExists verifies src/shared/logger.ts is present.
func TestRenderTSService_SharedLoggerExists(t *testing.T) {
	t.Parallel()
	target := t.TempDir()
	renderTS(t, target, "logger-check", false)

	body, err := os.ReadFile(filepath.Join(target, "src/shared/logger.ts"))
	if err != nil {
		t.Fatalf("src/shared/logger.ts missing: %v", err)
	}
	s := string(body)
	if !strings.Contains(s, "logger-check") {
		t.Errorf("src/shared/logger.ts: project name not substituted; snippet: %.200s", s)
	}
	if strings.Contains(s, "{{") || strings.Contains(s, "}}") {
		t.Errorf("src/shared/logger.ts: unresolved template marker; snippet: %.200s", s)
	}
}

// ---------------------------------------------------------------------------
// next-app template tests
// ---------------------------------------------------------------------------

func renderNextApp(t *testing.T, target, name string, force bool) *Result {
	t.Helper()
	res, err := Render(Options{
		Template: "next-app",
		Target:   target,
		Vars:     Vars{Name: name, ForgeVer: "9.9.9-test"},
		Force:    force,
	})
	if err != nil {
		t.Fatalf("Render next-app: %v", err)
	}
	return res
}

// TestRenderNextApp_AvailableTemplatesIncludesIt verifies next-app is listed.
func TestRenderNextApp_AvailableTemplatesIncludesIt(t *testing.T) {
	t.Parallel()
	found := false
	for _, tmpl := range AvailableTemplates() {
		if tmpl == "next-app" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("next-app not in AvailableTemplates(); got %v", AvailableTemplates())
	}
}

// TestRenderNextApp_Happy verifies all expected files are created.
func TestRenderNextApp_Happy(t *testing.T) {
	t.Parallel()
	target := t.TempDir()
	renderNextApp(t, target, "my-next-app", false)

	wantFiles := []string{
		"AGENTS.md",
		"CLAUDE.md",
		".cursorrules",
		".windsurfrules",
		"README.md",
		"ROLLBACK.md",
		"forge.config.ts",
		"next.config.ts",
		"tailwind.config.ts",
		"postcss.config.js",
		"package.json",
		"tsconfig.json",
		".forge/conventions.json",
		".forge/hygiene.yml",
		".forge-conventions",
		".forge/manifest",
		".forge/instructions/global.instructions.md",
		"app/layout.tsx",
		"app/page.tsx",
		"app/globals.css",
		"app/api/health/route.ts",
		"tests/unit/home.test.tsx",
		"tests/e2e/home.spec.ts",
		".gitignore",
		".gitleaks.toml",
		".github/workflows/ci.yml",
		".github/workflows/deploy-production.yml",
	}
	for _, rel := range wantFiles {
		path := filepath.Join(target, filepath.FromSlash(rel))
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected file %s not found", rel)
		}
	}
}

// TestRenderNextApp_NameSubstitution verifies {{.Name}} is replaced throughout.
func TestRenderNextApp_NameSubstitution(t *testing.T) {
	t.Parallel()
	target := t.TempDir()
	renderNextApp(t, target, "unique-proj-xyz", false)

	checkFiles := []string{
		"AGENTS.md",
		"CLAUDE.md",
		"README.md",
		"package.json",
		"forge.config.ts",
		"app/api/health/route.ts",
	}
	for _, rel := range checkFiles {
		body, err := os.ReadFile(filepath.Join(target, filepath.FromSlash(rel)))
		if err != nil {
			t.Errorf("read %s: %v", rel, err)
			continue
		}
		s := string(body)
		if !strings.Contains(s, "unique-proj-xyz") {
			t.Errorf("%s: project name 'unique-proj-xyz' not substituted; snippet: %.200s", rel, s)
		}
		if strings.Contains(s, "{{") || strings.Contains(s, "}}") {
			t.Errorf("%s: unresolved template marker; snippet: %.200s", rel, s)
		}
	}
}

// TestRenderNextApp_NoGoFiles verifies next-app does NOT generate Go files.
func TestRenderNextApp_NoGoFiles(t *testing.T) {
	t.Parallel()
	target := t.TempDir()
	res := renderNextApp(t, target, "no-go-check", false)

	for _, f := range res.Files {
		if strings.HasSuffix(f, ".go") {
			t.Errorf("next-app generated unexpected Go file: %s", f)
		}
	}
}

// TestRenderNextApp_PackageJSONHasScripts verifies package.json contains required scripts.
func TestRenderNextApp_PackageJSONHasScripts(t *testing.T) {
	t.Parallel()
	target := t.TempDir()
	renderNextApp(t, target, "pkg-next-check", false)

	body, err := os.ReadFile(filepath.Join(target, "package.json"))
	if err != nil {
		t.Fatalf("read package.json: %v", err)
	}
	s := string(body)
	for _, want := range []string{`"dev"`, `"build"`, `"test"`, `"lint"`} {
		if !strings.Contains(s, want) {
			t.Errorf("package.json missing script %s", want)
		}
	}
	if !strings.Contains(s, "pkg-next-check") {
		t.Errorf("package.json: project name not substituted")
	}
}

// TestRenderNextApp_HealthRouteExists verifies the /api/health route is correct.
func TestRenderNextApp_HealthRouteExists(t *testing.T) {
	t.Parallel()
	target := t.TempDir()
	renderNextApp(t, target, "health-check", false)

	body, err := os.ReadFile(filepath.Join(target, filepath.FromSlash("app/api/health/route.ts")))
	if err != nil {
		t.Fatalf("app/api/health/route.ts missing: %v", err)
	}
	s := string(body)
	if !strings.Contains(s, "health-check") {
		t.Errorf("route.ts: project name not substituted; snippet: %.200s", s)
	}
	if !strings.Contains(s, "status") {
		t.Errorf("route.ts: expected status field in response; snippet: %.200s", s)
	}
	if strings.Contains(s, "{{") || strings.Contains(s, "}}") {
		t.Errorf("route.ts: unresolved template marker; snippet: %.200s", s)
	}
}

// TestRenderNextApp_NextConfigHasSecurityHeaders verifies next.config.ts has security headers.
func TestRenderNextApp_NextConfigHasSecurityHeaders(t *testing.T) {
	t.Parallel()
	target := t.TempDir()
	renderNextApp(t, target, "sec-headers-check", false)

	body, err := os.ReadFile(filepath.Join(target, "next.config.ts"))
	if err != nil {
		t.Fatalf("next.config.ts missing: %v", err)
	}
	s := string(body)
	if !strings.Contains(s, "X-Frame-Options") {
		t.Errorf("next.config.ts: missing X-Frame-Options security header")
	}
	if strings.Contains(s, "{{") || strings.Contains(s, "}}") {
		t.Errorf("next.config.ts: unresolved template marker; snippet: %.200s", s)
	}
}

// TestRenderNextApp_AppRouterStructureExists verifies App Router files are present.
func TestRenderNextApp_AppRouterStructureExists(t *testing.T) {
	t.Parallel()
	target := t.TempDir()
	renderNextApp(t, target, "app-router-check", false)

	appFiles := []string{
		"app/layout.tsx",
		"app/page.tsx",
		"app/globals.css",
	}
	for _, rel := range appFiles {
		path := filepath.Join(target, filepath.FromSlash(rel))
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("App Router file missing: %s", rel)
		}
	}
}
