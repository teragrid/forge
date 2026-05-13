// Copyright 2024 The Forge Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// TEST-30: Failure-register sync linter.

package tasktests

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/teragrid/forge/internal/failure"
)

// goodEntry returns a valid failure.Entry for testing.
func goodEntry(id string) failure.Entry {
	return failure.Entry{
		ID:          id,
		Component:   "test-component",
		FailureMode: "test failure mode",
		Status:      failure.StatusTracked,
	}
}

// TC-30-01 (happy): a clean failure register passes Validate().
func TestTC3001_FailureRegisterCleanPasses(t *testing.T) {
	t.Parallel()
	reg := failure.New()
	reg.Entries = []failure.Entry{
		goodEntry("FR-001"),
		goodEntry("FR-002"),
	}
	if err := reg.Validate(); err != nil {
		t.Errorf("clean register failed Validate: %v", err)
	}
}

// TC-30-02 (negative): an entry with a missing component is rejected.
func TestTC3002_FailureRegisterMissingComponentFails(t *testing.T) {
	t.Parallel()
	reg := failure.New()
	bad := goodEntry("FR-001")
	bad.Component = ""
	reg.Entries = []failure.Entry{bad}
	if err := reg.Validate(); err == nil {
		t.Error("expected Validate to reject entry with missing component")
	}
}

// TC-30-03 (negative): an entry with an unknown Status is rejected.
func TestTC3003_FailureRegisterUnknownStatusFails(t *testing.T) {
	t.Parallel()
	reg := failure.New()
	bad := goodEntry("FR-001")
	bad.Status = "unknown-status"
	reg.Entries = []failure.Entry{bad}
	if err := reg.Validate(); err == nil {
		t.Error("expected Validate to reject entry with unknown status")
	}
}

// TC-30-04 (negative): duplicate entry IDs are rejected.
func TestTC3004_FailureRegisterDuplicateIDFails(t *testing.T) {
	t.Parallel()
	reg := failure.New()
	reg.Entries = []failure.Entry{
		goodEntry("FR-001"),
		goodEntry("FR-001"), // duplicate
	}
	if err := reg.Validate(); err == nil {
		t.Error("expected Validate to reject duplicate entry IDs")
	}
}

// TC-30-06 (idempotency): loading a register from disk and re-validating is idempotent.
func TestTC3006_FailureRegisterLoadIdempotent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "failure-register.json")

	reg := failure.New()
	reg.Entries = []failure.Entry{goodEntry("FR-001")}
	reg.GeneratedAt = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	data, err := json.MarshalIndent(reg, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	loaded, err := failure.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := loaded.Validate(); err != nil {
		t.Errorf("loaded register Validate failed: %v", err)
	}
	// Second load.
	loaded2, err := failure.Load(path)
	if err != nil {
		t.Fatalf("second Load: %v", err)
	}
	if err := loaded2.Validate(); err != nil {
		t.Errorf("second load Validate failed: %v", err)
	}
}

// TC-30-07 (data-accuracy): Active() returns only tracked entries.
func TestTC3007_FailureRegisterActiveFilterRetired(t *testing.T) {
	t.Parallel()
	reg := failure.New()
	reg.Entries = []failure.Entry{
		goodEntry("FR-001"),
		{ID: "FR-002", Component: "c", FailureMode: "f", Status: failure.StatusRetired},
	}
	active := reg.Active()
	if len(active) != 1 {
		t.Errorf("Active() returned %d entries, want 1", len(active))
	}
	if active[0].ID != "FR-001" {
		t.Errorf("Active()[0].ID = %q, want FR-001", active[0].ID)
	}
}

// TC-30-08 (regression): loading a non-existent file returns a valid empty register.
func TestTC3008_FailureRegisterMissingFileReturnsEmpty(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "does-not-exist.json")
	reg, err := failure.Load(path)
	if err != nil {
		t.Fatalf("Load of missing file returned error: %v", err)
	}
	if len(reg.Entries) != 0 {
		t.Errorf("empty register has %d entries, want 0", len(reg.Entries))
	}
}
