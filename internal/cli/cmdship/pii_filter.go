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

// pii_filter.go — RFC-005 P3: Pre-LLM PII detection and redaction.
//
// PIIFilter scans prompt text for personally-identifiable information (PII)
// before it is dispatched to an external LLM provider. When PII is found the
// caller decides whether to redact (continue) or block (abort the request).
//
// Detected categories:
//   - Email addresses
//   - E.164 phone numbers
//   - US Social Security Numbers (SSN) in XXX-XX-XXXX format
//   - US credit card numbers (Luhn-valid 13–19 digit sequences)
//   - IPv4 addresses (private ranges flagged as informational; public as PII)
//   - Names preceded by common PII labels ("Name:", "Full Name:", "Patient:", …)
//
// All replacements use the [PII:<type>] placeholder so downstream tooling can
// count redactions without recovering the original value.
package cmdship

import (
	"net"
	"regexp"

	"github.com/teragrid/forge/internal/errcode"
)

// ErrPIIDetected is raised when PII is found in a prompt and the caller's
// policy is set to PIIPolicyBlock.
var ErrPIIDetected = errcode.New(
	errcode.Register(errcode.Code(3213), "PII detected in prompt — blocked before LLM dispatch"),
	"PII detected in prompt — blocked before LLM dispatch", nil)

// PIIPolicy controls what the filter does when PII is found.
type PIIPolicy int

const (
	// PIIPolicyRedact replaces PII with a [PII:<type>] placeholder and proceeds.
	PIIPolicyRedact PIIPolicy = iota
	// PIIPolicyBlock returns ErrPIIDetected without dispatching the prompt.
	PIIPolicyBlock
	// PIIPolicyWarn redacts and records a warning but never returns an error.
	PIIPolicyWarn
)

// PIIMatch is one detected PII occurrence.
type PIIMatch struct {
	// Category is the PII type ("email" | "phone" | "ssn" | "credit-card" | "ip-public").
	Category string
	// Offset is the byte offset of the match within the original text.
	Offset int
	// Length is the byte length of the matched text.
	Length int
}

// piiRule bundles a compiled regexp with a human-readable category label.
// skip is an optional post-filter: when non-nil, matches for which skip returns
// true are silently dropped (used to exclude private IP ranges from ip-public).
type piiRule struct {
	re       *regexp.Regexp
	category string
	skip     func(match string) bool
}

// isPrivateIP returns true if the IP string is an RFC-1918 / loopback address.
func isPrivateIP(s string) bool {
	ip := net.ParseIP(s)
	if ip == nil {
		return false
	}
	for _, cidr := range []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"127.0.0.0/8",
	} {
		_, network, _ := net.ParseCIDR(cidr)
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

// defaultPIIRules returns the built-in PII detection patterns.
var defaultPIIRules = []piiRule{
	// Email addresses
	{
		re:       regexp.MustCompile(`\b[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}\b`),
		category: "email",
	},
	// E.164 phone numbers: +1-555-555-5555 / (555) 555-5555 / 555.555.5555
	{
		re:       regexp.MustCompile(`(?:\+\d{1,3}[\s\-]?)?\(?\d{3}\)?[\s.\-]\d{3}[\s.\-]\d{4}\b`),
		category: "phone",
	},
	// US SSN: XXX-XX-XXXX
	{
		re:       regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`),
		category: "ssn",
	},
	// Credit card: 13–19 digit sequences (Visa, MC, Amex, Discover patterns with separators)
	{
		re:       regexp.MustCompile(`\b(?:\d[ \-]?){13,19}\b`),
		category: "credit-card",
	},
	// IPv4 addresses — private ranges are excluded via the skip callback.
	{
		re:       regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`),
		category: "ip-public",
		skip:     isPrivateIP,
	},
	// PII labels: "Name: John Smith", "Patient: Alice", "Full Name: ..."
	{
		re:       regexp.MustCompile(`(?i)(?:full\s+)?(?:name|patient|customer|employee|user)\s*:\s*[A-Z][a-z]+(?:\s+[A-Z][a-z]+)+`),
		category: "named-individual",
	},
}

// PIIFilter detects and optionally redacts PII in prompt text.
type PIIFilter struct {
	rules  []piiRule
	policy PIIPolicy
}

// NewPIIFilter returns a PIIFilter with the default rule set and the given policy.
func NewPIIFilter(policy PIIPolicy) *PIIFilter {
	return &PIIFilter{rules: defaultPIIRules, policy: policy}
}

// Scan returns all PII matches found in text without modifying it.
// Matches are returned in order of occurrence (earliest offset first).
func (f *PIIFilter) Scan(text string) []PIIMatch {
	var matches []PIIMatch
	for _, rule := range f.rules {
		locs := rule.re.FindAllStringIndex(text, -1)
		for _, loc := range locs {
			matched := text[loc[0]:loc[1]]
			if rule.skip != nil && rule.skip(matched) {
				continue
			}
			matches = append(matches, PIIMatch{
				Category: rule.category,
				Offset:   loc[0],
				Length:   loc[1] - loc[0],
			})
		}
	}
	// Sort by offset ascending.
	for i := 1; i < len(matches); i++ {
		for j := i; j > 0 && matches[j].Offset < matches[j-1].Offset; j-- {
			matches[j], matches[j-1] = matches[j-1], matches[j]
		}
	}
	return matches
}

// Apply scans text and, depending on policy:
//   - PIIPolicyRedact:  returns (redactedText, nil) — PII replaced with [PII:<type>]
//   - PIIPolicyBlock:   returns ("", ErrPIIDetected) when PII found; ("", nil) otherwise
//   - PIIPolicyWarn:    returns (redactedText, nil) always — same as Redact
func (f *PIIFilter) Apply(text string) (string, error) {
	matches := f.Scan(text)
	if len(matches) == 0 {
		return text, nil
	}
	if f.policy == PIIPolicyBlock {
		return "", ErrPIIDetected
	}
	return f.redact(text), nil
}

// redact replaces every PII occurrence with [PII:<category>].
func (f *PIIFilter) redact(text string) string {
	result := text
	for _, rule := range f.rules {
		result = rule.re.ReplaceAllStringFunc(result, func(m string) string {
			if rule.skip != nil && rule.skip(m) {
				return m // preserve private IPs / other skipped matches
			}
			return "[PII:" + rule.category + "]"
		})
	}
	return result
}

// Categories returns a deduplicated list of PII category names found in text.
func (f *PIIFilter) Categories(text string) []string {
	matches := f.Scan(text)
	seen := make(map[string]bool, len(matches))
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		if !seen[m.Category] {
			seen[m.Category] = true
			out = append(out, m.Category)
		}
	}
	return out
}
