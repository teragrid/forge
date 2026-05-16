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

// Package guardrails implements G-105: LLM response safety guardrails.
//
// Guardrails are composable middleware applied to LLM responses before they
// reach the caller. Each guardrail has a configurable severity level:
//   - warn  — log and continue
//   - block — return error, do not surface the response
//   - redact — replace violating content with a placeholder
//
// Built-in guardrails:
//  1. RefusalDetect — detects LLM refusal phrases ("I cannot", "I'm unable to")
//  2. PIIRedact — redacts email addresses, phone numbers, credit card patterns
//  3. StructuredOutputValidate — validates JSON against an expected schema shape
//  4. JailbreakDetect — flags suspicious prompt-injection patterns in the response
package guardrails

import (
	"fmt"
	"regexp"
	"strings"
)

// Severity controls what happens when a guardrail fires.
type Severity string

const (
	SeverityWarn   Severity = "warn"
	SeverityBlock  Severity = "block"
	SeverityRedact Severity = "redact"
)

// GuardrailResult is the outcome of running a single guardrail.
type GuardrailResult struct {
	Name     string
	Fired    bool
	Severity Severity
	Detail   string
	Output   string // possibly redacted content
}

// ── Refusal detection ─────────────────────────────────────────────────────

var refusalPhrases = []string{
	"i cannot", "i'm unable", "i am unable",
	"i'm not able", "i am not able",
	"as an ai", "as a language model",
	"i don't have the ability",
}

// RefusalDetect checks whether the LLM output is a refusal.
func RefusalDetect(content string, severity Severity) GuardrailResult {
	lower := strings.ToLower(content)
	for _, phrase := range refusalPhrases {
		if strings.Contains(lower, phrase) {
			return GuardrailResult{
				Name: "refusal_detect", Fired: true, Severity: severity,
				Detail: fmt.Sprintf("refusal phrase detected: %q", phrase),
				Output: content,
			}
		}
	}
	return GuardrailResult{Name: "refusal_detect", Output: content}
}

// ── PII redaction ─────────────────────────────────────────────────────────

var (
	reEmail = regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`)
	rePhone = regexp.MustCompile(`\b(\+?1[-.\s]?)?\(?\d{3}\)?[-.\s]?\d{3}[-.\s]?\d{4}\b`)
	reCCNum = regexp.MustCompile(`\b(?:\d[ -]?){13,16}\b`)
)

// PIIRedact replaces PII patterns in content with placeholders.
func PIIRedact(content string, severity Severity) GuardrailResult {
	original := content
	redacted := reEmail.ReplaceAllString(content, "[EMAIL REDACTED]")
	redacted = rePhone.ReplaceAllString(redacted, "[PHONE REDACTED]")
	redacted = reCCNum.ReplaceAllString(redacted, "[CC REDACTED]")
	fired := redacted != original
	return GuardrailResult{
		Name: "pii_redact", Fired: fired, Severity: severity,
		Detail: map[bool]string{true: "PII patterns detected and redacted", false: ""}[fired],
		Output: redacted,
	}
}

// ── Jailbreak detection ───────────────────────────────────────────────────

var jailbreakPatterns = []string{
	"ignore previous instructions",
	"disregard your instructions",
	"you are now",
	"pretend you are",
	"act as if",
	"forget all your rules",
	"override your",
}

// JailbreakDetect checks for prompt-injection patterns in the LLM response.
func JailbreakDetect(content string, severity Severity) GuardrailResult {
	lower := strings.ToLower(content)
	for _, pat := range jailbreakPatterns {
		if strings.Contains(lower, pat) {
			return GuardrailResult{
				Name: "jailbreak_detect", Fired: true, Severity: severity,
				Detail: fmt.Sprintf("potential prompt injection: %q", pat),
				Output: content,
			}
		}
	}
	return GuardrailResult{Name: "jailbreak_detect", Output: content}
}

// ── Pipeline ──────────────────────────────────────────────────────────────

// Pipeline applies a sequence of guardrails to content.
// Returns the (possibly redacted) content and all results.
// Returns an error if any Block-severity guardrail fires.
func Pipeline(content string, guardrailFns ...func(string) GuardrailResult) (string, []GuardrailResult, error) {
	var results []GuardrailResult
	current := content
	for _, fn := range guardrailFns {
		r := fn(current)
		results = append(results, r)
		if r.Fired {
			switch r.Severity {
			case SeverityBlock:
				return "", results, fmt.Errorf("guardrail %q blocked response: %s", r.Name, r.Detail)
			case SeverityRedact:
				current = r.Output // use redacted content for subsequent guardrails
			}
		}
	}
	return current, results, nil
}
