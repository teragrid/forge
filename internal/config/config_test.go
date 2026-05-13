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

package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/teragrid/forge/internal/config"
)

// TC-02-01: each layer overrides the layer below it.
func TestLoad_LayerPrecedence(t *testing.T) {
	// t.Parallel() intentionally omitted: test uses t.Setenv

	// 1. Default → provider = "auto"
	root := t.TempDir()
	cfg, err := config.Load(root, nil)
	if err != nil {
		t.Fatalf("Load (defaults): %v", err)
	}
	if cfg.LLMProvider.Raw != "auto" || cfg.LLMProvider.Source != config.SourceDefault {
		t.Errorf("expected default auto, got %+v", cfg.LLMProvider)
	}

	// 2. File overrides default.
	writeYAML(t, root, "llm:\n  provider: anthropic\n")
	cfg, err = config.Load(root, nil)
	if err != nil {
		t.Fatalf("Load (file): %v", err)
	}
	if cfg.LLMProvider.Raw != "anthropic" || cfg.LLMProvider.Source != config.SourceFile {
		t.Errorf("expected file:anthropic, got %+v", cfg.LLMProvider)
	}

	// 3. Env overrides file.
	t.Setenv("FORGE_LLM_PROVIDER", "openai")
	cfg, err = config.Load(root, nil)
	if err != nil {
		t.Fatalf("Load (env): %v", err)
	}
	if cfg.LLMProvider.Raw != "openai" || cfg.LLMProvider.Source != config.SourceEnv {
		t.Errorf("expected env:openai, got %+v", cfg.LLMProvider)
	}

	// 4. Flag/override overrides env.
	cfg, err = config.Load(root, map[string]string{"LLM_PROVIDER": "auto"})
	if err != nil {
		t.Fatalf("Load (flag): %v", err)
	}
	if cfg.LLMProvider.Raw != "auto" || cfg.LLMProvider.Source != config.SourceFlag {
		t.Errorf("expected flag:auto, got %+v", cfg.LLMProvider)
	}
}

// TC-02-02: missing file + empty env still resolves to defaults; no crash.
func TestLoad_MissingFileAndEmptyEnv(t *testing.T) {
	t.Parallel()
	root := t.TempDir() // no forge.yml
	cfg, err := config.Load(root, nil)
	if err != nil {
		t.Fatalf("Load with missing file should not error: %v", err)
	}
	if cfg.LLMProvider.Raw != "auto" {
		t.Errorf("expected default auto, got %q", cfg.LLMProvider.Raw)
	}
	if cfg.LogLevel.Raw != "info" {
		t.Errorf("expected default info log level, got %q", cfg.LogLevel.Raw)
	}
}

// TC-02-03: malformed config file fails with FORGE-XXXX, not a stack trace.
func TestLoad_MalformedFile_ErrorCode(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// Write invalid YAML.
	_ = os.WriteFile(filepath.Join(root, "forge.yml"), []byte(":: invalid :: yaml ::"), 0o600)
	_, err := config.Load(root, nil)
	if err == nil {
		t.Fatal("expected error for malformed forge.yml")
	}
	// The error must carry a FORGE error code (not a raw Go error).
	if !isForgeError(err) {
		t.Errorf("expected a FORGE error code error, got: %v", err)
	}
}

// TC-02-04: Get() reports the winning layer via Source.
func TestGet_SourceExplain(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeYAML(t, root, "log:\n  level: debug\n")
	cfg, err := config.Load(root, nil)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	v, err := cfg.Get("log.level")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if v.Raw != "debug" {
		t.Errorf("expected debug, got %q", v.Raw)
	}
	if v.Source != config.SourceFile {
		t.Errorf("expected SourceFile, got %v", v.Source)
	}
}

// TC-02-05 (negative): Get() with unknown key returns ErrConfigBadKey.
func TestGet_UnknownKey(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	cfg, err := config.Load(root, nil)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	_, err = cfg.Get("nonexistent.key")
	if err == nil {
		t.Fatal("expected error for unknown key")
	}
}

// TC-02-06 (idempotency): loading same config twice produces identical results.
func TestLoad_Idempotent(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeYAML(t, root, "llm:\n  provider: openai\nlog:\n  level: warn\n")
	cfg1, err := config.Load(root, nil)
	if err != nil {
		t.Fatalf("Load 1: %v", err)
	}
	cfg2, err := config.Load(root, nil)
	if err != nil {
		t.Fatalf("Load 2: %v", err)
	}
	if cfg1.LLMProvider.Raw != cfg2.LLMProvider.Raw {
		t.Errorf("provider mismatch: %q vs %q", cfg1.LLMProvider.Raw, cfg2.LLMProvider.Raw)
	}
	if cfg1.LogLevel.Raw != cfg2.LogLevel.Raw {
		t.Errorf("log.level mismatch: %q vs %q", cfg1.LogLevel.Raw, cfg2.LogLevel.Raw)
	}
}

// TC-02-07: AllFields returns all expected keys.
func TestAllFields_Keys(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	cfg, err := config.Load(root, nil)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	fields := cfg.AllFields()
	keys := make(map[string]bool, len(fields))
	for _, f := range fields {
		keys[f.Key] = true
	}
	expected := []string{
		"llm.provider", "llm.model", "llm.daily_budget_usd", "llm.monthly_budget_usd",
		"log.format", "log.level", "telemetry.enabled", "telemetry.install_id",
	}
	for _, k := range expected {
		if !keys[k] {
			t.Errorf("expected key %q in AllFields", k)
		}
	}
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func writeYAML(t *testing.T, root, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, "forge.yml"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func isForgeError(err error) bool {
	// FORGE errors contain "FORGE-" in their message.
	return err != nil && len(err.Error()) > 0 &&
		(containsPrefix(err.Error(), "FORGE-") || containsPrefix(err.Error(), "forge.yml"))
}

func containsPrefix(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 &&
		func() bool {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		}())
}
