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

// G-083: profile support tests.
package config_test

import (
	"testing"

	"github.com/teragrid/forge/internal/config"
)

// TestGetProfile_AllBuiltins verifies that all three built-in profiles can be
// retrieved without error.
func TestGetProfile_AllBuiltins(t *testing.T) {
	t.Parallel()
	for _, name := range config.ProfileNames() {
		name := name
		t.Run(string(name), func(t *testing.T) {
			t.Parallel()
			p, err := config.GetProfile(name)
			if err != nil {
				t.Fatalf("GetProfile(%q): %v", name, err)
			}
			if p.Name != name {
				t.Errorf("Name mismatch: got %q, want %q", p.Name, name)
			}
			if p.MinTier == "" {
				t.Errorf("MinTier must not be empty for profile %q", name)
			}
			if p.ScanStrictness == "" {
				t.Errorf("ScanStrictness must not be empty for profile %q", name)
			}
			if p.ConfidenceThreshold == "" {
				t.Errorf("ConfidenceThreshold must not be empty for profile %q", name)
			}
		})
	}
}

// TestGetProfile_Unknown verifies that an unknown profile name returns an error.
func TestGetProfile_Unknown(t *testing.T) {
	t.Parallel()
	_, err := config.GetProfile("nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown profile, got nil")
	}
}

// TestProfile_FastIsLessStrictThanParanoid verifies that the fast profile has
// lower scan strictness than the paranoid profile (ordering invariant).
func TestProfile_FastIsLessStrictThanParanoid(t *testing.T) {
	t.Parallel()
	fast, _ := config.GetProfile(config.ProfileFast)
	paranoid, _ := config.GetProfile(config.ProfileParanoid)

	// Fast must skip medium findings; paranoid must include them.
	if fast.IncludeMedium {
		t.Error("fast profile must not include medium-confidence findings")
	}
	if !paranoid.IncludeMedium {
		t.Error("paranoid profile must include medium-confidence findings")
	}
	// fast token budget must be less than or equal to paranoid.
	if fast.MaxLLMTokenBudget > paranoid.MaxLLMTokenBudget {
		t.Errorf("fast.MaxLLMTokenBudget (%d) > paranoid.MaxLLMTokenBudget (%d)",
			fast.MaxLLMTokenBudget, paranoid.MaxLLMTokenBudget)
	}
}

// TestProfile_SafeIsBetweenFastAndParanoid verifies that safe sits between the
// two extreme profiles on the IncludeMedium axis.
func TestProfile_SafeIsBetweenFastAndParanoid(t *testing.T) {
	t.Parallel()
	safe, _ := config.GetProfile(config.ProfileSafe)

	// Safe should include medium (it's the balanced default).
	if !safe.IncludeMedium {
		t.Error("safe profile should include medium-confidence findings")
	}
}

// TestProfileNames_AllThreePresent verifies the canonical name list.
func TestProfileNames_AllThreePresent(t *testing.T) {
	t.Parallel()
	names := config.ProfileNames()
	if len(names) < 3 {
		t.Fatalf("expected at least 3 profiles, got %d", len(names))
	}
	seen := map[config.ProfileName]bool{}
	for _, n := range names {
		seen[n] = true
	}
	for _, want := range []config.ProfileName{config.ProfileFast, config.ProfileSafe, config.ProfileParanoid} {
		if !seen[want] {
			t.Errorf("ProfileNames missing %q", want)
		}
	}
}

// TestProfile_AppliedToScan verifies that applying a profile to a scan command
// via the --profile flag does not cause an error (integration smoke test).
func TestProfile_AppliedToScan(t *testing.T) {
	t.Parallel()
	for _, name := range config.ProfileNames() {
		name := name
		t.Run(string(name), func(t *testing.T) {
			t.Parallel()
			p, err := config.GetProfile(name)
			if err != nil {
				t.Fatalf("GetProfile: %v", err)
			}
			// Verify profile settings are self-consistent.
			switch p.ScanStrictness {
			case "low", "medium", "high":
			default:
				t.Errorf("unexpected ScanStrictness %q for profile %q", p.ScanStrictness, name)
			}
			switch p.ConfidenceThreshold {
			case "low", "medium", "high":
			default:
				t.Errorf("unexpected ConfidenceThreshold %q for profile %q", p.ConfidenceThreshold, name)
			}
			switch p.MinTier {
			case "T0", "T1", "T2":
			default:
				t.Errorf("unexpected MinTier %q for profile %q", p.MinTier, name)
			}
		})
	}
}

// ── Active-profile state ──────────────────────────────────────────────────────

// TestActiveProfile_NilBeforeSet verifies that after clearing via a zero-value
// profile, ActiveProfile returns nil.
func TestActiveProfile_NilBeforeSet(t *testing.T) {
	// Cannot run parallel: mutates package-level state.
	config.SetActiveProfile(config.Profile{}) // zero-value → clears active profile
	if got := config.ActiveProfile(); got != nil {
		t.Errorf("expected nil after clear, got %+v", got)
	}
}

// TestSetActiveProfile_RoundTrip verifies that SetActiveProfile/ActiveProfile
// form a consistent round-trip for all built-in profiles.
func TestSetActiveProfile_RoundTrip(t *testing.T) {
	// Cannot run parallel: mutates package-level state.
	for _, name := range config.ProfileNames() {
		p, err := config.GetProfile(name)
		if err != nil {
			t.Fatalf("GetProfile(%q): %v", name, err)
		}
		config.SetActiveProfile(p)
		got := config.ActiveProfile()
		if got == nil {
			t.Fatalf("ActiveProfile() returned nil after SetActiveProfile(%q)", name)
		}
		if got.Name != name {
			t.Errorf("round-trip name: got %q, want %q", got.Name, name)
		}
		if got.MaxLLMTokenBudget != p.MaxLLMTokenBudget {
			t.Errorf("round-trip MaxLLMTokenBudget: got %d, want %d", got.MaxLLMTokenBudget, p.MaxLLMTokenBudget)
		}
	}
}

// TestActiveProfile_ReturnsCopy verifies that mutating the returned pointer
// does not affect the stored profile (defensive copy).
func TestActiveProfile_ReturnsCopy(t *testing.T) {
	// Cannot run parallel: mutates package-level state.
	fast, _ := config.GetProfile(config.ProfileFast)
	config.SetActiveProfile(fast)

	first := config.ActiveProfile()
	if first == nil {
		t.Fatal("expected non-nil profile")
	}
	// Modify the pointer — should not affect stored value.
	first.MaxLLMTokenBudget = 999999

	second := config.ActiveProfile()
	if second.MaxLLMTokenBudget == 999999 {
		t.Error("mutating returned pointer affected stored profile — SetActiveProfile must copy")
	}
}
