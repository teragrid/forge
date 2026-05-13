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
package cmdinsights

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/teragrid/forge/internal/audit"
)

// seedLedger writes n entries into a temp ledger and returns the path.
func seedLedger(t *testing.T, entries []audit.Entry) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, ".forge", "audit.log")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	l, err := audit.Open(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if _, err := l.Append(e); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// TC-STATS-01 (happy + data-accuracy): counts aggregate correctly.
func TestBuildReport_Counts(t *testing.T) {
	entries := []audit.Entry{
		{Verb: "scan", Action: "run"},
		{Verb: "scan", Action: "run"},
		{Verb: "ship", Action: "deploy"},
	}
	r := buildReport(entries, time.Time{}, "")
	if r.TotalEvents != 3 {
		t.Errorf("want TotalEvents=3, got %d", r.TotalEvents)
	}
	var scanStat *VerbStat
	for i := range r.Verbs {
		if r.Verbs[i].Verb == "scan" {
			scanStat = &r.Verbs[i]
		}
	}
	if scanStat == nil {
		t.Fatal("scan verb missing from report")
	}
	if scanStat.Count != 2 {
		t.Errorf("want scan count=2, got %d", scanStat.Count)
	}
	if scanStat.ActionBreakdown["run"] != 2 {
		t.Errorf("want scan/run breakdown=2, got %d", scanStat.ActionBreakdown["run"])
	}
}

// TC-STATS-02 (data-accuracy): verbs are sorted descending by count.
func TestBuildReport_SortedByCount(t *testing.T) {
	entries := []audit.Entry{
		{Verb: "lint", Action: "a"},
		{Verb: "scan", Action: "a"},
		{Verb: "scan", Action: "a"},
		{Verb: "scan", Action: "a"},
	}
	r := buildReport(entries, time.Time{}, "")
	if r.Verbs[0].Verb != "scan" {
		t.Errorf("expected scan first (highest count), got %s", r.Verbs[0].Verb)
	}
}

// TC-STATS-03 (boundary): empty ledger returns zero events.
func TestBuildReport_Empty(t *testing.T) {
	r := buildReport(nil, time.Time{}, "")
	if r.TotalEvents != 0 || len(r.Verbs) != 0 {
		t.Errorf("expected empty report, got %+v", r)
	}
}

// TC-STATS-04 (data-accuracy): --since filter excludes old events.
func TestBuildReport_SinceFilter(t *testing.T) {
	old := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	recent := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	cutoff := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	entries := []audit.Entry{
		{Verb: "old", Action: "a", Timestamp: old},
		{Verb: "new", Action: "b", Timestamp: recent},
	}
	r := buildReport(entries, cutoff, "2026-01-01")
	if r.TotalEvents != 1 {
		t.Errorf("want 1 event after filter, got %d", r.TotalEvents)
	}
	if r.Verbs[0].Verb != "new" {
		t.Errorf("wrong verb after filter: %s", r.Verbs[0].Verb)
	}
}

// TC-STATS-05 (false-positive guard): --since excludes nothing when all events are newer.
func TestBuildReport_SinceNoExclusion(t *testing.T) {
	future := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	cutoff := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	entries := []audit.Entry{{Verb: "scan", Action: "a", Timestamp: future}}
	r := buildReport(entries, cutoff, "2025-01-01")
	if r.TotalEvents != 1 {
		t.Errorf("should include event; TotalEvents=%d", r.TotalEvents)
	}
}

// TC-STATS-06 (happy, e2e): --json flag emits valid JSON with expected keys.
func TestNew_JSONOutput(t *testing.T) {
	root := seedLedger(t, []audit.Entry{
		{Verb: "scan", Action: "run"},
		{Verb: "ship", Action: "deploy"},
	})
	cmd := New()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--root", root, "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var report Report
	if err := json.Unmarshal(buf.Bytes(), &report); err != nil {
		t.Fatalf("JSON parse: %v\noutput: %s", err, buf.String())
	}
	if report.TotalEvents != 2 {
		t.Errorf("want 2 events, got %d", report.TotalEvents)
	}
}

// TC-STATS-07 (happy, e2e): human-readable table output contains verb names.
func TestNew_TextOutput(t *testing.T) {
	root := seedLedger(t, []audit.Entry{
		{Verb: "lint", Action: "check"},
	})
	cmd := New()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--root", root})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(buf.String(), "lint") {
		t.Errorf("expected 'lint' in output, got: %s", buf.String())
	}
}

// TC-STATS-08 (boundary): empty ledger prints "no audit events".
func TestNew_EmptyLedger(t *testing.T) {
	root := seedLedger(t, nil)
	cmd := New()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--root", root})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(buf.String(), "no audit events") {
		t.Errorf("expected 'no audit events', got: %s", buf.String())
	}
}

// TC-STATS-09 (negative): invalid --since date returns error.
func TestNew_InvalidSince(t *testing.T) {
	root := t.TempDir()
	cmd := New()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--root", root, "--since", "not-a-date"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error for invalid --since date")
	}
}

// TC-STATS-10 (idempotency): two consecutive runs return same TotalEvents.
func TestNew_Idempotent(t *testing.T) {
	root := seedLedger(t, []audit.Entry{
		{Verb: "scan", Action: "run"},
	})

	run := func() Report {
		cmd := New()
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)
		cmd.SetArgs([]string{"--root", root, "--json"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("Execute: %v", err)
		}
		var r Report
		_ = json.Unmarshal(buf.Bytes(), &r)
		return r
	}
	r1, r2 := run(), run()
	if r1.TotalEvents != r2.TotalEvents {
		t.Errorf("idempotency broken: %d vs %d", r1.TotalEvents, r2.TotalEvents)
	}
}
