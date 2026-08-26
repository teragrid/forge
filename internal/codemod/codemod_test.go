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
package codemod

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TC-CODEMOD-01 (happy + data-accuracy): gitignore-marker inserts block.
func TestGitignoreMarker_InsertsBlock(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("node_modules/\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	c := gitignoreMarker{}
	rep, err := c.Apply(dir, false)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if rep.Changed != 1 {
		t.Fatalf("expected Changed=1, got %d", rep.Changed)
	}
	body, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if !strings.Contains(string(body), markerStart) || !strings.Contains(string(body), markerEnd) {
		t.Fatalf("markers missing: %s", body)
	}
	if !strings.Contains(string(body), "node_modules/") {
		t.Fatal("existing content lost")
	}
}

// TC-CODEMOD-02 (idempotency): apply twice → second is no-op.
func TestGitignoreMarker_Idempotent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	c := gitignoreMarker{}
	if _, err := c.Apply(dir, false); err != nil {
		t.Fatalf("first: %v", err)
	}
	rep, err := c.Apply(dir, false)
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if rep.Changed != 0 {
		t.Fatalf("second apply should be no-op, Changed=%d", rep.Changed)
	}
	if rep.Detail != "no change" {
		t.Errorf("expected 'no change' detail, got %q", rep.Detail)
	}
}

// TC-CODEMOD-03 (boundary): dry-run does not modify file.
func TestGitignoreMarker_DryRunNoMutation(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("ignore-me\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	rep, err := gitignoreMarker{}.Apply(dir, true)
	if err != nil {
		t.Fatalf("dry-run: %v", err)
	}
	if rep.Changed != 1 {
		t.Errorf("dry-run should still report Changed=1, got %d", rep.Changed)
	}
	body, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if strings.Contains(string(body), markerStart) {
		t.Fatal("dry-run should not have written marker block")
	}
}

// TC-CODEMOD-04 (regression): refresh updates an outdated marker block in-place.
func TestGitignoreMarker_RefreshesExistingBlock(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	old := "before\n# forge:gitignore:start\nold-content\n# forge:gitignore:end\nafter\n"
	_ = os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(old), 0o600)
	if _, err := (gitignoreMarker{}).Apply(dir, false); err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))
	s := string(body)
	if strings.Contains(s, "old-content") {
		t.Fatal("old marker content not replaced")
	}
	if !strings.Contains(s, "before") || !strings.Contains(s, "after") {
		t.Fatal("surrounding content lost")
	}
}

// TC-CODEMOD-05 (happy): gitleaks-baseline writes file when missing.
func TestGitleaksBaseline_WritesIfMissing(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	rep, err := gitleaksBaseline{}.Apply(dir, false)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if rep.Changed != 1 {
		t.Fatalf("expected Changed=1, got %d", rep.Changed)
	}
	body, _ := os.ReadFile(filepath.Join(dir, ".gitleaks.toml"))
	if !strings.Contains(string(body), "aws-access-key") {
		t.Fatal("baseline body missing")
	}
}

// TC-CODEMOD-06 (false-positive guard): if file already exists, do nothing.
func TestGitleaksBaseline_KeepsExisting(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	custom := "title = \"my custom\"\n"
	_ = os.WriteFile(filepath.Join(dir, ".gitleaks.toml"), []byte(custom), 0o600)
	rep, err := gitleaksBaseline{}.Apply(dir, false)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if rep.Changed != 0 {
		t.Fatalf("should keep existing, Changed=%d", rep.Changed)
	}
	body, _ := os.ReadFile(filepath.Join(dir, ".gitleaks.toml"))
	if string(body) != custom {
		t.Fatalf("file modified: %s", body)
	}
}

// TC-CODEMOD-07 (negative): registry duplicate panics.
func TestRegistry_DuplicatePanics(t *testing.T) {
	t.Parallel()
	r := NewRegistry()
	r.Register(gitignoreMarker{})
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on duplicate")
		}
	}()
	r.Register(gitignoreMarker{})
}

// TC-CODEMOD-08 (data-accuracy): Default registry has both built-in codemods.
func TestDefault_HasBuiltins(t *testing.T) {
	t.Parallel()
	names := map[string]bool{}
	for _, c := range Default().All() {
		names[c.Name()] = true
	}
	for _, want := range []string{"gitignore-marker", "gitleaks-baseline"} {
		if !names[want] {
			t.Errorf("missing built-in codemod %q", want)
		}
	}
}

// TestDefaultMarkerBody_CoversForgeScratchState guards the third copy of the
// same managed .gitignore content (alongside canonicalGitignoreBlock in
// hygiene.go and canonicalGiSnippet in cmddoctor/doctor.go) — see
// TestCanonicalGitignoreBlock_CoversForgeScratchState for the root cause.
func TestDefaultMarkerBody_CoversForgeScratchState(t *testing.T) {
	t.Parallel()
	for _, pattern := range []string{
		".forge/scratch/",
		".forge/cache/",
		".forge/.snapshots/",
		".forge/agent/",
		".forge/learned/",
		".forge/trash/",
		".forge/token-ledger.jsonl",
	} {
		if !strings.Contains(defaultMarkerBody, pattern) {
			t.Errorf("defaultMarkerBody missing %q", pattern)
		}
	}
}
