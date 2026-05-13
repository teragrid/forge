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
// Package failure implements the Forge failure-register data model (ADR-016).
//
// The register lives at .forge/failure-register.json in the Forge source
// repo (or whichever project adopts Forge governance). JSON is used in
// place of the YAML format stated in ADR-016 because the codebase ships
// no third-party YAML dependency; the on-disk schema is otherwise identical.
//
// Machine-readable schema reference: docs/schemas/failure-register.schema.json
// (generated from this package's types).
package failure

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// DefaultPath is the project-relative path for the failure register.
const DefaultPath = ".forge/failure-register.json"

// Status of a failure-register entry.
type Status string

const (
	StatusTracked Status = "tracked"
	StatusRetired Status = "retired"
)

// Severity classification aligns with the incident severity model (ADR-020).
type Severity string

const (
	SeverityS0 Severity = "S0"
	SeverityS1 Severity = "S1"
	SeverityS2 Severity = "S2"
	SeverityS3 Severity = "S3"
)

// Entry is one row in the failure register.
type Entry struct {
	// ID is a stable, human-assigned identifier of the form FR-NNN.
	ID string `json:"id"`

	// Component is the system component that can fail (e.g. "audit-ledger").
	Component string `json:"component"`

	// Tier is the plugin tier (T1 / T2 / T3) or "system".
	Tier string `json:"tier,omitempty"`

	// FailureMode is a short human-readable description of the failure.
	FailureMode string `json:"failure_mode"`

	// Detection describes how the failure is detected (metrics, tests, etc.).
	Detection string `json:"detection,omitempty"`

	// Recovery describes the runbook or recovery steps.
	Recovery string `json:"recovery,omitempty"`

	// SeverityDefault is the default severity (S0–S3) for this failure mode.
	SeverityDefault Severity `json:"severity_default,omitempty"`

	// ErrorCodes lists FORGE-NNNN codes raised by this failure mode.
	ErrorCodes []string `json:"error_codes,omitempty"`

	// TestAnchor is the test ID that covers this failure mode (e.g. "TEST-23-04").
	TestAnchor string `json:"test_anchor,omitempty"`

	// DrillAnchor is the chaos-drill ID that exercises this failure mode.
	DrillAnchor string `json:"drill_anchor,omitempty"`

	// FirstSeenInDocTable is the date (YYYY-MM-DD) the entry first appeared
	// in the architecture §17.2 table.
	FirstSeenInDocTable string `json:"first_seen_in_doc_table,omitempty"`

	// Status is "tracked" (default) or "retired". Retired entries are kept
	// in the file but omitted from rendered dashboards.
	Status Status `json:"status"`
}

// Validate returns an error if the entry is incomplete or malformed.
func (e Entry) Validate() error {
	if e.ID == "" {
		return fmt.Errorf("entry: id is required")
	}
	if e.Component == "" {
		return fmt.Errorf("entry %s: component is required", e.ID)
	}
	if e.FailureMode == "" {
		return fmt.Errorf("entry %s: failure_mode is required", e.ID)
	}
	switch e.Status {
	case StatusTracked, StatusRetired:
	case "":
		return fmt.Errorf("entry %s: status is required (tracked|retired)", e.ID)
	default:
		return fmt.Errorf("entry %s: unknown status %q", e.ID, e.Status)
	}
	return nil
}

// Register is the root document stored in DefaultPath.
type Register struct {
	// APIVersion is the schema version (e.g. "forge.sh/v1").
	APIVersion string `json:"api_version"`

	// Kind must be "FailureRegister".
	Kind string `json:"kind"`

	// GeneratedAt is the ISO-8601 timestamp of the last write.
	GeneratedAt time.Time `json:"generated_at"`

	// Entries is the list of failure-register rows.
	Entries []Entry `json:"entries"`
}

// New returns an empty Register with the current API version.
func New() *Register {
	return &Register{
		APIVersion:  "forge.sh/v1",
		Kind:        "FailureRegister",
		GeneratedAt: time.Now().UTC(),
		Entries:     []Entry{},
	}
}

// Validate returns a non-nil error if the register is structurally invalid.
// It checks global fields and each entry, collecting all errors into one.
func (r *Register) Validate() error {
	if r.APIVersion == "" {
		return fmt.Errorf("register: api_version is required")
	}
	if r.Kind != "FailureRegister" {
		return fmt.Errorf("register: kind must be FailureRegister, got %q", r.Kind)
	}
	ids := make(map[string]bool, len(r.Entries))
	for i, e := range r.Entries {
		if err := e.Validate(); err != nil {
			return fmt.Errorf("entry %d: %w", i, err)
		}
		if ids[e.ID] {
			return fmt.Errorf("duplicate entry id %q", e.ID)
		}
		ids[e.ID] = true
	}
	return nil
}

// Active returns only entries with Status == StatusTracked.
func (r *Register) Active() []Entry {
	out := make([]Entry, 0, len(r.Entries))
	for _, e := range r.Entries {
		if e.Status == StatusTracked {
			out = append(out, e)
		}
	}
	return out
}

// Load reads and parses the register from path. If the file does not exist
// it returns a fresh empty register (no error).
func Load(path string) (*Register, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return New(), nil
		}
		return nil, fmt.Errorf("failure register: read %s: %w", path, err)
	}
	var r Register
	if err := json.Unmarshal(b, &r); err != nil {
		return nil, fmt.Errorf("failure register: parse %s: %w", path, err)
	}
	return &r, nil
}

// LoadDefault reads the register from root/DefaultPath.
func LoadDefault(root string) (*Register, error) {
	return Load(filepath.Join(root, DefaultPath))
}

// Save writes the register as indented JSON. It creates parent directories
// as needed.
func (r *Register) Save(path string) error {
	r.GeneratedAt = time.Now().UTC()
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Errorf("failure register: marshal: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("failure register: mkdir: %w", err)
	}
	return os.WriteFile(path, append(b, '\n'), 0o600)
}
