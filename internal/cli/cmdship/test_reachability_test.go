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

// Test-design checklist (always-write-tests.md 9-point):
//  1. Happy path           — a normally-configured project reports no orphans.
//  2. Boundary             — no config at all; empty file list; package.json
//     with an inline "jest" key.
//  3. Negative             — the real-world dead zone: ignored by the default
//     config, unmatched by the integration config.
//  4. Idempotency          — repeated calls give the same verdict.
//  5. Concurrency          — every case owns its TempDir.
//  6. Cross-cutting        — a MethodNone report is never OK(), so "unknown"
//     can never be rendered as "fine".
//  7. Regression           — this is the whole file: forge generated
//     tests/<slug>.integration.test.ts into a path no
//     runner collected, and the checkpoint went green.
//  8. Data-accuracy        — glob translation handles the ** / * distinction
//     rather than approximating it.
//  9. False-positive guard — a correctly-wired integration test must NOT be
//     reported as unreachable; a checker that cried
//     wolf would be turned off and the gap would return.
package cmdship

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func mustCompile(t *testing.T, expr string) *regexp.Regexp {
	t.Helper()
	re, err := regexp.Compile(expr)
	if err != nil {
		t.Fatalf("compile %q: %v", expr, err)
	}
	return re
}

func writeProjectFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// splitJestProject reproduces the configuration this bug was found in: a
// default config that excludes anything with `.integration.` in the name, and
// a second config that only collects files under an `integration/` directory.
func splitJestProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeProjectFile(t, filepath.Join(root, "package.json"), `{"name":"app","devDependencies":{"jest":"^29"}}`)
	writeProjectFile(t, filepath.Join(root, "jest.config.js"), `
module.exports = {
  preset: 'ts-jest',
  testPathIgnorePatterns: ['/node_modules/', '\\.(integration|e2e)\\.'],
};
`)
	writeProjectFile(t, filepath.Join(root, "jest.integration.config.js"), `
module.exports = {
  preset: 'ts-jest',
  testMatch: ['<rootDir>/tests/**/integration/*.test.ts'],
};
`)
	return root
}

// ── Regression: the dead zone ─────────────────────────────────────────────────

func TestVerifyTestsReachable_FlagsTheIntegrationDeadZone(t *testing.T) {
	t.Parallel()
	root := splitJestProject(t)

	// Exactly what `forge ship test` writes today.
	orphan := filepath.Join(root, "tests", "cold-outreach.integration.test.ts")
	writeProjectFile(t, orphan, "describe('x', () => {});")

	rep := verifyTestsReachable(root, []string{orphan})
	if rep.Method != MethodStatic {
		t.Fatalf("expected the config scan to decide this, got method %q (note: %s)", rep.Method, rep.Note)
	}
	if len(rep.Orphans) != 1 || !strings.Contains(rep.Orphans[0], "cold-outreach.integration.test.ts") {
		t.Fatalf("the unreachable file was not flagged: %+v", rep)
	}
	if rep.OK() {
		t.Fatal("a report with orphans must not be OK")
	}
	if !strings.Contains(rep.Summary(), "UNREACHABLE TESTS") {
		t.Fatalf("summary must name the problem plainly: %s", rep.Summary())
	}
}

// ── False-positive guard ──────────────────────────────────────────────────────

func TestVerifyTestsReachable_CorrectlyWiredIntegrationTestIsNotFlagged(t *testing.T) {
	t.Parallel()
	root := splitJestProject(t)

	// The same project, with the file where the integration config expects it.
	ok := filepath.Join(root, "tests", "outreach", "integration", "flow.test.ts")
	writeProjectFile(t, ok, "describe('x', () => {});")

	rep := verifyTestsReachable(root, []string{ok})
	if len(rep.Orphans) != 0 {
		t.Fatalf("a properly-wired test must not be reported unreachable: %+v", rep)
	}
	if !rep.OK() {
		t.Fatalf("expected an OK report, got %+v", rep)
	}
}

func TestVerifyTestsReachable_PlainUnitTestIsReachable(t *testing.T) {
	t.Parallel()
	root := splitJestProject(t)
	unit := filepath.Join(root, "tests", "cold-outreach.test.ts")
	writeProjectFile(t, unit, "describe('x', () => {});")

	rep := verifyTestsReachable(root, []string{unit})
	if len(rep.Orphans) != 0 {
		t.Fatalf("a unit test the default config collects must not be flagged: %+v", rep)
	}
}

// ── Boundary ──────────────────────────────────────────────────────────────────

func TestVerifyTestsReachable_NoConfigIsReportedUnknownNotGreen(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	f := filepath.Join(root, "tests", "a.test.ts")
	writeProjectFile(t, f, "x")

	rep := verifyTestsReachable(root, []string{f})
	if rep.Method != MethodNone {
		t.Fatalf("with no config there is nothing to decide on, got %q", rep.Method)
	}
	// The point of the whole file: absence of evidence must not read as green.
	if rep.OK() {
		t.Fatal("an undetermined report must never be OK()")
	}
	if !strings.Contains(rep.Summary(), "unverified") {
		t.Fatalf("summary must say it could not verify: %s", rep.Summary())
	}
}

func TestVerifyTestsReachable_NonJSFilesAreSkipped(t *testing.T) {
	t.Parallel()
	root := splitJestProject(t)
	goFile := filepath.Join(root, "tests", "thing_test.go")
	writeProjectFile(t, goFile, "package tests")

	rep := verifyTestsReachable(root, []string{goFile})
	if rep.Checked != 0 {
		t.Fatalf("go/pytest collect by convention, not config — nothing to check, got %d", rep.Checked)
	}
	if len(rep.Orphans) != 0 {
		t.Fatalf("a Go test must never be reported as an unreachable JS test: %+v", rep)
	}
}

func TestVerifyTestsReachable_PackageJSONInlineJestConfig(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeProjectFile(t, filepath.Join(root, "package.json"),
		`{"name":"app","jest":{"testPathIgnorePatterns":["\\.integration\\."]}}`)
	f := filepath.Join(root, "tests", "x.integration.test.ts")
	writeProjectFile(t, f, "x")

	rep := verifyTestsReachable(root, []string{f})
	if rep.Method != MethodStatic {
		t.Fatalf("an inline package.json jest block must be read, got %q", rep.Method)
	}
	if len(rep.Orphans) != 1 {
		t.Fatalf("the ignored file should be flagged: %+v", rep)
	}
}

// ── Idempotency ───────────────────────────────────────────────────────────────

func TestVerifyTestsReachable_IsStableAcrossCalls(t *testing.T) {
	t.Parallel()
	root := splitJestProject(t)
	f := filepath.Join(root, "tests", "a.integration.test.ts")
	writeProjectFile(t, f, "x")

	first := verifyTestsReachable(root, []string{f})
	second := verifyTestsReachable(root, []string{f})
	if first.Method != second.Method || len(first.Orphans) != len(second.Orphans) {
		t.Fatalf("verdict changed between identical calls: %+v vs %+v", first, second)
	}
}

// ── Data accuracy: glob translation ───────────────────────────────────────────

func TestGlobToRegexp_DistinguishesGlobstarFromStar(t *testing.T) {
	t.Parallel()
	cases := []struct {
		glob, path string
		want       bool
	}{
		// ** crosses directory boundaries; * must not. Getting this backwards
		// would make the checker report false orphans on every nested test,
		// which is the fastest way to get a safety check switched off.
		{"tests/**/integration/*.test.ts", "tests/a/b/integration/x.test.ts", true},
		{"tests/**/integration/*.test.ts", "tests/integration/x.test.ts", true},
		{"tests/*/x.test.ts", "tests/a/b/x.test.ts", false},
		{"tests/*.test.ts", "tests/x.test.ts", true},
		{"tests/*.test.ts", "tests/sub/x.test.ts", false},
		// Brace alternation is used by nearly every real testMatch.
		{"**/*.{test,spec}.{ts,tsx}", "src/a.spec.tsx", true},
		{"**/*.{test,spec}.{ts,tsx}", "src/a.stories.tsx", false},
		// Dots are literal, not "any character".
		{"tests/a.test.ts", "tests/aXtest.ts", false},
	}
	for _, c := range cases {
		re := mustCompile(t, globToRegexp(c.glob))
		if got := re.MatchString(c.path); got != c.want {
			t.Errorf("glob %q vs %q: got %v want %v (regexp %s)",
				c.glob, c.path, got, c.want, re.String())
		}
	}
}

func TestStripRootDir(t *testing.T) {
	t.Parallel()
	if got := stripRootDir("<rootDir>/tests/**/*.ts"); got != "tests/**/*.ts" {
		t.Fatalf("got %q", got)
	}
}

// ── Checkpoint wiring ─────────────────────────────────────────────────────────

func TestApplyReachability_DowngradesGreenToWarning(t *testing.T) {
	t.Parallel()
	root := splitJestProject(t)
	orphan := filepath.Join(root, "tests", "x.integration.test.ts")
	writeProjectFile(t, orphan, "x")

	cp := Checkpoint{Name: "Test", Status: "ok", Detail: "1 test file(s) found"}
	applyReachability(root, []string{orphan}, &cp)

	if cp.Status != "warning" {
		t.Fatalf("a checkpoint whose tests never run must not stay green, got %q", cp.Status)
	}
	if !strings.Contains(cp.Detail, "UNREACHABLE") {
		t.Fatalf("the detail must surface the problem: %s", cp.Detail)
	}
}

func TestApplyReachability_StaysQuietWithNothingToCheck(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	cp := Checkpoint{Name: "Test", Status: "ok", Detail: "original"}
	applyReachability(root, nil, &cp)
	if cp.Status != "ok" || cp.Detail != "original" {
		t.Fatalf("with nothing to check the checkpoint must be untouched: %+v", cp)
	}
}

func TestApplyReachability_NeverFailsThePipeline(t *testing.T) {
	t.Parallel()
	root := splitJestProject(t)
	orphan := filepath.Join(root, "tests", "x.integration.test.ts")
	writeProjectFile(t, orphan, "x")

	cp := Checkpoint{Name: "Test", Status: "ok"}
	applyReachability(root, []string{orphan}, &cp)
	// Advisory-first, per the 1.7.12 precedent: a project whose config forge
	// cannot parse must not be unable to ship.
	if cp.Status == "fail" {
		t.Fatal("reachability is advisory; it must never fail the checkpoint")
	}
}

// ── Regression: double-escaped regexes in config source ───────────────────────

// These fields hold regexes, so at the source level they are double-escaped:
// `['\\.(integration|e2e)\\.']` is the JS spelling of `\.(integration|e2e)\.`.
// Compiling the raw source text yields `\\.` — "a literal backslash then any
// character" — which matches nothing in a real path. Every ignore rule then
// looks inert, nothing appears excluded, and the dead zone is reported as
// reachable: a false green produced by the very check written to prevent false
// greens. Worth its own test because it fails silently, and in the
// safe-looking direction.
func TestUnescapeStringLiteral_ResolvesSourceLevelEscaping(t *testing.T) {
	t.Parallel()
	cases := []struct{ in, want string }{
		{`\\.(integration|e2e)\\.`, `\.(integration|e2e)\.`},
		{`/node_modules/`, `/node_modules/`},
		{`<rootDir>\/tests`, `<rootDir>/tests`},
		// A single-backslash regex escape must survive untouched — eating its
		// backslash would corrupt the pattern as badly as leaving it doubled.
		{`\d+\.test\.ts`, `\d+\.test\.ts`},
		{`no escapes here`, `no escapes here`},
	}
	for _, c := range cases {
		if got := unescapeStringLiteral(c.in); got != c.want {
			t.Errorf("unescape(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestParseJSTestConfigs_IgnorePatternCompilesToAWorkingRegex(t *testing.T) {
	t.Parallel()
	root := splitJestProject(t)
	matchers, parsed := parseJSTestConfigs(root, findJSTestConfigs(root))
	if parsed == 0 {
		t.Fatal("expected at least one config to yield usable rules")
	}
	var checked bool
	for _, m := range matchers {
		if m.name != "jest.config.js" {
			continue
		}
		checked = true
		if len(m.ignore) == 0 {
			t.Fatal("jest.config.js ignore patterns were not extracted")
		}
		// The end-to-end property: the compiled rule must actually exclude a
		// real path, not merely exist.
		if m.collects("tests/x.integration.test.ts") {
			t.Error("the ignore pattern compiled to something that matches nothing")
		}
		if !m.collects("tests/x.test.ts") {
			t.Error("the ignore pattern over-matched and excluded a plain unit test")
		}
	}
	if !checked {
		t.Fatal("jest.config.js was not among the parsed matchers")
	}
}

// ── Config discovery ──────────────────────────────────────────────────────────

func TestFindJSTestConfigs_PicksUpSplitConfigs(t *testing.T) {
	t.Parallel()
	root := splitJestProject(t)
	got := findJSTestConfigs(root)
	joined := strings.Join(got, ",")
	for _, want := range []string{"jest.config.js", "jest.integration.config.js"} {
		if !strings.Contains(joined, want) {
			t.Errorf("config discovery missed %s (got %v)", want, got)
		}
	}
}

func TestIsRunnerConfigName(t *testing.T) {
	t.Parallel()
	yes := []string{"jest.config.js", "jest.config.ts", "jest.integration.config.js",
		"vitest.config.mts", "jest.config.json"}
	no := []string{"package.json", "tsconfig.json", "jest.setup.js", "webpack.config.js", "README.md"}
	for _, n := range yes {
		if !isRunnerConfigName(n) {
			t.Errorf("%s should be recognised as a runner config", n)
		}
	}
	for _, n := range no {
		if isRunnerConfigName(n) {
			t.Errorf("%s must NOT be treated as a runner config", n)
		}
	}
}
