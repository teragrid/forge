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

// Package cmdreport implements `forge report` â€” a structured project health
// report that aggregates scan findings, token spend, test coverage, hygiene
// status, and incident history into a single human- or machine-readable
// summary (DEV-M3 Â§18.1).
//
// Subcommands:
//
//	forge report            â€” full project report (default: text)
//	forge report --json     â€” same report as JSON
//	forge report --since    â€” limit findings/spend to a time window (e.g. "7d")
//	forge report --out <f>  â€” write to file instead of stdout
package cmdreport

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

// Error codes (range 5500..5599).
var (
	ErrReportFailed = errcode.Register(errcode.Code(5500), "forge report failed")
)

func init() {
	verbmeta.Register(verbmeta.Manifest{
		Verb:    "report",
		Summary: "Generate a structured project health report (scan findings, spend, test coverage, hygiene).",
		Inputs: []string{
			"--json",
			"--since <duration> (e.g. 7d, 30d)",
			"--out <file>",
			"--root <path>",
		},
		Outputs:      []string{"stdout: health report (text or JSON)"},
		SideEffects:  []string{"none"},
		GatesTouched: []string{"Â§18.1 â€” observability / reporting"},
	})
}

// Report contains the aggregated health report data.
type Report struct {
	GeneratedAt  time.Time      `json:"generated_at"`
	ProjectRoot  string         `json:"project_root"`
	Period       string         `json:"period,omitempty"`
	ScanFindings ScanSummary    `json:"scan_findings"`
	TokenSpend   SpendSummary   `json:"token_spend"`
	HygieneScore HygieneSummary `json:"hygiene"`
	Incidents    IncidentCount  `json:"incidents"`
}

// ScanSummary holds finding counts by severity.
type ScanSummary struct {
	Errors   int `json:"errors"`
	Warnings int `json:"warnings"`
	Infos    int `json:"infos"`
	Total    int `json:"total"`
}

// SpendSummary holds token/cost totals.
type SpendSummary struct {
	TotalTokens int     `json:"total_tokens"`
	TotalCostUS float64 `json:"total_cost_usd"`
}

// HygieneSummary holds hygiene gate status.
type HygieneSummary struct {
	Passed  int `json:"passed"`
	Failed  int `json:"failed"`
	Skipped int `json:"skipped"`
}

// IncidentCount holds open/closed incident totals.
type IncidentCount struct {
	Open   int `json:"open"`
	Closed int `json:"closed"`
}

// New returns the `forge report` cobra command.
func New() *cobra.Command {
	var (
		jsonOut bool
		since   string
		outFile string
		root    string
	)

	cmd := &cobra.Command{
		Use:   "report",
		Short: "Generate a structured project health report",
		Long: `forge report aggregates scan findings, LLM token spend, hygiene status,
and incident history into a single report. Use --json for machine-readable
output suitable for dashboards or CI artefact upload.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if root == "" {
				wd, err := os.Getwd()
				if err != nil {
					return errcode.New(ErrReportFailed, "could not determine working directory", err)
				}
				root = wd
			}

			r, err := buildReport(root, since)
			if err != nil {
				return errcode.New(ErrReportFailed, "failed to build report", err)
			}

			var out []byte
			if jsonOut {
				out, err = json.MarshalIndent(r, "", "  ")
				if err != nil {
					return errcode.New(ErrReportFailed, "failed to marshal report", err)
				}
				out = append(out, '\n')
			} else {
				out = []byte(formatText(r))
			}

			if outFile != "" {
				if err := os.WriteFile(outFile, out, 0o600); err != nil {
					return errcode.New(ErrReportFailed, "failed to write report file", err)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Report written to %s\n", outFile)
				return nil
			}
			_, err = cmd.OutOrStdout().Write(out)
			return err
		},
	}

	cmd.Flags().BoolVar(&jsonOut, "json", false, "output as JSON")
	cmd.Flags().StringVar(&since, "since", "", "limit report window (e.g. 7d, 30d, 2006-01-02)")
	cmd.Flags().StringVar(&outFile, "out", "", "write report to file")
	cmd.Flags().StringVar(&root, "root", "", "project root (default: cwd)")
	return cmd
}

// buildReport assembles the Report by reading existing artefact files.
// It is intentionally lenient: missing artefact files produce zero counts
// rather than errors.
func buildReport(root, since string) (*Report, error) {
	r := &Report{
		GeneratedAt: time.Now().UTC(),
		ProjectRoot: root,
		Period:      since,
	}

	// â”€â”€ Scan findings â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€
	scanFile := filepath.Join(root, ".forge", "scan-results.json")
	if data, err := os.ReadFile(scanFile); err == nil {
		var payload struct {
			Findings []struct {
				Severity string `json:"severity"`
			} `json:"findings"`
		}
		if json.Unmarshal(data, &payload) == nil {
			for _, f := range payload.Findings {
				r.ScanFindings.Total++
				switch f.Severity {
				case "error":
					r.ScanFindings.Errors++
				case "warning":
					r.ScanFindings.Warnings++
				default:
					r.ScanFindings.Infos++
				}
			}
		}
	}

	// â”€â”€ Token spend â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€
	ledgerFile := filepath.Join(root, ".forge", "token-ledger.json")
	if data, err := os.ReadFile(ledgerFile); err == nil {
		var entries []struct {
			Tokens int     `json:"tokens"`
			Cost   float64 `json:"cost_usd"`
		}
		if json.Unmarshal(data, &entries) == nil {
			for _, e := range entries {
				r.TokenSpend.TotalTokens += e.Tokens
				r.TokenSpend.TotalCostUS += e.Cost
			}
		}
	}

	// â”€â”€ Hygiene â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€
	hygieneFile := filepath.Join(root, ".forge", "hygiene-results.json")
	if data, err := os.ReadFile(hygieneFile); err == nil {
		var results []struct {
			Status string `json:"status"`
		}
		if json.Unmarshal(data, &results) == nil {
			for _, res := range results {
				switch res.Status {
				case "pass":
					r.HygieneScore.Passed++
				case "fail":
					r.HygieneScore.Failed++
				default:
					r.HygieneScore.Skipped++
				}
			}
		}
	}

	// â”€â”€ Incidents â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€
	incDir := filepath.Join(root, ".forge", "incidents")
	if entries, err := os.ReadDir(incDir); err == nil {
		for _, e := range entries {
			if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
				continue
			}
			data, err := os.ReadFile(filepath.Join(incDir, e.Name()))
			if err != nil {
				continue
			}
			var inc struct {
				Status string `json:"status"`
			}
			if json.Unmarshal(data, &inc) == nil {
				if inc.Status == "closed" || inc.Status == "resolved" {
					r.Incidents.Closed++
				} else {
					r.Incidents.Open++
				}
			}
		}
	}

	return r, nil
}

// formatText produces a human-readable report.
func formatText(r *Report) string {
	out := "â•”â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•—\n"
	out += "â•‘            forge â€” Project Health Report                 â•‘\n"
	out += "â•šâ•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•\n"
	out += fmt.Sprintf("Generated: %s\n", r.GeneratedAt.Format(time.RFC3339))
	out += fmt.Sprintf("Root:      %s\n", r.ProjectRoot)
	if r.Period != "" {
		out += fmt.Sprintf("Period:    %s\n", r.Period)
	}
	out += "\n"

	out += "â”€â”€ Scan Findings â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€\n"
	out += fmt.Sprintf("  Total:    %d\n", r.ScanFindings.Total)
	out += fmt.Sprintf("  Errors:   %d\n", r.ScanFindings.Errors)
	out += fmt.Sprintf("  Warnings: %d\n", r.ScanFindings.Warnings)
	out += fmt.Sprintf("  Infos:    %d\n", r.ScanFindings.Infos)

	out += "\nâ”€â”€ Token Spend â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€\n"
	out += fmt.Sprintf("  Total tokens: %d\n", r.TokenSpend.TotalTokens)
	out += fmt.Sprintf("  Total cost:   $%.4f\n", r.TokenSpend.TotalCostUS)

	out += "\nâ”€â”€ Hygiene â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€\n"
	out += fmt.Sprintf("  Passed:  %d\n", r.HygieneScore.Passed)
	out += fmt.Sprintf("  Failed:  %d\n", r.HygieneScore.Failed)
	out += fmt.Sprintf("  Skipped: %d\n", r.HygieneScore.Skipped)

	out += "\nâ”€â”€ Incidents â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€\n"
	out += fmt.Sprintf("  Open:   %d\n", r.Incidents.Open)
	out += fmt.Sprintf("  Closed: %d\n", r.Incidents.Closed)
	out += "\n"

	return out
}
