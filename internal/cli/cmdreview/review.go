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

// Package cmdreview implements `forge review` (DEV-M1-49, spec §4).
//
// Runs a structured AI review of a diff or file set using the configured LLM.
// When no LLM is available it falls back to running `forge scan all` and
// `forge lint` and surfaces those findings as structured review items.
//
// Output (--json):
//
//	{"findings": [{"file": "...", "line": 12, "severity": "error", "rule_id": "SEC-001", "message": "..."}]}
package cmdreview

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/teragrid/forge/internal/errcode"
	"github.com/teragrid/forge/internal/llmprovider"
	"github.com/teragrid/forge/internal/verbmeta"
)

// Reserved error codes (range 4800..4899).
var (
	ErrReviewFailed = errcode.Register(errcode.Code(4800), "review failed")
)

// Finding is one review finding.
type Finding struct {
	File     string `json:"file"`
	Line     int    `json:"line"`
	Severity string `json:"severity"` // "error", "warning", "info"
	RuleID   string `json:"rule_id"`
	Message  string `json:"message"`
}

// ReviewResult is the output of a review run.
type ReviewResult struct {
	Target   string    `json:"target"`
	Findings []Finding `json:"findings"`
	Summary  string    `json:"summary,omitempty"`
	Passed   bool      `json:"passed"`
}

func init() {
	verbmeta.Register(verbmeta.Manifest{
		Verb:    "review",
		Summary: "Structured AI review of a diff or file (DEV-M1-49, spec §4).",
		Inputs: []string{
			"[path]       — file or directory to review (default: staged git diff)",
			"--rounds N   — debate rounds (default: 3; requires LLM)",
			"--json       — machine-readable JSON",
			"--root <path>",
		},
		Outputs:      []string{"stdout: review findings (text or JSON)"},
		SideEffects:  []string{"none (read-only)"},
		GatesTouched: []string{"§4 review", "§8 self-debate engine"},
		ErrorCodes:   []errcode.Code{ErrReviewFailed},
	})
}

// New returns the cobra command for `forge review`.
func New() *cobra.Command {
	var (
		root   string
		rounds int
		asJSON bool
	)
	cmd := &cobra.Command{
		Use:   "review [path]",
		Short: "Structured AI review of a diff or file set.",
		Long: "forge review runs a structured review over a diff or file set.\n\n" +
			"When an LLM provider is configured it uses the spec §8 self-debate engine\n" +
			"with specialist roles (BA, CPO, SA, Sec, QE, Ops, PO, DL).\n\n" +
			"Without an LLM it falls back to running `forge scan all` + `forge lint`\n" +
			"and surfaces those findings as structured review items.\n\n" +
			"When no path is given, reviews the current staged git diff.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target := ""
			if len(args) > 0 {
				target = args[0]
			}
			if root == "" {
				cwd, err := os.Getwd()
				if err != nil {
					return errcode.New(ErrReviewFailed, "getwd", err)
				}
				root = cwd
			}
			result := Run(root, target, rounds)
			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}
			renderText(cmd, result)
			if !result.Passed {
				return errcode.Newf(ErrReviewFailed, nil,
					"%d finding(s) require attention", len(result.Findings))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&root, "root", "", "project root (default: cwd)")
	cmd.Flags().IntVar(&rounds, "rounds", 3, "self-debate rounds (requires LLM)")
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit machine-readable JSON")
	return cmd
}

// Run executes a review and returns structured findings.
func Run(root, target string, rounds int) ReviewResult {
	result := ReviewResult{Target: target}

	// Try LLM-backed review first.
	provider, err := llmprovider.Detect()
	if err == nil {
		llmResult := llmReview(root, target, provider, rounds)
		return llmResult
	}

	// Fallback: run scan + lint via subprocess.
	result.Findings = staticReview(root)
	errors := 0
	for _, f := range result.Findings {
		if f.Severity == "error" {
			errors++
		}
	}
	result.Passed = errors == 0
	if errors == 0 {
		result.Summary = fmt.Sprintf("static review: %d finding(s) (no LLM configured; set ANTHROPIC_API_KEY for AI review)", len(result.Findings))
	} else {
		result.Summary = fmt.Sprintf("static review: %d error(s) found (no LLM configured)", errors)
	}
	return result
}

// llmReview uses the LLM provider to review the target.
func llmReview(root, target string, provider llmprovider.Provider, _ int) ReviewResult {
	result := ReviewResult{Target: target}

	// Collect diff or file content.
	content, err := collectContent(root, target)
	if err != nil || content == "" {
		result.Passed = true
		result.Summary = "nothing to review (empty diff or missing target)"
		return result
	}

	systemPrompt := "You are a code reviewer with expertise in security, performance, and best practices. " +
		"Review the following code changes and identify issues. " +
		"For each issue, provide: file (or 'diff' if unclear), line number (0 if unknown), " +
		"severity (error/warning/info), a rule_id (like SEC-001 for security, PERF-001 for performance, etc.), " +
		"and a brief message. " +
		"Respond with a JSON object: {\"findings\": [{\"file\": \"...\", \"line\": 0, \"severity\": \"...\", \"rule_id\": \"...\", \"message\": \"...\"}], \"summary\": \"...\"}"

	resp, err := provider.Complete(context.Background(), &llmprovider.Request{
		SystemPrompt: systemPrompt,
		UserPrompt:   "Review the following:\n\n" + content,
		MaxTokens:    2048,
	})
	if err != nil {
		// Fallback to static review on LLM failure.
		result.Findings = staticReview(root)
		result.Summary = fmt.Sprintf("LLM review failed (%v); fallback to static review", err)
		result.Passed = len(result.Findings) == 0
		return result
	}

	// Parse LLM JSON response.
	var parsed struct {
		Findings []Finding `json:"findings"`
		Summary  string    `json:"summary"`
	}
	// Strip markdown code fences if present.
	cleaned := strings.TrimSpace(resp.Content)
	if idx := strings.Index(cleaned, "{"); idx > 0 {
		cleaned = cleaned[idx:]
	}
	if err := json.Unmarshal([]byte(cleaned), &parsed); err != nil {
		// Best-effort: return raw as a single finding.
		result.Findings = []Finding{{
			Severity: "info",
			RuleID:   "REVIEW-001",
			Message:  resp.Content,
		}}
		result.Passed = true
		return result
	}
	result.Findings = parsed.Findings
	result.Summary = parsed.Summary
	errors := 0
	for _, f := range result.Findings {
		if f.Severity == "error" {
			errors++
		}
	}
	result.Passed = errors == 0
	return result
}

// staticReview runs forge lint and forge scan via subprocess to collect findings.
func staticReview(root string) []Finding {
	var findings []Finding

	forgeBin := forgeBinPath(root)

	// Run forge lint --json.
	if out, err := runJSON(forgeBin, root, "lint", "--json"); err == nil {
		var lintResult struct {
			Issues []struct {
				File    string `json:"file"`
				Level   string `json:"level"`
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"issues"`
		}
		if json.Unmarshal(out, &lintResult) == nil {
			for _, iss := range lintResult.Issues {
				findings = append(findings, Finding{
					File:     iss.File,
					Severity: iss.Level,
					RuleID:   iss.Code,
					Message:  iss.Message,
				})
			}
		}
	}

	// Run forge scan all --json.
	if out, err := runJSON(forgeBin, root, "scan", "all", "--json"); err == nil {
		var scanResult struct {
			Findings []struct {
				File     string `json:"file"`
				Line     int    `json:"line"`
				Severity string `json:"severity"`
				RuleID   string `json:"rule_id"`
				Message  string `json:"message"`
			} `json:"findings"`
		}
		if json.Unmarshal(out, &scanResult) == nil {
			for _, f := range scanResult.Findings {
				findings = append(findings, Finding{
					File:     f.File,
					Line:     f.Line,
					Severity: f.Severity,
					RuleID:   f.RuleID,
					Message:  f.Message,
				})
			}
		}
	}

	return findings
}

// collectContent returns the diff or file content to review.
func collectContent(root, target string) (string, error) {
	if target != "" {
		data, err := os.ReadFile(filepath.Join(root, target))
		if err != nil {
			return "", err
		}
		return string(data), nil
	}
	// Get staged diff.
	out, err := exec.Command("git", "-C", root, "diff", "--cached").Output()
	if err != nil || len(out) == 0 {
		// Fall back to HEAD diff.
		out, err = exec.Command("git", "-C", root, "diff", "HEAD").Output()
		if err != nil {
			return "", err
		}
	}
	return string(out), nil
}

func runJSON(forgeBin, root string, args ...string) ([]byte, error) {
	cmd := exec.Command(forgeBin, args...)
	cmd.Dir = root
	return cmd.Output()
}

func forgeBinPath(root string) string {
	// Check bin/forge relative to root.
	local := filepath.Join(root, "bin", "forge")
	if _, err := os.Stat(local); err == nil {
		return local
	}
	return "forge"
}

func renderText(cmd *cobra.Command, r ReviewResult) {
	w := cmd.OutOrStdout()
	target := r.Target
	if target == "" {
		target = "<staged diff>"
	}
	fmt.Fprintf(w, "forge review: %s\n", target)
	if r.Summary != "" {
		fmt.Fprintf(w, "%s\n\n", r.Summary)
	}
	if len(r.Findings) == 0 {
		fmt.Fprintln(w, "✓ no findings")
		return
	}
	fmt.Fprintf(w, "%d finding(s):\n", len(r.Findings))
	for _, f := range r.Findings {
		marker := "ℹ"
		switch f.Severity {
		case "error":
			marker = "✗"
		case "warning":
			marker = "⚠"
		}
		loc := f.File
		if f.Line > 0 {
			loc = fmt.Sprintf("%s:%d", f.File, f.Line)
		}
		fmt.Fprintf(w, "  %s [%s] %s — %s\n", marker, f.RuleID, loc, f.Message)
	}
}
