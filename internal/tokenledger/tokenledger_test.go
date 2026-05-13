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

package tokenledger_test

import (
	"math"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/teragrid/forge/internal/tokenledger"
)

func ledgerAt(t *testing.T) *tokenledger.Ledger {
	t.Helper()
	path := filepath.Join(t.TempDir(), ".forge", "token-ledger.jsonl")
	return tokenledger.New(path)
}

// ── Happy path ────────────────────────────────────────────────────────────────

func TestAppend_Happy(t *testing.T) {
	t.Parallel()
	l := ledgerAt(t)
	e := tokenledger.Entry{
		Model:        "claude-3-5-sonnet",
		InputTokens:  100,
		OutputTokens: 50,
		CostUSD:      0.00075,
		Operation:    "test-op",
	}
	if err := l.Append(e); err != nil {
		t.Fatalf("Append: %v", err)
	}
}

func TestReadAll_RoundTrip(t *testing.T) {
	t.Parallel()
	l := ledgerAt(t)
	want := tokenledger.Entry{
		Time:         time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
		Model:        "gpt-4o",
		InputTokens:  200,
		OutputTokens: 80,
		CostUSD:      0.0028,
		Operation:    "scaffold",
	}
	if err := l.Append(want); err != nil {
		t.Fatal(err)
	}
	entries, err := l.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	got := entries[0]
	if got.Model != want.Model {
		t.Errorf("model: got %q want %q", got.Model, want.Model)
	}
	if got.InputTokens != want.InputTokens {
		t.Errorf("input tokens: got %d want %d", got.InputTokens, want.InputTokens)
	}
	if got.OutputTokens != want.OutputTokens {
		t.Errorf("output tokens: got %d want %d", got.OutputTokens, want.OutputTokens)
	}
	if math.Abs(got.CostUSD-want.CostUSD) > 1e-9 {
		t.Errorf("cost: got %f want %f", got.CostUSD, want.CostUSD)
	}
}

func TestTotalCost_MultipleEntries(t *testing.T) {
	t.Parallel()
	l := ledgerAt(t)
	costs := []float64{0.001, 0.002, 0.0005}
	for _, c := range costs {
		if err := l.Append(tokenledger.Entry{Model: "m", CostUSD: c}); err != nil {
			t.Fatal(err)
		}
	}
	total, err := l.TotalCost()
	if err != nil {
		t.Fatal(err)
	}
	want := 0.001 + 0.002 + 0.0005
	if math.Abs(total-want) > 1e-9 {
		t.Errorf("total cost: got %f want %f", total, want)
	}
}

func TestSummary_ByModel(t *testing.T) {
	t.Parallel()
	l := ledgerAt(t)
	entries := []tokenledger.Entry{
		{Model: "claude", InputTokens: 100, OutputTokens: 50, CostUSD: 0.001},
		{Model: "claude", InputTokens: 200, OutputTokens: 80, CostUSD: 0.002},
		{Model: "gpt-4o", InputTokens: 150, OutputTokens: 60, CostUSD: 0.003},
	}
	for _, e := range entries {
		if err := l.Append(e); err != nil {
			t.Fatal(err)
		}
	}
	s, err := l.Summary()
	if err != nil {
		t.Fatal(err)
	}
	if s.TotalCalls != 3 {
		t.Errorf("total calls: got %d want 3", s.TotalCalls)
	}
	if len(s.ByModel) != 2 {
		t.Errorf("model count: got %d want 2", len(s.ByModel))
	}
	claude := s.ByModel["claude"]
	if claude == nil {
		t.Fatal("claude model missing from summary")
	}
	if claude.Calls != 2 {
		t.Errorf("claude calls: got %d want 2", claude.Calls)
	}
	if claude.InputTokens != 300 {
		t.Errorf("claude input tokens: got %d want 300", claude.InputTokens)
	}
}

// ── Boundary: empty ledger ────────────────────────────────────────────────────

func TestReadAll_Empty(t *testing.T) {
	t.Parallel()
	l := ledgerAt(t)
	entries, err := l.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll on empty ledger: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}

func TestTotalCost_Empty(t *testing.T) {
	t.Parallel()
	l := ledgerAt(t)
	total, err := l.TotalCost()
	if err != nil {
		t.Fatal(err)
	}
	if total != 0 {
		t.Errorf("expected 0 for empty ledger, got %f", total)
	}
}

func TestSummary_Empty(t *testing.T) {
	t.Parallel()
	l := ledgerAt(t)
	s, err := l.Summary()
	if err != nil {
		t.Fatal(err)
	}
	if s.TotalCalls != 0 {
		t.Errorf("expected 0 calls for empty ledger")
	}
}

// ── Boundary: zero cost ───────────────────────────────────────────────────────

func TestAppend_ZeroCost(t *testing.T) {
	t.Parallel()
	l := ledgerAt(t)
	if err := l.Append(tokenledger.Entry{Model: "m", CostUSD: 0}); err != nil {
		t.Fatal(err)
	}
	total, _ := l.TotalCost()
	if total != 0 {
		t.Errorf("expected 0 cost, got %f", total)
	}
}

// ── Idempotency / append-only ─────────────────────────────────────────────────

func TestAppend_MultipleAppendsGrow(t *testing.T) {
	t.Parallel()
	l := ledgerAt(t)
	for i := 0; i < 5; i++ {
		if err := l.Append(tokenledger.Entry{Model: "m", CostUSD: 0.001}); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := l.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 5 {
		t.Errorf("expected 5 entries, got %d", len(entries))
	}
}

// ── Concurrency ───────────────────────────────────────────────────────────────

func TestAppend_Concurrent(t *testing.T) {
	t.Parallel()
	l := ledgerAt(t)
	const workers = 20
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			_ = l.Append(tokenledger.Entry{Model: "m", CostUSD: 0.001})
		}()
	}
	wg.Wait()

	entries, err := l.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll after concurrent writes: %v", err)
	}
	if len(entries) != workers {
		t.Errorf("expected %d entries, got %d", workers, len(entries))
	}
}

// ── Data accuracy ─────────────────────────────────────────────────────────────

// Append an entry with known timestamp, read back, verify round-trip fidelity.
func TestAppend_TimeRoundTrip(t *testing.T) {
	t.Parallel()
	l := ledgerAt(t)
	ts := time.Date(2024, 6, 15, 12, 30, 0, 0, time.UTC)
	if err := l.Append(tokenledger.Entry{Time: ts, Model: "m"}); err != nil {
		t.Fatal(err)
	}
	entries, _ := l.ReadAll()
	if !entries[0].Time.Equal(ts) {
		t.Errorf("time mismatch: got %v want %v", entries[0].Time, ts)
	}
}

// ── False-positive guard ──────────────────────────────────────────────────────

// Two distinct models must produce two distinct ByModel entries.
func TestSummary_TwoModelsAreDistinct(t *testing.T) {
	t.Parallel()
	l := ledgerAt(t)
	_ = l.Append(tokenledger.Entry{Model: "a", CostUSD: 1.0})
	_ = l.Append(tokenledger.Entry{Model: "b", CostUSD: 2.0})

	s, _ := l.Summary()
	if s.ByModel["a"].TotalCostUSD == s.ByModel["b"].TotalCostUSD {
		t.Error("distinct models must not share the same cost aggregate")
	}
}
