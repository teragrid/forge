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

// Package cmdscan — G-021/G-022/G-023 scan extensions.
//
// G-021: three scan modes — report (default), suggest, apply.
// G-022: Confidence field (high|medium|low) on Finding + confidence filtering.
// G-023: scan history written to .forge/scan-history/<timestamp>-<family>.json;
//
//	--since <ref> loads baseline and diffs against current run.
package cmdscan

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ScanMode controls what action the scanner takes with its findings.
type ScanMode string

const (
	// ModeReport is the default mode — findings are printed to stdout but no
	// automated changes are made. Safe to use in CI.
	ModeReport ScanMode = "report"
	// ModeSuggest writes a unified diff of proposed fixes to stdout. Does not
	// modify any files.
	ModeSuggest ScanMode = "suggest"
	// ModeApply applies fixes automatically. Only findings with Confidence=="high"
	// are applied by default; --include-medium widens the gate.
	ModeApply ScanMode = "apply"
)

// Confidence level of a finding — used by G-022.
type Confidence string

const (
	ConfidenceHigh   Confidence = "high"
	ConfidenceMedium Confidence = "medium"
	ConfidenceLow    Confidence = "low"
)

// ScanHistoryEntry is one persisted scan record in .forge/scan-history/.
type ScanHistoryEntry struct {
	SchemaVersion int       `json:"schema_version"`
	Family        string    `json:"family"`
	CapturedAt    string    `json:"captured_at"`
	Findings      []Finding `json:"findings"`
	Count         int       `json:"count"`
	Status        string    `json:"status"`
}

// ── G-021: three-mode output ─────────────────────────────────────────────────

// ApplyMode dispatches output based on the mode, then optionally persists history.
// family is the scanner name (e.g. "security"). root is the project root.
func ApplyMode(mode ScanMode, family, root string, res *ScanResult, includeMedium bool) error {
	switch mode {
	case ModeReport:
		// Handled by the caller (renderText / JSON).
	case ModeSuggest:
		renderSuggestions(family, res)
	case ModeApply:
		return applyFixes(family, root, res, includeMedium)
	}
	return nil
}

// renderSuggestions prints a unified-diff style suggestion for each finding
// without modifying any files.
func renderSuggestions(family string, res *ScanResult) {
	if len(res.Findings) == 0 {
		fmt.Printf("forge scan %s (suggest): no findings to suggest fixes for.\n", family)
		return
	}
	fmt.Printf("# forge scan %s -- suggested fixes (%d finding(s))\n\n", family, len(res.Findings))
	for _, f := range res.Findings {
		conf := f.Confidence
		if conf == "" {
			conf = "medium"
		}
		fmt.Printf("--- a/%s\n+++ b/%s\n", f.File, f.File)
		if f.Line > 0 {
			fmt.Printf("@@ -%d,1 +%d,1 @@\n", f.Line, f.Line)
		}
		fmt.Printf("-  %s\n", f.Match)
		fmt.Printf("+  # TODO: fix rule %s [%s confidence]\n\n", f.Rule, conf)
	}
}

// applyFixes attempts to automatically remediate findings.
// Only high-confidence findings are applied unless includeMedium is true.
// Currently applies only safe automated fixes (e.g. removing hardcoded secrets
// with a redacted placeholder). Each change is logged to stdout.
func applyFixes(family, root string, res *ScanResult, includeMedium bool) error {
	applied := 0
	for _, f := range res.Findings {
		conf := Confidence(f.Confidence)
		if conf == "" {
			conf = ConfidenceMedium
		}
		if conf == ConfidenceLow {
			continue
		}
		if conf == ConfidenceMedium && !includeMedium {
			continue
		}
		// Safe automatic fix: replace hardcoded secret lines with a redacted placeholder.
		if err := redactFindingInFile(root, f); err != nil {
			fmt.Fprintf(os.Stderr, "  apply: %s:%d — could not auto-fix: %v\n", f.File, f.Line, err)
		} else {
			fmt.Printf("  applied: %s:%d — %s [%s]\n", f.File, f.Line, f.Rule, conf)
			applied++
		}
	}
	fmt.Printf("forge scan %s (apply): %d fix(es) applied.\n", family, applied)
	return nil
}

// redactFindingInFile replaces the matched line in a file with a safe redacted placeholder.
func redactFindingInFile(root string, f Finding) error {
	absPath := f.File
	if !filepath.IsAbs(absPath) {
		absPath = filepath.Join(root, f.File)
	}
	data, err := os.ReadFile(absPath)
	if err != nil {
		return err
	}
	lines := strings.Split(string(data), "\n")
	if f.Line < 1 || f.Line > len(lines) {
		return fmt.Errorf("line %d out of range", f.Line)
	}
	origLine := lines[f.Line-1]
	if !strings.Contains(origLine, f.Match) {
		return fmt.Errorf("match %q not found in line %d", f.Match, f.Line)
	}
	lines[f.Line-1] = strings.ReplaceAll(origLine, f.Match,
		fmt.Sprintf("/* FORGE-REDACTED: %s */", f.Rule))
	return os.WriteFile(absPath, []byte(strings.Join(lines, "\n")), 0o600)
}

// ── G-022: confidence assignment helpers ─────────────────────────────────────

// AssignConfidence sets the Confidence field on each finding based on the
// specificity of the pattern match. This is called by each scanner after
// building its findings list.
//
// Heuristic:
//   - high   — exact keyword match (e.g. "password=", "sk_live_") AND no test file path
//   - medium — pattern match in a non-test file but ambiguous context
//   - low    — pattern match in a test file, fixture, or README
func AssignConfidence(findings []Finding) []Finding {
	for i := range findings {
		findings[i].Confidence = string(confidenceFor(findings[i]))
	}
	return findings
}

func confidenceFor(f Finding) Confidence {
	file := strings.ToLower(f.File)
	// Low-confidence: test fixtures, docs, scripts.
	if strings.Contains(file, "_test.") ||
		strings.Contains(file, "test/") ||
		strings.Contains(file, "fixture") ||
		strings.Contains(file, "example") ||
		strings.HasSuffix(file, ".md") ||
		strings.HasSuffix(file, ".txt") {
		return ConfidenceLow
	}
	// High-confidence indicators in rule or match.
	hi := []string{
		"PRIVATE KEY", "sk_live_", "sk_test_", "AWS_SECRET",
		"password=", "passwd=", "api_key=", "bearer ",
	}
	matchLower := strings.ToLower(f.Match)
	ruleLower := strings.ToLower(f.Rule)
	for _, h := range hi {
		if strings.Contains(matchLower, strings.ToLower(h)) ||
			strings.Contains(ruleLower, strings.ToLower(h)) {
			return ConfidenceHigh
		}
	}
	return ConfidenceMedium
}

// filterByConfidence returns only findings at or above the minimum confidence.
func filterByConfidence(findings []Finding, min Confidence) []Finding {
	order := map[Confidence]int{ConfidenceHigh: 3, ConfidenceMedium: 2, ConfidenceLow: 1}
	minScore := order[min]
	var out []Finding
	for _, f := range findings {
		c := Confidence(f.Confidence)
		if c == "" {
			c = ConfidenceMedium
		}
		if order[c] >= minScore {
			out = append(out, f)
		}
	}
	return out
}

// ── G-023: scan history ───────────────────────────────────────────────────────

// persistScanHistory appends the current scan result to
// .forge/scan-history/<timestamp>-<family>.json.
func persistScanHistory(root, family string, res *ScanResult) {
	dir := filepath.Join(root, ".forge", "scan-history")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	entry := ScanHistoryEntry{
		SchemaVersion: 1,
		Family:        family,
		CapturedAt:    time.Now().UTC().Format(time.RFC3339),
		Findings:      res.Findings,
		Count:         res.Count,
		Status:        res.Status,
	}
	ts := time.Now().UTC().Format("20060102-150405")
	path := filepath.Join(dir, ts+"-"+family+".json")
	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0o600)
}

// loadScanBaseline loads the most recent scan history entry for a family.
// Returns nil if no history exists.
func loadScanBaseline(root, family string) *ScanHistoryEntry {
	dir := filepath.Join(root, ".forge", "scan-history")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	// Sort descending to get most recent.
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() > entries[j].Name()
	})
	for _, e := range entries {
		if strings.Contains(e.Name(), "-"+family+".json") {
			data, err := os.ReadFile(filepath.Join(dir, e.Name()))
			if err != nil {
				continue
			}
			var entry ScanHistoryEntry
			if json.Unmarshal(data, &entry) == nil {
				return &entry
			}
		}
	}
	return nil
}

// diffFindings returns new findings not present in the baseline (by Rule+File+Line).
func diffFindings(current []Finding, baseline *ScanHistoryEntry) []Finding {
	if baseline == nil {
		return current
	}
	type key struct{ rule, file string; line int }
	baseKeys := make(map[key]bool, len(baseline.Findings))
	for _, f := range baseline.Findings {
		baseKeys[key{f.Rule, f.File, f.Line}] = true
	}
	var newFindings []Finding
	for _, f := range current {
		if !baseKeys[key{f.Rule, f.File, f.Line}] {
			newFindings = append(newFindings, f)
		}
	}
	return newFindings
}
