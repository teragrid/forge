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

// feature_status.go — enhanced `forge ship status` implementation.
//
// Usage:
//
//	forge ship status                    # table of all features (all statuses)
//	forge ship status --done             # only fully-shipped (status=done) features
//	forge ship status --status active    # filter by lifecycle stage
//	forge ship status --json             # machine-readable JSON array
//	forge ship status <slug>             # detail view for one feature
//
// Data sources (in priority order):
//  1. .forge/specs/<slug>/spec.yml  — SpecManifest: feature name, status, created_at
//  2. .forge/specs/<slug>/*.md      — presence of checkpoint files (spec.md, arch.md, …)
//
// Lifecycle stages:
//
//	draft   — spec.yml written, no checkpoint md files yet
//	active  — some checkpoints done but < 7
//	done    — all 7 checkpoint md files present  OR  manifest.Status == "done"
//	unknown — no spec.yml and no checkpoint files (empty dir)
package cmdship

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/teragrid/forge/internal/errcode"
)

// checkpointFiles lists the 7 checkpoint output files in order.
// A checkpoint is considered "done" when its corresponding .md file exists.
var checkpointFiles = []string{
	"spec.md",
	"arch.md",
	"test.md",
	"breakdown.md",
	"code.md",
	"ship.md",
	"qa-verify.md",
}

// checkpointNames maps file name → human label for the detail view.
var checkpointNames = []string{
	"spec",
	"arch",
	"test",
	"breakdown",
	"code",
	"ship",
	"qa-verify",
}

// ── Types ──────────────────────────────────────────────────────────────────────

// FeatureStatusEntry is the data model for a single feature row.
type FeatureStatusEntry struct {
	Slug             string `json:"slug"`
	Feature          string `json:"feature"`           // from manifest; falls back to slug
	Status           string `json:"status"`            // "draft"|"active"|"done"|"unknown"
	CheckpointsDone  int    `json:"checkpoints_done"`  // 0–7
	CheckpointsTotal int    `json:"checkpoints_total"` // always 7
	CreatedAt        string `json:"created_at,omitempty"`
}

// ── Core helpers ───────────────────────────────────────────────────────────────

// scanFeatures reads every subdirectory of specsDir and returns one
// FeatureStatusEntry per feature. Non-directory entries are skipped.
func scanFeatures(specsDir string) ([]FeatureStatusEntry, error) {
	entries, err := os.ReadDir(specsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var out []FeatureStatusEntry
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		slug := e.Name()
		out = append(out, buildEntry(specsDir, slug))
	}
	return out, nil
}

// buildEntry builds a FeatureStatusEntry for one slug by reading its spec.yml
// and counting checkpoint .md files.
func buildEntry(specsDir, slug string) FeatureStatusEntry {
	featureDir := filepath.Join(specsDir, slug)

	entry := FeatureStatusEntry{
		Slug:             slug,
		Feature:          slug, // fallback
		CheckpointsTotal: len(checkpointFiles),
	}

	// Load manifest for feature name, status, and creation date.
	m := loadSpecManifest(specsDir, slug)
	if m != nil {
		if m.Feature != "" {
			entry.Feature = m.Feature
		}
		entry.CreatedAt = m.CreatedAt
		// Trust manifest status if explicitly set to "done".
		if m.Status == "done" {
			entry.Status = "done"
		}
	}

	// Count completed checkpoints.
	entry.CheckpointsDone = countCheckpoints(featureDir)

	// Derive lifecycle status from checkpoint count when not already "done".
	if entry.Status != "done" {
		switch {
		case entry.CheckpointsDone == len(checkpointFiles):
			entry.Status = "done"
		case entry.CheckpointsDone > 0:
			entry.Status = "active"
		case m != nil:
			entry.Status = "draft"
		default:
			entry.Status = "unknown"
		}
	}

	return entry
}

// countCheckpoints returns the number of checkpoint .md files that exist
// inside featureDir.
func countCheckpoints(featureDir string) int {
	done := 0
	for _, f := range checkpointFiles {
		if _, err := os.Stat(filepath.Join(featureDir, f)); err == nil {
			done++
		}
	}
	return done
}

// ── Rendering ─────────────────────────────────────────────────────────────────

// renderStatusTable writes an aligned table of entries to w.
func renderStatusTable(w io.Writer, entries []FeatureStatusEntry) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "SLUG\tFEATURE\tSTATUS\tPROGRESS\tCREATED")
	fmt.Fprintln(tw, "────\t───────\t──────\t────────\t───────")
	for _, e := range entries {
		created := e.CreatedAt
		if len(created) > 10 {
			created = created[:10] // show date only
		}
		if created == "" {
			created = "—"
		}
		icon := statusIcon(e.Status)
		feature := e.Feature
		if len(feature) > 40 {
			feature = feature[:37] + "..."
		}
		fmt.Fprintf(tw, "%s\t%s\t%s %s\t%d/%d\t%s\n",
			e.Slug, feature, icon, e.Status,
			e.CheckpointsDone, e.CheckpointsTotal, created)
	}
	tw.Flush()
}

// renderStatusDetail writes the per-checkpoint detail view for a single feature.
func renderStatusDetail(w io.Writer, featureDir string, entry FeatureStatusEntry) {
	icon := statusIcon(entry.Status)
	fmt.Fprintf(w, "forge ship status: %s\n", entry.Slug)
	if entry.Feature != entry.Slug {
		fmt.Fprintf(w, "  feature:  %s\n", entry.Feature)
	}
	fmt.Fprintf(w, "  status:   %s %s\n", icon, entry.Status)
	if entry.CreatedAt != "" {
		created := entry.CreatedAt
		if len(created) > 10 {
			created = created[:10]
		}
		fmt.Fprintf(w, "  created:  %s\n", created)
	}
	fmt.Fprintln(w)
	for i, f := range checkpointFiles {
		marker := "○ pending"
		if _, err := os.Stat(filepath.Join(featureDir, f)); err == nil {
			marker = "✓ done"
		}
		fmt.Fprintf(w, "  [%d/%d] %-12s %s\n",
			i+1, len(checkpointFiles), checkpointNames[i], marker)
	}
}

func statusIcon(status string) string {
	switch status {
	case "done":
		return "✓"
	case "active":
		return "⏳"
	case "draft":
		return "✎"
	default:
		return "○"
	}
}

// ── Cobra command ──────────────────────────────────────────────────────────────

// newStatusCmd builds the enhanced `forge ship status` command.
// It replaces the inline statusCmd defined previously in New().
func newStatusCmd() *cobra.Command {
	var (
		filterStatus string // --status flag
		onlyDone     bool   // --done shorthand
		asJSON       bool   // --json flag
		root         string // --root flag
	)

	cmd := &cobra.Command{
		Use:   "status [slug]",
		Short: "Show pipeline status for all features or a single feature.",
		Long: strings.TrimSpace(`
Show the ship pipeline status for every feature or a specific one.

Without a slug argument, prints a table of all features tracked in
.forge/specs/ with their lifecycle stage and checkpoint progress.

Lifecycle stages:
  ✓ done    all 7 checkpoints completed (feature shipped)
  ⏳ active  some checkpoints done, pipeline still in-flight
  ✎ draft   spec.yml created, no checkpoint files yet
  ○ unknown  empty directory (no manifest, no checkpoints)

Examples:
  forge ship status                   list all features
  forge ship status --done            list only shipped features
  forge ship status --status active   list in-flight features
  forge ship status --json            machine-readable JSON
  forge ship status login-oauth       detail view for one feature
`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r := root
			if r == "" {
				var err error
				r, err = os.Getwd()
				if err != nil {
					return errcode.New(ErrShipFailed, "getwd", err)
				}
			}
			specsDir := filepath.Join(r, ".forge", "specs")

			// ── Single-feature detail view ──────────────────────────────────
			if len(args) == 1 {
				slug := args[0]
				featureDir := filepath.Join(specsDir, slug)
				if _, err := os.Stat(featureDir); os.IsNotExist(err) {
					fmt.Fprintf(cmd.OutOrStdout(), "feature %q not found in .forge/specs/\n", slug)
					return nil
				}
				entry := buildEntry(specsDir, slug)
				if asJSON {
					enc := json.NewEncoder(cmd.OutOrStdout())
					enc.SetIndent("", "  ")
					return enc.Encode(entry)
				}
				renderStatusDetail(cmd.OutOrStdout(), featureDir, entry)
				return nil
			}

			// ── List all features ───────────────────────────────────────────
			entries, err := scanFeatures(specsDir)
			if err != nil {
				return errcode.New(ErrShipFailed, "scan .forge/specs", err)
			}

			// Resolve effective status filter.
			filter := filterStatus
			if onlyDone {
				filter = "done"
			}

			// Apply filter.
			filtered := entries[:0:0] // empty, same backing array
			for _, e := range entries {
				if filter == "" || filter == "all" || e.Status == filter {
					filtered = append(filtered, e)
				}
			}

			if len(filtered) == 0 {
				if filter != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "no features with status %q found in .forge/specs/\n", filter)
				} else {
					fmt.Fprintln(cmd.OutOrStdout(), "no features found (.forge/specs/ is empty or missing)")
				}
				return nil
			}

			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(filtered)
			}

			// Print summary line then table.
			label := "feature"
			if len(filtered) != 1 {
				label = "features"
			}
			qualifier := ""
			if filter != "" && filter != "all" {
				qualifier = filter + " "
			}
			fmt.Fprintf(cmd.OutOrStdout(), "forge ship status — %d %s%s\n\n",
				len(filtered), qualifier, label)
			renderStatusTable(cmd.OutOrStdout(), filtered)
			return nil
		},
	}

	cmd.Flags().StringVar(&filterStatus, "status", "", "filter by lifecycle stage: draft|active|done|all")
	cmd.Flags().BoolVar(&onlyDone, "done", false, "shorthand for --status done (show only shipped features)")
	cmd.Flags().BoolVarP(&asJSON, "json", "j", false, "emit machine-readable JSON")
	cmd.Flags().StringVarP(&root, "root", "r", "", "project root (default: cwd)")

	return cmd
}
