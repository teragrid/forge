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
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeEmptyFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("test('x', () => {});\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestFeatureTestFiles_MatchesSlugAcrossSeparators(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mine := []string{
		filepath.Join(root, "tests", "unit", "add_rate_limit.test.ts"),
		filepath.Join(root, "src", "__tests__", "addRateLimit.spec.ts"),
		filepath.Join(root, "tests", "add-rate-limit.integration.test.ts"),
	}
	others := []string{
		filepath.Join(root, "tests", "unit", "billing.test.ts"),
		filepath.Join(root, "tests", "e2e", "login.spec.ts"),
	}
	all := append(append([]string{}, mine...), others...)
	for _, p := range all {
		writeEmptyFile(t, p)
	}

	got := featureTestFiles(root, "add-rate-limit", all)
	if len(got) != len(mine) {
		t.Fatalf("got %d feature test files %v, want %d %v", len(got), got, len(mine), mine)
	}
	for _, o := range others {
		for _, g := range got {
			if g == o {
				t.Errorf("unrelated test file leaked into the feature's set: %s", o)
			}
		}
	}
}

func TestFeatureTestFiles_EmptySlugMatchesNothingByName(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	p := filepath.Join(root, "tests", "anything.test.ts")
	writeEmptyFile(t, p)
	if got := featureTestFiles(root, "", []string{p}); len(got) != 0 {
		t.Fatalf("an empty slug must not match every file, got %v", got)
	}
}

// TestCheckTest_ReportsFeatureTestsNotRepoTotal is the regression guard for
// "671 test file(s) found; missing artifacts: ..." — the headline number was
// the whole repository's test count and said nothing about the feature, and a
// repo with hundreds of unrelated tests earned the checkpoint an unqualified
// ok while none of them belonged to it.
func TestCheckTest_ReportsFeatureTestsNotRepoTotal(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for _, n := range []string{"a", "b", "c", "d", "e"} {
		writeEmptyFile(t, filepath.Join(root, "tests", "unit", n+".test.ts"))
	}
	cp := checkTest(root, "add rate limiting", "", nil, true)
	if cp.Status == "ok" && strings.Contains(cp.Detail, "missing") {
		t.Fatalf("no feature tests exist — status must not be an unqualified ok: %+v", cp)
	}
	if !strings.Contains(cp.Detail, "0 test file(s) for this feature (5 in repo)") {
		t.Errorf("detail should separate the feature's tests from the repo total: %s", cp.Detail)
	}
	if cp.Status != "warning" {
		t.Errorf("status = %q, want warning when the feature has no tests of its own", cp.Status)
	}
}

// False-positive guard: once the feature has its own tests the checkpoint is ok.
func TestCheckTest_FeatureHasOwnTests_StaysOK(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeEmptyFile(t, filepath.Join(root, "tests", "add-rate-limiting.test.ts"))
	writeEmptyFile(t, filepath.Join(root, "tests", "other.test.ts"))
	cp := checkTest(root, "add rate limiting", "", nil, true)
	if cp.Status != "ok" {
		t.Fatalf("status = %q, want ok; detail: %s", cp.Status, cp.Detail)
	}
	if !strings.Contains(cp.Detail, "1 test file(s) for this feature (2 in repo)") {
		t.Errorf("unexpected detail: %s", cp.Detail)
	}
}
