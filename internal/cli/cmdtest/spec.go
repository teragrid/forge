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

// spec.go — forge test spec: generate and run tests from a YAML/Markdown spec file.
//
// Commands:
//
//	forge test spec <feature>
//	    Templates a YAML test spec at .forge/specs/<feature>/spec.yml describing
//	    test cases, families, and expected behaviours.  Writes by default;
//	    use --dry-run to preview without writing.
//
//	forge test run --spec <path> [--dry-run]
//	    Reads a spec.yml, derives the required test families, and delegates to
//	    Run().  In dry-run mode it prints the derived execution plan only.
//
// Spec file format (.forge/specs/<feature>/spec.yml):
//
//	feature:     "rate-limiter"
//	version:     1
//	description: "Token-bucket rate limiter that rejects requests over quota"
//	families:
//	  - unit
//	  - integration
//	cases:
//	  - id: TC-01
//	    name:   "Happy path — under quota passes"
//	    family: unit
//	    type:   happy_path
//	    arrange: "Create a limiter with quota=10"
//	    act:     "Send 5 requests"
//	    assert:  "All 5 succeed with status 200"

package cmdtest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/teragrid/forge/internal/errcode"
)

// ── Spec data model ───────────────────────────────────────────────────────────

// TestSpec is the canonical on-disk format for a feature test specification.
type TestSpec struct {
	Feature     string     `yaml:"feature"     json:"feature"`
	Version     int        `yaml:"version"     json:"version"`
	Description string     `yaml:"description" json:"description"`
	Families    []Family   `yaml:"families"    json:"families"`
	Cases       []SpecCase `yaml:"cases"       json:"cases"`
}

// SpecCase describes one test case in a spec file.
type SpecCase struct {
	ID      string `yaml:"id"      json:"id"`
	Name    string `yaml:"name"    json:"name"`
	Family  Family `yaml:"family"  json:"family"`
	Type    string `yaml:"type"    json:"type"`    // happy_path|boundary|negative|idempotency|concurrency|authz|regression|data_accuracy|false_positive
	Arrange string `yaml:"arrange" json:"arrange"` // given / setup
	Act     string `yaml:"act"     json:"act"`     // when / action
	Assert  string `yaml:"assert"  json:"assert"`  // then / expectation
}

// caseTypes lists the test-design categories from the always-write-tests checklist.
var caseTypes = []string{
	"happy_path",
	"boundary",
	"negative",
	"idempotency",
	"concurrency",
	"authz",
	"regression",
	"data_accuracy",
	"false_positive",
}

// ── Generate ─────────────────────────────────────────────────────────────────

// SpecGenerateOptions parameterises a spec generation run.
type SpecGenerateOptions struct {
	// Feature is the slug of the feature under test.
	Feature string
	// Description is a free-text summary; used as LLM context when provided.
	Description string
	// Families to include.  Defaults to unit + integration + regression.
	Families []Family
	// SpecPath is the output path.  Defaults to <Root>/.forge/specs/<Feature>/spec.yml.
	SpecPath string
	// DryRun suppresses the file write and prints the template instead.
	DryRun bool
	// Root is the project root.
	Root string
}

// SpecGenerateResult is returned by GenerateSpec.
type SpecGenerateResult struct {
	DryRun    bool   `json:"dry_run"`
	Feature   string `json:"feature"`
	SpecPath  string `json:"spec_path"`
	CaseCount int    `json:"case_count"`
	Ready     bool   `json:"ready"`
	Message   string `json:"message"`
	// Spec is the generated spec (populated in all modes).
	Spec *TestSpec `json:"spec,omitempty"`
}

// defaultSpecFamilies when none are specified.
var defaultSpecFamilies = []Family{FamilyUnit, FamilyIntegration, FamilyRegression}

// GenerateSpec produces (or dry-run-plans) a test spec YAML file.
func GenerateSpec(opts SpecGenerateOptions) *SpecGenerateResult {
	if opts.Feature == "" {
		return &SpecGenerateResult{
			DryRun:  opts.DryRun,
			Ready:   false,
			Message: "feature name required",
		}
	}

	root := opts.Root
	if root == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return &SpecGenerateResult{
				DryRun:  opts.DryRun,
				Feature: opts.Feature,
				Ready:   false,
				Message: fmt.Sprintf("getwd: %v", err),
			}
		}
		root = cwd
	}

	specPath := opts.SpecPath
	if specPath == "" {
		specPath = filepath.Join(root, ".forge", "specs", opts.Feature, "spec.yml")
	}

	families := opts.Families
	if len(families) == 0 {
		families = defaultSpecFamilies
	}

	// Build the spec template using the 9-point test design checklist.
	spec := buildSpecTemplate(opts.Feature, opts.Description, families)

	res := &SpecGenerateResult{
		DryRun:    opts.DryRun,
		Feature:   opts.Feature,
		SpecPath:  specPath,
		CaseCount: len(spec.Cases),
		Spec:      spec,
	}

	if opts.DryRun {
		res.Ready = true
		res.Message = fmt.Sprintf(
			"dry-run: would write %d-case spec to %s (edit, then run `forge test run --spec %s`)",
			len(spec.Cases), specPath, specPath,
		)
		return res
	}

	// Write the spec file.
	if err := os.MkdirAll(filepath.Dir(specPath), 0o750); err != nil {
		res.Ready = false
		res.Message = fmt.Sprintf("mkdir %s: %v", filepath.Dir(specPath), err)
		return res
	}
	data, err := yaml.Marshal(spec)
	if err != nil {
		res.Ready = false
		res.Message = fmt.Sprintf("marshal spec: %v", err)
		return res
	}
	header := fmt.Sprintf("# forge test spec — %s\n# Generated by `forge test spec %s`\n# Edit the cases below, then run `forge test run --spec %s`\n\n",
		opts.Feature, opts.Feature, specPath)
	if err := os.WriteFile(specPath, append([]byte(header), data...), 0o600); err != nil {
		res.Ready = false
		res.Message = fmt.Sprintf("write %s: %v", specPath, err)
		return res
	}

	res.Ready = true
	res.Message = fmt.Sprintf("wrote %d-case spec to %s", len(spec.Cases), specPath)
	return res
}

// buildSpecTemplate produces a skeleton spec covering all 9 test-design categories.
func buildSpecTemplate(feature, description string, families []Family) *TestSpec {
	if description == "" {
		description = fmt.Sprintf("Describe %s here", feature)
	}

	// Seed one case per type per first family (keeps the template focused).
	firstFamily := FamilyUnit
	if len(families) > 0 {
		firstFamily = families[0]
	}

	var cases []SpecCase
	for i, ct := range caseTypes {
		id := fmt.Sprintf("TC-%02d", i+1)
		cases = append(cases, SpecCase{
			ID:      id,
			Name:    caseNameTemplate(ct, feature),
			Family:  firstFamily,
			Type:    ct,
			Arrange: "TODO: setup state",
			Act:     "TODO: call the function/endpoint under test",
			Assert:  "TODO: verify expected outcome",
		})
	}

	return &TestSpec{
		Feature:     feature,
		Version:     1,
		Description: description,
		Families:    families,
		Cases:       cases,
	}
}

func caseNameTemplate(caseType, feature string) string {
	templates := map[string]string{
		"happy_path":     "Happy path — %s succeeds under normal conditions",
		"boundary":       "Boundary — %s at exact threshold values",
		"negative":       "Negative — %s rejects invalid input",
		"idempotency":    "Idempotency — %s produces same result on retry",
		"concurrency":    "Concurrency — %s is safe under concurrent access",
		"authz":          "AuthZ — unauthorized caller cannot invoke %s",
		"regression":     "Regression — original bug in %s cannot recur",
		"data_accuracy":  "Data accuracy — %s returns correct values",
		"false_positive": "False positive guard — valid call to %s must not be rejected",
	}
	tpl, ok := templates[caseType]
	if !ok {
		tpl = "%s: " + caseType
	}
	return fmt.Sprintf(tpl, feature)
}

// ── Run from spec ─────────────────────────────────────────────────────────────

// SpecRunOptions parameterises a spec-driven run.
type SpecRunOptions struct {
	// SpecPath is the path to the spec.yml.
	// Defaults to <Root>/.forge/specs/<feature>/spec.yml.
	SpecPath string
	// Feature is used when SpecPath is not provided to locate the default spec.
	Feature string
	// RunOptions for the test families derived from the spec.
	RunOptions RunOptions
}

// SpecRunResult is returned by RunFromSpec.
type SpecRunResult struct {
	DryRun    bool        `json:"dry_run"`
	SpecPath  string      `json:"spec_path"`
	Feature   string      `json:"feature"`
	CaseCount int         `json:"case_count"`
	Families  []Family    `json:"families"`
	Result    *TestResult `json:"result"`
	Ready     bool        `json:"ready"`
	Message   string      `json:"message"`
}

// RunFromSpec reads a spec file and executes (or plans) the families it declares.
func RunFromSpec(opts SpecRunOptions) *SpecRunResult {
	root := opts.RunOptions.Root
	if root == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return &SpecRunResult{
				DryRun:  opts.RunOptions.DryRun,
				Ready:   false,
				Message: fmt.Sprintf("getwd: %v", err),
			}
		}
		root = cwd
		opts.RunOptions.Root = root
	}

	specPath := opts.SpecPath
	if specPath == "" && opts.Feature != "" {
		specPath = filepath.Join(root, ".forge", "specs", opts.Feature, "spec.yml")
	}
	if specPath == "" {
		return &SpecRunResult{
			DryRun:  opts.RunOptions.DryRun,
			Ready:   false,
			Message: "provide --spec <path> or --feature <name>",
		}
	}

	spec, err := loadSpec(specPath)
	if err != nil {
		return &SpecRunResult{
			DryRun:   opts.RunOptions.DryRun,
			SpecPath: specPath,
			Ready:    false,
			Message:  fmt.Sprintf("read spec %s: %v", specPath, err),
		}
	}

	// Derive the unique family list from the spec (preserve declaration order).
	families := deduplicateFamilies(spec.Families, spec.Cases)

	testResult := Run(families, opts.RunOptions)

	res := &SpecRunResult{
		DryRun:    opts.RunOptions.DryRun,
		SpecPath:  specPath,
		Feature:   spec.Feature,
		CaseCount: len(spec.Cases),
		Families:  families,
		Result:    testResult,
		Ready:     testResult.Ready,
		Message: fmt.Sprintf(
			"spec %q: %d case(s) across %d family(ies)",
			spec.Feature, len(spec.Cases), len(families),
		),
	}
	return res
}

// loadSpec reads and unmarshals a spec.yml or spec.md (YAML front-matter).
func loadSpec(path string) (*TestSpec, error) {
	data, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		return nil, err
	}

	// Strip leading YAML comment lines (lines starting with #) before
	// unmarshalling — this allows us to write a human-readable header.
	lines := strings.Split(string(data), "\n")
	var yamlLines []string
	for _, l := range lines {
		if strings.HasPrefix(strings.TrimSpace(l), "#") {
			continue
		}
		yamlLines = append(yamlLines, l)
	}

	var spec TestSpec
	if err := yaml.Unmarshal([]byte(strings.Join(yamlLines, "\n")), &spec); err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}
	if spec.Feature == "" {
		return nil, fmt.Errorf("spec.feature is required")
	}
	return &spec, nil
}

// deduplicateFamilies merges families declared at the spec level with those
// referenced in individual cases (spec-level takes precedence in ordering).
func deduplicateFamilies(specFamilies []Family, cases []SpecCase) []Family {
	seen := make(map[Family]struct{})
	var result []Family
	for _, f := range specFamilies {
		if _, ok := seen[f]; !ok {
			seen[f] = struct{}{}
			result = append(result, f)
		}
	}
	for _, c := range cases {
		if c.Family == "" {
			continue
		}
		if _, ok := seen[c.Family]; !ok {
			seen[c.Family] = struct{}{}
			result = append(result, c.Family)
		}
	}
	if len(result) == 0 {
		result = defaultSpecFamilies
	}
	return result
}

// ── Cobra wiring ──────────────────────────────────────────────────────────────

// newSpecCmd returns the `forge test spec <feature>` leaf command.
// It writes a 9-case YAML spec to .forge/specs/<feature>/spec.yml by default.
// Use --dry-run to preview without writing.
// `forge test run --spec <path>` is the companion run command registered on
// the parent `forge test` command.
func newSpecCmd() *cobra.Command {
	var (
		genRoot        string
		genSpecPath    string
		genDescription string
		genFamilies    []string
		genDryRun      bool
		genJSON        bool
	)
	specCmd := &cobra.Command{
		Use:   "spec <feature>",
		Short: "Generate a YAML test spec for a feature and save to .forge/specs/<feature>/spec.yml.",
		Long: strings.TrimSpace(`
forge test spec <feature> templates a 9-case YAML test spec covering all
test-design categories and writes it to .forge/specs/<feature>/spec.yml.

Edit the file to fill in the TODO fields, then run:
  forge test run --spec .forge/specs/<feature>/spec.yml

Use --dry-run to preview the spec without writing the file.
`),
		Example: `  forge test spec rate-limiter
  forge test spec auth --description "JWT login flow"
  forge test spec payments --dry-run
  forge test run --spec .forge/specs/rate-limiter/spec.yml`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var families []Family
			for _, f := range genFamilies {
				families = append(families, Family(f))
			}
			res := GenerateSpec(SpecGenerateOptions{
				Feature:     args[0],
				Description: genDescription,
				Families:    families,
				SpecPath:    genSpecPath,
				DryRun:      genDryRun,
				Root:        genRoot,
			})
			return emitSpecGenerateResult(cmd, res, genJSON)
		},
	}
	specCmd.Flags().StringVar(&genRoot, "root", "", "project root (default: cwd)")
	specCmd.Flags().StringVar(&genSpecPath, "spec", "", "output path (default: .forge/specs/<feature>/spec.yml)")
	specCmd.Flags().StringVar(&genDescription, "description", "", "feature description injected into the spec")
	specCmd.Flags().StringArrayVar(&genFamilies, "family", nil, "test family to include (repeatable; default: unit,integration,regression)")
	specCmd.Flags().BoolVar(&genDryRun, "dry-run", false, "preview spec without writing the file")
	specCmd.Flags().BoolVar(&genJSON, "json", false, "emit machine-readable JSON")
	return specCmd
}

// ── Emitters ──────────────────────────────────────────────────────────────────

func emitSpecGenerateResult(cmd *cobra.Command, res *SpecGenerateResult, asJSON bool) error {
	if asJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		if err := enc.Encode(res); err != nil {
			return errcode.New(ErrTestCreateFailed, "encode JSON", err)
		}
	} else {
		renderSpecGenerateText(cmd, res)
	}
	if !res.Ready {
		return errcode.New(ErrTestCreateFailed, res.Message, nil)
	}
	return nil
}

func renderSpecGenerateText(cmd *cobra.Command, r *SpecGenerateResult) {
	w := cmd.OutOrStdout()
	mode := "dry-run"
	if !r.DryRun {
		mode = "written"
	}
	fmt.Fprintf(w, "forge test spec (%s)\n", mode)
	fmt.Fprintf(w, "  feature:  %s\n", r.Feature)
	fmt.Fprintf(w, "  spec:     %s\n", r.SpecPath)
	fmt.Fprintf(w, "  cases:    %d\n", r.CaseCount)
	if r.Spec != nil {
		fmt.Fprintf(w, "  families: %s\n", joinFamilies(r.Spec.Families))
	}
	fmt.Fprintln(w)
	if r.DryRun && r.Spec != nil {
		// Print a preview of the spec YAML.
		data, _ := yaml.Marshal(r.Spec)
		fmt.Fprintf(w, "--- spec preview ---\n%s\n", string(data))
	}
	fmt.Fprintln(w, r.Message)
	if r.Ready && !r.DryRun {
		fmt.Fprintf(w, "\nNext: edit %s then run `forge test run --spec %s`\n",
			r.SpecPath, r.SpecPath)
	}
}

func emitSpecRunResult(cmd *cobra.Command, res *SpecRunResult, asJSON bool) error {
	if asJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		if err := enc.Encode(res); err != nil {
			return errcode.New(ErrTestFailed, "encode JSON", err)
		}
	} else {
		renderSpecRunText(cmd, res)
	}
	if !res.Ready {
		return errcode.New(ErrTestFailed, res.Message, nil)
	}
	return nil
}

func renderSpecRunText(cmd *cobra.Command, r *SpecRunResult) {
	w := cmd.OutOrStdout()
	mode := "dry-run"
	if r.Result != nil && !r.Result.DryRun {
		mode = "live"
	}
	fmt.Fprintf(w, "forge test run --spec (%s)\n", mode)
	fmt.Fprintf(w, "  spec:     %s\n", r.SpecPath)
	fmt.Fprintf(w, "  feature:  %s\n", r.Feature)
	fmt.Fprintf(w, "  cases:    %d\n", r.CaseCount)
	fmt.Fprintf(w, "  families: %s\n\n", joinFamilies(r.Families))

	if r.Result != nil {
		ts := r.Result
		for _, fr := range ts.Families {
			icon := statusIcon(fr.Status)
			fmt.Fprintf(w, "  %s %-12s  %s\n", icon, fr.Family, fr.Detail)
		}
		fmt.Fprintf(w, "\npassed: %d  failed: %d  skipped: %d  (%s ms)\n",
			ts.Passed, ts.Failed, ts.Skipped, ts.Duration)
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, r.Message)
}

func joinFamilies(fs []Family) string {
	ss := make([]string, len(fs))
	for i, f := range fs {
		ss[i] = string(f)
	}
	return strings.Join(ss, ", ")
}
