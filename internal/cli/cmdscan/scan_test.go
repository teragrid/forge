package cmdscan

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestScan_SecretsClean(t *testing.T) {
	t.Parallel()
	// Smoke test that scan runs without crashing on a clean project.
	res, err := RunSecrets(t.TempDir())
	if err != nil {
		t.Fatalf("RunSecrets: %v", err)
	}
	if res.Status != "clean" && len(res.Findings) == 0 {
		t.Fatalf("empty dir should be clean, got status=%q findings=%d", res.Status, len(res.Findings))
	}
}

func TestCmd_Text(t *testing.T) {
	t.Parallel()
	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"secrets", "--root", t.TempDir()})
	_ = cmd.Execute() // may fail on non-clean, but should not crash
	if !strings.Contains(out.String(), "forge scan") {
		t.Fatalf("missing header: %s", out.String())
	}
}

func TestCmd_JSON(t *testing.T) {
	t.Parallel()
	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"secrets", "--root", t.TempDir(), "--json"})
	_ = cmd.Execute()

	var res ScanResult
	body := bytes.TrimSpace(out.Bytes())
	if i := bytes.LastIndexByte(body, '}'); i >= 0 {
		body = body[:i+1]
	}
	if err := json.Unmarshal(body, &res); err != nil {
		t.Fatalf("not JSON: %v", err)
	}
	if res.Status == "" {
		t.Fatal("expected Status in JSON output")
	}
}

func TestCmd_UnknownScanner(t *testing.T) {
	t.Parallel()
	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"no-such-scanner"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for unknown scanner")
	}
	if !strings.Contains(err.Error(), "FORGE-3000") {
		t.Fatalf("want FORGE-3000, got: %v", err)
	}
}
