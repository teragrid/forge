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

// --- dependabot-baseline ---

// TC-CODEMOD-09 (happy + data-accuracy): writes file with detected ecosystems.
func TestDependabot_WritesWithDetectedEcosystems(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// Drop a go.mod and a package.json so two ecosystems should be emitted.
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	rep, err := dependabotBaseline{}.Apply(dir, false)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if rep.Changed != 1 || len(rep.Files) != 1 {
		t.Fatalf("expected Changed=1 + 1 file, got %+v", rep)
	}
	body, _ := os.ReadFile(filepath.Join(dir, ".github", "dependabot.yml"))
	s := string(body)
	if !strings.Contains(s, `package-ecosystem: "gomod"`) {
		t.Errorf("missing gomod ecosystem:\n%s", s)
	}
	if !strings.Contains(s, `package-ecosystem: "npm"`) {
		t.Errorf("missing npm ecosystem:\n%s", s)
	}
	if !strings.Contains(s, "version: 2") {
		t.Errorf("missing version: 2")
	}
}

// TC-CODEMOD-10 (idempotency / false-positive guard): existing file is preserved.
func TestDependabot_KeepsExisting(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".github"), 0o755); err != nil {
		t.Fatal(err)
	}
	custom := "# my custom\nversion: 2\nupdates: []\n"
	path := filepath.Join(dir, ".github", "dependabot.yml")
	if err := os.WriteFile(path, []byte(custom), 0o600); err != nil {
		t.Fatal(err)
	}
	rep, err := dependabotBaseline{}.Apply(dir, false)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if rep.Changed != 0 {
		t.Fatalf("should keep existing, Changed=%d", rep.Changed)
	}
	body, _ := os.ReadFile(path)
	if string(body) != custom {
		t.Fatalf("file modified: %s", body)
	}
}

// TC-CODEMOD-11 (boundary): dry-run reports change but writes nothing.
func TestDependabot_DryRunNoMutation(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	rep, err := dependabotBaseline{}.Apply(dir, true)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if rep.Changed != 1 {
		t.Errorf("dry-run should report Changed=1, got %d", rep.Changed)
	}
	if _, err := os.Stat(filepath.Join(dir, ".github", "dependabot.yml")); !os.IsNotExist(err) {
		t.Fatal("dry-run wrote file")
	}
}

// TC-CODEMOD-12 (boundary): empty project still produces a valid file with default ecosystem.
func TestDependabot_EmptyProjectDefault(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if _, err := (dependabotBaseline{}).Apply(dir, false); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	body, _ := os.ReadFile(filepath.Join(dir, ".github", "dependabot.yml"))
	if !strings.Contains(string(body), `package-ecosystem: "github-actions"`) {
		t.Fatalf("expected default github-actions ecosystem:\n%s", body)
	}
}

// TC-CODEMOD-13 (data-accuracy): detectEcosystems is sorted and dedupes pip variants.
func TestDetectEcosystems_DedupAndSort(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	must := func(p string) {
		if err := os.WriteFile(filepath.Join(dir, p), []byte(""), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	must("requirements.txt")
	must("pyproject.toml")
	must("go.mod")
	got := detectEcosystems(dir)
	want := []string{"gomod", "pip"}
	if len(got) != len(want) {
		t.Fatalf("want %v, got %v", want, got)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("idx %d: want %q got %q", i, w, got[i])
		}
	}
}

// --- pre-commit-baseline ---

// TC-CODEMOD-14 (happy): writes file with go-hook when go.mod present.
func TestPreCommit_GoProject(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	rep, err := preCommitBaseline{}.Apply(dir, false)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if rep.Changed != 1 {
		t.Fatalf("Changed=%d", rep.Changed)
	}
	body, _ := os.ReadFile(filepath.Join(dir, ".pre-commit-config.yaml"))
	s := string(body)
	if !strings.Contains(s, "trailing-whitespace") || !strings.Contains(s, "gitleaks") {
		t.Errorf("baseline hooks missing:\n%s", s)
	}
	if !strings.Contains(s, "go-fmt") {
		t.Errorf("go hook missing on go project:\n%s", s)
	}
}

// TC-CODEMOD-15 (false-positive guard): non-go project does NOT include go-fmt hook.
func TestPreCommit_NonGoProject_NoGoHook(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	rep, err := preCommitBaseline{}.Apply(dir, false)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if rep.Changed != 1 {
		t.Fatalf("Changed=%d", rep.Changed)
	}
	body, _ := os.ReadFile(filepath.Join(dir, ".pre-commit-config.yaml"))
	if strings.Contains(string(body), "go-fmt") {
		t.Fatalf("go-fmt should NOT appear without go.mod:\n%s", body)
	}
}

// TC-CODEMOD-16 (idempotency / false-positive guard): existing file is preserved.
func TestPreCommit_KeepsExisting(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	custom := "# mine\nrepos: []\n"
	path := filepath.Join(dir, ".pre-commit-config.yaml")
	if err := os.WriteFile(path, []byte(custom), 0o600); err != nil {
		t.Fatal(err)
	}
	rep, err := preCommitBaseline{}.Apply(dir, false)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if rep.Changed != 0 {
		t.Fatalf("Changed=%d", rep.Changed)
	}
	body, _ := os.ReadFile(path)
	if string(body) != custom {
		t.Fatalf("file modified")
	}
}

// TC-CODEMOD-17 (boundary): dry-run reports change but writes nothing.
func TestPreCommit_DryRunNoMutation(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	rep, err := preCommitBaseline{}.Apply(dir, true)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if rep.Changed != 1 {
		t.Errorf("Changed=%d", rep.Changed)
	}
	if _, err := os.Stat(filepath.Join(dir, ".pre-commit-config.yaml")); !os.IsNotExist(err) {
		t.Fatal("dry-run wrote file")
	}
}

// TC-CODEMOD-18 (data-accuracy / regression): both new codemods registered with Default().
func TestDefault_HasNewBaselines(t *testing.T) {
	t.Parallel()
	names := map[string]bool{}
	for _, c := range Default().All() {
		names[c.Name()] = true
	}
	for _, want := range []string{"dependabot-baseline", "pre-commit-baseline"} {
		if !names[want] {
			t.Errorf("missing built-in codemod %q", want)
		}
	}
}
