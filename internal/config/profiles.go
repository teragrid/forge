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

// Package config — G-083: profile support.
//
// Profiles adjust global confidence thresholds, scan strictness, and LLM cost
// ceilings without requiring the user to set individual flags. Three built-in
// profiles are defined:
//
//   - fast     — maximises speed; uses T0 tier, skips medium-confidence checks
//   - safe     — balanced default; uses T1 tier, includes medium-confidence checks
//   - paranoid — maximises safety; uses T2 tier, applies all checks, max token budget
package config

import (
	"fmt"
)

// ProfileName identifies a configuration profile.
type ProfileName string

const (
	ProfileFast     ProfileName = "fast"
	ProfileSafe     ProfileName = "safe"
	ProfileParanoid ProfileName = "paranoid"
)

// Profile holds the settings driven by a named profile.
type Profile struct {
	Name                 ProfileName
	MinTier              string // T0, T1, T2
	IncludeMedium        bool   // include medium-confidence scan findings
	MaxLLMTokenBudget    int    // maximum tokens for a single LLM call (0 = provider default)
	ScanStrictness       string // "low", "medium", "high"
	ConfidenceThreshold  string // "low", "medium", "high" — minimum confidence to report
}

// builtinProfiles is the canonical profile table.
var builtinProfiles = map[ProfileName]Profile{
	ProfileFast: {
		Name:                ProfileFast,
		MinTier:             "T0",
		IncludeMedium:       false,
		MaxLLMTokenBudget:   4096,
		ScanStrictness:      "low",
		ConfidenceThreshold: "high",
	},
	ProfileSafe: {
		Name:                ProfileSafe,
		MinTier:             "T1",
		IncludeMedium:       true,
		MaxLLMTokenBudget:   8192,
		ScanStrictness:      "medium",
		ConfidenceThreshold: "medium",
	},
	ProfileParanoid: {
		Name:                ProfileParanoid,
		MinTier:             "T2",
		IncludeMedium:       true,
		MaxLLMTokenBudget:   32768,
		ScanStrictness:      "high",
		ConfidenceThreshold: "low",
	},
}

// GetProfile returns the built-in profile for name, or an error if unknown.
func GetProfile(name ProfileName) (Profile, error) {
	if p, ok := builtinProfiles[name]; ok {
		return p, nil
	}
	return Profile{}, fmt.Errorf("unknown profile %q; valid: fast, safe, paranoid", name)
}

// ProfileNames returns the list of valid profile names.
func ProfileNames() []ProfileName {
	return []ProfileName{ProfileFast, ProfileSafe, ProfileParanoid}
}
