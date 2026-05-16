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

package promptcompiler_test

import (
	"strings"
	"testing"

	"github.com/teragrid/forge/internal/promptcompiler"
)

// TestCompile_StripCommentLines verifies that //-- lines are removed.
func TestCompile_StripCommentLines(t *testing.T) {
	t.Parallel()
	src := "//-- developer note\nYou are a helpful assistant.\n//-- another note\nAnswer concisely."
	got := promptcompiler.Compile(src)
	if strings.Contains(got, "//--") {
		t.Errorf("Compile() left comment lines: %q", got)
	}
	if !strings.Contains(got, "You are a helpful assistant.") {
		t.Errorf("Compile() removed non-comment lines: %q", got)
	}
}

// TestCompile_CollapseBlankLines verifies consecutive blanks become one.
func TestCompile_CollapseBlankLines(t *testing.T) {
	t.Parallel()
	src := "line1\n\n\n\nline2"
	got := promptcompiler.Compile(src)
	if strings.Contains(got, "\n\n\n") {
		t.Errorf("Compile() left more than one consecutive blank line: %q", got)
	}
	if !strings.Contains(got, "line1") || !strings.Contains(got, "line2") {
		t.Errorf("Compile() removed content lines: %q", got)
	}
}

// TestCompile_TrimLeadingTrailingBlanks checks edge trimming.
func TestCompile_TrimLeadingTrailingBlanks(t *testing.T) {
	t.Parallel()
	src := "\n\n  \nhello\n\n  \n"
	got := promptcompiler.Compile(src)
	if strings.HasPrefix(got, "\n") {
		t.Errorf("Compile() left leading blank: %q", got)
	}
	if strings.HasSuffix(got, "\n") {
		t.Errorf("Compile() left trailing blank: %q", got)
	}
}

// TestCompile_EmptyInput returns empty string.
func TestCompile_EmptyInput(t *testing.T) {
	t.Parallel()
	if got := promptcompiler.Compile(""); got != "" {
		t.Errorf("Compile(\"\") = %q, want empty", got)
	}
}

// TestCompile_OnlyComments returns empty string.
func TestCompile_OnlyComments(t *testing.T) {
	t.Parallel()
	src := "//-- note one\n//-- note two\n"
	got := promptcompiler.Compile(src)
	if got != "" {
		t.Errorf("Compile(only comments) = %q, want empty", got)
	}
}

// TestCompile_TokenReduction verifies that compiled output is shorter or equal.
func TestCompile_TokenReduction(t *testing.T) {
	t.Parallel()
	src := "//-- preamble note (do not ship)\n" +
		"You are a code reviewer.\n" +
		"//-- TODO: remove before production\n\n\n\n" +
		"Review the following code for bugs.\n\n\n" +
		"Return JSON with fields: issues, severity, suggestion.\n"
	got := promptcompiler.Compile(src)
	if len(got) >= len(src) {
		t.Errorf("Compile() did not reduce length: before=%d after=%d", len(src), len(got))
	}
}

// TestCompile_PreservesContent ensures meaningful content is not lost.
func TestCompile_PreservesContent(t *testing.T) {
	t.Parallel()
	src := "//-- ignore\nSystem: be helpful.\n\nUser: {{query}}\n"
	got := promptcompiler.Compile(src)
	for _, want := range []string{"System: be helpful.", "User: {{query}}"} {
		if !strings.Contains(got, want) {
			t.Errorf("Compile() dropped required content %q from %q", want, got)
		}
	}
}

// TestCompile_Idempotent verifies compiling twice produces same result.
func TestCompile_Idempotent(t *testing.T) {
	t.Parallel()
	src := "//-- note\nYou are helpful.\n\n\nAnswer concisely."
	once := promptcompiler.Compile(src)
	twice := promptcompiler.Compile(once)
	if once != twice {
		t.Errorf("Compile() is not idempotent:\nonce:  %q\ntwice: %q", once, twice)
	}
}
