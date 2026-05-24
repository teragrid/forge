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
// Package cmdnew implements `forge new <template> <path>` (DEV-M0-13).
package cmdnew

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/teragrid/forge/internal/errcode"
	"github.com/teragrid/forge/internal/knowledge"
	"github.com/teragrid/forge/internal/scaffold"
	"github.com/teragrid/forge/internal/tsd"
	"github.com/teragrid/forge/internal/verbmeta"
)

// defaultTSDPath is auto-detected when --tsd is not specified.
const defaultTSDPath = ".forge/tsd.yml"

// Reserved error codes (range 1100..1199).
var (
	ErrUsage = errcode.Register(errcode.Code(1100), "invalid usage of `forge new`")
)

func init() {
	verbmeta.Register(verbmeta.Manifest{
		Verb:    "new",
		Summary: "Scaffold a new project from a built-in template.",
		Inputs: []string{
			"<description> (TSD mode: feature description; template + path in classic mode)",
			"<template> (classic mode only; one of: " + strings.Join(scaffold.AvailableTemplates(), ", ") + ")",
			"<path> (classic mode only; target directory)",
			"--tsd <file> (TSD file; default: .forge/tsd.yml if present)",
			"--name (project name; default basename of <path>)",
			"--module (Go module path; default example.com/<name>)",
			"--force (overwrite into a non-empty target)",
			"--list (list available templates and exit)",
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
		name     string
		module   string
		force    bool
		asJSON   bool
		listOnly bool
		tsdPath  string
	)
	cmd := &cobra.Command{
		Use:   "new [<template> <path> | <description>]",
		Short: "Scaffold a new project from a template or TSD file.",
		Long: "Two modes:\n\n" +
			"  TSD mode (recommended):\n" +
			"    forge new \"<description>\"\n" +
			"    Reads .forge/tsd.yml automatically. Override with --tsd <file>.\n\n" +
			"  Classic mode:\n" +
			"    forge new <template> <path>\n" +
			"    Uses a built-in template directly (no TSD file needed).\n\n" +
			"Available templates: " + strings.Join(scaffold.AvailableTemplates(), ", "),
		Args: func(cmd *cobra.Command, args []string) error {
			if listOnly {
				return nil
			}
			if tsdModeActive(tsdPath) {
				// TSD mode: require at least a description; optional output path
				if len(args) < 1 {
					return errcode.New(ErrUsage,
						`TSD mode requires a description: forge new "<description>"`, nil)
				}
				return nil
			}
			return cobra.ExactArgs(2)(cmd, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if listOnly {
				templates := scaffold.AvailableTemplates()
				if asJSON {
					enc := json.NewEncoder(cmd.OutOrStdout())
					enc.SetIndent("", "  ")
					return enc.Encode(map[string][]string{"templates": templates})
				}
				fmt.Fprintln(cmd.OutOrStdout(), "Available templates:")
				for _, t := range templates {
					fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", t)
				}
				return nil
			}

			// TSD mode: auto-detect .forge/tsd.yml or use --tsd value.
			if resolved := resolvedTSDPath(tsdPath); resolved != "" {
				description := args[0]
				outDir := ""
				if len(args) > 1 {
					outDir = args[1]
				}
				return runTSDMode(cmd, resolved, description, outDir, tsdPath != "")
			}

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
			// ADR-026: show relevant knowledge-base refs for the chosen template.
			if idx, kbErr := knowledge.Load(); kbErr == nil {
				entries := knowledge.SelectForNew(idx, tmpl)
				if len(entries) > 0 {
					fmt.Fprintln(cmd.OutOrStdout(), "\nRelated knowledge:")
					for _, e := range entries {
						if e.Intent != "" {
							fmt.Fprintf(cmd.OutOrStdout(), "  - %s: %s\n", e.ID, e.Intent)
						} else {
							fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", e.ID)
						}
					}
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&tsdPath, "tsd", "t", "", "path to TSD file (auto-detects .forge/tsd.yml when omitted)")
	cmd.Flags().StringVarP(&name, "name", "n", "", "project name (default: basename of <path>)")
	cmd.Flags().StringVar(&module, "module", "", "Go module path (default: example.com/<name>)")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "overwrite into a non-empty target")
	cmd.Flags().BoolVarP(&asJSON, "json", "j", false, "emit machine-readable JSON")
	cmd.Flags().BoolVarP(&listOnly, "list", "l", false, "list available templates and exit")
	return cmd
}

// tsdModeActive returns true when TSD mode should be used:
// either --tsd was explicitly set, or .forge/tsd.yml is present in the cwd.
func tsdModeActive(flagValue string) bool {
	if flagValue != "" {
		return true
	}
	_, err := os.Stat(defaultTSDPath)
	return err == nil
}

// resolvedTSDPath returns the TSD file to use, or "" if not in TSD mode.
func resolvedTSDPath(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	if _, err := os.Stat(defaultTSDPath); err == nil {
		return defaultTSDPath
	}
	return ""
}

// runTSDMode handles `forge new` when a TSD file is in play.
// explicit is true when --tsd was provided directly by the user.
func runTSDMode(cmd *cobra.Command, tsdFile, description, outDir string, explicit bool) error {
	// Verify the file exists before doing anything.
	if _, err := os.Stat(tsdFile); err != nil {
		return errcode.New(ErrUsage,
			fmt.Sprintf("TSD file not found: %s", tsdFile), err)
	}

	// 1. Parse.
	t, err := tsd.ParseFile(tsdFile)
	if err != nil {
		return errcode.New(ErrUsage, "TSD parse failed", err)
	}

	// 2. Validate — abort before writing any files.
	valErrs := tsd.Validate(t)
	if len(valErrs) > 0 {
		fmt.Fprintf(cmd.ErrOrStderr(), "tsd: %d validation error(s):\n", len(valErrs))
		for _, e := range valErrs {
			fmt.Fprintf(cmd.ErrOrStderr(), "  %s\n", e.Error())
		}
		return errcode.Newf(ErrUsage, nil, "TSD validation failed; fix errors above")
	}
	for _, k := range t.UnknownKeys {
		fmt.Fprintf(cmd.OutOrStdout(), "  warning: unknown TSD key %q (ignored)\n", k)
	}

	// 3. Resolve modules.
	modules, err := tsd.Resolve(t)
	if err != nil {
		return errcode.New(ErrUsage, "TSD module resolution failed", err)
	}

	source := "auto-detected"
	if explicit {
		source = "specified via --tsd"
	}
	fmt.Fprintf(cmd.OutOrStdout(), "TSD file: %s (%s)\n", tsdFile, source)
	fmt.Fprintf(cmd.OutOrStdout(), "Description: %s\n", description)
	fmt.Fprintf(cmd.OutOrStdout(), "Modules (%d): %s\n", len(modules), strings.Join(modules, ", "))

	// 4. Compose scaffold modules.
	// SkipMissing: scaffold dirs are populated incrementally; missing ones
	// produce a warning rather than a hard error so the command is always usable.
	composed, err := scaffold.Compose(modules, scaffold.CompositionOptions{SkipMissing: true})
	if err != nil {
		return errcode.New(ErrUsage, "scaffold composition failed", err)
	}
	for _, missing := range composed.MissingModules {
		fmt.Fprintf(cmd.OutOrStdout(), "  warning: module scaffold not yet available: %q (skipped)\n", missing)
	}

	// 5. Determine output directory.
	if outDir == "" {
		outDir = t.Project.Name
		if outDir == "" {
			outDir = "."
		}
	}

	// 6. Render template vars into file contents.
	name := t.Project.Name
	if name == "" {
		name = outDir
	}
	vars := scaffold.Vars{
		Name:         name,
		Module:       "example.com/" + name,
		Description:  description,
		Domain:       t.Project.Domain,
		AuthProvider: t.Stack.Backend.Auth,
		DBPrimary:    t.Stack.Database.Primary,
		Cloud:        t.Stack.Infra.Cloud,
		CIProvider:   t.Stack.Infra.CICD,
	}
	rendered := &scaffold.ComposedResult{Files: make(map[string][]byte)}
	for rel, body := range composed.Files {
		if strings.HasSuffix(rel, ".tmpl") {
			rendered.Files[strings.TrimSuffix(rel, ".tmpl")] = body
		} else {
			rendered.Files[rel] = body
		}
	}
	_ = vars // vars will be used in Phase 3 template rendering; composition is the P2 scope

	// 7. Ensure target directory exists.
	if err := os.MkdirAll(outDir, 0o755); err != nil { //nolint:gosec
		return errcode.New(ErrUsage, "create output dir", err)
	}

	// 8. Write to disk.
	written, err := scaffold.WriteComposed(rendered, outDir)
	if err != nil {
		return err
	}

	if len(written) > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "\nScaffolded %d file(s) into %s:\n", len(written), outDir)
		for _, f := range written {
			fmt.Fprintf(cmd.OutOrStdout(), "  + %s\n", f)
		}
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "\nNo module scaffold files found yet (add scaffold dirs to forge-knowledge/templates/).\n")
	}

	// 9. Show relevant KB refs.
	if idx, kbErr := knowledge.Load(); kbErr == nil {
		entries := knowledge.SelectForNew(idx, "tsd")
		if len(entries) > 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "\nRelated knowledge:")
			for _, e := range entries {
				if e.Intent != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "  - %s: %s\n", e.ID, e.Intent)
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", e.ID)
				}
			}
		}
	}
	return nil
}
