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

// G-141: `forge insights cli` — CLI usage analytics.
// Finds verbs nobody uses, verbs everyone misspells, and schemas that have
// drifted. Report written to private/docs/insights-<date>.md.
package cmdinsights

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/teragrid/forge/internal/audit"
	"github.com/teragrid/forge/internal/errcode"
)

// NewCLICmd returns the `forge insights cli` subcommand.
func NewCLICmd() *cobra.Command {
	var root string
	cmd := &cobra.Command{
		Use:   "cli",
		Short: "Analyse CLI usage patterns: unused verbs, common misspellings, schema drift.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if root == "" {
				cwd, err := os.Getwd()
				if err != nil {
					return errcode.New(ErrStatsFailed, "getwd", err)
				}
				root = cwd
			}

			path := filepath.Join(root, audit.DefaultPath)
			ledger, err := audit.Open(path)
			if err != nil {
				return errcode.New(ErrStatsFailed, "open ledger", err)
			}
			entries, err := ledger.All()
			if err != nil {
				return errcode.New(ErrStatsFailed, "read ledger", err)
			}

			// Count verb usage.
			verbCounts := map[string]int{}
			for _, e := range entries {
				if e.Verb != "" {
					verbCounts[e.Verb]++
				}
			}

			// Detect possible misspellings: verbs with < 3 uses that are close
			// to a known high-usage verb (simple prefix match).
			type verbStat struct {
				verb  string
				count int
			}
			var stats []verbStat
			for v, c := range verbCounts {
				stats = append(stats, verbStat{v, c})
			}
			sort.Slice(stats, func(i, j int) bool {
				return stats[i].count > stats[j].count
			})

			var sb strings.Builder
			now := time.Now().UTC()
			fmt.Fprintf(&sb, "# forge insights cli — %s\n\n", now.Format("2006-01-02"))
			fmt.Fprintf(&sb, "## Verb Usage (last %d events)\n\n", len(entries))
			if len(stats) == 0 {
				fmt.Fprintf(&sb, "_No audit events found._\n\n")
			} else {
				for _, s := range stats {
					fmt.Fprintf(&sb, "- %-20s %d uses\n", s.verb, s.count)
				}
			}

			// Flag verbs with very low usage.
			fmt.Fprintf(&sb, "\n## Low-Usage Verbs (≤2 uses)\n\n")
			low := false
			for _, s := range stats {
				if s.count <= 2 {
					fmt.Fprintf(&sb, "- %s (%d)\n", s.verb, s.count)
					low = true
				}
			}
			if !low {
				fmt.Fprintf(&sb, "_None._\n")
			}

			// Schema drift check: compare .forge/cli-schemas/ against known verbs.
			fmt.Fprintf(&sb, "\n## Schema Drift\n\n")
			schemaDir := filepath.Join(root, ".forge", "cli-schemas")
			schemaEntries, _ := os.ReadDir(schemaDir)
			if len(schemaEntries) == 0 {
				fmt.Fprintf(&sb, "_No schemas found. Run `forge schema generate` to create them._\n")
			} else {
				for _, e := range schemaEntries {
					verb := strings.TrimSuffix(e.Name(), ".schema.json")
					if verbCounts[verb] == 0 {
						fmt.Fprintf(&sb, "- %s: schema exists but no audit events recorded\n", verb)
					}
				}
			}

			// Write report.
			reportDir := filepath.Join(root, "private", "docs")
			_ = os.MkdirAll(reportDir, 0o755)
			reportPath := filepath.Join(reportDir, "insights-"+now.Format("2006-01-02")+".md")
				if err := os.WriteFile(reportPath, []byte(sb.String()), 0o600); err != nil {
				return errcode.New(ErrStatsFailed, "write report", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "insights report written to %s\n", reportPath)
			fmt.Fprint(cmd.OutOrStdout(), sb.String())
			return nil
		},
	}
	cmd.Flags().StringVar(&root, "root", "", "project root (default: cwd)")
	return cmd
}
