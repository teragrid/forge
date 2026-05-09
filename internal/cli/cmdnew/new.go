// Package cmdnew implements `forge new <template> <path>` (DEV-M0-13).
package cmdnew

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/teragrid/forge/internal/errcode"
	"github.com/teragrid/forge/internal/scaffold"
	"github.com/teragrid/forge/internal/verbmeta"
)

// Reserved error codes (range 1100..1199).
var (
	ErrUsage = errcode.Register(errcode.Code(1100), "invalid usage of `forge new`")
)

func init() {
	verbmeta.Register(verbmeta.Manifest{
		Verb:    "new",
		Summary: "Scaffold a new project from a built-in template.",
		Inputs: []string{
			"<template> (required; one of: " + strings.Join(scaffold.AvailableTemplates(), ", ") + ")",
			"<path> (required; target directory)",
			"--name (project name; default basename of <path>)",
			"--module (Go module path; default example.com/<name>)",
			"--force (overwrite into a non-empty target)",
			"--json (machine-readable output)",
		},
		Outputs: []string{
			"<path>/* — rendered template files",
			"stdout — created file list (text or JSON)",
		},
		SideEffects: []string{"creates <path> if missing", "writes files into <path>"},
		ErrorCodes:  []errcode.Code{ErrUsage, scaffold.ErrUnknownTemplate, scaffold.ErrTargetNotEmpty},
	})
}

// New returns the cobra command.
func New(forgeVersion string) *cobra.Command {
	var (
		name   string
		module string
		force  bool
		asJSON bool
	)
	cmd := &cobra.Command{
		Use:   "new <template> <path>",
		Short: "Scaffold a new project from a built-in template.",
		Long: "Renders a built-in template into <path>. Templates ship a managed " +
			".gitignore, .gitleaks.toml and .forge/manifest so the project starts " +
			"with the standard hygiene + secret-scan baseline.\n\nAvailable templates: " +
			strings.Join(scaffold.AvailableTemplates(), ", "),
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			tmpl, target := args[0], args[1]
			if tmpl == "" || target == "" {
				return errcode.New(ErrUsage, "template and path are both required", nil)
			}
			res, err := scaffold.Render(scaffold.Options{
				Template: tmpl,
				Target:   target,
				Vars: scaffold.Vars{
					Name:     name,
					Module:   module,
					ForgeVer: forgeVersion,
				},
				Force: force,
			})
			if err != nil {
				return err
			}
			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(res)
			}
			fmt.Fprintf(cmd.OutOrStdout(),
				"scaffolded %q into %s (%d files)\n", res.Template, res.Target, len(res.Files))
			for _, f := range res.Files {
				fmt.Fprintf(cmd.OutOrStdout(), "  + %s\n", f)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "\nNext: cd into the project and run its README's quickstart.")
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "project name (default: basename of <path>)")
	cmd.Flags().StringVar(&module, "module", "", "Go module path (default: example.com/<name>)")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite into a non-empty target")
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit machine-readable JSON")
	return cmd
}
