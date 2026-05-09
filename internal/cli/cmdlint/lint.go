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
	return cmd
}

// Run executes all lint checks against a project root.
func Run(root string) *LintResult {
	res := &LintResult{Root: root}

	res.Issues = append(res.Issues, checkManifestExists(root)...)
	res.Issues = append(res.Issues, checkGitignoreMarkers(root)...)
	res.Issues = append(res.Issues, checkGitleaksConfig(root)...)

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
