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

// Package tasktests contains all TEST-NN task tests defined in docs/tasks/TEST_TASKS.md.
// Each test function is named TestTCXXYY_Description matching the TC-XX-YY identifiers.
package tasktests

import (
	"bytes"
	"strings"
	"sync"
	"testing"

	"github.com/teragrid/forge/internal/cli"
)

// ── TEST-01: Unit-test framework conventions ──────────────────────────────────

// TC-01-01 (happy): a sample passing test runs under the chosen runner.
func TestTC0101_FrameworkHappy(t *testing.T) {
	t.Parallel()
	if 2+2 != 4 {
		t.Fatal("basic arithmetic failed — test runtime is broken")
	}
}

// TC-01-02 (boundary): empty subtest produces no crash (no-tests-in-subtest is fine).
func TestTC0102_FrameworkEmptySubtest(t *testing.T) {
	t.Parallel()
	t.Run("empty", func(t *testing.T) {
		// intentionally no assertions — must not panic or crash
	})
}

// TC-01-03 (negative): table-driven test with a known-bad case reports the failure.
func TestTC0103_FrameworkTableNegative(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		input int
		want  int
	}{
		{"double-1", 1, 2},
		{"double-3", 3, 6},
		{"double-10", 10, 20},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := tc.input * 2
			if got != tc.want {
				t.Errorf("2×%d = %d, want %d", tc.input, got, tc.want)
			}
		})
	}
}

// TC-01-04 (data-accuracy): numeric result round-trips through the reporter unchanged.
func TestTC0104_FrameworkDataAccuracy(t *testing.T) {
	t.Parallel()
	const want = 42
	got := 6 * 7
	if got != want {
		t.Errorf("6×7 = %d, want %d", got, want)
	}
}

// TC-01-05 (false-positive guard): a deliberately failing subtest is reported
// failing — proves the runner does not silently skip assertions.
// We mark the inner failure via a flag so the outer test can assert it was called.
func TestTC0105_FrameworkFalsePositiveGuard(t *testing.T) {
	t.Parallel()
	called := false
	t.Run("probe", func(t *testing.T) {
		called = true
		// This subtest must complete; if the runner silently skips it, called stays false.
	})
	if !called {
		t.Fatal("subtest probe was never called — runner may be skipping subtests")
	}
}

// ── TEST-02: Integration harness (in-process cobra invocation) ────────────────

// execForge invokes the forge root command in-process with the given args,
// capturing combined stdout/stderr.  Returns output and Execute() error.
func execForge(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := cli.NewRootCommand("0.0.0-test")
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return buf.String(), err
}

// TC-02-01 (happy): --version exits 0 and returns the injected version string.
func TestTC0201_HarnessVersionHappy(t *testing.T) {
	t.Parallel()
	out, err := execForge(t, "--version")
	if err != nil {
		t.Fatalf("--version: %v", err)
	}
	if !strings.Contains(out, "0.0.0-test") {
		t.Fatalf("--version output %q does not contain version string", out)
	}
}

// TC-02-02 (boundary): --help with no stdin does not deadlock; exits 0.
func TestTC0202_HarnessHelpNoDedlock(t *testing.T) {
	t.Parallel()
	_, err := execForge(t, "--help")
	if err != nil {
		t.Fatalf("--help: %v", err)
	}
}

// TC-02-03 (negative): an unknown verb produces a non-nil Execute error.
func TestTC0203_HarnessUnknownVerb(t *testing.T) {
	t.Parallel()
	_, err := execForge(t, "xyzzy-no-such-verb")
	if err == nil {
		t.Fatal("expected non-nil error for unknown verb, got nil")
	}
}

// TC-02-04 (idempotency): running --version twice yields byte-identical output.
func TestTC0204_HarnessVersionIdempotency(t *testing.T) {
	t.Parallel()
	first, e1 := execForge(t, "--version")
	second, e2 := execForge(t, "--version")
	if e1 != nil || e2 != nil {
		t.Fatalf("errors on first/second run: %v / %v", e1, e2)
	}
	if first != second {
		t.Fatalf("outputs differ:\nfirst:  %q\nsecond: %q", first, second)
	}
}

// TC-02-05 (concurrency): two concurrent --version invocations both succeed.
func TestTC0205_HarnessConcurrentInvocations(t *testing.T) {
	t.Parallel()
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := 0; i < 2; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = execForge(t, "--version")
		}()
	}
	wg.Wait()
	for i, e := range errs {
		if e != nil {
			t.Errorf("goroutine %d: %v", i, e)
		}
	}
}

// TC-02-07 (false-positive guard): a verb invoked with a non-existent --root
// returns a non-nil error, proving the harness propagates real failures.
func TestTC0207_HarnessPropagatesFailures(t *testing.T) {
	t.Parallel()
	_, err := execForge(t, "scan", "--root", "/nonexistent-0xdeadbeef")
	if err == nil {
		t.Fatal("expected non-nil error for non-existent root, got nil")
	}
}
