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

// Package cmdinit implements `forge init` — idiomatic project initialisation
// for the current directory (analogous to `git init` / `npm init`).
//
// Auto-detects the template to apply:
//   - package.json present  → ts-service
//   - go.mod present        → go-service
//   - (default)             → ts-service
//
// For polyglot monorepos both files may be present; --template overrides.
package cmdinit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/teragrid/forge/internal/errcode"
	"github.com/teragrid/forge/internal/scaffold"
	"github.com/teragrid/forge/internal/verbmeta"
)

// Reserved error codes (range 5100..5199).
var (
	ErrInitFailed = errcode.Register(errcode.Code(5100), "forge init failed")
	ErrInitUsage  = errcode.Register(errcode.Code(5101), "invalid usage of `forge init`")
)

func init() {
	verbmeta.Register(verbmeta.Manifest{
		Verb: "init",
		Summary: "Initialise the current directory as a Forge project (like git init / npm init).\n" +
			"Auto-detects the best template from existing files.",
		Inputs: []string{
			"--template <name>  (override auto-detection; default: auto)",
			"--name <name>      (project name; default: current directory name)",
			"--force            (overwrite files that already exist)",
			"--json             (machine-readable output)",
		},
		Outputs: []string{
			"<cwd>/* — rendered template files",
			"stdout  — created file list (text or JSON)",
		},
		SideEffects:  []string{"writes Forge scaffolding into the current directory"},
		GatesTouched: []string{"§4 scaffold"},
		ErrorCodes:   []errcode.Code{ErrInitFailed, ErrInitUsage},
	})
}

// New returns the cobra command for `forge init`.
func New(forgeVersion string) *cobra.Command {
	var (
		tmplFlag string
		name     string
		force    bool
		asJSON   bool
	)

	cmd := &cobra.Command{
		Use:   "init [path]",
		Short: "Initialise a directory as a Forge project.",
		Long: "forge init applies a scaffold template to a directory (default: current directory).\n\n" +
			"Template auto-detection order:\n" +
			"  1. --template flag\n" +
			"  2. package.json detected → ts-service\n" +
			"  3. go.mod detected       → go-service\n" +
			"  4. default               → ts-service\n\n" +
			"Available templates: " + strings.Join(scaffold.AvailableTemplates(), ", "),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var target string
			if len(args) == 1 && args[0] != "" {
				abs, err := filepath.Abs(args[0])
				if err != nil {
					return errcode.New(ErrInitFailed, "cannot resolve path", err)
				}
				target = abs
			} else {
				cwd, err := os.Getwd()
				if err != nil {
					return errcode.New(ErrInitFailed, "cannot determine current directory", err)
				}
				target = cwd
			}

			// Determine project name.
			if name == "" {
				name = filepath.Base(target)
			}

			// Auto-detect template if not explicitly provided.
			tmpl := tmplFlag
			if tmpl == "" {
				tmpl = detectTemplate(target)
			}

			// Validate the template exists.
			available := scaffold.AvailableTemplates()
			if !contains(available, tmpl) {
				return errcode.New(ErrInitUsage,
					fmt.Sprintf("unknown template %q; available: %s", tmpl, strings.Join(available, ", ")),
					nil)
			}

			res, err := scaffold.Render(scaffold.Options{
				Template: tmpl,
				Target:   target,
				Vars: scaffold.Vars{
					Name:     name,
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
				"initialised %q with template %q (%d files)\n", res.Target, res.Template, len(res.Files))
			for _, f := range res.Files {
				fmt.Fprintf(cmd.OutOrStdout(), "  + %s\n", f)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "\nNext steps:")
			switch tmpl {
			case "ts-service":
				fmt.Fprintln(cmd.OutOrStdout(), "  npm install")
				fmt.Fprintln(cmd.OutOrStdout(), "  npm run dev")
			case "go-service":
				fmt.Fprintln(cmd.OutOrStdout(), "  go mod tidy")
				fmt.Fprintln(cmd.OutOrStdout(), "  go test ./...")
			default:
				fmt.Fprintln(cmd.OutOrStdout(), "  forge doctor")
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&tmplFlag, "template", "", "template to apply (default: auto-detect)")
	cmd.Flags().StringVar(&name, "name", "", "project name (default: current directory name)")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite files that already exist")
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit machine-readable JSON")
	return cmd
}

// detectTemplate infers the best-fit template from files already in dir.
func detectTemplate(dir string) string {
	if fileExists(filepath.Join(dir, "package.json")) {
		return "ts-service"
	}
	if fileExists(filepath.Join(dir, "go.mod")) {
		return "go-service"
	}
	return "ts-service"
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func contains(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}
