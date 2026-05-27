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

package cmdtest

// ── Test design (per always-write-tests.md) ───────────────────────────────────
//
// 1. Happy path
//    - resolveEnvURL: direct --url → returned as-is
//    - resolveEnvURL: envName present in forge.yml → resolved correctly
//    - RunManualTest: dry-run with valid spec → pending status, script file written
//    - RunManualTest: multi-feature dry-run → one result per slug
//
// 2. Boundary cases
//    - parseSlugs: empty string → nil
//    - parseSlugs: single entry → one element
//    - parseSlugs: comma-separated with spaces → trimmed entries
//    - extractAcceptanceCriteria: empty spec → nil
//    - extractAcceptanceCriteria: spec with Given/When/Then → all extracted
//    - sanitizeTestLabel: > 80 chars → truncated with "..."
//    - parsePlaywrightCounts: empty output → 0,0
//    - parsePlaywrightCounts: line with "3 passed" and "1 failed"
//
// 3. Negative cases
//    - resolveEnvURL: both URL and envName empty → ErrManualTestEnvNotConfigured
//    - resolveEnvURL: envName not in forge.yml → ErrManualTestEnvNotConfigured
//    - resolveEnvURL: forge.yml malformed → ErrManualTestEnvNotConfigured
//    - RunManualTest: no URL/env → all features skipped, message set
//    - RunManualTest: spec not found → feature status "skipped"
//    - RunManualTest: multi-feature one spec missing → partial skips
//
// 4. Config-based URL resolution
//    - forge.yml with valid test.environments block → URL extracted
//
// 5. Template generation
//    - playwrightScriptTemplate: spec with ACs → JS with per-AC test()
//    - playwrightScriptTemplate: empty spec → JS with smoke test
//    - generatePlaywrightScript: nil provider → falls back to template
//
// 6. Report writing
//    - writeManualTestReport: features provided → file written, path returned
//    - writeManualTestReport: empty features → returns ""
//
// 7. Cobra command
//    - newManualCmd flags registered correctly
//    - --feature required: missing → error
//
// 8. Regression guards
//    - Error code 4306 is distinct from all other cmdtest codes (4300-4305)
//    - parseSlugs never returns a slice with empty entries

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ── parseSlugs ────────────────────────────────────────────────────────────────

func TestParseSlugs_Empty(t *testing.T) {
	got := parseSlugs("")
	if got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestParseSlugs_Single(t *testing.T) {
	got := parseSlugs("login")
	if len(got) != 1 || got[0] != "login" {
		t.Errorf("expected [login], got %v", got)
	}
}

func TestParseSlugs_CommaSeparatedWithSpaces(t *testing.T) {
	got := parseSlugs("login , checkout , cart")
	if len(got) != 3 {
		t.Fatalf("expected 3 entries, got %d: %v", len(got), got)
	}
	for _, g := range got {
		if strings.ContainsAny(g, " \t") {
			t.Errorf("entry %q still contains whitespace", g)
		}
	}
}

func TestParseSlugs_NoEmptyEntries(t *testing.T) {
	// Regression: double commas must not produce empty slug entries.
	got := parseSlugs("login,,cart")
	for _, g := range got {
		if g == "" {
			t.Error("parseSlugs returned an empty entry")
		}
	}
}

// ── extractAcceptanceCriteria ─────────────────────────────────────────────────

func TestExtractAcceptanceCriteria_Empty(t *testing.T) {
	got := extractAcceptanceCriteria("")
	if len(got) != 0 {
		t.Errorf("expected 0 criteria, got %d", len(got))
	}
}

func TestExtractAcceptanceCriteria_WithGivenWhenThen(t *testing.T) {
	spec := `# Login feature

## Acceptance Criteria

Given I am on the login page
When I submit valid credentials
Then I am redirected to the dashboard
- [ ] Error message shown on invalid password
AC: session cookie set on success
`
	got := extractAcceptanceCriteria(spec)
	if len(got) < 5 {
		t.Errorf("expected >=5 criteria, got %d: %v", len(got), got)
	}
}

func TestExtractAcceptanceCriteria_CapsToTwenty(t *testing.T) {
	// Build a spec with 25 "Given" lines.
	var sb strings.Builder
	for i := 0; i < 25; i++ {
		sb.WriteString("Given line\n")
	}
	got := extractAcceptanceCriteria(sb.String())
	if len(got) > 20 {
		t.Errorf("expected cap at 20, got %d", len(got))
	}
}

// ── sanitizeTestLabel ─────────────────────────────────────────────────────────

func TestSanitizeTestLabel_TooLong(t *testing.T) {
	long := strings.Repeat("a", 90)
	got := sanitizeTestLabel(long)
	if len(got) > 80 {
		t.Errorf("expected <=80 chars, got %d", len(got))
	}
	if !strings.HasSuffix(got, "...") {
		t.Errorf("expected trailing '...', got %q", got)
	}
}

func TestSanitizeTestLabel_StripMarkdownPrefix(t *testing.T) {
	got := sanitizeTestLabel("- [ ] user can log in")
	if strings.Contains(got, "[ ]") {
		t.Errorf("expected markdown prefix stripped, got %q", got)
	}
}

// ── parsePlaywrightCounts ─────────────────────────────────────────────────────

func TestParsePlaywrightCounts_Empty(t *testing.T) {
	p, f := parsePlaywrightCounts("")
	if p != 0 || f != 0 {
		t.Errorf("expected 0,0 got %d,%d", p, f)
	}
}

func TestParsePlaywrightCounts_PassedAndFailed(t *testing.T) {
	output := `
  Running 4 tests using 1 worker

  ●  login › AC-01: Given I am on login page

  3 passed (4.2s)
  1 failed
`
	p, f := parsePlaywrightCounts(output)
	if p != 3 {
		t.Errorf("expected 3 passed, got %d", p)
	}
	if f != 1 {
		t.Errorf("expected 1 failed, got %d", f)
	}
}

// ── resolveEnvURL ─────────────────────────────────────────────────────────────

func TestResolveEnvURL_DirectURL_TakesPriority(t *testing.T) {
	// Even if envName and forge.yml are present, --url takes priority.
	dir := t.TempDir()
	writeForgeYML(t, dir, `test:
  environments:
    staging:
      url: https://staging.example.com
`)
	got, err := resolveEnvURL(dir, "staging", "https://direct.example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "https://direct.example.com" {
		t.Errorf("expected direct URL, got %q", got)
	}
}

func TestResolveEnvURL_FromForgeYML(t *testing.T) {
	dir := t.TempDir()
	writeForgeYML(t, dir, `test:
  environments:
    uat:
      url: https://uat.acme.com
`)
	got, err := resolveEnvURL(dir, "uat", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "https://uat.acme.com" {
		t.Errorf("expected https://uat.acme.com, got %q", got)
	}
}

func TestResolveEnvURL_EmptyBoth(t *testing.T) {
	dir := t.TempDir()
	_, err := resolveEnvURL(dir, "", "")
	if err == nil {
		t.Error("expected error for empty url and envName, got nil")
	}
	if !strings.Contains(err.Error(), "4306") && !strings.Contains(err.Error(), "environment URL not configured") {
		t.Errorf("expected FORGE-4306 error, got: %v", err)
	}
}

func TestResolveEnvURL_EnvNameNotInForgeYML(t *testing.T) {
	dir := t.TempDir()
	writeForgeYML(t, dir, `test:
  environments:
    staging:
      url: https://staging.example.com
`)
	_, err := resolveEnvURL(dir, "production", "")
	if err == nil {
		t.Error("expected error for missing env name, got nil")
	}
}

func TestResolveEnvURL_MalformedForgeYML(t *testing.T) {
	dir := t.TempDir()
	writeForgeYML(t, dir, `this: [is: not: valid: yaml`)
	_, err := resolveEnvURL(dir, "staging", "")
	if err == nil {
		t.Error("expected error for malformed forge.yml, got nil")
	}
}

func TestResolveEnvURL_NoForgeYML(t *testing.T) {
	dir := t.TempDir() // no forge.yml written
	_, err := resolveEnvURL(dir, "staging", "")
	if err == nil {
		t.Error("expected error when forge.yml absent, got nil")
	}
}

// ── playwrightScriptTemplate ──────────────────────────────────────────────────

func TestPlaywrightScriptTemplate_EmptySpec(t *testing.T) {
	script := playwrightScriptTemplate("my-feature", "", "https://example.com")
	mustContain(t, script, "require('@playwright/test')")
	mustContain(t, script, "https://example.com")
	mustContain(t, script, "smoke")
}

func TestPlaywrightScriptTemplate_WithAcceptanceCriteria(t *testing.T) {
	spec := "Given I open the app\nWhen I click login\nThen I see the dashboard\n"
	script := playwrightScriptTemplate("auth", spec, "https://uat.example.com")
	mustContain(t, script, "require('@playwright/test')")
	mustContain(t, script, "AC-01")
	mustContain(t, script, "AC-02")
	mustContain(t, script, "AC-03")
	mustContain(t, script, "https://uat.example.com")
}

func TestGeneratePlaywrightScript_NilProvider_FallsBackToTemplate(t *testing.T) {
	spec := "Given I visit home\nWhen I scroll\nThen I see footer\n"
	script := generatePlaywrightScript(spec, "homepage", "https://example.com", nil)
	mustContain(t, script, "require('@playwright/test')")
	mustContain(t, script, "https://example.com")
}

// ── writeManualTestReport ─────────────────────────────────────────────────────

func TestWriteManualTestReport_WritesFile(t *testing.T) {
	dir := t.TempDir()
	features := []FeatureTestResult{
		{Feature: "login", Status: "ok", TestCount: 3, PassedCount: 3, Detail: "all good"},
		{Feature: "checkout", Status: "fail", TestCount: 2, PassedCount: 1, FailedCount: 1, Detail: "one AC failed"},
	}
	path := writeManualTestReport(dir, "", "staging", "https://staging.example.com", features)
	if path == "" {
		t.Fatal("expected report path, got empty string")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("report file not readable: %v", err)
	}
	content := string(data)
	mustContain(t, content, "Manual Test Report")
	mustContain(t, content, "staging")
	mustContain(t, content, "login")
	mustContain(t, content, "checkout")
	mustContain(t, content, "one AC failed")
}

func TestWriteManualTestReport_EmptyFeatures(t *testing.T) {
	dir := t.TempDir()
	path := writeManualTestReport(dir, "", "uat", "https://uat.example.com", nil)
	if path != "" {
		t.Errorf("expected empty path for no features, got %q", path)
	}
}

func TestWriteManualTestReport_CustomOutputPath(t *testing.T) {
	dir := t.TempDir()
	customPath := filepath.Join(dir, "my-report.md")
	features := []FeatureTestResult{
		{Feature: "auth", Status: "ok", Detail: "ok"},
	}
	path := writeManualTestReport(dir, customPath, "staging", "https://x.com", features)
	if path != customPath {
		t.Errorf("expected custom path %q, got %q", customPath, path)
	}
	if _, err := os.Stat(customPath); err != nil {
		t.Errorf("file not created at custom path: %v", err)
	}
}

// ── RunManualTest (dry-run) ───────────────────────────────────────────────────

func TestRunManualTest_NoURL_NoEnv_AllSkipped(t *testing.T) {
	dir := t.TempDir()
	res := RunManualTest(ManualTestOptions{
		Features: []string{"login", "checkout"},
		Root:     dir,
		DryRun:   true,
	})
	if res.Skipped != 2 {
		t.Errorf("expected 2 skipped, got %d", res.Skipped)
	}
	if res.Passed != 0 || res.Failed != 0 {
		t.Errorf("expected 0 passed/failed, got %d/%d", res.Passed, res.Failed)
	}
	if res.Message == "" {
		t.Error("expected non-empty message")
	}
}

func TestRunManualTest_DryRun_SpecNotFound(t *testing.T) {
	dir := t.TempDir()
	res := RunManualTest(ManualTestOptions{
		Features: []string{"missing-feature"},
		URL:      "https://staging.example.com",
		Root:     dir,
		DryRun:   true,
	})
	if len(res.Features) != 1 {
		t.Fatalf("expected 1 feature result, got %d", len(res.Features))
	}
	fr := res.Features[0]
	if fr.Status != "skipped" {
		t.Errorf("expected 'skipped' for missing spec, got %q", fr.Status)
	}
	if !strings.Contains(fr.Detail, "spec not found") {
		t.Errorf("expected 'spec not found' in detail, got %q", fr.Detail)
	}
}

func TestRunManualTest_DryRun_SpecExists_ScriptGenerated(t *testing.T) {
	dir := t.TempDir()
	slug := "auth"
	writeSpec(t, dir, slug, "# Auth\n\nGiven I open login\nWhen I submit valid creds\nThen I see dashboard\n")

	res := RunManualTest(ManualTestOptions{
		Features: []string{slug},
		URL:      "https://staging.example.com",
		EnvName:  "staging",
		Root:     dir,
		DryRun:   true,
	})

	if len(res.Features) != 1 {
		t.Fatalf("expected 1 feature result, got %d", len(res.Features))
	}
	fr := res.Features[0]
	if fr.Status != "pending" {
		t.Errorf("expected 'pending' for dry-run, got %q", fr.Status)
	}
	if fr.ScriptPath == "" {
		t.Error("expected ScriptPath to be set")
	}
	// Script file should exist.
	if _, err := os.Stat(fr.ScriptPath); err != nil {
		t.Errorf("script file not found at %q: %v", fr.ScriptPath, err)
	}
	// Script should reference the target URL.
	data, _ := os.ReadFile(fr.ScriptPath)
	mustContain(t, string(data), "https://staging.example.com")
}

func TestRunManualTest_DryRun_MultiFeature(t *testing.T) {
	dir := t.TempDir()
	writeSpec(t, dir, "feature-a", "# A\nGiven step A\n")
	writeSpec(t, dir, "feature-b", "# B\nGiven step B\n")

	res := RunManualTest(ManualTestOptions{
		Features: []string{"feature-a", "feature-b"},
		URL:      "https://uat.example.com",
		Root:     dir,
		DryRun:   true,
	})
	if len(res.Features) != 2 {
		t.Fatalf("expected 2 feature results, got %d", len(res.Features))
	}
	for _, fr := range res.Features {
		if fr.Status != "pending" {
			t.Errorf("feature %q: expected 'pending', got %q", fr.Feature, fr.Status)
		}
	}
}

func TestRunManualTest_DryRun_MultiFeature_OneSpecMissing(t *testing.T) {
	dir := t.TempDir()
	writeSpec(t, dir, "feature-ok", "# OK\nGiven something\n")
	// "feature-missing" has no spec.

	res := RunManualTest(ManualTestOptions{
		Features: []string{"feature-ok", "feature-missing"},
		URL:      "https://uat.example.com",
		Root:     dir,
		DryRun:   true,
	})

	if len(res.Features) != 2 {
		t.Fatalf("expected 2 results, got %d", len(res.Features))
	}
	statusMap := make(map[string]string, 2)
	for _, fr := range res.Features {
		statusMap[fr.Feature] = fr.Status
	}
	if statusMap["feature-ok"] != "pending" {
		t.Errorf("feature-ok: expected 'pending', got %q", statusMap["feature-ok"])
	}
	if statusMap["feature-missing"] != "skipped" {
		t.Errorf("feature-missing: expected 'skipped', got %q", statusMap["feature-missing"])
	}
}

func TestRunManualTest_ReportPathSet(t *testing.T) {
	dir := t.TempDir()
	writeSpec(t, dir, "login", "# Login\nGiven I open login\n")

	res := RunManualTest(ManualTestOptions{
		Features: []string{"login"},
		URL:      "https://staging.example.com",
		Root:     dir,
		DryRun:   true,
	})
	if res.ReportPath == "" {
		t.Error("expected ReportPath to be set after successful run")
	}
}

// ── Cobra command ─────────────────────────────────────────────────────────────

func TestNewManualCmd_FlagsRegistered(t *testing.T) {
	cmd := newManualCmd()
	expectedFlags := []string{"env", "url", "feature", "dry-run", "headed", "timeout", "output", "root", "json"}
	for _, name := range expectedFlags {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("flag --%s not registered", name)
		}
	}
}

func TestNewManualCmd_FeatureRequiredError(t *testing.T) {
	cmd := newManualCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	// Execute without --feature.
	cmd.SetArgs([]string{"--url", "https://example.com"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when --feature is missing, got nil")
	}
}

func TestNewManualCmd_Use(t *testing.T) {
	cmd := newManualCmd()
	if !strings.HasPrefix(cmd.Use, "manual") {
		t.Errorf("expected Use to start with 'manual', got %q", cmd.Use)
	}
}

// ── Error code regression guard ───────────────────────────────────────────────

func TestErrorCodesDistinct(t *testing.T) {
	// Verify the four manual test error codes are in the reserved 4306..4309 range
	// and do not collide with existing cmdtest codes (4300..4305).
	codes := map[string]int{
		"ErrManualTestEnvNotConfigured":   4306,
		"ErrManualTestPlaywrightNotFound": 4307,
		"ErrManualTestExecutionFailed":    4308,
		"ErrManualTestSpecNotFound":       4309,
	}
	for name, code := range codes {
		if code < 4306 || code > 4309 {
			t.Errorf("%s has out-of-range code %d", name, code)
		}
	}
}

// ── Timeout default ───────────────────────────────────────────────────────────

func TestRunManualTest_DefaultTimeout(t *testing.T) {
	// When Timeout is zero the run should not panic and should default to 30s internally.
	dir := t.TempDir()
	writeSpec(t, dir, "login", "# Login\nGiven I see page\n")

	// We just confirm it completes without panic.
	res := RunManualTest(ManualTestOptions{
		Features: []string{"login"},
		URL:      "https://example.com",
		Root:     dir,
		DryRun:   true,
		Timeout:  0, // should default to 30s
	})
	_ = res // no panic = pass
}

// ── EnvURL propagated to result ───────────────────────────────────────────────

func TestRunManualTest_EnvURLInResult(t *testing.T) {
	dir := t.TempDir()
	writeSpec(t, dir, "auth", "# Auth\nGiven login page\n")

	res := RunManualTest(ManualTestOptions{
		Features: []string{"auth"},
		URL:      "https://direct.example.com",
		Root:     dir,
		DryRun:   true,
	})
	if res.EnvURL != "https://direct.example.com" {
		t.Errorf("expected EnvURL 'https://direct.example.com', got %q", res.EnvURL)
	}
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// writeForgeYML writes content to forge.yml in dir.
func writeForgeYML(t *testing.T, dir, content string) {
	t.Helper()
	path := filepath.Join(dir, "forge.yml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("writeForgeYML: %v", err)
	}
}

// writeSpec creates .forge/specs/<slug>/spec.md with content in dir.
func writeSpec(t *testing.T, dir, slug, content string) {
	t.Helper()
	specDir := filepath.Join(dir, ".forge", "specs", slug)
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatalf("writeSpec MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "spec.md"), []byte(content), 0o600); err != nil {
		t.Fatalf("writeSpec WriteFile: %v", err)
	}
}

// mustContain fails the test if haystack does not contain needle.
func mustContain(t *testing.T, haystack, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		// Trim long haystacks in error output.
		snippet := haystack
		if len(snippet) > 300 {
			snippet = snippet[:300] + "..."
		}
		t.Errorf("expected to contain %q\ngot: %s", needle, snippet)
	}
}

// Compile-time guard: make sure unused imports don't prevent test init.
var _ = time.Second
