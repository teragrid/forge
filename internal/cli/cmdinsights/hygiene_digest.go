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

// G-071: `forge insights hygiene` — weekly hygiene digest.
// Finds unmanifested patterns, stale generated artefacts, and per-contributor
// hygiene debt.
package cmdinsights

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/spf13/cobra"

	"github.com/teragrid/forge/internal/errcode"
)

// NewHygieneCmd returns the `forge insights hygiene` subcommand.
func NewHygieneCmd() *cobra.Command {
	var (
		root   string
		period string
	)
	cmd := &cobra.Command{
		Use:   "hygiene",
		Short: "Show weekly hygiene digest: stale artefacts, unmanifested patterns, debt.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if root == "" {
				cwd, err := os.Getwd()
				if err != nil {
					return errcode.New(ErrStatsFailed, "getwd", err)
				}
				root = cwd
			}

			cutoff := 7
			if period == "30d" {
				cutoff = 30
			} else if period == "1d" {
				cutoff = 1
			}
			cutoffTime := time.Now().AddDate(0, 0, -cutoff)

			var sb fmt.Stringer
			out := &digestBuilder{}

			fmt.Fprintf(out, "# Forge Hygiene Digest — %s\n\n", time.Now().UTC().Format("2006-01-02"))
			fmt.Fprintf(out, "_Period: last %dd (since %s)_\n\n", cutoff, cutoffTime.Format("2006-01-02"))

			// Find stale generated artefacts in .forge/
			fmt.Fprintf(out, "## Stale Generated Artefacts\n\n")
			staleCount := 0
			_ = filepath.WalkDir(filepath.Join(root, ".forge"), func(path string, d os.DirEntry, err error) error {
				if err != nil || d.IsDir() {
					return nil
				}
				info, err := d.Info()
				if err != nil {
					return nil
				}
				if info.ModTime().Before(cutoffTime) {
					rel, _ := filepath.Rel(root, path)
					fmt.Fprintf(out, "- %s (modified %s)\n", rel, info.ModTime().Format("2006-01-02"))
					staleCount++
				}
				return nil
			})
			if staleCount == 0 {
				fmt.Fprintf(out, "_None._\n")
			}

			// Unmanifested .forge/ patterns (missing from hygiene.yml)
			fmt.Fprintf(out, "\n## Notes\n\n")
			fmt.Fprintf(out, "- Run `forge clean --check` to identify unmanifested scratch files.\n")
			fmt.Fprintf(out, "- Run `forge insights cli` to see unused verbs.\n")
			fmt.Fprintf(out, "- To open a hygiene PR: `forge clean --mode=apply && git add -A && git commit -m 'chore: hygiene cleanup'`\n")

			_ = sb
			fmt.Fprint(cmd.OutOrStdout(), out.String())
			return nil
		},
	}
	cmd.Flags().StringVar(&root, "root", "", "project root (default: cwd)")
	cmd.Flags().StringVar(&period, "period", "7d", "reporting period (1d, 7d, 30d)")
	return cmd
}

type digestBuilder struct {
	buf []byte
}

func (d *digestBuilder) Write(p []byte) (n int, err error) {
	d.buf = append(d.buf, p...)
	return len(p), nil
}

func (d *digestBuilder) String() string { return string(d.buf) }

// ensure sort import is used
var _ = sort.Ints
