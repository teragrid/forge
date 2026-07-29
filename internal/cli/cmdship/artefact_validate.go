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

// artefact_validate.go — J8/J9: artefact write-path validation, shared by
// every checkpoint that writes an LLM-generated Markdown artefact to disk.
//
// Two independently-confirmed incidents motivated this:
//   - a raw LLM conversational preamble ("I'll review this feature
//     specification and provide comprehensive improvements...") was written
//     verbatim as the first line of a generated spec.md (J8);
//   - a generated spec.md was truncated mid-Gherkin-block with no closing
//     content and no error surfaced (J9).
//
// Both checks are cheap, structural string checks — not a second LLM call —
// per the spec's NFR ("Preamble/truncation detection is cheap").
package cmdship

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// validateArtefact strips conversational preamble from raw and reports
// whether the result looks structurally complete. Callers should write only
// the returned cleaned content, and treat complete == false as a signal to
// retry once (see generateWithValidation) rather than writing a broken file.
func validateArtefact(raw string) (cleaned string, complete bool) {
	cleaned = stripPreamble(raw)
	complete = looksComplete(cleaned)
	return cleaned, complete
}

// stripPreamble removes any conversational preamble the LLM prepended before
// the actual artefact content, by cutting everything before the first
// Markdown heading line (one or more '#' followed by non-'#' content). If
// the content already starts with a heading (after trimming leading
// whitespace) it is returned unchanged. If no heading is found anywhere, the
// content is returned unchanged — looksComplete will likely flag it as
// incomplete separately.
func stripPreamble(s string) string {
	trimmed := strings.TrimLeft(s, " \t\r\n")
	if strings.HasPrefix(trimmed, "#") {
		return trimmed
	}
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		l := strings.TrimSpace(line)
		if strings.HasPrefix(l, "#") {
			return strings.Join(lines[i:], "\n")
		}
	}
	return s
}

// looksComplete applies cheap structural checks to decide whether a
// generated artefact looks whole (J9): unbalanced fenced code blocks (odd
// ``` count) is the clearest truncation signal, and the response should end
// on terminal punctuation, a closing fence, a heading, a list item, or
// another structurally terminal character — anything else (e.g. cut off
// mid-sentence inside a Gherkin block) is treated as truncated.
func looksComplete(s string) bool {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return false
	}
	if strings.Count(trimmed, "```")%2 != 0 {
		return false // unbalanced fenced code block — cut off mid-block
	}

	last := lastNonEmptyLine(trimmed)
	if last == "" {
		return false
	}
	if strings.HasSuffix(last, "```") {
		return true
	}
	// An odd number of single backticks on the last line means an inline code
	// span was opened and never closed — the clearest same-line signal that
	// generation stopped mid-token (e.g. "...Return: `{platform, external_
	// account_id, oauth_" cut off inside a backtick-wrapped example). Checked
	// before the heading/list-item/terminal-punctuation shortcuts below,
	// which would otherwise false-positive on exactly this shape (root-caused
	// 2026-07-23: a numbered list item — "2. Return: `{...`" — cut off
	// mid-word inside an unclosed inline code span was accepted as complete
	// because the line's *prefix* matched isListItem, even though the line's
	// own content was plainly truncated).
	if strings.Count(last, "`")%2 != 0 {
		return false
	}
	lastTrimmed := strings.TrimSpace(last)
	if strings.HasPrefix(lastTrimmed, "#") {
		return true // ends on a heading — acceptable terminal shape
	}
	if isListItem(lastTrimmed) {
		return true // ends on a complete Markdown list item — a normal,
		// intentional document ending (e.g. "- happy path"), not a truncation
		// signal; a genuinely cut-off list item ends mid-word instead, which
		// this check does not attempt to distinguish in general (out of scope
		// for a cheap structural check per the spec's NFR) — the unclosed-
		// backtick check above catches the common case where the cut-off
		// happens inside an inline code span, which is the shape actually
		// observed to recur.
	}
	runes := []rune(last)
	switch runes[len(runes)-1] {
	case '.', '!', '?', ':', ')', ']', '"', '\'', '`', '|', '>':
		return true
	}
	return false
}

// isListItem reports whether line (already trimmed) is a Markdown list item:
// "- ", "* ", "+ " (unordered) or "1. "/"1) " (ordered).
func isListItem(line string) bool {
	if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") || strings.HasPrefix(line, "+ ") {
		return true
	}
	i := 0
	for i < len(line) && line[i] >= '0' && line[i] <= '9' {
		i++
	}
	return i > 0 && i < len(line) && (line[i] == '.' || line[i] == ')') &&
		i+1 < len(line) && line[i+1] == ' '
}

// lastNonEmptyLine returns the last non-blank line of s, or "" if s has none.
func lastNonEmptyLine(s string) string {
	lines := strings.Split(s, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		l := strings.TrimRight(lines[i], " \t\r")
		if l != "" {
			return l
		}
	}
	return ""
}

// generateWithValidation runs invoke, validates the result (J8/J9/J11), and
// on an incomplete response retries once before giving up. Returns the
// cleaned (preamble-stripped) content and whether it is structurally
// complete; err is only non-nil when invoke itself failed (an LLM/transport
// error, distinct from a structurally-incomplete-but-successful response).
//
// invoke's second return value is the provider's own truncation signal (see
// llmprovider.Response.Truncated) — apiTruncated == true is authoritative
// and short-circuits straight to "incomplete" without needing the
// looksComplete() heuristic to guess correctly. That heuristic has a known
// blind spot: a markdown list item that gets cut off mid-word (e.g. "- **Cost
// awareness**: ...for posit") still starts with "- ", so looksComplete alone
// waves it through as a normal short list item. The API telling us it hit
// MaxTokens doesn't have that ambiguity.
//
// Callers should treat (content=="" && err==nil) as "no content generated"
// (unchanged existing behaviour), and (content!="" && !complete) as the J9
// case: write a stub / log a failure instead of the broken content.
func generateWithValidation(invoke func() (string, bool, error)) (content string, complete bool, err error) {
	generated, apiTruncated, genErr := invoke()
	if genErr != nil {
		return "", false, genErr
	}
	if generated == "" {
		return "", false, nil
	}
	cleaned, ok := validateArtefact(generated)
	if ok && !apiTruncated {
		return cleaned, true, nil
	}

	// J9/J11: retry once before giving up.
	retryGenerated, retryAPITruncated, retryErr := invoke()
	if retryErr != nil || retryGenerated == "" {
		return cleaned, false, nil
	}
	cleaned2, ok2 := validateArtefact(retryGenerated)
	if retryAPITruncated {
		ok2 = false
	}
	return cleaned2, ok2, nil
}

// filePathRefPattern matches backtick-wrapped, path-shaped tokens: at least
// one directory separator followed by a filename with an extension, e.g.
// `src/app/api/x/route.ts` or `internal/cli/cmdship/ship.go`. Restricting to
// backtick-wrapped tokens (inline code spans, the convention every spec/arch
// prompt is instructed to use for file paths) keeps false positives low —
// ordinary prose sentences don't get flagged.
var filePathRefPattern = regexp.MustCompile("`((?:[A-Za-z0-9_.-]+/)+[A-Za-z0-9_.-]+\\.[A-Za-z0-9]{1,6})`")

// findUnverifiedFileReferences scans generated Markdown for backtick-wrapped
// file-path-shaped references and returns the ones that do not exist on disk
// under root, deduplicated and in first-seen order.
//
// Root cause this addresses: LLM-generated spec/arch content has repeatedly
// (confirmed 3 separate times in one session, 2026-07-20) invented plausible-
// looking but nonexistent file paths (e.g. a project's real
// `src/app/api/integrations/facebook/callback/route.ts` rendered by the model
// as `src/app/api/auth/facebook/callback/route.ts`) and been trusted as fact
// by whoever read the artefact next. This is a cheap, structural check — not
// a second LLM call — that surfaces the mismatch instead of silently trusting
// it; it does not attempt to also verify DB column/table names or invented
// infrastructure, which need repo-specific knowledge this check doesn't have.
func findUnverifiedFileReferences(root, content string) []string {
	if root == "" {
		return nil
	}
	seen := make(map[string]bool)
	var unverified []string
	for _, m := range filePathRefPattern.FindAllStringSubmatch(content, -1) {
		candidate := m[1]
		if seen[candidate] {
			continue
		}
		seen[candidate] = true
		// Paths under .forge/ are artefacts this checkpoint (or a sibling
		// one) is about to create, not existing code — never flag those.
		if strings.HasPrefix(candidate, ".forge/") {
			continue
		}
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(candidate))); os.IsNotExist(err) {
			unverified = append(unverified, candidate)
		}
	}
	return unverified
}

// appendUnverifiedPathsWarning appends a visible callout listing any
// unverified file-path references (see findUnverifiedFileReferences) to the
// end of a generated artefact, so a human reviewer sees the flag inline
// rather than needing to separately grep the repo to catch a hallucinated
// path. Returns content unchanged when there is nothing to flag.
func appendUnverifiedPathsWarning(root, content string) string {
	unverified := findUnverifiedFileReferences(root, content)
	if len(unverified) == 0 {
		return content
	}
	var b strings.Builder
	b.WriteString(content)
	if !strings.HasSuffix(content, "\n") {
		b.WriteString("\n")
	}
	b.WriteString("\n---\n\n> **⚠ Unverified file references (automated check):** the following paths mentioned above were not found in this repository — verify before relying on them, they may be hallucinated:\n")
	for _, p := range unverified {
		b.WriteString("> - `" + p + "`\n")
	}
	return b.String()
}
