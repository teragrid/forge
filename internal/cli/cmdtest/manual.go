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

// manual.go — `forge test manual`: AI-driven manual testing via Playwright.
//
// Role: Manual Test Expert
// ─────────────────────────
// This command embeds a "Manual Test Expert" persona that:
//  1. Reads the feature spec (spec.md / spec.yml in .forge/specs/<slug>/)
//  2. Uses the LLM to synthesise a Playwright test script covering every
//     acceptance criterion in Given/When/Then style.
//  3. Executes the script via `npx playwright test` against the target
//     environment (UAT, staging, or a custom URL).
//  4. Writes a structured Markdown report to
//     .forge/manual-tests/<slug>-<env>-<ts>.md
//
// Usage examples:
//
//	forge test manual --env staging --feature login
//	forge test manual --url https://uat.acme.com --feature checkout,cart
//	forge test manual --env uat --feature rate-limiter --headed
//	forge test manual --env staging --feature login --dry-run
//
// Environment URL resolution order:
//  1. --url flag (direct URL; takes absolute priority)
//  2. forge.yml → test.environments.<name>.url  (named env lookup)
//  3. Neither → FORGE-4306 error with setup hint
//
// Playwright detection:
//
//	npx is allowlisted through internal/procspawn. If `npx` is not on PATH
//	the command degrades to --dry-run and prints an install hint.
package cmdtest

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
	gopkgyaml "gopkg.in/yaml.v3"

	"github.com/teragrid/forge/internal/errcode"
	"github.com/teragrid/forge/internal/llmprovider"
	"github.com/teragrid/forge/internal/procspawn"
)

// ── Error codes (range 4306..4309) ────────────────────────────────────────────

var (
	// ErrManualTestEnvNotConfigured is returned when no target URL can be resolved.
	ErrManualTestEnvNotConfigured = errcode.Register(errcode.Code(4306),
		"manual test: target environment URL not configured")
	// ErrManualTestPlaywrightNotFound is returned when npx/playwright is not on PATH.
	ErrManualTestPlaywrightNotFound = errcode.Register(errcode.Code(4307),
		"manual test: playwright not found (run: npx playwright install)")
	// ErrManualTestExecutionFailed is returned when Playwright exits non-zero.
	ErrManualTestExecutionFailed = errcode.Register(errcode.Code(4308),
		"manual test: playwright execution failed")
	// ErrManualTestSpecNotFound is returned when no spec.md exists for a slug.
	ErrManualTestSpecNotFound = errcode.Register(errcode.Code(4309),
		"manual test: feature spec not found")
)

// ── Options & results ────────────────────────────────────────────────────────

// ManualTestOptions parameterises a `forge test manual` run.
type ManualTestOptions struct {
	// Features is a list of feature slugs to test. Each must have a
	// spec.md in .forge/specs/<slug>/spec.md.
	Features []string

	// EnvName is a named environment (e.g. "uat", "staging"). Its URL is
	// resolved from forge.yml → test.environments.<name>.url.
	EnvName string

	// URL is a direct target URL. Takes priority over EnvName.
	URL string

	// DryRun generates the Playwright script but does not execute it.
	// The script path and content are included in the result.
	DryRun bool

	// Headed runs Playwright with a visible browser window (--headed flag).
	Headed bool

	// Timeout is the per-test timeout. Defaults to 30 s.
	Timeout time.Duration

	// OutputPath is an optional custom report file path.
	// Default: .forge/manual-tests/<slug>-<env>-<ts>.md
	OutputPath string

	// Root is the project root. Defaults to cwd.
	Root string
}

// ManualTestResult is the structured output of a `forge test manual` run.
type ManualTestResult struct {
	DryRun     bool                `json:"dry_run"`
	EnvName    string              `json:"env_name"`
	EnvURL     string              `json:"env_url"`
	Features   []FeatureTestResult `json:"features"`
	Passed     int                 `json:"passed"`
	Failed     int                 `json:"failed"`
	Skipped    int                 `json:"skipped"`
	ReportPath string              `json:"report_path,omitempty"`
	Message    string              `json:"message"`
}

// FeatureTestResult is the per-feature portion of ManualTestResult.
type FeatureTestResult struct {
	Feature          string `json:"feature"`
	Status           string `json:"status"` // "ok", "fail", "skipped", "pending"
	TestCount        int    `json:"test_count"`
	PassedCount      int    `json:"passed_count"`
	FailedCount      int    `json:"failed_count"`
	Detail           string `json:"detail"`
	ScriptPath       string `json:"script_path,omitempty"`
	PlaywrightOutput string `json:"playwright_output,omitempty"`
	DurationMs       int64  `json:"duration_ms"`
}

// ── Core logic ────────────────────────────────────────────────────────────────

// RunManualTest is the testable core of `forge test manual`. No cobra I/O here.
func RunManualTest(opts ManualTestOptions) *ManualTestResult {
	res := &ManualTestResult{
		DryRun:  opts.DryRun,
		EnvName: opts.EnvName,
	}

	// Resolve target environment URL.
	envURL, err := resolveEnvURL(opts.Root, opts.EnvName, opts.URL)
	if err != nil {
		res.Message = err.Error()
		for _, f := range opts.Features {
			res.Features = append(res.Features, FeatureTestResult{
				Feature: f,
				Status:  "skipped",
				Detail:  "environment URL not resolved: " + err.Error(),
			})
			res.Skipped++
		}
		return res
	}
	res.EnvURL = envURL

	// Detect LLM provider (best-effort; fallback to template if unavailable).
	provider, _ := llmprovider.Detect()

	// Detect Playwright availability.
	pwReady := playwrightInstalled(opts.Root)

	perTestTimeout := opts.Timeout
	if perTestTimeout <= 0 {
		perTestTimeout = 30 * time.Second
	}

	// Per-feature loop.
	for _, slug := range opts.Features {
		fr := runFeatureManualTest(manualFeatureRunArgs{
			Root:            opts.Root,
			Slug:            slug,
			EnvName:         opts.EnvName,
			EnvURL:          envURL,
			DryRun:          opts.DryRun,
			Headed:          opts.Headed,
			Timeout:         perTestTimeout,
			Provider:        provider,
			PlaywrightReady: pwReady,
		})
		res.Features = append(res.Features, fr)
		switch fr.Status {
		case "ok":
			res.Passed++
		case "fail":
			res.Failed++
		default:
			res.Skipped++
		}
	}

	// Write consolidated report.
	res.ReportPath = writeManualTestReport(opts.Root, opts.OutputPath, opts.EnvName, envURL, res.Features)

	//nolint:gocritic // ifElseChain: conditions are heterogeneous, not switchable
	if res.Failed > 0 {
		res.Message = fmt.Sprintf("%d feature(s) failed manual testing — see report: %s", res.Failed, res.ReportPath)
	} else if res.Skipped > 0 && res.Passed == 0 {
		res.Message = fmt.Sprintf("%d feature(s) skipped — resolve errors above then rerun", res.Skipped)
	} else if opts.DryRun {
		res.Message = fmt.Sprintf("dry-run: %d feature script(s) generated — review then rerun without --dry-run", len(opts.Features))
	} else {
		res.Message = fmt.Sprintf("%d/%d feature(s) passed manual testing on %s", res.Passed, len(opts.Features), envURL)
	}
	return res
}

// manualFeatureRunArgs bundles per-feature run arguments (avoids wide parameter lists).
type manualFeatureRunArgs struct {
	Root            string
	Slug            string
	EnvName         string
	EnvURL          string
	DryRun          bool
	Headed          bool
	Timeout         time.Duration
	Provider        llmprovider.Provider
	PlaywrightReady bool
}

// runFeatureManualTest handles one feature slug: load spec → generate script → execute.
func runFeatureManualTest(a manualFeatureRunArgs) FeatureTestResult {
	t0 := time.Now()
	fr := FeatureTestResult{Feature: a.Slug}

	// Load spec.md.
	specPath := filepath.Join(a.Root, ".forge", "specs", a.Slug, "spec.md")
	specData, err := os.ReadFile(specPath)
	if err != nil {
		fr.Status = "skipped"
		fr.Detail = fmt.Sprintf("spec not found: %s (run: forge ship spec %q first)", specPath, a.Slug)
		fr.DurationMs = time.Since(t0).Milliseconds()
		return fr
	}

	// Generate Playwright script.
	script := generatePlaywrightScript(string(specData), a.Slug, a.EnvURL, a.Provider)

	// Write script to .forge/manual-tests/<slug>/.
	scriptDir := filepath.Join(a.Root, ".forge", "manual-tests", a.Slug)
	envLabel := a.EnvName
	if envLabel == "" {
		envLabel = "env"
	}
	scriptName := fmt.Sprintf("test-%s.spec.js", envLabel)
	scriptPath := filepath.Join(scriptDir, scriptName)
	if mkErr := os.MkdirAll(scriptDir, 0o755); mkErr == nil {
		_ = os.WriteFile(scriptPath, []byte(script), 0o600)
		fr.ScriptPath = scriptPath
	}

	if a.DryRun {
		fr.Status = "pending"
		fr.Detail = fmt.Sprintf("dry-run: Playwright script generated at %s — review and run without --dry-run", scriptPath)
		fr.DurationMs = time.Since(t0).Milliseconds()
		return fr
	}

	if !a.PlaywrightReady {
		fr.Status = "skipped"
		fr.Detail = fmt.Sprintf(
			"Playwright not found — script generated at %s; install with: npx playwright install",
			scriptPath,
		)
		fr.DurationMs = time.Since(t0).Milliseconds()
		return fr
	}

	// Execute Playwright.
	output, passed := runPlaywright(a.Root, scriptPath, a.Headed, a.Timeout)
	fr.PlaywrightOutput = output
	fr.DurationMs = time.Since(t0).Milliseconds()

	// Parse test count from Playwright output: "N passed" / "N failed".
	numPassed, numFailed := parsePlaywrightCounts(output)
	fr.TestCount = numPassed + numFailed
	fr.PassedCount = numPassed
	fr.FailedCount = numFailed

	if passed {
		fr.Status = "ok"
		fr.Detail = fmt.Sprintf("Playwright: %d/%d passed against %s", numPassed, fr.TestCount, a.EnvURL)
	} else {
		fr.Status = "fail"
		fr.Detail = fmt.Sprintf("Playwright: %d passed, %d failed against %s", numPassed, numFailed, a.EnvURL)
	}
	return fr
}

// ── Environment URL resolution ────────────────────────────────────────────────

// forgeYMLTestSection is used to parse the test.environments section of forge.yml.
type forgeYMLTestSection struct {
	Test struct {
		Environments map[string]struct {
			URL string `yaml:"url"`
		} `yaml:"environments"`
	} `yaml:"test"`
}

// resolveEnvURL returns the target URL in priority order:
//  1. directURL (--url flag) — used as-is.
//  2. forge.yml → test.environments.<envName>.url
//  3. Returns ErrManualTestEnvNotConfigured with a setup hint.
func resolveEnvURL(root, envName, directURL string) (string, error) {
	if directURL != "" {
		return directURL, nil
	}
	if envName == "" {
		return "", errcode.Newf(ErrManualTestEnvNotConfigured, nil,
			"provide --url <target> or --env <name> with the environment URL configured in forge.yml")
	}

	// Try forge.yml test.environments.<name>.url.
	forgePath := filepath.Join(root, "forge.yml")
	data, err := os.ReadFile(forgePath)
	if err == nil {
		var cfg forgeYMLTestSection
		if yamlErr := gopkgyaml.Unmarshal(data, &cfg); yamlErr == nil {
			if env, ok := cfg.Test.Environments[envName]; ok && env.URL != "" {
				return env.URL, nil
			}
		}
	}

	return "", errcode.Newf(ErrManualTestEnvNotConfigured, nil,
		"environment %q URL not found — add to forge.yml:\n\ntest:\n  environments:\n    %s:\n      url: https://your-%s.example.com",
		envName, envName, envName)
}

// ── Playwright script generation ──────────────────────────────────────────────

const manualTestExpertSystem = `You are a senior Manual Test Expert with 20+ years of experience testing web applications.
Your task is to translate feature acceptance criteria into a precise, self-contained Playwright test script.

Rules:
- Use @playwright/test (JavaScript).
- Export a single test.describe block named after the feature.
- Set the baseURL via the provided target URL in a test.use({ baseURL }) call.
- Each Given/When/Then acceptance criterion becomes one test() case.
- Also add: boundary tests (empty inputs, max-length), negative tests (invalid inputs, 401/403), and one smoke test (page loads, key element visible).
- Use realistic locators (data-testid preferred, then aria-label, then text).
- Set a 30 s timeout per test via test.setTimeout(30_000).
- Output ONLY the JavaScript code — no explanations, no markdown code fences.`

// generatePlaywrightScript calls the LLM to produce a Playwright spec.js for
// the given feature and target URL. Falls back to playwrightScriptTemplate
// when no LLM provider is available or the call fails.
func generatePlaywrightScript(specContent, slug, targetURL string, p llmprovider.Provider) string {
	if p == nil {
		return playwrightScriptTemplate(slug, specContent, targetURL)
	}

	userPrompt := fmt.Sprintf(
		"Feature slug: %s\nTarget URL: %s\n\nSpec:\n%s",
		slug, targetURL, specContent,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	resp, err := p.Complete(ctx, &llmprovider.Request{
		SystemPrompt: manualTestExpertSystem,
		UserPrompt:   userPrompt,
		MaxTokens:    2000,
		Capability:   "test:manual",
	})
	if err != nil || resp == nil || resp.Content == "" {
		return playwrightScriptTemplate(slug, specContent, targetURL)
	}

	// Strip accidental markdown fences the LLM might emit despite instructions.
	script := strings.TrimSpace(resp.Content)
	script = strings.TrimPrefix(script, "```javascript")
	script = strings.TrimPrefix(script, "```js")
	script = strings.TrimPrefix(script, "```")
	script = strings.TrimSuffix(script, "```")
	return strings.TrimSpace(script)
}

// playwrightScriptTemplate is the LLM-free fallback: a commented skeleton that
// the developer can fill in. It parses acceptance criteria lines from spec.md
// (lines starting with "- Given", "- When", "- Then", "- [ ]", "AC:").
func playwrightScriptTemplate(slug, specContent, targetURL string) string {
	var sb strings.Builder
	sb.WriteString("// AUTO-GENERATED by forge test manual (template mode — no LLM provider configured)\n")
	sb.WriteString("// Review and complete each test before running.\n\n")
	sb.WriteString("const { test, expect } = require('@playwright/test');\n\n")
	sb.WriteString(fmt.Sprintf("test.use({ baseURL: %q });\n\n", targetURL))
	sb.WriteString(fmt.Sprintf("test.setTimeout(30_000);\n\n"))
	sb.WriteString(fmt.Sprintf("test.describe(%q, () => {\n\n", slug))

	// Extract acceptance criteria hints from spec.
	criteria := extractAcceptanceCriteria(specContent)
	if len(criteria) == 0 {
		sb.WriteString("  test('smoke — page loads', async ({ page }) => {\n")
		sb.WriteString("    await page.goto('/');\n")
		sb.WriteString("    await expect(page).toHaveTitle(/.+/);\n")
		sb.WriteString("  });\n\n")
	} else {
		for i, criterion := range criteria {
			label := sanitizeTestLabel(criterion)
			sb.WriteString(fmt.Sprintf("  test('AC-%02d: %s', async ({ page }) => {\n", i+1, label))
			sb.WriteString("    // TODO: implement this acceptance criterion\n")
			sb.WriteString(fmt.Sprintf("    // Spec criterion: %s\n", criterion))
			sb.WriteString("    await page.goto('/');\n")
			sb.WriteString("    // await expect(...).toBeVisible();\n")
			sb.WriteString("  });\n\n")
		}
	}

	sb.WriteString("});\n")
	return sb.String()
}

// extractAcceptanceCriteria extracts lines from spec.md that look like AC items.
func extractAcceptanceCriteria(spec string) []string {
	var out []string
	for _, line := range strings.Split(spec, "\n") {
		t := strings.TrimSpace(line)
		if t == "" {
			continue
		}
		lower := strings.ToLower(t)
		if strings.HasPrefix(lower, "given ") ||
			strings.HasPrefix(lower, "- given ") ||
			strings.HasPrefix(lower, "when ") ||
			strings.HasPrefix(lower, "- when ") ||
			strings.HasPrefix(lower, "then ") ||
			strings.HasPrefix(lower, "- then ") ||
			strings.HasPrefix(lower, "ac:") ||
			strings.HasPrefix(lower, "- [ ]") ||
			strings.HasPrefix(lower, "- [x]") {
			out = append(out, t)
			if len(out) >= 20 { // cap to avoid runaway template
				break
			}
		}
	}
	return out
}

// sanitizeTestLabel trims Markdown list markers and caps at 80 chars.
func sanitizeTestLabel(s string) string {
	s = strings.TrimPrefix(s, "- [ ] ")
	s = strings.TrimPrefix(s, "- [x] ")
	s = strings.TrimPrefix(s, "- ")
	s = strings.TrimPrefix(s, "* ")
	// Remove internal quotes to avoid JS string escaping issues.
	s = strings.ReplaceAll(s, `"`, "'")
	if len(s) > 80 {
		s = s[:77] + "..."
	}
	return s
}

// ── Playwright execution ──────────────────────────────────────────────────────

// playwrightInstalled returns true when `npx` and (optionally) a local
// playwright binary can be found. We detect `npx` via exec.LookPath because
// it is the launch mechanism; the actual playwright binary may live in
// node_modules/.bin/ which npx resolves automatically.
func playwrightInstalled(root string) bool {
	npxBin := "npx"
	if runtime.GOOS == "windows" {
		npxBin = "npx.cmd"
	}
	sp := procspawn.New(npxBin, "npx", "node")
	// Quick probe: npx --version — timeout 5 s.
	res, err := sp.Run(npxBin, []string{"--version"}, procspawn.Options{
		Dir:     root,
		Timeout: 5 * time.Second,
	})
	return err == nil && res != nil && res.ExitCode == 0
}

// runPlaywright executes `npx playwright test <scriptPath>` via procspawn.
// Returns the combined stdout+stderr output and whether all tests passed.
func runPlaywright(root, scriptPath string, headed bool, timeout time.Duration) (string, bool) {
	npxBin := "npx"
	if runtime.GOOS == "windows" {
		npxBin = "npx.cmd"
	}
	sp := procspawn.New(npxBin, "npx")

	args := []string{"playwright", "test", scriptPath, "--reporter=line"}
	if headed {
		args = append(args, "--headed")
	}

	res, err := sp.Run(npxBin, args, procspawn.Options{
		Dir:     root,
		Timeout: timeout + 60*time.Second, // add 60 s for Playwright startup
	})
	if err != nil || res == nil {
		detail := "playwright execution error"
		if err != nil {
			detail = err.Error()
		}
		return detail, false
	}

	output := res.Stdout
	if res.Stderr != "" {
		output += "\nSTDERR:\n" + res.Stderr
	}
	return output, res.ExitCode == 0
}

// parsePlaywrightCounts extracts "N passed" and "N failed" from Playwright line output.
func parsePlaywrightCounts(output string) (passed, failed int) {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		var n int
		if _, err := fmt.Sscanf(line, "%d passed", &n); err == nil && n > 0 {
			passed = n
		}
		if _, err := fmt.Sscanf(line, "%d failed", &n); err == nil && n > 0 {
			failed = n
		}
		// Also handle "N passed (Xs)" format.
		if strings.Contains(line, "passed") && strings.Contains(line, "(") {
			fmt.Sscanf(strings.Split(line, "passed")[0], "%d", &passed) //nolint:errcheck
		}
	}
	return passed, failed
}

// ── Report writer ─────────────────────────────────────────────────────────────

// writeManualTestReport writes a Markdown report to .forge/manual-tests/ and
// returns the file path, or "" if the write fails.
func writeManualTestReport(root, customPath, envName, envURL string, features []FeatureTestResult) string {
	if len(features) == 0 {
		return ""
	}

	ts := time.Now().UTC().Format("20060102-150405")
	envLabel := envName
	if envLabel == "" {
		envLabel = "custom"
	}

	// Build default output path from first feature slug (or "mixed").
	if customPath == "" {
		slug := "mixed"
		if len(features) == 1 {
			slug = features[0].Feature
		}
		reportDir := filepath.Join(root, ".forge", "manual-tests")
		if err := os.MkdirAll(reportDir, 0o755); err != nil {
			return ""
		}
		customPath = filepath.Join(reportDir, fmt.Sprintf("%s-%s-%s.md", slug, envLabel, ts))
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Manual Test Report\n\n"))
	sb.WriteString(fmt.Sprintf("**Environment:** %s (%s)\n", envLabel, envURL))
	sb.WriteString(fmt.Sprintf("**Date:** %s\n", time.Now().UTC().Format("2006-01-02 15:04:05 UTC")))
	sb.WriteString(fmt.Sprintf("**Generated by:** `forge test manual`\n\n---\n\n"))

	// Summary table.
	passed, failed, skipped := 0, 0, 0
	for _, f := range features {
		switch f.Status {
		case "ok":
			passed++
		case "fail":
			failed++
		default:
			skipped++
		}
	}
	sb.WriteString("## Summary\n\n")
	sb.WriteString(fmt.Sprintf("| Total | Passed | Failed | Skipped |\n"))
	sb.WriteString(fmt.Sprintf("|-------|--------|--------|---------|\n"))
	sb.WriteString(fmt.Sprintf("| %d | %d | %d | %d |\n\n", len(features), passed, failed, skipped))

	// Per-feature sections.
	sb.WriteString("## Results\n\n")
	for _, f := range features {
		statusIcon := "○"
		switch f.Status {
		case "ok":
			statusIcon = "✓"
		case "fail":
			statusIcon = "✗"
		case "pending":
			statusIcon = "⏳"
		}
		sb.WriteString(fmt.Sprintf("### %s %s\n\n", statusIcon, f.Feature))
		sb.WriteString(fmt.Sprintf("**Status:** %s\n", f.Status))
		sb.WriteString(fmt.Sprintf("**Detail:** %s\n", f.Detail))
		if f.TestCount > 0 {
			sb.WriteString(fmt.Sprintf("**Tests:** %d passed / %d total\n", f.PassedCount, f.TestCount))
		}
		if f.ScriptPath != "" {
			sb.WriteString(fmt.Sprintf("**Script:** `%s`\n", f.ScriptPath))
		}
		if f.PlaywrightOutput != "" {
			sb.WriteString("\n<details><summary>Playwright output</summary>\n\n```\n")
			sb.WriteString(f.PlaywrightOutput)
			sb.WriteString("\n```\n\n</details>\n")
		}
		sb.WriteString("\n")
	}

	if err := os.WriteFile(customPath, []byte(sb.String()), 0o600); err != nil {
		return ""
	}
	return customPath
}

// ── Cobra command ─────────────────────────────────────────────────────────────

// newManualCmd returns the `forge test manual` cobra subcommand.
func newManualCmd() *cobra.Command {
	var (
		envName    string
		targetURL  string
		features   string // comma-separated feature slugs
		dryRun     bool
		headed     bool
		timeoutSec int
		outputPath string
		root       string
		asJSON     bool
	)

	cmd := &cobra.Command{
		Use:   "manual [--env <name> | --url <url>] --feature <slug>[,slug2,...]",
		Short: "AI-driven manual testing via Playwright against UAT, staging, or any URL.",
		Long: strings.TrimSpace(`
forge test manual embeds a Manual Test Expert persona that:

  1. Reads each feature spec from .forge/specs/<slug>/spec.md
  2. Uses the LLM to generate a Playwright test script covering every
     acceptance criterion (Given/When/Then format)
  3. Executes the script against the target environment
  4. Writes a structured Markdown report

Environment setup (add to forge.yml):

  test:
    environments:
      staging:
        url: https://staging.example.com
      uat:
        url: https://uat.example.com

Examples:

  forge test manual --env staging --feature login
  forge test manual --url https://uat.acme.com --feature checkout,cart
  forge test manual --env uat --feature rate-limiter --headed
  forge test manual --env staging --feature login --dry-run
`),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			r := root
			if r == "" {
				var err error
				r, err = os.Getwd()
				if err != nil {
					return err
				}
			}

			slugs := parseSlugs(features)
			if len(slugs) == 0 {
				return fmt.Errorf("--feature is required; provide one or more feature slugs (e.g. --feature login,checkout)")
			}

			opts := ManualTestOptions{
				Features:   slugs,
				EnvName:    envName,
				URL:        targetURL,
				DryRun:     dryRun,
				Headed:     headed,
				Timeout:    time.Duration(timeoutSec) * time.Second,
				OutputPath: outputPath,
				Root:       r,
			}

			res := RunManualTest(opts)

			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				if encErr := enc.Encode(res); encErr != nil {
					return encErr
				}
			} else {
				renderManualTestResult(cmd, res)
			}

			if res.Failed > 0 {
				return errcode.New(ErrManualTestExecutionFailed,
					fmt.Sprintf("%d feature(s) failed; see report: %s", res.Failed, res.ReportPath), nil)
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&envName, "env", "e", "", "named environment (e.g. uat, staging) — URL looked up from forge.yml")
	cmd.Flags().StringVar(&targetURL, "url", "", "direct target URL (overrides --env)")
	cmd.Flags().StringVarP(&features, "feature", "f", "", "comma-separated feature slugs to test (required)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "generate Playwright script without executing it")
	cmd.Flags().BoolVar(&headed, "headed", false, "run Playwright with a visible browser window")
	cmd.Flags().IntVar(&timeoutSec, "timeout", 30, "per-test timeout in seconds")
	cmd.Flags().StringVarP(&outputPath, "output", "o", "", "custom report output path (default: .forge/manual-tests/)")
	cmd.Flags().StringVarP(&root, "root", "r", "", "project root (default: cwd)")
	cmd.Flags().BoolVarP(&asJSON, "json", "j", false, "emit machine-readable JSON")

	_ = cmd.MarkFlagRequired("feature")

	return cmd
}

// parseSlugs splits a comma-separated slug string and trims whitespace.
func parseSlugs(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// renderManualTestResult writes a human-readable summary to cmd.OutOrStdout().
func renderManualTestResult(cmd *cobra.Command, res *ManualTestResult) {
	out := cmd.OutOrStdout()
	env := res.EnvName
	if env == "" {
		env = res.EnvURL
	}
	fmt.Fprintf(out, "\nforge test manual — %s (%s)\n\n", env, res.EnvURL)
	for _, f := range res.Features {
		icon := "○"
		switch f.Status {
		case "ok":
			icon = "✓"
		case "fail":
			icon = "✗"
		case "pending":
			icon = "⏳"
		}
		fmt.Fprintf(out, "  %s %-30s %s\n", icon, f.Feature, f.Detail)
		if f.ScriptPath != "" && (f.Status == "pending" || f.Status == "skipped") {
			fmt.Fprintf(out, "       script: %s\n", f.ScriptPath)
		}
	}
	fmt.Fprintf(out, "\n%s\n", res.Message)
	if res.ReportPath != "" {
		fmt.Fprintf(out, "  report: %s\n", res.ReportPath)
	}
	fmt.Fprintln(out)
}
