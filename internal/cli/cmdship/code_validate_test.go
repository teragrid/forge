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

package cmdship

import (
	"strings"
	"testing"
)

// ISSUE 13: a real Jest suite ending in `});` was judged "truncated" by the
// Markdown completeness check and replaced by a red stub.
func TestValidateCode_AcceptsRealTestSuites(t *testing.T) {
	ts := "import { describe, it, expect } from '@jest/globals';\n" +
		"describe('x', () => {\n  it('a # b // c', () => {\n    expect(`}`).toBe('}');\n  });\n});"
	cases := map[string]string{
		"bare":        ts,
		"fenced":      "```typescript\n" + ts + "\n```",
		"prose+fence": "Here is the suite:\n\n```ts\n" + ts + "\n```\n\nHope it helps.",
	}
	for name, raw := range cases {
		code, ok := validateCode(raw, "typescript")
		if !ok {
			t.Errorf("%s: valid suite rejected", name)
		}
		if strings.Contains(code, "```") || strings.Contains(code, "Hope it helps") {
			t.Errorf("%s: fence or prose leaked into code: %q", name, code)
		}
	}
}

func TestValidateCode_RejectsTruncation(t *testing.T) {
	cases := map[string]string{
		"open brace":     "describe('x', () => {\n  it('a', () => {\n    expect(1).toBe(1);\n",
		"open fence":     "```ts\ndescribe('x', () => {});\n",
		"empty":          "  ",
		"stray close":    "});\n",
		"unterminated s": "describe('x', () => {\n  it('a', () => {\n    expect(foo(\n",
	}
	for name, raw := range cases {
		if _, ok := validateCode(raw, "typescript"); ok {
			t.Errorf("%s: truncated/invalid answer accepted", name)
		}
	}
}

func TestValidateCode_Python(t *testing.T) {
	py := "import pytest\n\n# a (comment\ndef test_a():\n    s = \"\"\"multi ( line\"\"\"\n    assert [1, 2] == [1, 2]\n"
	if _, ok := validateCode(py, "python"); !ok {
		t.Fatal("valid pytest file rejected")
	}
	if _, ok := validateCode("def test_a():\n    assert foo(\n", "python"); ok {
		t.Fatal("truncated pytest file accepted")
	}
}

func TestGenerateCodeWithValidation_UsesSubmittedAnswer(t *testing.T) {
	answer := "describe('real', () => {\n  it('works', () => { expect(1).toBe(1); });\n});"
	calls := 0
	got, ok, err := generateCodeWithValidation("typescript", func() (string, bool, error) {
		calls++
		return answer, false, nil
	})
	if err != nil || !ok || !strings.Contains(got, "describe('real'") || calls != 1 {
		t.Fatalf("got=%q ok=%v err=%v calls=%d", got, ok, err, calls)
	}
}

// ISSUE 7: the first qa-verify failure must list every required file and
// heading, not reveal them one failure at a time.
func TestManualTestPlanGate_FailureListsFullChecklist(t *testing.T) {
	root := t.TempDir()
	res := manualTestPlanGate.Handler(HookContext{Root: root, SpecName: "f", Result: &Checkpoint{Status: "ok"}})
	if res.Verdict != VerdictFail {
		t.Fatalf("want fail, got %v", res.Verdict)
	}
	for _, want := range []string{"testing-pipeline.md", "qa-report.md", "Security Reviewer", "DevOps / SRE", "Compliance / CPO", "production"} {
		if !strings.Contains(res.Message, want) {
			t.Errorf("failure message missing %q: %s", want, res.Message)
		}
	}
}
