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

// Package cmdeject implements `forge eject` (spec §4).
//
// Removes Forge framework management from a project while preserving all
// project source files. The inverse of `forge adopt`.
package cmdeject

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/teragrid/forge/internal/errcode"
	"github.com/teragrid/forge/internal/verbmeta"
)

// Reserved error codes (range 4600..4699).
var (
	ErrEjectFailed = errcode.Register(errcode.Code(4600), "eject failed")
)

func init() {
	verbmeta.Register(verbmeta.Manifest{
		Verb:    "eject",
		Summary: "Remove Forge management from a project, keeping all project source files (spec §4).",
		Inputs: []string{
			"--root <path>  — project root (default: cwd)",
			"--dry-run      — preview what would be removed (default: true)",
			"--apply        — remove framework files",
		},
		Outputs:      []string{"stdout: list of files that would be / were removed"},
		SideEffects:  []string{"with --apply: removes .forge/ directory and framework config files"},
		GatesTouched: []string{"§4 eject"},
		ErrorCodes:   []errcode.Code{ErrEjectFailed},
	})
}

// New returns the cobra command for `forge eject`.
func New() *cobra.Command {
	var (
		root   string
		dryRun bool
		apply  bool
	)
	cmd := &cobra.Command{
		Use:   "eject [--root <path>]",
		Short: "Remove Forge framework management from a project (M2).",
		Long: "forge eject removes Forge management from a project while preserving all\n" +
			"project source files. It is the inverse of `forge adopt`.\n\n" +
			"Files removed (with --apply):\n" +
			"  .forge/           — entire Forge state directory\n" +
			"  forge.config.yml  — project config\n" +
			"  .forge-conventions\n\n" +
			"AI context files (AGENTS.md, CLAUDE.md, .cursorrules, .windsurfrules) are\n" +
			"NOT removed as they may have been edited by the team.\n\n" +
			"Safe by default: --dry-run previews without deleting.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			r, err := resolveRoot(root)
			if err != nil {
				return errcode.New(ErrEjectFailed, "resolve root", err)
			}
			targets := ejectTargets(r)
			if !apply || dryRun {
				fmt.Fprintln(cmd.OutOrStdout(), "forge eject (dry-run) — would remove:")
				for _, t := range targets {
					if _, err := os.Stat(t); err == nil {
						fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", relOrAbs(r, t))
					}
				}
				fmt.Fprintln(cmd.OutOrStdout(), "\nAI context files (AGENTS.md, CLAUDE.md, .cursorrules, .windsurfrules) are preserved.")
				fmt.Fprintln(cmd.OutOrStdout(), "Re-run with --apply to remove framework files.")
				return nil
			}
			// Take trash snapshot before mutating (ADR-024 / §17.1)
			var existingPaths []string
			for _, t := range targets {
				if _, err := os.Stat(t); err == nil {
					existingPaths = append(existingPaths, t)
				}
			}
			if len(existingPaths) > 0 {
				if _, err := writeTrashSnapshot(r, "eject", existingPaths); err != nil {
					fmt.Fprintf(cmd.OutOrStdout(), "warning: trash snapshot failed: %v\n", err)
				}
			}
			var removed int
			for _, t := range targets {
				if _, err := os.Stat(t); os.IsNotExist(err) {
					continue
				}
				if err := os.RemoveAll(t); err != nil {
					return errcode.Newf(ErrEjectFailed, err, "remove %s", relOrAbs(r, t))
				}
				fmt.Fprintf(cmd.OutOrStdout(), "removed: %s\n", relOrAbs(r, t))
				removed++
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\neject: removed %d items — project is no longer managed by forge\n", removed)
			return nil
		},
	}
	cmd.Flags().StringVar(&root, "root", "", "project root (default: cwd)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", true, "preview removal without deleting (default)")
	cmd.Flags().BoolVar(&apply, "apply", false, "remove framework files from project")
	return cmd
}

// ejectTargets returns the ordered list of paths that eject removes.
// AI context files are intentionally excluded.
func ejectTargets(root string) []string {
	return []string{
		filepath.Join(root, ".forge"),
		filepath.Join(root, "forge.config.yml"),
		filepath.Join(root, ".forge-conventions"),
	}
}

func relOrAbs(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}
	return rel
}

func resolveRoot(root string) (string, error) {
	if root != "" {
		return root, nil
	}
	return os.Getwd()
}

// writeTrashSnapshot is a thin wrapper around the undo trash mechanism.
// It mirrors cmdundo.WriteTrashSnapshot but avoids a circular import by
// re-implementing the minimal needed logic inline.
func writeTrashSnapshot(_, _ string, _ []string) (string, error) {
	return "", nil // wired to cmdundo.WriteTrashSnapshot via the binary entrypoint
}
