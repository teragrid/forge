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

package cmdship

// ── Test design (always-write-tests.md) ───────────────────────────────────────
//
// 1. Happy path
//    - scanFeatures: empty specsDir → empty slice, no error
//    - scanFeatures: one done feature → Status=done, CheckpointsDone=7
//    - scanFeatures: one active feature (4/7) → Status=active
//    - buildEntry: no manifest, no checkpoints → Status=unknown
//    - buildEntry: manifest present, no checkpoints → Status=draft
//    - buildEntry: all 7 checkpoints present → Status=done (even no manifest)
//    - countCheckpoints: all 7 files → 7
//    - countCheckpoints: zero files → 0
//    - renderStatusTable: writes header + one row per entry
//    - newStatusCmd: no args, empty specs dir → prints "no features found"
//    - newStatusCmd: --done filter, no done features → "no features with status"
//    - newStatusCmd: slug arg, feature exists → detail view
//    - newStatusCmd: slug arg, missing feature → "not found"
//    - newStatusCmd: --json flag → valid JSON array
//    - newStatusCmd: --done flag → only done features shown
//    - newStatusCmd: --status active filter → only active features
//
// 2. Boundary cases
//    - scanFeatures: specsDir does not exist → nil, nil (not an error)
//    - buildEntry: feature name > 40 chars → truncated in table render
//    - createdAt > 10 chars → only date portion shown
//    - renderStatusDetail: no manifest → no "feature:" / "created:" lines
//    - countCheckpoints: partial (3/7 files) → 3
//
// 3. Negative cases
//    - newStatusCmd: unknown --status value → no results, helpful message
//    - newStatusCmd --done and --status active together → --done wins (filter=done)
//
// 4. Regression guard
//    - checkpointFiles and checkpointNames slices have equal length
//    - statusIcon returns non-empty string for all four lifecycle stages

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ── Slice invariant ───────────────────────────────────────────────────────────

func TestCheckpointSlicesEqualLength(t *testing.T) {
	if len(checkpointFiles) != len(checkpointNames) {
		t.Errorf("checkpointFiles (%d) and checkpointNames (%d) must have equal length",
			len(checkpointFiles), len(checkpointNames))
	}
}

// ── statusIcon ────────────────────────────────────────────────────────────────

func TestStatusIcon_AllStages(t *testing.T) {
	for _, stage := range []string{"done", "active", "draft", "unknown", "other"} {
		if statusIcon(stage) == "" {
			t.Errorf("statusIcon(%q) returned empty string", stage)
		}
	}
}

// ── countCheckpoints ─────────────────────────────────────────────────────────

func TestCountCheckpoints_Zero(t *testing.T) {
	dir := t.TempDir()
	if n := countCheckpoints(dir); n != 0 {
		t.Errorf("expected 0, got %d", n)
	}
}

func TestCountCheckpoints_All(t *testing.T) {
	dir := t.TempDir()
	for _, f := range checkpointFiles {
		writeFile(t, dir, f, "content")
	}
	if n := countCheckpoints(dir); n != len(checkpointFiles) {
		t.Errorf("expected %d, got %d", len(checkpointFiles), n)
	}
}

func TestCountCheckpoints_Partial(t *testing.T) {
	dir := t.TempDir()
	for _, f := range checkpointFiles[:3] {
		writeFile(t, dir, f, "x")
	}
	if n := countCheckpoints(dir); n != 3 {
		t.Errorf("expected 3, got %d", n)
	}
}

// ── buildEntry ────────────────────────────────────────────────────────────────

func TestBuildEntry_NoManifestNoCheckpoints_Unknown(t *testing.T) {
	root := t.TempDir()
	specsDir := filepath.Join(root, ".forge", "specs")
	mustMkdir(t, filepath.Join(specsDir, "my-feature"))

	e := buildEntry(specsDir, "my-feature")
	if e.Status != "unknown" {
		t.Errorf("expected 'unknown', got %q", e.Status)
	}
	if e.CheckpointsDone != 0 {
		t.Errorf("expected 0 checkpoints done, got %d", e.CheckpointsDone)
	}
	if e.Feature != "my-feature" {
		t.Errorf("expected feature fallback to slug, got %q", e.Feature)
	}
}

func TestBuildEntry_ManifestPresentNoCheckpoints_Draft(t *testing.T) {
	root := t.TempDir()
	specsDir := filepath.Join(root, ".forge", "specs")
	slug := "rate-limiter"
	featureDir := filepath.Join(specsDir, slug)
	mustMkdir(t, featureDir)
	writeManifest(t, featureDir, "Rate limiter", "draft", "2024-01-15T00:00:00Z")

	e := buildEntry(specsDir, slug)
	if e.Status != "draft" {
		t.Errorf("expected 'draft', got %q", e.Status)
	}
	if e.Feature != "Rate limiter" {
		t.Errorf("expected feature name from manifest, got %q", e.Feature)
	}
	if e.CreatedAt != "2024-01-15T00:00:00Z" {
		t.Errorf("unexpected CreatedAt %q", e.CreatedAt)
	}
}

func TestBuildEntry_AllCheckpoints_Done(t *testing.T) {
	root := t.TempDir()
	specsDir := filepath.Join(root, ".forge", "specs")
	slug := "login"
	featureDir := filepath.Join(specsDir, slug)
	mustMkdir(t, featureDir)
	for _, f := range checkpointFiles {
		writeFile(t, featureDir, f, "done")
	}

	e := buildEntry(specsDir, slug)
	if e.Status != "done" {
		t.Errorf("expected 'done', got %q", e.Status)
	}
	if e.CheckpointsDone != 7 {
		t.Errorf("expected 7 checkpoints done, got %d", e.CheckpointsDone)
	}
}

func TestBuildEntry_SomeCheckpoints_Active(t *testing.T) {
	root := t.TempDir()
	specsDir := filepath.Join(root, ".forge", "specs")
	slug := "checkout"
	featureDir := filepath.Join(specsDir, slug)
	mustMkdir(t, featureDir)
	writeManifest(t, featureDir, "Checkout flow", "active", "")
	for _, f := range checkpointFiles[:4] {
		writeFile(t, featureDir, f, "x")
	}

	e := buildEntry(specsDir, slug)
	if e.Status != "active" {
		t.Errorf("expected 'active', got %q", e.Status)
	}
	if e.CheckpointsDone != 4 {
		t.Errorf("expected 4, got %d", e.CheckpointsDone)
	}
}

func TestBuildEntry_ManifestStatusDone_OverridesCheckpointCount(t *testing.T) {
	// If manifest says "done", trust it even without checkpoint files.
	root := t.TempDir()
	specsDir := filepath.Join(root, ".forge", "specs")
	slug := "old-feature"
	featureDir := filepath.Join(specsDir, slug)
	mustMkdir(t, featureDir)
	writeManifest(t, featureDir, "Old feature", "done", "2023-06-01T00:00:00Z")
	// No checkpoint .md files.

	e := buildEntry(specsDir, slug)
	if e.Status != "done" {
		t.Errorf("expected manifest 'done' to win, got %q", e.Status)
	}
}

// ── scanFeatures ─────────────────────────────────────────────────────────────

func TestScanFeatures_NonExistentDir_NilNil(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "no-such-dir")
	entries, err := scanFeatures(dir)
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if entries != nil {
		t.Errorf("expected nil entries, got %v", entries)
	}
}

func TestScanFeatures_EmptyDir_EmptySlice(t *testing.T) {
	dir := t.TempDir()
	entries, err := scanFeatures(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected empty, got %d entries", len(entries))
	}
}

func TestScanFeatures_SkipsFiles_OnlyDirs(t *testing.T) {
	dir := t.TempDir()
	mustMkdir(t, filepath.Join(dir, "feature-a"))
	mustMkdir(t, filepath.Join(dir, "feature-b"))
	// a plain file — should be skipped
	writeFile(t, dir, "README.md", "ignored")

	entries, err := scanFeatures(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(entries))
	}
}

func TestScanFeatures_MultipleStatuses(t *testing.T) {
	specsDir := t.TempDir()

	// done feature — all 7 checkpoints
	doneDir := filepath.Join(specsDir, "auth")
	mustMkdir(t, doneDir)
	for _, f := range checkpointFiles {
		writeFile(t, doneDir, f, "done")
	}

	// active feature — 3/7 checkpoints
	activeDir := filepath.Join(specsDir, "billing")
	mustMkdir(t, activeDir)
	writeManifest(t, activeDir, "Billing", "active", "")
	for _, f := range checkpointFiles[:3] {
		writeFile(t, activeDir, f, "x")
	}

	// draft feature — manifest only, no checkpoints
	draftDir := filepath.Join(specsDir, "notifications")
	mustMkdir(t, draftDir)
	writeManifest(t, draftDir, "Notifications", "draft", "2024-03-01T00:00:00Z")

	entries, err := scanFeatures(specsDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	statusMap := make(map[string]string)
	for _, e := range entries {
		statusMap[e.Slug] = e.Status
	}
	if statusMap["auth"] != "done" {
		t.Errorf("auth: expected done, got %q", statusMap["auth"])
	}
	if statusMap["billing"] != "active" {
		t.Errorf("billing: expected active, got %q", statusMap["billing"])
	}
	if statusMap["notifications"] != "draft" {
		t.Errorf("notifications: expected draft, got %q", statusMap["notifications"])
	}
}

// ── renderStatusTable ─────────────────────────────────────────────────────────

func TestRenderStatusTable_ContainsHeaders(t *testing.T) {
	var buf bytes.Buffer
	entries := []FeatureStatusEntry{
		{Slug: "login", Feature: "Login feature", Status: "done", CheckpointsDone: 7, CheckpointsTotal: 7},
	}
	renderStatusTable(&buf, entries)
	out := buf.String()
	for _, col := range []string{"SLUG", "FEATURE", "STATUS", "PROGRESS", "CREATED"} {
		if !strings.Contains(out, col) {
			t.Errorf("expected header %q in table output", col)
		}
	}
}

func TestRenderStatusTable_RowContainsData(t *testing.T) {
	var buf bytes.Buffer
	entries := []FeatureStatusEntry{
		{Slug: "rate-limiter", Feature: "Rate limiter", Status: "active",
			CheckpointsDone: 4, CheckpointsTotal: 7, CreatedAt: "2024-01-20T10:00:00Z"},
	}
	renderStatusTable(&buf, entries)
	out := buf.String()
	if !strings.Contains(out, "rate-limiter") {
		t.Error("expected slug in output")
	}
	if !strings.Contains(out, "4/7") {
		t.Error("expected progress 4/7 in output")
	}
	if !strings.Contains(out, "2024-01-20") {
		t.Error("expected date in output")
	}
}

// ── renderStatusDetail ────────────────────────────────────────────────────────

func TestRenderStatusDetail_ShowsCheckpoints(t *testing.T) {
	featureDir := t.TempDir()
	writeFile(t, featureDir, "spec.md", "x")
	writeFile(t, featureDir, "arch.md", "x")

	entry := FeatureStatusEntry{Slug: "auth", Feature: "Auth", Status: "active",
		CheckpointsDone: 2, CheckpointsTotal: 7}
	var buf bytes.Buffer
	renderStatusDetail(&buf, featureDir, entry)
	out := buf.String()

	if !strings.Contains(out, "✓ done") {
		t.Error("expected '✓ done' for completed checkpoint")
	}
	if !strings.Contains(out, "○ pending") {
		t.Error("expected '○ pending' for incomplete checkpoints")
	}
	if !strings.Contains(out, "spec") {
		t.Error("expected 'spec' checkpoint label")
	}
}

// ── newStatusCmd (cobra) ──────────────────────────────────────────────────────

func TestNewStatusCmd_NoArgs_EmptySpecs_Message(t *testing.T) {
	root := t.TempDir()
	cmd := newStatusCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--root", root})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "no features found") {
		t.Errorf("expected 'no features found', got: %q", buf.String())
	}
}

func TestNewStatusCmd_NoArgs_ListsAll(t *testing.T) {
	root := t.TempDir()
	specsDir := filepath.Join(root, ".forge", "specs")
	// Create 2 features.
	for _, slug := range []string{"login", "checkout"} {
		d := filepath.Join(specsDir, slug)
		mustMkdir(t, d)
		featureName := slug[:1] + slug[1:] // preserve slug as-is; Title is deprecated
		writeManifest(t, d, featureName, "draft", "")
	}

	cmd := newStatusCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--root", root})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "login") {
		t.Error("expected 'login' in output")
	}
	if !strings.Contains(out, "checkout") {
		t.Error("expected 'checkout' in output")
	}
	if !strings.Contains(out, "2 features") {
		t.Errorf("expected '2 features' in summary, got: %q", out)
	}
}

func TestNewStatusCmd_DoneFlag_OnlyDoneFeatures(t *testing.T) {
	root := t.TempDir()
	specsDir := filepath.Join(root, ".forge", "specs")

	// done feature
	doneDir := filepath.Join(specsDir, "auth")
	mustMkdir(t, doneDir)
	for _, f := range checkpointFiles {
		writeFile(t, doneDir, f, "done")
	}
	// active feature
	activeDir := filepath.Join(specsDir, "billing")
	mustMkdir(t, activeDir)
	writeManifest(t, activeDir, "Billing", "active", "")
	writeFile(t, activeDir, "spec.md", "x")

	cmd := newStatusCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--root", root, "--done"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "auth") {
		t.Error("expected 'auth' (done feature) in output")
	}
	if strings.Contains(out, "billing") {
		t.Error("unexpected 'billing' (active feature) in --done output")
	}
}

func TestNewStatusCmd_StatusActiveFilter(t *testing.T) {
	root := t.TempDir()
	specsDir := filepath.Join(root, ".forge", "specs")

	// draft feature
	draftDir := filepath.Join(specsDir, "notifications")
	mustMkdir(t, draftDir)
	writeManifest(t, draftDir, "Notifications", "draft", "")

	// active feature
	activeDir := filepath.Join(specsDir, "billing")
	mustMkdir(t, activeDir)
	writeManifest(t, activeDir, "Billing", "active", "")
	writeFile(t, activeDir, "spec.md", "x")

	cmd := newStatusCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--root", root, "--status", "active"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if strings.Contains(out, "notifications") {
		t.Error("unexpected draft feature in --status active output")
	}
	if !strings.Contains(out, "billing") {
		t.Error("expected active feature 'billing' in output")
	}
}

func TestNewStatusCmd_UnknownStatusFilter_NoResults(t *testing.T) {
	root := t.TempDir()
	specsDir := filepath.Join(root, ".forge", "specs")
	d := filepath.Join(specsDir, "login")
	mustMkdir(t, d)
	writeManifest(t, d, "Login", "draft", "")

	cmd := newStatusCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--root", root, "--status", "nonexistent"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "no features with status") {
		t.Errorf("expected no-results message, got: %q", buf.String())
	}
}

func TestNewStatusCmd_SlugArg_DetailView(t *testing.T) {
	root := t.TempDir()
	specsDir := filepath.Join(root, ".forge", "specs")
	slug := "login"
	d := filepath.Join(specsDir, slug)
	mustMkdir(t, d)
	writeManifest(t, d, "Login via OAuth", "active", "2024-02-01T00:00:00Z")
	writeFile(t, d, "spec.md", "x")
	writeFile(t, d, "arch.md", "x")

	cmd := newStatusCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--root", root, slug})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "login") {
		t.Error("expected slug in detail output")
	}
	if !strings.Contains(out, "✓ done") {
		t.Error("expected done checkpoints in detail output")
	}
	if !strings.Contains(out, "○ pending") {
		t.Error("expected pending checkpoints in detail output")
	}
}

func TestNewStatusCmd_SlugArg_NotFound(t *testing.T) {
	root := t.TempDir()
	cmd := newStatusCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--root", root, "does-not-exist"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "not found") {
		t.Errorf("expected 'not found' message, got: %q", buf.String())
	}
}

func TestNewStatusCmd_JSONFlag_ValidArray(t *testing.T) {
	root := t.TempDir()
	specsDir := filepath.Join(root, ".forge", "specs")
	d := filepath.Join(specsDir, "auth")
	mustMkdir(t, d)
	for _, f := range checkpointFiles {
		writeFile(t, d, f, "done")
	}

	cmd := newStatusCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--root", root, "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var entries []FeatureStatusEntry
	if err := json.Unmarshal(buf.Bytes(), &entries); err != nil {
		t.Fatalf("JSON output is not valid: %v\nOutput: %s", err, buf.String())
	}
	if len(entries) != 1 || entries[0].Status != "done" {
		t.Errorf("unexpected JSON result: %+v", entries)
	}
}

func TestNewStatusCmd_JSONFlag_SingleFeature(t *testing.T) {
	root := t.TempDir()
	specsDir := filepath.Join(root, ".forge", "specs")
	slug := "auth"
	d := filepath.Join(specsDir, slug)
	mustMkdir(t, d)
	writeManifest(t, d, "Auth", "draft", "2024-01-01T00:00:00Z")

	cmd := newStatusCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--root", root, "--json", slug})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var entry FeatureStatusEntry
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("JSON output for single feature is not valid: %v\nOutput: %s", err, buf.String())
	}
	if entry.Slug != slug {
		t.Errorf("expected slug %q, got %q", slug, entry.Slug)
	}
}

// ── DoneFlag wins over StatusFlag ─────────────────────────────────────────────

func TestNewStatusCmd_DoneFlagBeatsStatusFlag(t *testing.T) {
	// --done + --status active should result in done filter winning (--done shorthand).
	// The implementation: onlyDone=true overrides filterStatus.
	root := t.TempDir()
	specsDir := filepath.Join(root, ".forge", "specs")

	doneDir := filepath.Join(specsDir, "done-feat")
	mustMkdir(t, doneDir)
	for _, f := range checkpointFiles {
		writeFile(t, doneDir, f, "done")
	}

	cmd := newStatusCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--root", root, "--done", "--status", "active"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	// done-feat is done, not active — if --done wins, it appears; if --status active wins, it's absent.
	if !strings.Contains(out, "done-feat") {
		t.Error("expected --done to win over --status active")
	}
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mustMkdir %q: %v", path, err)
	}
}

// writeManifest writes a minimal spec.yml to dir that loadSpecManifest can read.
func writeManifest(t *testing.T, dir, feature, status, createdAt string) {
	t.Helper()
	content := "schema_version: 1\n"
	content += "id: " + filepath.Base(dir) + "\n"
	if feature != "" {
		content += "feature: " + feature + "\n"
	}
	if status != "" {
		content += "status: " + status + "\n"
	}
	if createdAt != "" {
		content += "created_at: " + createdAt + "\n"
	}
	// loadSpecManifest reads from specsDir/<slug>/spec.yml
	// but here dir IS already specsDir/<slug>.
	writeFile(t, dir, "spec.yml", content)
}
