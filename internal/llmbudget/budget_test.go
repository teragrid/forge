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
package llmbudget

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TC-LB-01: happy path — Add records and DailySpend returns correct sum.
func TestBudget_DailySpend(t *testing.T) {
	b := New()
	now := time.Now().UTC()
	b.Add(Record{Timestamp: now, CostUSD: 0.01})
	b.Add(Record{Timestamp: now, CostUSD: 0.02})
	if got := b.DailySpend(now); got != 0.03 {
		t.Fatalf("DailySpend: want 0.03, got %g", got)
	}
}

// TC-LB-02: MonthlySpend includes records from different days in the same month.
func TestBudget_MonthlySpend(t *testing.T) {
	b := New()
	now := time.Now().UTC()
	yesterday := now.AddDate(0, 0, -1)
	b.Add(Record{Timestamp: now, CostUSD: 0.10})
	b.Add(Record{Timestamp: yesterday, CostUSD: 0.05})
	got := b.MonthlySpend(now)
	// Use integer cents to avoid float64 precision drift.
	if int(got*10000+0.5) != 1500 {
		t.Fatalf("MonthlySpend: want 0.15, got %g", got)
	}
}

// TC-LB-03: boundary — empty budget returns $0 spend.
func TestBudget_EmptySpend(t *testing.T) {
	b := New()
	if got := b.DailySpend(time.Now()); got != 0 {
		t.Fatalf("empty daily spend: want 0, got %g", got)
	}
	if got := b.MonthlySpend(time.Now()); got != 0 {
		t.Fatalf("empty monthly spend: want 0, got %g", got)
	}
}

// TC-LB-04: negative limit is rejected.
func TestBudget_SetLimits_Negative(t *testing.T) {
	b := New()
	if err := b.SetLimits(-1, 0); err == nil {
		t.Fatal("want error for negative daily limit, got nil")
	}
	if err := b.SetLimits(0, -1); err == nil {
		t.Fatal("want error for negative monthly limit, got nil")
	}
}

// TC-LB-05: CheckLimits passes when under limit.
func TestBudget_CheckLimits_UnderLimit(t *testing.T) {
	b := New()
	b.SetLimits(1.00, 10.00) //nolint:errcheck
	b.Add(Record{Timestamp: time.Now().UTC(), CostUSD: 0.50})
	if err := b.CheckLimits(time.Now()); err != nil {
		t.Fatalf("CheckLimits: unexpected error: %v", err)
	}
}

// TC-LB-06: CheckLimits fails at daily limit.
func TestBudget_CheckLimits_DailyExceeded(t *testing.T) {
	b := New()
	b.SetLimits(1.00, 0) //nolint:errcheck
	b.Add(Record{Timestamp: time.Now().UTC(), CostUSD: 1.00})
	if err := b.CheckLimits(time.Now()); err == nil {
		t.Fatal("CheckLimits: want error at daily limit, got nil")
	}
}

// TC-LB-07: CheckLimits fails at monthly limit.
func TestBudget_CheckLimits_MonthlyExceeded(t *testing.T) {
	b := New()
	b.SetLimits(0, 5.00) //nolint:errcheck
	b.Add(Record{Timestamp: time.Now().UTC(), CostUSD: 5.00})
	if err := b.CheckLimits(time.Now()); err == nil {
		t.Fatal("CheckLimits: want error at monthly limit, got nil")
	}
}

// TC-LB-08: Zero limits are treated as unlimited (false-positive guard).
func TestBudget_CheckLimits_ZeroLimitsUnlimited(t *testing.T) {
	b := New()
	for i := 0; i < 100; i++ {
		b.Add(Record{Timestamp: time.Now().UTC(), CostUSD: 999.99})
	}
	if err := b.CheckLimits(time.Now()); err != nil {
		t.Fatalf("zero limits should be unlimited, got error: %v", err)
	}
}

// TC-LB-09: Reset clears records, preserves limits.
func TestBudget_Reset_PreservesLimits(t *testing.T) {
	b := New()
	b.SetLimits(10.00, 100.00) //nolint:errcheck
	b.Add(Record{Timestamp: time.Now().UTC(), CostUSD: 5.00})
	b.Reset(false)
	if len(b.Records) != 0 {
		t.Fatalf("Reset: want 0 records, got %d", len(b.Records))
	}
	if b.Config.DailyLimitUSD != 10.00 {
		t.Fatalf("Reset: limits should be preserved, got daily=%g", b.Config.DailyLimitUSD)
	}
}

// TC-LB-10: Reset with resetLimits=true zeroes Config.
func TestBudget_Reset_ClearsLimits(t *testing.T) {
	b := New()
	b.SetLimits(10.00, 100.00) //nolint:errcheck
	b.Reset(true)
	if b.Config.DailyLimitUSD != 0 || b.Config.MonthlyLimitUSD != 0 {
		t.Fatal("Reset(true): limits should be zeroed")
	}
}

// TC-LB-11: Save / Load round-trip preserves records and limits.
func TestBudget_SaveLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, DefaultPath)

	b := New()
	b.SetLimits(5.00, 50.00) //nolint:errcheck
	b.Add(Record{Timestamp: time.Now().UTC(), Verb: "scan", Model: "gpt-4o", CostUSD: 0.25})

	if err := b.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	b2, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(b2.Records) != 1 {
		t.Fatalf("Load: want 1 record, got %d", len(b2.Records))
	}
	if b2.Records[0].Verb != "scan" {
		t.Fatalf("Load: want verb=scan, got %s", b2.Records[0].Verb)
	}
	if b2.Config.DailyLimitUSD != 5.00 {
		t.Fatalf("Load: want daily=5, got %g", b2.Config.DailyLimitUSD)
	}
}

// TC-LB-12: missing file returns New() (no error).
func TestBudget_MissingFile(t *testing.T) {
	b, err := Load(filepath.Join(t.TempDir(), "nonexistent.json"))
	if err != nil {
		t.Fatalf("Load missing: %v", err)
	}
	if len(b.Records) != 0 {
		t.Fatalf("want 0 records, got %d", len(b.Records))
	}
}

// TC-LB-13: bad JSON returns error.
func TestBudget_BadJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.json")
	os.WriteFile(path, []byte("not json"), 0o600) //nolint:errcheck
	if _, err := Load(path); err == nil {
		t.Fatal("want error for bad JSON, got nil")
	}
}

// TC-LB-14: idempotent save — second save produces same output.
func TestBudget_IdempotentSave(t *testing.T) {
	path := filepath.Join(t.TempDir(), DefaultPath)
	b := New()
	b.Add(Record{Timestamp: time.Date(2026, 5, 9, 0, 0, 0, 0, time.UTC), CostUSD: 0.01})
	b.Save(path) //nolint:errcheck
	first, _ := os.ReadFile(path)
	b.Save(path) //nolint:errcheck
	second, _ := os.ReadFile(path)
	if string(first) != string(second) {
		t.Fatal("idempotent save: outputs differ")
	}
}

// TC-LB-15: JSON keys are snake_case as specified.
func TestBudget_JSONKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), DefaultPath)
	b := New()
	b.SetLimits(1.0, 10.0) //nolint:errcheck
	b.Save(path)           //nolint:errcheck
	data, _ := os.ReadFile(path)
	var raw map[string]any
	json.Unmarshal(data, &raw) //nolint:errcheck
	if _, ok := raw["api_version"]; !ok {
		t.Error("missing key api_version")
	}
	if _, ok := raw["config"]; !ok {
		t.Error("missing key config")
	}
}

// TC-LB-16: DailySpend excludes records from other days.
func TestBudget_DailySpend_ExcludesOtherDays(t *testing.T) {
	b := New()
	today := time.Now().UTC()
	yesterday := today.AddDate(0, 0, -1)
	b.Add(Record{Timestamp: today, CostUSD: 0.10})
	b.Add(Record{Timestamp: yesterday, CostUSD: 0.50})
	if got := b.DailySpend(today); got != 0.10 {
		t.Fatalf("DailySpend excludes other days: want 0.10, got %g", got)
	}
}

// TC-LB-17: MonthlySpend excludes records from other months.
func TestBudget_MonthlySpend_ExcludesOtherMonths(t *testing.T) {
	b := New()
	now := time.Date(2026, 5, 15, 0, 0, 0, 0, time.UTC)
	lastMonth := time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC)
	b.Add(Record{Timestamp: now, CostUSD: 1.00})
	b.Add(Record{Timestamp: lastMonth, CostUSD: 9.00})
	if got := b.MonthlySpend(now); got != 1.00 {
		t.Fatalf("MonthlySpend excludes other months: want 1.00, got %g", got)
	}
}
// ── G-040: TestBudget_PerVerbLimits ──────────────────────────────────────────

// TestBudget_PerVerbLimits verifies that per-verb daily limits are enforced.
func TestBudget_PerVerbLimits(t *testing.T) {
        t.Parallel()
        b := New()
        verbs := []string{"scan", "fix", "ship", "deploy", "learn"}
        const verbLimit = 0.10

        // Set a daily limit for each verb.
        for _, v := range verbs {
                if err := b.SetVerbLimit(v, verbLimit); err != nil {
                        t.Fatalf("SetVerbLimit(%q): %v", v, err)
                }
        }

        now := time.Now().UTC()

        // Under limit: no error.
        b.Add(Record{Timestamp: now, Verb: "scan", CostUSD: 0.05})
        if err := b.CheckVerbLimit("scan", now); err != nil {
                t.Errorf("CheckVerbLimit under limit: unexpected error: %v", err)
        }

        // At/over limit: must return non-nil error.
        b.Add(Record{Timestamp: now, Verb: "scan", CostUSD: 0.06}) // total 0.11
        if err := b.CheckVerbLimit("scan", now); err == nil {
                t.Error("CheckVerbLimit over limit: want error, got nil")
        }

        // Other verbs are unaffected.
        if err := b.CheckVerbLimit("fix", now); err != nil {
                t.Errorf("CheckVerbLimit(fix) should be zero spend, got error: %v", err)
        }
}

// TestBudget_PerVerbLimit_ZeroUnlimited verifies that setting a zero limit
// removes the per-verb limit (effectively unlimited).
func TestBudget_PerVerbLimit_ZeroUnlimited(t *testing.T) {
        t.Parallel()
        b := New()
        now := time.Now().UTC()
        b.SetVerbLimit("scan", 0.01) //nolint:errcheck
        b.Add(Record{Timestamp: now, Verb: "scan", CostUSD: 1.00})
        // Remove limit by setting to 0.
        b.SetVerbLimit("scan", 0) //nolint:errcheck
        if err := b.CheckVerbLimit("scan", now); err != nil {
                t.Errorf("CheckVerbLimit after removal: want nil, got %v", err)
        }
}

// TestBudget_VerbDailySpend_IsolatesVerbs verifies that VerbDailySpend only
// counts records for the given verb.
func TestBudget_VerbDailySpend_IsolatesVerbs(t *testing.T) {
        t.Parallel()
        b := New()
        now := time.Now().UTC()
        b.Add(Record{Timestamp: now, Verb: "scan", CostUSD: 0.30})
        b.Add(Record{Timestamp: now, Verb: "fix", CostUSD: 0.20})
        if got := b.VerbDailySpend("scan", now); got != 0.30 {
                t.Errorf("VerbDailySpend(scan): want 0.30, got %g", got)
        }
        if got := b.VerbDailySpend("fix", now); got != 0.20 {
                t.Errorf("VerbDailySpend(fix): want 0.20, got %g", got)
        }
        if got := b.VerbDailySpend("deploy", now); got != 0.00 {
                t.Errorf("VerbDailySpend(deploy): want 0.00, got %g", got)
        }
}