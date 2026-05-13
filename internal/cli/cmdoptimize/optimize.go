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

// Package cmdoptimize implements `forge optimize` (M3-25).
//
// forge optimize analyses the project for LLM token cost, runtime
// performance, and bundle-size regressions, then generates a prioritised
// improvement plan via the configured LLM provider.
//
// Sub-commands:
//
//	tokens   — audit LLM context window usage (prompt sizes)
//	bundle   — audit frontend bundle size contributions
//	profile  — suggest hot-path refactors from pprof data
//	plan     — generate a full optimisation roadmap (LLM-assisted)
package cmdoptimize

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/teragrid/forge/internal/errcode"
	"github.com/teragrid/forge/internal/llmprovider"
	"github.com/teragrid/forge/internal/verbmeta"
)

// Reserved error codes (range 5600..5699).
var (
	ErrOptimizeFailed = errcode.Register(errcode.Code(5600), "optimize operation failed")
)

// OptimizeResult is the structured output of an optimization analysis.
type OptimizeResult struct {
	Target      string            `json:"target"`
	Suggestions []OptimizeSuggest `json:"suggestions"`
	Summary     string            `json:"summary"`
}

// OptimizeSuggest is one optimization suggestion.
type OptimizeSuggest struct {
	Category string `json:"category"` // "tokens" | "bundle" | "perf"
	Impact   string `json:"impact"`   // "high" | "medium" | "low"
	Message  string `json:"message"`
	File     string `json:"file,omitempty"`
}

func init() {
	verbmeta.Register(verbmeta.Manifest{
		Verb:    "optimize",
		Summary: "Analyse and optimise LLM costs, bundle size, and runtime performance (M3).",
		Inputs: []string{
			"tokens   — audit LLM context window usage",
			"bundle   — audit frontend bundle size",
			"profile  — suggest hot-path refactors",
			"plan     — LLM-assisted optimisation roadmap",
			"--root <path>",
			"--json",
		},
		Outputs:      []string{"stdout: optimization report or JSON"},
		SideEffects:  []string{"plan: writes .forge/optimize-plan.md via LLM"},
		GatesTouched: []string{"§14 NFR budgets"},
		ErrorCodes:   []errcode.Code{ErrOptimizeFailed},
	})
}

// New returns the cobra command for `forge optimize`.
func New() *cobra.Command {
	var (
		root    string
		jsonOut bool
	)
	cmd := &cobra.Command{
		Use:   "optimize <tokens|bundle|profile|plan>",
		Short: "Analyse and optimise LLM costs, bundle size, and runtime performance (M3).",
		Long: "forge optimize provides targeted analysis for common cost and performance\n" +
			"concerns in AI-assisted applications.\n\n" +
			"Sub-commands:\n" +
			"  tokens  — measure LLM prompt sizes and flag overlong contexts\n" +
			"  bundle  — check for oversized frontend bundles\n" +
			"  profile — suggest hot-path refactors (reads pprof output)\n" +
			"  plan    — generate a full optimisation roadmap via LLM",
	}
	cmd.PersistentFlags().StringVar(&root, "root", "", "Project root (default: cwd)")
	cmd.PersistentFlags().BoolVar(&jsonOut, "json", false, "Emit JSON output")

	cmd.AddCommand(
		newTokensCmd(&root, &jsonOut),
		newBundleCmd(&root, &jsonOut),
		newProfileCmd(&root, &jsonOut),
		newPlanCmd(&root, &jsonOut),
	)
	return cmd
}

func newTokensCmd(root *string, jsonOut *bool) *cobra.Command {
	return &cobra.Command{
		Use:   "tokens",
		Short: "Audit LLM context window usage across prompt files.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			r, err := resolveRoot(*root)
			if err != nil {
				return err
			}
			res := auditTokens(r)
			if *jsonOut {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(res)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "optimize tokens: %d suggestions found\n", len(res.Suggestions))
			for _, s := range res.Suggestions {
				fmt.Fprintf(cmd.OutOrStdout(), "  [%s] %s: %s\n", s.Impact, s.File, s.Message)
			}
			return nil
		},
	}
}

func newBundleCmd(root *string, jsonOut *bool) *cobra.Command {
	return &cobra.Command{
		Use:   "bundle",
		Short: "Audit frontend bundle size contributions.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			r, err := resolveRoot(*root)
			if err != nil {
				return err
			}
			res := auditBundle(r)
			if *jsonOut {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(res)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "optimize bundle: %d suggestions\n", len(res.Suggestions))
			for _, s := range res.Suggestions {
				fmt.Fprintf(cmd.OutOrStdout(), "  [%s] %s\n", s.Impact, s.Message)
			}
			return nil
		},
	}
}

func newProfileCmd(root *string, jsonOut *bool) *cobra.Command {
	return &cobra.Command{
		Use:   "profile",
		Short: "Suggest hot-path refactors from pprof data.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			r, err := resolveRoot(*root)
			if err != nil {
				return err
			}
			pprofPath := filepath.Join(r, "cpu.prof")
			if _, err := os.Stat(pprofPath); os.IsNotExist(err) {
				fmt.Fprintln(cmd.OutOrStdout(), "optimize profile: no cpu.prof found — run `go test -cpuprofile cpu.prof` first")
				return nil
			}
			res := OptimizeResult{
				Target:  pprofPath,
				Summary: "pprof analysis wiring in M3 — run `go tool pprof cpu.prof` for now",
			}
			if *jsonOut {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(res)
			}
			fmt.Fprintln(cmd.OutOrStdout(), res.Summary)
			return nil
		},
	}
}

func newPlanCmd(root *string, jsonOut *bool) *cobra.Command {
	return &cobra.Command{
		Use:   "plan",
		Short: "Generate a full optimisation roadmap via LLM.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			r, err := resolveRoot(*root)
			if err != nil {
				return err
			}
			provider, err := llmprovider.Detect()
			if err != nil {
				fmt.Fprintln(cmd.OutOrStdout(), "optimize plan: no LLM provider — set ANTHROPIC_API_KEY or OPENAI_API_KEY")
				return nil
			}
			tokens := auditTokens(r)
			bundle := auditBundle(r)
			prompt := fmt.Sprintf(
				"Given these optimization findings, generate a prioritised roadmap:\n\nToken issues: %d\nBundle issues: %d\n\nProvide 5-10 concrete action items ranked by impact.",
				len(tokens.Suggestions), len(bundle.Suggestions),
			)
			resp, err := provider.Complete(cmd.Context(), &llmprovider.Request{
				SystemPrompt: "You are a performance optimisation expert for AI-assisted software projects.",
				UserPrompt:   prompt,
				MaxTokens:    1024,
			})
			if err != nil {
				return errcode.New(ErrOptimizeFailed, "LLM plan generation", err)
			}
			planPath := filepath.Join(r, ".forge", "optimize-plan.md")
			_ = os.WriteFile(planPath, []byte(resp.Content), 0o644)
			if *jsonOut {
				res := OptimizeResult{Target: planPath, Summary: resp.Content}
				return json.NewEncoder(cmd.OutOrStdout()).Encode(res)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "optimize plan: saved to %s\n\n%s\n", planPath, resp.Content)
			return nil
		},
	}
}

func auditTokens(root string) OptimizeResult {
	var suggestions []OptimizeSuggest
	contextFiles := []string{
		"AGENTS.md", "CLAUDE.md", ".cursorrules", ".windsurfrules",
		".forge/context/snapshot.md", "docs/ARCHITECTURE_OVERVIEW.md",
	}
	for _, f := range contextFiles {
		path := filepath.Join(root, f)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		tokens := len(data) / 4
		if tokens > 32000 {
			impact := "high"
			if tokens < 64000 {
				impact = "medium"
			}
			suggestions = append(suggestions, OptimizeSuggest{
				Category: "tokens",
				Impact:   impact,
				File:     f,
				Message:  fmt.Sprintf("~%d estimated tokens — consider trimming to under 32k", tokens),
			})
		}
	}
	return OptimizeResult{Target: root, Suggestions: suggestions, Summary: fmt.Sprintf("%d token issues found", len(suggestions))}
}

func auditBundle(root string) OptimizeResult {
	var suggestions []OptimizeSuggest
	// Check for large node_modules or build artefacts accidentally in tracked files
	pkgPath := filepath.Join(root, "package.json")
	if _, err := os.Stat(pkgPath); err == nil {
		// Check dist/bundle sizes
		distPath := filepath.Join(root, "dist")
		if info, err := os.Stat(distPath); err == nil && info.IsDir() {
			size := dirSize(distPath)
			if size > 5*1024*1024 { // 5 MB
				suggestions = append(suggestions, OptimizeSuggest{
					Category: "bundle",
					Impact:   "medium",
					File:     "dist/",
					Message:  fmt.Sprintf("dist/ is %.1f MB — consider code splitting or tree shaking", float64(size)/(1024*1024)),
				})
			}
		}
	}
	return OptimizeResult{Target: root, Suggestions: suggestions, Summary: fmt.Sprintf("%d bundle issues found", len(suggestions))}
}

func dirSize(path string) int64 {
	var total int64
	_ = filepath.WalkDir(path, func(_ string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err == nil {
			total += info.Size()
		}
		return nil
	})
	return total
}

func resolveRoot(root string) (string, error) {
	if root != "" {
		return root, nil
	}
	return os.Getwd()
}
