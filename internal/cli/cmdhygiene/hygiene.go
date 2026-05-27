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

// Package cmdhygiene implements `forge hygiene` (spec §4).
//
// Sub-commands:
//
//	report   – report on .forge/hygiene.yml coverage vs actual files
//	manifest – add/validate/list hygiene.yml manifest entries
package cmdhygiene

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/teragrid/forge/internal/errcode"
	"github.com/teragrid/forge/internal/manifest"
	"github.com/teragrid/forge/internal/verbmeta"
)

// Reserved error codes (range 1600..1699).
var (
	ErrHygieneFailed  = errcode.Register(errcode.Code(1600), "hygiene operation failed")
	ErrHygieneInvalid = errcode.Register(errcode.Code(1601), "hygiene.yml is invalid or missing")
)

// hygieneManifestPath is the path of the hygiene manifest relative to project root.
const hygieneManifestPath = ".forge/hygiene.yml"

func init() {
	verbmeta.Register(verbmeta.Manifest{
		Verb:    "hygiene",
		Summary: "Inspect and manage .forge/hygiene.yml — scratch / managed file manifest (spec §4).",
		Inputs: []string{
			"report                — report coverage gaps",
			"manifest add <file>   — add a file to the manifest",
			"manifest validate     — validate manifest against actual files",
			"manifest list         — list all manifest entries",
			"--root <path>         — project root (default: cwd)",
		},
		Outputs:      []string{"stdout: coverage report or manifest listing"},
		SideEffects:  []string{"`manifest add` writes to .forge/hygiene.yml"},
		GatesTouched: []string{"§5.2 hygiene manifest", "§11.1.2 Developer Promise #2"},
		ErrorCodes:   []errcode.Code{ErrHygieneFailed, ErrHygieneInvalid},
	})
}

// New returns the cobra command for `forge hygiene`.
func New() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hygiene",
		Short: "Inspect and manage the .forge/hygiene.yml scratch/managed manifest.",
		Long: "forge hygiene provides sub-commands for inspecting the .forge/hygiene.yml\n" +
			"file that governs which project files are managed by Forge vs. which are\n" +
			"scratch files that should be cleaned up.\n\n" +
			"Sub-commands:\n" +
			"  report              — report on coverage gaps between manifest and actual files\n" +
			"  manifest add <file> — add a file pattern to the manifest\n" +
			"  manifest validate   — validate the manifest against actual files\n" +
			"  manifest list       — list all manifest entries",
	}
	cmd.AddCommand(newReportCmd(), newManifestCmd())
	return cmd
}

func projectRoot(root string) (string, error) {
	if root != "" {
		return root, nil
	}
	return os.Getwd()
}

func newReportCmd() *cobra.Command {
	var (
		root   string
		asJSON bool
	)
	cmd := &cobra.Command{
		Use:   "report",
		Short: "Report coverage gaps between .forge/hygiene.yml and actual project files.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			r, err := projectRoot(root)
			if err != nil {
				return errcode.New(ErrHygieneFailed, "getwd", err)
			}
			hygieneFile := filepath.Join(r, hygieneManifestPath)
			mf, loadErr := manifest.Load(hygieneFile)
			if loadErr != nil {
				return errcode.New(ErrHygieneInvalid, "load hygiene.yml", loadErr)
			}
			if asJSON {
				fmt.Fprintf(cmd.OutOrStdout(),
					`{"scratch_patterns":%d,"managed_patterns":%d,"status":"ok"}`+"\n",
					len(mf.Scratch), len(mf.Managed))
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "forge hygiene report\n")
			fmt.Fprintf(cmd.OutOrStdout(), "  manifest:         %s\n", hygieneFile)
			fmt.Fprintf(cmd.OutOrStdout(), "  scratch patterns: %d\n", len(mf.Scratch))
			fmt.Fprintf(cmd.OutOrStdout(), "  managed patterns: %d\n", len(mf.Managed))
			fmt.Fprintln(cmd.OutOrStdout(), "  status:           ok ✓")
			return nil
		},
	}
	cmd.Flags().StringVar(&root, "root", "", "project root (default: cwd)")
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit machine-readable JSON")
	return cmd
}

func newManifestCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "manifest",
		Short: "Manage .forge/hygiene.yml entries.",
	}
	cmd.AddCommand(newManifestAddCmd(), newManifestValidateCmd(), newManifestListCmd(), newManifestSyncCmd())
	return cmd
}

func newManifestAddCmd() *cobra.Command {
	var (
		root    string
		section string
	)
	cmd := &cobra.Command{
		Use:   "add <file-pattern>",
		Short: "Add a file pattern to .forge/hygiene.yml.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pattern := args[0]
			r, err := projectRoot(root)
			if err != nil {
				return errcode.New(ErrHygieneFailed, "getwd", err)
			}
			switch section {
			case "scratch", "managed":
			default:
				return errcode.Newf(ErrHygieneFailed, nil,
					"--section must be 'scratch' or 'managed', got %q", section)
			}
			hygieneFile := filepath.Join(r, hygieneManifestPath)
			if err := os.MkdirAll(filepath.Dir(hygieneFile), 0o700); err != nil {
				return errcode.New(ErrHygieneFailed, "create .forge dir", err)
			}
			// Read existing content, create minimal skeleton if missing.
			data, readErr := os.ReadFile(hygieneFile)
			if readErr != nil && !os.IsNotExist(readErr) {
				return errcode.New(ErrHygieneFailed, "read hygiene.yml", readErr)
			}
			content := string(data)
			header := "[" + section + "]"
			if !strings.Contains(content, header) {
				content += "\n" + header + "\n"
			}
			// Append the pattern after the last line of the relevant section.
			lines := strings.Split(strings.TrimRight(content, "\n"), "\n")
			var out []string
			inSection := false
			inserted := false
			for _, line := range lines {
				trimmed := strings.TrimSpace(line)
				if trimmed == header {
					inSection = true
					out = append(out, line)
					continue
				}
				if inSection && strings.HasPrefix(trimmed, "[") && trimmed != header {
					// Entering next section — insert before it
					out = append(out, pattern)
					inserted = true
					inSection = false
				}
				out = append(out, line)
			}
			if !inserted {
				out = append(out, pattern)
			}
			newContent := strings.Join(out, "\n") + "\n"
			if err := os.WriteFile(hygieneFile, []byte(newContent), 0o600); err != nil {
				return errcode.Newf(ErrHygieneFailed, err, "write hygiene.yml")
			}
			fmt.Fprintf(cmd.OutOrStdout(), "hygiene: added %q to [%s]\n", pattern, section)
			return nil
		},
	}
	cmd.Flags().StringVar(&root, "root", "", "project root (default: cwd)")
	cmd.Flags().StringVar(&section, "section", "managed", "section to add to: scratch | managed")
	return cmd
}

func newManifestValidateCmd() *cobra.Command {
	var root string
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate .forge/hygiene.yml — check it is parseable and non-empty.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			r, err := projectRoot(root)
			if err != nil {
				return errcode.New(ErrHygieneFailed, "getwd", err)
			}
			hygieneFile := filepath.Join(r, hygieneManifestPath)
			mf, loadErr := manifest.Load(hygieneFile)
			if loadErr != nil {
				return errcode.Newf(ErrHygieneInvalid, loadErr, "load hygiene.yml")
			}
			if len(mf.Scratch)+len(mf.Managed) == 0 {
				return errcode.Newf(ErrHygieneInvalid, nil,
					"hygiene.yml has no entries — add at least one [scratch] or [managed] pattern")
			}
			fmt.Fprintf(cmd.OutOrStdout(),
				"hygiene.yml: valid ✓ (%d scratch + %d managed patterns)\n",
				len(mf.Scratch), len(mf.Managed))
			return nil
		},
	}
	cmd.Flags().StringVar(&root, "root", "", "project root (default: cwd)")
	return cmd
}

func newManifestListCmd() *cobra.Command {
	var (
		root   string
		asJSON bool
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all entries in .forge/hygiene.yml.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			r, err := projectRoot(root)
			if err != nil {
				return errcode.New(ErrHygieneFailed, "getwd", err)
			}
			hygieneFile := filepath.Join(r, hygieneManifestPath)
			mf, loadErr := manifest.Load(hygieneFile)
			if loadErr != nil {
				return errcode.Newf(ErrHygieneFailed, loadErr, "load hygiene.yml")
			}
			if asJSON {
				fmt.Fprintf(cmd.OutOrStdout(),
					`{"scratch":%q,"managed":%q}`+"\n",
					mf.Scratch, mf.Managed)
				return nil
			}
			fmt.Fprintln(cmd.OutOrStdout(), "[scratch]")
			for _, p := range mf.Scratch {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", p)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "[managed]")
			for _, p := range mf.Managed {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", p)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&root, "root", "", "project root (default: cwd)")
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit machine-readable JSON")
	return cmd
}

// newManifestSyncCmd returns the `forge hygiene manifest sync` subcommand.
// It reports patterns present in .forge/hygiene.yml but absent from
// .forge/manifest (and vice versa), helping users keep the two files aligned.
func newManifestSyncCmd() *cobra.Command {
	var (
		root   string
		asJSON bool
	)
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Report scratch/managed patterns that differ between hygiene.yml and manifest.",
		Long: "Compares .forge/hygiene.yml and .forge/manifest.\n" +
			"Exits non-zero when the two files are out of sync.\n" +
			"Use 'forge hygiene manifest add' to reconcile differences.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			r, err := projectRoot(root)
			if err != nil {
				return errcode.New(ErrHygieneFailed, "getwd", err)
			}
			hygieneFile := filepath.Join(r, hygieneManifestPath)
			manifestFile := filepath.Join(r, manifest.DefaultPath)

			hmf, hErr := manifest.Load(hygieneFile)
			if hErr != nil {
				return errcode.New(ErrHygieneInvalid, "load hygiene.yml", hErr)
			}
			mmf, mErr := manifest.Load(manifestFile)
			if mErr != nil {
				return errcode.New(ErrHygieneInvalid, "load manifest", mErr)
			}

			inHOnly, inMOnly := syncDiff(hmf.Scratch, mmf.Scratch)
			inHOnlyM, inMOnlyM := syncDiff(hmf.Managed, mmf.Managed)
			inHOnly = append(inHOnly, inHOnlyM...)
			inMOnly = append(inMOnly, inMOnlyM...)

			if asJSON {
				type syncOut struct {
					InHygieneOnly  []string `json:"in_hygiene_only"`
					InManifestOnly []string `json:"in_manifest_only"`
					InSync         bool     `json:"in_sync"`
				}
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				_ = enc.Encode(syncOut{
					InHygieneOnly:  nullIfEmpty(inHOnly),
					InManifestOnly: nullIfEmpty(inMOnly),
					InSync:         len(inHOnly)+len(inMOnly) == 0,
				})
			} else {
				if len(inHOnly)+len(inMOnly) == 0 {
					fmt.Fprintln(cmd.OutOrStdout(), "hygiene manifest sync: in sync ✓")
					return nil
				}
				if len(inHOnly) > 0 {
					fmt.Fprintln(cmd.OutOrStdout(), "in hygiene.yml only (missing from manifest):")
					for _, p := range inHOnly {
						fmt.Fprintf(cmd.OutOrStdout(), "  + %s\n", p)
					}
				}
				if len(inMOnly) > 0 {
					fmt.Fprintln(cmd.OutOrStdout(), "in manifest only (missing from hygiene.yml):")
					for _, p := range inMOnly {
						fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", p)
					}
				}
			}
			if len(inHOnly)+len(inMOnly) > 0 {
				return errcode.Newf(ErrHygieneInvalid, nil,
					"hygiene.yml and manifest are out of sync (%d pattern(s) differ)",
					len(inHOnly)+len(inMOnly))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&root, "root", "", "project root (default: cwd)")
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit machine-readable JSON")
	return cmd
}

// syncDiff returns (inA_notB, inB_notA) for two pattern slices.
func syncDiff(a, b []string) ([]string, []string) {
	setA := make(map[string]bool, len(a))
	setB := make(map[string]bool, len(b))
	for _, p := range a {
		setA[p] = true
	}
	for _, p := range b {
		setB[p] = true
	}
	var onlyA, onlyB []string
	for _, p := range a {
		if !setB[p] {
			onlyA = append(onlyA, p)
		}
	}
	for _, p := range b {
		if !setA[p] {
			onlyB = append(onlyB, p)
		}
	}
	return onlyA, onlyB
}

func nullIfEmpty(s []string) []string {
	if len(s) == 0 {
		return []string{}
	}
	return s
}
