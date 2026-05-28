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

import (
	"strings"
	"testing"
)

func TestSpecStub_IncludesTopStatusSummary(t *testing.T) {
	t.Parallel()
	stub := specStub("rate limiting")

	mustContain := []string{
		"## Status Summary",
		"- Lifecycle: Draft",
		"- Version Scope: PATCH",
		"- Last Updated:",
		"- Checkpoint Progress:",
		"## What",
		"## Acceptance Criteria",
	}
	for _, token := range mustContain {
		if !strings.Contains(stub, token) {
			t.Fatalf("spec stub missing %q\n\n%s", token, stub)
		}
	}

	if strings.Index(stub, "## Status Summary") > strings.Index(stub, "## What") {
		t.Fatalf("status summary must be above ## What\n\n%s", stub)
	}
}

func TestSteering_RequirementsQuality_EnforcesStatusSummary(t *testing.T) {
	t.Parallel()
	if !strings.Contains(requirementsQualitySteering, "Status Summary") {
		t.Fatalf("requirements-quality steering must enforce top Status Summary block")
	}
	if !strings.Contains(requirementsQualitySteering, "Version Scope") {
		t.Fatalf("requirements-quality steering must require Version Scope")
	}
}

func TestSteering_ReleaseScopeGuard_PrefersPatch(t *testing.T) {
	t.Parallel()
	if !strings.Contains(reviewTechChangeSteering, "Prefer PATCH by default") {
		t.Fatalf("ship steering must include PATCH-first release scope guidance")
	}
}

// TestSteering_SpecStatusMaintenance_IsAlwaysOn verifies that
// spec-status-maintenance fires on every checkpoint.
func TestSteering_SpecStatusMaintenance_IsAlwaysOn(t *testing.T) {
	t.Parallel()
	steerings := DefaultSteerings()
	var sm *Steering
	for i := range steerings {
		if steerings[i].Name == "spec-status-maintenance" {
			sm = &steerings[i]
			break
		}
	}
	if sm == nil {
		t.Fatal("spec-status-maintenance steering not registered in DefaultSteerings")
	}
	for _, cp := range []string{"spec", "arch", "test", "breakdown", "code", "ship", "qa-verify"} {
		if !sm.Applies(cp, nil) {
			t.Errorf("spec-status-maintenance must apply to checkpoint %q but Applies returned false", cp)
		}
	}
}

// TestSteering_SpecStatusMaintenance_HasVelocityGuard ensures the steering
// text explicitly guards against bumping MINOR too frequently.
func TestSteering_SpecStatusMaintenance_HasVelocityGuard(t *testing.T) {
	t.Parallel()
	must := []string{
		"Version bump velocity guard",
		"PATCH",
		"MINOR",
		"MAJOR",
		"deliberate milestone",
	}
	for _, token := range must {
		if !strings.Contains(specStatusMaintenanceSteering, token) {
			t.Fatalf("spec-status-maintenance steering missing %q", token)
		}
	}
}

// TestSteering_PromptGuide_IncludesStatusSummaryRule ensures the always-on
// prompt-guide instructs the LLM to keep the Status Summary current.
func TestSteering_PromptGuide_IncludesStatusSummaryRule(t *testing.T) {
	t.Parallel()
	if !strings.Contains(promptGuideSteering, "Status Summary") {
		t.Fatal("prompt-guide must remind LLM to keep Status Summary current")
	}
}
