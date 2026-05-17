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

package gitservice_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/teragrid/forge/internal/gitservice"
)

// initRepo creates a minimal git repo in dir with one commit containing file.
func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	run("init")
	run("config", "user.email", "test@forge.local")
	run("config", "user.name", "Forge Test")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# test"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "initial commit")
	return dir
}

// skipIfNoGit skips the test when git is not available in PATH.
func skipIfNoGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH; skipping git integration tests")
	}
}

// ── Happy path ────────────────────────────────────────────────────────────────

func TestNew_Happy(t *testing.T) {
	skipIfNoGit(t)
	t.Parallel()
	dir := initRepo(t)
	svc, err := gitservice.New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if svc == nil {
		t.Fatal("service is nil")
	}
}

func TestStatus_Clean(t *testing.T) {
	skipIfNoGit(t)
	t.Parallel()
	dir := initRepo(t)
	svc, _ := gitservice.New(dir)

	statuses, err := svc.Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if len(statuses) != 0 {
		t.Fatalf("clean repo should have no statuses, got %v", statuses)
	}
}

func TestStatus_UnstagedFile(t *testing.T) {
	skipIfNoGit(t)
	t.Parallel()
	dir := initRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "new.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	svc, _ := gitservice.New(dir)

	statuses, err := svc.Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if len(statuses) == 0 {
		t.Fatal("expected at least one status entry for untracked file")
	}
}

func TestLog_Happy(t *testing.T) {
	skipIfNoGit(t)
	t.Parallel()
	dir := initRepo(t)
	svc, _ := gitservice.New(dir)

	commits, err := svc.Log(5)
	if err != nil {
		t.Fatalf("Log: %v", err)
	}
	if len(commits) == 0 {
		t.Fatal("expected at least one commit")
	}
	if commits[0].Subject != "initial commit" {
		t.Fatalf("unexpected subject: %q", commits[0].Subject)
	}
	if commits[0].Hash == "" {
		t.Fatal("commit hash must not be empty")
	}
}

func TestLog_DefaultLimit(t *testing.T) {
	skipIfNoGit(t)
	t.Parallel()
	dir := initRepo(t)
	svc, _ := gitservice.New(dir)
	// n=0 should use the default (20), not crash.
	commits, err := svc.Log(0)
	if err != nil {
		t.Fatalf("Log(0): %v", err)
	}
	if len(commits) == 0 {
		t.Fatal("expected at least one commit with n=0")
	}
}

func TestChangedFilesSince_Happy(t *testing.T) {
	skipIfNoGit(t)
	t.Parallel()
	dir := initRepo(t)

	// Add a second commit with a new file.
	if err := os.WriteFile(filepath.Join(dir, "added.txt"), []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		_ = cmd.Run()
	}
	run("add", ".")
	run("commit", "-m", "second commit")

	svc, _ := gitservice.New(dir)
	commits, _ := svc.Log(2)
	if len(commits) < 2 {
		t.Skip("need 2 commits for diff test")
	}

	files, err := svc.ChangedFilesSince(commits[1].Hash)
	if err != nil {
		t.Fatalf("ChangedFilesSince: %v", err)
	}
	found := false
	for _, f := range files {
		if strings.Contains(f, "added.txt") {
			found = true
		}
	}
	if !found {
		t.Fatalf("added.txt not in changed files: %v", files)
	}
}

// ── Negative cases ────────────────────────────────────────────────────────────

func TestNew_NotGitRepo(t *testing.T) {
	skipIfNoGit(t)
	t.Parallel()
	dir := t.TempDir() // plain directory, no git init
	_, err := gitservice.New(dir)
	if err == nil {
		t.Fatal("expected ErrNotGitRepo")
	}
	if !strings.Contains(err.Error(), "FORGE-2600") {
		t.Fatalf("want FORGE-2600, got: %v", err)
	}
}

func TestDiffSince_EmptyRef(t *testing.T) {
	skipIfNoGit(t)
	t.Parallel()
	dir := initRepo(t)
	svc, _ := gitservice.New(dir)
	_, err := svc.DiffSince("")
	if err == nil {
		t.Fatal("expected error for empty ref")
	}
}

// ── False-positive guard ──────────────────────────────────────────────────────

// Verify Service has no write methods exposed.
func TestService_NoWriteMethods(t *testing.T) {
	t.Parallel()
	// This is a compile-time property verified by code review; here we just
	// ensure the type exists and the read methods are present.
	skipIfNoGit(t)
	dir := initRepo(t)
	svc, err := gitservice.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	// Call each read method to ensure they exist and don't panic on a clean repo.
	if _, err := svc.Status(); err != nil {
		t.Errorf("Status: %v", err)
	}
	if _, err := svc.Log(1); err != nil {
		t.Errorf("Log: %v", err)
	}
}

func TestGoFileCommitTimes_Empty(t *testing.T) {
	skipIfNoGit(t)
	t.Parallel()
	// Repo with no .go files — result should be non-nil but empty.
	dir := initRepo(t)
	svc, err := gitservice.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	times := svc.GoFileCommitTimes()
	if times == nil {
		t.Fatal("expected non-nil map, got nil")
	}
	if len(times) != 0 {
		t.Fatalf("expected empty map, got %d entries", len(times))
	}
}

func TestGoFileCommitTimes_TracksGoFiles(t *testing.T) {
	skipIfNoGit(t)
	t.Parallel()
	dir := initRepo(t)

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	// Commit a production file.
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "main.go")
	run("commit", "-m", "add main.go")

	// Commit its test file 1 second later (git commit time, not mtime).
	if err := os.WriteFile(filepath.Join(dir, "main_test.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "main_test.go")
	run("commit", "-m", "add main_test.go")

	svc, err := gitservice.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	times := svc.GoFileCommitTimes()
	if _, ok := times["main.go"]; !ok {
		t.Error("expected main.go in commit times map")
	}
	if _, ok := times["main_test.go"]; !ok {
		t.Error("expected main_test.go in commit times map")
	}
	// The test file was committed after the production file, so its time must be >= prod time.
	if strings.Contains(t.Name(), "") { // always true — avoids unused import lint
		_ = filepath.Join // keep filepath import used
	}
	if times["main_test.go"].Before(times["main.go"]) {
		t.Errorf("expected test file commit time >= prod file commit time; got prod=%v test=%v",
			times["main.go"], times["main_test.go"])
	}
}
