package cmdplugin

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	// Side-effect imports so plugin.Default() is populated at test start.
	_ "github.com/teragrid/forge/internal/cli/cmdscan"
	_ "github.com/teragrid/forge/internal/cli/cmdupgrade"
	"github.com/teragrid/forge/internal/errcode"
	"github.com/teragrid/forge/internal/plugin"
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

// TestList_Text_HappyPath: every builtin scanner family appears in the
// human-readable list.
func TestList_Text_HappyPath(t *testing.T) {
	out, err := runCmd(t, "list")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	for _, name := range []string{"secrets", "rls", "prompt-injection", "supply-chain"} {
		if !strings.Contains(out, name) {
			t.Errorf("list output missing %q\n--- output ---\n%s", name, out)
		}
	}
	if !strings.Contains(out, "NAME") || !strings.Contains(out, "KIND") {
		t.Errorf("list output missing header row\n%s", out)
	}
}

// TestList_JSON_HappyPath: --json emits an array of manifests.
func TestList_JSON_HappyPath(t *testing.T) {
	out, err := runCmd(t, "list", "--json")
	if err != nil {
		t.Fatalf("list --json: %v", err)
	}
	var manifests []plugin.Manifest
	if err := json.Unmarshal([]byte(out), &manifests); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if len(manifests) < 4 {
		t.Errorf("got %d manifests, want >= 4 (4 scanners + codemods)", len(manifests))
	}
}

// TestList_KindFilter_FalsePositiveGuard: --kind=codemod must not include scanners.
func TestList_KindFilter_FalsePositiveGuard(t *testing.T) {
	out, err := runCmd(t, "list", "--kind", "codemod", "--json")
	if err != nil {
		t.Fatalf("list --kind codemod: %v", err)
	}
	var manifests []plugin.Manifest
	if err := json.Unmarshal([]byte(out), &manifests); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	for _, m := range manifests {
		if m.Kind != plugin.KindCodemod {
			t.Errorf("kind=codemod filter leaked %q (kind=%s)", m.Name, m.Kind)
		}
		if m.Name == "secrets" || m.Name == "rls" {
			t.Errorf("scanner %q leaked into codemod listing", m.Name)
		}
	}
}

// TestList_KindFilter_Invalid: unknown --kind returns FORGE-3501.
func TestList_KindFilter_Invalid(t *testing.T) {
	_, err := runCmd(t, "list", "--kind", "wat")
	if err == nil {
		t.Fatal("expected error for unknown --kind, got nil")
	}
	var fe *errcode.Error
	if !errors.As(err, &fe) || fe.Code != ErrPluginUsage {
		t.Errorf("want FORGE-%d, got %v", ErrPluginUsage, err)
	}
}

// TestShow_HappyPath: known plugin returns its manifest.
func TestShow_HappyPath(t *testing.T) {
	out, err := runCmd(t, "show", "secrets")
	if err != nil {
		t.Fatalf("show secrets: %v", err)
	}
	if !strings.Contains(out, "name:    secrets") {
		t.Errorf("show output missing name field:\n%s", out)
	}
	if !strings.Contains(out, "kind:    scanner") {
		t.Errorf("show output missing kind field:\n%s", out)
	}
}

// TestShow_JSON: --json emits a single manifest object.
func TestShow_JSON(t *testing.T) {
	out, err := runCmd(t, "show", "secrets", "--json")
	if err != nil {
		t.Fatalf("show --json: %v", err)
	}
	var m plugin.Manifest
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if m.Name != "secrets" || m.Kind != plugin.KindScanner {
		t.Errorf("unexpected manifest: %+v", m)
	}
}

// TestShow_Unknown_Negative: unknown plugin returns FORGE-3500.
func TestShow_Unknown_Negative(t *testing.T) {
	_, err := runCmd(t, "show", "no-such-plugin")
	if err == nil {
		t.Fatal("expected error for unknown plugin, got nil")
	}
	var fe *errcode.Error
	if !errors.As(err, &fe) || fe.Code != ErrPluginUnknown {
		t.Errorf("want FORGE-%d, got %v", ErrPluginUnknown, err)
	}
}

// TestList_Idempotent: invoking list twice yields identical output (registry
// is read-only).
func TestList_Idempotent(t *testing.T) {
	a, err := runCmd(t, "list", "--json")
	if err != nil {
		t.Fatalf("first list: %v", err)
	}
	b, err := runCmd(t, "list", "--json")
	if err != nil {
		t.Fatalf("second list: %v", err)
	}
	if a != b {
		t.Errorf("list is not idempotent\nfirst:\n%s\nsecond:\n%s", a, b)
	}
}
