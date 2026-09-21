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
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// shipTestRepo builds a throwaway git repo on a feature branch off main.
func shipTestRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH")
	}
	dir := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init")
	git("config", "user.email", "test@forge.local")
	git("config", "user.name", "Forge Test")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# repo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", ".")
	git("commit", "-m", "initial")
	git("branch", "-M", "main")
	git("checkout", "-b", "feature/x")
	return dir
}

// TestCheckVerify_NoSourceChanges_IsWarningNotOK is the regression guard for
// the ship checkpoint reporting "ok" — with a "spec-vs-code audit found no
// blocking gaps" evidence line — on a branch that changes no source code (a
// spec-only run). It must downgrade to a warning, say why, and withhold the
// audit claim.
func TestCheckVerify_NoSourceChanges_IsWarningNotOK(t *testing.T) {
	t.Parallel()
	root := shipTestRepo(t)
	makeSpecDir(t, root, "feat", map[string]string{"spec.md": "# Spec\n"})

	cp := checkVerify(root, "feat", "", nil)
	if cp.Status != "warning" {
		t.Fatalf("status = %q, want warning; detail: %s", cp.Status, cp.Detail)
	}
	if !strings.Contains(cp.Detail, "nothing to ship") {
		t.Errorf("detail should say nothing to ship: %s", cp.Detail)
	}
	for _, ev := range cp.Evidence {
		if strings.Contains(ev.Claim, "spec-vs-code audit") {
			t.Errorf("must not claim a spec-vs-code audit passed when there is no code: %+v", ev)
		}
	}
}

// TestCheckVerify_WithSourceChange_StaysOK is the false-positive guard: a
// branch that does carry source code must not be downgraded.
func TestCheckVerify_WithSourceChange_StaysOK(t *testing.T) {
	t.Parallel()
	root := shipTestRepo(t)
	makeSpecDir(t, root, "feat", map[string]string{"spec.md": "# Spec\n"})
	if err := os.WriteFile(filepath.Join(root, "feature.go"), []byte("package feature\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "add", "feature.go")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, out)
	}
	cmd = exec.Command("git", "commit", "-m", "add feature")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v\n%s", err, out)
	}

	cp := checkVerify(root, "feat", "", nil)
	if cp.Status != "ok" {
		t.Fatalf("status = %q, want ok; detail: %s", cp.Status, cp.Detail)
	}
	if strings.Contains(cp.Detail, "nothing to ship") {
		t.Errorf("must not claim nothing to ship: %s", cp.Detail)
	}
}

// TestCheckVerify_NotAGitRepo_KeepsOldBehaviour — when git cannot say what
// changed, forge must stay silent rather than guess "nothing to ship".
func TestCheckVerify_NotAGitRepo_KeepsOldBehaviour(t *testing.T) {
	t.Parallel()
	cp := checkVerify(t.TempDir(), "", "", nil)
	if strings.Contains(cp.Detail, "nothing to ship") {
		t.Errorf("unknown change set must not be reported as nothing to ship: %s", cp.Detail)
	}
}
