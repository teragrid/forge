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
// Package cli wires the cobra root command and registers all top-level verbs.
//
// Verbs live in subpackages (internal/cli/<verb>) and self-register via
// AddCommand from NewRootCommand. Keeping the wiring centralised makes the
// command tree trivially diff-able across PRs (per spec §4 Command Surface).
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/teragrid/forge/internal/cli/banner"
	"github.com/teragrid/forge/internal/cli/cmdadd"
	"github.com/teragrid/forge/internal/cli/cmdadopt"
	"github.com/teragrid/forge/internal/cli/cmdagents"
	"github.com/teragrid/forge/internal/cli/cmdask"
	"github.com/teragrid/forge/internal/cli/cmdaudit"
	"github.com/teragrid/forge/internal/cli/cmdbackup"
	"github.com/teragrid/forge/internal/cli/cmdbugfix"
	"github.com/teragrid/forge/internal/cli/cmdcheck"
	"github.com/teragrid/forge/internal/cli/cmdci"
	"github.com/teragrid/forge/internal/cli/cmdclean"
	"github.com/teragrid/forge/internal/cli/cmdcompanion"
	"github.com/teragrid/forge/internal/cli/cmdconfig"
	"github.com/teragrid/forge/internal/cli/cmdcontext"
	"github.com/teragrid/forge/internal/cli/cmddeploy"
	"github.com/teragrid/forge/internal/cli/cmddocs"
	"github.com/teragrid/forge/internal/cli/cmddoctor"
	"github.com/teragrid/forge/internal/cli/cmdeject"
	"github.com/teragrid/forge/internal/cli/cmdeval"
	"github.com/teragrid/forge/internal/cli/cmdexplain"
	"github.com/teragrid/forge/internal/cli/cmdfix"
	"github.com/teragrid/forge/internal/cli/cmdfixtures"
	"github.com/teragrid/forge/internal/cli/cmdgenerate"
	"github.com/teragrid/forge/internal/cli/cmdhygiene"
	"github.com/teragrid/forge/internal/cli/cmdincident"
	"github.com/teragrid/forge/internal/cli/cmdinit"
	"github.com/teragrid/forge/internal/cli/cmdinsights"
	"github.com/teragrid/forge/internal/cli/cmdlearn"
	"github.com/teragrid/forge/internal/cli/cmdlint"
	"github.com/teragrid/forge/internal/cli/cmdllm"
	"github.com/teragrid/forge/internal/cli/cmdmcp"
	"github.com/teragrid/forge/internal/cli/cmdmetrics"
	"github.com/teragrid/forge/internal/cli/cmdmigrate"
	"github.com/teragrid/forge/internal/cli/cmdnew"
	"github.com/teragrid/forge/internal/cli/cmdoptimize"
	"github.com/teragrid/forge/internal/cli/cmdplugin"
	"github.com/teragrid/forge/internal/cli/cmdpostmortem"
	"github.com/teragrid/forge/internal/cli/cmdreport"
	"github.com/teragrid/forge/internal/cli/cmdreview"
	"github.com/teragrid/forge/internal/cli/cmdscan"
	"github.com/teragrid/forge/internal/cli/cmdship"
	"github.com/teragrid/forge/internal/cli/cmdskill"
	"github.com/teragrid/forge/internal/cli/cmdsla"
	"github.com/teragrid/forge/internal/cli/cmdspend"
	"github.com/teragrid/forge/internal/cli/cmdtelemetry"
	"github.com/teragrid/forge/internal/cli/cmdtemplates"
	"github.com/teragrid/forge/internal/cli/cmdtest"
	"github.com/teragrid/forge/internal/cli/cmdtsd"
	"github.com/teragrid/forge/internal/cli/cmdundo"
	"github.com/teragrid/forge/internal/cli/cmdupgrade"
	"github.com/teragrid/forge/internal/cli/cmdversion"
	"github.com/teragrid/forge/internal/config"
	"github.com/teragrid/forge/internal/plugin"
)

// NewRootCommand returns the top-level `forge` cobra command. Wiring is kept
// minimal here so that adding a new verb is a one-line AddCommand call.
func NewRootCommand(version string) *cobra.Command {
	root := &cobra.Command{
		Use:   "forge",
		Short: "The LLM-native framework for shipping AI-generated code that survives production.",
		Long: `forge is a single-binary CLI that bundles the scan-fix-learn loop, ` +
			`LLM gateway, plugin runtime, and ship workflow described in ` +
			`docs/FORGE_FRAMEWORK_SPEC.md.`,
		Example: `  forge new ts-service my-app              # scaffold a production-grade TypeScript project
  forge scan all                           # check for secrets, vulnerabilities, and code issues
  forge ship                               # run the full 6-stage quality gate before pushing
  forge bugfix --bug "login fails with +"  # diagnose and auto-fix a bug with LLM assistance
  forge config set llm.model gpt-4o        # persist your default LLM model to forge.yml
  forge explain bugfix                     # learn what any command does, inputs, and side-effects
  forge test spec rate-limiter             # generate a 9-point YAML test spec for a feature
  forge test run --spec .forge/specs/rate-limiter/spec.yml  # run tests from the spec`,
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: false,
	}
	root.SetVersionTemplate("forge {{.Version}}\n")

	// Prepend the ASCII-art banner to the root help page only.
	// Subcommand help (e.g. `forge scan --help`) is unaffected.
	//
	// The usage template is reordered so that "Available Commands" appears
	// AFTER Flags. Without this, the long banner pushes the command list
	// above the terminal viewport, making it invisible without scrolling.
	root.SetUsageTemplate(`Usage:{{if .Runnable}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [command]{{end}}{{if gt (len .Aliases) 0}}

Aliases:
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

Examples:
{{.Example}}{{end}}{{if .HasAvailableLocalFlags}}

Flags:
{{.LocalFlags.FlagUsages | trimRightSpace}}{{end}}{{if .HasAvailableInheritedFlags}}

Global Flags:
{{.InheritedFlags.FlagUsages | trimRightSpace}}{{end}}{{if .HasAvailableSubCommands}}

{{range .Groups}}{{.Title}}
{{$gid := .ID}}{{range $.Commands}}{{if (and (eq .GroupID $gid) (or .IsAvailableCommand (eq .Name "help")))}}  {{rpad .Name .NamePadding }} {{.Short}}
{{end}}{{end}}
{{end}}{{range .Commands}}{{if (and (eq .GroupID "") (or .IsAvailableCommand (eq .Name "help")))}}  {{rpad .Name .NamePadding }} {{.Short}}
{{end}}{{end}}{{end}}{{if .HasHelpSubCommands}}

Additional help topics:{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{rpad .Name .NamePadding}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}

Use "{{.CommandPath}} [command] --help" for more information about a command.{{end}}
`)

	// Show the banner on every help invocation (root or subcommand).
	origHelp := root.HelpFunc()
	root.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		banner.Print(cmd.OutOrStdout())
		origHelp(cmd, args)
	})

	// Load any external plugins declared in .forge/plugins.json before
	// verbs run. Errors are silently swallowed (missing file is fine;
	// malformed JSON is logged to stderr so the user sees it but is not
	// blocked from running the CLI).
	root.PersistentPreRunE = func(cmd *cobra.Command, _ []string) error {
		// Resolve --profile before any verb runs.
		if profileName, _ := cmd.Root().PersistentFlags().GetString("profile"); profileName != "" {
			p, err := config.GetProfile(config.ProfileName(profileName))
			if err != nil {
				return err
			}
			config.SetActiveProfile(p)
		}
		// Bridge forge.yml llm.model → FORGE_COPILOT_MODEL when the env var is
		// not already set. This lets `forge config set llm.model <model>` persist
		// the default Copilot model without requiring a shell profile change.
		if os.Getenv("FORGE_COPILOT_MODEL") == "" {
			if wd, err := os.Getwd(); err == nil {
				if cfg, err := config.Load(wd, nil); err == nil && cfg.LLMModel.Raw != "" {
					_ = os.Setenv("FORGE_COPILOT_MODEL", cfg.LLMModel.Raw)
				}
			}
		}
		// Bridge --budget-usd → FORGE_BUDGET_USD env var so all LLM calls made
		// in this invocation respect the per-invocation cap.  0 means unlimited.
		if budgetUSD, _ := cmd.Root().PersistentFlags().GetFloat64("budget-usd"); budgetUSD > 0 {
			if os.Getenv("FORGE_BUDGET_USD") == "" {
				_ = os.Setenv("FORGE_BUDGET_USD", fmt.Sprintf("%.4f", budgetUSD))
			}
		}
		wd, err := os.Getwd()
		if err != nil {
			return nil // can't determine root; skip discovery
		}
		if _, err := plugin.Discover(wd); err != nil {
			cmd.PrintErrln("forge: plugin discovery warning:", err)
		}
		return nil
	}

	// ── Command groups (cobra 1.8+) — organise the help output into labelled
	// sections so the 50-command list is navigable at a glance.
	// GroupIDs must be registered before any command references them.
	const (
		groupCore     = "core"
		groupBuild    = "build"
		groupAnalysis = "analysis"
		groupOps      = "ops"
		groupAI       = "ai"
		groupConfig   = "config"
		groupAdvanced = "advanced"
	)
	root.AddGroup(
		&cobra.Group{ID: groupCore, Title: "Core Workflow:"},
		&cobra.Group{ID: groupBuild, Title: "Build & Generate:"},
		&cobra.Group{ID: groupAnalysis, Title: "Analysis & Insights:"},
		&cobra.Group{ID: groupOps, Title: "Deploy & Operations:"},
		&cobra.Group{ID: groupAI, Title: "AI & Automation:"},
		&cobra.Group{ID: groupConfig, Title: "Project & Configuration:"},
		&cobra.Group{ID: groupAdvanced, Title: "Advanced:"},
	)

	// Helper to assign a GroupID after construction (cobra Group field is not
	// settable via the factory, so we set it on the returned command pointer).
	inGroup := func(cmd *cobra.Command, id string) *cobra.Command {
		cmd.GroupID = id
		return cmd
	}

	root.AddCommand(
		// ── Core Workflow ─────────────────────────────────────────────────────
		inGroup(cmdship.New(), groupCore),
		inGroup(cmdscan.New(), groupCore),
		inGroup(cmdbugfix.New(), groupCore),
		inGroup(cmdtest.New(), groupCore),
		inGroup(cmdreview.New(), groupCore),

		// ── Build & Generate ──────────────────────────────────────────────────
		inGroup(cmdnew.New(version), groupBuild),
		inGroup(cmdgenerate.New(), groupBuild),
		inGroup(cmdadd.New(), groupBuild),
		inGroup(cmdupgrade.New(), groupBuild),
		inGroup(cmdfix.New(), groupBuild),
		inGroup(cmdmigrate.New(), groupBuild),

		// ── Analysis & Insights ───────────────────────────────────────────────
		inGroup(cmdinsights.New(), groupAnalysis),
		inGroup(cmdmetrics.New(), groupAnalysis),
		inGroup(cmdspend.New(), groupAnalysis),
		inGroup(cmdaudit.New(), groupAnalysis),
		inGroup(cmdeval.New(), groupAnalysis),
		inGroup(cmdreport.New(), groupAnalysis),
		inGroup(cmdsla.New(), groupAnalysis),
		inGroup(cmdlint.New(), groupAnalysis),

		// ── Deploy & Operations ───────────────────────────────────────────────
		inGroup(cmddeploy.New(), groupOps),
		inGroup(cmddeploy.NewRollback(), groupOps),
		inGroup(cmdci.New(), groupOps),
		inGroup(cmdundo.New(), groupOps),
		inGroup(cmdincident.New(), groupOps),
		inGroup(cmdpostmortem.New(), groupOps),
		inGroup(cmdbackup.New(), groupOps),
		inGroup(cmdoptimize.New(), groupOps),

		// ── AI & Automation ───────────────────────────────────────────────────
		inGroup(cmdcompanion.New(), groupAI),
		inGroup(cmdskill.New(), groupAI),
		inGroup(cmdagents.New(), groupAI),
		inGroup(cmdmcp.New(), groupAI),
		inGroup(cmdlearn.New(), groupAI),

		// ── Project & Configuration ───────────────────────────────────────────
		inGroup(cmdinit.New(version), groupConfig),
		inGroup(cmdconfig.New(), groupConfig),
		inGroup(cmdllm.New(), groupConfig),
		inGroup(cmdcontext.New(), groupConfig),
		inGroup(cmdask.New(), groupConfig),
		inGroup(cmddocs.New(), groupConfig),
		inGroup(cmdcheck.New(), groupConfig),
		inGroup(cmdhygiene.New(), groupConfig),
		inGroup(cmdtelemetry.New(), groupConfig),
		inGroup(cmdclean.New(), groupConfig),
		inGroup(cmddoctor.New(), groupConfig),

		// ── Advanced ──────────────────────────────────────────────────────────
		inGroup(cmdplugin.New(), groupAdvanced),
		inGroup(cmdtsd.New(), groupAdvanced),
		inGroup(cmdtemplates.New(), groupAdvanced),
		inGroup(cmdfixtures.New(), groupAdvanced),
		inGroup(cmdadopt.New(), groupAdvanced),
		inGroup(cmdeject.New(), groupAdvanced),
		inGroup(cmdexplain.New(), groupAdvanced),
		inGroup(cmdversion.New(version), groupAdvanced),
	)

	// Universal flags (G-080): every verb inherits these via PersistentFlags.
	// Child commands that define a same-named local flag shadow these silently
	// (pflag AddFlagSet skips duplicates), so there is no conflict.
	var (
		_globalJSON      bool
		_globalYes       bool
		_globalDryRun    bool
		_globalExplain   bool
		_globalWorkspace string
		_globalNoColor   bool
		_globalQuiet     bool
		_globalVerbose   string
		_globalProfile   string
		_globalBudgetUSD float64
	)
	pf := root.PersistentFlags()
	pf.BoolVar(&_globalJSON, "json", false, "Emit machine-readable NDJSON output")
	pf.BoolVarP(&_globalYes, "yes", "y", false, "Auto-approve all prompts")
	pf.BoolVar(&_globalDryRun, "dry-run", false, "Preview changes without side effects")
	pf.BoolVar(&_globalExplain, "explain", false, "Print LLM reasoning alongside output")
	pf.StringVar(&_globalWorkspace, "workspace", "", "Active workspace ID (overrides .forge/workspace)")
	pf.BoolVar(&_globalNoColor, "no-color", false, "Disable ANSI color output")
	pf.BoolVarP(&_globalQuiet, "quiet", "q", false, "Suppress non-essential output")
	pf.StringVar(&_globalVerbose, "verbose", "", "Log verbosity level (debug|info|warn)")
	pf.StringVar(&_globalProfile, "profile", "", "Behavior profile (fast|safe|paranoid): adjusts scan strictness, confidence threshold, and LLM token budget")
	pf.Float64Var(&_globalBudgetUSD, "budget-usd", 0, "per-invocation LLM spend cap in USD (0 = unlimited); sets FORGE_BUDGET_USD")

	// Silence the linter about intentionally unused persistent-flag bindings.
	// Commands read these via cmd.Root().PersistentFlags().GetBool/GetString.
	_ = _globalJSON
	_ = _globalYes
	_ = _globalDryRun
	_ = _globalExplain
	_ = _globalWorkspace
	_ = _globalNoColor
	_ = _globalQuiet
	_ = _globalVerbose
	_ = _globalProfile
	_ = _globalBudgetUSD

	// Register backward-compat aliases after all canonical commands are added.
	registerAliases(root)

	return root
}
