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

package cmdconfig_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/teragrid/forge/internal/cli/cmdconfig"
	"github.com/teragrid/forge/internal/config"
)

func run(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := cmdconfig.New()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs(args)
	err := cmd.Execute()
	return buf.String(), err
}

func writeYAML(t *testing.T, root, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, "forge.yml"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// TC-02-01 (happy): show prints all expected keys.
func TestConfigShow_AllKeys(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	out, err := run(t, "--root", root, "show")
	if err != nil {
		t.Fatalf("config show: %v", err)
	}
	for _, key := range []string{"llm.provider", "llm.model", "log.level", "telemetry.enabled"} {
		if !strings.Contains(out, key) {
			t.Errorf("expected %q in output:\n%s", key, out)
		}
	}
}

// TC-02-02 (happy): get returns the value for a known key.
func TestConfigGet_KnownKey(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeYAML(t, root, "llm:\n  provider: anthropic\n")
	out, err := run(t, "--root", root, "get", "llm.provider")
	if err != nil {
		t.Fatalf("config get: %v", err)
	}
	if strings.TrimSpace(out) != "anthropic" {
		t.Errorf("expected anthropic, got %q", out)
	}
}

// TC-02-03 (negative): get with unknown key errors.
func TestConfigGet_UnknownKey_Errors(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	_, err := run(t, "--root", root, "get", "bogus.key")
	if err == nil {
		t.Fatal("expected error for unknown key")
	}
}

// TC-02-04 (data-accuracy): explain shows the winning source.
func TestConfigExplain_Source(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeYAML(t, root, "log:\n  level: warn\n")
	out, err := run(t, "--root", root, "explain", "log.level")
	if err != nil {
		t.Fatalf("config explain: %v", err)
	}
	if !strings.Contains(out, "warn") {
		t.Errorf("expected warn in output: %s", out)
	}
	if !strings.Contains(out, "file") {
		t.Errorf("expected 'file' source in output: %s", out)
	}
}

// TC-02-05 (boundary): missing forge.yml — no crash, defaults apply.
func TestConfigShow_NoFile(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	out, err := run(t, "--root", root, "show")
	if err != nil {
		t.Fatalf("config show without file: %v", err)
	}
	if !strings.Contains(out, "auto") {
		t.Errorf("expected default 'auto' provider in output: %s", out)
	}
}

// TC-02-06 (json): --json emits valid JSON.
func TestConfigShow_JSON(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	out, err := run(t, "--root", root, "show", "--json")
	if err != nil {
		t.Fatalf("config show --json: %v", err)
	}
	if !strings.Contains(out, `"llm.provider"`) {
		t.Errorf("expected JSON with llm.provider key: %s", out)
	}
}

// TC-02-07 (explain all): explain without a key shows all fields.
func TestConfigExplainAll(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	out, err := run(t, "--root", root, "explain")
	if err != nil {
		t.Fatalf("config explain: %v", err)
	}
	if !strings.Contains(out, "llm.provider") {
		t.Errorf("expected all fields in explain output: %s", out)
	}
	if !strings.Contains(out, "default") {
		t.Errorf("expected 'default' source in explain output: %s", out)
	}
}

// ── config set ────────────────────────────────────────────────────────────────

// TC-02-08 (happy): set creates forge.yml and persists the value.
func TestConfigSet_Happy(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	out, err := run(t, "--root", root, "set", "llm.model", "gpt-4o")
	if err != nil {
		t.Fatalf("config set: %v", err)
	}
	if !strings.Contains(out, "llm.model") {
		t.Errorf("expected key in output: %s", out)
	}
	if !strings.Contains(out, "gpt-4o") {
		t.Errorf("expected value in output: %s", out)
	}
	// Verify value is readable back via 'get'.
	out2, err := run(t, "--root", root, "get", "llm.model")
	if err != nil {
		t.Fatalf("config get after set: %v", err)
	}
	if strings.TrimSpace(out2) != "gpt-4o" {
		t.Errorf("expected gpt-4o, got %q", out2)
	}
}

// TC-02-09 (happy/overwrite): set updates an existing value without clobbering others.
func TestConfigSet_Overwrite(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// Seed with two keys.
	writeYAML(t, root, "llm:\n  provider: anthropic\n  model: claude-3-5-sonnet\n")

	_, err := run(t, "--root", root, "set", "llm.model", "claude-sonnet-4-5")
	if err != nil {
		t.Fatalf("config set overwrite: %v", err)
	}
	// Provider should be preserved.
	out, err := run(t, "--root", root, "get", "llm.provider")
	if err != nil {
		t.Fatalf("config get provider: %v", err)
	}
	if strings.TrimSpace(out) != "anthropic" {
		t.Errorf("provider should be preserved as anthropic, got %q", out)
	}
	// Model should be updated.
	out2, err := run(t, "--root", root, "get", "llm.model")
	if err != nil {
		t.Fatalf("config get model: %v", err)
	}
	if strings.TrimSpace(out2) != "claude-sonnet-4-5" {
		t.Errorf("model should be claude-sonnet-4-5, got %q", out2)
	}
}

// TC-02-10 (negative): set with an unknown key returns an error.
func TestConfigSet_UnknownKey_Error(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	_, err := run(t, "--root", root, "set", "bogus.key", "value")
	if err == nil {
		t.Fatal("expected error for unknown key, got nil")
	}
}

// TC-02-11 (idempotency): setting the same key twice yields the final value.
func TestConfigSet_Idempotent(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	_, _ = run(t, "--root", root, "set", "log.level", "debug")
	_, _ = run(t, "--root", root, "set", "log.level", "info")

	out, err := run(t, "--root", root, "get", "log.level")
	if err != nil {
		t.Fatalf("config get: %v", err)
	}
	if strings.TrimSpace(out) != "info" {
		t.Errorf("expected final value info, got %q", out)
	}
}

// TC-02-12 (data-accuracy): WriteKey round-trip: set via WriteKey, load via config.Load.
func TestWriteKey_RoundTrip(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	if err := config.WriteKey(root, "llm.model", "claude-sonnet-4-5"); err != nil {
		t.Fatalf("WriteKey: %v", err)
	}
	cfg, err := config.Load(root, nil)
	if err != nil {
		t.Fatalf("Load after WriteKey: %v", err)
	}
	if cfg.LLMModel.Raw != "claude-sonnet-4-5" {
		t.Errorf("LLMModel.Raw: got %q want %q", cfg.LLMModel.Raw, "claude-sonnet-4-5")
	}
}

// TC-02-13 (false-positive guard): WriteKey does NOT modify unrelated keys.
func TestWriteKey_DoesNotCorruptOtherKeys(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeYAML(t, root, "llm:\n  provider: openai\nlog:\n  level: warn\n")

	if err := config.WriteKey(root, "llm.model", "gpt-4o"); err != nil {
		t.Fatalf("WriteKey: %v", err)
	}
	cfg, err := config.Load(root, nil)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.LLMProvider.Raw != "openai" {
		t.Errorf("llm.provider should be openai, got %q", cfg.LLMProvider.Raw)
	}
	if cfg.LogLevel.Raw != "warn" {
		t.Errorf("log.level should be warn, got %q", cfg.LogLevel.Raw)
	}
}
