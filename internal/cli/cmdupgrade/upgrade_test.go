package cmdupgrade

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TC-UPGRADE-01 (happy): list shows built-in codemods.
func TestUpgrade_ListText(t *testing.T) {
	t.Parallel()
	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"list"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	s := out.String()
	if !strings.Contains(s, "gitignore-marker") || !strings.Contains(s, "gitleaks-baseline") {
		t.Fatalf("missing built-ins: %s", s)
	}
}

// TC-UPGRADE-02 (happy + data-accuracy): JSON list parses.
func TestUpgrade_ListJSON(t *testing.T) {
	t.Parallel()
	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"list", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var got []map[string]string
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("not JSON: %v", err)
	}
	if len(got) < 2 {
		t.Fatalf("expected ≥2 codemods, got %d", len(got))
	}
}

// TC-UPGRADE-03 (negative): unknown codemod errors with FORGE-3300.
func TestUpgrade_UnknownCodemod(t *testing.T) {
	t.Parallel()
	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"no-such"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "FORGE-3300") {
		t.Fatalf("want FORGE-3300, got: %v", err)
	}
}

// TC-UPGRADE-04 (boundary + idempotency): dry-run twice does not mutate.
func TestUpgrade_DryRunNoMutation(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"gitignore-marker", "--root", dir})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("dry-run: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".gitignore")); !os.IsNotExist(err) {
		t.Fatal("dry-run created .gitignore")
	}
}

// TC-UPGRADE-05 (happy): --apply actually writes the file.
func TestUpgrade_ApplyWrites(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"gitignore-marker", "--root", dir, "--apply"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("apply: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatalf("apply did not write: %v", err)
	}
	if !strings.Contains(string(body), "forge:gitignore:start") {
		t.Fatalf("marker missing: %s", body)
	}
}
