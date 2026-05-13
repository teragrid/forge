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

// TEST-29: Bug-intake SLA test.

package tasktests

import (
	"testing"

	"github.com/teragrid/forge/internal/incident"
)

// TC-29-01 (happy): a new incident starts in StateIdentified.
func TestTC2901_IncidentInitialState(t *testing.T) {
	t.Parallel()
	inc := incident.New("INC-001", "Test incident", incident.SeverityS2, []string{"llm"})
	if inc.State != incident.StateIdentified {
		t.Errorf("initial state = %q, want %q", inc.State, incident.StateIdentified)
	}
	if err := inc.Validate(); err != nil {
		t.Errorf("Validate: %v", err)
	}
}

// TC-29-04 (idempotency): creating the same incident twice with the same data
// produces structurally identical incidents (no randomness in IDs).
func TestTC2904_IncidentIdempotentCreation(t *testing.T) {
	t.Parallel()
	inc1 := incident.New("INC-002", "Idempotency check", incident.SeverityS3, []string{"cache"})
	inc2 := incident.New("INC-002", "Idempotency check", incident.SeverityS3, []string{"cache"})
	if inc1.ID != inc2.ID {
		t.Errorf("IDs differ: %q vs %q", inc1.ID, inc2.ID)
	}
	if inc1.Title != inc2.Title {
		t.Errorf("Titles differ: %q vs %q", inc1.Title, inc2.Title)
	}
	if inc1.State != inc2.State {
		t.Errorf("States differ: %q vs %q", inc1.State, inc2.State)
	}
}

// TC-29-05 (boundary): valid state machine transitions are accepted.
func TestTC2905_IncidentValidTransitions(t *testing.T) {
	t.Parallel()
	table := []struct {
		from incident.State
		to   incident.State
		want bool
	}{
		{incident.StateIdentified, incident.StateInvestigating, true},
		{incident.StateInvestigating, incident.StateMonitoring, true},
		{incident.StateInvestigating, incident.StateMitigated, true},
		{incident.StateMonitoring, incident.StateMitigated, true},
		{incident.StateMitigated, incident.StateFixed, true},
		{incident.StateFixed, incident.StatePostMortemPublished, true},
		// Invalid transitions.
		{incident.StateIdentified, incident.StateFixed, false},
		{incident.StateMitigated, incident.StateIdentified, false},
		{incident.StatePostMortemPublished, incident.StateIdentified, false},
	}
	for _, tc := range table {
		got := incident.CanTransition(tc.from, tc.to)
		if got != tc.want {
			t.Errorf("CanTransition(%q, %q) = %v, want %v", tc.from, tc.to, got, tc.want)
		}
	}
}

// TC-29-08 (false-positive guard): an incident with SeverityS3 is still valid
// (S3 = lowest severity, not an error).
func TestTC2908_IncidentS3ValidNotBlocked(t *testing.T) {
	t.Parallel()
	inc := incident.New("INC-003", "Low severity incident", incident.SeverityS3, []string{"docs"})
	if err := inc.Validate(); err != nil {
		t.Errorf("S3 incident should be valid: %v", err)
	}
	if inc.IsOpen() == false {
		t.Error("newly created incident should be open")
	}
}

// TC-29-09 (data-accuracy): IsOpen returns false only when fixed or post-mortem published.
func TestTC2909_IncidentIsOpenSemantics(t *testing.T) {
	t.Parallel()
	openStates := []incident.State{
		incident.StateIdentified,
		incident.StateInvestigating,
		incident.StateMonitoring,
		incident.StateMitigated,
	}
	closedStates := []incident.State{
		incident.StateFixed,
		incident.StatePostMortemPublished,
	}
	for _, s := range openStates {
		inc := incident.New("INC-004", "test", incident.SeverityS2, nil)
		inc.State = s
		if !inc.IsOpen() {
			t.Errorf("state %q should be IsOpen=true", s)
		}
	}
	for _, s := range closedStates {
		inc := incident.New("INC-005", "test", incident.SeverityS2, nil)
		inc.State = s
		if inc.IsOpen() {
			t.Errorf("state %q should be IsOpen=false", s)
		}
	}
}
