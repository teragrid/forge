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

// Package procspawn implements DEV-M0-07: a process-spawn service with an
// explicit allow-list. Only binaries whose base name (or full path) appears in
// the allow-list may be executed. The deny-by-default design prevents Forge
// tools or LLM-generated code from executing arbitrary system commands.
//
// The Spawner is safe to use from multiple goroutines.
package procspawn

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/teragrid/forge/internal/errcode"
)

// Reserved error codes (range 2700..2799).
var (
	ErrNotAllowed = errcode.Register(errcode.Code(2700), "binary not in spawn allow-list")
	ErrTimeout    = errcode.Register(errcode.Code(2701), "spawned process exceeded time limit")
	ErrRunFailed  = errcode.Register(errcode.Code(2702), "spawned process exited non-zero")
)

// DefaultTimeout is applied when Options.Timeout is zero.
const DefaultTimeout = 60 * time.Second

// Result holds the output of a completed spawn.
type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
	Duration time.Duration
}

// Options controls a single spawn invocation.
type Options struct {
	// Dir is the working directory. Defaults to the current directory.
	Dir string
	// Env is additional environment variables in KEY=VALUE form.
	// These are *added* to a minimal, safe base environment (PATH only).
	Env []string
	// Timeout caps the process wall time. Zero → DefaultTimeout.
	Timeout time.Duration
}

// Spawner executes allow-listed binaries.
type Spawner struct {
	mu        sync.RWMutex
	allowList map[string]struct{}
}

// New returns a Spawner that permits only the named binaries.
// Each entry may be a bare binary name (e.g. "go", "git") or an absolute path.
func New(allowed ...string) *Spawner {
	s := &Spawner{allowList: make(map[string]struct{}, len(allowed))}
	for _, a := range allowed {
		s.allowList[strings.TrimSpace(a)] = struct{}{}
	}
	return s
}

// Allow adds additional binaries to the allow-list at runtime.
func (s *Spawner) Allow(binary string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.allowList[strings.TrimSpace(binary)] = struct{}{}
}

// Allowed returns true if the binary is currently in the allow-list.
func (s *Spawner) Allowed(binary string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.isAllowed(binary)
}

// Run executes the allow-listed binary with args. Returns ErrNotAllowed if the
// binary is not in the allow-list, ErrTimeout if the process exceeds its time
// limit, or ErrRunFailed if the process exits non-zero.
func (s *Spawner) Run(binary string, args []string, opts Options) (*Result, error) {
	s.mu.RLock()
	allowed := s.isAllowed(binary)
	s.mu.RUnlock()

	if !allowed {
		return nil, errcode.New(ErrNotAllowed,
			fmt.Sprintf("binary %q is not in the spawn allow-list", binary), nil)
	}

	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, binary, args...) //nolint:gosec // binary validated above
	if opts.Dir != "" {
		cmd.Dir = opts.Dir
	}
	// Minimal safe environment: inherit only PATH from the host.
	cmd.Env = append([]string{"PATH=" + pathEnv()}, opts.Env...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	err := cmd.Run()
	dur := time.Since(start)

	res := &Result{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		Duration: dur,
	}

	if ctx.Err() != nil {
		return res, errcode.New(ErrTimeout,
			fmt.Sprintf("binary %q exceeded %s timeout", binary, timeout), ctx.Err())
	}
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			res.ExitCode = exitErr.ExitCode()
		}
		return res, errcode.New(ErrRunFailed,
			fmt.Sprintf("binary %q exited %d: %s", binary, res.ExitCode, strings.TrimSpace(res.Stderr)),
			err)
	}
	return res, nil
}

// isAllowed checks whether binary (bare name or full path) is in the allow-list.
// Must be called with the read-lock held.
// On Windows (and any OS that appends extensions), the file extension is also
// stripped so that "cmd" matches "cmd.exe".
func (s *Spawner) isAllowed(binary string) bool {
	if _, ok := s.allowList[binary]; ok {
		return true
	}
	// Also accept the base name so callers passing full paths still match.
	base := filepath.Base(binary)
	if _, ok := s.allowList[base]; ok {
		return true
	}
	// Strip extension (e.g. .exe on Windows) and check again.
	if ext := filepath.Ext(base); ext != "" {
		name := strings.TrimSuffix(base, ext)
		if _, ok := s.allowList[name]; ok {
			return true
		}
	}
	return false
}

// pathEnv returns the PATH environment variable string from the current process.
func pathEnv() string {
	for _, e := range exec.Command("env").Environ() { //nolint:gosec // safe; just reading env
		if strings.HasPrefix(e, "PATH=") {
			return strings.TrimPrefix(e, "PATH=")
		}
	}
	return "/usr/local/bin:/usr/bin:/bin"
}
