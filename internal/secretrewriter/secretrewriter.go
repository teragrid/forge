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

// Package secretrewriter implements DEV-M0-09: a secrets-scrubbing layer that
// replaces sensitive values in prompt strings with stable, length-independent
// placeholders before text is transmitted to an external LLM provider.
//
// Default patterns cover the most common API-key formats (OpenAI sk- keys,
// Anthropic keys, generic hex/base64 tokens of 32+ chars, and JWT tokens).
// Callers may add project-specific patterns via [New].
//
// The placeholder format is [REDACTED:<type>] — the original value's length is
// deliberately NOT preserved in the replacement so that an adversary cannot
// infer key length from the scrubbed output.
package secretrewriter

import (
	"regexp"
)

// rewriteRule bundles a compiled regexp with a human-readable type label.
type rewriteRule struct {
	re    *regexp.Regexp
	label string
}

// Rewriter scrubs secret values from text.
type Rewriter struct {
	rules []rewriteRule
}

// Result is the output of a single Rewrite call.
type Result struct {
	// Text is the scrubbed string (all secrets replaced).
	Text string
	// Replacements is the number of substitutions made.
	Replacements int
}

// defaultRules returns the built-in secret detection patterns.
// Each pattern is anchored to the full secret value (group 1 is the part to replace).
var defaultRules = []rewriteRule{
	// OpenAI API keys: sk-... (20+ alphanum chars)
	{re: regexp.MustCompile(`\bsk-[A-Za-z0-9\-_]{20,}\b`), label: "openai-key"},
	// Anthropic API keys: sk-ant-...
	{re: regexp.MustCompile(`\bsk-ant-[A-Za-z0-9\-_]{20,}\b`), label: "anthropic-key"},
	// Generic high-entropy base64 tokens (32+ chars) preceded by = sign (env var values)
	{re: regexp.MustCompile(`(?:^|[\s;,])[A-Z][A-Z0-9_]+=([A-Za-z0-9+/]{32,}={0,2})`), label: "env-secret"},
	// JWT tokens: three base64url segments separated by dots
	{re: regexp.MustCompile(`\beyJ[A-Za-z0-9_\-]+\.[A-Za-z0-9_\-]+\.[A-Za-z0-9_\-]+\b`), label: "jwt"},
	// GitHub personal access tokens: ghp_... or github_pat_...
	{re: regexp.MustCompile(`\b(?:ghp_|github_pat_)[A-Za-z0-9_]{20,}\b`), label: "github-token"},
	// AWS access key IDs: AKIA...
	{re: regexp.MustCompile(`\bAKIA[A-Z0-9]{16}\b`), label: "aws-key"},
}

// New returns a Rewriter using the built-in patterns plus any extraRules.
// Extra rules are appended after the built-in set.
func New(extraRules ...*regexp.Regexp) *Rewriter {
	r := &Rewriter{}
	r.rules = append(r.rules, defaultRules...)
	for _, re := range extraRules {
		if re != nil {
			r.rules = append(r.rules, rewriteRule{re: re, label: "custom"})
		}
	}
	return r
}

// Rewrite returns a copy of text with all detected secrets replaced.
// Replacements are idempotent: running Rewrite twice on already-scrubbed text
// is safe and will not alter the placeholders (they do not match secret patterns).
func (r *Rewriter) Rewrite(text string) Result {
	out := text
	total := 0
	for _, rule := range r.rules {
		replaced := 0
		// For the env-secret pattern we have a capture group for just the value part;
		// for all others the full match is the secret.
		if rule.label == "env-secret" {
			out = rule.re.ReplaceAllStringFunc(out, func(m string) string {
				// Find the submatch to locate the value-only portion.
				sub := rule.re.FindStringSubmatchIndex(m)
				if len(sub) < 4 || sub[2] < 0 {
					return m
				}
				replaced++
				// Preserve the KEY= prefix; replace only the value (group 1).
				groupStart := sub[2] - sub[0] // offset of group 1 start within m
				return m[:groupStart] + placeholder(rule.label)
			})
		} else {
			out = rule.re.ReplaceAllStringFunc(out, func(_ string) string {
				replaced++
				return placeholder(rule.label)
			})
		}
		total += replaced
	}
	return Result{Text: out, Replacements: total}
}

// placeholder returns the redaction tag for the given secret type.
// The original secret length is intentionally not reflected in the placeholder.
func placeholder(label string) string {
	return "[REDACTED:" + label + "]"
}
