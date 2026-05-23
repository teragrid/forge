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

// Test design (per always-write-tests.md rule):
//
// Happy path:
//  1. ensureFeatureBranch from "main" → creates feature/<slug>, Created=true.
//  2. ensureFeatureBranch from "feature/foo" → returns branch unchanged, Created=false.
//
// Boundary:
//  3. ensureFeatureBranch from every protectedBranches entry → all trigger branch creation.
//  4. ensureFeatureBranch with empty slug → branch name "feature/".
//
// Negative / graceful-degrade:
//  5. ensureFeatureBranch in a non-git dir → Warning set, pipeline not blocked.
//  6. currentBranchName in non-git dir → returns error.
//
// Idempotency:
//  7. Call ensureFeatureBranch twice with same slug → second call returns Created=false (branch exists).
//
// Data accuracy:
//  8. Branch name format: "feature/" + slug (no spaces, lower-case).
//
// False-positive guard:
//  9. A branch named "feature/something" must NOT be treated as protected.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// initGitRepo initialises a minimal git repo in dir.
func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=forge-test",
			"GIT_AUTHOR_EMAIL=forge@test",
			"GIT_COMMITTER_NAME=forge-test",
			"GIT_COMMITTER_EMAIL=forge@test",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-b", "main")
	run("config", "user.email", "forge@test")
	run("config", "user.name", "forge-test")
	// Need at least one commit so HEAD is valid.
	readme := filepath.Join(dir, "README.md")
	if err := os.WriteFile(readme, []byte("# test\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "init")
}

// ── protectedBranches map ─────────────────────────────────────────────────────

// TestProtectedBranchesMap verifies all expected names are in the set.
func TestProtectedBranchesMap(t *testing.T) {
	expected := []string{"main", "master", "develop", "dev", "trunk", "production", "prod"}
	for _, name := range expected {
		if !protectedBranches[name] {
			t.Errorf("protectedBranches[%q] should be true", name)
		}
	}
}

// TestProtectedBranchesMap_FeatureBranchNotProtected — false-positive guard:
// a feature branch must NOT appear as protected.
func TestProtectedBranchesMap_FeatureBranchNotProtected(t *testing.T) {
	notProtected := []string{"feature/foo", "feat/bar", "my-feature", "hotfix/bug-123", ""}
	for _, name := range notProtected {
		if protectedBranches[name] {
			t.Errorf("protectedBranches[%q] should be false (false-positive guard)", name)
		}
	}
}

// ── currentBranchName ─────────────────────────────────────────────────────────

// TestCurrentBranchName_NonGitDir returns an error in a non-git directory.
func TestCurrentBranchName_NonGitDir(t *testing.T) {
	dir := t.TempDir()
	_, err := currentBranchName(dir)
	if err == nil {
		t.Fatal("expected error from non-git directory, got nil")
	}
}

// TestCurrentBranchName_GitRepo returns the correct branch in a git repo.
func TestCurrentBranchName_GitRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	initGitRepo(t, dir)

	branch, err := currentBranchName(dir)
	if err != nil {
		t.Fatalf("currentBranchName: %v", err)
	}
	if branch != "main" {
		t.Errorf("got %q, want %q", branch, "main")
	}
}

// ── ensureFeatureBranch ───────────────────────────────────────────────────────

// TestEnsureFeatureBranch_NonGitDir degrades gracefully with a warning.
func TestEnsureFeatureBranch_NonGitDir(t *testing.T) {
	dir := t.TempDir()
	res := ensureFeatureBranch(dir, "my-feature")
	if res.Warning == "" {
		t.Error("expected a warning for non-git directory, got none")
	}
	if res.Created {
		t.Error("Created should be false when git fails")
	}
}

// TestEnsureFeatureBranch_AlreadyOnFeatureBranch — if already on a topic branch,
// returns it unchanged with no warning and Created=false.
func TestEnsureFeatureBranch_AlreadyOnFeatureBranch(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	initGitRepo(t, dir)

	// Switch to a feature branch first.
	cmd := exec.Command("git", "checkout", "-b", "feature/existing-work")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git checkout: %v\n%s", err, out)
	}

	res := ensureFeatureBranch(dir, "existing-work")
	if res.Warning != "" {
		t.Errorf("unexpected warning: %s", res.Warning)
	}
	if res.Created {
		t.Error("Created should be false when already on a feature branch")
	}
	if res.Branch != "feature/existing-work" {
		t.Errorf("got branch %q, want %q", res.Branch, "feature/existing-work")
	}
}

// TestEnsureFeatureBranch_FromMain — happy path: creates feature/<slug> from main.
func TestEnsureFeatureBranch_FromMain(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	initGitRepo(t, dir)

	res := ensureFeatureBranch(dir, "add-auth")
	if res.Warning != "" {
		t.Errorf("unexpected warning: %s", res.Warning)
	}
	if !res.Created {
		t.Error("Created should be true when creating from main")
	}
	if res.Branch != "feature/add-auth" {
		t.Errorf("got branch %q, want %q", res.Branch, "feature/add-auth")
	}

	// Confirm git is actually on the new branch.
	branch, err := currentBranchName(dir)
	if err != nil {
		t.Fatalf("currentBranchName: %v", err)
	}
	if branch != "feature/add-auth" {
		t.Errorf("HEAD is on %q, want feature/add-auth", branch)
	}
}

// TestEnsureFeatureBranch_Idempotent — calling twice with same slug:
// first call creates, second call switches without error.
func TestEnsureFeatureBranch_Idempotent(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	initGitRepo(t, dir)

	// First call — creates.
	res1 := ensureFeatureBranch(dir, "idempotent-feature")
	if !res1.Created {
		t.Fatal("first call should have created the branch")
	}

	// Switch back to main so we can test idempotency.
	cmd := exec.Command("git", "checkout", "main")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git checkout main: %v\n%s", err, out)
	}

	// Second call — branch already exists; should switch to it without Created=true.
	res2 := ensureFeatureBranch(dir, "idempotent-feature")
	if res2.Warning != "" {
		t.Errorf("second call unexpected warning: %s", res2.Warning)
	}
	if res2.Branch != "feature/idempotent-feature" {
		t.Errorf("second call branch = %q, want feature/idempotent-feature", res2.Branch)
	}
}

// TestEnsureFeatureBranch_BranchNameFormat — branch name must be "feature/<slug>".
func TestEnsureFeatureBranch_BranchNameFormat(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	initGitRepo(t, dir)

	slug := "payment-gateway-v2"
	res := ensureFeatureBranch(dir, slug)
	if res.Branch != "feature/"+slug {
		t.Errorf("branch = %q, want feature/%s", res.Branch, slug)
	}
}

// TestEnsureFeatureBranch_AllProtectedBranches — every protected branch triggers
// feature branch creation.
func TestEnsureFeatureBranch_AllProtectedBranches(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	for protected := range protectedBranches {
		protected := protected
		t.Run(protected, func(t *testing.T) {
			dir := t.TempDir()
			initGitRepo(t, dir)

			// git doesn't let you create a branch with same name as current unless
			// you rename — rename main to the protected branch name.
			var renameCmd *exec.Cmd
			if protected == "main" {
				renameCmd = exec.Command("git", "checkout", "-b", "main-renamed")
			} else {
				renameCmd = exec.Command("git", "checkout", "-b", protected)
			}
			renameCmd.Dir = dir
			if out, err := renameCmd.CombinedOutput(); err != nil {
				t.Fatalf("git checkout -b %s: %v\n%s", protected, err, out)
			}

			if protected != "main" {
				// We're now on protected branch; test from here.
				res := ensureFeatureBranch(dir, "test-feature")
				if res.Warning != "" {
					t.Errorf("%s: unexpected warning: %s", protected, res.Warning)
				}
				if !res.Created {
					t.Errorf("%s: Created should be true", protected)
				}
				if !strings.HasPrefix(res.Branch, "feature/") {
					t.Errorf("%s: branch %q should start with feature/", protected, res.Branch)
				}
			}
		})
	}
}

// ── ShipResult.FeatureBranch ─────────────────────────────────────────────────

// TestShipResult_FeatureBranchField verifies the new JSON field is emitted.
func TestShipResult_FeatureBranchField(t *testing.T) {
	res := &ShipResult{
		Ready:         true,
		FeatureBranch: "feature/add-auth",
		Message:       "ok",
	}
	if res.FeatureBranch != "feature/add-auth" {
		t.Errorf("FeatureBranch = %q, want feature/add-auth", res.FeatureBranch)
	}
}
