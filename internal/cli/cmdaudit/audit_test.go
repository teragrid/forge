package cmdaudit

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// TC-CMDAUDIT-01 (happy): append → show shows entry.
func TestAudit_AppendThenShow(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	app := New()
	var aOut bytes.Buffer
	app.SetOut(&aOut)
	app.SetArgs([]string{"append", "--root", dir, "--verb", "scan", "--action", "secrets-clean"})
	if err := app.Execute(); err != nil {
		t.Fatalf("append: %v", err)
	}

	show := New()
	var sOut bytes.Buffer
	show.SetOut(&sOut)
	show.SetArgs([]string{"show", "--root", dir})
	if err := show.Execute(); err != nil {
		t.Fatalf("show: %v", err)
	}
	if !strings.Contains(sOut.String(), "scan/secrets-clean") {
		t.Fatalf("entry not shown: %s", sOut.String())
	}
}

// TC-CMDAUDIT-02 (happy + data-accuracy): verify reports intact JSON.
func TestAudit_VerifyJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	app := New()
	app.SetOut(new(bytes.Buffer))
	app.SetArgs([]string{"append", "--root", dir, "--verb", "x", "--action", "y"})
	_ = app.Execute()

	v := New()
	var out bytes.Buffer
	v.SetOut(&out)
	v.SetArgs([]string{"verify", "--root", dir, "--json"})
	if err := v.Execute(); err != nil {
		t.Fatalf("verify: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("not JSON: %v", err)
	}
	if got["intact"] != true {
		t.Fatalf("not intact: %+v", got)
	}
}

// TC-CMDAUDIT-03 (negative): append without --verb fails.
func TestAudit_AppendRequiresVerbAction(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"append", "--root", dir, "--action", "x"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "FORGE-3400") {
		t.Fatalf("want FORGE-3400, got: %v", err)
	}
}

// TC-CMDAUDIT-04 (negative): unknown subcommand errors.
func TestAudit_UnknownSubcommand(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"whatever", "--root", dir})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
}

// TC-CMDAUDIT-05 (boundary): show on empty ledger.
func TestAudit_ShowEmpty(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"show", "--root", dir})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("show empty: %v", err)
	}
	if !strings.Contains(out.String(), "0 entries") {
		t.Fatalf("expected '0 entries': %s", out.String())
	}
}
