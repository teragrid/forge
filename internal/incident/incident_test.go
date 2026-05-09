package incident

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TC-INC-01: New() creates incident in StateIdentified.
func TestNew_StateIdentified(t *testing.T) {
	inc := New("INC-001", "DB connection lost", SeverityS1, []string{"DB", "API"})
	if inc.State != StateIdentified {
		t.Errorf("want StateIdentified, got %s", inc.State)
	}
	if inc.ID != "INC-001" {
		t.Errorf("want ID=INC-001, got %s", inc.ID)
	}
	if inc.Severity != SeverityS1 {
		t.Errorf("want S1, got %s", inc.Severity)
	}
	if len(inc.Systems) != 2 {
		t.Errorf("want 2 systems, got %d", len(inc.Systems))
	}
}

// TC-INC-02: Legal state transitions succeed.
func TestTransition_Legal(t *testing.T) {
	cases := []struct{ from, to State }{
		{StateIdentified, StateInvestigating},
		{StateInvestigating, StateMonitoring},
		{StateMonitoring, StateInvestigating},
		{StateInvestigating, StateMitigated},
		{StateMitigated, StateFixed},
		{StateFixed, StatePostMortemPublished},
	}
	for _, tc := range cases {
		inc := &Incident{ID: "INC-X", State: tc.from}
		if err := Transition(inc, tc.to); err != nil {
			t.Errorf("legal transition %s→%s failed: %v", tc.from, tc.to, err)
		}
		if inc.State != tc.to {
			t.Errorf("state not updated: got %s", inc.State)
		}
	}
}

// TC-INC-03: Illegal state transition returns error.
func TestTransition_Illegal(t *testing.T) {
	inc := New("INC-002", "title", SeverityS2, nil)
	// Cannot jump from Identified → Fixed.
	if err := Transition(inc, StateFixed); err == nil {
		t.Fatal("want error for illegal transition Identified→Fixed, got nil")
	}
}

// TC-INC-04: Save + Load round-trip preserves all fields.
func TestSaveLoad_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	inc := New("INC-003", "test incident", SeverityS0, []string{"CLI"})
	inc.Notes = append(inc.Notes, "initial report")

	if err := Save(dir, inc); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load(dir, "INC-003")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Title != inc.Title {
		t.Errorf("title mismatch: got %q", got.Title)
	}
	if got.State != inc.State {
		t.Errorf("state mismatch: got %s", got.State)
	}
	if len(got.Notes) != 1 {
		t.Errorf("want 1 note, got %d", len(got.Notes))
	}
}

// TC-INC-05: Load non-existent file returns error.
func TestLoad_NotFound(t *testing.T) {
	dir := t.TempDir()
	if _, err := Load(dir, "INC-MISSING"); err == nil {
		t.Fatal("want error loading missing incident, got nil")
	}
}

// TC-INC-06: LoadAll on empty/missing dir returns nil, nil.
func TestLoadAll_EmptyDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nonexistent")
	list, err := LoadAll(dir)
	if err != nil {
		t.Fatalf("LoadAll empty: %v", err)
	}
	if list != nil {
		t.Fatalf("want nil list for missing dir, got %v", list)
	}
}

// TC-INC-07: LoadAll reads multiple incidents.
func TestLoadAll_Multiple(t *testing.T) {
	dir := t.TempDir()
	for _, id := range []string{"INC-010", "INC-011", "INC-012"} {
		inc := New(id, "title "+id, SeverityS3, nil)
		if err := Save(dir, inc); err != nil {
			t.Fatalf("Save %s: %v", id, err)
		}
	}
	list, err := LoadAll(dir)
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(list) != 3 {
		t.Errorf("want 3 incidents, got %d", len(list))
	}
}

// TC-INC-08: IsOpen returns false for terminal states.
func TestIsOpen_TerminalStates(t *testing.T) {
	for _, st := range []State{StateFixed, StatePostMortemPublished} {
		inc := &Incident{State: st}
		if inc.IsOpen() {
			t.Errorf("IsOpen: expected false for state %s", st)
		}
	}
}

// TC-INC-09: IsOpen returns true for non-terminal states.
func TestIsOpen_NonTerminal(t *testing.T) {
	for _, st := range []State{StateIdentified, StateInvestigating, StateMonitoring, StateMitigated} {
		inc := &Incident{State: st}
		if !inc.IsOpen() {
			t.Errorf("IsOpen: expected true for state %s", st)
		}
	}
}

// TC-INC-10: Validate rejects missing ID.
func TestValidate_MissingID(t *testing.T) {
	inc := &Incident{Title: "title", Severity: SeverityS1, State: StateIdentified}
	if err := inc.Validate(); err == nil {
		t.Fatal("want validation error for missing ID")
	}
}

// TC-INC-11: Validate rejects unknown severity.
func TestValidate_BadSeverity(t *testing.T) {
	inc := &Incident{ID: "INC-X", Title: "title", Severity: "S9", State: StateIdentified}
	if err := inc.Validate(); err == nil {
		t.Fatal("want validation error for unknown severity S9")
	}
}

// TC-INC-12: RenderMarkdown includes title and state.
func TestRenderMarkdown_Fields(t *testing.T) {
	inc := New("INC-020", "DB outage", SeverityS1, []string{"DB"})
	inc.Notes = append(inc.Notes, "first update")
	md := RenderMarkdown(inc)
	if !strings.Contains(md, "DB outage") {
		t.Error("markdown missing title")
	}
	if !strings.Contains(md, "identified") {
		t.Error("markdown missing state")
	}
	if !strings.Contains(md, "first update") {
		t.Error("markdown missing note")
	}
}

// TC-INC-13: RenderMarkdown sets Resolved: true for fixed incidents.
func TestRenderMarkdown_Resolved(t *testing.T) {
	inc := &Incident{ID: "INC-X", Title: "t", State: StateFixed, Severity: SeverityS2, CreatedAt: time.Now().UTC()}
	md := RenderMarkdown(inc)
	if !strings.Contains(md, "Resolved: true") {
		t.Errorf("want Resolved: true for StateFixed, got:\n%s", md)
	}
}

// TC-INC-14: Load rejects bad JSON.
func TestLoad_BadJSON(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "inc-bad.json")
	os.WriteFile(p, []byte("{not valid json}"), 0o644) //nolint:errcheck
	if _, err := Load(dir, "inc-bad"); err == nil {
		t.Fatal("want error for bad JSON")
	}
}

// TC-INC-15: Save is idempotent — double-save does not corrupt.
func TestSave_Idempotent(t *testing.T) {
	dir := t.TempDir()
	inc := New("INC-040", "idempotency", SeverityS3, nil)
	Save(dir, inc) //nolint:errcheck
	Save(dir, inc) //nolint:errcheck
	got, err := Load(dir, "INC-040")
	if err != nil {
		t.Fatalf("Load after double-save: %v", err)
	}
	if got.Title != "idempotency" {
		t.Errorf("title corrupted: %q", got.Title)
	}
}

// TC-INC-16: false-positive guard — completed incident does NOT IsOpen().
func TestFalsePositive_ClosedNotOpen(t *testing.T) {
	inc := &Incident{State: StatePostMortemPublished}
	if inc.IsOpen() {
		t.Fatal("false-positive: post-mortem-published should not be open")
	}
}
