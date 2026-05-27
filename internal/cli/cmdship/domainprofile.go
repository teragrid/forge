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

// domainprofile.go — P1: Domain Profile loader and built-in profiles.
//
// A Domain Profile is a YAML file at .forge/domains/<name>.yml that
// customises ship pipeline behaviour for a specific industry or domain
// (e.g. banking, healthcare, saas-b2b).
//
// Load order:
//  1. .forge/domains/<name>.yml  (project-local override)
//  2. Built-in profiles embedded in the binary (default, banking, …)
//  3. defaultProfile as final fallback
//
// Inheritance: a profile may set "extends: <parent>" to inherit parent
// checkpoint overrides.  The child wins on every explicit field; the parent
// fills in any absent ones.
package cmdship

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// DomainProfile defines domain-specific overrides for the ship pipeline.
type DomainProfile struct {
	Profile     string                `yaml:"profile"`
	Extends     string                `yaml:"extends,omitempty"`
	Description string                `yaml:"description,omitempty"`
	Checkpoints map[string]CPOverride `yaml:"checkpoints,omitempty"`
	LLM         DomainLLMPolicy       `yaml:"llm,omitempty"`
}

// CPOverride holds per-checkpoint overrides defined by a domain profile.
type CPOverride struct {
	// BudgetMultiplier scales the base maxTokens for this checkpoint.
	// 0 means "use the complexity-tier multiplier only (no domain override)".
	BudgetMultiplier float64  `yaml:"budget_multiplier,omitempty"`
	ExtraRoles       []string `yaml:"extra_roles,omitempty"`
	Steerings        []string `yaml:"steerings,omitempty"`
}

// DomainLLMPolicy specifies provider and data-handling constraints.
type DomainLLMPolicy struct {
	AllowedProviders []string `yaml:"allowed_providers,omitempty"`
	PIIDetection     bool     `yaml:"pii_detection,omitempty"`
}

// defaultProfile is the zero-override baseline returned when name is "" or unknown.
var defaultProfile = DomainProfile{
	Profile:     "default",
	Description: "Standard forge ship pipeline — no domain-specific overrides.",
}

// builtinProfiles contains the profiles shipped with the forge binary.
var builtinProfiles = map[string]DomainProfile{
	"default": defaultProfile,

	"banking": {
		Profile:     "banking",
		Extends:     "default",
		Description: "PCI-DSS, 4-eyes approval, EU data residency enforcement.",
		Checkpoints: map[string]CPOverride{
			"arch": {BudgetMultiplier: 2.0, Steerings: []string{"pci-dss-controls", "data-residency-eu"}},
			"ship": {BudgetMultiplier: 1.5, Steerings: []string{"pci-dss-controls"}},
		},
		LLM: DomainLLMPolicy{PIIDetection: true, AllowedProviders: []string{"azure-openai"}},
	},

	"healthcare": {
		Profile:     "healthcare",
		Extends:     "default",
		Description: "HIPAA PHI handling, audit immutability, consent management.",
		Checkpoints: map[string]CPOverride{
			"spec": {BudgetMultiplier: 1.5, Steerings: []string{"hipaa-phi-handling"}},
			"ship": {BudgetMultiplier: 1.5, Steerings: []string{"hipaa-audit-trail"}},
		},
		LLM: DomainLLMPolicy{PIIDetection: true},
	},

	"saas-b2b": {
		Profile:     "saas-b2b",
		Extends:     "default",
		Description: "Multi-tenancy isolation, tenant data segregation, RLS enforcement.",
		Checkpoints: map[string]CPOverride{
			"arch": {Steerings: []string{"tenant-isolation", "multi-tenancy-rls"}},
			"code": {Steerings: []string{"tenant-isolation"}},
		},
	},

	"data-heavy": {
		Profile:     "data-heavy",
		Extends:     "default",
		Description: "DB migration gates, schema evolution tracking, data pipeline patterns.",
		Checkpoints: map[string]CPOverride{
			"arch": {BudgetMultiplier: 1.5, Steerings: []string{"schema-migration", "db-pattern"}},
		},
	},

	"microservice": {
		Profile:     "microservice",
		Extends:     "default",
		Description: "API contract gates, backward-compat checks, service mesh readiness.",
		Checkpoints: map[string]CPOverride{
			"arch": {Steerings: []string{"api-versioning", "backward-compat"}},
			"ship": {Steerings: []string{"api-contract-check"}},
		},
	},
}

// LoadDomainProfile loads the domain profile for name.
// Returns the default profile when name is "" or no matching profile is found.
func LoadDomainProfile(root, name string) DomainProfile {
	if name == "" {
		name = "default"
	}
	// 1. Project-local file.
	path := filepath.Join(root, ".forge", "domains", name+".yml")
	if data, err := os.ReadFile(path); err == nil {
		var p DomainProfile
		if yaml.Unmarshal(data, &p) == nil && p.Profile != "" {
			return mergeDomainProfile(root, p)
		}
	}
	// 2. Built-in profiles.
	if p, ok := builtinProfiles[strings.ToLower(name)]; ok {
		return p
	}
	return defaultProfile
}

// CheckpointBudgetMultiplier returns the effective budget multiplier for the
// given checkpoint, combining the domain-profile override with the
// complexity-tier multiplier.  The domain profile wins when set explicitly
// (> 0); otherwise the complexity-tier multiplier is returned unchanged.
func (p DomainProfile) CheckpointBudgetMultiplier(checkpoint string, complexityMul float64) float64 {
	if ovr, ok := p.Checkpoints[checkpoint]; ok && ovr.BudgetMultiplier > 0 {
		return ovr.BudgetMultiplier
	}
	return complexityMul
}

// CheckpointSteerings returns extra steering names from the profile for the
// given checkpoint (nil when none are defined).
func (p DomainProfile) CheckpointSteerings(checkpoint string) []string {
	if ovr, ok := p.Checkpoints[checkpoint]; ok {
		return ovr.Steerings
	}
	return nil
}

// AvailableProfiles returns the sorted list of built-in profile names.
func AvailableProfiles() []string {
	names := make([]string, 0, len(builtinProfiles))
	for k := range builtinProfiles {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

// mergeDomainProfile applies the Extends inheritance chain (depth-1) so the
// child profile inherits parent checkpoint overrides and LLM policies wherever
// it has no explicit override of its own.
func mergeDomainProfile(root string, child DomainProfile) DomainProfile {
	if child.Extends == "" || child.Extends == child.Profile {
		return child
	}
	parent := LoadDomainProfile(root, child.Extends)
	if child.Checkpoints == nil {
		child.Checkpoints = make(map[string]CPOverride)
	}
	for k, v := range parent.Checkpoints {
		if _, exists := child.Checkpoints[k]; !exists {
			child.Checkpoints[k] = v
		}
	}
	if !child.LLM.PIIDetection {
		child.LLM.PIIDetection = parent.LLM.PIIDetection
	}
	if len(child.LLM.AllowedProviders) == 0 {
		child.LLM.AllowedProviders = parent.LLM.AllowedProviders
	}
	return child
}
