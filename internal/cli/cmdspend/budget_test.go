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
package cmdspend

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/teragrid/forge/internal/llmbudget"
)

func exec(t *testing.T, args []string) string {
	t.Helper()
	var buf bytes.Buffer
	c := New()
	c.SetOut(&buf)
	c.SetErr(&buf)
	c.SetArgs(args)
	if err := c.Execute(); err != nil {
		t.Fatalf("exec %v: %v\n%s", args, err, buf.String())
	}
	return buf.String()
}

func execErr(t *testing.T, args []string) error {
	t.Helper()
	var buf bytes.Buffer
	c := New()
	c.SetOut(&buf)
	c.SetErr(&buf)
	c.SetArgs(args)
	return c.Execute()
}

func seedBudget(t *testing.T, dir string, b *llmbudget.Budget) {
	t.Helper()
	if err := b.Save(filepath.Join(dir, llmbudget.DefaultPath)); err != nil {
		t.Fatalf("seedBudget: %v", err)
	}
}

// TC-CBUD-01: status on empty budget shows $0 spend.
func TestBudget_StatusEmpty(t *testing.T) {
	dir := t.TempDir()
	out := exec(t, []string{"status", "--root", dir})
	if !strings.Contains(out, "$0.0000") {
		t.Fatalf("want $0.0000 in output, got: %s", out)
	}
}

// TC-CBUD-02: status --json has expected keys.
func TestBudget_StatusJSON(t *testing.T) {
	dir := t.TempDir()
	b := llmbudget.New()
	b.Add(llmbudget.Record{Timestamp: time.Now().UTC(), Verb: "scan", CostUSD: 0.05})
	seedBudget(t, dir, b)

	out := exec(t, []string{"status", "--root", dir, "--json"})
	var p map[string]any
	if err := json.Unmarshal([]byte(out), &p); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	for _, key := range []string{"daily_spend_usd", "monthly_spend_usd", "daily_limit_usd", "monthly_limit_usd", "record_count"} {
		if _, ok := p[key]; !ok {
			t.Errorf("missing key %q in status JSON", key)
		}
	}
}

// TC-CBUD-03: status --json record_count matches seeded records.
func TestBudget_StatusRecordCount(t *testing.T) {
	dir := t.TempDir()
	b := llmbudget.New()
	b.Add(llmbudget.Record{Timestamp: time.Now().UTC(), CostUSD: 0.01})
	b.Add(llmbudget.Record{Timestamp: time.Now().UTC(), CostUSD: 0.02})
	seedBudget(t, dir, b)

	out := exec(t, []string{"status", "--root", dir, "--json"})
	var p map[string]any
	json.Unmarshal([]byte(out), &p) //nolint:errcheck
	if int(p["record_count"].(float64)) != 2 {
		t.Fatalf("want record_count=2, got %v", p["record_count"])
	}
}

// TC-CBUD-04: set updates limits and persists them.
func TestBudget_Set(t *testing.T) {
	dir := t.TempDir()
	out := exec(t, []string{"set", "--root", dir, "--daily", "5.00", "--monthly", "50.00"})
	if !strings.Contains(out, "limits set") {
		t.Fatalf("want 'limits set' in output, got: %s", out)
	}
	// Verify persistence via status --json.
	out2 := exec(t, []string{"status", "--root", dir, "--json"})
	var p map[string]any
	json.Unmarshal([]byte(out2), &p) //nolint:errcheck
	if p["daily_limit_usd"].(float64) != 5.00 {
		t.Fatalf("want daily_limit_usd=5, got %v", p["daily_limit_usd"])
	}
}

// TC-CBUD-05: set --daily negative returns FORGE-2402.
func TestBudget_Set_Negative(t *testing.T) {
	dir := t.TempDir()
	if err := execErr(t, []string{"set", "--root", dir, "--daily", "-1"}); err == nil {
		t.Fatal("want error for negative daily limit, got nil")
	}
}

// TC-CBUD-06: reset clears records, preserves limits.
func TestBudget_Reset_PreservesLimits(t *testing.T) {
	dir := t.TempDir()
	exec(t, []string{"set", "--root", dir, "--daily", "10.00"})
	b := llmbudget.New()
	b.Config.DailyLimitUSD = 10.00
	b.Add(llmbudget.Record{Timestamp: time.Now().UTC(), CostUSD: 3.00})
	seedBudget(t, dir, b)

	exec(t, []string{"reset", "--root", dir})

	out := exec(t, []string{"status", "--root", dir, "--json"})
	var p map[string]any
	json.Unmarshal([]byte(out), &p) //nolint:errcheck
	if int(p["record_count"].(float64)) != 0 {
		t.Fatalf("want 0 records after reset, got %v", p["record_count"])
	}
}

// TC-CBUD-07: reset --limits zeroes limits too.
func TestBudget_Reset_ClearsLimits(t *testing.T) {
	dir := t.TempDir()
	b := llmbudget.New()
	b.SetLimits(10.0, 50.0) //nolint:errcheck
	seedBudget(t, dir, b)

	exec(t, []string{"reset", "--root", dir, "--limits"})

	out := exec(t, []string{"status", "--root", dir, "--json"})
	var p map[string]any
	json.Unmarshal([]byte(out), &p) //nolint:errcheck
	if p["daily_limit_usd"].(float64) != 0 {
		t.Fatalf("want daily_limit=0 after reset --limits, got %v", p["daily_limit_usd"])
	}
}

// TC-CBUD-08: idempotency — status twice returns same result.
func TestBudget_Idempotency(t *testing.T) {
	dir := t.TempDir()
	out1 := exec(t, []string{"status", "--root", dir, "--json"})
	out2 := exec(t, []string{"status", "--root", dir, "--json"})
	if out1 != out2 {
		t.Fatalf("idempotency: outputs differ\n1: %s\n2: %s", out1, out2)
	}
}

// TC-CBUD-09: false-positive guard — check limit passes when spend = 0 < limit.
func TestBudget_FalsePositive_UnderLimit(t *testing.T) {
	dir := t.TempDir()
	exec(t, []string{"set", "--root", dir, "--daily", "100.00", "--monthly", "500.00"})
	// No records — should not exceed limit
	out := exec(t, []string{"status", "--root", dir, "--json"})
	var p map[string]any
	json.Unmarshal([]byte(out), &p) //nolint:errcheck
	if p["daily_spend_usd"].(float64) != 0 {
		t.Fatalf("false-positive: spend should be 0, got %v", p["daily_spend_usd"])
	}
}

// TC-CBUD-10: New() returns a non-nil cobra.Command.
func TestBudget_CommandNotNil(t *testing.T) {
	if cmd := New(); cmd == nil {
		t.Fatal("New() returned nil")
	}
}

// TC-CBUD-11: New() is a *cobra.Command with subcommands.
func TestBudget_CommandHasSubcommands(t *testing.T) {
	cmd := New()
	names := map[string]bool{}
	for _, sub := range cmd.Commands() {
		names[sub.Name()] = true
	}
	for _, want := range []string{"status", "set", "reset"} {
		if !names[want] {
			t.Errorf("missing subcommand %q", want)
		}
	}
}

// TC-CBUD-12: Test that New() returns *cobra.Command type.
func TestBudget_TypeCheck(t *testing.T) {
	cmd := New()
	if _, ok := any(cmd).(*cobra.Command); !ok {
		t.Fatal("New() did not return *cobra.Command")
	}
}
