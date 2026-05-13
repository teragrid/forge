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
