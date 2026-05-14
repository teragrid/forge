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

// Package cmdfixtures implements `forge fixtures` (spec §4, namespace 3 — Generate).
//
// forge fixtures is a focused ergonomic shortcut for generating JSON test
// fixture files in the current project. It delegates to the same template
// engine as `forge generate fixture <name>` but with a more discoverable
// entry-point for the "Generate" mental model.
//
// Usage:
//
//	forge fixtures <name> [--apply] [--output-dir <dir>] [--json]
//
// Equivalent to:
//
//	forge generate fixture <name> [--apply] [--output-dir <dir>] [--json]
package cmdfixtures

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/teragrid/forge/internal/errcode"
	"github.com/teragrid/forge/internal/verbmeta"
)

// Reserved error codes (range 6000..6099).
var (
	ErrFixturesFailed = errcode.Register(errcode.Code(6000), "fixtures generation failed")
)

func init() {
	verbmeta.Register(verbmeta.Manifest{
		Verb:    "fixtures",
		Summary: "Generate JSON test fixture files (spec §4, namespace 3 — Generate).",
		Inputs: []string{
			"<name>: fixture identifier",
			"--output-dir <dir> — override output directory (default: tests/fixtures/)",
			"--apply            — write files to disk (default: dry-run)",
			"--json             — machine-readable output",
		},
		Outputs:      []string{"stdout: generated fixture file path(s)"},
		SideEffects:  []string{"with --apply: writes JSON fixture file to tests/fixtures/"},
		GatesTouched: []string{"§4 generate"},
		ErrorCodes:   []errcode.Code{ErrFixturesFailed},
	})
}

// FixtureResult is the JSON output for one fixtures run.
type FixtureResult struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	Mode      string `json:"mode"` // "dry-run" | "apply"
	Created   bool   `json:"created"`
	Timestamp string `json:"timestamp"`
}

// New returns the cobra command for `forge fixtures`.
func New() *cobra.Command {
	var (
		outputDir string
		apply     bool
		asJSON    bool
	)
	cmd := &cobra.Command{
		Use:   "fixtures <name>",
		Short: "Generate a JSON test fixture file.",
		Long: "Generates a seed JSON fixture file in tests/fixtures/ for the named entity.\n\n" +
			"Safe by default: --apply is required to write to disk.\n\n" +
			"This is a focused shortcut for `forge generate fixture <name>`.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			cwd, err := os.Getwd()
			if err != nil {
				return errcode.New(ErrFixturesFailed, "getwd", err)
			}

			dir := outputDir
			if dir == "" {
				dir = filepath.Join(cwd, "tests", "fixtures")
			} else {
				if !filepath.IsAbs(dir) {
					dir = filepath.Join(cwd, dir)
				}
			}

			filename := name + ".json"
			fullPath := filepath.Join(dir, filename)
			relPath, _ := filepath.Rel(cwd, fullPath)

			mode := "dry-run"
			created := false

			if apply {
				mode = "apply"
				if err := os.MkdirAll(dir, 0o750); err != nil {
					return errcode.New(ErrFixturesFailed, "mkdir", err)
				}
				if _, err := os.Stat(fullPath); err == nil {
					// File already exists — skip without error.
					if asJSON {
						return printJSON(cmd, name, relPath, mode, false)
					}
					fmt.Fprintf(cmd.OutOrStdout(), "  skip   %s (already exists)\n", relPath)
					return nil
				}
				content := fixtureContent(name)
				if err := os.WriteFile(fullPath, []byte(content), 0o600); err != nil {
					return errcode.New(ErrFixturesFailed, "write", err)
				}
				created = true
			}

			if asJSON {
				return printJSON(cmd, name, relPath, mode, created)
			}

			if apply {
				fmt.Fprintf(cmd.OutOrStdout(), "  create %s\n", relPath)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "  (dry-run) would create %s\n", relPath)
				fmt.Fprintf(cmd.OutOrStdout(), "  Re-run with --apply to write the file.\n")
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&outputDir, "output-dir", "", "override output directory (default: tests/fixtures/)")
	cmd.Flags().BoolVar(&apply, "apply", false, "write the fixture file to disk")
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit machine-readable JSON")
	return cmd
}

func printJSON(cmd *cobra.Command, name, path, mode string, created bool) error {
	r := FixtureResult{
		Name:      name,
		Path:      path,
		Mode:      mode,
		Created:   created,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}

// fixtureContent returns a minimal seed JSON fixture for the named entity.
func fixtureContent(name string) string {
	return fmt.Sprintf(`{
  "_fixture": %q,
  "_generated_by": "forge fixtures",
  "items": [
    {
      "id": "00000000-0000-0000-0000-000000000001",
      "name": "example-%s",
      "created_at": "2026-01-01T00:00:00Z"
    }
  ]
}
`, name, name)
}
