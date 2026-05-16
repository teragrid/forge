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

// G-020: Integration test — all scanner families produce ≥1 finding against
// the canonical fixture project at tests/fixtures/canonical-project/.
package cmdscan_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/teragrid/forge/internal/cli/cmdscan"
)

// canonicalFixtureRoot resolves the path to tests/fixtures/canonical-project/
// relative to this test file's location, which is in internal/cli/cmdscan/.
func canonicalFixtureRoot() string {
	// This file lives at: internal/cli/cmdscan/canonical_project_test.go
	// Repo root is 3 directories above internal/cli/cmdscan/:
	//   cmdscan/ -> cli/ -> internal/ -> <repo root>
	_, thisFile, _, _ := runtime.Caller(0)
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..", "..")
	return filepath.Join(repoRoot, "tests", "fixtures", "canonical-project")
}

// TestAllScannerFamilies_CanonicalProject verifies that each of the 8 scanner
// families produces ≥1 finding when run against the canonical fixture project.
// This is the G-020 acceptance criterion.
func TestAllScannerFamilies_CanonicalProject(t *testing.T) {
	root := canonicalFixtureRoot()

	families := []struct {
		name string
		fn   func(string) (*cmdscan.ScanResult, error)
	}{
		{"security", cmdscan.RunSecurity},
		{"correctness", cmdscan.RunCorrectness},
		{"performance", cmdscan.RunPerformance},
		{"reliability", cmdscan.RunReliability},
		{"accessibility", cmdscan.RunAccessibility},
		{"cost", cmdscan.RunCost},
		{"compliance", cmdscan.RunCompliance},
		{"dx", cmdscan.RunDX},
	}

	for _, f := range families {
		f := f
		t.Run(f.name, func(t *testing.T) {
			t.Parallel()
			res, err := f.fn(root)
			if err != nil {
				t.Fatalf("[%s] scanner returned error: %v", f.name, err)
			}
			if res == nil {
				t.Fatalf("[%s] scanner returned nil result", f.name)
			}
			if len(res.Findings) == 0 {
				t.Errorf("[%s] expected ≥1 finding against canonical-project fixture, got 0", f.name)
			}
		})
	}
}
