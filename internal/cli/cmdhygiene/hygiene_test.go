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

// Gap test for G-060: hygiene drift detection.
package cmdhygiene_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/teragrid/forge/internal/cli/cmdhygiene"
)

// TestHygieneDriftDetection verifies that RunHygieneCheck detects files that
// are listed in the hygiene manifest as required but are absent from the
// repository (i.e. drift has occurred).
func TestHygieneDriftDetection(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	// Write a hygiene.yml that declares two required files.
	m := &cmdhygiene.HygieneManifest{
		RequiredFiles: []string{"CHANGELOG.md", "SECURITY.md"},
	}
	if err := cmdhygiene.SaveHygieneManifest(root, m); err != nil {
		t.Fatalf("SaveHygieneManifest: %v", err)
	}

	// Neither CHANGELOG.md nor SECURITY.md exist in root → both are drifted.
	res, err := cmdhygiene.RunHygieneCheck(root)
	if err != nil {
		t.Fatalf("RunHygieneCheck: %v", err)
	}

	if len(res.MissingRequired) != 2 {
		t.Errorf("MissingRequired: want 2, got %d: %v", len(res.MissingRequired), res.MissingRequired)
	}
	if res.Passed {
		t.Error("Passed should be false when required files are missing")
	}
}

// TestHygieneDriftDetection_NoDrift verifies that RunHygieneCheck reports no
// missing required files when all declared files actually exist.
func TestHygieneDriftDetection_NoDrift(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	// Create the required file.
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# test"), 0o644); err != nil {
		t.Fatalf("create README.md: %v", err)
	}

	m := &cmdhygiene.HygieneManifest{
		RequiredFiles: []string{"README.md"},
	}
	if err := cmdhygiene.SaveHygieneManifest(root, m); err != nil {
		t.Fatalf("SaveHygieneManifest: %v", err)
	}

	res, err := cmdhygiene.RunHygieneCheck(root)
	if err != nil {
		t.Fatalf("RunHygieneCheck: %v", err)
	}

	if len(res.MissingRequired) != 0 {
		t.Errorf("MissingRequired: want 0, got %v", res.MissingRequired)
	}
	if !res.Passed {
		t.Error("Passed should be true when all required files exist and no scratch files")
	}
}

// TestHygieneDriftDetection_EmptyManifest verifies that an empty hygiene
// manifest produces no violations.
func TestHygieneDriftDetection_EmptyManifest(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// No hygiene.yml — LoadHygieneManifest returns empty manifest.
	res, err := cmdhygiene.RunHygieneCheck(root)
	if err != nil {
		t.Fatalf("RunHygieneCheck: %v", err)
	}
	if !res.Passed {
		t.Error("Passed should be true for empty manifest")
	}
}
