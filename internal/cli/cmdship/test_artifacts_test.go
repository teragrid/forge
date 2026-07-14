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

// Test design for J4/J5 (fix-checkpoint-llm-quality-and-observability):
// the test checkpoint's expected-artifact list and generation prompt must
// match the target project's actually-detected language/framework, not
// assume TypeScript/Supabase-RLS (the original G-006 convention) or Go
// (generateTestStubs' prior hardcoded assumption) regardless of project.
//
// Regression fixture for the exact incident: running the test checkpoint on
// this very Go repo previously reported missing artifacts using
// <slug>.test.ts/.rls.test.ts/.scan.baseline.json — a TypeScript/Supabase
// naming convention meaningless in a Go project.

import (
	"os"
	"path/filepath"
	"testing"
)

// ── expectedTestArtifactNames (J5) ─────────────────────────────────────────────

// TestExpectedTestArtifactNames_GoProject — a Go project (go.mod present)
// must expect its own _test.go convention, never the TypeScript/Supabase-RLS
// filenames from an unrelated stack.
func TestExpectedTestArtifactNames_GoProject(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/testproj\n\ngo 1.17\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	names := expectedTestArtifactNames(root, "my-feature")

	// go.mod declares go 1.17 (< 1.18) — no fuzz support, so only the base
	// test file + scan baseline are expected.
	want := map[string]bool{"my-feature_test.go": true, "my-feature.scan.baseline.json": true}
	if len(names) != len(want) {
		t.Fatalf("expected %d artifact names, got %d: %v", len(want), len(names), names)
	}
	for _, n := range names {
		if !want[n] {
			t.Errorf("unexpected artifact name for a Go project: %q (want only %v)", n, want)
		}
	}
	for _, bad := range []string{"my-feature.test.ts", "my-feature.integration.test.ts", "my-feature.rls.test.ts"} {
		for _, n := range names {
			if n == bad {
				t.Errorf("Go project must not expect TypeScript/Supabase artifact %q", bad)
			}
		}
	}
}

// TestExpectedTestArtifactNames_TypeScriptDefault — a project with no
// detected language (or an explicit TS/Jest project) keeps the original
// G-006 four-artifact convention.
func TestExpectedTestArtifactNames_TypeScriptDefault(t *testing.T) {
	t.Parallel()
	root := t.TempDir() // no go.mod, no requirements.txt, no package.json
	names := expectedTestArtifactNames(root, "my-feature")

	want := []string{
		"my-feature.test.ts",
		"my-feature.integration.test.ts",
		"my-feature.rls.test.ts",
		"my-feature.scan.baseline.json",
	}
	if len(names) != len(want) {
		t.Fatalf("expected %d artifact names, got %d: %v", len(want), len(names), names)
	}
	for i, w := range want {
		if names[i] != w {
			t.Errorf("names[%d] = %q, want %q", i, names[i], w)
		}
	}
}

// TestMissingTestArtifacts_GoProject_DoesNotFlagTSFiles — the exact
// regression: missingTestArtifacts on a Go project with a real _test.go and
// scan baseline present must report nothing missing, not flag the absent
// TypeScript files as missing.
func TestMissingTestArtifacts_GoProject_DoesNotFlagTSFiles(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/testproj\n\ngo 1.17\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	testsDir := filepath.Join(root, "tests")
	if err := os.MkdirAll(testsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	slug := "my-feature"
	if err := os.WriteFile(filepath.Join(testsDir, slug+"_test.go"), []byte("package tests\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(testsDir, slug+".scan.baseline.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}

	if missing := missingTestArtifacts(root, slug); len(missing) != 0 {
		t.Errorf("Go project with real _test.go + scan.baseline.json should report no missing artifacts; got: %v", missing)
	}
	if !allTestArtifactsExist(root, slug) {
		t.Error("allTestArtifactsExist should be true for a Go project with its own artifacts present")
	}
}

// ── testStubLanguageProfile (J4) ───────────────────────────────────────────────

// TestTestStubLanguageProfile_MatchesDetectedLanguage — generateTestStubs'
// prompt must describe the project's actual language, not assume Go.
func TestTestStubLanguageProfile_MatchesDetectedLanguage(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		fw        TestFrameworkContext
		wantLabel string
		wantFence string
	}{
		{"go", TestFrameworkContext{Language: "go"}, "Go", "go"},
		{"python", TestFrameworkContext{Language: "python"}, "Python", "python"},
		{"java", TestFrameworkContext{Language: "java"}, "Java", "java"},
		{"typescript", TestFrameworkContext{Language: "typescript"}, "TypeScript", "typescript"},
		{"undetected-defaults-to-typescript", TestFrameworkContext{}, "TypeScript", "typescript"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, label, fence, hint := testStubLanguageProfile(tc.fw)
			if label != tc.wantLabel {
				t.Errorf("langLabel = %q, want %q", label, tc.wantLabel)
			}
			if fence != tc.wantFence {
				t.Errorf("codeFence = %q, want %q", fence, tc.wantFence)
			}
			if hint == "" {
				t.Error("tddHint should not be empty")
			}
		})
	}
}
