package cmdplugin

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
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

// ── helper ────────────────────────────────────────────────────────────────

func runCmdRoot(t *testing.T, root string, args ...string) (string, error) {
	t.Helper()
	cmd := New()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs(append(args, "--root", root))
	err := cmd.Execute()
	return buf.String(), err
}

// ── DEV-M2-02: install / upgrade / remove tests ───────────────────────────

// TC-02-01 (happy path): install → verify lock → upgrade → remove cycle.
func TestPlugin_InstallUpgradeRemove(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	out, err := runCmdRoot(t, dir, "install", "my-scanner@1.0.0")
	if err != nil {
		t.Fatalf("install: %v (out: %s)", err, out)
	}
	if !strings.Contains(out, "installed") {
		t.Errorf("expected 'installed' in output: %s", out)
	}
	lf, err := readLock(dir)
	if err != nil {
		t.Fatalf("readLock: %v", err)
	}
	if len(lf.Plugins) != 1 || lf.Plugins[0].Name != "my-scanner" || lf.Plugins[0].Version != "1.0.0" {
		t.Fatalf("unexpected lock: %+v", lf)
	}

	out, err = runCmdRoot(t, dir, "upgrade", "my-scanner", "--version", "2.0.0")
	if err != nil {
		t.Fatalf("upgrade: %v (out: %s)", err, out)
	}
	lf2, _ := readLock(dir)
	if lf2.Plugins[0].Version != "2.0.0" {
		t.Errorf("expected version 2.0.0, got %s", lf2.Plugins[0].Version)
	}

	out, err = runCmdRoot(t, dir, "remove", "my-scanner")
	if err != nil {
		t.Fatalf("remove: %v (out: %s)", err, out)
	}
	lf3, _ := readLock(dir)
	if len(lf3.Plugins) != 0 {
		t.Errorf("expected empty lock after remove, got %v", lf3.Plugins)
	}
}

// TC-02-02 (idempotency): same install twice → second is no-op.
func TestPlugin_Install_Idempotent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if _, err := runCmdRoot(t, dir, "install", "foo@1.0.0"); err != nil {
		t.Fatalf("first install: %v", err)
	}
	out, err := runCmdRoot(t, dir, "install", "foo@1.0.0")
	if err != nil {
		t.Fatalf("second install should not error: %v", err)
	}
	if !strings.Contains(out, "already installed") {
		t.Errorf("expected 'already installed' on second install: %s", out)
	}
	lf, _ := readLock(dir)
	if len(lf.Plugins) != 1 {
		t.Errorf("lock should still have exactly 1 entry, got %d", len(lf.Plugins))
	}
}

// TC-02-03 (negative): install without pinned version when lock already has different version.
func TestPlugin_Install_VersionConflict(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	lf := LockFile{Plugins: []LockEntry{{Name: "bar", Version: "1.0.0", Source: "in-tree"}}}
	if err := writeLock(dir, lf); err != nil {
		t.Fatalf("writeLock: %v", err)
	}
	_, err := runCmdRoot(t, dir, "install", "bar")
	if err == nil {
		t.Fatal("expected ErrPluginLockInvalid, got nil")
	}
	var fe *errcode.Error
	if !errors.As(err, &fe) || fe.Code != ErrPluginLockInvalid {
		t.Errorf("want FORGE-%d, got %v", ErrPluginLockInvalid, err)
	}
}

// TC-02-04: install of zero-length name → ErrPluginUsage.
func TestPlugin_Install_EmptyName(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_, err := runCmdRoot(t, dir, "install", "@1.0.0")
	if err == nil {
		t.Fatal("expected error for empty plugin name, got nil")
	}
}

// TC-02-05: upgrade of unknown plugin → ErrPluginUnknown.
func TestPlugin_Upgrade_Unknown(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_, err := runCmdRoot(t, dir, "upgrade", "ghost")
	if err == nil {
		t.Fatal("expected error for unknown plugin upgrade, got nil")
	}
	var fe *errcode.Error
	if !errors.As(err, &fe) || fe.Code != ErrPluginUnknown {
		t.Errorf("want FORGE-%d, got %v", ErrPluginUnknown, err)
	}
}

// TC-02-06: remove of unknown plugin → ErrPluginUnknown.
func TestPlugin_Remove_Unknown(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_, err := runCmdRoot(t, dir, "remove", "ghost")
	if err == nil {
		t.Fatal("expected error for unknown plugin remove, got nil")
	}
	var fe *errcode.Error
	if !errors.As(err, &fe) || fe.Code != ErrPluginUnknown {
		t.Errorf("want FORGE-%d, got %v", ErrPluginUnknown, err)
	}
}

// TC-02-07: lock file is valid JSON after round-trip.
func TestPlugin_LockFile_ValidJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if _, err := runCmdRoot(t, dir, "install", "alpha@0.1.0"); err != nil {
		t.Fatalf("install alpha: %v", err)
	}
	if _, err := runCmdRoot(t, dir, "install", "beta@0.2.0"); err != nil {
		t.Fatalf("install beta: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, lockFilePath))
	if err != nil {
		t.Fatalf("read lock: %v", err)
	}
	var parsed LockFile
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("invalid JSON in lock file: %v\n%s", err, raw)
	}
	if len(parsed.Plugins) != 2 {
		t.Errorf("expected 2 plugins, got %d", len(parsed.Plugins))
	}
}

// TC-02-08: remove one plugin, others survive.
func TestPlugin_Remove_PreservesOthers(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	for _, p := range []string{"a@1.0", "b@2.0", "c@3.0"} {
		if _, err := runCmdRoot(t, dir, "install", p); err != nil {
			t.Fatalf("install %s: %v", p, err)
		}
	}
	if _, err := runCmdRoot(t, dir, "remove", "b"); err != nil {
		t.Fatalf("remove b: %v", err)
	}
	lf, _ := readLock(dir)
	if len(lf.Plugins) != 2 {
		t.Fatalf("expected 2 remaining, got %d: %v", len(lf.Plugins), lf.Plugins)
	}
	for _, e := range lf.Plugins {
		if e.Name == "b" {
			t.Error("plugin 'b' still present after remove")
		}
	}
}
