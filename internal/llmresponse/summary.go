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

package llmresponse

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// maxSummaryChars is the hard cap on context_summary length. Kept well below
// ~500 tokens (≈ 375 words ≈ 2 000 chars at ~4 chars/token) so LLMs always
// have room for the next prompt without context overflow.
const maxSummaryChars = 2000

// SummaryParams carries all inputs for a deterministic context_summary.
type SummaryParams struct {
	// Verb is the forge subcommand (e.g. "ship", "scan", "doctor").
	Verb string
	// Checkpoint is the current pipeline stage (e.g. "code", "verify").
	Checkpoint string
	// FeatureName is the --name slug (e.g. "auth-email").
	FeatureName string
	// Status is the checkpoint outcome (completed/skipped/failed/…).
	Status string
	// FilesChanged is a list of paths modified during this checkpoint.
	FilesChanged []string
	// TestsPassed / TestsFailed track the test suite outcome.
	TestsPassed int
	TestsFailed int
	// LLMTokensUsed is the total token spend for this checkpoint.
	LLMTokensUsed int
	// CostUSD is the USD cost of LLM calls this checkpoint.
	CostUSD float64
	// ErrorCode is set when Status == "failed".
	ErrorCode string
	// ErrorHint is the one-line actionable hint for the failure.
	ErrorHint string
	// ExtraLines are optional appended lines (e.g. custom KB context).
	// Each is truncated to 120 chars to prevent blowout.
	ExtraLines []string
}

// GenerateSummary builds a deterministic, ≤2000-char UTF-8 string that gives
// an LLM complete situational awareness after one forge command.
// The output is stable — same inputs always produce the same summary.
func GenerateSummary(p SummaryParams) string {
	var b strings.Builder

	// Line 1: command identity
	fmt.Fprintf(&b, "forge %s", p.Verb)
	if p.Checkpoint != "" {
		fmt.Fprintf(&b, " %s", p.Checkpoint)
	}
	if p.FeatureName != "" {
		fmt.Fprintf(&b, " --name %s", p.FeatureName)
	}
	fmt.Fprintf(&b, " → %s\n", p.Status)

	// Line 2: test results (only when tests were run)
	if p.TestsPassed+p.TestsFailed > 0 {
		fmt.Fprintf(&b, "tests: %d passed / %d failed\n", p.TestsPassed, p.TestsFailed)
	}

	// Line 3: spend
	if p.LLMTokensUsed > 0 || p.CostUSD > 0 {
		fmt.Fprintf(&b, "spend: %d tokens / $%.5f\n", p.LLMTokensUsed, p.CostUSD)
	}

	// Line 4+: changed files (capped at 20 entries, then summarised)
	const maxFiles = 20
	if len(p.FilesChanged) > 0 {
		if len(p.FilesChanged) <= maxFiles {
			fmt.Fprintf(&b, "changed: %s\n", strings.Join(p.FilesChanged, ", "))
		} else {
			fmt.Fprintf(&b, "changed: %s … (+%d more)\n",
				strings.Join(p.FilesChanged[:maxFiles], ", "), len(p.FilesChanged)-maxFiles)
		}
	}

	// Line N: error (when present)
	if p.ErrorCode != "" {
		fmt.Fprintf(&b, "error: %s %s\n", p.ErrorCode, p.ErrorHint)
	}

	// Extra lines (e.g. injected KB context)
	for _, line := range p.ExtraLines {
		if utf8.RuneCountInString(line) > 120 {
			runes := []rune(line)
			line = string(runes[:120]) + "…"
		}
		fmt.Fprintf(&b, "%s\n", line)
	}

	return truncateSummary(b.String())
}

// truncateSummary clips s to maxSummaryChars at a word boundary, appending
// "…" if truncated. It never breaks a UTF-8 rune.
func truncateSummary(s string) string {
	if utf8.RuneCountInString(s) <= maxSummaryChars {
		return strings.TrimRightFunc(s, unicode.IsSpace)
	}
	runes := []rune(s)
	// Walk back from the limit to find a space or newline.
	cut := maxSummaryChars - 1 // leave room for "…"
	for cut > 0 && runes[cut] != ' ' && runes[cut] != '\n' {
		cut--
	}
	if cut == 0 {
		cut = maxSummaryChars - 1
	}
	return string(runes[:cut]) + "…"
}
