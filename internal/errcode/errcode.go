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
// Package errcode implements the `FORGE-XXXX` error-code framework
// (DEV-M0-03 / Arch §11).
//
// Every user-facing failure must surface as an Error wrapping a Code from a
// reserved range. The registry is process-wide and intentionally append-only:
// duplicate codes panic at init() so a duplicate landing in main is impossible
// to ship. A separate static-analysis lint (TODO post-MVP) walks `Register`
// call-sites to enforce range membership at build time.
package errcode

import (
	"errors"
	"fmt"
	"sort"
	"sync"
)

// Code is a 4-digit identifier rendered as `FORGE-XXXX`.
type Code int

// Reserved ranges (Arch §11). Adding a range requires an ADR amendment.
var reservedRanges = []struct {
	Lo, Hi int
	Owner  string
}{
	{1000, 1099, "cli/router"},
	{1100, 1199, "cli/new"},
	{1200, 1299, "cli/doctor"},
	{1300, 1399, "cli/clean"},
	{1400, 1499, "cli/explain"},
	{1500, 1599, "cli/version"},
	{2000, 2099, "config"},
	{2100, 2199, "fsutil"},
	{2200, 2299, "scaffold"},
	{2300, 2399, "manifest"},
	{2400, 2499, "llm/budget"},
	{3000, 3099, "scan/secrets"},
	{3100, 3199, "lint"},
	{3200, 3299, "ship"},
	{3300, 3399, "cli/upgrade"},
	{3400, 3499, "cli/audit"},
	{3500, 3599, "cli/plugin"},
	{3600, 3699, "cli/eval"},
	{3700, 3799, "cli/failure-register"},
	{3800, 3899, "cli/postmortem"},
	{3900, 3999, "cli/insights"},
	{4000, 4099, "cli/incident"},
	{4100, 4199, "cli/telemetry"},
	{4200, 4299, "plugin/wasm"},
	{4300, 4399, "cli/test"},
	// New verb ranges added for spec §4 gap-fill (M1).
	{1600, 1699, "cli/hygiene"},
	{1700, 1799, "cli/generate"},
	{1800, 1899, "cli/migrate"},
	{1900, 1999, "cli/check"},
	// Infrastructure service packages (DEV-M0-05..M0-09, M0-18).
	{2500, 2599, "internal/fssandbox"},
	{2600, 2699, "internal/gitservice"},
	{2700, 2799, "internal/procspawn"},
	{2800, 2899, "internal/tokenledger"},
	{4400, 4499, "cli/fix"},
	{4500, 4599, "cli/adopt"},
	{4600, 4699, "cli/eject"},
	{4700, 4799, "cli/context"},
	{4800, 4899, "cli/review"},
	{4900, 4999, "cli/ask"},
	{5000, 5099, "cli/docs"},
	{5100, 5199, "cli/init"},
	// M2/M3 verb ranges added for spec §4 gap-fill.
	{5200, 5299, "cli/learn"},
	{5300, 5399, "cli/deploy"},
	{5400, 5499, "cli/agents"},
	{5500, 5549, "cli/report"},
	{5550, 5599, "cli/undo"},
	{5600, 5699, "cli/optimize"},
	{5700, 5799, "cli/add"},
	{5800, 5849, "internal/learningloop"},
	{5850, 5899, "internal/reviewsla"},
	{5900, 5949, "internal/airgap"},
	{5950, 5999, "cli/sla"},
	// spec §4 gap-fill: fixtures + backup.
	{6000, 6099, "cli/fixtures"},
	{6100, 6199, "cli/backup"},
	// DEV-M3-30/M3-31: pre-push gate + post-push CI monitor.
	{6200, 6299, "cli/ci"},
	// ADR-026: knowledge-base injection.
	{6300, 6399, "internal/knowledge"},
	// Template enhancement (TEMPLATE_ENHANCEMENT_SPEC.md).
	{6400, 6449, "internal/tsd"},
	{6450, 6499, "internal/scaffold/compose"},
	{6500, 6549, "internal/cli/cmdtsd"},
	// DEV-M3-bugfix: post-delivery bug fix workflow.
	{6550, 6599, "cli/bugfix"},
	// forge skill: VS Code Copilot expert role installer.
	{6700, 6799, "cli/skill"},
	// forge mcp: Model Context Protocol server for AI chat integrations.
	{6800, 6899, "cli/mcp"},
	// forge metrics: Prometheus token-usage export.
	{6600, 6649, "cli/metrics"},
	// forge companion: zero-setup AI pairing.
	{6650, 6699, "cli/companion"},
	// forge agent: host-agent bridge for `forge ship --agent-mode` (no API key).
	{6900, 6999, "cli/agent"},
	{9000, 9099, "internal/test"},
}

// IsReserved reports whether c falls in any reserved range.
func IsReserved(c Code) bool {
	n := int(c)
	for _, r := range reservedRanges {
		if n >= r.Lo && n <= r.Hi {
			return true
		}
	}
	return false
}

// Format renders a Code as the canonical `FORGE-XXXX` string.
func (c Code) Format() string {
	return fmt.Sprintf("FORGE-%04d", int(c))
}

func (c Code) String() string { return c.Format() }

// entry is a registered code with its short description and remedy.
type entry struct {
	Code        Code
	Description string
	Remedy      string
}

var (
	regMu    sync.RWMutex
	registry = map[Code]entry{}
)

// Register declares a code with a one-line description. Panics on duplicate or
// out-of-range. Call from package-level init() so collisions fail at build.
func Register(c Code, description string) Code {
	return RegisterWithRemedy(c, description, "")
}

// RegisterWithRemedy declares a code with a description and a copy-pasteable
// remedy command. Panics on duplicate or out-of-range.
func RegisterWithRemedy(c Code, description, remedy string) Code {
	if !IsReserved(c) {
		panic(fmt.Sprintf("errcode.Register: %s is outside any reserved range", c))
	}
	if description == "" {
		panic(fmt.Sprintf("errcode.Register: %s missing description", c))
	}
	regMu.Lock()
	defer regMu.Unlock()
	if existing, ok := registry[c]; ok {
		panic(fmt.Sprintf("errcode.Register: duplicate %s (already %q)", c, existing.Description))
	}
	registry[c] = entry{Code: c, Description: description, Remedy: remedy}
	return c
}

// Description returns the registered description, or "" if unknown.
func Description(c Code) string {
	regMu.RLock()
	defer regMu.RUnlock()
	return registry[c].Description
}

// Remedy returns the registered remedy for c, or "" if unknown.
func Remedy(c Code) string {
	regMu.RLock()
	defer regMu.RUnlock()
	return registry[c].Remedy
}

// All returns a sorted snapshot of all registered codes; used by `forge explain
// errors` and the auto-generated docs (DEV-M0-03 TC-03-04).
func All() []Code {
	regMu.RLock()
	defer regMu.RUnlock()
	out := make([]Code, 0, len(registry))
	for c := range registry {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// Error pairs a Code with an underlying cause and a human-readable hint.
type Error struct {
	Code  Code
	Hint  string // actionable remediation (one sentence, no stack traces)
	Cause error
}

// New builds an Error. Cause may be nil.
func New(c Code, hint string, cause error) *Error {
	return &Error{Code: c, Hint: hint, Cause: cause}
}

// Newf is the printf variant (renders into Hint).
func Newf(c Code, cause error, format string, args ...any) *Error {
	return &Error{Code: c, Hint: fmt.Sprintf(format, args...), Cause: cause}
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	desc := Description(e.Code)
	switch {
	case e.Hint != "" && e.Cause != nil:
		return fmt.Sprintf("%s %s: %s (%v)", e.Code, desc, e.Hint, e.Cause)
	case e.Hint != "":
		return fmt.Sprintf("%s %s: %s", e.Code, desc, e.Hint)
	case e.Cause != nil:
		return fmt.Sprintf("%s %s: %v", e.Code, desc, e.Cause)
	default:
		return fmt.Sprintf("%s %s", e.Code, desc)
	}
}

// Unwrap exposes the cause for errors.Is / errors.As.
func (e *Error) Unwrap() error { return e.Cause }

// Is matches by Code so `errors.Is(err, errcode.New(C, "", nil))` works.
func (e *Error) Is(target error) bool {
	var t *Error
	if !errors.As(target, &t) {
		return false
	}
	return e.Code == t.Code
}

// ForgeCode implements the llmresponse.forgeErr interface: returns the
// canonical FORGE-XXXX string for structured JSON error envelopes.
func (e *Error) ForgeCode() string {
	if e == nil {
		return ""
	}
	return e.Code.Format()
}

// ForgeRemedy implements the llmresponse.forgeErr interface: returns the
// registered remedy for this error code, falling back to the Hint field.
func (e *Error) ForgeRemedy() string {
	if e == nil {
		return ""
	}
	if r := Remedy(e.Code); r != "" {
		return r
	}
	return e.Hint
}
