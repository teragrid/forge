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

package cmdci_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/teragrid/forge/internal/cli/cmdci"
	"github.com/teragrid/forge/internal/errcode"
)

// TC-31-01: forge ci command exists and has the three expected sub-commands.
func TestCI_CommandStructure(t *testing.T) {
	root := cmdci.New()
	if root.Use != "ci" {
		t.Errorf("root.Use = %q; want %q", root.Use, "ci")
	}

	sub := map[string]bool{}
	for _, c := range root.Commands() {
		sub[c.Name()] = true
	}
	for _, want := range []string{"watch", "fix", "gotcha"} {
		if !sub[want] {
			t.Errorf("expected sub-command %q to be registered", want)
		}
	}
}

// TC-31-02: forge ci gotcha writes a valid JSONL record.
func TestCI_GotchaWritesRecord(t *testing.T) {
	dir := t.TempDir()
	// Chdir so .forge/learned/gotchas.jsonl resolves under t.TempDir().
	orig, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	root := cmdci.New()
	var buf bytes.Buffer
	root.SetOut(&buf)

	root.SetArgs([]string{
		"gotcha",
		"--run-id", "9876543",
		"--branch", "feat/ci-monitor",
		"--sha", "abc123def456",
		"--url", "https://github.com/example/repo/actions/runs/9876543",
		"--conclusion", "failure",
		"--note", "missing go mod tidy",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	gotchaPath := filepath.Join(dir, ".forge", "learned", "gotchas.jsonl")
	raw, err := os.ReadFile(gotchaPath)
	if err != nil {
		t.Fatalf("read gotchas.jsonl: %v", err)
	}

	line := strings.TrimSpace(string(raw))
	var rec cmdci.GotchaRecord
	if err := json.Unmarshal([]byte(line), &rec); err != nil {
		t.Fatalf("unmarshal gotcha record: %v\nraw: %s", err, line)
	}

	checks := []struct {
		field, got, want string
	}{
		{"RunID", rec.RunID, "9876543"},
		{"Branch", rec.Branch, "feat/ci-monitor"},
		{"SHA", rec.SHA, "abc123def456"},
		{"Conclusion", rec.Conclusion, "failure"},
		{"Note", rec.Note, "missing go mod tidy"},
		{"URL", rec.URL, "https://github.com/example/repo/actions/runs/9876543"},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("record.%s = %q; want %q", c.field, c.got, c.want)
		}
	}
	if rec.TS == "" {
		t.Error("record.TS is empty; expected an RFC3339 timestamp")
	}
}

// TC-31-03: forge ci gotcha appends a second record without truncating.
func TestCI_GotchaAppendsRecords(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	for i := 0; i < 2; i++ {
		root := cmdci.New()
		root.SetArgs([]string{
			"gotcha",
			"--run-id", "111",
			"--conclusion", "failure",
		})
		if err := root.Execute(); err != nil {
			t.Fatalf("Execute iteration %d: %v", i, err)
		}
	}

	raw, _ := os.ReadFile(filepath.Join(dir, ".forge", "learned", "gotchas.jsonl"))
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 JSONL lines; got %d\n%s", len(lines), raw)
	}
}

// TC-31-04: forge ci gotcha --json emits valid JSON.
func TestCI_GotchaJSONOutput(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	root := cmdci.New()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{
		"gotcha",
		"--run-id", "42",
		"--conclusion", "failure",
		"--json",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var rec cmdci.GotchaRecord
	if err := json.Unmarshal(buf.Bytes(), &rec); err != nil {
		t.Fatalf("unmarshal --json output: %v\noutput: %s", err, buf.String())
	}
	if rec.RunID != "42" {
		t.Errorf("rec.RunID = %q; want %q", rec.RunID, "42")
	}
}

// TC-31-05: error codes are registered in the 6200-6299 range.
func TestCI_ErrorCodesInRange(t *testing.T) {
	codes := []errcode.Code{
		cmdci.CodeCIOperationFailed,
		cmdci.CodeCIWatchTimeout,
		cmdci.CodeCIRunFailed,
		cmdci.CodeCINoRunFound,
		cmdci.CodeCIGotchaWrite,
	}
	for _, c := range codes {
		if !errcode.IsReserved(c) {
			t.Errorf("errcode %d is not reserved; expected it in range 6200-6299", c)
		}
	}
}

// TC-31-06: forge ci watch returns error when GITHUB_TOKEN is absent and gh is not on PATH.
// (This is a unit-level smoke test for the no-credential path.)
func TestCI_WatchNoCredentials(t *testing.T) {
	// Temporarily clear GITHUB_TOKEN.
	prev := os.Getenv("GITHUB_TOKEN")
	os.Unsetenv("GITHUB_TOKEN")
	t.Cleanup(func() {
		if prev != "" {
			os.Setenv("GITHUB_TOKEN", prev)
		}
	})

	// Override PATH so that `gh` is not found.
	origPath := os.Getenv("PATH")
	os.Setenv("PATH", "")
	t.Cleanup(func() { os.Setenv("PATH", origPath) })

	root := cmdci.New()
	var errBuf bytes.Buffer
	root.SetErr(&errBuf)
	root.SetArgs([]string{"watch", "--sha", "deadbeef", "--repo", "example/repo"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error when no GitHub credentials are available; got nil")
	}
	var ferr *errcode.Error
	if !errors.As(err, &ferr) || ferr.Code != cmdci.CodeCIOperationFailed {
		t.Errorf("expected CodeCIOperationFailed (6200); got %v", err)
	}
}
