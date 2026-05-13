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

// Package cmdsla implements `forge sla` (M3-08): maintainer review-SLA
// dashboard generation.
//
// Sub-commands:
//
//	sla snapshot  — write a JSONL snapshot of PR records (input from stdin or file)
//	sla dashboard — print the SLA breach dashboard from a snapshot
//	sla check     — exit non-zero if any breaches exist (for CI)
package cmdsla

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/teragrid/forge/internal/errcode"
	"github.com/teragrid/forge/internal/reviewsla"
	"github.com/teragrid/forge/internal/verbmeta"
)

// Reserved error codes (range 5950..5959).
var (
	ErrSLAFailed = errcode.Register(errcode.Code(5950), "forge sla operation failed")
)

func init() {
	verbmeta.Register(verbmeta.Manifest{
		Verb:    "sla",
		Summary: "Maintainer review-SLA dashboard and CI gate (M3-08).",
		Inputs: []string{
			"snapshot  — write a JSONL PR snapshot from stdin or --in file",
			"dashboard — print breach dashboard from --snapshot file",
			"check     — exit non-zero if any SLA breaches exist (CI gate)",
			"--snapshot <path>",
			"--json",
		},
		Outputs:      []string{"stdout: dashboard or JSON breach list"},
		SideEffects:  []string{"snapshot: writes .forge/sla-snapshot.jsonl"},
		GatesTouched: []string{"M3-08 review-SLA"},
		ErrorCodes:   []errcode.Code{ErrSLAFailed},
	})
}

const defaultSnapshotPath = ".forge/sla-snapshot.jsonl"

// New returns the cobra command for `forge sla`.
func New() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sla <snapshot|dashboard|check>",
		Short: "Maintainer review-SLA dashboard (M3-08).",
		Long: "forge sla tracks maintainer review SLAs for open pull requests.\n\n" +
			"Snapshot PR data, then view the breach dashboard or use 'check' as a CI gate.\n\n" +
			"SLA targets:\n" +
			"  Initial triage:   48 hours\n" +
			"  First review:     7 days\n" +
			"  Merge decision:   30 days\n\n" +
			"See docs/MAINTAINER_SLA.md for the full policy.",
	}

	cmd.AddCommand(newSnapshotCmd(), newDashboardCmd(), newCheckCmd())
	return cmd
}

// newSnapshotCmd reads JSON-encoded PRRecord lines from stdin (or --in) and
// writes them to the snapshot file.
func newSnapshotCmd() *cobra.Command {
	var (
		inPath  string
		outPath string
	)
	c := &cobra.Command{
		Use:   "snapshot",
		Short: "Write a JSONL PR snapshot for SLA evaluation.",
		Long: "Reads JSON-encoded PRRecord objects (one per line) from stdin or --in,\n" +
			"and writes the snapshot to --out (default: " + defaultSnapshotPath + ").\n\n" +
			"Example input line:\n" +
			`  {"number":42,"title":"feat: foo","author":"alice","created_at":"2024-01-01T00:00:00Z","state":"open"}`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if outPath == "" {
				outPath = defaultSnapshotPath
			}
			src := cmd.InOrStdin()
			if inPath != "" {
				f, err := os.Open(inPath)
				if err != nil {
					return errcode.Newf(ErrSLAFailed, err, "open input %s", inPath)
				}
				defer f.Close() //nolint:errcheck
				src = f
			}

			var prs []reviewsla.PRRecord
			dec := json.NewDecoder(src)
			for dec.More() {
				var pr reviewsla.PRRecord
				if err := dec.Decode(&pr); err != nil {
					return errcode.Newf(ErrSLAFailed, err, "decode PR record")
				}
				prs = append(prs, pr)
			}

			if err := os.MkdirAll(".forge", 0o750); err != nil {
				return errcode.Newf(ErrSLAFailed, err, "create .forge directory")
			}
			f, err := os.Create(outPath)
			if err != nil {
				return errcode.Newf(ErrSLAFailed, err, "create snapshot %s", outPath)
			}
			defer f.Close() //nolint:errcheck

			if err := reviewsla.WriteSnapshot(f, prs); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "sla snapshot: wrote %d PR record(s) to %s\n", len(prs), outPath)
			return nil
		},
	}
	c.Flags().StringVar(&inPath, "in", "", "Input JSONL file (default: stdin)")
	c.Flags().StringVar(&outPath, "out", "", "Output snapshot path (default: "+defaultSnapshotPath+")")
	return c
}

func newDashboardCmd() *cobra.Command {
	var (
		snapshotPath string
		jsonOut      bool
	)
	c := &cobra.Command{
		Use:   "dashboard",
		Short: "Print the maintainer review-SLA breach dashboard.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if snapshotPath == "" {
				snapshotPath = defaultSnapshotPath
			}
			prs, err := reviewsla.ReadSnapshot(snapshotPath)
			if err != nil {
				return err
			}
			checker := reviewsla.NewChecker(reviewsla.DefaultPolicy)
			results := checker.CheckAll(prs)

			if jsonOut {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(results)
			}
			reviewsla.PrintDashboard(cmd.OutOrStdout(), results, reviewsla.DefaultPolicy)
			return nil
		},
	}
	c.Flags().StringVar(&snapshotPath, "snapshot", "", "Snapshot path (default: "+defaultSnapshotPath+")")
	c.Flags().BoolVar(&jsonOut, "json", false, "Emit JSON output")
	return c
}

func newCheckCmd() *cobra.Command {
	var snapshotPath string
	c := &cobra.Command{
		Use:   "check",
		Short: "Exit non-zero if any SLA breaches exist (CI gate).",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if snapshotPath == "" {
				snapshotPath = defaultSnapshotPath
			}
			prs, err := reviewsla.ReadSnapshot(snapshotPath)
			if err != nil {
				return err
			}
			checker := reviewsla.NewChecker(reviewsla.DefaultPolicy)
			results := checker.CheckAll(prs)
			breaches := reviewsla.Breaches(results)

			if len(breaches) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "sla check: all PRs within SLA targets")
				return nil
			}
			for _, b := range breaches {
				fmt.Fprintln(cmd.OutOrStderr(), "  BREACH:", b)
			}
			return errcode.Newf(ErrSLAFailed, nil, "%d SLA breach(es) detected", len(breaches))
		},
	}
	c.Flags().StringVar(&snapshotPath, "snapshot", "", "Snapshot path (default: "+defaultSnapshotPath+")")
	return c
}
