package cmdnew

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := New("9.9.9-test")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

func TestNew_Happy(t *testing.T) {
	t.Parallel()
	target := filepath.Join(t.TempDir(), "demo")
	out, err := runCmd(t, "go-service", target)
	if err != nil {
		t.Fatalf("run: %v\nout: %s", err, out)
	}
	if !strings.Contains(out, "scaffolded") {
		t.Fatalf("missing scaffold confirm: %s", out)
	}
	if _, err := os.Stat(filepath.Join(target, "main.go")); err != nil {
		t.Fatalf("main.go missing: %v", err)
	}
}

func TestNew_JSONOutput(t *testing.T) {
	t.Parallel()
	target := filepath.Join(t.TempDir(), "demo")
	out, err := runCmd(t, "go-service", target, "--json")
	if err != nil {
		t.Fatalf("run: %v\nout: %s", err, out)
	}
	var res struct {
		Template string   `json:"Template"`
		Files    []string `json:"Files"`
	}
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("not JSON: %v: %s", err, out)
	}
	if res.Template != "go-service" || len(res.Files) == 0 {
		t.Fatalf("bad json: %+v", res)
	}
}

func TestNew_UnknownTemplate(t *testing.T) {
	t.Parallel()
	target := filepath.Join(t.TempDir(), "demo")
	out, err := runCmd(t, "no-such-template", target)
	if err == nil {
		t.Fatalf("expected error, got: %s", out)
	}
	if !strings.Contains(err.Error(), "FORGE-2200") {
		t.Fatalf("want FORGE-2200, got %v", err)
	}
}

func TestNew_NeedsBothArgs(t *testing.T) {
	t.Parallel()
	if _, err := runCmd(t, "go-service"); err == nil {
		t.Fatal("expected arg-count error")
	}
}
