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

// prometheus_test.go — tests for ExportPrometheus on the token ledger.

package tokenledger

import (
	"strings"
	"testing"
	"time"
)

// helper to build a temp ledger path.
func newLedgerPath(t *testing.T) string {
	t.Helper()
	return t.TempDir() + "/ledger.jsonl"
}

// TestExportPrometheus_EmptyLedger verifies that an empty ledger returns the
// two metric families with zero values and does not error.
func TestExportPrometheus_EmptyLedger(t *testing.T) {
	t.Parallel()
	path := newLedgerPath(t)
	l := New(path)
	out, err := l.ExportPrometheus()
	if err != nil {
		t.Fatalf("ExportPrometheus on empty ledger: %v", err)
	}
	// Must contain both metric family headers.
	if !strings.Contains(out, "forge_tokens_total") {
		t.Error("missing forge_tokens_total metric family")
	}
	if !strings.Contains(out, "forge_cost_usd_total") {
		t.Error("missing forge_cost_usd_total metric family")
	}
}

// TestExportPrometheus_HappyPath verifies a single entry produces non-zero values.
func TestExportPrometheus_HappyPath(t *testing.T) {
	t.Parallel()
	path := newLedgerPath(t)
	l := New(path)
	entry := Entry{
		Time:         time.Now(),
		Model:        "gpt-4",
		InputTokens:  100,
		OutputTokens: 50,
		CostUSD:      0.03,
		Operation:    "ship",
	}
	if err := l.Append(entry); err != nil {
		t.Fatalf("Append: %v", err)
	}
	out, err := l.ExportPrometheus()
	if err != nil {
		t.Fatalf("ExportPrometheus: %v", err)
	}
	// Token values should be non-zero.
	if strings.Contains(out, "forge_tokens_total 0") {
		t.Error("expected non-zero forge_tokens_total")
	}
	if !strings.Contains(out, "gpt-4") {
		t.Error("expected model label in Prometheus output")
	}
}

// TestExportPrometheus_AggregatesAcrossEntries verifies multiple entries are summed.
func TestExportPrometheus_AggregatesAcrossEntries(t *testing.T) {
	t.Parallel()
	path := newLedgerPath(t)
	l := New(path)
	for i := 0; i < 3; i++ {
		if err := l.Append(Entry{
			Time:         time.Now(),
			Model:        "claude-3-sonnet",
			InputTokens:  10,
			OutputTokens: 5,
			CostUSD:      0.001,
			Operation:    "scan",
		}); err != nil {
			t.Fatalf("Append[%d]: %v", i, err)
		}
	}
	out, err := l.ExportPrometheus()
	if err != nil {
		t.Fatalf("ExportPrometheus: %v", err)
	}
	// 3 × (10+5) = 45 tokens.  Prometheus output should not show 0 or 15 total.
	if !strings.Contains(out, "claude-3-sonnet") {
		t.Error("expected model label claude-3-sonnet in output")
	}
	// Check cumulative cost non-zero.
	if strings.Contains(out, "forge_cost_usd_total 0") {
		t.Error("expected non-zero forge_cost_usd_total after 3 appends")
	}
}

// TestExportPrometheus_MultipleModels verifies separate metric series per model.
func TestExportPrometheus_MultipleModels(t *testing.T) {
	t.Parallel()
	path := newLedgerPath(t)
	l := New(path)
	models := []string{"gpt-4", "claude-3-haiku", "gemini-pro"}
	for _, m := range models {
		if err := l.Append(Entry{
			Time:         time.Now(),
			Model:        m,
			InputTokens:  20,
			OutputTokens: 10,
			CostUSD:      0.002,
			Operation:    "review",
		}); err != nil {
			t.Fatalf("Append %s: %v", m, err)
		}
	}
	out, err := l.ExportPrometheus()
	if err != nil {
		t.Fatalf("ExportPrometheus: %v", err)
	}
	for _, m := range models {
		if !strings.Contains(out, m) {
			t.Errorf("expected model %q in Prometheus output", m)
		}
	}
}

// TestExportPrometheus_FormatCheck verifies the output is valid Prometheus exposition.
// It checks the required structural elements: HELP, TYPE lines, and label syntax.
func TestExportPrometheus_FormatCheck(t *testing.T) {
	t.Parallel()
	path := newLedgerPath(t)
	l := New(path)
	if err := l.Append(Entry{
		Time: time.Now(), Model: "test-model",
		InputTokens: 5, OutputTokens: 5, CostUSD: 0.001,
		Operation: "test",
	}); err != nil {
		t.Fatalf("Append: %v", err)
	}
	out, err := l.ExportPrometheus()
	if err != nil {
		t.Fatalf("ExportPrometheus: %v", err)
	}
	// Valid exposition format requires # HELP and # TYPE lines.
	if !strings.Contains(out, "# HELP forge_tokens_total") {
		t.Error("missing # HELP forge_tokens_total line")
	}
	if !strings.Contains(out, "# TYPE forge_tokens_total") {
		t.Error("missing # TYPE forge_tokens_total line")
	}
	// Labels use {key="value"} syntax.
	if !strings.Contains(out, `model="test-model"`) {
		t.Errorf("expected label model=\"test-model\" in output, got:\n%s", out)
	}
}

// TestExportPrometheus_Idempotency verifies calling twice returns identical output.
func TestExportPrometheus_Idempotency(t *testing.T) {
	t.Parallel()
	path := newLedgerPath(t)
	l := New(path)
	if err := l.Append(Entry{
		Time:         time.Now(),
		Model:        "gpt-4o",
		InputTokens:  10,
		OutputTokens: 10,
		CostUSD:      0.01,
		Operation:    "ship",
	}); err != nil {
		t.Fatalf("Append: %v", err)
	}
	out1, err := l.ExportPrometheus()
	if err != nil {
		t.Fatalf("first ExportPrometheus: %v", err)
	}
	out2, err := l.ExportPrometheus()
	if err != nil {
		t.Fatalf("second ExportPrometheus: %v", err)
	}
	if out1 != out2 {
		t.Error("ExportPrometheus is not idempotent — output differed across two calls")
	}
}
