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

package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// TestVerbDocCoverage implements G-150: coverage check for verb documentation.
//
// Every verb registered on the root command should have a corresponding
// Markdown doc file at docs/verbs/<verb>.md. This test discovers the workspace
// root, enumerates all registered verbs, and reports any that lack a doc file.
//
// The test does not hard-fail on missing docs (docs are P3 / advisory) but it
// WILL fail if the docs/verbs/ directory cannot be found at all, which guards
// against the check silently passing on a mis-configured workspace.
func TestVerbDocCoverage(t *testing.T) {
	t.Parallel()

	// Locate the repository root relative to this test file.
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed — cannot determine repo root")
	}
	// thisFile is …/internal/cli/verb_docs_test.go; root is 3 levels up.
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..")
	verbsDir := filepath.Join(repoRoot, "docs", "verbs")

	// Structural guard: the docs/verbs/ directory must exist.
	if _, err := os.Stat(verbsDir); err != nil {
		t.Fatalf("G-150: docs/verbs/ directory not found at %s: %v", verbsDir, err)
	}

	root := NewRootCommand("0.0.0-dev")
	var missing []string
	var documented []string

	for _, cmd := range root.Commands() {
		verb := cmd.Name()
		docPath := filepath.Join(verbsDir, verb+".md")
		if _, err := os.Stat(docPath); err == nil {
			documented = append(documented, verb)
		} else {
			missing = append(missing, verb)
		}
	}

	t.Logf("G-150 verb doc coverage: %d/%d verbs have docs/verbs/<verb>.md",
		len(documented), len(documented)+len(missing))

	if len(missing) > 0 {
		t.Logf("G-150 missing docs (P3 advisory — add docs/verbs/<verb>.md for each):")
		for _, v := range missing {
			t.Logf("  - docs/verbs/%s.md", v)
		}
	}

	// Hard-fail only if ZERO verbs are documented — that would indicate the
	// check itself is broken (wrong path, missing directory, etc.).
	if len(documented) == 0 {
		t.Fatalf("G-150: no verb docs found in %s — coverage check is broken", verbsDir)
	}
}
