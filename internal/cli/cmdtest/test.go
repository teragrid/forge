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

// Package cmdtest implements `forge test` — the multi-family test orchestrator.
//
// Subcommands (families):
//
//	unit        – go test ./... (race detector on)
//	integration – integration tests (tag: integration)
//	regression  – regression guard tests (tag: regression)
//	e2e         – full end-to-end test suite
//	journey     – multi-step user journey tests
//	smoke       – lightweight health-check tests
//	contract    – API contract / schema compatibility tests
//	perf        – benchmark / performance tests (go test -bench)
//	load        – concurrent-load tests
//	soak        – long-running stability tests
//	chaos       – fault-injection / chaos drill tests
//	mutation    – mutation-testing run (requires mutants CLI)
//	snapshot    – golden-file / snapshot tests
//	all         – run every family in sequence
//
// The MVP implementation is a dry-run orchestrator: it validates that the
// requested family is known, emits a structured plan, and exits 0. Actual
// subprocess invocation lands in M1.
package cmdtest

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/teragrid/forge/internal/errcode"
	"github.com/teragrid/forge/internal/verbmeta"
)

// Reserved error codes (range 4300..4399).
var (
	ErrTestFailed        = errcode.Register(errcode.Code(4300), "test run failed")
	ErrTestUnknownFamily = errcode.Register(errcode.Code(4301), "unknown test family")
	ErrTestInvalidFlag   = errcode.Register(errcode.Code(4302), "invalid flag value for test family")
	ErrTestCreateFailed  = errcode.Register(errcode.Code(4303), "test generation failed; spec missing or LLM unavailable")
	ErrTestNotApproved   = errcode.Register(errcode.Code(4304), "tests not approved; run forge test approve first")
	ErrTestCINotReady    = errcode.Register(errcode.Code(4305), "CI/CD pipeline not configured; follow setup guidance")
)

// Family is a named test type.
type Family string

// All supported families — ordered from fastest to slowest so `forge test all`
// short-circuits on the first failure with the quickest feedback.
const (
	FamilyUnit        Family = "unit"
	FamilyIntegration Family = "integration"
	FamilyRegression  Family = "regression"
	FamilyE2E         Family = "e2e"
	FamilyJourney     Family = "journey"
	FamilySmoke       Family = "smoke"
	FamilyContract    Family = "contract"
	FamilyPerf        Family = "perf"
	FamilyLoad        Family = "load"
	FamilySoak        Family = "soak"
	FamilyChaos       Family = "chaos"
	FamilyMutation    Family = "mutation"
	FamilySnapshot    Family = "snapshot"
	FamilyAll         Family = "all"
)

// orderedFamilies is the canonical execution order used by `forge test all`.
var orderedFamilies = []Family{
	FamilySmoke,
	FamilyUnit,
	FamilyRegression,
	FamilySnapshot,
	FamilyContract,
	FamilyIntegration,
	FamilyJourney,
	FamilyE2E,
	FamilyPerf,
	FamilyLoad,
	FamilyChaos,
	FamilySoak,
	FamilyMutation,
}

// knownFamilies is the set of valid family names for validation.
var knownFamilies = func() map[Family]struct{} {
	m := make(map[Family]struct{}, len(orderedFamilies)+1)
	for _, f := range orderedFamilies {
		m[f] = struct{}{}
	}
	m[FamilyAll] = struct{}{}
	return m
}()

// FamilyMeta describes one test family.
type FamilyMeta struct {
	Family      Family `json:"family"`
	Description string `json:"description"`
	Tag         string `json:"tag,omitempty"` // go build tag that selects these tests
	Benchmark   bool   `json:"benchmark,omitempty"`
}

// FamilyRegistry returns metadata for every known family.
func FamilyRegistry() []FamilyMeta {
	return []FamilyMeta{
		{FamilyUnit, "Fast unit tests with the race detector (go test -race ./...)", "", false},
		{FamilyIntegration, "Integration tests requiring live dependencies (build tag: integration)", "integration", false},
		{FamilyRegression, "Regression guard tests that must fail on pre-fix code (build tag: regression)", "regression", false},
		{FamilyE2E, "Full end-to-end tests exercising the binary against a real environment", "e2e", false},
		{FamilyJourney, "Multi-step user journey tests mirroring GETTING_STARTED.md workflows", "journey", false},
		{FamilySmoke, "Quick health-check tests (< 5 s); first gate in CI", "smoke", false},
		{FamilyContract, "API contract and schema compatibility tests (build tag: contract)", "contract", false},
		{FamilyPerf, "Benchmark / performance tests (go test -bench=. -benchmem)", "", true},
		{FamilyLoad, "Concurrent load tests with configurable worker count (--workers)", "load", false},
		{FamilySoak, "Long-running stability tests (--duration default 1h)", "soak", false},
		{FamilyChaos, "Fault-injection / chaos drill tests (build tag: chaos)", "chaos", false},
		{FamilyMutation, "Mutation testing to validate test effectiveness (requires go-mutesting)", "mutation", false},
		{FamilySnapshot, "Golden-file / snapshot tests comparing output to stored baselines (build tag: snapshot)", "snapshot", false},
	}
}

// TestResult is the structured report emitted by `forge test`.
type TestResult struct {
	DryRun   bool           `json:"dry_run"`
	Families []FamilyResult `json:"families"`
	Passed   int            `json:"passed"`
	Failed   int            `json:"failed"`
	Skipped  int            `json:"skipped"`
	Duration string         `json:"duration_ms"`
	Ready    bool           `json:"ready"`
	Message  string         `json:"message"`
}

// FamilyResult is the per-family portion of TestResult.
type FamilyResult struct {
	Family     Family `json:"family"`
	Status     string `json:"status"` // "ok", "fail", "skipped", "pending"
	TestCount  int    `json:"test_count"`
	Detail     string `json:"detail"`
	DurationMs int64  `json:"duration_ms"`
}

func init() {
	verbmeta.Register(verbmeta.Manifest{
		Verb:    "test",
		Summary: "Run one or all test families: unit, integration, regression, e2e, journey, smoke, contract, perf, load, soak, chaos, mutation, snapshot, all. Also orchestrates a 4-phase lifecycle: create → approve → run → ci.",
		Inputs: []string{
			"<family>: unit|integration|regression|e2e|journey|smoke|contract|perf|load|soak|chaos|mutation|snapshot|all",
			"create <feature>     generate test scaffolding for a feature via LLM",
			"approve <feature>    review and approve generated tests",
			"run <feature>        run approved tests locally across selected families",
			"ci <feature>         trigger or guide CI/CD pipeline run on non-prod env",
			"--feature <slug>     run full lifecycle (create→approve→run→ci)",
			"--root <path>        (project root; default cwd)",
			"--dry-run            (print plan without executing; default in MVP)",
			"--parallel           (run families concurrently; default: sequential)",
			"--workers <n>        (concurrent workers for load/soak; default 10)",
			"--duration <d>       (soak/load duration, e.g. 30m, 1h; default 1h for soak)",
			"--timeout <d>        (per-family deadline; default 10m)",
			"--bench-count <n>    (perf: number of benchmark iterations; default 5)",
			"--fail-fast          (stop all families on first failure)",
			"--env <name>         (target non-prod environment for CI; default staging)",
			"--description <text> (feature description for LLM context)",
			"--auto-approve       (skip interactive approval step)",
			"--generate-config    (generate CI config if none found)",
			"--json               (machine-readable output)",
		},
		Outputs:      []string{"stdout: per-family status table (text or JSON)"},
		SideEffects:  []string{"--dry-run has no side effects; full run (M1) invokes go test and external tools"},
		GatesTouched: []string{"§4 5-checkpoint pipeline checkpoint 2 (Test)", "§16.5.4 test coverage gate"},
		ErrorCodes:   []errcode.Code{ErrTestFailed, ErrTestUnknownFamily, ErrTestInvalidFlag, ErrTestCreateFailed, ErrTestNotApproved, ErrTestCINotReady},
	})
}

// New returns the top-level cobra command with one subcommand per family
// and four lifecycle subcommands (create, approve, run, ci).
func New() *cobra.Command {
	var (
		root       string
		dryRun     bool
		parallel   bool
		workers    int
		duration   string
		timeout    string
		benchCount int
		failFast   bool
		asJSON     bool
		// Lifecycle flags (used when --feature is provided on the parent).
		feature        string
		description    string
		env            string
		autoApprove    bool
		generateConfig bool
	)

	// bindFlags attaches the shared flags to a subcommand.
	bindFlags := func(c *cobra.Command) {
		c.Flags().StringVar(&root, "root", "", "project root (default: cwd)")
		c.Flags().BoolVar(&dryRun, "dry-run", true, "print execution plan without running tests")
		c.Flags().BoolVar(&parallel, "parallel", false, "run families concurrently")
		c.Flags().IntVar(&workers, "workers", 10, "concurrent worker count (load/soak)")
		c.Flags().StringVar(&duration, "duration", "1h", "test duration (soak/load)")
		c.Flags().StringVar(&timeout, "timeout", "10m", "per-family timeout")
		c.Flags().IntVar(&benchCount, "bench-count", 5, "benchmark iteration count (perf)")
		c.Flags().BoolVar(&failFast, "fail-fast", false, "stop on first family failure")
		c.Flags().BoolVar(&asJSON, "json", false, "emit machine-readable JSON")
	}

	// runFamily is the shared RunE body for every single-family subcommand.
	runFamily := func(cmd *cobra.Command, f Family) error {
		r, err := projectRoot(root)
		if err != nil {
			return err
		}
		opts := RunOptions{
			Root:       r,
			DryRun:     dryRun,
			Workers:    workers,
			Duration:   duration,
			Timeout:    timeout,
			BenchCount: benchCount,
			FailFast:   failFast,
		}
		res := Run([]Family{f}, opts)
		return emit(cmd, res, asJSON)
	}

	parent := &cobra.Command{
		Use:   "test <family|create|approve|run|ci>",
		Short: "Run test families or orchestrate the 4-phase test lifecycle (create→approve→run→ci).",
		Long: strings.TrimSpace(`
forge test orchestrates every kind of automated test across a project.

Each family is a subcommand. Use 'forge test all' to run every family in order
from fastest to slowest, short-circuiting on the first failure (unless
--fail-fast=false is set).

Test families
  unit         Fast unit tests with race detector
  integration  Tests requiring live dependencies (tag: integration)
  regression   Pre-fix guard tests (tag: regression)
  e2e          Full end-to-end binary tests
  journey      Multi-step user journey tests
  smoke        Quick health-check tests (< 5 s)
  contract     API contract / schema compatibility tests
  perf         Benchmarks (go test -bench)
  load         Concurrent load tests
  soak         Long-running stability tests
  chaos        Fault-injection / chaos drill tests
  mutation     Mutation testing (requires go-mutesting)
  snapshot     Golden-file / snapshot tests
  all          Run every family in sequence

4-phase test lifecycle
  create <feature>   LLM generates test scaffolding for the named feature
  approve <feature>  Review and approve generated tests
  run <feature>      Run approved tests locally across selected families
  ci <feature>       Trigger or guide a CI/CD pipeline run on a non-prod env

  Use --feature <slug> on the parent command to run the full lifecycle in one step.

The MVP implementation is a dry-run planner (--dry-run=true by default).
Full subprocess invocation lands in M1.
`),
		RunE: func(cmd *cobra.Command, args []string) error {
			// When --feature is provided (or feature is a positional arg), run full lifecycle.
			feat := feature
			if feat == "" && len(args) > 0 {
				feat = args[0]
			}
			if feat == "" {
				return cmd.Help()
			}
			r, err := projectRoot(root)
			if err != nil {
				return err
			}
			res := RunLifecycle(LifecycleOptions{
				Feature:        feat,
				Description:    description,
				Env:            env,
				AutoApprove:    autoApprove,
				GenerateConfig: generateConfig,
				DryRun:         dryRun,
				Root:           r,
			})
			return emitLifecycleResult(cmd, res, asJSON)
		},
	}
	// Lifecycle flags on the parent.
	parent.Flags().StringVar(&feature, "feature", "", "feature slug — triggers full lifecycle (create→approve→run→ci)")
	parent.Flags().StringVar(&description, "description", "", "feature description for LLM context")
	parent.Flags().StringVar(&env, "env", "staging", "target non-prod environment for CI")
	parent.Flags().BoolVar(&autoApprove, "auto-approve", false, "skip interactive approval step")
	parent.Flags().BoolVar(&generateConfig, "generate-config", false, "generate CI config if none found")
	parent.Flags().StringVar(&root, "root", "", "project root (default: cwd)")
	parent.Flags().BoolVar(&dryRun, "dry-run", true, "print execution plan without running tests")
	parent.Flags().BoolVar(&asJSON, "json", false, "emit machine-readable JSON")

	// ── Family-subcommand factory ──────────────────────────────────────────

	makeSubcmd := func(f Family, short string) *cobra.Command {
		c := &cobra.Command{
			Use:   string(f),
			Short: short,
			Args:  cobra.NoArgs,
			RunE:  func(cmd *cobra.Command, _ []string) error { return runFamily(cmd, f) },
		}
		bindFlags(c)
		return c
	}

	parent.AddCommand(
		makeSubcmd(FamilyUnit, "Run unit tests with race detector (go test -race ./...)"),
		makeSubcmd(FamilyIntegration, "Run integration tests (build tag: integration)"),
		makeSubcmd(FamilyRegression, "Run regression guard tests (build tag: regression)"),
		makeSubcmd(FamilyE2E, "Run full end-to-end tests"),
		makeSubcmd(FamilyJourney, "Run multi-step user journey tests"),
		makeSubcmd(FamilySmoke, "Run quick health-check tests (< 5 s)"),
		makeSubcmd(FamilyContract, "Run API contract / schema tests (build tag: contract)"),
		makeSubcmd(FamilyPerf, "Run benchmarks (go test -bench=. -benchmem)"),
		makeSubcmd(FamilyLoad, "Run concurrent load tests"),
		makeSubcmd(FamilySoak, "Run long-running soak/stability tests"),
		makeSubcmd(FamilyChaos, "Run fault-injection / chaos drill tests"),
		makeSubcmd(FamilyMutation, "Run mutation tests (requires go-mutesting)"),
		makeSubcmd(FamilySnapshot, "Run golden-file / snapshot tests"),
	)

	// ── `forge test all` ───────────────────────────────────────────────────
	allCmd := &cobra.Command{
		Use:   "all",
		Short: "Run every test family in sequence (fastest → slowest).",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			r, err := projectRoot(root)
			if err != nil {
				return err
			}
			opts := RunOptions{
				Root:       r,
				DryRun:     dryRun,
				Workers:    workers,
				Duration:   duration,
				Timeout:    timeout,
				BenchCount: benchCount,
				FailFast:   failFast,
			}
			res := Run(orderedFamilies, opts)
			return emit(cmd, res, asJSON)
		},
	}
	bindFlags(allCmd)
	parent.AddCommand(allCmd)

	// ── Lifecycle subcommands ──────────────────────────────────────────────

	// create: generate test scaffolding via LLM.
	createCmd := &cobra.Command{
		Use:   "create <feature>",
		Short: "Generate test scaffolding for a feature via LLM.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := projectRoot(root)
			if err != nil {
				return err
			}
			res := CreateTests(CreateOptions{
				Feature:     args[0],
				Description: description,
				AutoApprove: autoApprove,
				DryRun:      dryRun,
				Root:        r,
			})
			return emitCreateResult(cmd, res, asJSON)
		},
	}
	createCmd.Flags().StringVar(&root, "root", "", "project root (default: cwd)")
	createCmd.Flags().BoolVar(&dryRun, "dry-run", true, "print plan without writing files")
	createCmd.Flags().StringVar(&description, "description", "", "feature description for LLM context")
	createCmd.Flags().BoolVar(&autoApprove, "auto-approve", false, "auto-approve after generation")
	createCmd.Flags().BoolVar(&asJSON, "json", false, "emit machine-readable JSON")

	// approve: mark pending tests as approved.
	approveCmd := &cobra.Command{
		Use:   "approve <feature>",
		Short: "Review and approve generated tests for a feature.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := projectRoot(root)
			if err != nil {
				return err
			}
			res := ApproveTests(ApproveOptions{
				Feature: args[0],
				DryRun:  dryRun,
				Root:    r,
			})
			return emitApproveResult(cmd, res, asJSON)
		},
	}
	approveCmd.Flags().StringVar(&root, "root", "", "project root (default: cwd)")
	approveCmd.Flags().BoolVar(&dryRun, "dry-run", true, "print plan without writing approved.json")
	approveCmd.Flags().BoolVar(&asJSON, "json", false, "emit machine-readable JSON")

	// run: run approved tests for a feature, or run tests from a spec file.
	var (
		runSpecPath    string
		runSpecFeature string
	)
	runCmd := &cobra.Command{
		Use:   "run [<feature>]",
		Short: "Run approved tests for a feature, or run tests from a spec file (--spec).",
		Long: strings.TrimSpace(`
Run approved tests for a named feature, OR execute the test families declared
in a spec file generated by 'forge test spec generate'.

  forge test run <feature>              run approved tests for the named feature
  forge test run --spec <path>          run families declared in a spec.yml
  forge test run --feature <name>       locate .forge/specs/<name>/spec.yml and run it
`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := projectRoot(root)
			if err != nil {
				return err
			}
			// Spec-driven path: --spec or --feature (for spec auto-path) with no positional arg.
			if runSpecPath != "" || (len(args) == 0 && runSpecFeature != "") {
				res := RunFromSpec(SpecRunOptions{
					SpecPath: runSpecPath,
					Feature:  runSpecFeature,
					RunOptions: RunOptions{
						Root:       r,
						DryRun:     dryRun,
						Workers:    workers,
						Duration:   duration,
						Timeout:    timeout,
						BenchCount: benchCount,
						FailFast:   failFast,
					},
				})
				return emitSpecRunResult(cmd, res, asJSON)
			}
			// Lifecycle path: positional <feature> arg.
			if len(args) == 0 {
				return cmd.Help()
			}
			res := RunFeatureTests(RunFeatureOptions{
				Feature:    args[0],
				Root:       r,
				DryRun:     dryRun,
				Workers:    workers,
				Duration:   duration,
				Timeout:    timeout,
				BenchCount: benchCount,
				FailFast:   failFast,
			})
			return emitRunFeatureResult(cmd, res, asJSON)
		},
	}
	runCmd.Flags().StringVar(&root, "root", "", "project root (default: cwd)")
	runCmd.Flags().BoolVar(&dryRun, "dry-run", true, "print plan without running tests")
	runCmd.Flags().IntVar(&workers, "workers", 10, "concurrent worker count (load/soak)")
	runCmd.Flags().StringVar(&duration, "duration", "1h", "test duration (soak/load)")
	runCmd.Flags().StringVar(&timeout, "timeout", "10m", "per-family timeout")
	runCmd.Flags().IntVar(&benchCount, "bench-count", 5, "benchmark iteration count (perf)")
	runCmd.Flags().BoolVar(&failFast, "fail-fast", false, "stop on first family failure")
	runCmd.Flags().BoolVar(&asJSON, "json", false, "emit machine-readable JSON")
	runCmd.Flags().StringVar(&runSpecPath, "spec", "", "path to spec.yml generated by `forge test spec generate`")
	runCmd.Flags().StringVar(&runSpecFeature, "feature", "", "feature slug — locates .forge/specs/<name>/spec.yml")

	// ci: trigger or guide CI/CD run for the feature on a non-prod env.
	ciCmd := &cobra.Command{
		Use:   "ci <feature>",
		Short: "Trigger or guide a CI/CD pipeline run for a feature on a non-prod environment.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := projectRoot(root)
			if err != nil {
				return err
			}
			res := RunCI(CIOptions{
				Feature:        args[0],
				Env:            env,
				DryRun:         dryRun,
				GenerateConfig: generateConfig,
				Root:           r,
			})
			return emitCIResult(cmd, res, asJSON)
		},
	}
	ciCmd.Flags().StringVar(&root, "root", "", "project root (default: cwd)")
	ciCmd.Flags().BoolVar(&dryRun, "dry-run", true, "print plan without triggering CI")
	ciCmd.Flags().StringVar(&env, "env", "staging", "target non-prod environment")
	ciCmd.Flags().BoolVar(&generateConfig, "generate-config", false, "generate CI config if none found")
	ciCmd.Flags().BoolVar(&asJSON, "json", false, "emit machine-readable JSON")

	parent.AddCommand(createCmd, approveCmd, runCmd, ciCmd, newSpecCmd())

	return parent
}

// ── Core logic ────────────────────────────────────────────────────────────────

// RunOptions holds resolved configuration for a test run.
type RunOptions struct {
	Root       string
	DryRun     bool
	Workers    int
	Duration   string
	Timeout    string
	BenchCount int
	FailFast   bool
}

// Run executes (or plans, in dry-run mode) the requested families and returns a
// consolidated TestResult. This is the testable core — no cobra I/O here.
func Run(families []Family, opts RunOptions) *TestResult {
	start := time.Now()
	res := &TestResult{
		DryRun:  opts.DryRun,
		Message: "MVP forge test is dry-run only; full subprocess invocation lands in M1",
	}

	for _, f := range families {
		fr := planFamily(f, opts)
		res.Families = append(res.Families, fr)
		switch fr.Status {
		case "ok":
			res.Passed++
		case "fail":
			res.Failed++
		case "skipped", "pending":
			res.Skipped++
		}
		if opts.FailFast && fr.Status == "fail" {
			break
		}
	}

	res.Duration = fmt.Sprintf("%d", time.Since(start).Milliseconds())
	res.Ready = res.Failed == 0
	if !res.Ready {
		res.Message = fmt.Sprintf("%d family(ies) failed; see details above", res.Failed)
	}
	return res
}

// planFamily returns the dry-run plan for one family.
func planFamily(f Family, opts RunOptions) FamilyResult {
	t0 := time.Now()
	switch f {
	case FamilyUnit:
		return FamilyResult{
			Family:     f,
			Status:     "pending",
			TestCount:  0,
			Detail:     fmt.Sprintf("will run: go test -race -count=1 ./... (root: %s)", opts.Root),
			DurationMs: time.Since(t0).Milliseconds(),
		}
	case FamilySmoke:
		return FamilyResult{
			Family:     f,
			Status:     "pending",
			Detail:     fmt.Sprintf("will run: go test -race -count=1 -run TestSmoke ./... (root: %s)", opts.Root),
			DurationMs: time.Since(t0).Milliseconds(),
		}
	case FamilyIntegration:
		return FamilyResult{
			Family:     f,
			Status:     "pending",
			Detail:     fmt.Sprintf("will run: go test -tags integration -race ./... (root: %s)", opts.Root),
			DurationMs: time.Since(t0).Milliseconds(),
		}
	case FamilyRegression:
		return FamilyResult{
			Family:     f,
			Status:     "pending",
			Detail:     fmt.Sprintf("will run: go test -tags regression -run TestRegression ./... (root: %s)", opts.Root),
			DurationMs: time.Since(t0).Milliseconds(),
		}
	case FamilyE2E:
		return FamilyResult{
			Family:     f,
			Status:     "pending",
			Detail:     fmt.Sprintf("will run: go test -tags e2e -timeout %s ./... (root: %s)", opts.Timeout, opts.Root),
			DurationMs: time.Since(t0).Milliseconds(),
		}
	case FamilyJourney:
		return FamilyResult{
			Family:     f,
			Status:     "pending",
			Detail:     fmt.Sprintf("will run: go test -run TestJourney -race -timeout %s ./internal/cli/... (root: %s)", opts.Timeout, opts.Root),
			DurationMs: time.Since(t0).Milliseconds(),
		}
	case FamilyContract:
		return FamilyResult{
			Family:     f,
			Status:     "pending",
			Detail:     fmt.Sprintf("will run: go test -tags contract -run TestContract ./... (root: %s)", opts.Root),
			DurationMs: time.Since(t0).Milliseconds(),
		}
	case FamilyPerf:
		return FamilyResult{
			Family:     f,
			Status:     "pending",
			Detail:     fmt.Sprintf("will run: go test -bench=. -benchmem -benchtime=%dx ./... (root: %s)", opts.BenchCount, opts.Root),
			DurationMs: time.Since(t0).Milliseconds(),
		}
	case FamilyLoad:
		return FamilyResult{
			Family:     f,
			Status:     "pending",
			Detail:     fmt.Sprintf("will run: go test -tags load -run TestLoad -timeout %s (workers: %d, root: %s)", opts.Duration, opts.Workers, opts.Root),
			DurationMs: time.Since(t0).Milliseconds(),
		}
	case FamilySoak:
		return FamilyResult{
			Family:     f,
			Status:     "pending",
			Detail:     fmt.Sprintf("will run: go test -tags soak -run TestSoak -timeout %s (workers: %d, root: %s)", opts.Duration, opts.Workers, opts.Root),
			DurationMs: time.Since(t0).Milliseconds(),
		}
	case FamilyChaos:
		return FamilyResult{
			Family:     f,
			Status:     "pending",
			Detail:     fmt.Sprintf("will run: go test -tags chaos -run TestChaos -timeout %s ./... (root: %s)", opts.Timeout, opts.Root),
			DurationMs: time.Since(t0).Milliseconds(),
		}
	case FamilyMutation:
		return FamilyResult{
			Family:     f,
			Status:     "pending",
			Detail:     fmt.Sprintf("will run: go-mutesting ./... (requires go-mutesting; root: %s)", opts.Root),
			DurationMs: time.Since(t0).Milliseconds(),
		}
	case FamilySnapshot:
		return FamilyResult{
			Family:     f,
			Status:     "pending",
			Detail:     fmt.Sprintf("will run: go test -tags snapshot -run TestSnapshot ./... (root: %s)", opts.Root),
			DurationMs: time.Since(t0).Milliseconds(),
		}
	default:
		return FamilyResult{
			Family:     f,
			Status:     "fail",
			Detail:     fmt.Sprintf("unknown family %q", f),
			DurationMs: time.Since(t0).Milliseconds(),
		}
	}
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func projectRoot(override string) (string, error) {
	if override != "" {
		return override, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", errcode.New(ErrTestFailed, "getwd", err)
	}
	return cwd, nil
}

func emit(cmd *cobra.Command, res *TestResult, asJSON bool) error {
	if asJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		if err := enc.Encode(res); err != nil {
			return errcode.New(ErrTestFailed, "encode JSON", err)
		}
	} else {
		renderText(cmd, res)
	}
	if !res.Ready {
		return errcode.New(ErrTestFailed, res.Message, nil)
	}
	return nil
}

func renderText(cmd *cobra.Command, r *TestResult) {
	w := cmd.OutOrStdout()
	mode := "dry-run"
	if !r.DryRun {
		mode = "live"
	}
	fmt.Fprintf(w, "forge test (%s)\n%s\n\n", mode, r.Message)

	for _, fr := range r.Families {
		icon := statusIcon(fr.Status)
		fmt.Fprintf(w, "  %s %-12s  %s\n", icon, fr.Family, fr.Detail)
	}

	fmt.Fprintf(w, "\npassed: %d  failed: %d  skipped: %d  (%s ms)\n",
		r.Passed, r.Failed, r.Skipped, r.Duration)

	if r.Ready {
		fmt.Fprintln(w, "\nAll requested test families passed.")
	} else {
		fmt.Fprintln(w, "\nTest run blocked by failure(s) above.")
	}
}

func statusIcon(s string) string {
	switch s {
	case "ok":
		return "✓"
	case "fail":
		return "✗"
	case "pending":
		return "⊘"
	default:
		return "–"
	}
}
