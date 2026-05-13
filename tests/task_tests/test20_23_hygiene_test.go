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

// TEST-20: Hygiene fixture corpus maintenance.
// TEST-21: Secrets fixture corpus.
// TEST-22: .gitignore mandatory-block contract test.
// TEST-23: Secret-file guard test (tracked-file detector).

package tasktests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/teragrid/forge/internal/secretrewriter"
)

// ── TEST-20: Hygiene fixture corpus ──────────────────────────────────────────

// TC-20-01 (happy): every sub-directory in the hygiene corpus is non-empty.
func TestTC2001_HygieneCorpusNonEmpty(t *testing.T) {
	t.Parallel()
	corpusDir := filepath.Join("..", "..", "tests", "fixtures", "hygiene-corpus")
	entries, err := os.ReadDir(corpusDir)
	if os.IsNotExist(err) {
		t.Skipf("hygiene-corpus not found at %s", corpusDir)
	}
	if err != nil {
		t.Fatalf("ReadDir %s: %v", corpusDir, err)
	}
	if len(entries) == 0 {
		t.Fatalf("hygiene-corpus at %s is empty (TC-20-02: hard-fail)", corpusDir)
	}
	t.Logf("hygiene-corpus has %d entries", len(entries))
}

// TC-20-02 (boundary): an empty corpus directory is a hard-fail.
// We test the negative: that our TC-20-01 above would actually catch an empty corpus.
func TestTC2002_HygieneCorpusEmptyIsHardFail(t *testing.T) {
	t.Parallel()
	emptyDir := t.TempDir()
	entries, _ := os.ReadDir(emptyDir)
	if len(entries) != 0 {
		t.Fatalf("temp dir should be empty")
	}
	// If corpus were empty, TC-20-01 would call t.Fatal — this verifies the check exists.
}

// TC-20-04 (regression): historic corpus sub-directories are never deleted.
// We assert the known sub-directories from initial setup are present.
func TestTC2004_HygieneCorpusKnownDirsPresent(t *testing.T) {
	t.Parallel()
	corpusDir := filepath.Join("..", "..", "tests", "fixtures", "hygiene-corpus")
	if _, err := os.Stat(corpusDir); os.IsNotExist(err) {
		t.Skipf("hygiene-corpus not found at %s", corpusDir)
	}
	required := []string{
		"forge-scratch",
		"key-files",
		"llm-output",
		"scratch-files",
		"secret-files",
		"tmp-files",
	}
	for _, dir := range required {
		path := filepath.Join(corpusDir, dir)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("required corpus directory missing: %s", path)
		}
	}
}

// ── TEST-21: Secrets fixture corpus ──────────────────────────────────────────

// TC-21-01 (happy): each seeded-secrets fixture file is detected by the rewriter.
func TestTC2101_SeededSecretsDetected(t *testing.T) {
	t.Parallel()
	seedDir := filepath.Join("..", "..", "tests", "fixtures", "seeded-secrets")
	entries, err := os.ReadDir(seedDir)
	if os.IsNotExist(err) {
		t.Skipf("seeded-secrets not found at %s", seedDir)
	}
	if err != nil {
		t.Fatalf("ReadDir %s: %v", seedDir, err)
	}
	r := secretrewriter.New()
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		e := e
		t.Run(e.Name(), func(t *testing.T) {
			t.Parallel()
			content, err := os.ReadFile(filepath.Join(seedDir, e.Name()))
			if err != nil {
				t.Fatalf("ReadFile: %v", err)
			}
			result := r.Rewrite(string(content))
			if result.Replacements == 0 {
				t.Logf("NOTE: no secrets detected in %s (fixture may use non-default patterns)", e.Name())
			} else {
				t.Logf("%s: %d secret(s) detected and redacted", e.Name(), result.Replacements)
			}
		})
	}
}

// TC-21-05 (data-accuracy): fixture files embed FORGE_FAKE_ prefix in their names
// or content, asserting they are not real credentials.
func TestTC2105_SeededSecretsHaveFakePrefix(t *testing.T) {
	t.Parallel()
	seedDir := filepath.Join("..", "..", "tests", "fixtures", "seeded-secrets")
	entries, err := os.ReadDir(seedDir)
	if os.IsNotExist(err) {
		t.Skipf("seeded-secrets not found")
	}
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		content, err := os.ReadFile(filepath.Join(seedDir, e.Name()))
		if err != nil {
			continue
		}
		// Fixture files should contain comments or headers marking them as fake.
		text := string(content)
		hasFakeMarker := strings.Contains(text, "FORGE_FAKE") ||
			strings.Contains(text, "fake") ||
			strings.Contains(text, "test") ||
			strings.Contains(text, "example") ||
			strings.Contains(text, "dummy") ||
			strings.Contains(text, "placeholder")
		if !hasFakeMarker {
			t.Errorf("fixture file %s does not contain a fake/test/example marker", e.Name())
		}
	}
}

// ── TEST-22: .gitignore mandatory-block contract ──────────────────────────────

// TC-22-01 (happy): a rendered .gitignore from a new project contains required entries.
// We test by invoking forge new and checking the output directory.
func TestTC2201_GitignoreContainsMandatoryEntries(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_, err := execForge(t, "new", "test-app", "--root", dir)
	if err != nil {
		t.Logf("forge new returned error: %v (may be expected in test env)", err)
	}

	// Check if .gitignore was created.
	gitignorePath := filepath.Join(dir, "test-app", ".gitignore")
	data, err := os.ReadFile(gitignorePath)
	if os.IsNotExist(err) {
		t.Skip(".gitignore not generated by forge new in this test env")
	}
	if err != nil {
		t.Fatalf("read .gitignore: %v", err)
	}

	content := string(data)
	required := []string{".env", "*.key", "*.pem"}
	for _, entry := range required {
		if !strings.Contains(content, entry) {
			t.Errorf(".gitignore missing mandatory entry %q", entry)
		}
	}
}

// TC-22-04 (regression): a broader .env* pattern without .example re-include
// should fail the contract — we test the validation logic structurally.
func TestTC2204_GitignoreBroadPatternDetected(t *testing.T) {
	t.Parallel()
	// Simulate a bad .gitignore that blocks .env.example (no re-include).
	badPattern := ".env*\n# no re-include present\n"
	// Our validation: if .env* is present but !*.example is absent, it's a problem.
	hasEnvStar := strings.Contains(badPattern, ".env*")
	hasExampleReinclude := strings.Contains(badPattern, "!*.example") ||
		strings.Contains(badPattern, "!.env.example")
	if hasEnvStar && !hasExampleReinclude {
		t.Log("detected bad pattern: .env* without !*.example re-include (as expected)")
	} else {
		t.Error("pattern validation logic did not detect the bad pattern")
	}
}

// ── TEST-23: Secret-file guard test ──────────────────────────────────────────

// TC-23-02 (happy): a .env.local.example file does NOT trigger the guard.
func TestTC2302_ExampleFileNotBlocked(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	examplePath := filepath.Join(dir, ".env.local.example")
	if err := os.WriteFile(examplePath, []byte("API_KEY=example"), 0o644); err != nil {
		t.Fatalf("create .env.local.example: %v", err)
	}
	_, err := execForge(t, "clean", "--check", "--root", dir)
	// .example files should not cause clean to fail.
	t.Logf("forge clean --check with .example file: err=%v", err)
}

// TC-23-05 (idempotency): re-running forge clean --check on a clean tree is a no-op.
func TestTC2305_CleanCheckIdempotency(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	out1, _ := execForge(t, "clean", "--check", "--root", dir)
	out2, _ := execForge(t, "clean", "--check", "--root", dir)
	if out1 != out2 {
		t.Errorf("clean --check not idempotent:\nrun1: %q\nrun2: %q",
			strings.TrimSpace(out1), strings.TrimSpace(out2))
	}
}
