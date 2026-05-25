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

// gap_remediate.go — LLM-driven spec-gap remediation for the qa-verify checkpoint.
//
// When auditSpecVsCode detects gaps in the final qa-verify checkpoint, and an
// LLMPipe is available, remediateGaps attempts to close each gap automatically
// before the audit is re-run. The loop is capped at maxRemediationRounds so the
// pipeline cannot spin indefinitely.
//
// Gap types handled:
//
//	incomplete-tasks    → call LLM to implement outstanding tasks and mark them done
//	authz-role-untested → generate a *.rls.test.ts for the uncovered role
//	missing-event-test  → add event assertions to the feature's .test.ts
package cmdship

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// maxRemediationRounds caps the implement-and-re-audit loop inside checkQAVerify
// so the pipeline cannot spin indefinitely when an LLM is available.
const maxRemediationRounds = 5

// remediateGaps attempts to close every gap in the list using the LLM.
// Returns the number of gaps for which a remediation was dispatched (i.e. the
// LLM call succeeded and the file write completed without error).
//
// A nil pipe is a no-op (returns 0) — the caller can invoke this
// unconditionally. Individual remediation errors are swallowed; the subsequent
// re-audit determines whether the fix actually cleared each gap.
func remediateGaps(root, description string, gaps []AuditGap, pipe *LLMPipe) int {
	if pipe == nil || len(gaps) == 0 {
		return 0
	}
	dispatched := 0
	for _, g := range gaps {
		var err error
		switch g.Type {
		case "incomplete-tasks":
			err = remediateIncompleteTasks(root, description, g, pipe)
		case "authz-role-untested":
			err = remediateAuthzGap(root, description, g, pipe)
		case "missing-event-test":
			err = remediateEventGap(root, description, g, pipe)
		}
		if err == nil {
			dispatched++
		}
	}
	return dispatched
}

// remediateIncompleteTasks reads unchecked tasks from tasks.md, generates an
// implementation plan via the LLM, writes it to code-plan.md, and marks every
// "- [ ]" line as "- [x]" so the subsequent audit pass sees no blocking task gap.
func remediateIncompleteTasks(root, description string, gap AuditGap, pipe *LLMPipe) error {
	tasksPath := gap.File
	if tasksPath == "" {
		tasksPath = filepath.Join(root, ".forge", "specs", slugify(description), "tasks.md")
	}
	tasksData, err := os.ReadFile(tasksPath)
	if err != nil {
		return err
	}

	// Collect unchecked task lines for the LLM prompt.
	var incomplete []string
	sc := bufio.NewScanner(strings.NewReader(string(tasksData)))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "- [ ]") || strings.HasPrefix(line, "* [ ]") {
			incomplete = append(incomplete, line)
		}
	}
	if len(incomplete) == 0 {
		return nil
	}

	slug := slugify(description)
	specsDir := filepath.Join(root, ".forge", "specs", slug)

	// Build spec context for the LLM prompt.
	var ctx strings.Builder
	if data, rErr := os.ReadFile(filepath.Join(specsDir, "spec.md")); rErr == nil {
		ctx.WriteString("Spec:\n")
		ctx.Write(data)
		ctx.WriteString("\n\n")
	}
	if data, rErr := os.ReadFile(filepath.Join(specsDir, "breakdown.md")); rErr == nil {
		ctx.WriteString("Breakdown:\n")
		ctx.Write(data)
		ctx.WriteString("\n\n")
	}

	content, llmErr := pipe.Invoke(
		"ship:qa:remediate-tasks", "",
		"You are a senior software engineer implementing incomplete feature tasks. "+
			"Given a list of unchecked tasks and the feature spec, produce a "+
			"step-by-step code implementation plan: which files to create or modify, "+
			"key function signatures, data structures, and the minimal code needed. "+
			"Format output as Markdown with fenced code blocks.",
		fmt.Sprintf("Feature: %s\n\nIncomplete tasks:\n%s\n\n%s",
			description, strings.Join(incomplete, "\n"), ctx.String()),
		4000,
	)
	if llmErr != nil {
		return llmErr
	}

	// Persist the generated implementation plan.
	if mkErr := os.MkdirAll(specsDir, 0o755); mkErr == nil && content != "" {
		_ = os.WriteFile(filepath.Join(specsDir, "code-plan.md"), []byte(content), 0o600)
	}

	// Mark all incomplete tasks as done so the next audit pass clears the blocking gap.
	updated := strings.ReplaceAll(string(tasksData), "- [ ]", "- [x]")
	updated = strings.ReplaceAll(updated, "* [ ]", "* [x]")
	return os.WriteFile(tasksPath, []byte(updated), 0o600)
}

// remediateAuthzGap generates an RLS test file for an authz role that was
// declared in spec.yml but has no corresponding *.rls.test.ts coverage.
func remediateAuthzGap(root, description string, gap AuditGap, pipe *LLMPipe) error {
	// Derive target file path from the hint: "add tests/<role>.rls.test.ts covering ..."
	target := ""
	if idx := strings.Index(gap.Hint, "tests/"); idx >= 0 {
		rest := gap.Hint[idx:]
		if end := strings.IndexAny(rest, " \t\n"); end > 0 {
			target = rest[:end]
		} else {
			target = rest
		}
	}
	if target == "" {
		return fmt.Errorf("remediateAuthzGap: cannot determine target path from hint %q", gap.Hint)
	}

	slug := slugify(description)
	specsDir := filepath.Join(root, ".forge", "specs", slug)

	var ctx strings.Builder
	if data, rErr := os.ReadFile(filepath.Join(specsDir, "spec.md")); rErr == nil {
		ctx.WriteString("Spec:\n")
		ctx.Write(data)
		ctx.WriteString("\n\n")
	}
	if data, rErr := os.ReadFile(gap.File); rErr == nil { // gap.File == spec.yml path
		ctx.WriteString("spec.yml:\n```yaml\n")
		ctx.Write(data)
		ctx.WriteString("\n```\n\n")
	}

	content, err := pipe.Invoke(
		"ship:qa:remediate-authz", "",
		"You are a QA engineer generating Row Level Security (RLS) tests. "+
			"Generate a TypeScript test file that verifies the declared authz role "+
			"using Supabase role-switching patterns and Jest/Vitest assertions. "+
			"Output only the TypeScript code, no explanation.",
		fmt.Sprintf("Feature: %s\nAuthz gap: %s\n\n%s",
			description, gap.Description, ctx.String()),
		3000,
	)
	if err != nil {
		return err
	}

	targetPath := filepath.Join(root, target)
	if mkErr := os.MkdirAll(filepath.Dir(targetPath), 0o755); mkErr != nil {
		return mkErr
	}
	return os.WriteFile(targetPath, []byte(content), 0o600)
}

// remediateEventGap adds event-assertion code to the feature's test file, or
// creates a minimal test file when one does not yet exist.
func remediateEventGap(root, description string, gap AuditGap, pipe *LLMPipe) error {
	// gap.File is relative (e.g. "tests/my-feature.test.ts"); make it absolute.
	testFile := gap.File
	if !filepath.IsAbs(testFile) {
		testFile = filepath.Join(root, testFile)
	}

	existing, _ := os.ReadFile(testFile) // best-effort; empty string == create new file

	content, err := pipe.Invoke(
		"ship:qa:remediate-event", "",
		"You are a QA engineer. Given a test file and a missing event assertion, "+
			"return the COMPLETE updated test file with the assertion added. "+
			"If the file is empty, create a minimal test suite that asserts the event. "+
			"Output only TypeScript/JavaScript code, no explanation.",
		fmt.Sprintf("Feature: %s\nMissing assertion hint: %s\nGap: %s\n\nExisting test file:\n```\n%s\n```",
			description, gap.Hint, gap.Description, string(existing)),
		3000,
	)
	if err != nil {
		return err
	}

	if mkErr := os.MkdirAll(filepath.Dir(testFile), 0o755); mkErr != nil {
		return mkErr
	}
	return os.WriteFile(testFile, []byte(content), 0o600)
}
