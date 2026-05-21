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

package cmdtsd_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/teragrid/forge/internal/cli/cmdtsd"
)

// ── helpers ──────────────────────────────────────────────────────────────────

func runCmd(t *testing.T, root *cobra.Command, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	var outBuf, errBuf bytes.Buffer
	root.SetOut(&outBuf)
	root.SetErr(&errBuf)
	root.SetArgs(args)
	err = root.Execute()
	return outBuf.String(), errBuf.String(), err
}

// newRootForTest wraps cmdtsd.New() in a minimal root command to avoid global
// state pollution.
func newRootForTest() *cobra.Command {
	root := &cobra.Command{Use: "forge", SilenceUsage: true, SilenceErrors: true}
	root.AddCommand(cmdtsd.New())
	return root
}

// chdir temporarily changes the working directory to dir, restoring it on
// cleanup.
func chdir(t *testing.T, dir string) {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
}

// ── TEST-TVAL-01: init --defaults creates .forge/tsd.yml ─────────────────────

func TestTSDInit_Defaults(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	root := newRootForTest()
	stdout, _, err := runCmd(t, root, "tsd", "init", "--defaults")
	if err != nil {
		t.Fatalf("tsd init --defaults: %v", err)
	}
	if !strings.Contains(stdout, ".forge/tsd.yml") {
		t.Errorf("expected filename in output, got: %s", stdout)
	}
	if _, err := os.Stat(filepath.Join(dir, ".forge", "tsd.yml")); err != nil {
		t.Errorf(".forge/tsd.yml not created: %v", err)
	}
}

// ── TEST-TVAL-02: init without --force fails when file exists ─────────────────

func TestTSDInit_ExistingFileWithoutForce(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	// Pre-create the TSD file.
	if err := os.MkdirAll(filepath.Join(dir, ".forge"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".forge", "tsd.yml"), []byte("tsd_version: 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	root := newRootForTest()
	_, _, err := runCmd(t, root, "tsd", "init")
	if err == nil {
		t.Fatal("expected error when file exists and no --force")
	}
}

// ── TEST-TVAL-03: init --force overwrites existing file ──────────────────────

func TestTSDInit_ForceOverwrites(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	if err := os.MkdirAll(filepath.Join(dir, ".forge"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".forge", "tsd.yml"), []byte("old: content\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	root := newRootForTest()
	_, _, err := runCmd(t, root, "tsd", "init", "--force")
	if err != nil {
		t.Fatalf("tsd init --force: %v", err)
	}

	content, readErr := os.ReadFile(filepath.Join(dir, ".forge", "tsd.yml"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !strings.Contains(string(content), "tsd_version: 1") {
		t.Errorf("overwritten file does not contain tsd_version: 1:\n%s", string(content))
	}
}

// ── TEST-TVAL-04: validate valid file → exit 0 ───────────────────────────────

func TestTSDValidate_ValidFile(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	validContent := `tsd_version: 1
project:
  name: "acme-test"
  type: saas
stack:
  frontend:
    framework: nextjs-15
`
	if err := os.MkdirAll(filepath.Join(dir, ".forge"), 0o755); err != nil {
		t.Fatal(err)
	}
	tsdPath := filepath.Join(dir, ".forge", "tsd.yml")
	if err := os.WriteFile(tsdPath, []byte(validContent), 0o600); err != nil {
		t.Fatal(err)
	}

	root := newRootForTest()
	stdout, _, err := runCmd(t, root, "tsd", "validate", tsdPath)
	if err != nil {
		t.Fatalf("validate of valid file failed: %v\nstdout: %s", err, stdout)
	}
	if !strings.Contains(stdout, "OK") {
		t.Errorf("expected OK in stdout, got: %s", stdout)
	}
}

// ── TEST-TVAL-05: validate invalid type → exit non-0 ─────────────────────────

func TestTSDValidate_InvalidType(t *testing.T) {
	dir := t.TempDir()

	invalidContent := `tsd_version: 1
project:
  name: "test"
  type: blog
`
	tsdPath := filepath.Join(dir, "bad.tsd.yml")
	if err := os.WriteFile(tsdPath, []byte(invalidContent), 0o600); err != nil {
		t.Fatal(err)
	}

	root := newRootForTest()
	_, stderr, err := runCmd(t, root, "tsd", "validate", tsdPath)
	if err == nil {
		t.Fatal("expected error for invalid TSD")
	}
	if !strings.Contains(stderr, "error") {
		t.Errorf("expected error message in stderr, got: %s", stderr)
	}
}

// ── TEST-TVAL-06: validate --json valid → {"valid":true} ─────────────────────

func TestTSDValidate_JSON_Valid(t *testing.T) {
	dir := t.TempDir()

	validContent := `tsd_version: 1
project:
  name: "json-test"
`
	tsdPath := filepath.Join(dir, "tsd.yml")
	if err := os.WriteFile(tsdPath, []byte(validContent), 0o600); err != nil {
		t.Fatal(err)
	}

	root := newRootForTest()
	stdout, _, err := runCmd(t, root, "tsd", "validate", "--json", tsdPath)
	if err != nil {
		t.Fatalf("validate --json of valid file: %v", err)
	}

	var out struct {
		Valid    bool     `json:"valid"`
		Errors   []string `json:"errors"`
		Warnings []string `json:"warnings"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("JSON unmarshal failed: %v\nstdout: %s", err, stdout)
	}
	if !out.Valid {
		t.Errorf("expected valid=true, got %+v", out)
	}
	if len(out.Errors) != 0 {
		t.Errorf("expected empty errors, got %v", out.Errors)
	}
}

// ── TEST-TVAL-07: validate --json invalid → {"valid":false,"errors":[...]} ───

func TestTSDValidate_JSON_Invalid(t *testing.T) {
	dir := t.TempDir()

	invalidContent := `tsd_version: 1
project:
  name: ""
  type: blog
`
	tsdPath := filepath.Join(dir, "invalid.tsd.yml")
	if err := os.WriteFile(tsdPath, []byte(invalidContent), 0o600); err != nil {
		t.Fatal(err)
	}

	root := newRootForTest()
	stdout, _, _ := runCmd(t, root, "tsd", "validate", "--json", tsdPath)

	var out struct {
		Valid    bool     `json:"valid"`
		Errors   []string `json:"errors"`
		Warnings []string `json:"warnings"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("JSON unmarshal failed: %v\nstdout: %s", err, stdout)
	}
	if out.Valid {
		t.Errorf("expected valid=false for invalid TSD")
	}
	if len(out.Errors) == 0 {
		t.Errorf("expected non-empty errors in JSON output")
	}
}

// ── TEST-TVAL-08: validate missing file → exit 1 ─────────────────────────────

func TestTSDValidate_MissingFile(t *testing.T) {
	dir := t.TempDir()
	root := newRootForTest()
	_, _, err := runCmd(t, root, "tsd", "validate", filepath.Join(dir, "missing.tsd.yml"))
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

// ── TEST-TVAL-09: validate with unknown key → warns but valid ────────────────

func TestTSDValidate_UnknownKeyWarns(t *testing.T) {
	dir := t.TempDir()

	content := `tsd_version: 1
project:
  name: "warn-test"
future_feature: enabled
`
	tsdPath := filepath.Join(dir, "warn.tsd.yml")
	if err := os.WriteFile(tsdPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	root := newRootForTest()
	stdout, _, err := runCmd(t, root, "tsd", "validate", tsdPath)
	// Should still exit 0 (warnings don't fail).
	if err != nil {
		t.Fatalf("unknown key should not cause error, got: %v\nstdout: %s", err, stdout)
	}
	if !strings.Contains(stdout, "warning") {
		t.Errorf("expected warning in stdout, got: %s", stdout)
	}
}

// ── Regression: init creates idempotent content ───────────────────────────────

func TestTSDInit_IdempotentContent(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	for _, dir := range []string{dirA, dirB} {
		chdir(t, dir)
		root := newRootForTest()
		if _, _, err := runCmd(t, root, "tsd", "init", "--defaults"); err != nil {
			t.Fatalf("init in %s: %v", dir, err)
		}
	}

	contentA, err := os.ReadFile(filepath.Join(dirA, ".forge", "tsd.yml"))
	if err != nil {
		t.Fatal(err)
	}
	contentB, err := os.ReadFile(filepath.Join(dirB, ".forge", "tsd.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(contentA) != string(contentB) {
		t.Errorf("init not idempotent: content differs")
	}
}

// ── TG-01: forge tsd init --defaults produces skeleton with messaging: section ─

func TestTSDInit_Defaults_HasMessagingSection(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	chdir(t, dir)
	root := newRootForTest()
	if _, _, err := runCmd(t, root, "tsd", "init", "--defaults"); err != nil {
		t.Fatalf("init: %v", err)
	}
	content, err := os.ReadFile(filepath.Join(dir, ".forge", "tsd.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "messaging:") {
		t.Error("skeleton TSD should contain 'messaging:' section")
	}
	for _, key := range []string{"queue:", "realtime:", "email:", "sms:"} {
		if !strings.Contains(string(content), key) {
			t.Errorf("skeleton TSD should contain messaging sub-key %q", key)
		}
	}
}
