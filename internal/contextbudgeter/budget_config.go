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

// G-048: Token-budget-as-code.
// Typed forge.yaml config for token budgets; soft budget downgrades model,
// hard budget returns BudgetExceededError.
package contextbudgeter

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// BudgetExceededError is returned when the hard token budget is reached.
type BudgetExceededError struct {
	Prompt    string
	Estimated int
	HardLimit int
}

func (e *BudgetExceededError) Error() string {
	return fmt.Sprintf("token budget exceeded: estimated %d tokens > hard limit %d",
		e.Estimated, e.HardLimit)
}

// BudgetConfig is the typed token-budget configuration from forge.yaml.
type BudgetConfig struct {
	// SoftLimit is the token count at which the model is downgraded to a
	// cheaper tier. 0 means no soft limit.
	SoftLimit int `yaml:"soft_limit"`
	// HardLimit is the token count at which the call is aborted with
	// BudgetExceededError. 0 means no hard limit.
	HardLimit int `yaml:"hard_limit"`
	// DowngradeModel is the model to switch to when SoftLimit is reached.
	DowngradeModel string `yaml:"downgrade_model"`
}

// ApplyBudget checks prompt against cfg and returns the effective model name.
// Returns BudgetExceededError if HardLimit > 0 and prompt exceeds it.
// Returns (DowngradeModel, nil) if SoftLimit > 0 and prompt exceeds it.
// Otherwise returns (currentModel, nil).
func ApplyBudget(prompt, currentModel string, cfg BudgetConfig) (string, error) {
	tokens := EstimateTokens(prompt)

	if cfg.HardLimit > 0 && tokens > cfg.HardLimit {
		return "", &BudgetExceededError{
			Prompt:    prompt,
			Estimated: tokens,
			HardLimit: cfg.HardLimit,
		}
	}

	if cfg.SoftLimit > 0 && tokens > cfg.SoftLimit && cfg.DowngradeModel != "" {
		return cfg.DowngradeModel, nil
	}

	return currentModel, nil
}

// forgeYAML is the minimal forge.yaml schema used for token-budget parsing.
type forgeYAML struct {
	TokenBudget BudgetConfig `yaml:"token_budget"`
}

// LoadBudgetConfig reads the token_budget section from forge.yaml in root.
// If the file does not exist or the section is absent, a zero BudgetConfig is
// returned (no limits enforced). Root may be empty to use the working directory.
func LoadBudgetConfig(root string) (BudgetConfig, error) {
	if root == "" {
		var err error
		root, err = os.Getwd()
		if err != nil {
			return BudgetConfig{}, fmt.Errorf("LoadBudgetConfig: getwd: %w", err)
		}
	}
	path := filepath.Join(root, "forge.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return BudgetConfig{}, nil // no file → no limits
		}
		return BudgetConfig{}, fmt.Errorf("LoadBudgetConfig: read %s: %w", path, err)
	}
	var fd forgeYAML
	if err := yaml.Unmarshal(data, &fd); err != nil {
		return BudgetConfig{}, fmt.Errorf("LoadBudgetConfig: parse %s: %w", path, err)
	}
	return fd.TokenBudget, nil
}
