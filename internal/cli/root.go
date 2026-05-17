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
	"os"

	"github.com/spf13/cobra"

	"github.com/teragrid/forge/internal/cli/banner"
	"github.com/teragrid/forge/internal/cli/cmdadd"
	"github.com/teragrid/forge/internal/cli/cmdadopt"
	"github.com/teragrid/forge/internal/cli/cmdagents"
	"github.com/teragrid/forge/internal/cli/cmdask"
	"github.com/teragrid/forge/internal/cli/cmdaudit"
	"github.com/teragrid/forge/internal/cli/cmdbackup"
	"github.com/teragrid/forge/internal/cli/cmdcheck"
	"github.com/teragrid/forge/internal/cli/cmdci"
	"github.com/teragrid/forge/internal/cli/cmdclean"
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
	"github.com/teragrid/forge/internal/cli/cmdmigrate"
	"github.com/teragrid/forge/internal/cli/cmdnew"
	"github.com/teragrid/forge/internal/cli/cmdoptimize"
	"github.com/teragrid/forge/internal/cli/cmdplugin"
	"github.com/teragrid/forge/internal/cli/cmdpostmortem"
	"github.com/teragrid/forge/internal/cli/cmdreport"
	"github.com/teragrid/forge/internal/cli/cmdreview"
	"github.com/teragrid/forge/internal/cli/cmdscan"
	"github.com/teragrid/forge/internal/cli/cmdship"
	"github.com/teragrid/forge/internal/cli/cmdsla"
	"github.com/teragrid/forge/internal/cli/cmdspend"
	"github.com/teragrid/forge/internal/cli/cmdtelemetry"
	"github.com/teragrid/forge/internal/cli/cmdtest"
	"github.com/teragrid/forge/internal/cli/cmdundo"
	"github.com/teragrid/forge/internal/cli/cmdupgrade"
	"github.com/teragrid/forge/internal/cli/cmdversion"
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

Available Commands:{{range .Commands}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if .HasHelpSubCommands}}

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
		wd, err := os.Getwd()
		if err != nil {
			return nil // can't determine root; skip discovery
		}
		if _, err := plugin.Discover(wd); err != nil {
			cmd.PrintErrln("forge: plugin discovery warning:", err)
		}
		return nil
	}

	root.AddCommand(
		cmdversion.New(version),
		cmdnew.New(version),
		cmddoctor.New(),
		cmdclean.New(),
		cmdexplain.New(),
		cmdscan.New(),
		cmdlint.New(),
		cmdship.New(),
		cmdtest.New(),
		cmdupgrade.New(),
		cmdaudit.New(),
		cmdplugin.New(),
		cmdeval.New(),
		cmdpostmortem.New(),
		cmdinsights.New(),
		cmdspend.New(),
		cmdincident.New(),
		cmdtelemetry.New(),
		// New verbs: spec §4 gap fill
		cmdhygiene.New(),
		cmdgenerate.New(),
		cmdmigrate.New(),
		cmdcheck.New(),
		cmdfix.New(),
		cmdadopt.New(),
		cmdeject.New(),
		cmdreview.New(),
		cmdcontext.New(),
		cmdask.New(),
		cmddocs.New(),
		cmdinit.New(version),
		cmdconfig.New(),
		// M2 verbs
		cmdlearn.New(),
		cmddeploy.New(),
		cmddeploy.NewRollback(),
		cmdagents.New(),
		// M3 verbs
		cmdundo.New(),
		cmdoptimize.New(),
		cmdadd.New(),
		cmdreport.New(),
		cmdsla.New(),
		// spec §4 gap-fill: fixtures + backup
		cmdfixtures.New(),
		cmdbackup.New(),
		// DEV-M3-31: post-push CI monitor
		cmdci.New(),
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
	pf.StringVar(&_globalProfile, "profile", "", "Named credentials profile to load")

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

	// Register backward-compat aliases after all canonical commands are added.
	registerAliases(root)

	return root
}
