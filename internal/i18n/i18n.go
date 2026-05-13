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

// Package i18n provides a minimal localisation scaffold for Forge (M3-11).
//
// For v1.0.0 only English (en) is supported. The design intentionally
// separates message IDs from their text so that community translators can
// contribute new locales without touching Go code.
//
// Usage:
//
//	msg := i18n.T("error.plugin.not_found", map[string]string{"name": "forge-scanner-rls"})
//	// → "plugin not found: forge-scanner-rls"
//
// The locale is resolved from FORGE_LOCALE → LC_ALL → LC_MESSAGES → LANG →
// "en" (fallback). Only the language subtag is used (e.g. "fr" from "fr-FR").
//
// Adding a new locale:
//  1. Create internal/i18n/locales/<lang>.json with the same keys as en.json.
//  2. Add the file to the //go:embed directive below.
//  3. Ship — no code changes required.
package i18n

import (
	"embed"
	"encoding/json"
	"os"
	"strings"
	"sync"
)

//go:embed locales/*.json
var localeFS embed.FS

// catalog holds all loaded locale catalogs (locale → key → template).
var (
	catalogMu sync.RWMutex
	catalog   = map[string]map[string]string{}
	loaded    = map[string]bool{}
)

// T returns the localised message for key in the active locale.
// vars is substituted into the message template using {{.Key}} syntax.
//
// If key is missing in the active locale, T falls back to "en". If it is
// missing in "en" too, the raw key is returned so errors are always visible.
func T(key string, vars map[string]string) string {
	locale := activeLocale()
	msg := lookup(locale, key)
	if msg == "" && locale != "en" {
		msg = lookup("en", key)
	}
	if msg == "" {
		return key
	}
	return interpolate(msg, vars)
}

// Locale returns the currently active locale tag (e.g. "en", "fr").
func Locale() string { return activeLocale() }

// ── Internal helpers ──────────────────────────────────────────────────────────

func activeLocale() string {
	for _, env := range []string{"FORGE_LOCALE", "LC_ALL", "LC_MESSAGES", "LANG"} {
		if v := os.Getenv(env); v != "" {
			// Take only the primary language subtag.
			v = strings.ToLower(strings.FieldsFunc(v, func(r rune) bool {
				return r == '_' || r == '-' || r == '.'
			})[0])
			return v
		}
	}
	return "en"
}

// lookup returns the message for key in locale, loading the locale file on
// first access.
func lookup(locale, key string) string {
	ensure(locale)
	catalogMu.RLock()
	defer catalogMu.RUnlock()
	if m, ok := catalog[locale]; ok {
		return m[key]
	}
	return ""
}

// ensure loads a locale catalog if not already loaded.
func ensure(locale string) {
	catalogMu.Lock()
	defer catalogMu.Unlock()
	if loaded[locale] {
		return
	}
	loaded[locale] = true // mark even on error so we don't retry constantly
	data, err := localeFS.ReadFile("locales/" + locale + ".json")
	if err != nil {
		return // locale file missing; fallback will cover it
	}
	var m map[string]string
	if err := json.Unmarshal(data, &m); err != nil {
		return
	}
	catalog[locale] = m
}

// interpolate replaces {{Key}} placeholders in msg with vars[Key].
func interpolate(msg string, vars map[string]string) string {
	if len(vars) == 0 {
		return msg
	}
	for k, v := range vars {
		msg = strings.ReplaceAll(msg, "{{"+k+"}}", v)
	}
	return msg
}
