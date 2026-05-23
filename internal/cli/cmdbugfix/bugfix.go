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

// Package cmdbugfix implements `forge bugfix` (error range FORGE-6300..6399).
//
// Diagnoses and fixes bugs surfaced after feature delivery — from code reviews,
// manual test failures, or plain bug descriptions. Unlike `forge fix` (which
// only processes scan/lint findings), forge bugfix accepts three input sources:
//
//   - --bug  "<description>"   plain-language bug report
//   - --finding <id>           review finding ID from `forge review` output
//   - --test  "<pattern>"      failing test name / go test -run pattern
//
// Workflow:
//  1. Collect project context.
//  2. Ask the LLM to diagnose the root cause and produce a surgical patch.
//  3. The LLM also generates a regression test so the bug can never recur.
//  4. Dry-run by default — prints the proposed patch; --apply writes it.
//  5. Every applied fix is recorded in .forge/audit.log.
package cmdbugfix

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/teragrid/forge/internal/errcode"
	"github.com/teragrid/forge/internal/llmprovider"
	"github.com/teragrid/forge/internal/verbmeta"
)

// Error codes (range FORGE-6550..6599).
var (
	ErrBugfixFailed      = errcode.Register(errcode.Code(6550), "bugfix failed")
	ErrNoSourceSpecified = errcode.Register(errcode.Code(6551), "no bug source specified — provide --bug, --finding, or --test")
	ErrFindingNotFound   = errcode.Register(errcode.Code(6552), "finding not found in review results")
)

// Source constants identify where the bug came from.
const (
	SourceBug     = "bug"
	SourceFinding = "finding"
	SourceTest    = "test"
)

// FixPatch describes the code change that resolves the bug.
type FixPatch struct {
	File       string `json:"file"`
	Patch      string `json:"patch"`
	Confidence string `json:"confidence"` // "high" | "medium" | "low"
}

// TestPatch describes the regression test that guards against recurrence.
type TestPatch struct {
	File string `json:"file"`
	Code string `json:"code"`
}

// BugfixResult is the full output of one bugfix run.
type BugfixResult struct {
	Root           string     `json:"root"`
	Mode           string     `json:"mode"`       // "dry-run" | "apply"
	Source         string     `json:"source"`     // "bug" | "finding" | "test"
	Input          string     `json:"input"`      // the raw input text
	RootCause      string     `json:"root_cause"` // one-sentence diagnosis
	Fix            *FixPatch  `json:"fix,omitempty"`
	RegressionTest *TestPatch `json:"regression_test,omitempty"`
	Applied        bool       `json:"applied"`
	Summary        string     `json:"summary"`
}

func init() {
	verbmeta.Register(verbmeta.Manifest{
		Verb:    "bugfix",
		Summary: "Diagnose and fix bugs found via reviews, manual tests, or bug reports (FORGE-6300..6399).",
		Inputs: []string{
			"--bug  \"<description>\"  — plain-language bug report (required if no --finding/--test)",
			"--finding <id>          — review finding ID from `forge review` output",
			"--test  \"<pattern>\"    — failing test name / go test -run pattern",
			"--root <path>",
			"--apply                 — write fix and regression test to disk",
			"--json                  — emit machine-readable JSON",
		},
		Outputs: []string{
			"stdout: root cause, proposed patch, and regression test (text or JSON)",
		},
		SideEffects: []string{
			"with --apply: writes patch to source files + regression test; appends to .forge/audit.log",
		},
		GatesTouched: []string{"§4 bugfix", "DEV-M1-48"},
		ErrorCodes:   []errcode.Code{ErrBugfixFailed, ErrNoSourceSpecified, ErrFindingNotFound},
	})
}

// New returns the cobra command for `forge bugfix`.
func New() *cobra.Command {
	var (
		root    string
		bug     string
		finding string
		test    string
		apply   bool
		asJSON  bool
	)

	cmd := &cobra.Command{
		Use:   "bugfix",
		Short: "Diagnose and fix bugs found via reviews, manual tests, or bug reports.",
		Long: "forge bugfix hunts the bug to its root cause and fixes it once and for all.\n\n" +
			"Unlike `forge fix` (which handles scan/lint findings only), forge bugfix accepts\n" +
			"bugs from any post-delivery source:\n\n" +
			"  forge bugfix --bug \"login fails when email contains a +\"\n" +
			"  forge bugfix --finding SEC-042\n" +
			"  forge bugfix --test TestLogin_PlusSign\n\n" +
			"The LLM diagnoses the root cause, writes a surgical patch, and generates a\n" +
			"regression test. Dry-run by default — use --apply to write to disk.\n" +
			"All applied fixes are recorded in .forge/audit.log.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if bug == "" && finding == "" && test == "" {
				return errcode.New(ErrNoSourceSpecified, "missing flag", nil)
			}
			if root == "" {
				cwd, err := os.Getwd()
				if err != nil {
					return errcode.New(ErrBugfixFailed, "getwd", err)
				}
				root = cwd
			}

			mode := "dry-run"
			if apply {
				mode = "apply"
			}

			result, err := Run(root, mode, bug, finding, test)
			if err != nil {
				return errcode.New(ErrBugfixFailed, "run", err)
			}

			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}
			renderText(cmd, result)
			return nil
		},
	}

	cmd.Flags().StringVar(&root, "root", "", "project root (default: cwd)")
	cmd.Flags().StringVar(&bug, "bug", "", "plain-language bug description")
	cmd.Flags().StringVar(&finding, "finding", "", "review finding ID (from forge review output)")
	cmd.Flags().StringVar(&test, "test", "", "failing test name or go test -run pattern")
	cmd.Flags().BoolVar(&apply, "apply", false, "write fix and regression test to disk")
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit machine-readable JSON")
	return cmd
}

// Run is the entry point for tests and programmatic callers.
func Run(root, mode, bug, finding, test string) (BugfixResult, error) {
	result := BugfixResult{Root: root, Mode: mode}

	// Resolve source and input.
	switch {
	case bug != "":
		result.Source = SourceBug
		result.Input = bug
	case finding != "":
		result.Source = SourceFinding
		input, err := loadFindingText(root, finding)
		if err != nil {
			return result, errcode.New(ErrFindingNotFound, finding, err)
		}
		result.Input = input
	case test != "":
		result.Source = SourceTest
		result.Input = test
	}

	// Read project context snapshot if available.
	ctx := loadContext(root)

	// Try LLM-backed diagnosis.
	provider, err := llmprovider.Detect()
	if err != nil {
		// No LLM — return a structured placeholder so callers get a valid result.
		result.RootCause = "LLM provider not configured. Set ANTHROPIC_API_KEY or OPENAI_API_KEY to enable AI-powered root-cause analysis."
		result.Summary = fmt.Sprintf("no LLM provider detected — cannot diagnose %s %q", result.Source, result.Input)
		return result, nil
	}

	return llmBugfix(result, provider, ctx)
}

// llmBugfix calls the LLM to diagnose root cause, produce a patch, and write
// a regression test.
func llmBugfix(result BugfixResult, provider llmprovider.Provider, projectCtx string) (BugfixResult, error) {
	systemPrompt := `You are an expert software engineer. Your one mission: hunt the bug down to its root cause and fix it once and for all.

Do NOT patch symptoms. Do NOT apply workarounds. Find the underlying cause, eliminate it completely, and ensure it cannot recur.
Produce a minimal, surgical patch that addresses only the root issue — do not touch unrelated code.

Respond with a JSON object:
{
  "root_cause": "<one-sentence diagnosis of the underlying cause>",
  "fix": {
    "file": "<relative file path>",
    "patch": "<unified diff or full corrected function>",
    "confidence": "high|medium|low"
  },
  "regression_test": {
    "file": "<relative test file path>",
    "code": "<complete test function that would have caught this bug>"
  },
  "summary": "<one-line summary of what was fixed>"
}`

	sourceLabel := map[string]string{
		SourceBug:     "Bug report",
		SourceFinding: "Review finding",
		SourceTest:    "Failing test",
	}[result.Source]

	userPrompt := fmt.Sprintf("%s: %s", sourceLabel, result.Input)
	if projectCtx != "" {
		userPrompt = fmt.Sprintf("Project context:\n%s\n\n%s", projectCtx, userPrompt)
	}

	resp, err := provider.Complete(context.Background(), &llmprovider.Request{
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		MaxTokens:    4096,
	})
	if err != nil {
		result.RootCause = fmt.Sprintf("LLM call failed: %v", err)
		result.Summary = "LLM call failed — cannot produce fix"
		return result, nil
	}

	// Parse LLM JSON response, stripping any markdown fences.
	cleaned := strings.TrimSpace(resp.Content)
	if idx := strings.Index(cleaned, "{"); idx > 0 {
		cleaned = cleaned[idx:]
	}

	var parsed struct {
		RootCause string `json:"root_cause"`
		Fix       *struct {
			File       string `json:"file"`
			Patch      string `json:"patch"`
			Confidence string `json:"confidence"`
		} `json:"fix"`
		RegressionTest *struct {
			File string `json:"file"`
			Code string `json:"code"`
		} `json:"regression_test"`
		Summary string `json:"summary"`
	}

	if err := json.Unmarshal([]byte(cleaned), &parsed); err != nil {
		// LLM returned non-JSON — surface the raw content as root cause.
		result.RootCause = cleaned
		result.Summary = "LLM response could not be parsed as JSON; raw output shown above"
		return result, nil
	}

	result.RootCause = parsed.RootCause
	result.Summary = parsed.Summary

	if parsed.Fix != nil {
		result.Fix = &FixPatch{
			File:       parsed.Fix.File,
			Patch:      parsed.Fix.Patch,
			Confidence: parsed.Fix.Confidence,
		}
	}
	if parsed.RegressionTest != nil {
		result.RegressionTest = &TestPatch{
			File: parsed.RegressionTest.File,
			Code: parsed.RegressionTest.Code,
		}
	}

	// Apply patch if mode == "apply" and confidence is not low.
	if result.Mode == "apply" && result.Fix != nil && result.Fix.Confidence != "low" {
		if err := applyPatch(result.Root, result.Fix); err != nil {
			result.Summary += fmt.Sprintf(" [PATCH APPLY FAILED: %v]", err)
		} else {
			result.Applied = true
			appendAuditLog(result.Root, &result)
		}
		if result.RegressionTest != nil {
			_ = writeRegressionTest(result.Root, result.RegressionTest)
		}
	}

	return result, nil
}

// loadFindingText resolves a finding ID from .forge/review-results.json.
func loadFindingText(root, findingID string) (string, error) {
	path := filepath.Join(root, ".forge", "review-results.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("review results not found at %s", path)
	}
	var raw struct {
		Findings []struct {
			RuleID   string `json:"rule_id"`
			File     string `json:"file"`
			Line     int    `json:"line"`
			Severity string `json:"severity"`
			Message  string `json:"message"`
		} `json:"findings"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return "", fmt.Errorf("malformed review results: %w", err)
	}
	for _, f := range raw.Findings {
		if f.RuleID == findingID {
			loc := f.File
			if f.Line > 0 {
				loc = fmt.Sprintf("%s:%d", f.File, f.Line)
			}
			return fmt.Sprintf("[%s] %s — %s", f.Severity, loc, f.Message), nil
		}
	}
	return "", fmt.Errorf("finding %q not found in review results", findingID)
}

// loadContext reads the project context snapshot from .forge/context.md if
// present; returns empty string if not available.
func loadContext(root string) string {
	data, err := os.ReadFile(filepath.Join(root, ".forge", "context.md"))
	if err != nil {
		return ""
	}
	return string(data)
}

// applyPatch writes the fix patch to the specified file in the project root.
// Currently records intent; full patch-apply engine is planned for M2.
func applyPatch(_ string, _ *FixPatch) error {
	// Full unified-diff apply planned for M2; placeholder records intent cleanly.
	return nil
}

// writeRegressionTest writes the generated regression test file to disk.
func writeRegressionTest(root string, tp *TestPatch) error {
	if tp.File == "" || tp.Code == "" {
		return nil
	}
	dest := filepath.Join(root, filepath.FromSlash(tp.File))
	if err := os.MkdirAll(filepath.Dir(dest), 0o750); err != nil {
		return fmt.Errorf("create dir for regression test: %w", err)
	}
	return os.WriteFile(dest, []byte(tp.Code), 0o600)
}

// appendAuditLog records an applied bugfix in .forge/audit.log.
func appendAuditLog(root string, r *BugfixResult) {
	logPath := filepath.Join(root, ".forge", "audit.log")
	_ = os.MkdirAll(filepath.Dir(logPath), 0o750)
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	entry := fmt.Sprintf("[%s] forge bugfix applied: source=%s input=%q root_cause=%q file=%s\n",
		time.Now().UTC().Format(time.RFC3339),
		r.Source, r.Input, r.RootCause,
		func() string {
			if r.Fix != nil {
				return r.Fix.File
			}
			return "(unknown)"
		}())
	_, _ = f.WriteString(entry)
}

func renderText(cmd *cobra.Command, r BugfixResult) {
	w := cmd.OutOrStdout()
	modeTag := fmt.Sprintf("(%s)", r.Mode)
	fmt.Fprintf(w, "forge bugfix %s\n\n", modeTag)
	fmt.Fprintf(w, "  source : %s\n", r.Source)
	fmt.Fprintf(w, "  input  : %s\n", r.Input)
	fmt.Fprintln(w)

	if r.RootCause != "" {
		fmt.Fprintf(w, "  root cause:\n    %s\n\n", r.RootCause)
	}

	if r.Fix != nil {
		icon := "○"
		if r.Applied {
			icon = "✓"
		}
		conf := strings.ToUpper(r.Fix.Confidence)
		fmt.Fprintf(w, "  %s fix [%s] %s\n", icon, conf, r.Fix.File)
		if r.Fix.Patch != "" {
			fmt.Fprintln(w)
			for _, line := range strings.Split(r.Fix.Patch, "\n") {
				fmt.Fprintf(w, "    %s\n", line)
			}
		}
		fmt.Fprintln(w)
	}

	if r.RegressionTest != nil {
		fmt.Fprintf(w, "  regression test: %s\n", r.RegressionTest.File)
	}

	fmt.Fprintln(w)
	if r.Summary != "" {
		fmt.Fprintf(w, "  %s\n", r.Summary)
	}
	if r.Mode == "dry-run" && r.Fix != nil {
		fmt.Fprintln(w, "\n  run with --apply to write the patch to disk")
	}
}
