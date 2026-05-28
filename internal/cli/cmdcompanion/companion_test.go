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

// companion_test.go — tests for `forge companion`.

package cmdcompanion

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// execCmd runs cmd with args and captures stdout+stderr.
func execCmd(t *testing.T, cmd *cobra.Command, args ...string) (string, error) {
	t.Helper()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return buf.String(), err
}

// TestNew_ReturnsCommand verifies the command is constructed correctly.
func TestNew_ReturnsCommand(t *testing.T) {
	t.Parallel()
	cmd := New()
	if cmd == nil {
		t.Fatal("New() returned nil")
	}
	if cmd.Use != "companion" {
		t.Errorf("Use = %q, want %q", cmd.Use, "companion")
	}
}

// TestNew_HasExpectedSubcommands verifies install/update/status/guide exist.
func TestNew_HasExpectedSubcommands(t *testing.T) {
	t.Parallel()
	cmd := New()
	want := map[string]bool{"install": false, "update": false, "status": false, "guide": false}
	for _, sub := range cmd.Commands() {
		want[sub.Name()] = true
	}
	for name, found := range want {
		if !found {
			t.Errorf("subcommand %q not registered", name)
		}
	}
}

// TestGuide_PrintsVibeCodeContent verifies guide contains key vibe-coding headings.
func TestGuide_PrintsVibeCodeContent(t *testing.T) {
	t.Parallel()
	cmd := New()
	out, err := execCmd(t, cmd, "guide")
	if err != nil {
		t.Fatalf("guide returned error: %v", err)
	}
	checks := []string{
		"Vibe-Coding",
		"Feature Workflow",
		"Bugfix Workflow",
		"Morning Standup",
		"forge ship",
		"forge bugfix",
	}
	for _, want := range checks {
		if !strings.Contains(out, want) {
			t.Errorf("guide output missing %q", want)
		}
	}
}

// TestStatus_ShowsPlatforms verifies status output mentions all 4 platforms.
func TestStatus_ShowsPlatforms(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	cmd := New()
	out, err := execCmd(t, cmd, "status", "--root", root)
	if err != nil {
		t.Fatalf("status returned error: %v", err)
	}
	platforms := []string{"VS Code Copilot", "Claude", "Cursor", "Windsurf"}
	for _, p := range platforms {
		if !strings.Contains(out, p) {
			t.Errorf("status output missing platform %q", p)
		}
	}
}

// TestStatus_MarksMissingPlatforms verifies '–' marker for uninstalled platforms.
func TestStatus_MarksMissingPlatforms(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	cmd := New()
	out, err := execCmd(t, cmd, "status", "--root", root)
	if err != nil {
		t.Fatalf("status returned error: %v", err)
	}
	if !strings.Contains(out, "not installed") {
		t.Error("expected 'not installed' for unconfigured platforms")
	}
}

// TestStatus_MarksInstalledPlatform verifies '✓' marker when file exists.
func TestStatus_MarksInstalledPlatform(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// Simulate the Copilot chatmode file existing.
	chatmodeDir := filepath.Join(root, ".github", "chatmodes")
	if err := os.MkdirAll(chatmodeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(chatmodeDir, "forge-expert.chatmode.md"), []byte("# forge"), 0o600); err != nil {
		t.Fatal(err)
	}

	cmd := New()
	out, err := execCmd(t, cmd, "status", "--root", root)
	if err != nil {
		t.Fatalf("status returned error: %v", err)
	}
	if !strings.Contains(out, "✓") {
		t.Error("expected ✓ check for installed platform")
	}
}

// TestInstall_WritesFiles verifies install subcommand actually writes files.
func TestInstall_WritesFiles(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	cmd := New()
	_, err := execCmd(t, cmd, "install", "--root", root, "--for", "copilot")
	if err != nil {
		t.Fatalf("install returned error: %v", err)
	}
	// Copilot chatmode file must exist after install.
	chatmode := filepath.Join(root, ".github", "chatmodes", "forge-expert.chatmode.md")
	if _, err := os.Stat(chatmode); os.IsNotExist(err) {
		t.Errorf("chatmode file not created: %s", chatmode)
	}
}

// TestInstall_SkipsExistingWithoutForce verifies idempotency without --force.
func TestInstall_SkipsExistingWithoutForce(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// First install.
	cmd1 := New()
	if _, err := execCmd(t, cmd1, "install", "--root", root, "--for", "copilot"); err != nil {
		t.Fatalf("first install: %v", err)
	}
	// Second install without --force → should skip.
	cmd2 := New()
	out, err := execCmd(t, cmd2, "install", "--root", root, "--for", "copilot")
	if err != nil {
		t.Fatalf("second install: %v", err)
	}
	if !strings.Contains(out, "up to date") {
		t.Errorf("expected 'up to date' message on second install, got: %s", out)
	}
}

// TestInstall_ForceOverwrites verifies --force rewrites existing files.
func TestInstall_ForceOverwrites(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// Write a sentinel file.
	chatmodeDir := filepath.Join(root, ".github", "chatmodes")
	if err := os.MkdirAll(chatmodeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(chatmodeDir, "forge-expert.chatmode.md")
	if err := os.WriteFile(path, []byte("old-content"), 0o600); err != nil {
		t.Fatal(err)
	}

	cmd := New()
	if _, err := execCmd(t, cmd, "install", "--root", root, "--for", "copilot", "--force"); err != nil {
		t.Fatalf("install --force: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) == "old-content" {
		t.Error("--force should have overwritten the existing file")
	}
}

// TestUpdate_RegeneratesFiles verifies update overwrites without --force flag.
func TestUpdate_RegeneratesFiles(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	chatmodeDir := filepath.Join(root, ".github", "chatmodes")
	if err := os.MkdirAll(chatmodeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(chatmodeDir, "forge-expert.chatmode.md")
	if err := os.WriteFile(path, []byte("stale"), 0o600); err != nil {
		t.Fatal(err)
	}

	cmd := New()
	if _, err := execCmd(t, cmd, "update", "--root", root, "--for", "copilot"); err != nil {
		t.Fatalf("update: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) == "stale" {
		t.Error("update should have regenerated the stale skill file")
	}
}

// TestVibeCodeGuide_ContainsTopTenCommands verifies the 10 daily commands are listed.
func TestVibeCodeGuide_ContainsTopTenCommands(t *testing.T) {
	t.Parallel()
	guide := vibeCodeGuide()
	commands := []string{"forge ship", "forge bugfix", "forge scan", "forge review", "forge test"}
	for _, c := range commands {
		if !strings.Contains(guide, c) {
			t.Errorf("guide missing command %q", c)
		}
	}
}

// TestVibeCodeGuide_FalsePositiveGuard verifies the guide does NOT contain
// internal/implementation details that would confuse end users.
func TestVibeCodeGuide_FalsePositiveGuard(t *testing.T) {
	t.Parallel()
	guide := vibeCodeGuide()
	// The guide is user-facing, not developer docs — should not expose internal paths.
	forbidden := []string{"internal/cli", "errcode.Register", "verbmeta.Register"}
	for _, f := range forbidden {
		if strings.Contains(guide, f) {
			t.Errorf("guide should not contain internal detail %q", f)
		}
	}
}
