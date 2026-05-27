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
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
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

// ── Issue #15: forge hygiene manifest sync ────────────────────────────────────

// writeHygieneINI writes a minimal INI-format manifest file used by forge hygiene.
func writeHygieneINI(t *testing.T, path string, scratch, managed []string) {
	t.Helper()
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	var b strings.Builder
	b.WriteString("[scratch]\n")
	for _, p := range scratch {
		b.WriteString(p + "\n")
	}
	b.WriteString("[managed]\n")
	for _, p := range managed {
		b.WriteString(p + "\n")
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		t.Fatal(err)
	}
}

// TC-H15-01: hygiene manifest sync reports when hygiene.yml has patterns
// not present in .forge/manifest (exits non-zero).
func TestManifestSync_OutOfSync(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeHygieneINI(t, filepath.Join(root, ".forge", "manifest"), []string{"_scratch_*"}, nil)
	writeHygieneINI(t, filepath.Join(root, ".forge", "hygiene.yml"), []string{"_scratch_*", "fix_*"}, nil)

	cmd := cmdhygiene.New()
	var out strings.Builder
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"manifest", "sync", "--root", root})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected non-zero exit when files are out of sync")
	}
	if !strings.Contains(out.String(), "fix_*") {
		t.Errorf("expected 'fix_*' mentioned in output, got: %s", out.String())
	}
}

// TC-H15-02: hygiene manifest sync exits 0 when files are in sync.
func TestManifestSync_InSync(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	patterns := []string{"_scratch_*", "tmp_*"}
	writeHygieneINI(t, filepath.Join(root, ".forge", "manifest"), patterns, nil)
	writeHygieneINI(t, filepath.Join(root, ".forge", "hygiene.yml"), patterns, nil)

	cmd := cmdhygiene.New()
	var out strings.Builder
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"manifest", "sync", "--root", root})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected no error when in sync, got: %v\noutput: %s", err, out.String())
	}
	if !strings.Contains(out.String(), "in sync") {
		t.Errorf("expected 'in sync' in output, got: %s", out.String())
	}
}

// TC-H15-03 (JSON mode): --json flag emits valid JSON with in_sync field.
func TestManifestSync_JSONOutput(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeHygieneINI(t, filepath.Join(root, ".forge", "manifest"), []string{"_scratch_*"}, nil)
	writeHygieneINI(t, filepath.Join(root, ".forge", "hygiene.yml"), []string{"_scratch_*", "extra_*"}, nil)

	cmd := cmdhygiene.New()
	var out strings.Builder
	cmd.SetOut(&out)
	cmd.SetErr(io.Discard)
	cmd.SilenceUsage = true
	cmd.SetArgs([]string{"manifest", "sync", "--root", root, "--json"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected non-zero exit when patterns differ")
	}

	var result struct {
		InHygieneOnly  []string `json:"in_hygiene_only"`
		InManifestOnly []string `json:"in_manifest_only"`
		InSync         bool     `json:"in_sync"`
	}
	// Extract JSON from output (may have trailing newline from error)
	body := strings.TrimSpace(out.String())
	if err := json.Unmarshal([]byte(body), &result); err != nil {
		t.Fatalf("--json output is not valid JSON: %v\noutput: %s", err, out.String())
	}
	if result.InSync {
		t.Error("in_sync should be false when patterns differ")
	}
}

// TC-H15-04 (false-positive guard): manifest has extra pattern not in hygiene.yml.
func TestManifestSync_ManifestOnlyPattern(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeHygieneINI(t, filepath.Join(root, ".forge", "manifest"), []string{"_scratch_*", "only_in_manifest"}, nil)
	writeHygieneINI(t, filepath.Join(root, ".forge", "hygiene.yml"), []string{"_scratch_*"}, nil)

	cmd := cmdhygiene.New()
	var out strings.Builder
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"manifest", "sync", "--root", root})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when manifest has extra patterns")
	}
	if !strings.Contains(out.String(), "only_in_manifest") {
		t.Errorf("expected 'only_in_manifest' in output, got: %s", out.String())
	}
}
