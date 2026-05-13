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
package cmdaudit

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/teragrid/forge/internal/failure"
)

// TC-CMDAUDIT-01 (happy): append → show shows entry.
func TestAudit_AppendThenShow(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	app := New()
	var aOut bytes.Buffer
	app.SetOut(&aOut)
	app.SetArgs([]string{"append", "--root", dir, "--verb", "scan", "--action", "secrets-clean"})
	if err := app.Execute(); err != nil {
		t.Fatalf("append: %v", err)
	}

	show := New()
	var sOut bytes.Buffer
	show.SetOut(&sOut)
	show.SetArgs([]string{"show", "--root", dir})
	if err := show.Execute(); err != nil {
		t.Fatalf("show: %v", err)
	}
	if !strings.Contains(sOut.String(), "scan/secrets-clean") {
		t.Fatalf("entry not shown: %s", sOut.String())
	}
}

// TC-CMDAUDIT-02 (happy + data-accuracy): verify reports intact JSON.
func TestAudit_VerifyJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	app := New()
	app.SetOut(new(bytes.Buffer))
	app.SetArgs([]string{"append", "--root", dir, "--verb", "x", "--action", "y"})
	_ = app.Execute()

	v := New()
	var out bytes.Buffer
	v.SetOut(&out)
	v.SetArgs([]string{"verify", "--root", dir, "--json"})
	if err := v.Execute(); err != nil {
		t.Fatalf("verify: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("not JSON: %v", err)
	}
	if got["intact"] != true {
		t.Fatalf("not intact: %+v", got)
	}
}

// TC-CMDAUDIT-03 (negative): append without --verb fails.
func TestAudit_AppendRequiresVerbAction(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"append", "--root", dir, "--action", "x"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "FORGE-3400") {
		t.Fatalf("want FORGE-3400, got: %v", err)
	}
}

// TC-CMDAUDIT-04 (negative): unknown subcommand errors.
func TestAudit_UnknownSubcommand(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"whatever", "--root", dir})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
}

// TC-CMDAUDIT-05 (boundary): show on empty ledger.
func TestAudit_ShowEmpty(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"show", "--root", dir})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("show empty: %v", err)
	}
	if !strings.Contains(out.String(), "0 entries") {
		t.Fatalf("expected '0 entries': %s", out.String())
	}
}

// --- failure-register sub-subcommand tests ---

// seedRegister writes a failure register JSON into dir/.forge/.
func seedRegister(t *testing.T, dir string, reg *failure.Register) {
	t.Helper()
	dotForge := filepath.Join(dir, ".forge")
	if err := os.MkdirAll(dotForge, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := reg.Save(filepath.Join(dotForge, "failure-register.json")); err != nil {
		t.Fatal(err)
	}
}

// TC-CMDAUDIT-FR-01 (happy): failure-register lint passes for valid schema.
func TestAudit_FailureRegisterLint_OK(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	reg := failure.New()
	reg.Entries = []failure.Entry{
		{ID: "FR-001", Component: "c", FailureMode: "m", TestAnchor: "TEST-01", Status: failure.StatusTracked},
	}
	seedRegister(t, dir, reg)

	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"failure-register", "lint", "--root", dir})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("lint: %v\noutput: %s", err, out.String())
	}
	if !strings.Contains(out.String(), "OK") {
		t.Errorf("expected OK in output, got: %s", out.String())
	}
}

// TC-CMDAUDIT-FR-02 (happy): failure-register list --json returns valid JSON.
func TestAudit_FailureRegisterList_JSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	reg := failure.New()
	reg.Entries = []failure.Entry{
		{ID: "FR-001", Component: "c", FailureMode: "m", Status: failure.StatusTracked},
		{ID: "FR-002", Component: "c", FailureMode: "retired", Status: failure.StatusRetired},
	}
	seedRegister(t, dir, reg)

	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"failure-register", "list", "--root", dir, "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("list: %v\noutput: %s", err, out.String())
	}
	var entries []failure.Entry
	if err := json.Unmarshal(out.Bytes(), &entries); err != nil {
		t.Fatalf("JSON parse: %v\noutput: %s", err, out.String())
	}
	// list returns only active entries (1).
	if len(entries) != 1 {
		t.Errorf("expected 1 active entry, got %d: %+v", len(entries), entries)
	}
}

// TC-CMDAUDIT-FR-03 (negative): failure-register verify returns FORGE-3702 on drift.
func TestAudit_FailureRegisterVerify_Drift(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	reg := failure.New()
	// Entry missing test_anchor → drift.
	reg.Entries = []failure.Entry{
		{ID: "FR-001", Component: "c", FailureMode: "m", Status: failure.StatusTracked},
	}
	seedRegister(t, dir, reg)

	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"failure-register", "verify", "--root", dir})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected drift error")
	}
	if !strings.Contains(err.Error(), "FORGE-3702") {
		t.Errorf("expected FORGE-3702, got: %v", err)
	}
}

// TC-CMDAUDIT-FR-04 (happy): failure-register verify passes when all entries have test_anchor.
func TestAudit_FailureRegisterVerify_OK(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	reg := failure.New()
	reg.Entries = []failure.Entry{
		{ID: "FR-001", Component: "c", FailureMode: "m", TestAnchor: "TEST-01", Status: failure.StatusTracked},
	}
	seedRegister(t, dir, reg)

	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"failure-register", "verify", "--root", dir})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TC-CMDAUDIT-FR-05 (boundary): failure-register list on empty register returns [].
func TestAudit_FailureRegisterList_Empty(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	seedRegister(t, dir, failure.New())

	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"failure-register", "list", "--root", dir, "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("list: %v", err)
	}
	var entries []failure.Entry
	_ = json.Unmarshal(out.Bytes(), &entries)
	if len(entries) != 0 {
		t.Errorf("expected empty list, got %d", len(entries))
	}
}
