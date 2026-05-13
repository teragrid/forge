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
// Package cmdeval implements `forge eval` (DEV-M2-04).
package cmdeval

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/teragrid/forge/internal/errcode"
	"github.com/teragrid/forge/internal/eval"
	"github.com/teragrid/forge/internal/verbmeta"
)

// Reserved error codes (range 3600..3699).
var (
	ErrEvalLoadFailed     = errcode.Register(errcode.Code(3600), "failed to load scenario(s)")
	ErrEvalScenarioFailed = errcode.Register(errcode.Code(3601), "one or more scenarios failed")
	ErrEvalUsage          = errcode.Register(errcode.Code(3602), "eval command usage error")
)

func init() {
	verbmeta.Register(verbmeta.Manifest{
		Verb:    "eval",
		Summary: "Run scenario regression suites; CI-gateable. JSON scenarios under .forge/eval/ by default.",
		Inputs: []string{
			"<path> (file or directory; default '.forge/eval')",
			"--json (machine-readable Report)",
			"--ci (exit non-zero if any scenario fails; default true)",
		},
		Outputs:      []string{"stdout: human-readable summary or JSON Report"},
		SideEffects:  []string{"executes commands declared in scenarios; uses tmpdir per scenario when workdir='tmpdir'"},
		GatesTouched: []string{"§16.5.4 #6 — token budget / regression suite"},
		ErrorCodes:   []errcode.Code{ErrEvalLoadFailed, ErrEvalScenarioFailed, ErrEvalUsage},
	})
}

// New returns the cobra command.
func New() *cobra.Command {
	var (
		asJSON bool
		ci     bool
	)
	cmd := &cobra.Command{
		Use:   "eval [path]",
		Short: "Run scenario regression suites (M2.x).",
		Long: "Loads *.scenario.json files from the given path (file or directory; " +
			"default '.forge/eval') and runs each scenario, asserting on exit code, " +
			"stdout substrings, and stdout JSON keys. Exits non-zero on any failure when --ci.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := ".forge/eval"
			if len(args) == 1 {
				path = args[0]
			}
			scenarios, err := loadScenarios(path)
			if err != nil {
				return errcode.New(ErrEvalLoadFailed, fmt.Sprintf("load %s", path), err)
			}

			runner := eval.NewRunner()
			report := runner.RunAll(context.Background(), scenarios)

			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				if err := enc.Encode(report); err != nil {
					return err
				}
			} else {
				renderText(cmd, report)
			}

			if ci && report.Failed > 0 {
				return errcode.Newf(ErrEvalScenarioFailed, nil,
					"%d/%d scenarios failed", report.Failed, report.Total)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit machine-readable JSON Report")
	cmd.Flags().BoolVar(&ci, "ci", true, "exit non-zero if any scenario fails")
	return cmd
}

func loadScenarios(path string) ([]*eval.Scenario, error) {
	st, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if st.IsDir() {
		return eval.LoadDir(path)
	}
	s, err := eval.LoadScenario(path)
	if err != nil {
		return nil, err
	}
	return []*eval.Scenario{s}, nil
}

func renderText(cmd *cobra.Command, r eval.Report) {
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "forge eval — %d scenario(s)\n", r.Total)
	for _, s := range r.Scenarios {
		mark := "PASS"
		if !s.Passed {
			mark = "FAIL"
		}
		fmt.Fprintf(w, "  [%s] %s (%s)\n", mark, s.Name, s.Duration)
		if !s.Passed {
			for _, st := range s.Steps {
				if st.Passed {
					continue
				}
				fmt.Fprintf(w, "      ✗ %s: %s\n", st.StepID, st.Reason)
			}
		}
	}
	fmt.Fprintf(w, "\nresult: %d passed, %d failed\n", r.Passed, r.Failed)
}
