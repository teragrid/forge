package cmdlint

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRun_ProjectWithManifest(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	_ = os.MkdirAll(filepath.Join(root, ".forge"), 0o755)
	_ = os.WriteFile(filepath.Join(root, ".forge", "manifest"), []byte("test"), 0o600)
	_ = os.WriteFile(filepath.Join(root, ".gitignore"), []byte("forge:gitignore:start\n*.tmp\nforge:gitignore:end\n"), 0o600)
	_ = os.WriteFile(filepath.Join(root, ".gitleaks.toml"), []byte("test"), 0o600)

	res := Run(root)
	if !res.Passed {
		t.Fatalf("expected passed, got errors=%d issues=%v", res.Errors, res.Issues)
	}
}

func TestRun_MissingManifest(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	res := Run(root)
	if res.Passed {
		t.Fatal("empty dir should fail on missing manifest")
	}
	if res.Errors == 0 {
		t.Fatal("expected at least one error")
	}
}

func TestCmd_Text(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--root", root})
	_ = cmd.Execute()
	if !strings.Contains(out.String(), "forge lint") {
		t.Fatalf("missing header: %s", out.String())
	}
}

func TestCmd_JSON(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--root", root, "--json"})
	_ = cmd.Execute()

	var res LintResult
	body := bytes.TrimSpace(out.Bytes())
	if i := bytes.LastIndexByte(body, '}'); i >= 0 {
		body = body[:i+1]
	}
	if err := json.Unmarshal(body, &res); err != nil {
		t.Fatalf("not JSON: %v", err)
	}
	if res.Root == "" {
		t.Fatal("expected Root in JSON output")
	}
}
