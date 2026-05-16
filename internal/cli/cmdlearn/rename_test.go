// Copyright 2024 The Forge Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Gap tests G-090 and G-091: verify that the renamed subcommands
// `forge learn teach` and `forge learn session` are accessible and functional,
// replacing the old top-level aliases `forge teach` and `forge session`.
package cmdlearn

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLearnTeach_ReplacesForgeTeach (G-090) — verifies that `forge learn teach`
// is accessible as a subcommand of the learn command and writes a preference
// entry, confirming it replaces the legacy `forge teach` alias.
func TestLearnTeach_ReplacesForgeTeach(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	cmd := New()

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	// Ensure that `teach` is registered as a subcommand.
	var teachFound bool
	for _, sub := range cmd.Commands() {
		if sub.Name() == "teach" {
			teachFound = true
			break
		}
	}
	if !teachFound {
		t.Fatal("forge learn teach subcommand not registered; expected it to replace forge teach alias")
	}

	// Execute `forge learn teach --text "prefer short names"` and verify the
	// preference is written to preferences.yml.
	cmd.SetArgs([]string{"teach", "--text", "prefer short names", "--root", root})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("forge learn teach: %v", err)
	}

	prefPath := filepath.Join(root, ".forge", "learned", "preferences.yml")
	data, err := os.ReadFile(prefPath)
	if err != nil {
		t.Fatalf("preferences.yml not created: %v", err)
	}
	if !strings.Contains(string(data), "prefer short names") {
		t.Errorf("preferences.yml does not contain expected text: %s", string(data))
	}
}

// TestLearnSession_ReplacesForgeSession (G-091) — verifies that `forge learn session`
// is accessible as a subcommand of the learn command and outputs a session digest,
// confirming it replaces the legacy `forge session` alias.
func TestLearnSession_ReplacesForgeSession(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	// Create a .forge/session/ directory with a sample .log file.
	sessionDir := filepath.Join(root, ".forge", "session")
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatal(err)
	}
	logContent := "line1\nline2\nline3\n"
	if err := os.WriteFile(filepath.Join(sessionDir, "test.log"), []byte(logContent), 0o600); err != nil {
		t.Fatal(err)
	}

	r := root
	jsonOut := false
	_ = r
	_ = jsonOut
	cmd := New()

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	// Ensure that `session` is registered as a subcommand.
	var sessionFound bool
	for _, sub := range cmd.Commands() {
		if sub.Name() == "session" {
			sessionFound = true
			break
		}
	}
	if !sessionFound {
		t.Fatal("forge learn session subcommand not registered; expected it to replace forge session alias")
	}

	cmd.SetArgs([]string{"session", "--root", root})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("forge learn session: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "test.log") {
		t.Errorf("learn session output does not mention test.log: %q", output)
	}
}
