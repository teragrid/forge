package cmdaudit

// Additional tests to push coverage above 85%.
// These cover the JSON paths, text branches, and error branches
// not exercised by audit_test.go.

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/teragrid/forge/internal/failure"
)

// --- audit.go coverage ---

// TC-CMDAUDIT-06 (happy, JSON): show --json emits valid JSON array.
func TestAudit_ShowJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Append one entry first.
	app := New()
	app.SetOut(new(bytes.Buffer))
	app.SetArgs([]string{"append", "--root", dir, "--verb", "deploy", "--action", "prod"})
	if err := app.Execute(); err != nil {
		t.Fatalf("append: %v", err)
	}

	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"show", "--root", dir, "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("show --json: %v", err)
	}
	var entries []map[string]any
	if err := json.Unmarshal(out.Bytes(), &entries); err != nil {
		t.Fatalf("not JSON: %v — output: %s", err, out.String())
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0]["verb"] != "deploy" {
		t.Errorf("expected verb=deploy, got %v", entries[0]["verb"])
	}
}

// TC-CMDAUDIT-07 (happy, JSON): append --json emits the appended entry.
func TestAudit_AppendJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"append", "--root", dir, "--verb", "ship", "--action", "deploy", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("append --json: %v", err)
	}
	var entry map[string]any
	if err := json.Unmarshal(out.Bytes(), &entry); err != nil {
		t.Fatalf("not JSON: %v — output: %s", err, out.String())
	}
	if entry["verb"] != "ship" {
		t.Errorf("expected verb=ship, got %v", entry["verb"])
	}
}

// TC-CMDAUDIT-08 (happy): verify text mode on intact chain prints "intact".
func TestAudit_VerifyText_Intact(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	app := New()
	app.SetOut(new(bytes.Buffer))
	app.SetArgs([]string{"append", "--root", dir, "--verb", "v", "--action", "a"})
	if err := app.Execute(); err != nil {
		t.Fatalf("append: %v", err)
	}

	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"verify", "--root", dir})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("verify text intact: %v", err)
	}
	if !strings.Contains(out.String(), "intact") {
		t.Errorf("expected 'intact' in output, got: %s", out.String())
	}
}

// TC-CMDAUDIT-09 (negative): verify text mode on tampered ledger → FORGE-3401.
func TestAudit_VerifyText_Broken(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	ledgerPath := filepath.Join(dir, ".forge", "audit.log")
	if err := os.MkdirAll(filepath.Dir(ledgerPath), 0o755); err != nil {
		t.Fatal(err)
	}
	// Write a syntactically valid JSONL entry but with a wrong hash.
	corrupt := `{"ts":"2024-01-01T00:00:00Z","verb":"v","action":"a","prev_hash":"","hash":"000000000000000000000000000000000000000000000000000000000000dead"}` + "\n"
	if err := os.WriteFile(ledgerPath, []byte(corrupt), 0o600); err != nil {
		t.Fatal(err)
	}

	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"verify", "--root", dir})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected tamper error")
	}
	if !strings.Contains(err.Error(), "FORGE-3401") {
		t.Errorf("expected FORGE-3401, got: %v", err)
	}
	if !strings.Contains(out.String(), "BROKEN") {
		t.Errorf("expected 'BROKEN' in output, got: %s", out.String())
	}
}

// TC-CMDAUDIT-10 (boundary): short() returns unchanged string when len ≤ 12.
func TestAudit_Short_SmallHash(t *testing.T) {
	const h = "abc"
	if got := short(h); got != h {
		t.Errorf("short(%q) = %q, want %q", h, got, h)
	}
}

// --- failure_register.go coverage ---

// TC-CMDAUDIT-FR-06 (happy, JSON): lint --json valid schema → {"ok":true}.
func TestAudit_FailureRegisterLint_JSON_OK(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	reg := failure.New()
	reg.Entries = []failure.Entry{
		{ID: "FR-001", Component: "c", FailureMode: "m", TestAnchor: "T-01", Status: failure.StatusTracked},
	}
	seedRegister(t, dir, reg)

	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"failure-register", "lint", "--root", dir, "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("lint --json: %v\noutput: %s", err, out.String())
	}
	var result map[string]any
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("not JSON: %v — output: %s", err, out.String())
	}
	if result["ok"] != true {
		t.Errorf("expected ok=true, got: %v", result)
	}
}

// TC-CMDAUDIT-FR-07 (negative, JSON): lint --json invalid schema → {"ok":false,"error":"..."}.
func TestAudit_FailureRegisterLint_JSON_Invalid(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// Kind="Bad" passes JSON parsing but fails Register.Validate().
	writeRawRegister(t, dir, `{"api_version":"forge.sh/v1","kind":"Bad","generated_at":"2024-01-01T00:00:00Z","entries":[]}`)

	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"failure-register", "lint", "--root", dir, "--json"})
	// In JSON mode, validation errors are encoded in the payload; command exits 0.
	_ = cmd.Execute()
	var result map[string]any
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("not JSON: %v — output: %s", err, out.String())
	}
	if result["ok"] != false {
		t.Errorf("expected ok=false, got: %v", result)
	}
	if result["error"] == "" || result["error"] == nil {
		t.Errorf("expected non-empty error field, got: %v", result)
	}
}

// TC-CMDAUDIT-FR-08 (negative, text): lint text mode invalid schema → FORGE-3701.
func TestAudit_FailureRegisterLint_Text_Invalid(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeRawRegister(t, dir, `{"api_version":"forge.sh/v1","kind":"Bad","generated_at":"2024-01-01T00:00:00Z","entries":[]}`)

	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"failure-register", "lint", "--root", dir})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error from lint on invalid schema")
	}
	if !strings.Contains(err.Error(), "FORGE-3701") {
		t.Errorf("expected FORGE-3701, got: %v", err)
	}
	if !strings.Contains(out.String(), "INVALID") {
		t.Errorf("expected INVALID in output, got: %s", out.String())
	}
}

// TC-CMDAUDIT-FR-09 (happy, text): list text mode shows entry details.
func TestAudit_FailureRegisterList_Text(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	reg := failure.New()
	reg.Entries = []failure.Entry{
		{
			ID:              "FR-999",
			Component:       "mycomp",
			FailureMode:     "mode-A",
			Status:          failure.StatusTracked,
			SeverityDefault: failure.SeverityS1,
		},
	}
	seedRegister(t, dir, reg)

	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"failure-register", "list", "--root", dir})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("list text: %v\noutput: %s", err, out.String())
	}
	if !strings.Contains(out.String(), "FR-999") {
		t.Errorf("expected FR-999 in output, got: %s", out.String())
	}
	if !strings.Contains(out.String(), "mycomp") {
		t.Errorf("expected mycomp in output, got: %s", out.String())
	}
}

// TC-CMDAUDIT-FR-10 (negative): list with invalid schema → FORGE-3701.
func TestAudit_FailureRegisterList_InvalidSchema(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeRawRegister(t, dir, `{"api_version":"forge.sh/v1","kind":"Wrong","generated_at":"2024-01-01T00:00:00Z","entries":[]}`)

	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"failure-register", "list", "--root", dir})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected FORGE-3701 error from list on invalid schema")
	}
	if !strings.Contains(err.Error(), "FORGE-3701") {
		t.Errorf("expected FORGE-3701, got: %v", err)
	}
}

// TC-CMDAUDIT-FR-11 (negative, JSON): verify --json with drift → {"ok":false,"drifts":[...]}.
func TestAudit_FailureRegisterVerify_JSON_Drift(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	reg := failure.New()
	// TestAnchor absent → drift.
	reg.Entries = []failure.Entry{
		{ID: "FR-001", Component: "c", FailureMode: "m", Status: failure.StatusTracked},
	}
	seedRegister(t, dir, reg)

	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"failure-register", "verify", "--root", dir, "--json"})
	// In JSON mode, drift is encoded in payload; command may exit 0.
	_ = cmd.Execute()
	var report map[string]any
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatalf("not JSON: %v — output: %s", err, out.String())
	}
	if report["ok"] != false {
		t.Errorf("expected ok=false with drift, got: %v", report)
	}
	drifts, ok := report["drifts"].([]any)
	if !ok || len(drifts) == 0 {
		t.Errorf("expected non-empty drifts array, got: %v", report)
	}
}

// TC-CMDAUDIT-FR-12 (happy, text): verify text mode no-drift → "no drift" message.
func TestAudit_FailureRegisterVerify_Text_NoDrift(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	reg := failure.New()
	reg.Entries = []failure.Entry{
		{ID: "FR-001", Component: "c", FailureMode: "m", TestAnchor: "T-01", Status: failure.StatusTracked},
	}
	seedRegister(t, dir, reg)

	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"failure-register", "verify", "--root", dir})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("verify text no drift: %v", err)
	}
	if !strings.Contains(out.String(), "no drift") {
		t.Errorf("expected 'no drift' in output, got: %s", out.String())
	}
}

// TC-CMDAUDIT-FR-13 (negative): verify invalid schema → FORGE-3701.
func TestAudit_FailureRegisterVerify_InvalidSchema(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeRawRegister(t, dir, `{"api_version":"forge.sh/v1","kind":"Bad","generated_at":"2024-01-01T00:00:00Z","entries":[]}`)

	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"failure-register", "verify", "--root", dir})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected FORGE-3701 error")
	}
	if !strings.Contains(err.Error(), "FORGE-3701") {
		t.Errorf("expected FORGE-3701, got: %v", err)
	}
}

// TC-CMDAUDIT-FR-14 (negative): failure-register unknown subcommand → FORGE-3700.
func TestAudit_FailureRegisterUnknownSubcommand(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	seedRegister(t, dir, failure.New())

	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"failure-register", "nope", "--root", dir})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for unknown subcommand")
	}
	if !strings.Contains(err.Error(), "FORGE-3700") {
		t.Errorf("expected FORGE-3700, got: %v", err)
	}
}

// writeRawRegister writes raw bytes directly to the failure-register path,
// bypassing the failure.Save() round-trip.
func writeRawRegister(t *testing.T, dir, raw string) {
	t.Helper()
	dotForge := filepath.Join(dir, ".forge")
	if err := os.MkdirAll(dotForge, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dotForge, "failure-register.json")
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
}
