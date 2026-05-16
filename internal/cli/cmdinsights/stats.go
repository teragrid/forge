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
// Package cmdinsights implements `forge insights` — local telemetry rollup from
// the audit ledger (.forge/audit.log). No remote calls; purely local.
// DEV-M3-02.
package cmdinsights

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/teragrid/forge/internal/audit"
	"github.com/teragrid/forge/internal/errcode"
	"github.com/teragrid/forge/internal/verbmeta"
)

// Reserved error codes (range 3900–3999).
var (
	ErrStatsFailed = errcode.Register(errcode.Code(3900), "stats operation failed")
)

func init() {
	verbmeta.Register(verbmeta.Manifest{
		Verb:    "insights",
		Summary: "Show local telemetry rollup from the audit ledger.",
		Inputs: []string{
			"--root <path> (default: cwd)",
			"--since <YYYY-MM-DD> (default: all time)",
			"--json",
		},
		Outputs:      []string{"stdout: per-verb action counts and timeline"},
		SideEffects:  []string{"none (read-only)"},
		GatesTouched: []string{"DEV-M3-02 local telemetry", "§16.5.5 observability"},
		ErrorCodes:   []errcode.Code{ErrStatsFailed},
	})
}

// VerbStat holds aggregate counts for one verb.
type VerbStat struct {
	Verb            string         `json:"verb"`
	Count           int            `json:"count"`
	LastSeen        time.Time      `json:"last_seen"`
	ActionBreakdown map[string]int `json:"action_breakdown,omitempty"`
}

// Report is the full stats output.
type Report struct {
	GeneratedAt time.Time  `json:"generated_at"`
	TotalEvents int        `json:"total_events"`
	SinceFilter string     `json:"since,omitempty"`
	Verbs       []VerbStat `json:"verbs"`
	// G-013: quick_ratio_30d — fraction of ship runs that used --quick.
	// Printed as a workflow-smell banner when > 20%.
	QuickRatio30d float64 `json:"quick_ratio_30d,omitempty"`
	QuickSmell    bool    `json:"quick_smell,omitempty"`
}

// New returns the cobra command.
func New() *cobra.Command {
	var (
		root   string
		since  string
		asJSON bool
	)
	cmd := &cobra.Command{
		Use:   "insights",
		Short: "Show local telemetry rollup from the audit ledger.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if root == "" {
				cwd, err := os.Getwd()
				if err != nil {
					return errcode.New(ErrStatsFailed, "getwd", err)
				}
				root = cwd
			}
			var sinceT time.Time
			if since != "" {
				t, err := time.Parse("2006-01-02", since)
				if err != nil {
					return errcode.Newf(ErrStatsFailed, err, "--since: invalid date %q (want YYYY-MM-DD)", since)
				}
				sinceT = t
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
			report := buildReport(entries, sinceT, since)
			return renderReport(cmd, report, asJSON)
		},
	}
	cmd.Flags().StringVar(&root, "root", "", "project root (default: cwd)")
	cmd.Flags().StringVar(&since, "since", "", "filter events from YYYY-MM-DD onwards")
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit machine-readable JSON")
	cmd.AddCommand(NewCLICmd(), NewHygieneCmd())
	return cmd
}

func buildReport(entries []audit.Entry, sinceT time.Time, sinceLabel string) Report {
	type accum struct {
		count    int
		lastSeen time.Time
		actions  map[string]int
	}
	verbMap := map[string]*accum{}
	// G-013: track ship --quick usage over last 30d.
	thirtyDaysAgo := time.Now().UTC().AddDate(0, 0, -30)
	var shipTotal30d, shipQuick30d int

	for _, e := range entries {
		if !sinceT.IsZero() && e.Timestamp.Before(sinceT) {
			continue
		}
		a, ok := verbMap[e.Verb]
		if !ok {
			a = &accum{actions: map[string]int{}}
			verbMap[e.Verb] = a
		}
		a.count++
		if e.Timestamp.After(a.lastSeen) {
			a.lastSeen = e.Timestamp
		}
		a.actions[e.Action]++

		// Count --quick usage in ship runs over the last 30d.
		if e.Verb == "ship" && !e.Timestamp.Before(thirtyDaysAgo) {
			shipTotal30d++
			if det, ok2 := e.Detail["flag"]; ok2 && det == "--quick" {
				shipQuick30d++
			}
		}
	}

	var verbs []VerbStat
	for v, a := range verbMap {
		verbs = append(verbs, VerbStat{
			Verb:            v,
			Count:           a.count,
			LastSeen:        a.lastSeen,
			ActionBreakdown: a.actions,
		})
	}
	sort.Slice(verbs, func(i, j int) bool {
		if verbs[i].Count != verbs[j].Count {
			return verbs[i].Count > verbs[j].Count // descending
		}
		return verbs[i].Verb < verbs[j].Verb
	})

	total := 0
	for _, vs := range verbs {
		total += vs.Count
	}

	// G-013: compute quick_ratio_30d.
	var quickRatio float64
	if shipTotal30d > 0 {
		quickRatio = float64(shipQuick30d) / float64(shipTotal30d)
	}

	return Report{
		GeneratedAt:   time.Now().UTC(),
		TotalEvents:   total,
		SinceFilter:   sinceLabel,
		Verbs:         verbs,
		QuickRatio30d: quickRatio,
		QuickSmell:    quickRatio > 0.20,
	}
}

func renderReport(cmd *cobra.Command, r Report, asJSON bool) error {
	if asJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(r)
	}
	if r.TotalEvents == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "stats: no audit events found")
		return nil
	}
	tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	fmt.Fprintf(tw, "VERB\tCOUNT\tLAST SEEN\n")
	for _, vs := range r.Verbs {
		fmt.Fprintf(tw, "%s\t%d\t%s\n", vs.Verb, vs.Count, vs.LastSeen.Format("2006-01-02 15:04:05Z"))
	}
	_ = tw.Flush()
	fmt.Fprintf(cmd.OutOrStdout(), "\ntotal events: %d\n", r.TotalEvents)
	// G-013: workflow smell banner.
	if r.QuickSmell {
		fmt.Fprintf(cmd.OutOrStdout(),
			"\n⚠  WORKFLOW SMELL: --quick flag used in %.0f%% of ship runs (30d). "+
				"Consider addressing root causes.\n", r.QuickRatio30d*100)
	}
	return nil
}
