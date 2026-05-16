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
package cmdincident

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/teragrid/forge/internal/incident"
)

func exec(t *testing.T, args []string) string {
	t.Helper()
	var buf bytes.Buffer
	c := New()
	c.SetOut(&buf)
	c.SetErr(&buf)
	c.SetArgs(args)
	if err := c.Execute(); err != nil {
		t.Fatalf("exec %v: %v\n%s", args, err, buf.String())
	}
	return buf.String()
}

func execErr(t *testing.T, args []string) error {
	t.Helper()
	var buf bytes.Buffer
	c := New()
	c.SetOut(&buf)
	c.SetErr(&buf)
	c.SetArgs(args)
	return c.Execute()
}

// TC-CI-01: new creates incident and shows it.
func TestIncident_New(t *testing.T) {
	dir := t.TempDir()
	out := exec(t, []string{"new", "--root", dir, "--id", "INC-001",
		"--title", "DB down", "--severity", "S1", "--systems", "DB,API"})
	if !strings.Contains(out, "INC-001") {
		t.Fatalf("want INC-001 in output, got: %s", out)
	}
}

// TC-CI-02: new --json output is valid JSON with required keys.
func TestIncident_New_JSON(t *testing.T) {
	dir := t.TempDir()
	out := exec(t, []string{"new", "--root", dir, "--id", "INC-002",
		"--title", "test", "--json"})
	var p map[string]any
	if err := json.Unmarshal([]byte(out), &p); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	for _, k := range []string{"id", "title", "state", "severity"} {
		if _, ok := p[k]; !ok {
			t.Errorf("missing key %q in JSON", k)
		}
	}
}

// TC-CI-03: update transitions state.
func TestIncident_Update_State(t *testing.T) {
	dir := t.TempDir()
	exec(t, []string{"new", "--root", dir, "--id", "INC-003", "--title", "t"})
	out := exec(t, []string{"update", "INC-003", "--root", dir, "--state", "investigating"})
	if !strings.Contains(out, "investigating") {
		t.Fatalf("want 'investigating' in output, got: %s", out)
	}
}

// TC-CI-04: update --note appends note.
func TestIncident_Update_Note(t *testing.T) {
	dir := t.TempDir()
	exec(t, []string{"new", "--root", dir, "--id", "INC-004", "--title", "t"})
	exec(t, []string{"update", "INC-004", "--root", dir, "--note", "ping DB team"})
	// Verify persistence.
	inc, err := incident.Load(filepath.Join(dir, incident.DefaultDir), "INC-004")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(inc.Notes) != 1 || inc.Notes[0] != "ping DB team" {
		t.Errorf("note not persisted: %v", inc.Notes)
	}
}

// TC-CI-05: update illegal transition returns error.
func TestIncident_Update_IllegalTransition(t *testing.T) {
	dir := t.TempDir()
	exec(t, []string{"new", "--root", dir, "--id", "INC-005", "--title", "t"})
	if err := execErr(t, []string{"update", "INC-005", "--root", dir, "--state", "fixed"}); err == nil {
		t.Fatal("want error for illegal transition identified→fixed")
	}
}

// TC-CI-06: list shows all incidents.
func TestIncident_List(t *testing.T) {
	dir := t.TempDir()
	exec(t, []string{"new", "--root", dir, "--id", "INC-010", "--title", "first"})
	exec(t, []string{"new", "--root", dir, "--id", "INC-011", "--title", "second"})
	out := exec(t, []string{"list", "--root", dir})
	if !strings.Contains(out, "INC-010") || !strings.Contains(out, "INC-011") {
		t.Fatalf("want both incidents in list, got: %s", out)
	}
}

// TC-CI-07: list --open excludes closed incidents.
func TestIncident_List_OpenOnly(t *testing.T) {
	dir := t.TempDir()
	exec(t, []string{"new", "--root", dir, "--id", "INC-020", "--title", "open"})
	exec(t, []string{"new", "--root", dir, "--id", "INC-021", "--title", "closed"})
	exec(t, []string{"close", "INC-021", "--root", dir})
	out := exec(t, []string{"list", "--root", dir, "--open"})
	if strings.Contains(out, "INC-021") {
		t.Fatalf("want closed incident excluded from --open list, got: %s", out)
	}
	if !strings.Contains(out, "INC-020") {
		t.Fatalf("want open incident in --open list, got: %s", out)
	}
}

// TC-CI-08: list --json returns a JSON array.
func TestIncident_List_JSON(t *testing.T) {
	dir := t.TempDir()
	exec(t, []string{"new", "--root", dir, "--id", "INC-030", "--title", "t"})
	out := exec(t, []string{"list", "--root", dir, "--json"})
	var arr []map[string]any
	if err := json.Unmarshal([]byte(out), &arr); err != nil {
		t.Fatalf("invalid JSON array: %v\n%s", err, out)
	}
}

// TC-CI-09: close drives incident to fixed.
func TestIncident_Close(t *testing.T) {
	dir := t.TempDir()
	exec(t, []string{"new", "--root", dir, "--id", "INC-040", "--title", "t"})
	out := exec(t, []string{"close", "INC-040", "--root", dir})
	if !strings.Contains(out, "fixed") {
		t.Fatalf("want 'fixed' in close output, got: %s", out)
	}
}

// TC-CI-10: close --postmortem sets postmortem link.
func TestIncident_Close_Postmortem(t *testing.T) {
	dir := t.TempDir()
	exec(t, []string{"new", "--root", dir, "--id", "INC-041", "--title", "t"})
	exec(t, []string{"close", "INC-041", "--root", dir, "--postmortem", "docs/pm/pm-001.md"})
	inc, err := incident.Load(filepath.Join(dir, incident.DefaultDir), "INC-041")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if inc.Postmortem != "docs/pm/pm-001.md" {
		t.Errorf("postmortem link not set: %q", inc.Postmortem)
	}
	if inc.State != incident.StatePostMortemPublished {
		t.Errorf("want StatePostMortemPublished, got %s", inc.State)
	}
}

// TC-CI-11: new missing required --id returns error.
func TestIncident_New_MissingID(t *testing.T) {
	dir := t.TempDir()
	if err := execErr(t, []string{"new", "--root", dir, "--title", "t"}); err == nil {
		t.Fatal("want error for missing --id")
	}
}

// TC-CI-12: idempotency — same list call returns same result.
func TestIncident_Idempotency(t *testing.T) {
	dir := t.TempDir()
	exec(t, []string{"new", "--root", dir, "--id", "INC-050", "--title", "idem", "--json"})
	out1 := exec(t, []string{"list", "--root", dir, "--json"})
	out2 := exec(t, []string{"list", "--root", dir, "--json"})
	if out1 != out2 {
		t.Fatalf("list not idempotent:\n1: %s\n2: %s", out1, out2)
	}
}

// TC-CI-13: false-positive guard — closed incident not in --open list.
func TestIncident_FalsePositive_ClosedNotOpen(t *testing.T) {
	dir := t.TempDir()
	exec(t, []string{"new", "--root", dir, "--id", "INC-060", "--title", "check"})
	exec(t, []string{"close", "INC-060", "--root", dir})
	out := exec(t, []string{"list", "--root", dir, "--open"})
	if strings.Contains(out, "INC-060") {
		t.Fatalf("false-positive: closed incident INC-060 appeared in --open list")
	}
}

// ── G-111: TestIncident_Triage ────────────────────────────────────────────────

// TestIncident_Triage_Text verifies that `forge incident triage` (text mode)
// outputs a human-readable triage summary.
func TestIncident_Triage_Text(t *testing.T) {
	out := exec(t, []string{"triage"})
	if !strings.Contains(out, "triage:") {
		t.Errorf("triage text output missing 'triage:' prefix, got: %q", out)
	}
}

// TestIncident_Triage_JSON verifies that `forge incident triage --json` emits
// valid JSON containing the expected fields: status and labels.
func TestIncident_Triage_JSON(t *testing.T) {
	out := exec(t, []string{"triage", "--json"})
	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("triage --json produced invalid JSON: %v\noutput: %q", err, out)
	}
	if _, ok := result["status"]; !ok {
		t.Errorf("triage JSON missing 'status' field, got keys: %v", mapKeys(result))
	}
	if _, ok := result["labels"]; !ok {
		t.Errorf("triage JSON missing 'labels' field, got keys: %v", mapKeys(result))
	}
	if result["status"] != "triage_pending" {
		t.Errorf("status: want %q, got %v", "triage_pending", result["status"])
	}
}

// TestIncident_Triage_WithInputFile verifies that `--input` is accepted even
// when the file path is provided (no errors on valid file).
func TestIncident_Triage_WithInputFile(t *testing.T) {
	// Write a minimal JSON bundle.
	bundle := `{"errors":["timeout in CI"],"context":"test run"}`
	f := filepath.Join(t.TempDir(), "bundle.json")
	if err := os.WriteFile(f, []byte(bundle), 0o644); err != nil {
		t.Fatalf("write bundle: %v", err)
	}
	out := exec(t, []string{"triage", "--input", f, "--json"})
	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("triage --input --json produced invalid JSON: %v\noutput: %q", err, out)
	}
	if result["status"] != "triage_pending" {
		t.Errorf("status: want %q, got %v", "triage_pending", result["status"])
	}
}

// mapKeys returns the keys of a map as a sorted slice (for error messages).
func mapKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
