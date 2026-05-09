package cmdeval

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/teragrid/forge/internal/errcode"
	"github.com/teragrid/forge/internal/eval"
)

func runCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := New()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return buf.String(), err
}

// writeScenario writes a JSON scenario file.
func writeScenario(t *testing.T, dir, name string, s eval.Scenario) string {
	t.Helper()
	body, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, body, 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// 1. Happy: a directory of one trivial scenario passes.
func TestEval_HappyPath_Dir(t *testing.T) {
	tmp := t.TempDir()
	zero := 0
	// On all OSes, `go version` is available because the test runner has Go.
	writeScenario(t, tmp, "go.scenario.json", eval.Scenario{
		Name: "go-runs",
		Steps: []eval.Step{{
			ID:  "v",
			Run: []string{"go", "version"},
			Expect: eval.Expect{
				Exit:           &zero,
				StdoutContains: []string{"go version"},
			},
		}},
	})
	out, err := runCmd(t, tmp)
	if err != nil {
		t.Fatalf("err: %v\nout: %s", err, out)
	}
	if !strings.Contains(out, "PASS") {
		t.Errorf("output missing PASS:\n%s", out)
	}
	if !strings.Contains(out, "1 passed, 0 failed") {
		t.Errorf("output missing summary:\n%s", out)
	}
}

// 2. JSON output: --json emits a valid Report.
func TestEval_JSONReport(t *testing.T) {
	tmp := t.TempDir()
	zero := 0
	writeScenario(t, tmp, "go.scenario.json", eval.Scenario{
		Name: "go-runs",
		Steps: []eval.Step{{
			Run:    []string{"go", "version"},
			Expect: eval.Expect{Exit: &zero},
		}},
	})
	out, err := runCmd(t, tmp, "--json")
	if err != nil {
		t.Fatalf("err: %v\nout: %s", err, out)
	}
	var rep eval.Report
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if rep.Total != 1 || rep.Passed != 1 || rep.Failed != 0 {
		t.Errorf("counts: %+v", rep)
	}
}

// 3. Negative: missing path → FORGE-3600.
func TestEval_MissingPath(t *testing.T) {
	_, err := runCmd(t, "no/such/path")
	if err == nil {
		t.Fatal("expected error for missing path")
	}
	var fe *errcode.Error
	if !errors.As(err, &fe) || fe.Code != ErrEvalLoadFailed {
		t.Errorf("want FORGE-%d, got %v", ErrEvalLoadFailed, err)
	}
}

// 4. CI gate: failing scenario returns FORGE-3601.
func TestEval_CIFailureExits(t *testing.T) {
	tmp := t.TempDir()
	zero := 0
	writeScenario(t, tmp, "fail.scenario.json", eval.Scenario{
		Name: "always-fails",
		Steps: []eval.Step{{
			Run: []string{"go", "version"},
			// Assert wrong exit code on purpose.
			Expect: eval.Expect{Exit: intPtr(99), StdoutContains: []string{"go version"}},
		}},
	})
	_ = zero
	_, err := runCmd(t, tmp)
	if err == nil {
		t.Fatal("expected CI failure")
	}
	var fe *errcode.Error
	if !errors.As(err, &fe) || fe.Code != ErrEvalScenarioFailed {
		t.Errorf("want FORGE-%d, got %v", ErrEvalScenarioFailed, err)
	}
}

// 5. False-positive guard: --ci=false suppresses non-zero exit.
func TestEval_CIFlagOff(t *testing.T) {
	tmp := t.TempDir()
	writeScenario(t, tmp, "fail.scenario.json", eval.Scenario{
		Name: "always-fails",
		Steps: []eval.Step{{
			Run:    []string{"go", "version"},
			Expect: eval.Expect{Exit: intPtr(99)},
		}},
	})
	_, err := runCmd(t, tmp, "--ci=false")
	if err != nil {
		t.Errorf("with --ci=false, expected nil error; got %v", err)
	}
}

// 6. Single-file argument also works.
func TestEval_SingleFileArg(t *testing.T) {
	tmp := t.TempDir()
	zero := 0
	p := writeScenario(t, tmp, "one.scenario.json", eval.Scenario{
		Name: "one",
		Steps: []eval.Step{{
			Run:    []string{"go", "version"},
			Expect: eval.Expect{Exit: &zero},
		}},
	})
	out, err := runCmd(t, p, "--json")
	if err != nil {
		t.Fatalf("err: %v\nout: %s", err, out)
	}
	var rep eval.Report
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if rep.Total != 1 {
		t.Errorf("total: want 1, got %d", rep.Total)
	}
}

// 7. Backward-compat / regression: scenario file with bad JSON returns FORGE-3600.
func TestEval_BadJSON(t *testing.T) {
	tmp := t.TempDir()
	bad := filepath.Join(tmp, "bad.scenario.json")
	if err := os.WriteFile(bad, []byte("{ not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := runCmd(t, tmp)
	if err == nil {
		t.Fatal("expected load error")
	}
	var fe *errcode.Error
	if !errors.As(err, &fe) || fe.Code != ErrEvalLoadFailed {
		t.Errorf("want FORGE-%d, got %v", ErrEvalLoadFailed, err)
	}
}

func intPtr(n int) *int { return &n }
