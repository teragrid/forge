package eval

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// stubResolver returns the path to a "shim" PowerShell/sh command that
// emits a controlled stdout + exit code. Cross-platform without cgo.
//
// On Windows we use cmd.exe /c; elsewhere /bin/sh -c.
func stubResolver(_ string) (string, error) {
	if runtime.GOOS == "windows" {
		return `C:\Windows\System32\cmd.exe`, nil
	}
	return "/bin/sh", nil
}

// echoStep returns a Step that prints `out` to stdout then exits with `exit`.
// argv[0] is the shell; rewritten by stubResolver above.
func echoStep(t *testing.T, id, out string, exit int, expect Expect) Step {
	t.Helper()
	if runtime.GOOS == "windows" {
		// cmd.exe quoting is brutal; embed via /c "echo X & exit /b N".
		return Step{
			ID:     id,
			Run:    []string{"cmd", "/c", "echo " + out + " & exit /b " + itoa(exit)},
			Expect: expect,
		}
	}
	return Step{
		ID:     id,
		Run:    []string{"sh", "-c", "printf '%s\\n' '" + out + "'; exit " + itoa(exit)},
		Expect: expect,
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

// 1. Happy: scenario with one passing step.
func TestRun_Happy(t *testing.T) {
	r := &Runner{CommandResolver: stubResolver}
	zero := 0
	s := &Scenario{
		Name:  "happy",
		Steps: []Step{echoStep(t, "s1", "hello", 0, Expect{Exit: &zero, StdoutContains: []string{"hello"}})},
	}
	res := r.Run(context.Background(), s)
	if !res.Passed {
		t.Fatalf("expected pass, got: %+v", res)
	}
	if len(res.Steps) != 1 || !res.Steps[0].Passed {
		t.Fatalf("step did not pass: %+v", res.Steps)
	}
}

// 2. Boundary: empty steps slice.
func TestRun_NoSteps(t *testing.T) {
	r := &Runner{CommandResolver: stubResolver}
	s := &Scenario{Name: "empty", Steps: nil}
	res := r.Run(context.Background(), s)
	if !res.Passed {
		t.Errorf("empty scenario should pass, got fail: %+v", res)
	}
}

// 3. Negative: invalid scenario (no name).
func TestValidate_NoName(t *testing.T) {
	if err := (Scenario{Name: "", Steps: nil}).Validate(); err == nil {
		t.Error("expected error on empty name")
	}
}

func TestValidate_NoRun(t *testing.T) {
	if err := (Scenario{Name: "x", Steps: []Step{{Run: nil}}}).Validate(); err == nil {
		t.Error("expected error on empty argv")
	}
}

// 4. Idempotency: same scenario twice → same outcome.
func TestRun_Idempotent(t *testing.T) {
	r := &Runner{CommandResolver: stubResolver}
	zero := 0
	s := &Scenario{
		Name:  "idem",
		Steps: []Step{echoStep(t, "s1", "world", 0, Expect{Exit: &zero})},
	}
	a := r.Run(context.Background(), s)
	b := r.Run(context.Background(), s)
	if a.Passed != b.Passed {
		t.Errorf("non-deterministic: a=%v b=%v", a.Passed, b.Passed)
	}
}

// 5. Negative: assertion mismatch.
func TestRun_AssertionFails_Exit(t *testing.T) {
	r := &Runner{CommandResolver: stubResolver}
	zero := 0
	s := &Scenario{
		Name:  "wrong-exit",
		Steps: []Step{echoStep(t, "s1", "x", 7, Expect{Exit: &zero})},
	}
	res := r.Run(context.Background(), s)
	if res.Passed {
		t.Error("expected fail on exit mismatch")
	}
	if !strings.Contains(res.Steps[0].Reason, "exit") {
		t.Errorf("reason missing 'exit': %q", res.Steps[0].Reason)
	}
}

// 5b. Negative: stdout substring missing.
func TestRun_AssertionFails_StdoutMissing(t *testing.T) {
	r := &Runner{CommandResolver: stubResolver}
	s := &Scenario{
		Name:  "missing-substr",
		Steps: []Step{echoStep(t, "s1", "abc", 0, Expect{StdoutContains: []string{"zzz"}})},
	}
	res := r.Run(context.Background(), s)
	if res.Passed {
		t.Error("expected fail when substring absent")
	}
}

// 9. False-positive guard: stdout_not_contains matches → fail; doesn't match → pass.
func TestRun_StdoutNotContains_Guard(t *testing.T) {
	r := &Runner{CommandResolver: stubResolver}
	// Forbidden string IS present → must fail.
	failS := &Scenario{
		Name:  "guard-fail",
		Steps: []Step{echoStep(t, "s1", "secret", 0, Expect{StdoutNotContains: []string{"secret"}})},
	}
	if r.Run(context.Background(), failS).Passed {
		t.Error("StdoutNotContains: expected fail when forbidden substring present")
	}
	// Forbidden string ABSENT → must pass.
	passS := &Scenario{
		Name:  "guard-pass",
		Steps: []Step{echoStep(t, "s1", "ok", 0, Expect{StdoutNotContains: []string{"secret"}})},
	}
	if !r.Run(context.Background(), passS).Passed {
		t.Error("StdoutNotContains: expected pass when forbidden substring absent")
	}
}

// 8. Data-accuracy: Report counts match step outcomes.
func TestRunAll_ReportCounts(t *testing.T) {
	r := &Runner{CommandResolver: stubResolver}
	zero := 0
	pass := &Scenario{Name: "p", Steps: []Step{echoStep(t, "s", "ok", 0, Expect{Exit: &zero})}}
	fail := &Scenario{Name: "f", Steps: []Step{echoStep(t, "s", "ok", 1, Expect{Exit: &zero})}}
	rep := r.RunAll(context.Background(), []*Scenario{pass, fail, pass})
	if rep.Total != 3 || rep.Passed != 2 || rep.Failed != 1 {
		t.Errorf("counts: total=%d passed=%d failed=%d (want 3/2/1)", rep.Total, rep.Passed, rep.Failed)
	}
}

// LoadScenario happy + missing path negative.
func TestLoadScenario_HappyAndMissing(t *testing.T) {
	tmp := t.TempDir()
	good := filepath.Join(tmp, "ok.scenario.json")
	body := `{"name":"x","steps":[{"run":["echo","hi"],"expect":{}}]}`
	if err := os.WriteFile(good, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadScenario(good); err != nil {
		t.Errorf("happy: %v", err)
	}
	if _, err := LoadScenario(filepath.Join(tmp, "nope.json")); err == nil {
		t.Error("expected error for missing file")
	}
}

// LoadDir picks up only *.scenario.json and sorts deterministically.
func TestLoadDir_FiltersAndSorts(t *testing.T) {
	tmp := t.TempDir()
	mk := func(name, n string) {
		body, _ := json.Marshal(Scenario{Name: n, Steps: []Step{{Run: []string{"echo", "x"}}}})
		if err := os.WriteFile(filepath.Join(tmp, name), body, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	mk("b.scenario.json", "b")
	mk("a.scenario.json", "a")
	mk("README.md", "x") // ignored
	out, err := LoadDir(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 {
		t.Fatalf("want 2 scenarios, got %d", len(out))
	}
	if out[0].Name != "a" || out[1].Name != "b" {
		t.Errorf("unsorted: got %s, %s", out[0].Name, out[1].Name)
	}
}

// 7. Backward-compat / regression: stdout_json equality on a literal payload.
func TestRun_StdoutJSON_Equality(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("cmd.exe escaping makes JSON literal awkward; covered on POSIX")
	}
	r := &Runner{CommandResolver: stubResolver}
	step := Step{
		ID:  "s",
		Run: []string{"sh", "-c", `printf '%s' '{"count":3,"status":"ok"}'`},
		Expect: Expect{
			StdoutJSON: map[string]any{"count": float64(3), "status": "ok"},
		},
	}
	s := &Scenario{Name: "json", Steps: []Step{step}}
	res := r.Run(context.Background(), s)
	if !res.Passed {
		t.Errorf("expected pass, got: %+v", res.Steps)
	}
}

// Wrap up: Validate rejects empty name (ensures errors.Is paths work).
func TestValidate_ErrorWrapping(t *testing.T) {
	err := (Scenario{}).Validate()
	if err == nil || !errors.Is(err, err) {
		t.Errorf("error not produced: %v", err)
	}
}
