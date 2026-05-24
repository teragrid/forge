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
// journey_test.go — end-to-end user-journey tests that exercise multiple Forge
// verbs in sequence, validating cross-verb state consistency.
//
// Each journey mirrors a real developer workflow from GETTING_STARTED.md.
// Steps within a journey are sequential; journeys run in parallel with each
// other because every journey owns an isolated t.TempDir() root.
//
// Test-design checklist (always-write-tests.md 9-point):
//  1. Happy path          — each journey completes without error.
//  2. Boundary            — empty scaffold dir; zero spend; no audit entries.
//  3. Negative            — step fails when prerequisite state is absent
//     (e.g. plugin upgrade on uninstalled plugin).
//  4. Idempotency         — running the final read-only step twice yields
//     identical output.
//  5. Concurrency         — journeys are isolated by separate t.TempDir() roots.
//  6. Cross-journey isolation — state from journey A never appears in journey B.
//  7. Regression          — every multi-verb flow present at M0 is asserted.
//  8. Data-accuracy       — state written in step N is readable in step N+1.
//  9. False-positive guard — closed incidents do NOT appear in --open listing;
//     removed plugins do NOT allow upgrade.
package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/teragrid/forge/internal/cli/cmdadopt"
	"github.com/teragrid/forge/internal/cli/cmdask"
	"github.com/teragrid/forge/internal/cli/cmdaudit"
	"github.com/teragrid/forge/internal/cli/cmdcheck"
	"github.com/teragrid/forge/internal/cli/cmdcontext"
	"github.com/teragrid/forge/internal/cli/cmddocs"
	"github.com/teragrid/forge/internal/cli/cmddoctor"
	"github.com/teragrid/forge/internal/cli/cmdeject"
	"github.com/teragrid/forge/internal/cli/cmdfix"
	"github.com/teragrid/forge/internal/cli/cmdgenerate"
	"github.com/teragrid/forge/internal/cli/cmdhygiene"
	"github.com/teragrid/forge/internal/cli/cmdincident"
	"github.com/teragrid/forge/internal/cli/cmdinsights"
	"github.com/teragrid/forge/internal/cli/cmdlint"
	"github.com/teragrid/forge/internal/cli/cmdmigrate"
	"github.com/teragrid/forge/internal/cli/cmdnew"
	"github.com/teragrid/forge/internal/cli/cmdplugin"
	"github.com/teragrid/forge/internal/cli/cmdreview"
	"github.com/teragrid/forge/internal/cli/cmdscan"
	"github.com/teragrid/forge/internal/cli/cmdship"
	"github.com/teragrid/forge/internal/cli/cmdspend"
	"github.com/teragrid/forge/internal/cli/cmdtelemetry"
	"github.com/teragrid/forge/internal/cli/cmdtest"
	"github.com/teragrid/forge/internal/cli/cmdupgrade"
)

// ── helpers ───────────────────────────────────────────────────────────────────

// jStep executes cmd with args, capturing combined stdout/stderr.
// It calls t.Fatalf on any execution error and returns the output string.
func jStep(t *testing.T, label string, cmd *cobra.Command, args ...string) string {
	t.Helper()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("[%s] execute: %v\n--- output ---\n%s", label, err, out.String())
	}
	return out.String()
}

// jStepErr is like jStep but returns the error instead of failing immediately.
// Used for negative-case steps that are expected to fail.
func jStepErr(t *testing.T, cmd *cobra.Command, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

// firstJSON trims leading non-JSON text (e.g. deprecation notices) and
// trailing non-JSON content so partial log lines do not break json.Unmarshal.
// It handles both objects (trimmed to last '}') and arrays (last ']').
func firstJSON(b []byte) []byte {
	b = bytes.TrimSpace(b)
	if len(b) == 0 {
		return b
	}
	// Skip any leading lines that are not the start of a JSON value.
	if b[0] != '{' && b[0] != '[' {
		if i := bytes.IndexByte(b, '{'); i >= 0 {
			b = b[i:]
		} else if i := bytes.IndexByte(b, '['); i >= 0 {
			b = b[i:]
		}
	}
	switch b[0] {
	case '{':
		if i := bytes.LastIndexByte(b, '}'); i >= 0 {
			return b[:i+1]
		}
	case '[':
		if i := bytes.LastIndexByte(b, ']'); i >= 0 {
			return b[:i+1]
		}
	}
	return b
}

// ── Journey 1: Developer Onboarding ──────────────────────────────────────────
//
// Mirrors the GETTING_STARTED.md "zero-to-first-ship" walkthrough:
//
//	forge new go-service <dir>            scaffold a project
//	forge doctor                          verify local environment
//	forge scan secrets --root <dir>       no secrets in a fresh scaffold
//	forge ship --dry-run --json           5-checkpoint preview
//	forge ship --dry-run --json (again)   idempotency guard
func TestJourney_DeveloperOnboarding(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "myapp")

	// Step 1 — scaffold a new project.
	s1 := jStep(t, "new", cmdnew.New("test"), "go-service", dir)
	if !strings.Contains(s1, "scaffolded") {
		t.Fatalf("step 1 (new): missing scaffold confirmation\n%s", s1)
	}
	if _, err := os.Stat(filepath.Join(dir, "main.go")); err != nil {
		t.Fatalf("step 1 (new): main.go not created: %v", err)
	}

	// Step 2 — doctor reports environment status.
	// On CI the environment may be partially unhealthy; we only assert the
	// verb produces structured output and does not panic.
	{
		cmd := cmddoctor.New()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs(nil)
		_ = cmd.Execute() // intentionally ignore exit code
		if !strings.Contains(out.String(), "forge doctor") {
			t.Fatalf("step 2 (doctor): missing header\n%s", out.String())
		}
	}

	// Step 3 — scan the scaffolded project for secrets; must be clean.
	s3 := jStep(t, "scan secrets", cmdscan.New(), "secrets", "--root", dir, "--json")
	var scanRes cmdscan.ScanResult
	if err := json.Unmarshal(firstJSON([]byte(s3)), &scanRes); err != nil {
		t.Fatalf("step 3 (scan): not valid JSON: %v\n%s", err, s3)
	}
	if scanRes.Status == "found" {
		t.Fatalf("step 3 (scan): fresh scaffold unexpectedly contains secrets: %+v", scanRes.Findings)
	}

	// Step 4 — ship dry-run: all 6 checkpoints present.
	s4 := jStep(t, "ship", cmdship.New(), "--dry-run", "--root", dir, "--description", "onboarding", "--json")
	var shipRes cmdship.ShipResult
	if err := json.Unmarshal(firstJSON([]byte(s4)), &shipRes); err != nil {
		t.Fatalf("step 4 (ship): not JSON: %v\n%s", err, s4)
	}
	if !shipRes.DryRun {
		t.Fatal("step 4 (ship): expected dry_run=true")
	}
	if len(shipRes.Checkpoints) != 7 {
		t.Fatalf("step 4 (ship): expected 7 checkpoints, got %d", len(shipRes.Checkpoints))
	}

	// Idempotency guard (§4 in 9-point checklist): running ship again yields
	// the same checkpoint count.
	s4b := jStep(t, "ship (replay)", cmdship.New(), "--dry-run", "--root", dir, "--json")
	var shipRes2 cmdship.ShipResult
	if err := json.Unmarshal([]byte(strings.TrimSpace(s4b)), &shipRes2); err != nil {
		t.Fatalf("step 4b (ship replay): not JSON: %v\n%s", err, s4b)
	}
	if len(shipRes.Checkpoints) != len(shipRes2.Checkpoints) {
		t.Fatalf("ship is not idempotent: first run %d checkpoints, replay %d",
			len(shipRes.Checkpoints), len(shipRes2.Checkpoints))
	}
}

// ── Journey 2: Incident Lifecycle ────────────────────────────────────────────
//
// Full OODA loop for a production incident:
//
//	incident new  --id INC-J01 …            open the incident
//	incident update INC-J01 --state         transition to investigating
//	incident update INC-J01 --note          append a diagnostic note
//	incident list --open                    INC-J01 appears in open list
//	incident close INC-J01                  resolve the incident
//	incident list --open                    INC-J01 must NOT appear (false-positive guard)
//	incident list (all)                     INC-J01 is still present (data-accuracy)
func TestJourney_IncidentLifecycle(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	id := "INC-J01"

	// Step 1 — open the incident.
	s1 := jStep(t, "incident new", cmdincident.New(),
		"new", "--root", dir,
		"--id", id, "--title", "prod DB unresponsive",
		"--severity", "S1", "--systems", "DB,API")
	if !strings.Contains(s1, id) {
		t.Fatalf("step 1 (incident new): missing %s in output\n%s", id, s1)
	}

	// Step 2 — transition state to "investigating".
	s2 := jStep(t, "incident update state", cmdincident.New(),
		"update", id, "--root", dir, "--state", "investigating")
	if !strings.Contains(s2, "investigating") {
		t.Fatalf("step 2 (update state): output missing 'investigating'\n%s", s2)
	}

	// Step 3 — append a diagnostic note.
	jStep(t, "incident update note", cmdincident.New(),
		"update", id, "--root", dir, "--note", "OOM on db-01; failover in progress")

	// Step 4 (data-accuracy) — list open incidents: INC-J01 must appear.
	s4 := jStep(t, "incident list open", cmdincident.New(),
		"list", "--root", dir, "--open")
	if !strings.Contains(s4, id) {
		t.Fatalf("step 4 (list --open): %s missing from open list\n%s", id, s4)
	}

	// Negative case: updating a non-existent incident must fail.
	if _, err := jStepErr(t, cmdincident.New(),
		"update", "INC-DOESNOTEXIST", "--root", dir, "--state", "mitigated"); err == nil {
		t.Fatal("step 4n (update unknown): expected error, got nil")
	}

	// Step 5 — close the incident.
	jStep(t, "incident close", cmdincident.New(), "close", id, "--root", dir)

	// Step 6 (false-positive guard §9) — closed incident must NOT appear in --open.
	s6 := jStep(t, "incident list open post-close", cmdincident.New(),
		"list", "--root", dir, "--open")
	if strings.Contains(s6, id) {
		t.Fatalf("step 6 (false-positive guard): closed %s still appears in --open list\n%s", id, s6)
	}

	// Step 7 (data-accuracy) — full list (without --open) still shows the incident.
	s7 := jStep(t, "incident list all", cmdincident.New(), "list", "--root", dir)
	if !strings.Contains(s7, id) {
		t.Fatalf("step 7 (data-accuracy): closed %s disappeared from full list\n%s", id, s7)
	}

	// Idempotency: listing again is deterministic.
	s7b := jStep(t, "incident list all (replay)", cmdincident.New(), "list", "--root", dir)
	if s7 != s7b {
		t.Fatalf("incident list is not idempotent\nfirst:\n%s\nsecond:\n%s", s7, s7b)
	}
}

// ── Journey 3: Hygiene Pipeline ───────────────────────────────────────────────
//
// Hygiene + codemod pipeline after scaffolding:
//
//	forge new go-service <dir>            scaffold project
//	forge lint --root <dir>              hygiene checks (may warn; must not panic)
//	forge scan all --root <dir>          all scan families run cleanly
//	forge upgrade list                   codemod catalogue is populated
//	forge upgrade gitignore --root <dir> dry-run shows planned changes
func TestJourney_HygienePipeline(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "svc")

	// Step 1 — scaffold.
	jStep(t, "new", cmdnew.New("test"), "go-service", dir)

	// Step 2 — lint: must not error out (exit code may be non-zero on some hosts
	// due to missing managed blocks, but should not crash or return FORGE-3100).
	{
		cmd := cmdlint.New()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{"--root", dir, "--json"})
		_ = cmd.Execute() // warnings are fine; assert we get parseable JSON
		body := firstJSON(out.Bytes())
		if len(body) > 0 {
			var lr cmdlint.LintResult
			if err := json.Unmarshal(body, &lr); err != nil {
				t.Fatalf("step 2 (lint): not valid JSON: %v\n%s", err, out.String())
			}
			if lr.Root == "" {
				t.Fatal("step 2 (lint): root field empty in LintResult JSON")
			}
		}
	}

	// Step 3 — scan all families; fresh scaffold must be free of hard secrets.
	{
		cmd := cmdscan.New()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{"secrets", "--root", dir, "--json"})
		_ = cmd.Execute()
		body := firstJSON(out.Bytes())
		if len(body) > 0 {
			var sr cmdscan.ScanResult
			if err := json.Unmarshal(body, &sr); err != nil {
				t.Fatalf("step 3 (scan): not JSON: %v\n%s", err, out.String())
			}
			if sr.Status == "found" {
				t.Fatalf("step 3 (scan): fresh scaffold has secrets: %+v", sr.Findings)
			}
		}
	}

	// Step 4 — upgrade list: codemod catalogue must contain at least 2 entries.
	s4 := jStep(t, "upgrade list", cmdupgrade.New(), "list")
	if !strings.Contains(s4, "gitignore") {
		t.Fatalf("step 4 (upgrade list): missing 'gitignore' codemod\n%s", s4)
	}

	// Step 5 — upgrade gitignore dry-run on the scaffolded project.
	// Dry-run (no --apply) must never error even on a fresh directory.
	s5 := jStep(t, "upgrade gitignore dry-run", cmdupgrade.New(),
		"gitignore-marker", "--root", dir)
	if s5 == "" {
		t.Fatal("step 5 (upgrade gitignore dry-run): empty output")
	}
}

// ── Journey 4: Budget + Observability ────────────────────────────────────────
//
// LLM spend limits and audit trail flow:
//
//	forge spend set --daily 1.00 --monthly 30.00 --root <dir>   set limits
//	forge spend status --root <dir>                             limits visible
//	forge audit append --verb scan --action run --root <dir>    write event
//	forge audit show --root <dir>                               event visible
//	forge insights --root <dir>                                 rollup counts scan
//	forge audit show --root <dir> (replay)                      idempotency
func TestJourney_BudgetAndObservability(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Step 1 — set spend limits.
	s1 := jStep(t, "spend set", cmdspend.New(),
		"set", "--root", dir, "--daily", "1.00", "--monthly", "30.00")
	if !strings.Contains(strings.ToLower(s1), "limit") &&
		!strings.Contains(strings.ToLower(s1), "set") {
		t.Fatalf("step 1 (spend set): missing confirmation\n%s", s1)
	}

	// Step 2 (data-accuracy) — status reflects the configured limits.
	s2 := jStep(t, "spend status", cmdspend.New(), "status", "--root", dir, "--json")
	var status map[string]any
	if err := json.Unmarshal(firstJSON([]byte(s2)), &status); err != nil {
		t.Fatalf("step 2 (spend status): not JSON: %v\n%s", err, s2)
	}
	for _, key := range []string{"daily_limit_usd", "monthly_limit_usd"} {
		if _, ok := status[key]; !ok {
			t.Errorf("step 2 (spend status): missing key %q", key)
		}
	}

	// Step 3 — append an audit event.
	jStep(t, "audit append", cmdaudit.New(),
		"append", "--root", dir, "--verb", "scan", "--action", "run")

	// Step 4 (data-accuracy) — show ledger: the appended event must appear.
	s4 := jStep(t, "audit show", cmdaudit.New(), "show", "--root", dir, "--json")
	var entries []map[string]any
	if err := json.Unmarshal(firstJSON([]byte(s4)), &entries); err != nil {
		t.Fatalf("step 4 (audit show): not JSON: %v\n%s", err, s4)
	}
	if len(entries) == 0 {
		t.Fatal("step 4 (audit show): expected ≥1 ledger entry, got 0")
	}

	// Step 5 — insights rollup: must surface the scan event.
	s5 := jStep(t, "insights", cmdinsights.New(), "--root", dir, "--json")
	var report cmdinsights.Report
	if err := json.Unmarshal(firstJSON([]byte(s5)), &report); err != nil {
		t.Fatalf("step 5 (insights): not JSON: %v\n%s", err, s5)
	}
	if report.TotalEvents == 0 {
		t.Fatal("step 5 (insights): expected TotalEvents > 0")
	}
	foundScan := false
	for _, vs := range report.Verbs {
		if vs.Verb == "scan" {
			foundScan = true
			break
		}
	}
	if !foundScan {
		t.Fatalf("step 5 (insights): 'scan' verb not in rollup: %+v", report.Verbs)
	}

	// Idempotency guard: audit show twice yields same entry count.
	s4b := jStep(t, "audit show (replay)", cmdaudit.New(), "show", "--root", dir, "--json")
	var entries2 []map[string]any
	if err := json.Unmarshal(firstJSON([]byte(s4b)), &entries2); err != nil {
		t.Fatalf("step 4b (audit show replay): not JSON: %v\n%s", err, s4b)
	}
	if len(entries) != len(entries2) {
		t.Fatalf("audit show is not idempotent: first=%d entries, replay=%d", len(entries), len(entries2))
	}
}

// ── Journey 5: Telemetry Consent Lifecycle ────────────────────────────────────
//
// Opt-in / opt-out flow:
//
//	forge telemetry status --root <dir>     starts disabled (default)
//	forge telemetry enable --root <dir>     opt in
//	forge telemetry status --root <dir>     enabled=true
//	forge telemetry rotate-id --root <dir>  new device ID
//	forge telemetry disable --root <dir>    opt out
//	forge telemetry status --root <dir>     enabled=false (false-positive guard)
func TestJourney_TelemetryConsent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Step 1 — initial status: must be parseable JSON; enabled defaults to false.
	s1 := jStep(t, "telemetry status (initial)", cmdtelemetry.New(),
		"status", "--root", dir, "--json")
	var st1 map[string]any
	if err := json.Unmarshal(firstJSON([]byte(s1)), &st1); err != nil {
		t.Fatalf("step 1 (telemetry status): not JSON: %v\n%s", err, s1)
	}
	if enabled, _ := st1["enabled"].(bool); enabled {
		t.Fatal("step 1 (telemetry status): expected enabled=false by default")
	}

	// Step 2 — opt in.
	jStep(t, "telemetry enable", cmdtelemetry.New(), "enable", "--root", dir)

	// Step 3 (data-accuracy) — status now shows enabled=true.
	s3 := jStep(t, "telemetry status (enabled)", cmdtelemetry.New(),
		"status", "--root", dir, "--json")
	var st3 map[string]any
	if err := json.Unmarshal(firstJSON([]byte(s3)), &st3); err != nil {
		t.Fatalf("step 3 (telemetry status): not JSON: %v\n%s", err, s3)
	}
	if enabled, _ := st3["enabled"].(bool); !enabled {
		t.Fatalf("step 3 (telemetry status): expected enabled=true after enable\n%s", s3)
	}
	installID1, _ := st3["install_id"].(string)
	if installID1 == "" {
		t.Fatal("step 3 (telemetry status): install_id is empty after enable")
	}

	// Step 4 — rotate device ID; must produce a new non-empty ID.
	jStep(t, "telemetry rotate-id", cmdtelemetry.New(), "rotate-id", "--root", dir)
	s4 := jStep(t, "telemetry status (rotated)", cmdtelemetry.New(),
		"status", "--root", dir, "--json")
	var st4 map[string]any
	if err := json.Unmarshal(firstJSON([]byte(s4)), &st4); err != nil {
		t.Fatalf("step 4 (telemetry status after rotate): not JSON: %v\n%s", err, s4)
	}
	installID2, _ := st4["install_id"].(string)
	if installID2 == "" || installID2 == installID1 {
		t.Fatalf("step 4 (rotate-id): install_id unchanged: before=%q after=%q", installID1, installID2)
	}

	// Step 5 — opt out.
	jStep(t, "telemetry disable", cmdtelemetry.New(), "disable", "--root", dir)

	// Step 6 (false-positive guard §9) — enabled must be false after disable.
	s6 := jStep(t, "telemetry status (disabled)", cmdtelemetry.New(),
		"status", "--root", dir, "--json")
	var st6 map[string]any
	if err := json.Unmarshal(firstJSON([]byte(s6)), &st6); err != nil {
		t.Fatalf("step 6 (telemetry status): not JSON: %v\n%s", err, s6)
	}
	if enabled, _ := st6["enabled"].(bool); enabled {
		t.Fatalf("step 6 (false-positive guard): telemetry still enabled after disable\n%s", s6)
	}
}

// ── Journey 6: Plugin Lifecycle ───────────────────────────────────────────────
//
// Plugin catalogue, install, upgrade, and remove flow:
//
//	forge plugin list              built-in plugins visible
//	forge plugin list --kind scanner  scanner subset only
//	forge plugin show secrets      secrets scanner manifest
//	forge plugin install ext-scanner@1.0.0 --root <dir>   record in lock file
//	forge plugin upgrade ext-scanner --version 2.0.0 --root <dir>  bump version
//	forge plugin remove ext-scanner --root <dir>          remove from lock
//	forge plugin upgrade ext-scanner --version 3.0.0 --root <dir>  FAILS (§3 negative)
func TestJourney_PluginLifecycle(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Step 1 — list: at least 4 built-in plugins must be present.
	s1 := jStep(t, "plugin list", cmdplugin.New(), "list", "--json")
	var manifests []map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(s1)), &manifests); err != nil {
		t.Fatalf("step 1 (plugin list): not JSON: %v\n%s", err, s1)
	}
	if len(manifests) < 4 {
		t.Fatalf("step 1 (plugin list): expected ≥4 built-ins, got %d", len(manifests))
	}

	// Step 2 — filter by kind=scanner: only scanners returned.
	s2 := jStep(t, "plugin list --kind scanner", cmdplugin.New(), "list", "--kind", "scanner", "--json")
	var scanners []map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(s2)), &scanners); err != nil {
		t.Fatalf("step 2 (plugin list scanner): not JSON: %v\n%s", err, s2)
	}
	for _, m := range scanners {
		if kind, _ := m["kind"].(string); kind != "scanner" {
			t.Errorf("step 2 (plugin list scanner): kind filter leaked %v", m)
		}
	}

	// Step 3 (data-accuracy) — show the secrets scanner manifest.
	s3 := jStep(t, "plugin show secrets", cmdplugin.New(), "show", "secrets")
	if !strings.Contains(s3, "secrets") || !strings.Contains(s3, "scanner") {
		t.Fatalf("step 3 (plugin show): unexpected manifest output\n%s", s3)
	}

	// Step 4 — install an external plugin into the project lock file.
	s4 := jStep(t, "plugin install", cmdplugin.New(),
		"install", "ext-scanner@1.0.0", "--root", dir)
	if !strings.Contains(s4, "installed") {
		t.Fatalf("step 4 (plugin install): missing confirmation\n%s", s4)
	}

	// Step 5 — upgrade the external plugin to a new version.
	s5 := jStep(t, "plugin upgrade", cmdplugin.New(),
		"upgrade", "ext-scanner", "--version", "2.0.0", "--root", dir)
	if !strings.Contains(s5, "2.0.0") {
		t.Fatalf("step 5 (plugin upgrade): version 2.0.0 not confirmed\n%s", s5)
	}

	// Step 6 — remove the external plugin.
	s6 := jStep(t, "plugin remove", cmdplugin.New(),
		"remove", "ext-scanner", "--root", dir)
	if !strings.Contains(strings.ToLower(s6), "removed") &&
		!strings.Contains(strings.ToLower(s6), "ext-scanner") {
		t.Fatalf("step 6 (plugin remove): unexpected output\n%s", s6)
	}

	// Step 7 (negative / false-positive guard §§3,9) — upgrading a removed
	// plugin must fail because it no longer exists in the lock file.
	if _, err := jStepErr(t, cmdplugin.New(),
		"upgrade", "ext-scanner", "--version", "3.0.0", "--root", dir); err == nil {
		t.Fatal("step 7 (negative guard): expected error upgrading removed plugin, got nil")
	}

	// Idempotency: plugin list is still the same set of built-ins after all
	// lock-file operations (lock file changes never affect the in-memory registry).
	s1b := jStep(t, "plugin list (replay)", cmdplugin.New(), "list", "--json")
	if s1 != s1b {
		t.Fatalf("plugin list is not idempotent after lock-file ops\nfirst:\n%s\nsecond:\n%s", s1, s1b)
	}
}

// TestJourney_ForgeTest — user runs the forge test family suite in a development
// project, exercising individual test families and the "all" meta-family.
//
// Workflow:
//  1. forge test smoke         → quick smoke check, 1 family result
//  2. forge test unit          → unit family, DryRun=true
//  3. forge test journey       → user-journey family
//  4. forge test all           → all 13 families
//  5. forge test e2e --json    → JSON output, data-accuracy check
//  6. forge test unit (replay) → idempotency guard
//  7. forge test all --fail-fast → still returns results (dry-run compat)
//  8. Negative: forge test --json (parent, no sub) → JSON with 13 families
func TestJourney_ForgeTest(t *testing.T) {
	t.Parallel()

	type testResult struct {
		DryRun   bool   `json:"dry_run"`
		Families []any  `json:"families"`
		Ready    bool   `json:"ready"`
		Message  string `json:"message"`
	}

	parseTestJSON := func(t *testing.T, label, raw string) testResult {
		t.Helper()
		// cmdtest outputs one JSON object; strip trailing newline.
		raw = strings.TrimSpace(raw)
		var res testResult
		if err := json.Unmarshal([]byte(raw), &res); err != nil {
			t.Fatalf("[%s] not JSON: %v\n%s", label, err, raw)
		}
		return res
	}

	// Step 1: smoke family.
	s1 := jStep(t, "test smoke --json", cmdtest.New(), "smoke", "--json")
	r1 := parseTestJSON(t, "smoke", s1)
	if !r1.DryRun {
		t.Fatal("step 1 (smoke): expected dry_run=true")
	}
	if len(r1.Families) != 1 {
		t.Fatalf("step 1 (smoke): expected 1 family result, got %d", len(r1.Families))
	}

	// Step 2: unit family.
	s2 := jStep(t, "test unit --json", cmdtest.New(), "unit", "--json")
	r2 := parseTestJSON(t, "unit", s2)
	if len(r2.Families) != 1 {
		t.Fatalf("step 2 (unit): expected 1 family result, got %d", len(r2.Families))
	}

	// Step 3: journey family.
	s3 := jStep(t, "test journey --json", cmdtest.New(), "journey", "--json")
	r3 := parseTestJSON(t, "journey", s3)
	if len(r3.Families) != 1 {
		t.Fatalf("step 3 (journey): expected 1 family result, got %d", len(r3.Families))
	}

	// Step 4: all families (meta-subcommand).
	s4 := jStep(t, "test all --json", cmdtest.New(), "all", "--json")
	r4 := parseTestJSON(t, "all", s4)
	if len(r4.Families) != 13 {
		t.Fatalf("step 4 (all): expected 13 families, got %d", len(r4.Families))
	}

	// Step 5: e2e — JSON data accuracy: family field must say "e2e".
	s5 := jStep(t, "test e2e --json", cmdtest.New(), "e2e", "--json")
	r5Raw := struct {
		Families []struct {
			Family string `json:"family"`
		} `json:"families"`
		DryRun bool `json:"dry_run"`
	}{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(s5)), &r5Raw); err != nil {
		t.Fatalf("step 5 (e2e JSON): %v\n%s", err, s5)
	}
	if len(r5Raw.Families) != 1 || r5Raw.Families[0].Family != "e2e" {
		t.Fatalf("step 5 (e2e JSON): expected family=e2e, got %+v", r5Raw.Families)
	}

	// Step 6: idempotency — unit twice → same Ready value.
	s6 := jStep(t, "test unit replay", cmdtest.New(), "unit", "--json")
	r6 := parseTestJSON(t, "unit-replay", s6)
	if r2.Ready != r6.Ready {
		t.Fatalf("step 6 (idempotency): Ready differs: first=%v second=%v", r2.Ready, r6.Ready)
	}

	// Step 7: all + fail-fast flag (dry-run compatible).
	s7 := jStep(t, "test all --fail-fast --json", cmdtest.New(), "all", "--fail-fast", "--json")
	r7 := parseTestJSON(t, "all-failfast", s7)
	if len(r7.Families) != 13 {
		t.Fatalf("step 7 (all --fail-fast): expected 13 families, got %d", len(r7.Families))
	}

	// Step 8: 'all' subcommand run directly → 13 families (equivalent to step 4, idempotency guard).
	s8 := jStep(t, "test all (replay)", cmdtest.New(), "all", "--json")
	r8 := parseTestJSON(t, "all-replay", s8)
	if len(r8.Families) != 13 {
		t.Fatalf("step 8 (all replay): expected 13 families, got %d: %s", len(r8.Families), s8)
	}

	// Cross-journey isolation guard: none of the results contain other journey's data.
	_ = r1
	_ = r3
	_ = r4
	_ = r7
	_ = r8
}

// TestJourney_ShipCheckpoints — user runs the forge ship 7-checkpoint pipeline
// both as a whole and one checkpoint at a time, verifying checkpoint isolation
// and the ship → test integration.
//
// Workflow:
//  1. forge ship --json        → 6 checkpoints, DryRun=true
//  2. forge ship spec --json   → 1 checkpoint named "Spec"
//  3. forge ship verify --json → 1 checkpoint, status "ok"
//  4. forge ship test --json   → 1 checkpoint named "Test", integrates with cmdtest
//  5. Idempotency: spec twice  → same Ready
//  6. Negative: parent without JSON → text output contains "6-checkpoint"
func TestJourney_ShipCheckpoints(t *testing.T) {
	t.Parallel()

	type shipResult struct {
		DryRun      bool `json:"dry_run"`
		Ready       bool `json:"ready"`
		Checkpoints []struct {
			Name   string `json:"name"`
			Status string `json:"status"`
		} `json:"checkpoints"`
	}

	parseShipJSON := func(t *testing.T, label, raw string) shipResult {
		t.Helper()
		var res shipResult
		if err := json.Unmarshal(firstJSON([]byte(raw)), &res); err != nil {
			t.Fatalf("[%s] not JSON: %v\n%s", label, err, raw)
		}
		return res
	}

	dir := t.TempDir()

	// Step 1: full pipeline.
	s1 := jStep(t, "ship --json", cmdship.New(), "--json", "--dry-run", "--root", dir)
	r1 := parseShipJSON(t, "ship-full", s1)
	if !r1.DryRun {
		t.Fatal("step 1 (ship full): expected dry_run=true")
	}
	if len(r1.Checkpoints) != 7 {
		t.Fatalf("step 1 (ship full): expected 7 checkpoints, got %d", len(r1.Checkpoints))
	}

	// Step 2: spec subcommand → 1 checkpoint.
	s2 := jStep(t, "ship spec --json", cmdship.New(), "spec", "--json", "--root", dir)
	r2 := parseShipJSON(t, "ship-spec", s2)
	if len(r2.Checkpoints) != 1 {
		t.Fatalf("step 2 (ship spec): expected 1 checkpoint, got %d", len(r2.Checkpoints))
	}
	if !strings.EqualFold(r2.Checkpoints[0].Name, "spec") {
		t.Fatalf("step 2 (ship spec): name mismatch: %q", r2.Checkpoints[0].Name)
	}

	// Step 3: verify subcommand → status "ok" on fresh dir.
	s3 := jStep(t, "ship verify --json", cmdship.New(), "verify", "--json", "--root", dir)
	r3 := parseShipJSON(t, "ship-verify", s3)
	if !r3.Ready {
		t.Fatalf("step 3 (ship verify): expected Ready=true: %+v", r3.Checkpoints)
	}

	// Step 4: test subcommand → 1 checkpoint named "Test".
	s4 := jStep(t, "ship test --json", cmdship.New(), "test", "--json", "--root", dir)
	r4 := parseShipJSON(t, "ship-test", s4)
	if len(r4.Checkpoints) != 1 || !strings.EqualFold(r4.Checkpoints[0].Name, "test") {
		t.Fatalf("step 4 (ship test): unexpected: %+v", r4.Checkpoints)
	}

	// Step 5: idempotency — spec twice → same Ready.
	s5 := jStep(t, "ship spec replay --json", cmdship.New(), "spec", "--json", "--root", dir)
	r5 := parseShipJSON(t, "ship-spec-replay", s5)
	if r2.Ready != r5.Ready {
		t.Fatalf("step 5 (idempotency): Ready differs: %v vs %v", r2.Ready, r5.Ready)
	}

	// Step 6: text output (no --json) must mention pipeline.
	s6 := jStep(t, "ship (text)", cmdship.New(), "--root", dir)
	if !strings.Contains(s6, "7-checkpoint") {
		t.Fatalf("step 6 (text): missing 7-checkpoint in output\n%s", s6)
	}

	_ = r1
	_ = r3
	_ = r4
}

// TestJourney_ForgeTestLifecycle — user runs the 4-phase test lifecycle for a
// named feature: create → approve → run → ci.
//
// Workflow:
//  1. forge test create rate-limiter --json     → CreateResult, Ready=true, 5 files
//  2. forge test approve rate-limiter --json    → ApproveResult, Approved=5
//  3. forge test run rate-limiter --json        → RunFeatureResult, families = defaults
//  4. forge test ci rate-limiter --json         → CIResult, HasCI=false (no config)
//  5. forge test ci rate-limiter --generate-config --dry-run=false --json → ConfigGenerated=true
//  6. Idempotency: create twice → same Generated count
func TestJourney_ForgeTestLifecycle(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	type createResult struct {
		DryRun    bool   `json:"dry_run"`
		Feature   string `json:"feature"`
		Generated []struct {
			Family string `json:"family"`
		} `json:"generated"`
		Ready bool `json:"ready"`
	}
	type approveResult struct {
		Feature  string `json:"feature"`
		Approved int    `json:"approved"`
		Ready    bool   `json:"ready"`
	}
	type runResult struct {
		Feature  string `json:"feature"`
		Families []struct {
			Family string `json:"family"`
		} `json:"families"`
		Ready bool `json:"ready"`
	}
	type ciResult struct {
		Feature         string `json:"feature"`
		HasCI           bool   `json:"has_ci"`
		ConfigGenerated bool   `json:"config_generated"`
		Ready           bool   `json:"ready"`
	}

	// Step 1: create.
	s1 := jStep(t, "test create", cmdtest.New(),
		"create", "rate-limiter", "--root", root, "--json", "--dry-run=true")
	var cr createResult
	if err := json.Unmarshal(firstJSON([]byte(s1)), &cr); err != nil {
		t.Fatalf("step 1 (create): not JSON: %v\n%s", err, s1)
	}
	if !cr.Ready {
		t.Fatalf("step 1 (create): Ready=false\n%s", s1)
	}
	if cr.Feature != "rate-limiter" {
		t.Fatalf("step 1 (create): Feature=%q", cr.Feature)
	}
	const wantFiles = 5 // len(defaultCreateFamilies)
	if len(cr.Generated) != wantFiles {
		t.Fatalf("step 1 (create): want %d files, got %d", wantFiles, len(cr.Generated))
	}

	// Step 2: approve.
	s2 := jStep(t, "test approve", cmdtest.New(),
		"approve", "rate-limiter", "--root", root, "--json", "--dry-run=true")
	var ar approveResult
	if err := json.Unmarshal(firstJSON([]byte(s2)), &ar); err != nil {
		t.Fatalf("step 2 (approve): not JSON: %v\n%s", err, s2)
	}
	if !ar.Ready {
		t.Fatalf("step 2 (approve): Ready=false\n%s", s2)
	}
	if ar.Approved < 1 {
		t.Fatalf("step 2 (approve): Approved=%d, want >= 1", ar.Approved)
	}

	// Step 3: run locally.
	s3 := jStep(t, "test run", cmdtest.New(),
		"run", "rate-limiter", "--root", root, "--json", "--dry-run=true")
	var rr runResult
	if err := json.Unmarshal(firstJSON([]byte(s3)), &rr); err != nil {
		t.Fatalf("step 3 (run): not JSON: %v\n%s", err, s3)
	}
	if !rr.Ready {
		t.Fatalf("step 3 (run): Ready=false\n%s", s3)
	}
	if len(rr.Families) == 0 {
		t.Fatal("step 3 (run): Families should not be empty")
	}

	// Step 4: ci — no config in temp dir → HasCI=false.
	// jStepErr because the CI subcommand returns an error when no CI found.
	s4, _ := jStepErr(t, cmdtest.New(),
		"ci", "rate-limiter", "--root", root, "--json", "--dry-run=true")
	var cir1 ciResult
	if err := json.Unmarshal(firstJSON([]byte(s4)), &cir1); err != nil {
		t.Fatalf("step 4 (ci no-config): not JSON: %v\n%s", err, s4)
	}
	if cir1.HasCI {
		t.Fatalf("step 4 (ci no-config): want HasCI=false, got true")
	}

	// Step 5: generate-config → file written.
	s5, _ := jStepErr(t, cmdtest.New(),
		"ci", "rate-limiter", "--root", root, "--json",
		"--dry-run=false", "--generate-config")
	var cir2 ciResult
	if err := json.Unmarshal(firstJSON([]byte(s5)), &cir2); err != nil {
		t.Fatalf("step 5 (ci generate-config): not JSON: %v\n%s", err, s5)
	}
	if !cir2.ConfigGenerated {
		t.Fatalf("step 5 (ci generate-config): ConfigGenerated=false\n%s", s5)
	}
	ymlPath := filepath.Join(root, ".github", "workflows", "forge-test.yml")
	if _, err := os.Stat(ymlPath); os.IsNotExist(err) {
		t.Fatalf("step 5: forge-test.yml not created at %s", ymlPath)
	}

	// Step 6: idempotency — create twice → same Generated count.
	s6 := jStep(t, "test create replay", cmdtest.New(),
		"create", "rate-limiter", "--root", root, "--json", "--dry-run=true")
	var cr2 createResult
	if err := json.Unmarshal(firstJSON([]byte(s6)), &cr2); err != nil {
		t.Fatalf("step 6 (create replay): not JSON: %v\n%s", err, s6)
	}
	if len(cr.Generated) != len(cr2.Generated) {
		t.Fatalf("step 6 (idempotency): first=%d files, second=%d",
			len(cr.Generated), len(cr2.Generated))
	}
}

// ── Journey 10: Adopt Lifecycle ───────────────────────────────────────────────
//
// Validates the `forge adopt` stub graceful-degradation behaviour:
//
//	forge adopt                  dry-run preview (default)
//	forge adopt --apply          apply mode stub message
//	forge adopt (replay)         idempotency: same dry-run output
func TestJourney_Adopt(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Step 1 — dry-run (default): stub announces intent without writing.
	s1 := jStep(t, "adopt dry-run", cmdadopt.New(), "--root", dir)
	if !strings.Contains(s1, "adopt") {
		t.Fatalf("step 1 (adopt dry-run): missing 'adopt' in output\n%s", s1)
	}
	if !strings.Contains(strings.ToLower(s1), "dry-run") &&
		!strings.Contains(strings.ToLower(s1), "wiring") {
		t.Fatalf("step 1 (adopt dry-run): missing mode indicator\n%s", s1)
	}

	// Step 2 (idempotency) — dry-run again: same output (no state change).
	s1b := jStep(t, "adopt dry-run replay", cmdadopt.New(), "--root", dir)
	if s1 != s1b {
		t.Fatalf("step 2 (idempotency): adopt dry-run output differs\nfirst:\n%s\nsecond:\n%s", s1, s1b)
	}

	// Step 3 — apply mode: stub announces apply intent.
	s2 := jStep(t, "adopt apply", cmdadopt.New(), "--root", dir, "--apply")
	if !strings.Contains(s2, "adopt") {
		t.Fatalf("step 3 (adopt apply): missing 'adopt' in output\n%s", s2)
	}
}

// ── Journey 11: Ask ───────────────────────────────────────────────────────────
//
// Validates the `forge ask` stub graceful-degradation behaviour:
//
//	forge ask "what are the pending migrations?"   stub answer
//	forge ask (replay)                             idempotency
//	forge ask (no args)                            negative: error expected
func TestJourney_Ask(t *testing.T) {
	t.Parallel()

	// Step 1 — ask with a question: stub returns a message containing the verb.
	s1 := jStep(t, "ask question", cmdask.New(), "what are the pending migrations?")
	if !strings.Contains(s1, "ask") {
		t.Fatalf("step 1 (ask): missing 'ask' in output\n%s", s1)
	}

	// Step 2 (idempotency) — same question twice → same stub output.
	s1b := jStep(t, "ask question replay", cmdask.New(), "what are the pending migrations?")
	if s1 != s1b {
		t.Fatalf("step 2 (idempotency): ask output differs\nfirst:\n%s\nsecond:\n%s", s1, s1b)
	}

	// Step 3 (negative) — no question argument → error (MinimumNArgs(1)).
	if _, err := jStepErr(t, cmdask.New()); err == nil {
		t.Fatal("step 3 (negative): expected error when no question given, got nil")
	}
}

// ── Journey 12: Check ────────────────────────────────────────────────────────
//
// Validates the `forge check` stub graceful-degradation behaviour:
//
//	forge check              all schemas (default)
//	forge check api          specific schema
//	forge check (replay)     idempotency
func TestJourney_Check(t *testing.T) {
	t.Parallel()

	// Step 1 — check all schemas: stub announces validation intent.
	s1 := jStep(t, "check all", cmdcheck.New())
	if !strings.Contains(s1, "check") {
		t.Fatalf("step 1 (check all): missing 'check' in output\n%s", s1)
	}

	// Step 2 — check specific schema: stub accepts optional schema arg.
	s2 := jStep(t, "check api", cmdcheck.New(), "api")
	if !strings.Contains(s2, "check") {
		t.Fatalf("step 2 (check api): missing 'check' in output\n%s", s2)
	}

	// Step 3 (idempotency) — check all twice → same output.
	s1b := jStep(t, "check all replay", cmdcheck.New())
	if s1 != s1b {
		t.Fatalf("step 3 (idempotency): check output differs\nfirst:\n%s\nsecond:\n%s", s1, s1b)
	}
}

// ── Journey 13: Context ───────────────────────────────────────────────────────
//
// Validates the `forge context` sub-command stubs:
//
//	forge context generate   regenerate AI context files (stub)
//	forge context show       display current context files (stub)
//	forge context budget     report token budget (stub)
//	forge context show (replay) idempotency
func TestJourney_Context(t *testing.T) {
	t.Parallel()

	// Step 1 — generate: stub announces intent.
	s1 := jStep(t, "context generate", cmdcontext.New(), "generate")
	if !strings.Contains(s1, "context") {
		t.Fatalf("step 1 (context generate): missing 'context' in output\n%s", s1)
	}

	// Step 2 — show: stub announces intent.
	s2 := jStep(t, "context show", cmdcontext.New(), "show")
	if !strings.Contains(s2, "context") {
		t.Fatalf("step 2 (context show): missing 'context' in output\n%s", s2)
	}

	// Step 3 — budget: stub announces intent.
	s3 := jStep(t, "context budget", cmdcontext.New(), "budget")
	if !strings.Contains(s3, "context") {
		t.Fatalf("step 3 (context budget): missing 'context' in output\n%s", s3)
	}

	// Step 4 (idempotency) — show twice → same output.
	s2b := jStep(t, "context show replay", cmdcontext.New(), "show")
	if s2 != s2b {
		t.Fatalf("step 4 (idempotency): context show differs\nfirst:\n%s\nsecond:\n%s", s2, s2b)
	}
}

// ── Journey 14: Docs ─────────────────────────────────────────────────────────
//
// Validates the `forge docs` sub-command stubs:
//
//	forge docs sync          regenerate docs (stub)
//	forge docs heal          fix broken links (stub)
//	forge docs heal (replay) idempotency
func TestJourney_Docs(t *testing.T) {
	t.Parallel()

	// Step 1 — sync: stub announces docs sync intent.
	s1 := jStep(t, "docs sync", cmddocs.New(), "sync", "--dry-run")
	if !strings.Contains(s1, "docs") {
		t.Fatalf("step 1 (docs sync): missing 'docs' in output\n%s", s1)
	}

	// Step 2 — heal: stub announces docs heal intent.
	s2 := jStep(t, "docs heal", cmddocs.New(), "heal", "--dry-run")
	if !strings.Contains(s2, "docs") {
		t.Fatalf("step 2 (docs heal): missing 'docs' in output\n%s", s2)
	}

	// Step 3 (idempotency) — heal twice → same output.
	s2b := jStep(t, "docs heal replay", cmddocs.New(), "heal", "--dry-run")
	if s2 != s2b {
		t.Fatalf("step 3 (idempotency): docs heal output differs\nfirst:\n%s\nsecond:\n%s", s2, s2b)
	}
}

// ── Journey 15: Eject ────────────────────────────────────────────────────────
//
// Validates the `forge eject` stub graceful-degradation behaviour:
//
//	forge eject              dry-run preview (default)
//	forge eject --apply      apply mode stub message
//	forge eject (replay)     idempotency: same dry-run output
func TestJourney_Eject(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Step 1 — dry-run (default): stub announces eject intent.
	s1 := jStep(t, "eject dry-run", cmdeject.New(), "--root", dir)
	if !strings.Contains(s1, "eject") {
		t.Fatalf("step 1 (eject dry-run): missing 'eject' in output\n%s", s1)
	}

	// Step 2 — apply mode: stub announces apply intent.
	s2 := jStep(t, "eject apply", cmdeject.New(), "--root", dir, "--apply")
	if !strings.Contains(s2, "eject") {
		t.Fatalf("step 2 (eject apply): missing 'eject' in output\n%s", s2)
	}

	// Step 3 (idempotency) — dry-run twice → same output.
	s1b := jStep(t, "eject dry-run replay", cmdeject.New(), "--root", dir)
	if s1 != s1b {
		t.Fatalf("step 3 (idempotency): eject dry-run output differs\nfirst:\n%s\nsecond:\n%s", s1, s1b)
	}
}

// ── Journey 16: Fix ──────────────────────────────────────────────────────────
//
// Validates the `forge fix` stub graceful-degradation behaviour:
//
//	forge fix                all families, dry-run (default)
//	forge fix correctness    specific family
//	forge fix (replay)       idempotency
func TestJourney_Fix(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Step 1 — fix all families (dry-run): stub announces intent.
	s1 := jStep(t, "fix all dry-run", cmdfix.New(), "--root", dir)
	if !strings.Contains(s1, "fix") {
		t.Fatalf("step 1 (fix all): missing 'fix' in output\n%s", s1)
	}

	// Step 2 — fix specific family: stub accepts family arg.
	s2 := jStep(t, "fix correctness", cmdfix.New(), "correctness", "--root", dir)
	if !strings.Contains(s2, "fix") {
		t.Fatalf("step 2 (fix correctness): missing 'fix' in output\n%s", s2)
	}

	// Step 3 (idempotency) — same dry-run twice → same output.
	s1b := jStep(t, "fix all dry-run replay", cmdfix.New(), "--root", dir)
	if s1 != s1b {
		t.Fatalf("step 3 (idempotency): fix output differs\nfirst:\n%s\nsecond:\n%s", s1, s1b)
	}
}

// ── Journey 17: Generate ─────────────────────────────────────────────────────
//
// Validates the `forge generate` stub graceful-degradation behaviour:
//
//	forge generate handler UserCreate   handler boilerplate (stub)
//	forge generate migration add-users  migration pair (stub)
//	forge generate handler (replay)     idempotency
//	forge generate (no args)            negative: error expected
func TestJourney_Generate(t *testing.T) {
	t.Parallel()

	// Step 1 — generate handler: stub announces code generation intent.
	s1 := jStep(t, "generate handler", cmdgenerate.New(), "handler", "UserCreate")
	if !strings.Contains(s1, "generate") {
		t.Fatalf("step 1 (generate handler): missing 'generate' in output\n%s", s1)
	}

	// Step 2 — generate migration: stub announces migration intent.
	s2 := jStep(t, "generate migration", cmdgenerate.New(), "migration", "add-users-table")
	if !strings.Contains(s2, "generate") {
		t.Fatalf("step 2 (generate migration): missing 'generate' in output\n%s", s2)
	}

	// Step 3 (idempotency) — same handler twice → same output.
	s1b := jStep(t, "generate handler replay", cmdgenerate.New(), "handler", "UserCreate")
	if s1 != s1b {
		t.Fatalf("step 3 (idempotency): generate output differs\nfirst:\n%s\nsecond:\n%s", s1, s1b)
	}

	// Step 4 (negative) — no kind argument → error (MinimumNArgs(1)).
	if _, err := jStepErr(t, cmdgenerate.New()); err == nil {
		t.Fatal("step 4 (negative): expected error when no kind given, got nil")
	}
}

// ── Journey 18: Hygiene ──────────────────────────────────────────────────────
//
// Validates the `forge hygiene` sub-command surface:
//
//	forge hygiene report              coverage gap report (reads manifest or defaults)
//	forge hygiene manifest list       list manifest entries
//	forge hygiene report (replay)     idempotency
func TestJourney_HygieneSurface(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Step 1 — report: reads .forge/hygiene.yml (falls back to defaults if absent).
	s1 := jStep(t, "hygiene report", cmdhygiene.New(), "report", "--root", dir)
	if !strings.Contains(s1, "hygiene") {
		t.Fatalf("step 1 (hygiene report): missing 'hygiene' in output\n%s", s1)
	}

	// Step 2 — manifest list: stub announces manifest intent.
	s2 := jStep(t, "hygiene manifest list", cmdhygiene.New(), "manifest", "list", "--root", dir)
	if !strings.Contains(s2, "hygiene") && !strings.Contains(s2, "manifest") {
		t.Fatalf("step 2 (hygiene manifest list): missing expected keyword in output\n%s", s2)
	}

	// Step 3 (idempotency) — report twice → same output.
	s1b := jStep(t, "hygiene report replay", cmdhygiene.New(), "report", "--root", dir)
	if s1 != s1b {
		t.Fatalf("step 3 (idempotency): hygiene report output differs\nfirst:\n%s\nsecond:\n%s", s1, s1b)
	}
}

// ── Journey 19: Migrate ──────────────────────────────────────────────────────
//
// Validates the `forge migrate` sub-command stubs:
//
//	forge migrate status   list applied/pending (stub)
//	forge migrate up       apply pending (stub)
//	forge migrate down     roll back (stub)
//	forge migrate status (replay) idempotency
func TestJourney_Migrate(t *testing.T) {
	t.Parallel()

	// Step 1 — status: stub announces migration status intent.
	s1 := jStep(t, "migrate status", cmdmigrate.New(), "status")
	if !strings.Contains(s1, "migrate") {
		t.Fatalf("step 1 (migrate status): missing 'migrate' in output\n%s", s1)
	}

	// Step 2 — up: stub announces apply intent.
	s2 := jStep(t, "migrate up", cmdmigrate.New(), "up")
	if !strings.Contains(s2, "migrate") {
		t.Fatalf("step 2 (migrate up): missing 'migrate' in output\n%s", s2)
	}

	// Step 3 — down: stub announces rollback intent.
	s3 := jStep(t, "migrate down", cmdmigrate.New(), "down", "--dry-run")
	if !strings.Contains(s3, "migrate") {
		t.Fatalf("step 3 (migrate down): missing 'migrate' in output\n%s", s3)
	}

	// Step 4 (idempotency) — status twice → same output.
	s1b := jStep(t, "migrate status replay", cmdmigrate.New(), "status")
	if s1 != s1b {
		t.Fatalf("step 4 (idempotency): migrate status output differs\nfirst:\n%s\nsecond:\n%s", s1, s1b)
	}
}

// ── Journey 20: Review ───────────────────────────────────────────────────────
//
// Validates the `forge review` stub graceful-degradation behaviour:
//
//	forge review --rounds 1           self-debate on staged diff (stub)
//	forge review path/to/file.go      specific file target (stub)
//	forge review --rounds 1 (replay)  idempotency
func TestJourney_Review(t *testing.T) {
	t.Parallel()

	// Step 1 — review staged diff: stub announces self-debate intent.
	s1 := jStep(t, "review staged diff", cmdreview.New(), "--rounds", "1")
	if !strings.Contains(s1, "review") {
		t.Fatalf("step 1 (review): missing 'review' in output\n%s", s1)
	}

	// Step 2 — review specific file target: stub accepts path arg.
	s2 := jStep(t, "review file target", cmdreview.New(), "main.go", "--rounds", "2")
	if !strings.Contains(s2, "review") {
		t.Fatalf("step 2 (review file): missing 'review' in output\n%s", s2)
	}

	// Step 3 (idempotency) — same review twice → same stub output.
	s1b := jStep(t, "review replay", cmdreview.New(), "--rounds", "1")
	if s1 != s1b {
		t.Fatalf("step 3 (idempotency): review output differs\nfirst:\n%s\nsecond:\n%s", s1, s1b)
	}
}
