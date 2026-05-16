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

package contextbudgeter_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/teragrid/forge/internal/contextbudgeter"
)

// TestEstimateTokens validates the heuristic (4 chars ≈ 1 token).
func TestEstimateTokens_Basic(t *testing.T) {
	t.Parallel()
	cases := []struct {
		text string
		want int
	}{
		{"", 0},
		{"abcd", 1},   // exactly 4 chars → 1 token
		{"abc", 1},    // 3 chars → ceil(3/4)=1
		{"abcde", 2},  // 5 chars → 2
		{strings.Repeat("a", 100), 25},
	}
	for _, tc := range cases {
		got := contextbudgeter.EstimateTokens(tc.text)
		if got != tc.want {
			t.Errorf("EstimateTokens(%q) = %d, want %d", tc.text, got, tc.want)
		}
	}
}

// TestCheckBudget_UnderLimit passes when estimated tokens ≤ maxTokens.
func TestCheckBudget_UnderLimit(t *testing.T) {
	t.Parallel()
	if err := contextbudgeter.CheckBudget("hello", 100); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestCheckBudget_OverLimit returns error when over.
func TestCheckBudget_OverLimit(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("word ", 1000) // ~5000 chars → ~1250 tokens
	if err := contextbudgeter.CheckBudget(long, 10); err == nil {
		t.Error("expected budget error, got nil")
	}
}

// TestCheckBudget_ZeroMaxSkips bypasses check when maxTokens == 0.
func TestCheckBudget_ZeroMaxSkips(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("x", 100000)
	if err := contextbudgeter.CheckBudget(long, 0); err != nil {
		t.Errorf("CheckBudget with maxTokens=0 should skip: %v", err)
	}
}

// TestApplyBudget_HardLimitBlocks returns BudgetExceededError.
func TestApplyBudget_HardLimitBlocks(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("a", 400) // 100 tokens
	cfg := contextbudgeter.BudgetConfig{HardLimit: 10}
	_, err := contextbudgeter.ApplyBudget(long, "gpt-4", cfg)
	if err == nil {
		t.Fatal("expected BudgetExceededError")
	}
	var be *contextbudgeter.BudgetExceededError
	if !errors.As(err, &be) {
		t.Errorf("expected *BudgetExceededError, got %T: %v", err, err)
	}
	if be.HardLimit != 10 {
		t.Errorf("HardLimit = %d, want 10", be.HardLimit)
	}
}

// TestApplyBudget_SoftLimitDowngrades returns downgrade model.
func TestApplyBudget_SoftLimitDowngrades(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("a", 400) // 100 tokens
	cfg := contextbudgeter.BudgetConfig{SoftLimit: 10, DowngradeModel: "gpt-3.5-turbo"}
	model, err := contextbudgeter.ApplyBudget(long, "gpt-4", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if model != "gpt-3.5-turbo" {
		t.Errorf("expected downgraded model %q, got %q", "gpt-3.5-turbo", model)
	}
}

// TestApplyBudget_UnderBothLimits returns currentModel unchanged.
func TestApplyBudget_UnderBothLimits(t *testing.T) {
	t.Parallel()
	cfg := contextbudgeter.BudgetConfig{SoftLimit: 1000, HardLimit: 5000, DowngradeModel: "cheap"}
	model, err := contextbudgeter.ApplyBudget("short prompt", "gpt-4", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if model != "gpt-4" {
		t.Errorf("expected model %q unchanged, got %q", "gpt-4", model)
	}
}

// TestBudgetExceededError_Message formats correctly.
func TestBudgetExceededError_Message(t *testing.T) {
	t.Parallel()
	err := &contextbudgeter.BudgetExceededError{Estimated: 200, HardLimit: 100}
	msg := err.Error()
	if !strings.Contains(msg, "200") || !strings.Contains(msg, "100") {
		t.Errorf("error message missing numbers: %q", msg)
	}
}

// ── G-048: LoadBudgetConfig from forge.yaml ───────────────────────────────────

// TestLoadBudgetConfig_ParsesYAML verifies that LoadBudgetConfig reads the
// token_budget section from a forge.yaml file in the given root directory.
func TestLoadBudgetConfig_ParsesYAML(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	yaml := `
token_budget:
  soft_limit: 4000
  hard_limit: 8000
  downgrade_model: "claude-3-5-haiku-20241022"
`
	if err := os.WriteFile(filepath.Join(root, "forge.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatalf("write forge.yaml: %v", err)
	}
	cfg, err := contextbudgeter.LoadBudgetConfig(root)
	if err != nil {
		t.Fatalf("LoadBudgetConfig: %v", err)
	}
	if cfg.SoftLimit != 4000 {
		t.Errorf("SoftLimit: want 4000, got %d", cfg.SoftLimit)
	}
	if cfg.HardLimit != 8000 {
		t.Errorf("HardLimit: want 8000, got %d", cfg.HardLimit)
	}
	if cfg.DowngradeModel != "claude-3-5-haiku-20241022" {
		t.Errorf("DowngradeModel: want %q, got %q", "claude-3-5-haiku-20241022", cfg.DowngradeModel)
	}
}

// TestLoadBudgetConfig_NoFileMeansNoLimits verifies that a missing forge.yaml
// returns a zero BudgetConfig (no limits enforced).
func TestLoadBudgetConfig_NoFileMeansNoLimits(t *testing.T) {
	t.Parallel()
	root := t.TempDir() // empty dir — no forge.yaml
	cfg, err := contextbudgeter.LoadBudgetConfig(root)
	if err != nil {
		t.Fatalf("LoadBudgetConfig (no file): %v", err)
	}
	if cfg.SoftLimit != 0 || cfg.HardLimit != 0 || cfg.DowngradeModel != "" {
		t.Errorf("expected zero BudgetConfig for missing file, got %+v", cfg)
	}
}

// TestLoadBudgetConfig_MissingSection returns zero config when token_budget
// section is absent from forge.yaml.
func TestLoadBudgetConfig_MissingSection(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	yaml := `
llm:
  provider: anthropic
  model: claude-3-5-sonnet-20241022
`
	if err := os.WriteFile(filepath.Join(root, "forge.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatalf("write forge.yaml: %v", err)
	}
	cfg, err := contextbudgeter.LoadBudgetConfig(root)
	if err != nil {
		t.Fatalf("LoadBudgetConfig: %v", err)
	}
	if cfg.SoftLimit != 0 || cfg.HardLimit != 0 {
		t.Errorf("expected zero budget when section absent, got %+v", cfg)
	}
}

// TestLoadBudgetConfig_IntegrationWithApplyBudget verifies the end-to-end
// flow: load config from forge.yaml and apply it to a prompt.
func TestLoadBudgetConfig_IntegrationWithApplyBudget(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	yaml := `
token_budget:
  soft_limit: 5
  hard_limit: 50
  downgrade_model: "haiku"
`
	if err := os.WriteFile(filepath.Join(root, "forge.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatalf("write forge.yaml: %v", err)
	}
	cfg, err := contextbudgeter.LoadBudgetConfig(root)
	if err != nil {
		t.Fatalf("LoadBudgetConfig: %v", err)
	}
	// Prompt exceeds soft limit (5 tokens) → should downgrade.
	longPrompt := strings.Repeat("word ", 10) // ~50 chars → 13 tokens > 5
	model, err := contextbudgeter.ApplyBudget(longPrompt, "sonnet", cfg)
	if err != nil {
		t.Fatalf("ApplyBudget: %v", err)
	}
	if model != "haiku" {
		t.Errorf("expected downgrade to %q, got %q", "haiku", model)
	}
}
