// Package cli wires the cobra root command and registers all top-level verbs.
//
// Verbs live in subpackages (internal/cli/<verb>) and self-register via
// AddCommand from NewRootCommand. Keeping the wiring centralised makes the
// command tree trivially diff-able across PRs (per spec §4 Command Surface).
package cli

import (
	"github.com/spf13/cobra"

	"github.com/teragrid/forge/internal/cli/cmdaudit"
	"github.com/teragrid/forge/internal/cli/cmdclean"
	"github.com/teragrid/forge/internal/cli/cmddoctor"
	"github.com/teragrid/forge/internal/cli/cmdexplain"
	"github.com/teragrid/forge/internal/cli/cmdlint"
	"github.com/teragrid/forge/internal/cli/cmdnew"
	"github.com/teragrid/forge/internal/cli/cmdscan"
	"github.com/teragrid/forge/internal/cli/cmdship"
	"github.com/teragrid/forge/internal/cli/cmdupgrade"
	"github.com/teragrid/forge/internal/cli/cmdversion"
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

	root.AddCommand(
		cmdversion.New(version),
		cmdnew.New(version),
		cmddoctor.New(),
		cmdclean.New(),
		cmdexplain.New(),
		cmdscan.New(),
		cmdlint.New(),
		cmdship.New(),
		cmdupgrade.New(),
		cmdaudit.New(),
	)

	return root
}
