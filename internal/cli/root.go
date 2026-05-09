// Package cli wires the cobra root command and registers all top-level verbs.
//
// Verbs live in subpackages (internal/cli/<verb>) and self-register via
// AddCommand from NewRootCommand. Keeping the wiring centralised makes the
// command tree trivially diff-able across PRs (per spec §4 Command Surface).
package cli

import (
	"github.com/spf13/cobra"
)

// NewRootCommand returns the top-level `forge` cobra command. Wiring is kept
// minimal here so that adding a new verb is a one-line AddCommand call.
func NewRootCommand(version string) *cobra.Command {
	root := &cobra.Command{
		Use:   "forge",
		Short: "The LLM-native framework for shipping AI-generated code that survives production.",
		Long: `forge is a single-binary CLI that bundles the scan-fix-learn loop, ` +
			`LLM gateway, plugin runtime, and ship workflow described in ` +
			`FORGE_FRAMEWORK_SPEC.md.`,
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: false,
	}

	root.SetVersionTemplate("forge {{.Version}}\n")

	// Future verbs (new, doctor, explain, ship, scan, ...) register here.
	// Tracked in tasks/DEVELOPMENT_TASKS.md DEV-M0-* and DEV-M1-*.

	return root
}
