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

// Package cmdship — G-010: project-local prompt overrides.
//
// Forge checks for .forge/prompts/<op>.prompt.md before falling back to the
// built-in system prompt. This lets teams customize the LLM instructions for
// each checkpoint without forking the binary.
//
// G-011: learning loop wiring.
// After each failure in checkSpec/checkTest/checkBreakdown, forge appends a
// failure record to .forge/learned/<checkpoint>-failures.jsonl. At the start
// of each checkpoint forge reads the last 3 failures and surfaces them as
// context prefix "⚠️ Recent failures" in the LLM call.
package cmdship

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ── G-010: project-local prompt overrides ────────────────────────────────────

// loadProjectPrompt returns the content of .forge/prompts/<op>.prompt.md
// if it exists, otherwise returns "".
func loadProjectPrompt(root, op string) string {
	p := filepath.Join(root, ".forge", "prompts", op+".prompt.md")
	data, err := os.ReadFile(p)
	if err != nil {
		return ""
	}
	return string(data)
}

// promptOverrides is the set of operation names for which project-local
// prompt files are consulted.
var promptOverrides = []string{ //nolint:unused // referenced via loadProjectPrompt callers
	"ship-spec",
	"ship-test",
	"ship-breakdown",
	"ship-code",
	"ship-ship",
}

// ensurePromptTemplates writes the default .forge/prompts/ template files when
// they do not exist. This is called on first `forge ship` run.
func ensurePromptTemplates(root string) { //nolint:unused // called from ship init path
	dir := filepath.Join(root, ".forge", "prompts")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	defaults := map[string]string{
		"ship-spec": "You are a senior product engineer writing a feature specification.\n" +
			"Produce a Markdown spec with sections: Goal, Scope, Acceptance Criteria, Non-Goals, Open Questions.\n",
		"ship-test": "You are a senior QA engineer writing failing test stubs for TDD.\n" +
			"Tests MUST compile but MUST fail at runtime. Use Jest + supertest for TypeScript, testing.T for Go.\n",
		"ship-breakdown": "You are a delivery lead decomposing a feature spec into atomic tasks.\n" +
			"Format: numbered list. Each task: title, effort (XS/S/M/L), dependencies, acceptance criteria.\n",
		"ship-code": "You are a senior engineer writing a step-by-step implementation plan.\n" +
			"Reference spec.md and breakdown.md. Be precise about file paths and function signatures.\n",
		"ship-ship": "You are reviewing code before shipping.\n" +
			"Check: security, correctness, performance, test coverage, and API contract compliance.\n",
	}
	for name, content := range defaults {
		path := filepath.Join(dir, name+".prompt.md")
		if _, err := os.Stat(path); os.IsNotExist(err) {
			_ = os.WriteFile(path, []byte(content), 0o600)
		}
	}
}

// ── G-011: learning loop wiring ──────────────────────────────────────────────

// FailureRecord is appended to .forge/learned/<checkpoint>-failures.jsonl on
// every checkpoint failure. It is read back at the start of the next run to
// surface recent patterns.
type FailureRecord struct {
	TS         string `json:"ts"`
	Checkpoint string `json:"checkpoint"`
	Feature    string `json:"feature"`
	Detail     string `json:"detail"`
}

// appendFailure records a checkpoint failure to .forge/learned/<cp>-failures.jsonl.
func appendFailure(root, checkpoint, feature, detail string) {
	dir := filepath.Join(root, ".forge", "learned")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	rec := FailureRecord{
		TS:         time.Now().UTC().Format(time.RFC3339),
		Checkpoint: checkpoint,
		Feature:    feature,
		Detail:     detail,
	}
	data, err := json.Marshal(rec)
	if err != nil {
		return
	}
	path := filepath.Join(dir, checkpoint+"-failures.jsonl")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.Write(data)
	_, _ = f.WriteString("\n")
}

// loadRecentFailures reads the last n failure records for a checkpoint.
// Returns a human-readable summary suitable for prepending to an LLM prompt.
func loadRecentFailures(root, checkpoint string, n int) string {
	path := filepath.Join(root, ".forge", "learned", checkpoint+"-failures.jsonl")
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	lines := splitLines(string(data))
	if len(lines) == 0 {
		return ""
	}
	// Take the last n non-empty lines.
	var recent []string
	for i := len(lines) - 1; i >= 0 && len(recent) < n; i-- {
		if lines[i] == "" {
			continue
		}
		var rec FailureRecord
		if err := json.Unmarshal([]byte(lines[i]), &rec); err == nil {
			recent = append(recent, fmt.Sprintf("  [%s] %s: %s", rec.TS[:10], rec.Feature, rec.Detail))
		}
	}
	if len(recent) == 0 {
		return ""
	}
	out := fmt.Sprintf("\u26a0\ufe0f Recent %s failures (last %d):\n", checkpoint, len(recent))
	for _, r := range recent {
		out += r + "\n"
	}
	return out
}

// ── G-015: post-feature knowledge extraction ─────────────────────────────────

// LearnedPattern is one pattern extracted from a completed ship run and
// written to .forge/learned/patterns-<slug>.jsonl.
type LearnedPattern struct {
	TS          string   `json:"ts"`
	Feature     string   `json:"feature"`
	PatternType string   `json:"pattern_type"` // "good-practice" | "anti-pattern" | "gap"
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Checkpoints []string `json:"checkpoints,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

// extractAndLearnFromFeature is called after a successful ship run to extract
// patterns and lessons learned.
//
// It writes to two destinations:
//  1. .forge/learned/patterns-<slug>.jsonl   — run history (fast lookup)
//  2. forge-knowledge/knowledge-base/patterns/workflow/learned/<slug>-<ts>.md
//     — KB-formatted entry with frontmatter, re-indexed on next forge scan run
//
// A nil pipe silently skips extraction (no-op).
func extractAndLearnFromFeature(root, description string, result *ShipResult, pipe *LLMPipe) {
	if pipe == nil || result == nil || !result.Ready {
		return
	}
	// Build a concise summary of the run for the LLM.
	var summary strings.Builder
	summary.WriteString(fmt.Sprintf("Feature: %s\n", description))
	summary.WriteString(fmt.Sprintf("Checkpoints (%d):\n", len(result.Checkpoints)))
	for _, cp := range result.Checkpoints {
		detail := cp.Detail
		if len(detail) > 120 {
			detail = detail[:120] + "…"
		}
		fmt.Fprintf(&summary, "  [%s] %s — %s\n", cp.Status, cp.Name, detail)
		if cp.RemediationRounds > 0 {
			fmt.Fprintf(&summary, "    remediation_rounds: %d\n", cp.RemediationRounds)
		}
	}

	systemPrompt := `You are a senior engineering coach extracting lessons from a completed feature ship run.
Analyse the checkpoint results and extract up to 3 patterns (good practices, anti-patterns, or recurring gaps).
Return a JSON array of objects with these fields:
  pattern_type: "good-practice" | "anti-pattern" | "gap"
  title: short title (≤ 10 words)
  description: one sentence explanation
  checkpoints: array of checkpoint names where this pattern appeared
  tags: array of ≤3 relevant tags

Rules: only include patterns with clear evidence; return [] if nothing notable.
Return only the JSON array — no markdown, no explanation.`

	userPrompt := "Ship run summary:\n" + summary.String()

	resp, err := pipe.Invoke("ship-learn", "", systemPrompt, userPrompt, 800)
	if err != nil || resp == "" {
		return
	}

	slug := slugify(description)
	ts := time.Now().UTC().Format(time.RFC3339)
	tsShort := time.Now().UTC().Format("20060102-150405")

	// ── Destination 1: .forge/learned/patterns-<slug>.jsonl ──────────────────
	dir := filepath.Join(root, ".forge", "learned")
	if mkErr := os.MkdirAll(dir, 0o755); mkErr == nil {
		path := filepath.Join(dir, "patterns-"+slug+".jsonl")
		if f, fErr := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600); fErr == nil {
			envelope := LearnedPattern{
				TS:          ts,
				Feature:     description,
				PatternType: "extraction",
				Title:       "LLM extraction",
				Description: resp,
			}
			if data, mErr := json.Marshal(envelope); mErr == nil {
				_, _ = f.Write(data)
				_, _ = f.WriteString("\n")
			}
			_ = f.Close()
		}
	}

	// ── Destination 2: forge-knowledge KB markdown with frontmatter ───────────
	// Written to forge-knowledge/knowledge-base/patterns/workflow/learned/
	// so the gen-knowledge-index tool picks it up on the next re-index run.
	kbDir := filepath.Join(root, "forge-knowledge", "knowledge-base", "patterns", "workflow", "learned")
	if mkErr := os.MkdirAll(kbDir, 0o755); mkErr == nil {
		kbPath := filepath.Join(kbDir, slug+"-"+tsShort+".md")

		// Collect checkpoint names that ran successfully.
		var checkpointNames []string
		for _, cp := range result.Checkpoints {
			checkpointNames = append(checkpointNames, strings.ToLower(cp.Name))
		}

		// Write KB-formatted markdown with YAML frontmatter.
		kbContent := fmt.Sprintf(`---
id: learned-%s-%s
title: "Learned patterns: %s"
category: patterns/workflow
forge_integration:
  ship_checkpoints: [%s]
  scan_families: [security, compliance, reliability]
  tags: [learned, post-ship, workflow]
---

# Learned Patterns: %s

_Extracted by `+"`forge ship`"+` learning loop — %s_

## Feature Context

%s

## Extracted Patterns

%s

## Ship Run Summary

%s
`,
			slug, tsShort,
			description,
			strings.Join(checkpointNames, ", "),
			description,
			ts,
			description,
			resp,
			summary.String(),
		)
		_ = os.WriteFile(kbPath, []byte(kbContent), 0o600)
	}
}

// splitLines splits s by newline, handling CRLF.
func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			line := s[start:i]
			if len(line) > 0 && line[len(line)-1] == '\r' {
				line = line[:len(line)-1]
			}
			lines = append(lines, line)
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
