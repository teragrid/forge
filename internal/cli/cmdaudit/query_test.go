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
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/teragrid/forge/internal/audit"
)

// runCmd executes the given root cobra.Command with args and returns stdout as
// a string. Fails the test on any error.
func runCmd(t *testing.T, root *cobra.Command, args []string) string {
	t.Helper()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs(args)
	if err := root.Execute(); err != nil {
		t.Fatalf("runCmd %v: %v", args, err)
	}
	return buf.String()
}

// seedEntries opens the ledger at dir/.forge/audit.log and appends entries.
func seedEntries(t *testing.T, dir string, entries []audit.Entry) {
	t.Helper()
	l, err := audit.Open(filepath.Join(dir, audit.DefaultPath))
	if err != nil {
		t.Fatalf("seedEntries open: %v", err)
	}
	for _, e := range entries {
		if _, err := l.Append(e); err != nil {
			t.Fatalf("seedEntries append: %v", err)
		}
	}
}

// TC-QUERY-01: happy path — unfiltered returns all entries.
func TestQuery_AllEntries(t *testing.T) {
	dir := t.TempDir()
	seedEntries(t, dir, []audit.Entry{
		{Verb: "scan", Action: "run"},
		{Verb: "ship", Action: "run"},
	})
	out := runCmd(t, New(), []string{"query", "--root", dir})
	if !strings.Contains(out, "2 result(s)") {
		t.Fatalf("want 2 results, got: %s", out)
	}
}

// TC-QUERY-02: --verb filter returns only matching entries.
func TestQuery_VerbFilter(t *testing.T) {
	dir := t.TempDir()
	seedEntries(t, dir, []audit.Entry{
		{Verb: "scan", Action: "run"},
		{Verb: "ship", Action: "run"},
		{Verb: "scan", Action: "lint"},
	})
	out := runCmd(t, New(), []string{"query", "--root", dir, "--verb", "scan"})
	if !strings.Contains(out, "2 result(s)") {
		t.Fatalf("want 2 scan results, got: %s", out)
	}
}

// TC-QUERY-03: --action filter
func TestQuery_ActionFilter(t *testing.T) {
	dir := t.TempDir()
	seedEntries(t, dir, []audit.Entry{
		{Verb: "scan", Action: "run"},
		{Verb: "ship", Action: "run"},
		{Verb: "eval", Action: "run"},
	})
	out := runCmd(t, New(), []string{"query", "--root", dir, "--action", "run"})
	if !strings.Contains(out, "3 result(s)") {
		t.Fatalf("want 3 run results, got: %s", out)
	}
}

// TC-QUERY-04: --verb + --action combined (AND semantics).
func TestQuery_VerbAndActionCombined(t *testing.T) {
	dir := t.TempDir()
	seedEntries(t, dir, []audit.Entry{
		{Verb: "scan", Action: "run"},
		{Verb: "ship", Action: "run"},
		{Verb: "scan", Action: "lint"},
	})
	out := runCmd(t, New(), []string{"query", "--root", dir, "--verb", "scan", "--action", "run"})
	if !strings.Contains(out, "1 result(s)") {
		t.Fatalf("want 1 result, got: %s", out)
	}
}

// TC-QUERY-05: empty ledger returns 0 results (not an error).
func TestQuery_EmptyLedger(t *testing.T) {
	dir := t.TempDir()
	out := runCmd(t, New(), []string{"query", "--root", dir})
	if !strings.Contains(out, "0 result(s)") {
		t.Fatalf("want 0 results, got: %s", out)
	}
}

// TC-QUERY-06: --limit caps output count.
func TestQuery_Limit(t *testing.T) {
	dir := t.TempDir()
	seedEntries(t, dir, []audit.Entry{
		{Verb: "scan", Action: "a"},
		{Verb: "scan", Action: "b"},
		{Verb: "scan", Action: "c"},
	})
	out := runCmd(t, New(), []string{"query", "--root", dir, "--limit", "2"})
	if !strings.Contains(out, "2 result(s)") {
		t.Fatalf("want 2 results with limit, got: %s", out)
	}
}

// TC-QUERY-07: --json emits valid JSON array.
func TestQuery_JSON(t *testing.T) {
	dir := t.TempDir()
	seedEntries(t, dir, []audit.Entry{
		{Verb: "ship", Action: "run"},
	})
	out := runCmd(t, New(), []string{"query", "--root", dir, "--json"})
	var entries []audit.Entry
	if err := json.Unmarshal([]byte(out), &entries); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if len(entries) != 1 {
		t.Fatalf("want 1 entry, got %d", len(entries))
	}
	if entries[0].Verb != "ship" {
		t.Fatalf("want verb=ship, got %s", entries[0].Verb)
	}
}

// TC-QUERY-08: invalid --since format returns error.
func TestQuery_BadSince(t *testing.T) {
	dir := t.TempDir()
	cmd := New()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"query", "--root", dir, "--since", "not-a-date"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error for bad --since, got nil")
	}
}

// TC-QUERY-09: --since filters old entries correctly.
func TestQuery_SinceFilter(t *testing.T) {
	dir := t.TempDir()
	l, err := audit.Open(filepath.Join(dir, audit.DefaultPath))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	// Append an entry with an old timestamp (7 days ago).
	old := audit.Entry{Verb: "scan", Action: "old", Timestamp: time.Now().UTC().AddDate(0, 0, -7)}
	if _, err := l.Append(old); err != nil {
		t.Fatalf("append old: %v", err)
	}
	// Append a recent entry.
	recent := audit.Entry{Verb: "scan", Action: "new", Timestamp: time.Now().UTC()}
	if _, err := l.Append(recent); err != nil {
		t.Fatalf("append recent: %v", err)
	}

	yesterday := time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02")
	out := runCmd(t, New(), []string{"query", "--root", dir, "--since", yesterday})
	if !strings.Contains(out, "1 result(s)") {
		t.Fatalf("want 1 recent result, got: %s", out)
	}
}

// TC-QUERY-10: false-positive guard — filter for nonexistent verb returns 0, not error.
func TestQuery_NoMatchIsNotError(t *testing.T) {
	dir := t.TempDir()
	seedEntries(t, dir, []audit.Entry{{Verb: "scan", Action: "run"}})
	out := runCmd(t, New(), []string{"query", "--root", dir, "--verb", "nonexistent"})
	if !strings.Contains(out, "0 result(s)") {
		t.Fatalf("want 0 results, got: %s", out)
	}
}

// TC-QUERY-11: data-accuracy — JSON result count matches seeded count.
func TestQuery_DataAccuracy(t *testing.T) {
	dir := t.TempDir()
	seedEntries(t, dir, []audit.Entry{
		{Verb: "ship", Action: "a"},
		{Verb: "ship", Action: "b"},
		{Verb: "ship", Action: "c"},
	})
	out := runCmd(t, New(), []string{"query", "--root", dir, "--verb", "ship", "--json"})
	var entries []audit.Entry
	if err := json.Unmarshal([]byte(out), &entries); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("data-accuracy: want 3 entries, got %d", len(entries))
	}
}
