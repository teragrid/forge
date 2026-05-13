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

package procspawn_test

import (
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/teragrid/forge/internal/procspawn"
)

// echoCmd returns the platform-appropriate echo command and its args.
func echoCmd() (string, []string) {
	if runtime.GOOS == "windows" {
		return "cmd", []string{"/c", "echo", "hello"}
	}
	return "echo", []string{"hello"}
}

// echoName returns just the binary name for allow-list registration.
func echoName() string {
	if runtime.GOOS == "windows" {
		return "cmd"
	}
	return "echo"
}

// skipIfNoBinary skips the test when the named binary is not available.
func skipIfNoBinary(t *testing.T, binary string) {
	t.Helper()
	if _, err := exec.LookPath(binary); err != nil {
		t.Skipf("%s not found in PATH; skipping", binary)
	}
}

// ── Happy path ────────────────────────────────────────────────────────────────

func TestRun_AllowedBinary(t *testing.T) {
	t.Parallel()
	binary, args := echoCmd()
	skipIfNoBinary(t, binary)

	s := procspawn.New(echoName())
	res, err := s.Run(binary, args, procspawn.Options{})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(res.Stdout, "hello") {
		t.Fatalf("expected 'hello' in stdout, got %q", res.Stdout)
	}
	if res.ExitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", res.ExitCode)
	}
}

func TestAllowed_TrueForListed(t *testing.T) {
	t.Parallel()
	s := procspawn.New("go", "git")
	if !s.Allowed("go") {
		t.Error("expected 'go' to be allowed")
	}
	if !s.Allowed("git") {
		t.Error("expected 'git' to be allowed")
	}
}

// ── Negative cases ────────────────────────────────────────────────────────────

func TestRun_NotAllowed(t *testing.T) {
	t.Parallel()
	s := procspawn.New("go") // only 'go' is allowed
	_, err := s.Run("rm", []string{"-rf", "/"}, procspawn.Options{})
	if err == nil {
		t.Fatal("expected ErrNotAllowed")
	}
	if !strings.Contains(err.Error(), "FORGE-2700") {
		t.Fatalf("expected FORGE-2700, got: %v", err)
	}
}

func TestAllowed_FalseForUnlisted(t *testing.T) {
	t.Parallel()
	s := procspawn.New("go")
	if s.Allowed("rm") {
		t.Error("'rm' should NOT be allowed")
	}
}

func TestRun_EmptyAllowList(t *testing.T) {
	t.Parallel()
	s := procspawn.New() // nothing allowed
	_, err := s.Run("echo", []string{"hi"}, procspawn.Options{})
	if err == nil {
		t.Fatal("expected error with empty allow-list")
	}
	if !strings.Contains(err.Error(), "FORGE-2700") {
		t.Fatalf("expected FORGE-2700, got: %v", err)
	}
}

// ── Boundary cases ────────────────────────────────────────────────────────────

func TestRun_BaseNameMatchesFullPath(t *testing.T) {
	t.Parallel()
	binary, args := echoCmd()
	skipIfNoBinary(t, binary)

	// Register by basename; run by full path lookup.
	s := procspawn.New(echoName())
	full, err := exec.LookPath(binary)
	if err != nil {
		t.Skip("cannot lookup full path")
	}
	res, err := s.Run(full, args, procspawn.Options{})
	if err != nil {
		t.Fatalf("Run with full path: %v", err)
	}
	if res.ExitCode != 0 {
		t.Fatalf("exit code %d", res.ExitCode)
	}
}

func TestRun_CustomTimeout(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("sleep timing test not reliable on Windows")
	}
	skipIfNoBinary(t, "sleep")

	s := procspawn.New("sleep")
	_, err := s.Run("sleep", []string{"5"}, procspawn.Options{
		Timeout: 50 * time.Millisecond,
	})
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(err.Error(), "FORGE-2701") {
		t.Fatalf("expected FORGE-2701, got: %v", err)
	}
}

// ── Allow at runtime ─────────────────────────────────────────────────────────

func TestAllow_RuntimeAddition(t *testing.T) {
	t.Parallel()
	s := procspawn.New()
	if s.Allowed("git") {
		t.Error("git should not be allowed before Allow()")
	}
	s.Allow("git")
	if !s.Allowed("git") {
		t.Error("git should be allowed after Allow()")
	}
}

// ── Concurrency ───────────────────────────────────────────────────────────────

func TestRun_ConcurrentAllowedChecks(t *testing.T) {
	t.Parallel()
	s := procspawn.New("go", "git", "echo")
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.Allowed("go")
			s.Allow("rustc")
			s.Allowed("rustc")
		}()
	}
	wg.Wait()
}

// ── False-positive guard ──────────────────────────────────────────────────────

// Verify that a binary whose name is a prefix of an allowed name is NOT allowed.
func TestAllowed_PrefixDoesNotMatch(t *testing.T) {
	t.Parallel()
	s := procspawn.New("goblin")
	if s.Allowed("go") {
		t.Error("'go' must NOT match allow-list entry 'goblin'")
	}
}
