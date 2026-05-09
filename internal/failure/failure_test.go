package failure

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// --- Entry.Validate tests ---

// TC-FR-01 (happy): valid entry passes validation.
func TestEntry_Validate_Happy(t *testing.T) {
	e := Entry{
		ID:          "FR-001",
		Component:   "audit-ledger",
		FailureMode: "Ledger corrupted.",
		Status:      StatusTracked,
	}
	if err := e.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TC-FR-02 (negative): missing ID returns error.
func TestEntry_Validate_MissingID(t *testing.T) {
	e := Entry{Component: "c", FailureMode: "m", Status: StatusTracked}
	if err := e.Validate(); err == nil {
		t.Fatal("expected error for missing ID")
	}
}

// TC-FR-03 (negative): missing component returns error.
func TestEntry_Validate_MissingComponent(t *testing.T) {
	e := Entry{ID: "FR-001", FailureMode: "m", Status: StatusTracked}
	if err := e.Validate(); err == nil {
		t.Fatal("expected error for missing component")
	}
}

// TC-FR-04 (negative): missing failure_mode returns error.
func TestEntry_Validate_MissingFailureMode(t *testing.T) {
	e := Entry{ID: "FR-001", Component: "c", Status: StatusTracked}
	if err := e.Validate(); err == nil {
		t.Fatal("expected error for missing failure_mode")
	}
}

// TC-FR-05 (negative): unknown status returns error.
func TestEntry_Validate_UnknownStatus(t *testing.T) {
	e := Entry{ID: "FR-001", Component: "c", FailureMode: "m", Status: "unknown"}
	if err := e.Validate(); err == nil {
		t.Fatal("expected error for unknown status")
	}
}

// TC-FR-06 (false-positive guard): retired status is valid.
func TestEntry_Validate_RetiredOK(t *testing.T) {
	e := Entry{ID: "FR-001", Component: "c", FailureMode: "m", Status: StatusRetired}
	if err := e.Validate(); err != nil {
		t.Fatalf("unexpected error for retired: %v", err)
	}
}

// --- Register.Validate tests ---

// TC-FR-07 (happy): well-formed register passes.
func TestRegister_Validate_Happy(t *testing.T) {
	r := New()
	r.Entries = []Entry{{ID: "FR-001", Component: "c", FailureMode: "m", Status: StatusTracked}}
	if err := r.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TC-FR-08 (negative): wrong kind returns error.
func TestRegister_Validate_WrongKind(t *testing.T) {
	r := New()
	r.Kind = "Other"
	if err := r.Validate(); err == nil {
		t.Fatal("expected error for wrong kind")
	}
}

// TC-FR-09 (negative): duplicate entry IDs return error.
func TestRegister_Validate_DuplicateID(t *testing.T) {
	r := New()
	e := Entry{ID: "FR-001", Component: "c", FailureMode: "m", Status: StatusTracked}
	r.Entries = []Entry{e, e}
	if err := r.Validate(); err == nil {
		t.Fatal("expected error for duplicate IDs")
	}
}

// TC-FR-10 (boundary): empty entries list is valid.
func TestRegister_Validate_EmptyEntries(t *testing.T) {
	r := New()
	if err := r.Validate(); err != nil {
		t.Fatalf("empty register should be valid: %v", err)
	}
}

// --- Active() tests ---

// TC-FR-11 (data-accuracy): Active() filters out retired entries.
func TestRegister_Active_FiltersRetired(t *testing.T) {
	r := New()
	r.Entries = []Entry{
		{ID: "FR-001", Component: "c", FailureMode: "tracked", Status: StatusTracked},
		{ID: "FR-002", Component: "c", FailureMode: "retired", Status: StatusRetired},
		{ID: "FR-003", Component: "c", FailureMode: "tracked2", Status: StatusTracked},
	}
	active := r.Active()
	if len(active) != 2 {
		t.Errorf("want 2 active, got %d", len(active))
	}
	for _, e := range active {
		if e.Status != StatusTracked {
			t.Errorf("unexpected retired in active: %s", e.ID)
		}
	}
}

// TC-FR-12 (boundary): Active() on empty register returns empty slice.
func TestRegister_Active_Empty(t *testing.T) {
	r := New()
	if a := r.Active(); len(a) != 0 {
		t.Errorf("want 0, got %d", len(a))
	}
}

// --- Load / Save tests ---

// TC-FR-13 (happy + data-accuracy): Save round-trips through Load.
func TestRegister_SaveLoad_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "failure-register.json")

	r := New()
	r.Entries = []Entry{
		{
			ID:              "FR-001",
			Component:       "scan-engine",
			FailureMode:     "Engine OOM on large corpus.",
			Detection:       "OOMKill metric",
			SeverityDefault: SeverityS1,
			ErrorCodes:      []string{"FORGE-2001"},
			Status:          StatusTracked,
		},
	}
	if err := r.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(loaded.Entries) != 1 {
		t.Fatalf("want 1 entry, got %d", len(loaded.Entries))
	}
	got := loaded.Entries[0]
	if got.ID != "FR-001" || got.Component != "scan-engine" ||
		got.SeverityDefault != SeverityS1 || len(got.ErrorCodes) != 1 {
		t.Errorf("round-trip mismatch: %+v", got)
	}
}

// TC-FR-14 (boundary): Load on missing file returns a fresh empty register.
func TestLoad_MissingFile(t *testing.T) {
	r, err := Load("/no/such/file/failure-register.json")
	if err != nil {
		t.Fatalf("expected nil error for missing file, got %v", err)
	}
	if r == nil || len(r.Entries) != 0 {
		t.Error("expected empty register for missing file")
	}
}

// TC-FR-15 (negative): Load on corrupt JSON returns error.
func TestLoad_BadJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(path, []byte("{bad"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected error for bad JSON")
	}
}

// TC-FR-16 (idempotency): Save then Save again preserves data, updates GeneratedAt.
func TestRegister_Save_Idempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fr.json")
	r := New()
	r.Entries = []Entry{{ID: "FR-001", Component: "c", FailureMode: "m", Status: StatusTracked}}

	if err := r.Save(path); err != nil {
		t.Fatalf("first save: %v", err)
	}
	t1 := r.GeneratedAt

	time.Sleep(2 * time.Millisecond)
	if err := r.Save(path); err != nil {
		t.Fatalf("second save: %v", err)
	}
	t2 := r.GeneratedAt

	if !t2.After(t1) {
		t.Error("GeneratedAt should advance on second save")
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("load after second save: %v", err)
	}
	if len(loaded.Entries) != 1 || loaded.Entries[0].ID != "FR-001" {
		t.Error("data lost after second save")
	}
}

// TC-FR-17 (data-accuracy): JSON output contains expected keys.
func TestRegister_JSON_Keys(t *testing.T) {
	r := New()
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"api_version", "kind", "generated_at", "entries"} {
		if _, ok := m[key]; !ok {
			t.Errorf("missing key %q in JSON", key)
		}
	}
}

// TC-FR-18 (data-accuracy): LoadDefault uses root/DefaultPath.
func TestLoadDefault_PathContract(t *testing.T) {
	dir := t.TempDir()
	dotForge := filepath.Join(dir, ".forge")
	if err := os.MkdirAll(dotForge, 0o755); err != nil {
		t.Fatal(err)
	}
	r := New()
	r.Entries = []Entry{{ID: "FR-099", Component: "test", FailureMode: "unit test", Status: StatusTracked}}
	if err := r.Save(filepath.Join(dotForge, "failure-register.json")); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadDefault(dir)
	if err != nil {
		t.Fatalf("LoadDefault: %v", err)
	}
	if len(loaded.Entries) != 1 || loaded.Entries[0].ID != "FR-099" {
		t.Errorf("LoadDefault path mismatch: got %+v", loaded.Entries)
	}
}
