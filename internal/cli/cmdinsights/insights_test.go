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

package cmdinsights

import (
	"testing"
	"time"

	"github.com/teragrid/forge/internal/audit"
)

// G-092: QuickSmell is set when --quick usage exceeds 20% of ship runs in 30d.

// makeShipEntry returns an audit.Entry for a ship run, optionally flagging --quick.
func makeShipEntry(quick bool) audit.Entry {
	detail := map[string]string{}
	if quick {
		detail["flag"] = "--quick"
	}
	return audit.Entry{
		Verb:      "ship",
		Action:    "run",
		Timestamp: time.Now().UTC(),
		Detail:    detail,
	}
}

// TestQuickRatio_SmellRaised verifies that QuickSmell=true and QuickRatio30d>0.20
// when more than 20% of recent ship runs used --quick.
func TestQuickRatio_SmellRaised(t *testing.T) {
	t.Parallel()
	// 3 quick out of 5 ship runs = 60% → well above 20% threshold.
	entries := []audit.Entry{
		makeShipEntry(true),
		makeShipEntry(true),
		makeShipEntry(true),
		makeShipEntry(false),
		makeShipEntry(false),
	}
	r := buildReport(entries, time.Time{}, "")

	if !r.QuickSmell {
		t.Errorf("expected QuickSmell=true when ratio=%.2f > 0.20", r.QuickRatio30d)
	}
	if r.QuickRatio30d <= 0.20 {
		t.Errorf("expected QuickRatio30d > 0.20, got %.4f", r.QuickRatio30d)
	}
}

// TestQuickRatio_NoSmell verifies QuickSmell=false when --quick usage is ≤ 20%.
func TestQuickRatio_NoSmell(t *testing.T) {
	t.Parallel()
	// 1 quick out of 10 ship runs = 10% → below 20% threshold.
	entries := make([]audit.Entry, 9)
	for i := range entries {
		entries[i] = makeShipEntry(false)
	}
	entries = append(entries, makeShipEntry(true))

	r := buildReport(entries, time.Time{}, "")

	if r.QuickSmell {
		t.Errorf("expected QuickSmell=false when ratio=%.2f ≤ 0.20", r.QuickRatio30d)
	}
}

// TestQuickRatio_NoShipRuns verifies QuickRatio30d=0 and QuickSmell=false when
// there are no ship runs at all.
func TestQuickRatio_NoShipRuns(t *testing.T) {
	t.Parallel()
	entries := []audit.Entry{
		{Verb: "scan", Action: "run", Timestamp: time.Now().UTC()},
		{Verb: "learn", Action: "add", Timestamp: time.Now().UTC()},
	}
	r := buildReport(entries, time.Time{}, "")

	if r.QuickSmell {
		t.Error("expected QuickSmell=false when there are no ship runs")
	}
	if r.QuickRatio30d != 0 {
		t.Errorf("expected QuickRatio30d=0, got %.4f", r.QuickRatio30d)
	}
}

// TestQuickRatio_ExactlyAtThreshold verifies QuickSmell=false at exactly 20%.
func TestQuickRatio_ExactlyAtThreshold(t *testing.T) {
	t.Parallel()
	// 1 quick out of 5 = 20.0% → NOT above threshold (threshold is > 0.20).
	entries := []audit.Entry{
		makeShipEntry(true),
		makeShipEntry(false),
		makeShipEntry(false),
		makeShipEntry(false),
		makeShipEntry(false),
	}
	r := buildReport(entries, time.Time{}, "")

	if r.QuickSmell {
		t.Errorf("expected QuickSmell=false at exactly 20%%, got ratio=%.4f", r.QuickRatio30d)
	}
}

// TestQuickRatio_OldEntriesExcluded verifies that ship runs older than 30 days
// are not counted in the ratio.
func TestQuickRatio_OldEntriesExcluded(t *testing.T) {
	t.Parallel()
	old := time.Now().UTC().AddDate(0, 0, -31) // 31 days ago — outside window
	entries := []audit.Entry{
		// Many old --quick runs that should be ignored.
		{Verb: "ship", Action: "run", Timestamp: old, Detail: map[string]string{"flag": "--quick"}},
		{Verb: "ship", Action: "run", Timestamp: old, Detail: map[string]string{"flag": "--quick"}},
		{Verb: "ship", Action: "run", Timestamp: old, Detail: map[string]string{"flag": "--quick"}},
		// One recent non-quick run.
		makeShipEntry(false),
	}
	r := buildReport(entries, time.Time{}, "")

	// Only the 1 recent non-quick run counts → ratio = 0%.
	if r.QuickSmell {
		t.Errorf("expected QuickSmell=false (old entries excluded), got ratio=%.4f", r.QuickRatio30d)
	}
	if r.QuickRatio30d != 0 {
		t.Errorf("expected QuickRatio30d=0 (old entries excluded), got %.4f", r.QuickRatio30d)
	}
}

// TestInsightsQuickRatio is the G-092 spec-exact test name.
// Verifies that QuickSmell is raised when --quick usage exceeds 20% of ship runs.
func TestInsightsQuickRatio(t *testing.T) {
	t.Parallel()
	// 4 quick out of 5 ship runs = 80% → well above 20% threshold.
	entries := []audit.Entry{
		makeShipEntry(true),
		makeShipEntry(true),
		makeShipEntry(true),
		makeShipEntry(true),
		makeShipEntry(false),
	}
	r := buildReport(entries, time.Time{}, "")

	if !r.QuickSmell {
		t.Errorf("expected QuickSmell=true when quick ratio=%.2f > 0.20", r.QuickRatio30d)
	}
	if r.QuickRatio30d <= 0.20 {
		t.Errorf("QuickRatio30d = %.4f, want > 0.20", r.QuickRatio30d)
	}
}
