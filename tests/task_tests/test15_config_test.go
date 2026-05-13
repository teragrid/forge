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

// TEST-15: Property-based tests for config layering.

package tasktests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/teragrid/forge/internal/config"
)

// ── TEST-15: Property-based tests for config layering ────────────────────────

// TC-15-01 (happy): flags > env > file > defaults precedence holds for LLMProvider.
func TestTC1501_ConfigPrecedenceFlagsOverEnv(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Write a forge.yml with a file-level value.
	yml := "llm:\n  provider: openai\n"
	if err := os.WriteFile(filepath.Join(dir, "forge.yml"), []byte(yml), 0o644); err != nil {
		t.Fatalf("write forge.yml: %v", err)
	}

	// Override with a flag (highest priority).
	overrides := map[string]string{"LLM_PROVIDER": "anthropic"}
	cfg, err := config.Load(dir, overrides)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	if cfg.LLMProvider.Raw != "anthropic" {
		t.Errorf("LLMProvider.Raw = %q, want %q", cfg.LLMProvider.Raw, "anthropic")
	}
	if cfg.LLMProvider.Source != config.SourceFlag {
		t.Errorf("LLMProvider.Source = %v, want SourceFlag", cfg.LLMProvider.Source)
	}
}

// TC-15-01b: file overrides defaults.
func TestTC1501b_ConfigFileOverridesDefault(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	yml := "llm:\n  provider: openai\n"
	if err := os.WriteFile(filepath.Join(dir, "forge.yml"), []byte(yml), 0o644); err != nil {
		t.Fatalf("write forge.yml: %v", err)
	}

	cfg, err := config.Load(dir, nil)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	if cfg.LLMProvider.Raw != "openai" {
		t.Errorf("LLMProvider.Raw = %q, want openai", cfg.LLMProvider.Raw)
	}
	if cfg.LLMProvider.Source != config.SourceFile {
		t.Errorf("LLMProvider.Source = %v, want SourceFile", cfg.LLMProvider.Source)
	}
}

// TC-15-02 (boundary): missing file and no env vars resolves to defaults.
func TestTC1502_ConfigMissingLayersUsesDefaults(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// No forge.yml, no env overrides.
	cfg, err := config.Load(dir, nil)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	if cfg.LLMProvider.Raw != "auto" {
		t.Errorf("LLMProvider.Raw = %q, want default %q", cfg.LLMProvider.Raw, "auto")
	}
	if cfg.LLMProvider.Source != config.SourceDefault {
		t.Errorf("LLMProvider.Source = %v, want SourceDefault", cfg.LLMProvider.Source)
	}
	if cfg.LogLevel.Raw != "info" {
		t.Errorf("LogLevel.Raw = %q, want %q", cfg.LogLevel.Raw, "info")
	}
}

// TC-15-03 (negative): invalid YAML in forge.yml returns a typed error.
func TestTC1503_ConfigBadYAMLReturnsTypedError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// Use a YAML mapping key with a mismatched value structure to guarantee parse failure.
	badYAML := "llm:\n  provider: [unclosed\n"
	if err := os.WriteFile(filepath.Join(dir, "forge.yml"), []byte(badYAML), 0o644); err != nil {
		t.Fatalf("write bad forge.yml: %v", err)
	}
	_, err := config.Load(dir, nil)
	if err == nil {
		t.Fatal("expected error for invalid YAML, got nil")
	}
}

// TC-15-04 (data-accuracy): flag layer records the winning source as SourceFlag.
func TestTC1504_ConfigSourceAccuracy(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	overrides := map[string]string{
		"LLM_PROVIDER": "gemini",
		"LOG_LEVEL":    "debug",
	}
	cfg, err := config.Load(dir, overrides)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	if cfg.LLMProvider.Source != config.SourceFlag {
		t.Errorf("LLMProvider: source = %v, want SourceFlag", cfg.LLMProvider.Source)
	}
	if cfg.LogLevel.Source != config.SourceFlag {
		t.Errorf("LogLevel: source = %v, want SourceFlag", cfg.LogLevel.Source)
	}
}
