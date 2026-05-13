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
// Package eval implements the forge scenario harness (DEV-M2-04).
//
// A Scenario is a deterministic regression test for a forge verb (or any
// command). Each Scenario has one or more Steps; each Step runs a command
// and asserts on its exit code, stdout (substring or JSON-equality), and
// stderr (substring). Scenarios are stored as JSON files so the harness
// has no third-party dependencies.
//
// The harness is the substrate the LLM-prompt eval suite (M3) will sit on
// top of: a "prompt" step type can be added without changing the runner
// shape — it just becomes another command-with-assertions.
package eval

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Scenario is one regression test.
type Scenario struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	// Workdir, if set to "tmpdir", runs each step in a fresh tempdir
	// (deleted after the scenario). If empty, uses the current dir.
	// Any other value is treated as an absolute or relative path.
	Workdir string `json:"workdir,omitempty"`
	Steps   []Step `json:"steps"`
}

// Step is one command-with-assertions.
type Step struct {
	ID     string   `json:"id,omitempty"`
	Run    []string `json:"run"` // argv; e.g. ["forge", "scan", "secrets", "--json"]
	Expect Expect   `json:"expect"`
}

// Expect describes the assertions for a Step.
type Expect struct {
	// Exit, if non-nil, asserts on the exit code.
	Exit *int `json:"exit,omitempty"`
	// StdoutContains: every string here must appear in stdout.
	StdoutContains []string `json:"stdout_contains,omitempty"`
	// StdoutNotContains: none of these strings may appear in stdout
	// (false-positive guard).
	StdoutNotContains []string `json:"stdout_not_contains,omitempty"`
	// StderrContains: every string here must appear in stderr.
	StderrContains []string `json:"stderr_contains,omitempty"`
	// StdoutJSON, if set, parses stdout as JSON and asserts every key in
	// this map equals the parsed value (deep equality on JSON-serialised forms).
	StdoutJSON map[string]any `json:"stdout_json,omitempty"`
}

// Result is the outcome of a single step.
type StepResult struct {
	StepID   string `json:"step_id"`
	Cmd      string `json:"cmd"`
	ExitCode int    `json:"exit_code"`
	Passed   bool   `json:"passed"`
	Reason   string `json:"reason,omitempty"`
	Duration string `json:"duration"`
}

// ScenarioResult aggregates per-step results.
type ScenarioResult struct {
	Name     string       `json:"name"`
	Passed   bool         `json:"passed"`
	Steps    []StepResult `json:"steps"`
	Duration string       `json:"duration"`
}

// Report is the top-level eval output.
type Report struct {
	Total     int              `json:"total"`
	Passed    int              `json:"passed"`
	Failed    int              `json:"failed"`
	Scenarios []ScenarioResult `json:"scenarios"`
}

// Validate checks the scenario is well-formed.
func (s Scenario) Validate() error {
	if strings.TrimSpace(s.Name) == "" {
		return fmt.Errorf("scenario: name is required")
	}
	for i, st := range s.Steps {
		if len(st.Run) == 0 {
			return fmt.Errorf("scenario %q step %d: run argv is required", s.Name, i)
		}
	}
	return nil
}

// LoadScenario reads a single .json scenario file.
func LoadScenario(path string) (*Scenario, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read scenario %s: %w", path, err)
	}
	var s Scenario
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, fmt.Errorf("parse scenario %s: %w", path, err)
	}
	if err := s.Validate(); err != nil {
		return nil, err
	}
	return &s, nil
}

// LoadDir recursively loads every *.scenario.json under root.
// Returns the scenarios sorted by Name (deterministic order).
func LoadDir(root string) ([]*Scenario, error) {
	var paths []string
	err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if strings.HasSuffix(info.Name(), ".scenario.json") {
			paths = append(paths, p)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	out := make([]*Scenario, 0, len(paths))
	for _, p := range paths {
		s, err := LoadScenario(p)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, nil
}

// Runner executes Scenarios.
type Runner struct {
	// CommandResolver maps argv[0] -> absolute path to executable.
	// Default uses exec.LookPath. Tests may override to point at a stub.
	CommandResolver func(name string) (string, error)
	// Env is appended to the inherited environment for every step.
	Env []string
}

// NewRunner returns a Runner with default lookup.
func NewRunner() *Runner {
	return &Runner{CommandResolver: exec.LookPath}
}

// Run executes one scenario and returns its result.
func (r *Runner) Run(ctx context.Context, s *Scenario) ScenarioResult {
	start := time.Now()
	res := ScenarioResult{Name: s.Name, Passed: true}

	wd, cleanup, err := r.prepareWorkdir(s.Workdir)
	if err != nil {
		res.Passed = false
		res.Steps = append(res.Steps, StepResult{
			Reason: fmt.Sprintf("prepare workdir: %v", err),
		})
		res.Duration = time.Since(start).String()
		return res
	}
	defer cleanup()

	for i, step := range s.Steps {
		sr := r.runStep(ctx, wd, step, i)
		res.Steps = append(res.Steps, sr)
		if !sr.Passed {
			res.Passed = false
		}
	}
	res.Duration = time.Since(start).String()
	return res
}

// RunAll executes a slice of scenarios and returns an aggregated Report.
func (r *Runner) RunAll(ctx context.Context, scenarios []*Scenario) Report {
	rep := Report{Total: len(scenarios)}
	for _, s := range scenarios {
		sr := r.Run(ctx, s)
		rep.Scenarios = append(rep.Scenarios, sr)
		if sr.Passed {
			rep.Passed++
		} else {
			rep.Failed++
		}
	}
	return rep
}

func (r *Runner) prepareWorkdir(spec string) (string, func(), error) {
	noop := func() {}
	switch spec {
	case "":
		wd, err := os.Getwd()
		return wd, noop, err
	case "tmpdir":
		dir, err := os.MkdirTemp("", "forge-eval-*")
		if err != nil {
			return "", noop, err
		}
		return dir, func() { _ = os.RemoveAll(dir) }, nil
	default:
		return spec, noop, nil
	}
}

func (r *Runner) runStep(ctx context.Context, wd string, step Step, idx int) StepResult {
	start := time.Now()
	id := step.ID
	if id == "" {
		id = fmt.Sprintf("step-%d", idx)
	}
	sr := StepResult{StepID: id, Cmd: strings.Join(step.Run, " ")}

	resolver := r.CommandResolver
	if resolver == nil {
		resolver = exec.LookPath
	}
	bin, err := resolver(step.Run[0])
	if err != nil {
		sr.Reason = fmt.Sprintf("resolve %q: %v", step.Run[0], err)
		sr.Duration = time.Since(start).String()
		return sr
	}

	cmd := exec.CommandContext(ctx, bin, step.Run[1:]...)
	cmd.Dir = wd
	if len(r.Env) > 0 {
		cmd.Env = append(os.Environ(), r.Env...)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	sr.ExitCode = cmd.ProcessState.ExitCode()
	sr.Duration = time.Since(start).String()

	if reason := assert(step.Expect, sr.ExitCode, stdout.String(), stderr.String()); reason != "" {
		sr.Reason = reason
		// runErr is informational; assertion outcome is authoritative.
		_ = runErr
		return sr
	}
	sr.Passed = true
	return sr
}

// assert returns "" on pass, or a human-readable reason on failure.
func assert(e Expect, exitCode int, stdout, stderr string) string {
	if e.Exit != nil && *e.Exit != exitCode {
		return fmt.Sprintf("exit: want %d, got %d", *e.Exit, exitCode)
	}
	for _, s := range e.StdoutContains {
		if !strings.Contains(stdout, s) {
			return fmt.Sprintf("stdout missing substring %q", s)
		}
	}
	for _, s := range e.StdoutNotContains {
		if strings.Contains(stdout, s) {
			return fmt.Sprintf("stdout contains forbidden substring %q", s)
		}
	}
	for _, s := range e.StderrContains {
		if !strings.Contains(stderr, s) {
			return fmt.Sprintf("stderr missing substring %q", s)
		}
	}
	if len(e.StdoutJSON) > 0 {
		var got map[string]any
		if err := json.Unmarshal([]byte(stdout), &got); err != nil {
			return fmt.Sprintf("stdout is not valid JSON: %v", err)
		}
		for k, want := range e.StdoutJSON {
			gv, ok := got[k]
			if !ok {
				return fmt.Sprintf("stdout JSON missing key %q", k)
			}
			if !jsonEqual(want, gv) {
				return fmt.Sprintf("stdout JSON %q: want %v, got %v", k, want, gv)
			}
		}
	}
	return ""
}

// jsonEqual compares two values via canonical JSON encoding.
func jsonEqual(a, b any) bool {
	ja, err1 := json.Marshal(a)
	jb, err2 := json.Marshal(b)
	if err1 != nil || err2 != nil {
		return false
	}
	return bytes.Equal(ja, jb)
}
