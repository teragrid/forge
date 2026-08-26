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

// Package cmdship — G-006: four named test artifacts produced at the Test checkpoint.
//
// The four artifacts written to tests/<slug>/ are:
//
//  1. <slug>.test.ts        — unit test stubs (TypeScript; intentionally failing)
//  2. <slug>.integration.test.ts — integration test stubs
//  3. <slug>.rls.test.ts    — Row-Level Security test stubs
//  4. <slug>.scan.baseline.json  — scan baseline (empty allowlist)
//
// Tests are intentionally failing ("red") so that the Code checkpoint can verify
// TDD compliance. If all stubs pass at generation time the Test checkpoint aborts
// with ErrTestsGreenAtStubStage (FORGE-3201).
package cmdship

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/teragrid/forge/internal/errcode"
)

// ErrTestsGreenAtStubStage is returned when generated test stubs already pass.
// This signals a TDD violation: tests must be RED at stub creation time.
var ErrTestsGreenAtStubStage = errcode.Register(errcode.Code(3202),
	"test stubs already passing — tests must be failing (red) at stub stage (TDD requirement)")

// RFC-005 §6 error codes.
var (
	// ErrFrameworkDetectFailed is returned when test framework detection fails.
	ErrFrameworkDetectFailed = errcode.Register(errcode.Code(3210),
		"test framework detection failed")
	// ErrTraceabilityWriteFailed is returned when traceability.yaml cannot be written.
	ErrTraceabilityWriteFailed = errcode.Register(errcode.Code(3211),
		"traceability.yaml write failed")
	// ErrTestGateBLOCK is returned when the test phase gate rejects the pipeline.
	ErrTestGateBLOCK = errcode.Register(errcode.Code(3212),
		"test phase gate: BLOCK — composite score below threshold")
)

// TestArtifactPaths holds the paths of the test artifacts for a slug.
// Language-specific fields are set based on the detected stack (RFC-005 §6).
// TypeScript fields (UnitTest, IntegrationTest, RLSTest) are populated only
// for TypeScript/JavaScript projects. Go/Python/Java fields are populated only
// for their respective languages. ScanBaseline is always populated.
type TestArtifactPaths struct {
	// TypeScript / JavaScript (original G-006)
	UnitTest        string // tests/<slug>.test.ts
	IntegrationTest string // tests/<slug>.integration.test.ts
	RLSTest         string // tests/<slug>.rls.test.ts (Supabase only)
	// Go
	GoTest     string // tests/<slug>_test.go
	GoFuzzTest string // tests/<slug>_fuzz_test.go (go 1.18+)
	// Python
	PyTest string // tests/test_<slug>.py
	// Java
	JavaTest string // tests/<slug>Test.java
	// Language-agnostic
	ScanBaseline string // tests/<slug>.scan.baseline.json
	Traceability string // .forge/specs/<slug>/traceability.yaml (set by WriteTraceability caller)
}

// writeTestArtifacts is the original G-006 entry point, now a shim that calls
// writeTestArtifactsWithContext using the auto-detected test framework. This
// preserves backward-compatibility while fixing G10 (hardcoded TypeScript).
func writeTestArtifacts(root, slug, feature, specMarkdown string, pipe *LLMPipe) (TestArtifactPaths, error) {
	fw := detectTestFramework(root)
	return writeTestArtifactsWithContext(root, slug, feature, specMarkdown, fw, pipe)
}

// expectedTestArtifactNames returns the artifact filenames this project's
// detected stack should produce, mirroring the exact switch in
// writeTestArtifactsWithContext (J5, fix-checkpoint-llm-quality-and-observability):
// Go/Python/Java projects get their own convention, never the TypeScript/
// Supabase-RLS filenames from an unrelated stack. ScanBaseline is
// language-agnostic and always expected.
func expectedTestArtifactNames(root, slug string) []string {
	fw := detectTestFramework(root)
	switch fw.Language {
	case "go":
		names := []string{slug + "_test.go"}
		if fw.FuzzSupport {
			names = append(names, slug+"_fuzz_test.go")
		}
		return append(names, slug+".scan.baseline.json")
	case "python":
		return []string{"test_" + slug + ".py", slug + ".scan.baseline.json"}
	case "java":
		return []string{slug + "Test.java", slug + ".scan.baseline.json"}
	default:
		// TypeScript / unknown — original G-006 convention.
		return []string{
			slug + ".test.ts",
			slug + ".integration.test.ts",
			slug + ".rls.test.ts",
			slug + ".scan.baseline.json",
		}
	}
}

// allTestArtifactsExist returns true when all of this project's expected
// artifacts (per its detected stack, J5) are present.
func allTestArtifactsExist(root, slug string) bool { //nolint:unused // used in ship dry-run gate
	testsDir := filepath.Join(root, "tests")
	for _, name := range expectedTestArtifactNames(root, slug) {
		if _, err := os.Stat(filepath.Join(testsDir, name)); err != nil {
			return false
		}
	}
	return true
}

// missingTestArtifacts lists the expected artifact filenames (per this
// project's detected stack, J5) that are absent.
func missingTestArtifacts(root, slug string) []string {
	testsDir := filepath.Join(root, "tests")
	var missing []string
	for _, name := range expectedTestArtifactNames(root, slug) {
		if _, err := os.Stat(filepath.Join(testsDir, name)); err != nil {
			missing = append(missing, name)
		}
	}
	return missing
}

// ── Static stub generators ────────────────────────────────────────────────────

func unitTestStub(slug, feature string) string {
	return fmt.Sprintf(`// AUTO-GENERATED by forge ship test — G-006 unit test stubs
// Feature: %s
// Status: RED (intentionally failing — complete implementation in forge ship code)
//
// Edit this file to add real assertions once the feature is implemented.

import { describe, it, expect, beforeAll, afterAll } from '@jest/globals';

describe('%s — unit', () => {
  it('should be implemented (placeholder — will fail until feature is complete)', () => {
    // TODO: replace with real assertion after forge ship code
    expect(false).toBe(true); // intentionally failing
  });

  it('given valid input, when processed, then expected output returned', () => {
    // TODO: implement
    expect(false).toBe(true);
  });

  it('given invalid input, when processed, then validation error returned', () => {
    // TODO: implement
    expect(false).toBe(true);
  });
});
`, feature, slug)
}

func integrationTestStub(slug, feature string) string {
	return fmt.Sprintf(`// AUTO-GENERATED by forge ship test — G-006 integration test stubs
// Feature: %s
// Status: RED (intentionally failing)

import { describe, it, expect, beforeAll, afterAll } from '@jest/globals';
import request from 'supertest';
import { app } from '../src/app';

describe('%s — integration', () => {
  let server: ReturnType<typeof app.listen>;

  beforeAll(() => { server = app.listen(0); });
  afterAll(() => server.close());

  it('POST /api/%s — happy path returns 200', async () => {
    // TODO: replace with real test after forge ship code
    const res = await request(server).post('/api/%s').send({});
    expect(res.status).toBe(200); // will fail until implementation is complete
  });

  it('POST /api/%s — unauthenticated returns 401', async () => {
    const res = await request(server).post('/api/%s').send({});
    expect(res.status).toBe(401);
  });

  it('POST /api/%s — invalid payload returns 422', async () => {
    const res = await request(server).post('/api/%s').send({ invalid: true });
    expect(res.status).toBe(422);
  });
});
`, feature, slug, slug, slug, slug, slug, slug, slug)
}

func rlsTestStub(slug, feature string) string {
	return fmt.Sprintf(`// AUTO-GENERATED by forge ship test — G-006 RLS test stubs
// Feature: %s
// Status: RED (intentionally failing)
//
// These tests verify Row-Level Security policies for the feature.
// Run against a local Supabase instance: supabase start

import { describe, it, expect, beforeAll, afterAll } from '@jest/globals';
import { createClient } from '@supabase/supabase-js';

const SUPABASE_URL = process.env.SUPABASE_URL ?? 'http://localhost:54321';
const ANON_KEY    = process.env.SUPABASE_ANON_KEY ?? '';
const SERVICE_KEY = process.env.SUPABASE_SERVICE_KEY ?? '';

describe('%s — RLS policies', () => {
  const anon    = createClient(SUPABASE_URL, ANON_KEY);
  const service = createClient(SUPABASE_URL, SERVICE_KEY);

  it('anon role cannot read %s rows', async () => {
    // TODO: replace table name + assertion after schema migration
    const { data, error } = await anon.from('%s').select('*');
    expect(error).not.toBeNull(); // RLS should block anon reads
  });

  it('service role can read all %s rows', async () => {
    const { error } = await service.from('%s').select('*');
    expect(error).toBeNull();
  });

  it('tenant-A cannot read tenant-B rows', async () => {
    // TODO: implement with two tenant JWT fixtures
    expect(false).toBe(true); // placeholder
  });
});
`, feature, slug, slug, slug, slug, slug)
}

// ScanBaseline is the G-006 scan baseline JSON — an empty allowlist that
// downstream scan runs diff against to detect regressions.
type ScanBaseline struct {
	SchemaVersion int              `json:"schema_version"`
	Feature       string           `json:"feature"`
	Slug          string           `json:"slug"`
	Families      []string         `json:"families"`
	Allowlist     []ScanAllowEntry `json:"allowlist"`
	GeneratedAt   string           `json:"generated_at"`
}

// ScanAllowEntry represents a single allowlisted finding.
type ScanAllowEntry struct {
	Rule     string `json:"rule"`
	File     string `json:"file"`
	Reason   string `json:"reason"`
	ReviewBy string `json:"review_by"` // RFC-3339 date; G-069 expiry gate
}

func scanBaseline(slug, feature string) ScanBaseline {
	return ScanBaseline{
		SchemaVersion: 1,
		Feature:       feature,
		Slug:          slug,
		Families:      []string{"secrets", "sca", "sast", "license", "iac", "container", "api", "supply-chain"},
		Allowlist:     []ScanAllowEntry{},
		GeneratedAt:   "",
	}
}

// ── RFC-005 §6: framework-aware test generation (G10 fix) ─────────────────────

// bugFixSignals are substrings in a feature description that indicate a bug-fix.
// When any signal is present, D7 (Regression Guard) is mandatory at score ≥8.
// RFC-005 §6.6.
var bugFixSignals = []string{
	"fix", "bug", "issue", "regression", "broken", "crash",
	"nil pointer", "panic", "incorrect", "wrong output",
	"not working", "doesn't work", "race condition",
}

// IsBugFix reports whether the feature description contains a bug-fix signal.
func IsBugFix(feature string) bool {
	lower := strings.ToLower(feature)
	for _, sig := range bugFixSignals {
		if strings.Contains(lower, sig) {
			return true
		}
	}
	return false
}

// CheckTestFilesResult holds the result of a file-presence check.
type CheckTestFilesResult struct {
	AllPresent bool
	Missing    []string
}

// CheckTestFilesExist checks whether each file in the list exists on disk.
func CheckTestFilesExist(files []string) CheckTestFilesResult {
	var missing []string
	for _, f := range files {
		if _, err := os.Stat(f); err != nil {
			missing = append(missing, f)
		}
	}
	return CheckTestFilesResult{
		AllPresent: len(missing) == 0,
		Missing:    missing,
	}
}

// writeTestArtifactsWithContext generates framework-appropriate test stubs.
// It dispatches to language-specific generators based on fw.Language, writing
// only the correct stub type for the detected stack (G10 fix: no cross-language
// injection). An LLMPipe may be nil — static stubs are always written as a
// fallback. RFC-005 §6.3.
//
// Returns a non-nil error (satisfying IsAgentTurn) when a bridge-backed pipe
// paused on a host-agent turn mid-generation. In that case NO files are
// written for the branch that paused: the pre-fix behaviour wrote the static
// RED placeholder to disk regardless of *why* generation didn't produce
// content, which let a pending turn look identical to "there was nothing to
// enrich". That placeholder then satisfied allTestArtifactsExist forever,
// silently discarding the host agent's real answer once it was later
// submitted — confirmed live: submitting a real `ping()` assertion for
// ship:test:unit never reached tests/*.test.ts, because the artifact-exists
// guard in checkTest had already been tripped by the placeholder from the
// paused attempt. Returning the error instead lets the caller keep the
// "not yet written" state so the next run's Lookup call replays the real
// answer into the actual file.
func writeTestArtifactsWithContext(root, slug, feature, specMD string, fw TestFrameworkContext, pipe *LLMPipe) (TestArtifactPaths, error) {
	testsDir := filepath.Join(root, "tests")
	if err := os.MkdirAll(testsDir, 0o755); err != nil {
		return TestArtifactPaths{}, nil
	}

	isFix := IsBugFix(feature)
	paths := TestArtifactPaths{}

	switch fw.Language {
	case "go":
		paths.GoTest = filepath.Join(testsDir, slug+"_test.go")
		content := goTestStub(slug, feature, isFix)
		if pipe != nil {
			gen, err := llmGoStub(pipe, slug, feature, specMD, isFix)
			if IsAgentTurn(err) {
				return TestArtifactPaths{}, err
			}
			if gen != "" {
				content = gen
			}
		}
		_ = os.WriteFile(paths.GoTest, []byte(content), 0o600)

		if fw.FuzzSupport {
			paths.GoFuzzTest = filepath.Join(testsDir, slug+"_fuzz_test.go")
			_ = os.WriteFile(paths.GoFuzzTest, []byte(goFuzzTestStub(slug, feature)), 0o600)
		}

	case "python":
		paths.PyTest = filepath.Join(testsDir, "test_"+slug+".py")
		content := pyTestStub(slug, feature, isFix)
		_ = os.WriteFile(paths.PyTest, []byte(content), 0o600)

	case "java":
		paths.JavaTest = filepath.Join(testsDir, slug+"Test.java")
		content := javaTestStub(slug, feature, isFix)
		_ = os.WriteFile(paths.JavaTest, []byte(content), 0o600)

	default:
		// TypeScript / unknown — fall through to the original G-006 generators.
		paths.UnitTest = filepath.Join(testsDir, slug+".test.ts")
		paths.IntegrationTest = filepath.Join(testsDir, slug+".integration.test.ts")
		paths.RLSTest = filepath.Join(testsDir, slug+".rls.test.ts")

		unitContent := unitTestStub(slug, feature)
		integContent := integrationTestStub(slug, feature)
		rlsContent := rlsTestStub(slug, feature)

		if pipe != nil {
			ctx := ""
			if specMD != "" {
				ctx = "Feature spec:\n" + specMD + "\n\n"
			}
			// Framework name always comes from the already-detected fw context,
			// never a hardcoded literal — this project's `fw.TestRunner` is
			// "jest", but a Vitest project must not get Jest-flavored stubs
			// (or vice versa). Previously only the unit/integration prompts
			// named a framework at all (hardcoded "Jest"/"Jest + supertest"),
			// and the RLS prompt named none — which is exactly how a real run
			// on this Jest project got a Vitest-flavored `rls.test.ts` back
			// (root-caused 2026-07-24: LLM defaults to Vitest imports absent
			// an explicit instruction).
			runner := fw.TestRunner
			if runner == "" {
				runner = "Jest"
			}
			runnerTitle := strings.ToUpper(runner[:1]) + runner[1:]

			unitFn := func() (string, bool, error) {
				return pipe.InvokeChecked("ship:test:unit", "",
					"You are a senior TypeScript QA engineer writing failing unit test stubs for TDD. "+
						"Tests MUST compile but MUST fail at runtime. Use "+runnerTitle+" — import test "+
						"functions (describe/it/expect/etc.) from \""+runner+"\", never a different test framework.",
					ctx+"Generate failing unit test stubs for feature: "+feature,
					// 6000: 2000 reliably truncated a real unit test suite — same
					// undersized-budget root cause as ship:spec:generate/ship:arch:generate
					// (see ship.go). Wrapped in generateWithValidation below so a
					// truncated response is caught rather than written as-is.
					6000)
			}
			gen, complete, err := generateWithValidation(unitFn)
			if IsAgentTurn(err) {
				return TestArtifactPaths{}, err
			}
			if err == nil && complete && gen != "" {
				unitContent = gen
			}

			integFn := func() (string, bool, error) {
				return pipe.InvokeChecked("ship:test:integration", "",
					"You are writing failing integration test stubs ("+runnerTitle+" + supertest) for TDD. "+
						"Tests MUST fail. Import test functions from \""+runner+"\", never a different test framework.",
					ctx+"Generate failing integration test stubs for feature: "+feature, 6000)
			}
			gen, complete, err = generateWithValidation(integFn)
			if IsAgentTurn(err) {
				return TestArtifactPaths{}, err
			}
			if err == nil && complete && gen != "" {
				integContent = gen
			}

			rlsFn := func() (string, bool, error) {
				return pipe.InvokeChecked("ship:test:rls", "",
					"You are writing Row-Level Security test stubs for Supabase, using "+runnerTitle+" as the "+
						"test runner — import test functions (describe/it/expect/beforeAll/etc.) from \""+runner+
						"\", never a different test framework. Tests MUST fail.",
					ctx+"Generate failing RLS test stubs for feature: "+feature, 4000)
			}
			gen, complete, err = generateWithValidation(rlsFn)
			if IsAgentTurn(err) {
				return TestArtifactPaths{}, err
			}
			if err == nil && complete && gen != "" {
				rlsContent = gen
			}
		}

		_ = os.WriteFile(paths.UnitTest, []byte(unitContent), 0o600)
		_ = os.WriteFile(paths.IntegrationTest, []byte(integContent), 0o600)
		_ = os.WriteFile(paths.RLSTest, []byte(rlsContent), 0o600)
	}

	// ScanBaseline is language-agnostic — always written.
	paths.ScanBaseline = filepath.Join(testsDir, slug+".scan.baseline.json")
	baseline := scanBaseline(slug, feature)
	if data, err := json.MarshalIndent(baseline, "", "  "); err == nil {
		_ = os.WriteFile(paths.ScanBaseline, data, 0o600)
	}

	return paths, nil
}

// ── Go stub generators ─────────────────────────────────────────────────────────

func goTestStub(_ string, feature string, isBugFix bool) string {
	title := featureTitle(feature)
	regression := ""
	if isBugFix {
		regression = fmt.Sprintf(`
// Regression: %s — must fail on pre-fix code, pass on post-fix code.
func Test%sRegression_FailsPreFix(t *testing.T) {
	t.Skip("TODO: fill in reproduction of original bug")
}

// Regression: %s — idempotency guard.
func Test%sRegression_Idempotent(t *testing.T) {
	t.Skip("TODO: call fixed function twice, assert same result")
}
`, feature, title, feature, title)
	}
	return fmt.Sprintf(`// AUTO-GENERATED by forge ship test — RFC-005 §6 Go test stubs
// Feature: %s
// Status: RED (intentionally incomplete — fill in at forge ship code)

package cmdship

import "testing"

func Test%s_HappyPath(t *testing.T) {
	t.Skip("TODO: implement happy-path test")
}

func Test%s_ErrorPath(t *testing.T) {
	t.Skip("TODO: implement error-path test")
}
%s`, feature, title, title, regression)
}

func goFuzzTestStub(_ string, feature string) string {
	title := featureTitle(feature)
	return fmt.Sprintf(`// AUTO-GENERATED by forge ship test — RFC-005 §6 Go fuzz test stubs
// Feature: %s
// Status: RED

package cmdship

import "testing"

func Fuzz%s(f *testing.F) {
	f.Add("seed input") // TODO: add representative seed corpus
	f.Fuzz(func(t *testing.T, in string) {
		_ = in // TODO: call function under test; must not panic
	})
}
`, feature, title)
}

// ── Python stub generator ───────────────────────────────────────────────────

func pyTestStub(slug, feature string, isBugFix bool) string {
	title := featureTitle(feature)
	regression := ""
	if isBugFix {
		regression = fmt.Sprintf(`
    def test_%s_regression_fails_pre_fix(self):
        """Regression: must fail on pre-fix code, pass on post-fix."""
        pytest.skip("TODO: reproduce original bug")

    def test_%s_regression_idempotent(self):
        """Regression: calling function twice must yield same result."""
        pytest.skip("TODO: implement idempotency assertion")
`, slug, slug)
	}
	return fmt.Sprintf(`# AUTO-GENERATED by forge ship test — RFC-005 §6 Python test stubs
# Feature: %s
# Status: RED (intentionally incomplete)

import pytest


class Test%s:
    def test_happy_path(self):
        """Happy path: TODO implement."""
        pytest.skip("TODO: implement happy-path test")

    def test_error_path(self):
        """Error path: TODO implement."""
        pytest.skip("TODO: implement error-path test")
%s`, feature, title, regression)
}

// ── Java stub generator ─────────────────────────────────────────────────────

func javaTestStub(_ string, feature string, isBugFix bool) string {
	title := featureTitle(feature)
	regression := ""
	if isBugFix {
		regression = fmt.Sprintf(`
    @Test
    @Disabled("TODO: regression — reproduce original bug, must fail on pre-fix code")
    void test%sRegression_failsPreFix() {}

    @Test
    @Disabled("TODO: regression — idempotency guard")
    void test%sRegression_idempotent() {}
`, title, title)
	}
	return fmt.Sprintf(`// AUTO-GENERATED by forge ship test — RFC-005 §6 Java test stubs
// Feature: %s
// Status: RED (intentionally incomplete)

import org.junit.jupiter.api.Disabled;
import org.junit.jupiter.api.Test;

class %sTest {

    @Test
    @Disabled("TODO: implement happy-path test")
    void testHappyPath() {}

    @Test
    @Disabled("TODO: implement error-path test")
    void testErrorPath() {}
%s}
`, feature, title, regression)
}

// ── LLM generator for Go (optional enrichment) ─────────────────────────────

func llmGoStub(pipe *LLMPipe, _ string, feature, specMD string, isBugFix bool) (string, error) {
	ctx := ""
	if specMD != "" {
		ctx = "Feature spec:\n" + specMD + "\n\n"
	}
	bugFixHint := ""
	if isBugFix {
		bugFixHint = " Include two //Regression: labeled test stubs that FAIL on pre-fix code and PASS after the fix."
	}
	genFn := func() (string, bool, error) {
		return pipe.InvokeChecked("ship:test:go", "",
			"You are a senior Go QA engineer writing failing test stubs for TDD. "+
				"Tests MUST compile but must call t.Skip(). Use standard library testing only."+bugFixHint,
			ctx+"Generate failing Go test stubs for feature: "+feature,
			// 6000: was 2000 — same undersized-budget root cause as the
			// TypeScript stub generators above and ship:spec:generate.
			6000)
	}
	gen, complete, err := generateWithValidation(genFn)
	if IsAgentTurn(err) {
		return "", err
	}
	if err != nil || !complete || gen == "" {
		return "", nil
	}
	return gen, nil
}

// featureTitle converts a feature string to a CamelCase identifier suitable
// for Go/Python/Java test function names.
func featureTitle(feature string) string {
	if feature == "" {
		return "Feature"
	}
	words := strings.Fields(strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == ' ' {
			return r
		}
		return ' '
	}, feature))
	if len(words) == 0 {
		return "Feature"
	}
	var sb strings.Builder
	for _, w := range words {
		if len(w) == 0 {
			continue
		}
		sb.WriteByte(w[0] &^ 32) // uppercase first byte
		if len(w) > 1 {
			sb.WriteString(strings.ToLower(w[1:]))
		}
	}
	result := sb.String()
	if result == "" {
		return "Feature"
	}
	return result
}
