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

// Tests for the banner package.
// These tests intentionally do not use t.Parallel() because they share and
// reset the package-level sync.Once guard.
package banner

import (
	"bytes"
	"strings"
	"sync"
	"testing"
)

// TestIsColorEnabled_NoColor verifies that NO_COLOR=1 disables color output.
func TestIsColorEnabled_NoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	if isColorEnabled() {
		t.Error("isColorEnabled must return false when NO_COLOR is set")
	}
}

// TestIsColorEnabled_DumbTerm verifies that TERM=dumb disables color output.
func TestIsColorEnabled_DumbTerm(t *testing.T) {
	t.Setenv("TERM", "dumb")
	if isColorEnabled() {
		t.Error("isColorEnabled must return false when TERM=dumb")
	}
}

// TestIsColorEnabled_Both verifies that both conditions together disable color.
func TestIsColorEnabled_Both(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	t.Setenv("TERM", "dumb")
	if isColorEnabled() {
		t.Error("isColorEnabled must return false when NO_COLOR=1 and TERM=dumb")
	}
}

// TestPrint_ColorDisabled_NoANSI verifies that Print produces output with no
// ANSI escape codes when NO_COLOR is set.
func TestPrint_ColorDisabled_NoANSI(t *testing.T) {
	// Not parallel: modifies package-level once.
	t.Setenv("NO_COLOR", "1")
	once = sync.Once{} // reset idempotency guard
	defer func() { once = sync.Once{} }()

	var buf bytes.Buffer
	Print(&buf)
	got := buf.String()
	if got == "" {
		t.Fatal("Print must write something to the writer")
	}
	if strings.Contains(got, "\033[") {
		t.Error("ANSI escape codes must not appear in output when NO_COLOR=1")
	}
}

// TestPrint_Idempotent verifies that Print is a no-op after the first call.
func TestPrint_Idempotent(t *testing.T) {
	// Not parallel: modifies package-level once.
	t.Setenv("NO_COLOR", "1")
	once = sync.Once{} // reset idempotency guard
	defer func() { once = sync.Once{} }()

	var first, second bytes.Buffer
	Print(&first)
	Print(&second) // second call must be a no-op (banner already printed)

	if first.Len() == 0 {
		t.Fatal("Print must write output on the first call")
	}
	if second.Len() != 0 {
		t.Errorf("Print must be a no-op after the first call; second call wrote %d bytes", second.Len())
	}
}
