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

// G-050: Unit tests for forge optimize self-opt (DSPy-style convergence loop).
package cmdoptimize_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	cmdoptimize "github.com/teragrid/forge/internal/cli/cmdoptimize"
)

func execOptimize(t *testing.T, args []string) string {
	t.Helper()
	var buf bytes.Buffer
	c := cmdoptimize.New()
	c.SetOut(&buf)
	c.SetErr(&buf)
	c.SetArgs(args)
	if err := c.Execute(); err != nil {
		t.Fatalf("exec optimize %v: %v\n%s", args, err, buf.String())
	}
	return buf.String()
}

// TestSelfOpt_Converges verifies that the self-opt loop runs and converges
// (stops before max-iterations when improvement falls below threshold).
func TestSelfOpt_Converges(t *testing.T) {
	t.Parallel()
	// With synthetic scoring and a generous threshold, the loop must converge
	// well before the 50-iteration ceiling.
	out := execOptimize(t, []string{
		"self-opt",
		"--max-iterations", "50",
		"--convergence-threshold", "0.001",
	})
	// Verify at least one iteration line was printed.
	if !strings.Contains(out, "iter 1:") {
		t.Errorf("self-opt output missing iteration line, got: %q", out)
	}
	// The loop should have converged — output should not contain "iter 50".
	if strings.Contains(out, "iter 50:") {
		t.Errorf("self-opt should have converged before iteration 50, got: %q", out)
	}
}

// TestSelfOpt_JSON verifies that --json emits valid JSON containing a slice
// of SelfOptResult objects each with iteration, score, and converged fields.
func TestSelfOpt_JSON(t *testing.T) {
	t.Parallel()
	out := execOptimize(t, []string{
		"self-opt",
		"--json",
		"--max-iterations", "5",
		"--convergence-threshold", "0.001",
	})
	var results []map[string]any
	if err := json.Unmarshal([]byte(out), &results); err != nil {
		t.Fatalf("self-opt --json produced invalid JSON: %v\noutput: %q", err, out)
	}
	if len(results) == 0 {
		t.Fatal("self-opt --json returned empty results slice")
	}
	first := results[0]
	if _, ok := first["iteration"]; !ok {
		t.Errorf("first result missing 'iteration' field, keys: %v", mapKeys(first))
	}
	if _, ok := first["score"]; !ok {
		t.Errorf("first result missing 'score' field, keys: %v", mapKeys(first))
	}
	if _, ok := first["converged"]; !ok {
		t.Errorf("first result missing 'converged' field, keys: %v", mapKeys(first))
	}
}

// TestSelfOpt_MaxIterationsCap verifies that when convergence-threshold is 0
// (never converges), the loop still stops at --max-iterations.
func TestSelfOpt_MaxIterationsCap(t *testing.T) {
	t.Parallel()
	const maxIter = 3
	out := execOptimize(t, []string{
		"self-opt",
		"--json",
		"--max-iterations", "3",
		"--convergence-threshold", "0.0", // never converge
	})
	var results []map[string]any
	if err := json.Unmarshal([]byte(out), &results); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %q", err, out)
	}
	if len(results) > maxIter {
		t.Errorf("expected at most %d iterations, got %d", maxIter, len(results))
	}
}

func mapKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
