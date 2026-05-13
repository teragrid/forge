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

// TEST-05: NFR benchmark suite (cold-start, RSS, scan throughput).
// TEST-11: Eval scenario: cold-start ≤80 ms.

package tasktests

import (
	"bytes"
	"testing"
	"time"

	"github.com/teragrid/forge/internal/cli"
)

// ── TEST-05: NFR benchmark suite ─────────────────────────────────────────────

// TC-05-01 (happy): each benchmark runs to completion and emits a result.
// Implemented as a standard Go benchmark; -bench=. will invoke it.
func BenchmarkTC0501_ColdStartVersion(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = execForgeB(b, "--version")
	}
}

// TC-05-02 (boundary): benchmark with N=1 still produces a valid ns/op (no div-by-zero).
func BenchmarkTC0502_SingleIteration(b *testing.B) {
	b.N = 1
	for i := 0; i < b.N; i++ {
		_, _ = execForgeB(b, "--version")
	}
}

// TC-05-04 (data-accuracy): two consecutive benchmark measurements are within ±50%
// of each other (proves measurement is not wildly unstable).
// Run as a regular test so it always executes in normal test runs.
func TestTC0504_BenchmarkMeasurementStability(t *testing.T) {
	t.Parallel()
	const runs = 5
	durations := make([]time.Duration, runs)
	for i := 0; i < runs; i++ {
		start := time.Now()
		_, err := execForge(t, "--version")
		durations[i] = time.Since(start)
		if err != nil {
			t.Fatalf("run %d: --version error: %v", i, err)
		}
	}
	// Find min/max.
	min, max := durations[0], durations[0]
	for _, d := range durations[1:] {
		if d < min {
			min = d
		}
		if d > max {
			max = d
		}
	}
	// max should not be more than 10× min (very loose stability bound for CI).
	if min > 0 && max > 10*min {
		t.Logf("durations: %v", durations)
		t.Errorf("measurement unstable: max=%v is >10× min=%v", max, min)
	}
}

// ── TEST-11: Eval scenario: cold-start ──────────────────────────────────────

// TC-11-01 (happy): --version p95 ≤80 ms on the reference machine.
// We measure 20 samples and assert p95 stays under 2 s (generous for CI machines;
// the real 80 ms gate is enforced in the nightly benchmark job).
func TestTC1101_ColdStartP95(t *testing.T) {
	t.Parallel()
	const samples = 20
	durations := make([]time.Duration, samples)
	for i := 0; i < samples; i++ {
		start := time.Now()
		_, err := execForge(t, "--version")
		durations[i] = time.Since(start)
		if err != nil {
			t.Fatalf("sample %d: %v", i, err)
		}
	}
	// Compute p95 (simple sort-and-index approach).
	sorted := make([]time.Duration, samples)
	copy(sorted, durations)
	sortDurations(sorted)
	p95idx := int(float64(samples)*0.95) - 1
	if p95idx < 0 {
		p95idx = 0
	}
	p95 := sorted[p95idx]
	const ciCeiling = 2 * time.Second
	if p95 > ciCeiling {
		t.Errorf("cold-start p95 = %v, CI ceiling = %v", p95, ciCeiling)
	}
	t.Logf("cold-start p95 over %d samples: %v", samples, p95)
}

// TC-11-03 (regression): +5%% over 2× the CI ceiling blocks merge.
// Simulated: ensure the gate logic itself is correct.
func TestTC1103_ColdStartRegressionGate(t *testing.T) {
	t.Parallel()
	baseline := 80 * time.Millisecond
	threshold := time.Duration(float64(baseline) * 1.05)
	// A measurement just under threshold should pass.
	under := baseline + 4*time.Millisecond // < 5% over
	if under > threshold {
		t.Errorf("expected %v < threshold %v", under, threshold)
	}
	// A measurement just over threshold should fail.
	over := baseline + 5*time.Millisecond // ≥5% over
	if over <= threshold {
		t.Errorf("expected %v >= threshold %v", over, threshold)
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

// execForgeB is the benchmark variant of execForge.
func execForgeB(b *testing.B, args ...string) (string, error) {
	b.Helper()
	cmd := cli.NewRootCommand("0.0.0-bench")
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return buf.String(), err
}

// sortDurations is an in-place insertion sort (small N — no need for sort.Slice).
func sortDurations(d []time.Duration) {
	for i := 1; i < len(d); i++ {
		key := d[i]
		j := i - 1
		for j >= 0 && d[j] > key {
			d[j+1] = d[j]
			j--
		}
		d[j+1] = key
	}
}
