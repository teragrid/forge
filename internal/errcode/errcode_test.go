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
package errcode

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// Test design (per always-write-tests.md, applicable points only):
//   * Happy: Register + Description roundtrip.
//   * Negative: out-of-range Register panics.
//   * Negative: duplicate Register panics with both descriptions.
//   * Boundary: code on range boundary (lo, hi) accepted.
//   * Data-accuracy: Format() always 4-digit zero-padded.
//   * Regression: Error.Is matches by Code regardless of cause/hint.
//   * False-positive guard: Description on unregistered returns "" (no panic).

func TestCode_Format(t *testing.T) {
	t.Parallel()
	cases := map[Code]string{
		1:    "FORGE-0001",
		42:   "FORGE-0042",
		1099: "FORGE-1099",
		9000: "FORGE-9000",
	}
	for c, want := range cases {
		if got := c.Format(); got != want {
			t.Errorf("%d.Format() = %q, want %q", int(c), got, want)
		}
	}
}

func TestRegister_Happy(t *testing.T) {
	t.Parallel()
	const c Code = 9001
	got := Register(c, "test code")
	if got != c {
		t.Fatalf("Register returned %v, want %v", got, c)
	}
	if Description(c) != "test code" {
		t.Fatalf("Description(%v) = %q, want %q", c, Description(c), "test code")
	}
}

func TestRegister_OutOfRange_Panics(t *testing.T) {
	t.Parallel()
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for out-of-range code")
		}
		if !strings.Contains(fmt.Sprint(r), "outside any reserved range") {
			t.Fatalf("panic message = %v, want contains 'outside any reserved range'", r)
		}
	}()
	Register(Code(7777), "should panic")
}

func TestRegister_Duplicate_Panics(t *testing.T) {
	t.Parallel()
	const c Code = 9002
	Register(c, "first")
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for duplicate code")
		}
		msg := fmt.Sprint(r)
		if !strings.Contains(msg, "duplicate") || !strings.Contains(msg, "first") {
			t.Fatalf("panic message = %v, want both 'duplicate' and 'first'", r)
		}
	}()
	Register(c, "second")
}

func TestRegister_BoundariesAccepted(t *testing.T) {
	t.Parallel()
	// 9000 and 9099 are the lo/hi of the test-internal range.
	Register(Code(9050), "boundary lo+50")
	if !IsReserved(9000) || !IsReserved(9099) {
		t.Fatal("range endpoints should be reserved")
	}
	if IsReserved(8999) || IsReserved(9100) {
		t.Fatal("just outside endpoints must be unreserved")
	}
}

func TestError_FormatString(t *testing.T) {
	t.Parallel()
	const c Code = 9003
	Register(c, "thing failed")
	e := New(c, "do X then retry", errors.New("eof"))
	got := e.Error()
	for _, want := range []string{"FORGE-9003", "thing failed", "do X then retry", "eof"} {
		if !strings.Contains(got, want) {
			t.Errorf("Error() = %q, want contains %q", got, want)
		}
	}
}

func TestError_Is_MatchesByCode(t *testing.T) {
	t.Parallel()
	const c Code = 9004
	Register(c, "match-by-code")
	e1 := New(c, "first", errors.New("a"))
	e2 := New(c, "second", errors.New("b"))
	if !errors.Is(e1, e2) {
		t.Fatal("errors.Is should match by Code")
	}
	other := New(Code(9005), "other", nil)
	Register(Code(9005), "other")
	if errors.Is(e1, other) {
		t.Fatal("different Codes must not match")
	}
}

func TestDescription_Unregistered_Empty(t *testing.T) {
	t.Parallel()
	if got := Description(Code(9098)); got != "" {
		t.Fatalf("Description(unregistered) = %q, want empty", got)
	}
}

// TestErrorCodes_NoDuplicates verifies that All() returns no duplicate codes (G-081).
func TestErrorCodes_NoDuplicates(t *testing.T) {
	t.Parallel()
	all := All()
	seen := make(map[Code]bool, len(all))
	for _, c := range all {
		if seen[c] {
			t.Errorf("duplicate error code registered: %d", c)
		}
		seen[c] = true
	}
}

// TestErrorCodes_AllIsSorted verifies that All() returns codes in ascending order (G-081).
func TestErrorCodes_AllIsSorted(t *testing.T) {
	t.Parallel()
	all := All()
	for i := 1; i < len(all); i++ {
		if all[i] <= all[i-1] {
			t.Errorf("All() not sorted at index %d: %d not greater than %d", i, all[i], all[i-1])
		}
	}
}

// TestRegisterWithRemedy_Happy verifies RegisterWithRemedy stores remedy text (T-008).
func TestRegisterWithRemedy_Happy(t *testing.T) {
	t.Parallel()
	const c Code = 9060
	RegisterWithRemedy(c, "test with remedy", "forge doctor  # run diagnostics")
	if Remedy(c) != "forge doctor  # run diagnostics" {
		t.Fatalf("Remedy(%d) = %q, want remedy text", int(c), Remedy(c))
	}
}

// TestForgeErr_Interface verifies *Error implements ForgeCode/ForgeRemedy (T-008).
func TestForgeErr_Interface(t *testing.T) {
	t.Parallel()
	const c Code = 9061
	RegisterWithRemedy(c, "interface test", "retry after fix")
	e := New(c, "hint text", nil)
	if e.ForgeCode() != "FORGE-9061" {
		t.Fatalf("ForgeCode() = %q, want FORGE-9061", e.ForgeCode())
	}
	if e.ForgeRemedy() != "retry after fix" {
		t.Fatalf("ForgeRemedy() = %q, want 'retry after fix'", e.ForgeRemedy())
	}
}

// TestForgeErr_HintFallback verifies ForgeRemedy falls back to Hint when no remedy registered (T-008).
func TestForgeErr_HintFallback(t *testing.T) {
	t.Parallel()
	const c Code = 9062
	Register(c, "no remedy registered")
	e := New(c, "check the logs", nil)
	if e.ForgeRemedy() != "check the logs" {
		t.Fatalf("ForgeRemedy() should fall back to Hint; got %q", e.ForgeRemedy())
	}
}

// TestRemedy_Unregistered_Empty verifies Remedy returns "" for unknown code (false-positive guard).
func TestRemedy_Unregistered_Empty(t *testing.T) {
	t.Parallel()
	if got := Remedy(Code(9097)); got != "" {
		t.Fatalf("Remedy(unregistered) = %q, want empty string", got)
	}
}
