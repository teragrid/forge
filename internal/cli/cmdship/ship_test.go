package cmdship

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestRun_DryRun(t *testing.T) {
	t.Parallel()
	res := Run(t.TempDir(), "test change")
	if !res.DryRun {
		t.Fatal("expected dry_run=true")
	}
	if len(res.Checkpoints) != 5 {
		t.Fatalf("expected 5 checkpoints, got %d", len(res.Checkpoints))
	}
}

func TestCmd_Text(t *testing.T) {
	t.Parallel()
	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(nil)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "5-checkpoint") {
		t.Fatalf("missing pipeline output: %s", out.String())
	}
}

func TestCmd_JSON(t *testing.T) {
	t.Parallel()
	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var res ShipResult
	if err := json.Unmarshal(out.Bytes(), &res); err != nil {
		t.Fatalf("not JSON: %v: %s", err, out.String())
	}
	if !res.DryRun || len(res.Checkpoints) == 0 {
		t.Fatalf("bad JSON: %+v", res)
	}
}
