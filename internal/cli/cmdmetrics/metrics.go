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

// Package cmdmetrics implements `forge metrics` (P2: Prometheus token-ledger export).
//
// forge metrics reads .forge/token-ledger.jsonl and prints cumulative
// token and cost counters in Prometheus text format.  The output is suitable
// for scraping by a Prometheus pull gateway or piping to a Push Gateway:
//
//	forge metrics | curl --data-binary @- \
//	    http://pushgateway:9091/metrics/job/forge
//
// Error codes reserved in range 6800–6849.
package cmdmetrics

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/teragrid/forge/internal/errcode"
	"github.com/teragrid/forge/internal/tokenledger"
	"github.com/teragrid/forge/internal/verbmeta"
)

// Reserved error codes (range 6600..6649).
var (
	ErrMetricsFailed = errcode.Register(errcode.Code(6600), "metrics export failed")
)

func init() {
	verbmeta.Register(verbmeta.Manifest{
		Verb:    "metrics",
		Summary: "Export token-ledger data as Prometheus text format (P2, ADR-026).",
		Inputs: []string{
			"--root <path>  — project root (default: .)",
			"--ledger <path> — override ledger file path",
		},
		Outputs: []string{
			"stdout: Prometheus text format with forge_tokens_total and forge_cost_usd_total",
		},
		SideEffects:  []string{"none (read-only)"},
		GatesTouched: []string{},
		ErrorCodes:   []errcode.Code{ErrMetricsFailed},
	})
}

// New returns the cobra command for `forge metrics`.
func New() *cobra.Command {
	var (
		root       string
		ledgerPath string
	)

	cmd := &cobra.Command{
		Use:   "metrics",
		Short: "Export token-ledger data as Prometheus text format",
		Long: `forge metrics reads the token ledger (.forge/token-ledger.jsonl) and
prints cumulative token and cost counters in Prometheus text format.

Metrics exported:
  forge_tokens_total{model,operation,type}  — cumulative token counter
  forge_cost_usd_total{model,operation}      — cumulative cost in USD

Pipe the output to a Prometheus Push Gateway:
  forge metrics | curl --data-binary @- http://pushgateway:9091/metrics/job/forge`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if root == "" {
				cwd, err := os.Getwd()
				if err != nil {
					return errcode.New(ErrMetricsFailed, "cannot determine working directory", err)
				}
				root = cwd
			}
			if ledgerPath == "" {
				ledgerPath = filepath.Join(root, tokenledger.DefaultPath)
			}

			ledger := tokenledger.New(ledgerPath)
			output, err := ledger.ExportPrometheus()
			if err != nil {
				return errcode.New(ErrMetricsFailed,
					fmt.Sprintf("failed to export metrics from %s", ledgerPath), err)
			}
			if output == "" {
				fmt.Fprintln(cmd.OutOrStdout(), "# No token-ledger entries found.")
				return nil
			}
			fmt.Fprint(cmd.OutOrStdout(), output)
			return nil
		},
	}

	cmd.Flags().StringVar(&root, "root", "", "project root directory (default: current directory)")
	cmd.Flags().StringVar(&ledgerPath, "ledger", "", "override ledger file path")
	return cmd
}
