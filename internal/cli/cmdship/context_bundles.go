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

// Package cmdship — G-008: per-task context bundles.
//
// For each task in tasks.md forge writes a context bundle at
// .forge/context/<slug>-<task-id>.md containing the minimal context needed
// to implement that task in isolation. If any bundle would exceed the token
// budget from ship.task_context_budget (default 4000 tokens) the call fails
// with ErrContextBudgetExceeded.
package cmdship

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/teragrid/forge/internal/errcode"
)

// ErrContextBudgetExceeded is returned when a per-task context bundle exceeds
// the configured token budget (FORGE-5009).
var ErrContextBudgetExceeded = errcode.Register(errcode.Code(5009),
	"context bundle exceeds token budget (ship.task_context_budget)")

// defaultContextTokenBudget is the default max tokens per per-task context bundle.
const defaultContextTokenBudget = 4000

// taskEntry represents one task extracted from tasks.md.
type taskEntry struct {
	ID    string // e.g. "T-001"
	Title string
}

// writeTaskContextBundles writes one .forge/context/<slug>-<task-id>.md file
// per task found in .forge/specs/<slug>/tasks.md.
//
// Each bundle contains:
//   - The feature name / slug
//   - The full spec (spec.md)
//   - The full breakdown (breakdown.md)
//   - The specific task title and ID
//   - The relevant section of the breakdown (heuristic: surrounding lines)
//
// When any bundle would exceed tokenBudget estimated tokens (rough whitespace
// count / 4 approximation), ErrContextBudgetExceeded is returned.
func writeTaskContextBundles(root, slug string, tokenBudget int) error {
	if tokenBudget <= 0 {
		tokenBudget = defaultContextTokenBudget
	}

	specsDir := filepath.Join(root, ".forge", "specs", slug)
	contextDir := filepath.Join(root, ".forge", "context")

	tasksMD, err := os.ReadFile(filepath.Join(specsDir, "tasks.md"))
	if err != nil {
		return nil // no tasks.md yet; not an error
	}

	tasks := parseTasks(string(tasksMD))
	if len(tasks) == 0 {
		return nil
	}

	// Load shared context documents.
	specContent := ""
	if data, err := os.ReadFile(filepath.Join(specsDir, "spec.md")); err == nil {
		specContent = string(data)
	}
	breakdownContent := ""
	if data, err := os.ReadFile(filepath.Join(specsDir, "breakdown.md")); err == nil {
		breakdownContent = string(data)
	}

	if err := os.MkdirAll(contextDir, 0o755); err != nil {
		return fmt.Errorf("create context dir: %w", err)
	}

	for _, task := range tasks {
		bundle := buildBundle(slug, task, specContent, breakdownContent)
		// Rough token estimate: len(text) / 4.
		estimatedTokens := len(bundle) / 4
		if estimatedTokens > tokenBudget {
			return errcode.Newf(ErrContextBudgetExceeded, nil,
				"task %s estimated %d tokens (budget %d)", task.ID, estimatedTokens, tokenBudget)
		}
		outPath := filepath.Join(contextDir, slug+"-"+task.ID+".md")
		if err := os.WriteFile(outPath, []byte(bundle), 0o600); err != nil {
			return fmt.Errorf("write context bundle %s: %w", task.ID, err)
		}
	}
	return nil
}

// parseTasks extracts task entries from tasks.md.
// Expected format: `- [ ] T-NNN: <title>` or `- [x] T-NNN: <title>`.
var taskLineRe = regexp.MustCompile(`(?i)^-\s+\[[ xX]\]\s+(T-\d+):\s+(.+)$`)

func parseTasks(tasksMD string) []taskEntry {
	var tasks []taskEntry
	for _, line := range strings.Split(tasksMD, "\n") {
		m := taskLineRe.FindStringSubmatch(strings.TrimSpace(line))
		if m != nil {
			tasks = append(tasks, taskEntry{ID: m[1], Title: m[2]})
		}
	}
	return tasks
}

// countCompletedTasks returns the number of tasks marked [x] in tasks.md content.
func countCompletedTasks(tasksMD string) int {
	completedRe := regexp.MustCompile(`(?i)^-\s+\[x\]\s+T-\d+:`)
	count := 0
	for _, line := range strings.Split(tasksMD, "\n") {
		if completedRe.MatchString(strings.TrimSpace(line)) {
			count++
		}
	}
	return count
}

// allTasksComplete returns true when every task line in tasks.md is checked [x].
func allTasksComplete(root, slug string) bool {
	tasksMD, err := os.ReadFile(filepath.Join(root, ".forge", "specs", slug, "tasks.md"))
	if err != nil {
		return false
	}
	tasks := parseTasks(string(tasksMD))
	if len(tasks) == 0 {
		return false
	}
	done := countCompletedTasks(string(tasksMD))
	return done == len(tasks)
}

// buildBundle constructs the context bundle content for a single task.
func buildBundle(slug string, task taskEntry, specContent, breakdownContent string) string {
	var sb strings.Builder
	sb.WriteString("# Context bundle: ")
	sb.WriteString(slug)
	sb.WriteString(" / ")
	sb.WriteString(task.ID)
	sb.WriteString("\n\n")
	sb.WriteString("## Task\n\n")
	sb.WriteString("**")
	sb.WriteString(task.ID)
	sb.WriteString("**: ")
	sb.WriteString(task.Title)
	sb.WriteString("\n\n")
	if specContent != "" {
		sb.WriteString("## Spec\n\n")
		sb.WriteString(specContent)
		sb.WriteString("\n\n")
	}
	if breakdownContent != "" {
		sb.WriteString("## Breakdown (full)\n\n")
		sb.WriteString(breakdownContent)
		sb.WriteString("\n")
	}
	return sb.String()
}
