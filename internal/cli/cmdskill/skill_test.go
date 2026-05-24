// Copyright 2024 The Forge Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmdskill_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/teragrid/forge/internal/cli/cmdskill"
)

// ── helpers ───────────────────────────────────────────────────────────────────

func runCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := cmdskill.New()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs(args)
	err := root.Execute()
	return buf.String(), err
}

func expectedPaths(dir, name string) []string {
	return []string{
		filepath.Join(dir, ".github", "chatmodes", name+".chatmode.md"),
		filepath.Join(dir, ".github", "instructions", name+".instructions.md"),
		filepath.Join(dir, ".github", "prompts", "forge-setup.prompt.md"),
		filepath.Join(dir, ".github", "prompts", "forge-ship.prompt.md"),
		filepath.Join(dir, ".github", "prompts", "forge-scan.prompt.md"),
		filepath.Join(dir, ".github", "prompts", "forge-bugfix.prompt.md"),
	}
}

// ── install happy path ────────────────────────────────────────────────────────

func TestInstall_CreatesAllFiles(t *testing.T) {
	dir := t.TempDir()
	out, err := runCmd(t, "install", "--root", dir, "--name", "forge-expert")
	if err != nil {
		t.Fatalf("install: %v\noutput: %s", err, out)
	}
	for _, p := range expectedPaths(dir, "forge-expert") {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("expected file to exist: %s", p)
		}
	}
}

func TestInstall_ChatmodeContainsForgeCmds(t *testing.T) {
	dir := t.TempDir()
	_, err := runCmd(t, "install", "--root", dir)
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, ".github", "chatmodes", "forge-expert.chatmode.md"))
	if err != nil {
		t.Fatalf("read chatmode: %v", err)
	}
	for _, want := range []string{"forge ship", "forge scan", "forge bugfix", "forge new", "forge config"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("chatmode missing %q", want)
		}
	}
}

func TestInstall_CustomName(t *testing.T) {
	dir := t.TempDir()
	_, err := runCmd(t, "install", "--root", dir, "--name", "my-ai")
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	for _, p := range expectedPaths(dir, "my-ai") {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("expected file: %s", p)
		}
	}
	// Default name files must NOT be created.
	defaultChatmode := filepath.Join(dir, ".github", "chatmodes", "forge-expert.chatmode.md")
	if _, err := os.Stat(defaultChatmode); err == nil {
		t.Error("default-named chatmode should not be created when --name is overridden")
	}
}

// ── install boundary/idempotency ──────────────────────────────────────────────

func TestInstall_SkipsExistingWithoutForce(t *testing.T) {
	dir := t.TempDir()
	// First install.
	_, err := runCmd(t, "install", "--root", dir)
	if err != nil {
		t.Fatalf("first install: %v", err)
	}
	// Overwrite the chatmode with sentinel content.
	chatmodePath := filepath.Join(dir, ".github", "chatmodes", "forge-expert.chatmode.md")
	sentinel := "sentinel-content-do-not-overwrite"
	if err := os.WriteFile(chatmodePath, []byte(sentinel), 0o644); err != nil {
		t.Fatal(err)
	}
	// Second install without --force: should skip existing file.
	out, err := runCmd(t, "install", "--root", dir)
	if err != nil {
		t.Fatalf("second install: %v", err)
	}
	if !strings.Contains(out, "skipped") {
		t.Errorf("expected 'skipped' in output; got: %s", out)
	}
	data, _ := os.ReadFile(chatmodePath)
	if string(data) != sentinel {
		t.Error("existing file was overwritten without --force")
	}
}

func TestInstall_ForceOverwrites(t *testing.T) {
	dir := t.TempDir()
	_, _ = runCmd(t, "install", "--root", dir)
	chatmodePath := filepath.Join(dir, ".github", "chatmodes", "forge-expert.chatmode.md")
	_ = os.WriteFile(chatmodePath, []byte("old"), 0o644)

	_, err := runCmd(t, "install", "--root", dir, "--force")
	if err != nil {
		t.Fatalf("force install: %v", err)
	}
	data, _ := os.ReadFile(chatmodePath)
	if string(data) == "old" {
		t.Error("--force did not overwrite existing file")
	}
	if !strings.Contains(string(data), "forge ship") {
		t.Error("overwritten chatmode missing forge content")
	}
}

// ── dry-run ───────────────────────────────────────────────────────────────────

func TestInstall_DryRun_WritesNoFiles(t *testing.T) {
	dir := t.TempDir()
	out, err := runCmd(t, "install", "--root", dir, "--dry-run")
	if err != nil {
		t.Fatalf("dry-run: %v", err)
	}
	if !strings.Contains(out, "would write") {
		t.Errorf("expected 'would write' in dry-run output; got: %s", out)
	}
	for _, p := range expectedPaths(dir, "forge-expert") {
		if _, err := os.Stat(p); err == nil {
			t.Errorf("dry-run must not write files, but found: %s", p)
		}
	}
}

// ── JSON output ───────────────────────────────────────────────────────────────

func TestInstall_JSONOutput(t *testing.T) {
	dir := t.TempDir()
	out, err := runCmd(t, "install", "--root", dir, "--json")
	if err != nil {
		t.Fatalf("install --json: %v", err)
	}
	var res cmdskill.InstallResult
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("unmarshal JSON: %v\nraw: %s", err, out)
	}
	if len(res.Written) == 0 {
		t.Error("expected at least one written file in JSON output")
	}
	if res.Root != dir {
		t.Errorf("root = %q, want %q", res.Root, dir)
	}
}

// ── list ─────────────────────────────────────────────────────────────────────

func TestList_BeforeInstall(t *testing.T) {
	dir := t.TempDir()
	out, err := runCmd(t, "list", "--root", dir)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if !strings.Contains(out, "missing") {
		t.Errorf("expected 'missing' before install; got: %s", out)
	}
}

func TestList_AfterInstall(t *testing.T) {
	dir := t.TempDir()
	_, _ = runCmd(t, "install", "--root", dir)
	out, err := runCmd(t, "list", "--root", dir)
	if err != nil {
		t.Fatalf("list after install: %v", err)
	}
	if !strings.Contains(out, "present") {
		t.Errorf("expected 'present' after install; got: %s", out)
	}
}

func TestList_JSONOutput(t *testing.T) {
	dir := t.TempDir()
	_, _ = runCmd(t, "install", "--root", dir)
	out, err := runCmd(t, "list", "--root", dir, "--json")
	if err != nil {
		t.Fatalf("list --json: %v", err)
	}
	var files []cmdskill.SkillFile
	if err := json.Unmarshal([]byte(out), &files); err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, out)
	}
	if len(files) == 0 {
		t.Error("expected at least one file in JSON list output")
	}
}

// ── remove ────────────────────────────────────────────────────────────────────

func TestRemove_DeletesFiles(t *testing.T) {
	dir := t.TempDir()
	_, _ = runCmd(t, "install", "--root", dir)
	out, err := runCmd(t, "remove", "--root", dir, "--yes")
	if err != nil {
		t.Fatalf("remove: %v", err)
	}
	if !strings.Contains(out, "removed") {
		t.Errorf("expected 'removed' in output; got: %s", out)
	}
	for _, p := range expectedPaths(dir, "forge-expert") {
		if _, err := os.Stat(p); err == nil {
			t.Errorf("file should be deleted after remove: %s", p)
		}
	}
}

func TestRemove_NothingToRemove(t *testing.T) {
	dir := t.TempDir()
	out, err := runCmd(t, "remove", "--root", dir, "--yes")
	if err != nil {
		t.Fatalf("remove on empty dir: %v", err)
	}
	if !strings.Contains(out, "nothing to remove") {
		t.Errorf("expected 'nothing to remove'; got: %s", out)
	}
}

// ── cobra registration ────────────────────────────────────────────────────────

func TestNew_ReturnsNonNilCommand(t *testing.T) {
	t.Parallel()
	cmd := cmdskill.New()
	if cmd == nil {
		t.Fatal("New() returned nil")
	}
	if cmd.Use != "skill" {
		t.Errorf("Use = %q, want %q", cmd.Use, "skill")
	}
}

func TestNew_HasRequiredSubcommands(t *testing.T) {
	t.Parallel()
	cmd := cmdskill.New()
	names := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		names[sub.Name()] = true
	}
	for _, want := range []string{"install", "list", "remove"} {
		if !names[want] {
			t.Errorf("missing subcommand %q", want)
		}
	}
}

// ── false-positive guard ──────────────────────────────────────────────────────

func TestInstall_DoesNotCreateFilesOutsideRoot(t *testing.T) {
	dir := t.TempDir()
	_, err := runCmd(t, "install", "--root", dir, "--name", "forge-expert")
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	// Only .github/ inside dir should be created; no files at cwd or above.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() != ".github" {
			t.Errorf("unexpected entry created in root: %s", e.Name())
		}
	}
}

// ── cobra integration wiring (invoked via parent) ─────────────────────────────

func TestSkillCommand_RegisteredWithParent(t *testing.T) {
	t.Parallel()
	parent := &cobra.Command{Use: "forge"}
	parent.AddCommand(cmdskill.New())
	found := false
	for _, c := range parent.Commands() {
		if c.Name() == "skill" {
			found = true
		}
	}
	if !found {
		t.Error("skill command not found in parent")
	}
}
