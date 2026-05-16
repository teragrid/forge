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
// Package cmdlint implements `forge lint` — hygiene + convention checker.
// Validates .forge/manifest, .gitignore markers, gitleaks.toml, and project structure.
package cmdlint

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/teragrid/forge/internal/errcode"
	"github.com/teragrid/forge/internal/verbmeta"
)

// Reserved error codes (range 3100..3199).
var (
	ErrLintFailed = errcode.Register(errcode.Code(3100), "lint check failed")
)

// LintIssue represents one convention violation.
type LintIssue struct {
	File    string `json:"file"`
	Level   string `json:"level"` // "error", "warning", "info"
	Code    string `json:"code"`  // "M001", "M002", ...
	Message string `json:"message"`
}

// LintResult summarizes all issues found.
type LintResult struct {
	Root   string      `json:"root"`
	Issues []LintIssue `json:"issues"`
	Errors int         `json:"errors"`
	Passed bool        `json:"passed"`
}

func init() {
	verbmeta.Register(verbmeta.Manifest{
		Verb:    "lint",
		Summary: "Check project hygiene: .forge/manifest, .gitignore markers, gitleaks.toml, conventions.",
		Inputs: []string{
			"--root <path> (project root; default cwd)",
			"--json (machine-readable output)",
		},
		Outputs:      []string{"stdout: issue list (text or JSON)", "exit: 0 ok, non-zero on errors"},
		SideEffects:  []string{},
		GatesTouched: []string{"§16.5.4 #11 — repo hygiene"},
		ErrorCodes:   []errcode.Code{ErrLintFailed},
	})
}

// New returns the cobra command.
func New() *cobra.Command {
	var (
		root   string
		asJSON bool
	)
	cmd := &cobra.Command{
		Use:   "lint",
		Short: "Check project hygiene and conventions.",
		Long: "Validates that the project adheres to Forge conventions: " +
			".forge/manifest exists, .gitignore has marker blocks, .gitleaks.toml configured.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if root == "" {
				cwd, err := os.Getwd()
				if err != nil {
					return errcode.New(ErrLintFailed, "getwd", err)
				}
				root = cwd
			}

			res := Run(root)
			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				if err := enc.Encode(res); err != nil {
					return err
				}
			} else {
				renderText(cmd, res)
			}

			if !res.Passed {
				return errcode.Newf(ErrLintFailed, nil,
					"%d error(s) — fix and re-run `forge lint`", res.Errors)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&root, "root", "", "project root (default: cwd)")
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit machine-readable JSON")

	// G-043: forge lint prompts — KV-cache prefix-break detection.
	cmd.AddCommand(newPromptsLintCmd())
	return cmd
}

// Run executes all lint checks against a project root.
func Run(root string) *LintResult {
	res := &LintResult{Root: root}

	res.Issues = append(res.Issues, checkManifestExists(root)...)
	res.Issues = append(res.Issues, checkGitignoreMarkers(root)...)
	res.Issues = append(res.Issues, checkGitleaksConfig(root)...)
	res.Issues = append(res.Issues, checkInstructionsPack(root)...) // M1-25
	res.Issues = append(res.Issues, checkNewPatterns(root)...)      // M1-25

	res.Errors = 0
	for _, iss := range res.Issues {
		if iss.Level == "error" {
			res.Errors++
		}
	}
	res.Passed = res.Errors == 0
	return res
}

func checkManifestExists(root string) []LintIssue {
	path := filepath.Join(root, ".forge", "manifest")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return []LintIssue{{
			File:    ".forge/manifest",
			Level:   "error",
			Code:    "M001",
			Message: ".forge/manifest missing — run `forge new` to scaffold a project or add the file manually",
		}}
	}
	return []LintIssue{}
}

func checkGitignoreMarkers(root string) []LintIssue {
	path := filepath.Join(root, ".gitignore")
	data, err := os.ReadFile(path)
	if err != nil {
		// .gitignore missing is OK but warn.
		return []LintIssue{{
			File:    ".gitignore",
			Level:   "warning",
			Code:    "M002",
			Message: ".gitignore missing — add one with `forge new` or create manually",
		}}
	}

	content := string(data)
	hasStart := strings.Contains(content, "forge:gitignore:start")
	hasEnd := strings.Contains(content, "forge:gitignore:end")

	if !hasStart || !hasEnd {
		return []LintIssue{{
			File:    ".gitignore",
			Level:   "info",
			Code:    "M003",
			Message: ".gitignore missing forge marker block (forge:gitignore:start/end) — patterns between markers are auto-managed",
		}}
	}
	return []LintIssue{}
}

func checkGitleaksConfig(root string) []LintIssue {
	path := filepath.Join(root, ".gitleaks.toml")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return []LintIssue{{
			File:    ".gitleaks.toml",
			Level:   "warning",
			Code:    "M004",
			Message: ".gitleaks.toml missing — add one with `forge new` to prevent secret leaks",
		}}
	}
	return []LintIssue{}
}

func renderText(cmd *cobra.Command, r *LintResult) {
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "forge lint %s\n", r.Root)
	if len(r.Issues) == 0 {
		fmt.Fprintln(w, "✓ all checks passed")
		return
	}
	fmt.Fprintf(w, "%d issue(s):\n", len(r.Issues))
	for _, iss := range r.Issues {
		marker := "⚠ "
		if iss.Level == "error" {
			marker = "✗ "
		} else if iss.Level == "info" {
			marker = "ℹ "
		}
		fmt.Fprintf(w, "  %s%s [%s] %s\n", marker, iss.File, iss.Code, iss.Message)
	}
}

// checkInstructionsPack verifies that .forge/instructions/defaults.md exists (M1-25).
func checkInstructionsPack(root string) []LintIssue {
	paths := []string{
		filepath.Join(root, ".forge", "instructions", "defaults.md"),
		filepath.Join(root, ".forge", "instructions", "global.instructions.md"),
	}
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return []LintIssue{}
		}
	}
	return []LintIssue{{
		File:  ".forge/instructions/defaults.md",
		Level: "warning",
		Code:  "M005",
		Message: ".forge/instructions/defaults.md missing — run `forge new` to scaffold, or create with the Forge instructions template. " +
			"See: https://github.com/teragrid/forge/blob/main/docs/FORGE_FRAMEWORK_SPEC.md#instructions-pack",
	}}
}

// checkNewPatterns detects novel code patterns that lack RFC-link anchors (M1-25).
// A "new pattern" is a function or struct whose doc comment does not reference
// an internal RFC, ADR, or spec anchor (e.g. RFC-1234, ADR-001, §3.2).
// This is a best-effort heuristic; it only flags patterns in .forge/**/*.go.
func checkNewPatterns(root string) []LintIssue {
	forgeDir := filepath.Join(root, ".forge")
	if _, err := os.Stat(forgeDir); os.IsNotExist(err) {
		return []LintIssue{}
	}

	rfcPattern := strings.Join([]string{"RFC-", "ADR-", "§", "spec:", "DEV-M"}, "|")
	var issues []LintIssue
	_ = filepath.WalkDir(forgeDir, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(d.Name(), ".go") {
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return nil
		}
		lines := strings.Split(string(data), "\n")
		for i, line := range lines {
			trimmed := strings.TrimSpace(line)
			// Detect exported func/type/var declarations with no anchor in preceding comment.
			if strings.HasPrefix(trimmed, "func ") || strings.HasPrefix(trimmed, "type ") {
				// Look back up to 5 lines for a doc comment with an anchor.
				hasAnchor := false
				for j := i - 1; j >= max0(i-5); j-- {
					commentLine := strings.TrimSpace(lines[j])
					if !strings.HasPrefix(commentLine, "//") {
						break
					}
					if containsAny(commentLine, rfcPattern) {
						hasAnchor = true
						break
					}
				}
				if !hasAnchor && isExported(trimmed) {
					rel := strings.TrimPrefix(p, root+string(os.PathSeparator))
					issues = append(issues, LintIssue{
						File:  rel,
						Level: "info",
						Code:  "M006",
						Message: fmt.Sprintf("line %d: exported declaration lacks RFC/ADR/spec anchor in doc comment — "+
							"add a reference e.g. // RFC-1234 or // ADR-001", i+1),
					})
				}
			}
		}
		return nil
	})
	return issues
}

func max0(n int) int {
	if n < 0 {
		return 0
	}
	return n
}

func containsAny(s, pattern string) bool {
	for _, p := range strings.Split(pattern, "|") {
		if strings.Contains(s, p) {
			return true
		}
	}
	return false
}

func isExported(line string) bool {
	// After "func " or "type " grab the first identifier character.
	for _, prefix := range []string{"func ", "type "} {
		if strings.HasPrefix(line, prefix) {
			rest := strings.TrimPrefix(line, prefix)
			// Strip pointer/paren prefix.
			rest = strings.TrimLeft(rest, "*(")
			if len(rest) == 0 {
				return false
			}
			c := rest[0]
			return c >= 'A' && c <= 'Z'
		}
	}
	return false
}

// ── G-043: forge lint prompts ─────────────────────────────────────────────────

// PromptLintIssue describes one KV-cache or ordering problem in a prompt file.
type PromptLintIssue struct {
	File    string `json:"file"`
	Kind    string `json:"kind"` // "prefix_break" | "unstable_order" | "variable_in_prefix"
	Message string `json:"message"`
}

// CheckPromptKVCache analyses prompt files for patterns that break provider-side
// KV-cache prefix stability:
//   - System prompt region contains a variable substitution ({{.VarName}}).
//   - Static prefix region comes AFTER dynamic content.
//
// A file is considered a "prompt" if it has a .md extension inside .forge/prompts/.
func CheckPromptKVCache(root string) []PromptLintIssue {
	promptDir := filepath.Join(root, ".forge", "prompts")
	var issues []PromptLintIssue

	_ = filepath.WalkDir(promptDir, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".md") && !strings.HasSuffix(d.Name(), ".prompt.ts") {
			return nil
		}
		data, err := os.ReadFile(p) //nolint:gosec
		if err != nil {
			return nil
		}
		rel := strings.TrimPrefix(p, root+string(os.PathSeparator))
		content := string(data)
		lines := strings.Split(content, "\n")

		// Check 1: variable in system-prompt region (first N lines before the first
		// "---" separator indicate the system prompt prefix region).
		inSystemRegion := true
		seenDynamic := false
		for i, line := range lines {
			if strings.TrimSpace(line) == "---" {
				inSystemRegion = false
			}
			isDynamic := strings.Contains(line, "{{.") || strings.Contains(line, "${")
			if inSystemRegion && isDynamic {
				issues = append(issues, PromptLintIssue{
					File: rel, Kind: "variable_in_prefix",
					Message: fmt.Sprintf("line %d: variable substitution in prefix-stable region breaks KV-cache — move variables after the '---' separator", i+1),
				})
			}
			if !inSystemRegion && isDynamic {
				seenDynamic = true
			}
			// Check 2: static content after dynamic content (re-ordering risk).
			if !inSystemRegion && !isDynamic && seenDynamic && strings.TrimSpace(line) != "" && !strings.HasPrefix(strings.TrimSpace(line), "#") {
				// Static non-header line after dynamic content — might destabilise prefix.
				// Only warn once per file.
				issues = append(issues, PromptLintIssue{
					File: rel, Kind: "unstable_order",
					Message: fmt.Sprintf("line %d: static content after dynamic content may destabilise KV-cache prefix — place all static sections before dynamic sections", i+1),
				})
				seenDynamic = false // suppress duplicate warnings in same file
			}
		}
		return nil
	})
	return issues
}

func newPromptsLintCmd() *cobra.Command {
	var (
		root   string
		asJSON bool
	)
	return &cobra.Command{
		Use:   "prompts",
		Short: "Lint prompt files for KV-cache prefix-break issues (G-043).",
		Long: "Analyses .forge/prompts/*.md and .forge/prompts/*.prompt.ts files for\n" +
			"patterns that break provider-side KV-cache prefix stability:\n" +
			"  variable_in_prefix  — variable substitution in the static prefix region\n" +
			"  unstable_order      — static content placed after dynamic content\n\n" +
			"Re-ordering or inserting content before the prefix-stable region causes the\n" +
			"provider to re-compute the full prefix, wasting tokens and increasing latency.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			r := root
			if r == "" {
				cwd, err := os.Getwd()
				if err != nil {
					return errcode.New(ErrLintFailed, "getwd", err)
				}
				r = cwd
			}
			issues := CheckPromptKVCache(r)
			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(issues)
			}
			if len(issues) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "lint prompts: no KV-cache issues found")
				return nil
			}
			for _, iss := range issues {
				fmt.Fprintf(cmd.OutOrStdout(), "  [%s] %s: %s\n", iss.Kind, iss.File, iss.Message)
			}
			return errcode.Newf(ErrLintFailed, nil, "%d prompt lint issue(s)", len(issues))
		},
	}
}
