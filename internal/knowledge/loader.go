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

package knowledge

import (
	"strings"

	"github.com/teragrid/forge/internal/contextbudgeter"
)

// FormatDocs renders entries as compact one-line strings: "id: intent | snippet".
// Each line is ~60–100 chars, contributing ~15–25 tokens.
func FormatDocs(entries []Entry) []string {
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		var sb strings.Builder
		sb.WriteString(e.ID)
		if e.Intent != "" {
			sb.WriteString(": ")
			sb.WriteString(e.Intent)
		}
		if e.Snippet != "" && e.Snippet != e.Intent {
			sb.WriteString(" | ")
			sb.WriteString(e.Snippet)
		}
		out = append(out, sb.String())
	}
	return out
}

// AppendDocs appends a <knowledge> XML block to systemPrompt when entries is
// non-empty. The block is compact: one bullet per entry, no newlines inside.
func AppendDocs(systemPrompt string, entries []Entry) string {
	if len(entries) == 0 {
		return systemPrompt
	}
	docs := FormatDocs(entries)
	var sb strings.Builder
	sb.WriteString(systemPrompt)
	sb.WriteString("\n\n<knowledge>\n")
	for _, d := range docs {
		sb.WriteString("- ")
		sb.WriteString(d)
		sb.WriteByte('\n')
	}
	sb.WriteString("</knowledge>")
	return sb.String()
}

// AppendDocsBudgeted is like AppendDocs but trims entries (tail-first, lowest
// score last) until contextbudgeter.CheckBudget approves the combined prompt.
// Falls back to the original systemPrompt when even one entry exceeds budget.
func AppendDocsBudgeted(systemPrompt string, entries []Entry, maxTokens int) string {
	if maxTokens <= 0 {
		return AppendDocs(systemPrompt, entries)
	}
	for len(entries) > 0 {
		candidate := AppendDocs(systemPrompt, entries)
		if contextbudgeter.CheckBudget(candidate, maxTokens) == nil {
			return candidate
		}
		entries = entries[:len(entries)-1]
	}
	return systemPrompt
}

// TokensAdded estimates how many tokens the knowledge block adds to a prompt.
// Useful for pre-flight budget checks without building the full string.
func TokensAdded(entries []Entry) int {
	return contextbudgeter.EstimateTokens(AppendDocs("", entries))
}
