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

// Package eval – quarantine.go implements the ADR-023 eval flake quarantine
// policy (M3-21). Flaky scenarios are marked quarantined in a sidecar file so
// the CI eval-proof gate skips them while the underlying cause is investigated.
//
// Quarantine lifecycle:
//
//	Active → Quarantined (via QuarantineScenario)
//	Quarantined → Active  (via UnquarantineScenario)
//	Quarantined → Deleted (when the scenario is fixed and re-validated)
//
// The quarantine registry is stored at .forge/eval-quarantine.json and is
// committed to version control so the exemption is visible in PRs.
package eval

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// QuarantineEntry records one quarantined scenario.
type QuarantineEntry struct {
	ScenarioName  string `json:"scenario_name"`
	Reason        string `json:"reason"`
	QuarantinedBy string `json:"quarantined_by,omitempty"` // GitHub handle or "ci"
	QuarantinedAt string `json:"quarantined_at"`           // RFC3339
	// ExpiresAt is a hard expiry. CI will fail the build if a quarantine
	// has been open longer than MaxQuarantineDays without review.
	ExpiresAt     string `json:"expires_at"`
	TrackingIssue string `json:"tracking_issue,omitempty"` // e.g. "https://github.com/teragrid/forge/issues/123"
}

// QuarantineRegistry is the full on-disk quarantine state.
type QuarantineRegistry struct {
	SchemaVersion string            `json:"schema_version"`
	Entries       []QuarantineEntry `json:"entries"`
}

// DefaultQuarantinePath is where the quarantine registry is stored.
const DefaultQuarantinePath = ".forge/eval-quarantine.json"

// MaxQuarantineDays is the ADR-023 hard cap: a scenario may not remain
// quarantined for more than 14 calendar days without a renewed exemption.
const MaxQuarantineDays = 14

// LoadQuarantine reads the quarantine registry from path.
// Returns an empty registry if the file does not exist.
func LoadQuarantine(path string) (*QuarantineRegistry, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &QuarantineRegistry{SchemaVersion: "1"}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("quarantine: read %s: %w", path, err)
	}
	var reg QuarantineRegistry
	if err := json.Unmarshal(data, &reg); err != nil {
		return nil, fmt.Errorf("quarantine: parse %s: %w", path, err)
	}
	return &reg, nil
}

// Save writes the registry back to path (mode 0644).
func (r *QuarantineRegistry) Save(path string) error {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Errorf("quarantine: marshal: %w", err)
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

// QuarantineScenario adds scenarioName to the quarantine list with the given
// reason, owner, and tracking issue. The expiry is set to MaxQuarantineDays
// from now.
func (r *QuarantineRegistry) QuarantineScenario(scenarioName, reason, by, trackingIssue string) error {
	if r.IsQuarantined(scenarioName) {
		return fmt.Errorf("quarantine: scenario %q is already quarantined", scenarioName)
	}
	now := time.Now().UTC()
	r.Entries = append(r.Entries, QuarantineEntry{
		ScenarioName:  scenarioName,
		Reason:        reason,
		QuarantinedBy: by,
		QuarantinedAt: now.Format(time.RFC3339),
		ExpiresAt:     now.AddDate(0, 0, MaxQuarantineDays).Format(time.RFC3339),
		TrackingIssue: trackingIssue,
	})
	return nil
}

// UnquarantineScenario removes scenarioName from the quarantine list.
func (r *QuarantineRegistry) UnquarantineScenario(scenarioName string) error {
	for i, e := range r.Entries {
		if e.ScenarioName == scenarioName {
			r.Entries = append(r.Entries[:i], r.Entries[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("quarantine: scenario %q not found", scenarioName)
}

// IsQuarantined returns true if scenarioName is currently quarantined
// (regardless of expiry — expiry is checked separately by CheckExpired).
func (r *QuarantineRegistry) IsQuarantined(scenarioName string) bool {
	for _, e := range r.Entries {
		if e.ScenarioName == scenarioName {
			return true
		}
	}
	return false
}

// CheckExpired returns an error listing any quarantines that have passed their
// ExpiresAt date. CI uses this to block builds with stale exemptions.
func (r *QuarantineRegistry) CheckExpired() error {
	now := time.Now().UTC()
	var expired []string
	for _, e := range r.Entries {
		exp, err := time.Parse(time.RFC3339, e.ExpiresAt)
		if err != nil {
			continue // malformed entry; ignore
		}
		if now.After(exp) {
			expired = append(expired, fmt.Sprintf("%s (expired %s, issue: %s)",
				e.ScenarioName, e.ExpiresAt, e.TrackingIssue))
		}
	}
	if len(expired) == 0 {
		return nil
	}
	msg := "quarantine: the following scenarios have expired exemptions and must be fixed or renewed:\n"
	for _, s := range expired {
		msg += "  - " + s + "\n"
	}
	return fmt.Errorf("%s", msg)
}
