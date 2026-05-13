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
