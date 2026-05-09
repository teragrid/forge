package cmddoctor

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestRun_HasChecks(t *testing.T) {
	t.Parallel()
	r := Run()
	if r.OS == "" || r.Arch == "" {
		t.Fatal("OS/Arch must be populated")
	}
	if len(r.Checks) < 3 {
		t.Fatalf("expected >=3 checks, got %d", len(r.Checks))
	}
}

func TestCmd_Text(t *testing.T) {
	t.Parallel()
	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(nil)
	// We don't assert healthy/unhealthy because CI runners differ; we only
	// assert the verb produces structured output and does not crash.
	_ = cmd.Execute()
	if !strings.Contains(out.String(), "forge doctor") {
		t.Fatalf("missing header: %s", out.String())
	}
}

func TestCmd_JSON(t *testing.T) {
	t.Parallel()
	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--json"})
	_ = cmd.Execute()

	var rep Report
	// Skip the trailing error message line if doctor is unhealthy on this host.
	body := bytes.TrimSpace(out.Bytes())
	// JSON must occupy the first object; trim anything past the closing brace.
	if i := bytes.LastIndexByte(body, '}'); i >= 0 {
		body = body[:i+1]
	}
	if err := json.Unmarshal(body, &rep); err != nil {
		t.Fatalf("not JSON: %v\noutput: %s", err, out.String())
	}
	if len(rep.Checks) == 0 {
		t.Fatal("expected checks in JSON output")
	}
}
