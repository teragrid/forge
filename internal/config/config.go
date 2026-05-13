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

// Package config implements the layered configuration loader for forge (DEV-M0-02).
//
// Resolution order (lowest to highest priority):
//
//  1. Built-in defaults (hard-coded in this package).
//  2. Project file — `forge.yml` in the project root (or the directory specified
//     by the FORGE_CONFIG env var).
//  3. Environment variables — `FORGE_<SECTION>_<KEY>` (e.g. FORGE_LLM_PROVIDER).
//  4. Caller-supplied overrides (for CLI flag integration).
//
// Each resolved key tracks its winning [Source] so `forge config explain` can
// report where each value came from.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/teragrid/forge/internal/errcode"
)

// Reserved error codes (range 2000..2099).
var (
	ErrConfigParse   = errcode.Register(errcode.Code(2000), "forge.yml parse error")
	ErrConfigBadKey  = errcode.Register(errcode.Code(2001), "unknown config key")
	ErrConfigBadType = errcode.Register(errcode.Code(2002), "config value type mismatch")
)

// Source identifies where a configuration value came from.
type Source int

const (
	SourceDefault Source = iota
	SourceFile
	SourceEnv
	SourceFlag
)

func (s Source) String() string {
	switch s {
	case SourceFile:
		return "file"
	case SourceEnv:
		return "env"
	case SourceFlag:
		return "flag"
	default:
		return "default"
	}
}

// Value holds a resolved config value plus its provenance.
type Value struct {
	Raw    string
	Source Source
	// SourceDetail is a human-readable note (e.g. the env-var name or file path).
	SourceDetail string
}

// Config is the resolved, layered configuration for forge.
type Config struct {
	// LLM settings.
	LLMProvider         Value // "anthropic" | "openai" | "auto"
	LLMModel            Value // model identifier
	LLMDailyBudgetUSD   Value // numeric string
	LLMMonthlyBudgetUSD Value // numeric string

	// Telemetry.
	TelemetryEnabled   Value // "true" | "false"
	TelemetryInstallID Value // UUID string

	// Logging.
	LogFormat Value // "json" | "text" | "auto"
	LogLevel  Value // "debug" | "info" | "warn" | "error"

	// Internal — path from which the file layer was loaded.
	FilePath string
}

// fileData mirrors the forge.yml schema for YAML unmarshalling.
type fileData struct {
	LLM struct {
		Provider         string  `yaml:"provider"`
		Model            string  `yaml:"model"`
		DailyBudgetUSD   float64 `yaml:"daily_budget_usd"`
		MonthlyBudgetUSD float64 `yaml:"monthly_budget_usd"`
	} `yaml:"llm"`
	Telemetry struct {
		Enabled   *bool  `yaml:"enabled"`
		InstallID string `yaml:"install_id"`
	} `yaml:"telemetry"`
	Log struct {
		Format string `yaml:"format"`
		Level  string `yaml:"level"`
	} `yaml:"log"`
}

// defaults returns the built-in default values.
func defaults() *Config {
	mk := func(v string) Value { return Value{Raw: v, Source: SourceDefault, SourceDetail: "built-in"} }
	return &Config{
		LLMProvider:         mk("auto"),
		LLMModel:            mk(""),
		LLMDailyBudgetUSD:   mk("0"),
		LLMMonthlyBudgetUSD: mk("0"),
		TelemetryEnabled:    mk("false"),
		TelemetryInstallID:  mk(""),
		LogFormat:           mk("auto"),
		LogLevel:            mk("info"),
	}
}

// DefaultFilePath returns the canonical forge.yml path for the given project root.
func DefaultFilePath(root string) string {
	return filepath.Join(root, "forge.yml")
}

// Load resolves the full config stack. root is the project directory.
// overrides map keys follow the same naming as env vars (e.g. "LLM_PROVIDER").
func Load(root string, overrides map[string]string) (*Config, error) {
	c := defaults()

	// Layer 2 — file.
	filePath := DefaultFilePath(root)
	if env := os.Getenv("FORGE_CONFIG"); env != "" {
		filePath = env
	}
	c.FilePath = filePath

	if err := applyFile(c, filePath); err != nil {
		return nil, err
	}

	// Layer 3 — environment.
	applyEnv(c)

	// Layer 4 — caller overrides (flags).
	if len(overrides) > 0 {
		if err := applyOverrides(c, overrides); err != nil {
			return nil, err
		}
	}

	return c, nil
}

// applyFile reads forge.yml (if it exists) and promotes matching fields.
func applyFile(c *Config, path string) error {
	b, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		if os.IsNotExist(err) {
			return nil // missing file is fine — use defaults
		}
		return errcode.New(ErrConfigParse, "read config file", err)
	}
	var fd fileData
	if err := yaml.Unmarshal(b, &fd); err != nil {
		return errcode.Newf(ErrConfigParse, err, "parse %s", path)
	}
	detail := fmt.Sprintf("file:%s", path)
	set := func(v *Value, raw string) {
		if raw != "" {
			*v = Value{Raw: raw, Source: SourceFile, SourceDetail: detail}
		}
	}
	setF := func(v *Value, f float64) {
		if f != 0 {
			*v = Value{Raw: strconv.FormatFloat(f, 'f', -1, 64), Source: SourceFile, SourceDetail: detail}
		}
	}
	set(&c.LLMProvider, fd.LLM.Provider)
	set(&c.LLMModel, fd.LLM.Model)
	setF(&c.LLMDailyBudgetUSD, fd.LLM.DailyBudgetUSD)
	setF(&c.LLMMonthlyBudgetUSD, fd.LLM.MonthlyBudgetUSD)
	if fd.Telemetry.Enabled != nil {
		val := "false"
		if *fd.Telemetry.Enabled {
			val = "true"
		}
		c.TelemetryEnabled = Value{Raw: val, Source: SourceFile, SourceDetail: detail}
	}
	set(&c.TelemetryInstallID, fd.Telemetry.InstallID)
	set(&c.LogFormat, fd.Log.Format)
	set(&c.LogLevel, fd.Log.Level)
	return nil
}

// applyEnv overrides from FORGE_* environment variables.
func applyEnv(c *Config) {
	envMap := []struct {
		envKey string
		field  *Value
	}{
		{"FORGE_LLM_PROVIDER", &c.LLMProvider},
		{"FORGE_LLM_MODEL", &c.LLMModel},
		{"FORGE_LLM_DAILY_BUDGET_USD", &c.LLMDailyBudgetUSD},
		{"FORGE_LLM_MONTHLY_BUDGET_USD", &c.LLMMonthlyBudgetUSD},
		{"FORGE_TELEMETRY_ENABLED", &c.TelemetryEnabled},
		{"FORGE_TELEMETRY_INSTALL_ID", &c.TelemetryInstallID},
		{"FORGE_LOG_FORMAT", &c.LogFormat},
		{"FORGE_LOG_LEVEL", &c.LogLevel},
	}
	for _, e := range envMap {
		if v := os.Getenv(e.envKey); v != "" {
			*e.field = Value{Raw: v, Source: SourceEnv, SourceDetail: fmt.Sprintf("env:%s", e.envKey)}
		}
	}
}

// applyOverrides applies flag-level values. Keys use the env suffix convention
// (e.g. "LLM_PROVIDER" sets LLMProvider).
func applyOverrides(c *Config, ov map[string]string) error {
	fieldMap := map[string]*Value{
		"LLM_PROVIDER":           &c.LLMProvider,
		"LLM_MODEL":              &c.LLMModel,
		"LLM_DAILY_BUDGET_USD":   &c.LLMDailyBudgetUSD,
		"LLM_MONTHLY_BUDGET_USD": &c.LLMMonthlyBudgetUSD,
		"TELEMETRY_ENABLED":      &c.TelemetryEnabled,
		"TELEMETRY_INSTALL_ID":   &c.TelemetryInstallID,
		"LOG_FORMAT":             &c.LogFormat,
		"LOG_LEVEL":              &c.LogLevel,
	}
	for k, v := range ov {
		field, ok := fieldMap[strings.ToUpper(k)]
		if !ok {
			return errcode.Newf(ErrConfigBadKey, nil, "unknown config key %q", k)
		}
		*field = Value{Raw: v, Source: SourceFlag, SourceDetail: fmt.Sprintf("flag:--%s", strings.ToLower(k))}
	}
	return nil
}

// AllFields returns a snapshot of all config fields as a flat list of
// (key, Value) pairs, sorted alphabetically. Used by `forge config show`.
func (c *Config) AllFields() []Field {
	return []Field{
		{Key: "llm.daily_budget_usd", Value: c.LLMDailyBudgetUSD},
		{Key: "llm.model", Value: c.LLMModel},
		{Key: "llm.monthly_budget_usd", Value: c.LLMMonthlyBudgetUSD},
		{Key: "llm.provider", Value: c.LLMProvider},
		{Key: "log.format", Value: c.LogFormat},
		{Key: "log.level", Value: c.LogLevel},
		{Key: "telemetry.enabled", Value: c.TelemetryEnabled},
		{Key: "telemetry.install_id", Value: c.TelemetryInstallID},
	}
}

// Field is one entry in the flat config view.
type Field struct {
	Key   string
	Value Value
}

// Get returns the Value for a dotted key (e.g. "llm.provider"). Returns an
// error if the key is not recognised.
func (c *Config) Get(key string) (Value, error) {
	for _, f := range c.AllFields() {
		if f.Key == strings.ToLower(key) {
			return f.Value, nil
		}
	}
	return Value{}, errcode.Newf(ErrConfigBadKey, nil, "unknown config key %q", key)
}
