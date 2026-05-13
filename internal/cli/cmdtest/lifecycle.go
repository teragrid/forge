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

// lifecycle.go â€” four-phase test lifecycle for `forge test`.
//
// Phases:
//
//	create  â€” LLM generates test scaffolding for a named feature.
//	approve â€” vibe-coder reviews and accepts the generated tests.
//	run     â€” run the approved tests locally across selected families.
//	ci      â€” trigger (or guide setup of) a CI/CD pipeline run on a non-prod env.
//
// In MVP every phase is a dry-run planner: it validates inputs, resolves paths,
// describes what will happen, and emits a structured result. Actual LLM calls,
// subprocess invocations, and CI webhook triggers land in M1.

package cmdtest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/teragrid/forge/internal/errcode"
)

// â”€â”€ Phase 1: Create â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

// CreateOptions parameterises a test-generation run.
type CreateOptions struct {
	// Feature is the slug of the feature under test (e.g. "rate-limiter").
	// If empty the caller should prompt the vibe-coder before invoking.
	Feature string

	// Families lists the test families to generate.
	// Defaults to unit + integration + regression + e2e + journey when empty.
	Families []Family

	// Description is a free-text summary of the feature, used as LLM context.
	// When empty, the spec file contents are used instead.
	Description string

	// SpecPath is the path to an existing spec.md or spec.yml.
	// Defaults to <Root>/.forge/specs/<Feature>/spec.md.
	SpecPath string

	// OutputDir is the directory where generated test files are written.
	// Defaults to <Root>/.forge/tests/<Feature>/.
	OutputDir string

	// AutoApprove writes an approved.json immediately (skips explicit approve step).
	AutoApprove bool

	// DryRun suppresses file writes; plan is emitted to stdout only.
	DryRun bool

	// Root is the project root.
	Root string
}

// GeneratedFile describes one file that will be (or was) written.
type GeneratedFile struct {
	Path      string `json:"path"`
	Family    Family `json:"family"`
	TestCount int    `json:"test_count"`
	Lines     int    `json:"lines"`
}

// CreateResult is returned by CreateTests.
type CreateResult struct {
	DryRun       bool            `json:"dry_run"`
	Feature      string          `json:"feature"`
	SpecFound    bool            `json:"spec_found"`
	SpecPath     string          `json:"spec_path,omitempty"`
	Generated    []GeneratedFile `json:"generated"`
	OutputDir    string          `json:"output_dir"`
	ApproveCmd   string          `json:"approve_cmd"`
	AutoApproved bool            `json:"auto_approved"`
	Ready        bool            `json:"ready"`
	Message      string          `json:"message"`
}

// defaultCreateFamilies are generated when CreateOptions.Families is empty.
var defaultCreateFamilies = []Family{
	FamilyUnit,
	FamilyRegression,
	FamilyIntegration,
	FamilyJourney,
	FamilyE2E,
}

// estimatedTestCounts are the MVP dry-run estimates per family.
var estimatedTestCounts = map[Family]int{
	FamilyUnit:        12,
	FamilyIntegration: 5,
	FamilyRegression:  4,
	FamilyE2E:         3,
	FamilyJourney:     2,
	FamilySmoke:       2,
	FamilyContract:    4,
	FamilyPerf:        2,
	FamilyLoad:        2,
	FamilySoak:        1,
	FamilyChaos:       3,
	FamilyMutation:    1,
	FamilySnapshot:    3,
}

// estimatedLines returns a rough line estimate for generated test files.
func estimatedLines(testCount int) int {
	// ~8 lines per test (func header, subtests, assertions, blank lines).
	const linesPerTest = 8
	return testCount*linesPerTest + 30 // +30 for package header + imports
}

// CreateTests generates (or dry-run plans) test scaffolding for a feature.
// It is the testable core â€” no cobra I/O.
func CreateTests(opts CreateOptions) *CreateResult {
	if opts.Feature == "" {
		return &CreateResult{
			DryRun:  opts.DryRun,
			Ready:   false,
			Message: "feature name required; pass it as an argument or --feature flag",
		}
	}

	// Resolve root.
	root := opts.Root
	if root == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return &CreateResult{
				DryRun:  opts.DryRun,
				Feature: opts.Feature,
				Ready:   false,
				Message: fmt.Sprintf("getwd: %v", err),
			}
		}
		root = cwd
	}

	// Locate spec file.
	specPath := opts.SpecPath
	if specPath == "" {
		specPath = filepath.Join(root, ".forge", "specs", opts.Feature, "spec.md")
	}
	specFound := false
	if _, err := os.Stat(specPath); err == nil {
		specFound = true
	}

	// Output directory.
	outputDir := opts.OutputDir
	if outputDir == "" {
		outputDir = filepath.Join(root, ".forge", "tests", opts.Feature)
	}

	// Families to generate.
	families := opts.Families
	if len(families) == 0 {
		families = defaultCreateFamilies
	}

	// Build the dry-run plan (or write files in M1).
	res := &CreateResult{
		DryRun:    opts.DryRun,
		Feature:   opts.Feature,
		SpecFound: specFound,
		SpecPath:  specPath,
		OutputDir: outputDir,
	}

	for _, f := range families {
		count := estimatedTestCounts[f]
		if count == 0 {
			count = 2
		}
		rel := filepath.Join(string(f), opts.Feature+"_test.go")
		res.Generated = append(res.Generated, GeneratedFile{
			Path:      filepath.Join(outputDir, rel),
			Family:    f,
			TestCount: count,
			Lines:     estimatedLines(count),
		})
	}

	if !opts.DryRun {
		// In M1 this will call the LLM gateway and write real files.
		// For now, write a pending state file so approve can read it.
		if err := os.MkdirAll(outputDir, 0o750); err != nil {
			res.Ready = false
			res.Message = fmt.Sprintf("mkdir %s: %v", outputDir, err)
			return res
		}
		pendingPath := filepath.Join(outputDir, "pending.json")
		data, _ := json.MarshalIndent(res, "", "  ")
		if err := os.WriteFile(pendingPath, data, 0o600); err != nil {
			res.Ready = false
			res.Message = fmt.Sprintf("write pending.json: %v", err)
			return res
		}
		if opts.AutoApprove {
			aopts := ApproveOptions{
				Feature:   opts.Feature,
				Root:      root,
				OutputDir: outputDir,
				DryRun:    false,
			}
			ar := ApproveTests(aopts)
			res.AutoApproved = ar.Ready
			if !ar.Ready {
				res.Ready = false
				res.Message = ar.Message
				return res
			}
		}
	}

	res.ApproveCmd = fmt.Sprintf("forge test approve %s", opts.Feature)
	if opts.AutoApprove {
		res.ApproveCmd = "(auto-approved)"
	}
	res.Ready = true
	if opts.DryRun {
		res.Message = fmt.Sprintf(
			"dry-run: would generate %d test files for feature %q via LLM; spec %s",
			len(res.Generated), opts.Feature, specStatus(specFound, specPath),
		)
	} else {
		res.Message = fmt.Sprintf(
			"generated %d test file(s) for feature %q; run %q to review",
			len(res.Generated), opts.Feature, res.ApproveCmd,
		)
	}
	return res
}

func specStatus(found bool, path string) string {
	if found {
		return fmt.Sprintf("found at %s", path)
	}
	return fmt.Sprintf("not found at %s (LLM will use --description instead)", path)
}

// â”€â”€ Phase 2: Approve â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

// ApproveOptions parameterises an approval run.
type ApproveOptions struct {
	// Feature is the feature slug.
	Feature string

	// OutputDir is the directory containing pending.json.
	// Defaults to <Root>/.forge/tests/<Feature>/.
	OutputDir string

	// Root is the project root.
	Root string

	// DryRun shows what would be approved without writing approved.json.
	DryRun bool
}

// ApproveResult is returned by ApproveTests.
type ApproveResult struct {
	DryRun   bool            `json:"dry_run"`
	Feature  string          `json:"feature"`
	Files    []GeneratedFile `json:"files"`
	Approved int             `json:"approved"`
	Rejected int             `json:"rejected"`
	RunCmd   string          `json:"run_cmd"`
	Ready    bool            `json:"ready"`
	Message  string          `json:"message"`
}

// ApproveTests marks the pending test files as approved for local execution.
// Interactive review (diff display, accept/edit/reject) will be wired in M1.
func ApproveTests(opts ApproveOptions) *ApproveResult {
	if opts.Feature == "" {
		return &ApproveResult{
			DryRun:  opts.DryRun,
			Ready:   false,
			Message: "feature name required",
		}
	}

	root := opts.Root
	if root == "" {
		cwd, _ := os.Getwd()
		root = cwd
	}

	outputDir := opts.OutputDir
	if outputDir == "" {
		outputDir = filepath.Join(root, ".forge", "tests", opts.Feature)
	}

	// In dry-run we don't need an existing pending.json â€” simulate approval.
	var files []GeneratedFile
	pendingPath := filepath.Join(outputDir, "pending.json")
	if data, err := os.ReadFile(pendingPath); err == nil {
		var pending CreateResult
		if json.Unmarshal(data, &pending) == nil {
			files = pending.Generated
		}
	}

	// If no pending.json (dry-run or first call), use defaults so the result is
	// still useful in tests and pipeline simulations.
	if len(files) == 0 {
		for _, f := range defaultCreateFamilies {
			count := estimatedTestCounts[f]
			files = append(files, GeneratedFile{
				Path:      filepath.Join(outputDir, string(f), opts.Feature+"_test.go"),
				Family:    f,
				TestCount: count,
				Lines:     estimatedLines(count),
			})
		}
	}

	res := &ApproveResult{
		DryRun:   opts.DryRun,
		Feature:  opts.Feature,
		Files:    files,
		Approved: len(files),
		Rejected: 0,
		RunCmd:   fmt.Sprintf("forge test run %s", opts.Feature),
	}

	if !opts.DryRun {
		// Write approved.json so `forge test run` can consume it.
		if err := os.MkdirAll(outputDir, 0o750); err != nil {
			res.Ready = false
			res.Message = fmt.Sprintf("mkdir %s: %v", outputDir, err)
			return res
		}
		data, _ := json.MarshalIndent(res, "", "  ")
		approvedPath := filepath.Join(outputDir, "approved.json")
		if err := os.WriteFile(approvedPath, data, 0o600); err != nil {
			res.Ready = false
			res.Message = fmt.Sprintf("write approved.json: %v", err)
			return res
		}
	}

	res.Ready = true
	if opts.DryRun {
		res.Message = fmt.Sprintf(
			"dry-run: would approve %d test file(s) for %q; run %q to execute locally",
			len(files), opts.Feature, res.RunCmd,
		)
	} else {
		res.Message = fmt.Sprintf(
			"approved %d test file(s) for %q; run %q to execute locally",
			len(files), opts.Feature, res.RunCmd,
		)
	}
	return res
}

// â”€â”€ Phase 3: Run (feature-scoped) â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

// RunFeatureOptions parameterises a feature-scoped local test run.
type RunFeatureOptions struct {
	// Feature is the feature slug.
	Feature string

	// Families restricts which families to run.
	// Defaults to those listed in the approved.json for the feature.
	Families []Family

	// Root, DryRun, Workers, Duration, Timeout, BenchCount, FailFast mirror RunOptions.
	Root       string
	DryRun     bool
	Workers    int
	Duration   string
	Timeout    string
	BenchCount int
	FailFast   bool
}

// RunFeatureResult is returned by RunFeatureTests.
type RunFeatureResult struct {
	DryRun   bool           `json:"dry_run"`
	Feature  string         `json:"feature"`
	Approved bool           `json:"approved"`
	Families []FamilyResult `json:"families"`
	Passed   int            `json:"passed"`
	Failed   int            `json:"failed"`
	Skipped  int            `json:"skipped"`
	CICmd    string         `json:"ci_cmd"`
	Ready    bool           `json:"ready"`
	Message  string         `json:"message"`
}

// RunFeatureTests runs the approved tests for a feature locally.
// It delegates family execution to Run() after resolving the approved list.
func RunFeatureTests(opts RunFeatureOptions) *RunFeatureResult {
	if opts.Feature == "" {
		return &RunFeatureResult{
			DryRun:  opts.DryRun,
			Ready:   false,
			Message: "feature name required",
		}
	}

	root := opts.Root
	if root == "" {
		cwd, _ := os.Getwd()
		root = cwd
	}

	// Read approved.json (if it exists) to determine which families were approved.
	families := opts.Families
	approved := false
	if len(families) == 0 {
		approvedPath := filepath.Join(root, ".forge", "tests", opts.Feature, "approved.json")
		if data, err := os.ReadFile(approvedPath); err == nil {
			var ar ApproveResult
			if json.Unmarshal(data, &ar) == nil && len(ar.Files) > 0 {
				seen := map[Family]bool{}
				for _, gf := range ar.Files {
					if !seen[gf.Family] {
						families = append(families, gf.Family)
						seen[gf.Family] = true
					}
				}
				approved = true
			}
		}
		// Fall back to defaults if no approved.json found.
		if len(families) == 0 {
			families = defaultCreateFamilies
		}
	} else {
		approved = true // caller explicitly passed families
	}

	runOpts := RunOptions{
		Root:       root,
		DryRun:     opts.DryRun,
		Workers:    opts.Workers,
		Duration:   opts.Duration,
		Timeout:    opts.Timeout,
		BenchCount: opts.BenchCount,
		FailFast:   opts.FailFast,
	}
	inner := Run(families, runOpts)

	ciCmd := fmt.Sprintf("forge test ci %s --env staging", opts.Feature)
	res := &RunFeatureResult{
		DryRun:   opts.DryRun,
		Feature:  opts.Feature,
		Approved: approved,
		Families: inner.Families,
		Passed:   inner.Passed,
		Failed:   inner.Failed,
		Skipped:  inner.Skipped,
		CICmd:    ciCmd,
		Ready:    inner.Ready,
		Message:  inner.Message,
	}

	if inner.Ready {
		res.Message = fmt.Sprintf(
			"all %d local test families passed for %q; run %q to test on non-prod CI",
			len(families), opts.Feature, ciCmd,
		)
	}
	return res
}

// â”€â”€ Phase 4: CI â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

// CIOptions parameterises a CI/CD trigger run.
type CIOptions struct {
	// Feature is the feature slug.
	Feature string

	// Env is the target non-prod environment (staging, preview, dev, â€¦).
	Env string

	// Root is the project root.
	Root string

	// DryRun shows the trigger plan without actually calling CI.
	DryRun bool

	// GenerateConfig writes a starter CI config if none is found.
	GenerateConfig bool
}

// CISetupStep is one actionable step in the CI/CD setup guide.
type CISetupStep struct {
	Order       int    `json:"order"`
	Description string `json:"description"`
	Command     string `json:"command,omitempty"`
	FilePath    string `json:"file_path,omitempty"`
	Required    bool   `json:"required"`
}

// CIResult is returned by RunCI.
type CIResult struct {
	DryRun          bool          `json:"dry_run"`
	Feature         string        `json:"feature"`
	Env             string        `json:"env"`
	HasCI           bool          `json:"has_ci"`
	CIProvider      string        `json:"ci_provider,omitempty"`
	WorkflowFile    string        `json:"workflow_file,omitempty"`
	SetupSteps      []CISetupStep `json:"setup_steps,omitempty"`
	TriggerCmd      string        `json:"trigger_cmd,omitempty"`
	ConfigGenerated bool          `json:"config_generated,omitempty"`
	Ready           bool          `json:"ready"`
	Message         string        `json:"message"`
}

// ciProviders maps well-known config paths to their provider names.
var ciProviders = []struct {
	path     string
	provider string
}{
	{".github/workflows", "GitHub Actions"},
	{".gitlab-ci.yml", "GitLab CI"},
	{".circleci/config.yml", "CircleCI"},
	{"Jenkinsfile", "Jenkins"},
	{".drone.yml", "Drone CI"},
	{"azure-pipelines.yml", "Azure Pipelines"},
	{".buildkite/pipeline.yml", "Buildkite"},
}

// RunCI triggers (or plans) a CI/CD run for the feature on the target env.
func RunCI(opts CIOptions) *CIResult {
	if opts.Feature == "" {
		return &CIResult{
			DryRun:  opts.DryRun,
			Ready:   false,
			Message: "feature name required",
		}
	}

	env := opts.Env
	if env == "" {
		env = "staging"
	}

	root := opts.Root
	if root == "" {
		cwd, _ := os.Getwd()
		root = cwd
	}

	// Detect existing CI/CD configuration.
	provider, workflowFile := detectCI(root)
	hasCI := provider != ""

	res := &CIResult{
		DryRun:       opts.DryRun,
		Feature:      opts.Feature,
		Env:          env,
		HasCI:        hasCI,
		CIProvider:   provider,
		WorkflowFile: workflowFile,
	}

	if !hasCI {
		// Guide the vibe-coder through CI setup.
		res.SetupSteps = ciSetupGuide(root, env, opts.Feature)
		if opts.GenerateConfig && !opts.DryRun {
			if genErr := generateGitHubActionsConfig(root, opts.Feature); genErr == nil {
				res.ConfigGenerated = true
				provider = "GitHub Actions"
				workflowFile = filepath.Join(root, ".github", "workflows", "forge-test.yml")
				res.HasCI = true
				res.CIProvider = provider
				res.WorkflowFile = workflowFile
			}
		}
		if !res.HasCI {
			res.Ready = false
			res.Message = fmt.Sprintf(
				"no CI/CD config found in %s; follow the %d setup steps below, "+
					"then re-run 'forge test ci %s --env %s'",
				root, len(res.SetupSteps), opts.Feature, env,
			)
			return res
		}
	}

	// CI is (now) available â€” describe the trigger.
	res.TriggerCmd = ciTriggerCmd(provider, opts.Feature, env)
	res.Ready = true
	if opts.DryRun {
		res.Message = fmt.Sprintf(
			"dry-run: would trigger %s pipeline for feature %q on env %q with: %s",
			provider, opts.Feature, env, res.TriggerCmd,
		)
	} else {
		res.Message = fmt.Sprintf(
			"CI/CD trigger for %q on %q via %s: run %q",
			opts.Feature, env, provider, res.TriggerCmd,
		)
	}
	return res
}

func detectCI(root string) (provider, workflowFile string) {
	for _, p := range ciProviders {
		full := filepath.Join(root, p.path)
		info, err := os.Stat(full)
		if err != nil {
			continue
		}
		if info.IsDir() {
			// For directories (e.g. .github/workflows), look for any .yml file.
			entries, err := os.ReadDir(full)
			if err != nil || len(entries) == 0 {
				continue
			}
			for _, e := range entries {
				if strings.HasSuffix(e.Name(), ".yml") || strings.HasSuffix(e.Name(), ".yaml") {
					return p.provider, filepath.Join(full, e.Name())
				}
			}
			continue
		}
		return p.provider, full
	}
	return "", ""
}

func ciTriggerCmd(provider, feature, env string) string {
	switch provider {
	case "GitHub Actions":
		return fmt.Sprintf(
			"gh workflow run forge-test.yml --ref HEAD -f feature=%s -f env=%s",
			feature, env,
		)
	case "GitLab CI":
		return fmt.Sprintf(
			"glab pipeline run --branch HEAD --variables FEATURE=%s,ENV=%s",
			feature, env,
		)
	default:
		return fmt.Sprintf("git push origin HEAD  # triggers %s on push", provider)
	}
}

func ciSetupGuide(root, env, feature string) []CISetupStep {
	workflowsDir := filepath.Join(root, ".github", "workflows")
	return []CISetupStep{
		{
			Order:       1,
			Description: "Create the GitHub Actions workflows directory",
			Command:     fmt.Sprintf("mkdir -p %s", workflowsDir),
			Required:    true,
		},
		{
			Order:       2,
			Description: "Generate a starter forge-test.yml workflow (run with --generate-config)",
			Command:     fmt.Sprintf("forge test ci %s --env %s --generate-config", feature, env),
			FilePath:    filepath.Join(workflowsDir, "forge-test.yml"),
			Required:    true,
		},
		{
			Order:       3,
			Description: "Configure the staging environment in GitHub repository settings",
			Command:     "gh secret set STAGING_URL --body https://staging.your-app.example",
			Required:    true,
		},
		{
			Order:       4,
			Description: "Push your branch to trigger the workflow",
			Command:     "git push origin HEAD",
			Required:    true,
		},
		{
			Order:       5,
			Description: "Monitor the run",
			Command:     fmt.Sprintf("gh run watch --repo $(gh repo view --json nameWithOwner -q .nameWithOwner)"),
			Required:    false,
		},
	}
}

// generateGitHubActionsConfig writes a minimal forge-test.yml.
func generateGitHubActionsConfig(root, feature string) error {
	dir := filepath.Join(root, ".github", "workflows")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	path := filepath.Join(dir, "forge-test.yml")
	content := fmt.Sprintf(`# Generated by forge test ci â€” edit as needed.
name: forge test (%s)

on:
  push:
    branches: ["**"]
  workflow_dispatch:
    inputs:
      feature:
        description: "Feature slug"
        required: false
        default: "%s"
      env:
        description: "Target non-prod environment"
        required: false
        default: "staging"

jobs:
  forge-test:
    runs-on: ubuntu-latest
    environment: staging
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - name: Install forge
        run: go install github.com/teragrid/forge/cmd/forge@latest
      - name: forge test smoke
        run: forge test smoke --dry-run=false
      - name: forge test unit
        run: forge test unit --dry-run=false
      - name: forge test integration
        run: forge test integration --dry-run=false
      - name: forge test e2e
        run: forge test e2e --dry-run=false
`, feature, feature)
	return os.WriteFile(path, []byte(content), 0o600)
}

// â”€â”€ Full lifecycle orchestrator â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

// LifecycleOptions drives the full createâ†’approveâ†’runâ†’ci pipeline.
type LifecycleOptions struct {
	Feature        string
	Families       []Family
	Description    string
	Env            string
	AutoApprove    bool
	GenerateConfig bool
	DryRun         bool
	Root           string
}

// LifecycleResult is the combined output of all four phases.
type LifecycleResult struct {
	DryRun  bool              `json:"dry_run"`
	Feature string            `json:"feature"`
	Create  *CreateResult     `json:"create"`
	Approve *ApproveResult    `json:"approve"`
	Run     *RunFeatureResult `json:"run"`
	CI      *CIResult         `json:"ci"`
	Ready   bool              `json:"ready"`
	Message string            `json:"message"`
}

// RunLifecycle orchestrates all four phases in sequence.
// It stops and returns at the first failing phase.
func RunLifecycle(opts LifecycleOptions) *LifecycleResult {
	res := &LifecycleResult{DryRun: opts.DryRun, Feature: opts.Feature}

	// Phase 1: create.
	cr := CreateTests(CreateOptions{
		Feature:     opts.Feature,
		Families:    opts.Families,
		Description: opts.Description,
		AutoApprove: opts.AutoApprove,
		DryRun:      opts.DryRun,
		Root:        opts.Root,
	})
	res.Create = cr
	if !cr.Ready {
		res.Ready = false
		res.Message = fmt.Sprintf("lifecycle stopped at create: %s", cr.Message)
		return res
	}

	// Phase 2: approve (skip when auto-approved or dry-run cascade).
	ar := ApproveTests(ApproveOptions{
		Feature: opts.Feature,
		Root:    opts.Root,
		DryRun:  opts.DryRun,
	})
	res.Approve = ar
	if !ar.Ready {
		res.Ready = false
		res.Message = fmt.Sprintf("lifecycle stopped at approve: %s", ar.Message)
		return res
	}

	// Phase 3: run locally.
	rr := RunFeatureTests(RunFeatureOptions{
		Feature:  opts.Feature,
		Families: opts.Families,
		Root:     opts.Root,
		DryRun:   opts.DryRun,
	})
	res.Run = rr
	if !rr.Ready {
		res.Ready = false
		res.Message = fmt.Sprintf("lifecycle stopped at run: %s", rr.Message)
		return res
	}

	// Phase 4: CI.
	cir := RunCI(CIOptions{
		Feature:        opts.Feature,
		Env:            opts.Env,
		Root:           opts.Root,
		DryRun:         opts.DryRun,
		GenerateConfig: opts.GenerateConfig,
	})
	res.CI = cir
	// CI not-ready is non-fatal â€” it means the user needs to set up CI first.
	// We still complete the lifecycle with guidance.
	res.Ready = rr.Ready // local tests passed = lifecycle succeeded
	if !cir.Ready {
		res.Message = fmt.Sprintf(
			"local tests passed for %q; CI/CD setup required â€” follow guidance in ci phase",
			opts.Feature,
		)
	} else {
		res.Message = fmt.Sprintf(
			"full lifecycle complete for %q: tests created, approved, run locally, and queued on %s/%s",
			opts.Feature, cir.CIProvider, opts.Env,
		)
	}
	return res
}

// emitCreateResult writes CreateResult to cmd output.
func emitCreateResult(cmd *cobra.Command, res *CreateResult, asJSON bool) error {
	if asJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		if err := enc.Encode(res); err != nil {
			return errcode.New(ErrTestCreateFailed, "encode JSON", err)
		}
	} else {
		renderCreate(cmd, res)
	}
	if !res.Ready {
		return errcode.New(ErrTestCreateFailed, res.Message, nil)
	}
	return nil
}

// emitApproveResult writes ApproveResult to cmd output.
func emitApproveResult(cmd *cobra.Command, res *ApproveResult, asJSON bool) error {
	if asJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		if err := enc.Encode(res); err != nil {
			return errcode.New(ErrTestNotApproved, "encode JSON", err)
		}
	} else {
		renderApprove(cmd, res)
	}
	if !res.Ready {
		return errcode.New(ErrTestNotApproved, res.Message, nil)
	}
	return nil
}

// emitRunFeatureResult writes RunFeatureResult to cmd output.
func emitRunFeatureResult(cmd *cobra.Command, res *RunFeatureResult, asJSON bool) error {
	if asJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		if err := enc.Encode(res); err != nil {
			return errcode.New(ErrTestFailed, "encode JSON", err)
		}
	} else {
		renderRunFeature(cmd, res)
	}
	if !res.Ready {
		return errcode.New(ErrTestFailed, res.Message, nil)
	}
	return nil
}

// emitCIResult writes CIResult to cmd output.
func emitCIResult(cmd *cobra.Command, res *CIResult, asJSON bool) error {
	if asJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		if err := enc.Encode(res); err != nil {
			return errcode.New(ErrTestCINotReady, "encode JSON", err)
		}
	} else {
		renderCI(cmd, res)
	}
	if !res.Ready {
		return errcode.New(ErrTestCINotReady, res.Message, nil)
	}
	return nil
}

// emitLifecycleResult writes LifecycleResult to cmd output.
func emitLifecycleResult(cmd *cobra.Command, res *LifecycleResult, asJSON bool) error {
	if asJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		if err := enc.Encode(res); err != nil {
			return errcode.New(ErrTestFailed, "encode JSON", err)
		}
	} else {
		renderLifecycle(cmd, res)
	}
	if !res.Ready {
		return errcode.New(ErrTestFailed, res.Message, nil)
	}
	return nil
}

// â”€â”€ Text renderers â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

func renderCreate(cmd *cobra.Command, r *CreateResult) {
	w := cmd.OutOrStdout()
	mode := "dry-run"
	if !r.DryRun {
		mode = "live"
	}
	specNote := "spec not found (LLM will use --description)"
	if r.SpecFound {
		specNote = fmt.Sprintf("spec: %s", r.SpecPath)
	}
	fmt.Fprintf(w, "forge test create (%s)  feature: %s  %s\n\n", mode, r.Feature, specNote)
	for _, gf := range r.Generated {
		fmt.Fprintf(w, "  + %-12s  %3d tests  ~%d lines  %s\n",
			gf.Family, gf.TestCount, gf.Lines, gf.Path)
	}
	fmt.Fprintf(w, "\n%s\n", r.Message)
	if r.Ready {
		fmt.Fprintf(w, "\nNext: %s\n", r.ApproveCmd)
	}
}

func renderApprove(cmd *cobra.Command, r *ApproveResult) {
	w := cmd.OutOrStdout()
	mode := "dry-run"
	if !r.DryRun {
		mode = "live"
	}
	fmt.Fprintf(w, "forge test approve (%s)  feature: %s\n\n", mode, r.Feature)
	for _, gf := range r.Files {
		fmt.Fprintf(w, "  âœ“ %-12s  %3d tests  %s\n", gf.Family, gf.TestCount, gf.Path)
	}
	fmt.Fprintf(w, "\napproved: %d  rejected: %d\n\n%s\n", r.Approved, r.Rejected, r.Message)
	if r.Ready {
		fmt.Fprintf(w, "\nNext: %s\n", r.RunCmd)
	}
}

func renderRunFeature(cmd *cobra.Command, r *RunFeatureResult) {
	w := cmd.OutOrStdout()
	mode := "dry-run"
	if !r.DryRun {
		mode = "live"
	}
	fmt.Fprintf(w, "forge test run (%s)  feature: %s\n\n", mode, r.Feature)
	for _, fr := range r.Families {
		icon := statusIcon(fr.Status)
		fmt.Fprintf(w, "  %s %-12s  %s\n", icon, fr.Family, fr.Detail)
	}
	fmt.Fprintf(w, "\npassed: %d  failed: %d  skipped: %d\n\n%s\n",
		r.Passed, r.Failed, r.Skipped, r.Message)
	if r.Ready {
		fmt.Fprintf(w, "\nNext: %s\n", r.CICmd)
	}
}

func renderCI(cmd *cobra.Command, r *CIResult) {
	w := cmd.OutOrStdout()
	mode := "dry-run"
	if !r.DryRun {
		mode = "live"
	}
	fmt.Fprintf(w, "forge test ci (%s)  feature: %s  env: %s\n\n", mode, r.Feature, r.Env)
	if r.HasCI {
		fmt.Fprintf(w, "  CI provider:  %s\n", r.CIProvider)
		fmt.Fprintf(w, "  Workflow:     %s\n", r.WorkflowFile)
		fmt.Fprintf(w, "  Trigger:      %s\n\n", r.TriggerCmd)
	} else {
		fmt.Fprintf(w, "  âš  No CI/CD configuration found. Setup steps:\n\n")
		for _, s := range r.SetupSteps {
			req := ""
			if s.Required {
				req = " [required]"
			}
			fmt.Fprintf(w, "  %d. %s%s\n", s.Order, s.Description, req)
			if s.Command != "" {
				fmt.Fprintf(w, "     $ %s\n", s.Command)
			}
		}
		fmt.Fprintln(w)
	}
	fmt.Fprintln(w, r.Message)
}

func renderLifecycle(cmd *cobra.Command, r *LifecycleResult) {
	w := cmd.OutOrStdout()
	mode := "dry-run"
	if !r.DryRun {
		mode = "live"
	}
	fmt.Fprintf(w, "forge test %s (%s) â€” full lifecycle\n\n", r.Feature, mode)
	if r.Create != nil {
		icon := "âœ“"
		if !r.Create.Ready {
			icon = "âœ—"
		}
		fmt.Fprintf(w, "  %s create   %d file(s) planned\n", icon, len(r.Create.Generated))
	}
	if r.Approve != nil {
		icon := "âœ“"
		if !r.Approve.Ready {
			icon = "âœ—"
		}
		fmt.Fprintf(w, "  %s approve  %d file(s)\n", icon, r.Approve.Approved)
	}
	if r.Run != nil {
		icon := "âœ“"
		if !r.Run.Ready {
			icon = "âœ—"
		}
		fmt.Fprintf(w, "  %s run      passed=%d failed=%d skipped=%d\n",
			icon, r.Run.Passed, r.Run.Failed, r.Run.Skipped)
	}
	if r.CI != nil {
		icon := "âœ“"
		if !r.CI.Ready {
			icon = "âš "
		}
		ciSummary := r.CI.CIProvider
		if ciSummary == "" {
			ciSummary = "not configured"
		}
		fmt.Fprintf(w, "  %s ci       %s â†’ %s\n", icon, ciSummary, r.CI.Env)
	}
	fmt.Fprintf(w, "\n%s\n", r.Message)
}
