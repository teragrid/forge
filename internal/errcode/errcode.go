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

// entry is a registered code with its short description.
type entry struct {
	Code        Code
	Description string
}

var (
	regMu    sync.RWMutex
	registry = map[Code]entry{}
)

// Register declares a code with a one-line description. Panics on duplicate or
// out-of-range. Call from package-level init() so collisions fail at build.
func Register(c Code, description string) Code {
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
	registry[c] = entry{Code: c, Description: description}
	return c
}

// Description returns the registered description, or "" if unknown.
func Description(c Code) string {
	regMu.RLock()
	defer regMu.RUnlock()
	return registry[c].Description
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
