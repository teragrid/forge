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
package cmdpostmortem

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// validPostmortem returns a minimal post-mortem that passes all checks.
const validPostmortem = `# INC-001 — Test incident

- **Status:** draft
- **Severity:** S2
- **Incident date:** 2026-01-01

## 1. Summary
Brief description.

## 2. Impact
Low impact.

## 3. Timeline
- 00:00 detected

## 4. Root cause
Five-whys.

## 5. What went well
Rollback worked.

## 6. Action items

- [ ] AI-01 — Fix the root cause — owner: @alice — due: 2026-02-01 — issue: #42 — register: FR-001

## 7. Lessons / non-actions
Nothing to add.

## 8. Bypass log
No bypass used.
`

// TC-PM-01 (happy): valid INC-*.md passes lint.
func TestLintFile_Happy(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "INC-001-test.md")
	write(t, path, validPostmortem)
	r := lintFile(path)
	if !r.OK {
		t.Fatalf("expected OK, got issues: %v", r.Issues)
	}
}

// TC-PM-02 (negative): missing section detected.
func TestLintFile_MissingSection(t *testing.T) {
	content := strings.Replace(validPostmortem, "## 4. Root cause", "## 4. Cause", 1)
	dir := t.TempDir()
	path := filepath.Join(dir, "INC-002-missing.md")
	write(t, path, content)
	r := lintFile(path)
	if r.OK {
		t.Fatal("expected FAIL for missing section")
	}
	found := false
	for _, iss := range r.Issues {
		if strings.Contains(iss, "## 4. Root cause") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected issue mentioning '## 4. Root cause', got: %v", r.Issues)
	}
}

// TC-PM-03 (negative): no valid action item detected.
func TestLintFile_NoActionItem(t *testing.T) {
	content := strings.Replace(validPostmortem,
		"- [ ] AI-01 — Fix the root cause — owner: @alice — due: 2026-02-01 — issue: #42 — register: FR-001",
		"- TODO: fix something", 1)
	dir := t.TempDir()
	path := filepath.Join(dir, "INC-003-noai.md")
	write(t, path, content)
	r := lintFile(path)
	if r.OK {
		t.Fatal("expected FAIL for missing action item")
	}
	hasIssue := false
	for _, iss := range r.Issues {
		if strings.Contains(iss, "Action items") {
			hasIssue = true
		}
	}
	if !hasIssue {
		t.Errorf("expected action item issue, got: %v", r.Issues)
	}
}

// TC-PM-04 (negative): action item present but no register/commit ref.
func TestLintFile_NoRegisterRef(t *testing.T) {
	// Keep the action item but strip the register: FR-001 reference.
	content := strings.Replace(validPostmortem, " — register: FR-001", "", 1)
	dir := t.TempDir()
	path := filepath.Join(dir, "INC-004-noreg.md")
	write(t, path, content)
	r := lintFile(path)
	if r.OK {
		t.Fatal("expected FAIL for missing register reference")
	}
}

// TC-PM-05 (false-positive guard): commit: sha satisfies the register-ref check.
func TestLintFile_CommitRefSatisfies(t *testing.T) {
	content := strings.Replace(validPostmortem, " — register: FR-001", " — commit: abc1234", 1)
	dir := t.TempDir()
	path := filepath.Join(dir, "INC-005-commit.md")
	write(t, path, content)
	r := lintFile(path)
	if !r.OK {
		t.Fatalf("commit ref should satisfy register check; issues: %v", r.Issues)
	}
}

// TC-PM-06 (boundary): lintDir on absent directory returns empty slice.
func TestLintDir_Absent(t *testing.T) {
	reports, err := lintDir("/no/such/dir/postmortems")
	if err != nil {
		t.Fatalf("expected nil err for missing dir, got %v", err)
	}
	if len(reports) != 0 {
		t.Errorf("expected 0 reports, got %d", len(reports))
	}
}

// TC-PM-07 (boundary): lintDir ignores non-INC-*.md files.
func TestLintDir_IgnoresOtherFiles(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "README.md"), "not a postmortem")
	write(t, filepath.Join(dir, "notes.txt"), "notes")
	reports, err := lintDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != 0 {
		t.Errorf("expected 0 reports, got %d: %+v", len(reports), reports)
	}
}

// TC-PM-08 (data-accuracy): lintDir finds all INC-*.md files and sorts them.
func TestLintDir_FindsAll(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "INC-002-b.md"), validPostmortem)
	write(t, filepath.Join(dir, "INC-001-a.md"), validPostmortem)
	reports, err := lintDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != 2 {
		t.Fatalf("want 2, got %d", len(reports))
	}
	if !strings.HasSuffix(reports[0].File, "INC-001-a.md") {
		t.Errorf("expected sorted order: got %s first", reports[0].File)
	}
}

// TC-PM-09 (data-accuracy): --json flag emits valid JSON array.
func TestNew_JSONOutput(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "INC-001-json.md"), validPostmortem)

	cmd := New()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{dir, "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var reports []FileReport
	if err := json.Unmarshal(buf.Bytes(), &reports); err != nil {
		t.Fatalf("JSON parse: %v; output: %s", err, buf.String())
	}
	if len(reports) != 1 || !reports[0].OK {
		t.Errorf("unexpected report: %+v", reports)
	}
}

// TC-PM-10 (negative/CI-gate): non-zero exit for invalid postmortem.
func TestNew_FailureExitCode(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "INC-001-bad.md"), "# No sections at all")

	cmd := New()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{dir})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected non-nil error for invalid postmortem")
	}
}

// TC-PM-11 (idempotency): running lint twice on same dir returns same result.
func TestLintDir_Idempotent(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "INC-001-idem.md"), validPostmortem)
	r1, _ := lintDir(dir)
	r2, _ := lintDir(dir)
	if len(r1) != len(r2) {
		t.Errorf("results differ: %d vs %d", len(r1), len(r2))
	}
	if r1[0].OK != r2[0].OK {
		t.Errorf("result mismatch")
	}
}

// --- helpers ---

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
