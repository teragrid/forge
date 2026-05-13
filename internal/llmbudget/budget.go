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
// Package llmbudget implements the LLM spend tracker and budget enforcer
// (DEV-M3-03). Usage records are persisted at .forge/llm-budget.json.
// Spend limits are configurable and enforced before each LLM call via
// CheckLimits.
package llmbudget

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// DefaultPath is the project-relative path for budget persistence.
const DefaultPath = ".forge/llm-budget.json"

// Record captures one LLM API invocation.
type Record struct {
	Timestamp        time.Time `json:"ts"`
	Verb             string    `json:"verb"`
	Model            string    `json:"model"`
	PromptTokens     int       `json:"prompt_tokens"`
	CompletionTokens int       `json:"completion_tokens"`
	CostUSD          float64   `json:"cost_usd"`
}

// Config holds configurable spend limits. Zero means unlimited.
type Config struct {
	DailyLimitUSD   float64 `json:"daily_limit_usd"`
	MonthlyLimitUSD float64 `json:"monthly_limit_usd"`
}

// Budget persists LLM spend history and enforces configurable limits.
type Budget struct {
	APIVersion string   `json:"api_version"`
	Kind       string   `json:"kind"`
	Config     Config   `json:"config"`
	Records    []Record `json:"records"`
}

// New returns an empty Budget ready for use.
func New() *Budget {
	return &Budget{
		APIVersion: "forge.sh/v1",
		Kind:       "LLMBudget",
		Records:    []Record{},
	}
}

// Load reads the budget from path. Missing file returns New() (no error).
func Load(path string) (*Budget, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return New(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("llmbudget: read %s: %w", path, err)
	}
	var b Budget
	if err := json.Unmarshal(data, &b); err != nil {
		return nil, fmt.Errorf("llmbudget: parse %s: %w", path, err)
	}
	if b.Records == nil {
		b.Records = []Record{}
	}
	return &b, nil
}

// LoadDefault loads from filepath.Join(root, DefaultPath).
func LoadDefault(root string) (*Budget, error) {
	return Load(filepath.Join(root, DefaultPath))
}

// Save writes the budget to path (mode 0600, parent dirs created).
func (b *Budget) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("llmbudget: mkdir: %w", err)
	}
	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return fmt.Errorf("llmbudget: marshal: %w", err)
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o600)
}

// Add appends r to the record list. If r.Timestamp is zero it is set to now.
func (b *Budget) Add(r Record) {
	if r.Timestamp.IsZero() {
		r.Timestamp = time.Now().UTC()
	}
	b.Records = append(b.Records, r)
}

// DailySpend returns the total CostUSD for the calendar day of t (UTC).
func (b *Budget) DailySpend(t time.Time) float64 {
	y, mo, d := t.UTC().Date()
	var sum float64
	for _, r := range b.Records {
		ry, rmo, rd := r.Timestamp.UTC().Date()
		if ry == y && rmo == mo && rd == d {
			sum += r.CostUSD
		}
	}
	return sum
}

// MonthlySpend returns the total CostUSD for the calendar month of t (UTC).
func (b *Budget) MonthlySpend(t time.Time) float64 {
	y, mo, _ := t.UTC().Date()
	var sum float64
	for _, r := range b.Records {
		ry, rmo, _ := r.Timestamp.UTC().Date()
		if ry == y && rmo == mo {
			sum += r.CostUSD
		}
	}
	return sum
}

// CheckLimits returns an error if the existing spend meets or exceeds a
// configured limit for the day/month of t. Zero limits are unlimited.
func (b *Budget) CheckLimits(t time.Time) error {
	if b.Config.DailyLimitUSD > 0 {
		if d := b.DailySpend(t); d >= b.Config.DailyLimitUSD {
			return fmt.Errorf("llmbudget: daily spend $%.4f meets/exceeds limit $%.4f",
				d, b.Config.DailyLimitUSD)
		}
	}
	if b.Config.MonthlyLimitUSD > 0 {
		if m := b.MonthlySpend(t); m >= b.Config.MonthlyLimitUSD {
			return fmt.Errorf("llmbudget: monthly spend $%.4f meets/exceeds limit $%.4f",
				m, b.Config.MonthlyLimitUSD)
		}
	}
	return nil
}

// SetLimits updates the spend limits. Returns an error for negative values.
func (b *Budget) SetLimits(dailyUSD, monthlyUSD float64) error {
	if dailyUSD < 0 {
		return fmt.Errorf("llmbudget: daily limit must be ≥ 0, got %g", dailyUSD)
	}
	if monthlyUSD < 0 {
		return fmt.Errorf("llmbudget: monthly limit must be ≥ 0, got %g", monthlyUSD)
	}
	b.Config.DailyLimitUSD = dailyUSD
	b.Config.MonthlyLimitUSD = monthlyUSD
	return nil
}

// Reset clears all records. If resetLimits is true the Config is also zeroed.
func (b *Budget) Reset(resetLimits bool) {
	b.Records = []Record{}
	if resetLimits {
		b.Config = Config{}
	}
}
