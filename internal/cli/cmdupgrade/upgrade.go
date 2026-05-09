// Package cmdupgrade implements `forge upgrade` (M2 codemod runner).
package cmdupgrade

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/teragrid/forge/internal/codemod"
	"github.com/teragrid/forge/internal/errcode"
	"github.com/teragrid/forge/internal/verbmeta"
)

// Reserved error codes (range 3300..3399).
var (
	ErrUnknownCodemod = errcode.Register(errcode.Code(3300), "unknown codemod")
	ErrUpgradeFailed  = errcode.Register(errcode.Code(3301), "upgrade codemod failed")
)

func init() {
	verbmeta.Register(verbmeta.Manifest{
		Verb:    "upgrade",
		Summary: "Run a codemod to upgrade managed files (gitignore, gitleaks baseline, …).",
		Inputs: []string{
			"<codemod-name> (required; or 'list' to enumerate)",
			"--root <path> (default cwd)",
			"--apply (apply changes; default is dry-run)",
			"--json",
		},
		Outputs:      []string{"stdout: codemod report (text or JSON)"},
		SideEffects:  []string{"--apply mutates project files (codemod-specific)"},
		GatesTouched: []string{"§16.5.4 #11 — repo hygiene"},
		ErrorCodes:   []errcode.Code{ErrUnknownCodemod, ErrUpgradeFailed},
	})
}

// New returns the cobra command.
func New() *cobra.Command {
	var (
		root   string
		apply  bool
		asJSON bool
	)
	cmd := &cobra.Command{
		Use:   "upgrade <codemod>",
		Short: "Run a codemod (default: dry-run).",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			if name == "list" {
				return listCodemods(cmd, asJSON)
			}

			if root == "" {
				cwd, err := os.Getwd()
				if err != nil {
					return errcode.New(ErrUpgradeFailed, "getwd", err)
				}
				root = cwd
			}

			c, ok := codemod.Default().Lookup(name)
			if !ok {
				known := []string{}
				for _, k := range codemod.Default().All() {
					known = append(known, k.Name())
				}
				return errcode.Newf(ErrUnknownCodemod, nil, "unknown codemod %q; known: %v", name, known)
			}
			rep, err := c.Apply(root, !apply)
			if err != nil {
				return errcode.Newf(ErrUpgradeFailed, err, "codemod %s", name)
			}
			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(rep)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "forge upgrade %s (dry_run=%v)\n", rep.Codemod, rep.DryRun)
			fmt.Fprintf(cmd.OutOrStdout(), "changed: %d file(s)\n", rep.Changed)
			for _, f := range rep.Files {
				fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", f)
			}
			if rep.Detail != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "detail:  %s\n", rep.Detail)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&root, "root", "", "project root (default: cwd)")
	cmd.Flags().BoolVar(&apply, "apply", false, "apply changes (default: dry-run)")
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit machine-readable JSON")
	return cmd
}

func listCodemods(cmd *cobra.Command, asJSON bool) error {
	all := codemod.Default().All()
	if asJSON {
		out := make([]map[string]string, 0, len(all))
		for _, c := range all {
			out = append(out, map[string]string{"name": c.Name(), "description": c.Description()})
		}
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Codemods (%d):\n", len(all))
	for _, c := range all {
		fmt.Fprintf(cmd.OutOrStdout(), "  %-20s %s\n", c.Name(), c.Description())
	}
	return nil
}
