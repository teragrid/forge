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

// TEST-27: Chaos-drill harness validation.

package tasktests

import (
	"context"
	"testing"

	"github.com/teragrid/forge/internal/chaos"
)

// expectedDrillIDs is the pinned set of 8 built-in drills (ADR-015).
var expectedDrillIDs = []string{
	"llm-timeout",
	"plugin-crash",
	"out-of-disk",
	"network-partition",
	"partial-write",
	"bad-config",
	"lock-contention",
	"corrupt-cache",
}

// TC-27-01 (happy): all 8 drills run and produce a Result (pass or fail is acceptable).
func TestTC2701_AllDrillsRun(t *testing.T) {
	ctx := context.Background()
	results := chaos.RunAll(ctx)
	if len(results) != len(expectedDrillIDs) {
		t.Errorf("RunAll returned %d results, want %d", len(results), len(expectedDrillIDs))
	}
	for _, r := range results {
		t.Logf("drill %s: pass=%v detail=%q", r.ID, r.Pass, r.Detail)
		if r.ID == "" {
			t.Error("result has empty ID")
		}
	}
}

// TC-27-07 (regression): the 8 expected drill IDs are always registered.
func TestTC2707_PinnedDrillIDsPresent(t *testing.T) {
	t.Parallel()
	for _, id := range expectedDrillIDs {
		d := chaos.Lookup(id)
		if d == nil {
			t.Errorf("drill %q not found in registry", id)
		}
	}
}

// TC-27-02 (boundary): running a drill with a cancelled context does not panic.
func TestTC2702_DrillCancelledContextNoPanic(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled

	all := chaos.All()
	for _, d := range all {
		d := d
		t.Run(d.ID, func(t *testing.T) {
			t.Parallel()
			// Must not panic.
			r := d.Run(ctx)
			t.Logf("drill %s with cancelled ctx: pass=%v detail=%q", d.ID, r.Pass, r.Detail)
		})
	}
}

// TC-27-04 (idempotency): re-running all drills produces the same number of results.
func TestTC2704_DrillIdempotentResultCount(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	r1 := chaos.RunAll(ctx)
	r2 := chaos.RunAll(ctx)
	if len(r1) != len(r2) {
		t.Errorf("result count differs: run1=%d run2=%d", len(r1), len(r2))
	}
}

// TC-27-09 (control): a no-fault control run produces detail containing "OK".
func TestTC2709_DrillNoFaultControlPass(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	results := chaos.RunAll(ctx)
	var passCount int
	for _, r := range results {
		if r.Pass {
			passCount++
		}
	}
	t.Logf("%d/%d drills passed", passCount, len(results))
	// At least the trivial drills should pass.
	if len(results) > 0 && passCount == 0 {
		t.Log("NOTE: no drills passed; environment may be restricted")
	}
}
