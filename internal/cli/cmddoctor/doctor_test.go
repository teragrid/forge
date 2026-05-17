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
package cmddoctor

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRun_HasChecks(t *testing.T) {
	t.Parallel()
	r := Run("")
	if r.OS == "" || r.Arch == "" {
		t.Fatal("OS/Arch must be populated")
	}
	if len(r.Checks) < 4 {
		t.Fatalf("expected >=4 checks, got %d", len(r.Checks))
	}
}

func TestCmd_Text(t *testing.T) {
	t.Parallel()
	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(nil)
	// We don't assert healthy/unhealthy because CI runners differ; we only
	// assert the verb produces structured output and does not crash.
	_ = cmd.Execute()
	if !strings.Contains(out.String(), "forge doctor") {
		t.Fatalf("missing header: %s", out.String())
	}
}

func TestCmd_JSON(t *testing.T) {
	t.Parallel()
	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--json"})
	_ = cmd.Execute()

	var rep Report
	// Skip the trailing error message line if doctor is unhealthy on this host.
	body := bytes.TrimSpace(out.Bytes())
	// JSON must occupy the first object; trim anything past the closing brace.
	if i := bytes.LastIndexByte(body, '}'); i >= 0 {
		body = body[:i+1]
	}
	if err := json.Unmarshal(body, &rep); err != nil {
		t.Fatalf("not JSON: %v\noutput: %s", err, out.String())
	}
	if len(rep.Checks) == 0 {
		t.Fatal("expected checks in JSON output")
	}
}

// ── DEV-M1-38: .gitignore drift detection tests ───────────────────────────

// TC-38-01: canonical managed block → StatusOK.
func TestCheckGitignoreDrift_Canonical(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(canonicalGiSnippet), 0o600); err != nil {
		t.Fatal(err)
	}
	c := checkGitignoreDrift(dir)
	if c.Status != StatusOK {
		t.Fatalf("expected StatusOK for canonical block, got %s: %s", c.Status, c.Detail)
	}
}

// TC-38-02: hand-edited managed block → StatusWarn + hint.
func TestCheckGitignoreDrift_HandEdited(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	drifted := giDriftStart + "\n# HAND EDITED\n" + giDriftEnd + "\n"
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(drifted), 0o600); err != nil {
		t.Fatal(err)
	}
	c := checkGitignoreDrift(dir)
	if c.Status != StatusWarn {
		t.Fatalf("expected StatusWarn for drifted block, got %s", c.Status)
	}
	if !strings.Contains(c.Hint, "forge upgrade gitignore") {
		t.Errorf("hint should mention 'forge upgrade gitignore', got %q", c.Hint)
	}
}

// TC-38-03: no .gitignore → StatusOK (silent, first-time user).
func TestCheckGitignoreDrift_NoFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	c := checkGitignoreDrift(dir)
	if c.Status != StatusOK {
		t.Fatalf("expected StatusOK when no .gitignore, got %s", c.Status)
	}
}

// TC-38-04: user section edited outside markers → StatusOK (false-positive guard).
func TestCheckGitignoreDrift_UserSectionOutsideMarkers(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// Canonical block with additional user rule appended after it.
	content := canonicalGiSnippet + "# my custom rule\n"
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	c := checkGitignoreDrift(dir)
	if c.Status != StatusOK {
		t.Fatalf("user content outside markers should NOT trigger drift, got %s: %s", c.Status, c.Detail)
	}
}

// TC-38-05: Run("") returns a report containing the gitignore drift check.
func TestRun_IncludesGitignoreDriftCheck(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	rep := Run(dir)
	found := false
	for _, c := range rep.Checks {
		if c.Name == ".gitignore managed block" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected '.gitignore managed block' check in Report")
	}
}

// ── G-114: schema-drift checks ────────────────────────────────────────────────

// TestCmd_DriftFlag verifies that passing --drift does not panic and runs
// without returning a stack trace in the output. G-114.
func TestCmd_DriftFlag(t *testing.T) {
	t.Parallel()
	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--drift"})
	_ = cmd.Execute()
	if strings.Contains(out.String(), "panic") {
		t.Errorf("--drift output must not contain 'panic': %s", out.String())
	}
}

// TestCheckSchemaDrift_EmptyDir verifies that checkSchemaDrift on an empty
// directory returns a non-nil slice of checks (all warn: missing generated
// files) and that every Check has a non-empty Name. G-114.
func TestCheckSchemaDrift_EmptyDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	checks := checkSchemaDrift(dir)
	if checks == nil {
		t.Fatal("checkSchemaDrift must return a non-nil slice")
	}
	for _, c := range checks {
		if c.Name == "" {
			t.Error("each Check returned by checkSchemaDrift must have a non-empty Name")
		}
	}
}
