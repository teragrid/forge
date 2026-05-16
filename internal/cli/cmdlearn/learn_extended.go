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

// Package cmdlearn — G-030/G-031/G-032/G-033/G-034 learn subcommands.
//
// G-030: forge learn promote    — mines conventions.jsonl, proposes lint rule PR
// G-031: forge learn antipatterns — mines git log for revert/hotfix, writes anti-patterns.md
// G-032: forge learn teach      — appends user text/files to preferences.yml
// G-033: forge learn session    — summarises .forge/session/*.log
// G-034: forge learn instructions — proposes edits to .forge/instructions/*.instructions.md
package cmdlearn

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// ConventionEntry is one record in .forge/learned/conventions.jsonl.
type ConventionEntry struct {
	TS         string `json:"ts"`
	Rule       string `json:"rule"`
	Rejections int    `json:"rejections"`
	Applies    int    `json:"applies"`
	Verb       string `json:"verb,omitempty"`
	Detail     string `json:"detail,omitempty"`
}

// NewPromoteCmd returns `forge learn promote` (G-030).
func NewPromoteCmd(root *string, _ *bool) *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "promote",
		Short: "Mine conventions.jsonl for rules with ≥3 rejections and propose a lint-rule PR (G-030).",
		RunE: func(cmd *cobra.Command, _ []string) error {
			r, _ := resolveLearnRoot(*root)
			path := filepath.Join(r, ".forge", "learned", "conventions.jsonl")
			data, err := os.ReadFile(path)
			if err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("read conventions: %w", err)
			}

			type ruleStat struct {
				Rejections int
				Detail     string
			}
			stats := map[string]*ruleStat{}
			for _, line := range strings.Split(string(data), "\n") {
				if line == "" {
					continue
				}
				var e ConventionEntry
				if json.Unmarshal([]byte(line), &e) == nil {
					if _, ok := stats[e.Rule]; !ok {
						stats[e.Rule] = &ruleStat{Detail: e.Detail}
					}
					stats[e.Rule].Rejections += e.Rejections
				}
			}

			var candidates []string
			for rule, s := range stats {
				if s.Rejections >= 3 {
					candidates = append(candidates, fmt.Sprintf("  %s (%d rejections): %s", rule, s.Rejections, s.Detail))
				}
			}

			if len(candidates) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "learn promote: no rules with ≥3 rejections found in conventions.jsonl")
				return nil
			}

			body := "# Proposed Lint Rules\n\nRules promoted by `forge learn promote`:\n\n"
			for _, c := range candidates {
				body += "- " + c + "\n"
			}

			if dryRun {
				fmt.Fprintln(cmd.OutOrStdout(), "learn promote (dry-run): would propose the following lint rules:")
				for _, c := range candidates {
					fmt.Fprintln(cmd.OutOrStdout(), c)
				}
				return nil
			}

			// Attempt gh pr create.
			ghPath, err := exec.LookPath("gh")
			if err != nil {
				fmt.Fprintln(cmd.OutOrStdout(), "learn promote: gh CLI not found; print proposed rules:")
				for _, c := range candidates {
					fmt.Fprintln(cmd.OutOrStdout(), c)
				}
				return nil
			}
			out, err := exec.Command(ghPath, "pr", "create", //nolint:gosec
				"--title", "forge learn: promote lint rules from conventions.jsonl",
				"--body", body, "--draft").CombinedOutput()
			if err != nil {
				return fmt.Errorf("gh pr create: %w — %s", err, strings.TrimSpace(string(out)))
			}
			fmt.Fprintf(cmd.OutOrStdout(), "learn promote: draft PR created: %s\n", strings.TrimSpace(string(out)))
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print proposed rules without creating a PR")
	return cmd
}

// NewAntiPatternsCmd returns `forge learn antipatterns` (G-031).
func NewAntiPatternsCmd(root *string, _ *bool) *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "antipatterns",
		Short: "Mine git log for revert/hotfix commits and write .forge/instructions/anti-patterns.md (G-031).",
		RunE: func(cmd *cobra.Command, _ []string) error {
			r, _ := resolveLearnRoot(*root)

			// Mine git log for revert/hotfix commit subjects.
			gitPath, err := exec.LookPath("git")
			if err != nil {
				fmt.Fprintln(cmd.OutOrStdout(), "learn antipatterns: git not found; skipping log mining")
				return nil
			}
			out, err := exec.Command(gitPath, "-C", r, "log", //nolint:gosec
				"--oneline", "--grep=revert", "--grep=hotfix", "--grep=fix:", "--all-match=false",
				"--since=90 days ago", "--format=%s").Output()
			if err != nil {
				out = nil // not fatal — might not be a git repo
			}

			var patterns []string
			scanner := bufio.NewScanner(strings.NewReader(string(out)))
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if line != "" {
					patterns = append(patterns, line)
				}
			}

			outDir := filepath.Join(r, ".forge", "instructions")
			if err := os.MkdirAll(outDir, 0o755); err != nil {
				return fmt.Errorf("mkdir instructions: %w", err)
			}

			var sb strings.Builder
			sb.WriteString("# Anti-patterns (auto-generated by forge learn antipatterns)\n\n")
			sb.WriteString(fmt.Sprintf("Generated: %s\n\n", time.Now().UTC().Format(time.RFC3339)))
			sb.WriteString("The following patterns were mined from git revert/hotfix commits (last 90 days):\n\n")
			if len(patterns) == 0 {
				sb.WriteString("_No revert/hotfix commits found in the last 90 days — great work!_\n")
			} else {
				for _, p := range patterns {
					sb.WriteString("- ")
					sb.WriteString(p)
					sb.WriteString("\n")
				}
			}

			if dryRun {
				fmt.Fprintln(cmd.OutOrStdout(), "learn antipatterns (dry-run): would write the following to anti-patterns.md:")
				fmt.Fprintln(cmd.OutOrStdout(), sb.String())
				return nil
			}

			outPath := filepath.Join(outDir, "anti-patterns.md")
			if err := os.WriteFile(outPath, []byte(sb.String()), 0o600); err != nil {
				return fmt.Errorf("write anti-patterns.md: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "learn antipatterns: wrote %d pattern(s) to %s\n",
				len(patterns), outPath)
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview output without writing anti-patterns.md")
	return cmd
}

// NewTeachCmd returns `forge learn teach` (G-032).
func NewTeachCmd(root *string, _ *bool) *cobra.Command {
	var text string
	cmd := &cobra.Command{
		Use:   "teach",
		Short: "Append a preference to .forge/learned/preferences.yml (G-032).",
		Long:  "Preferences are prepended to the LLM system prompt as a context block on every forge ship run.",
		RunE: func(cmd *cobra.Command, args []string) error {
			r, _ := resolveLearnRoot(*root)

			// Combine --text flag + positional args.
			parts := []string{text}
			parts = append(parts, args...)
			combined := strings.TrimSpace(strings.Join(parts, " "))
			if combined == "" {
				return fmt.Errorf("forge learn teach: provide preference text via --text or as argument")
			}

			prefDir := filepath.Join(r, ".forge", "learned")
			if err := os.MkdirAll(prefDir, 0o755); err != nil {
				return fmt.Errorf("mkdir learned: %w", err)
			}
			prefPath := filepath.Join(prefDir, "preferences.yml")
			f, err := os.OpenFile(prefPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
			if err != nil {
				return fmt.Errorf("open preferences.yml: %w", err)
			}
			defer f.Close()
			entry := fmt.Sprintf("- ts: %s\n  text: %q\n", time.Now().UTC().Format(time.RFC3339), combined)
			if _, err := f.WriteString(entry); err != nil {
				return fmt.Errorf("write preference: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "learn teach: preference appended to %s\n", prefPath)
			return nil
		},
	}
	cmd.Flags().StringVar(&text, "text", "", "Preference text to record")
	return cmd
}

// NewSessionCmd returns `forge learn session` (G-033).
func NewSessionCmd(root *string, _ *bool) *cobra.Command {
	return &cobra.Command{
		Use:   "session",
		Short: "Summarise .forge/session/*.log files and write a digest to stdout (G-033).",
		RunE: func(cmd *cobra.Command, _ []string) error {
			r, _ := resolveLearnRoot(*root)
			sessionDir := filepath.Join(r, ".forge", "session")
			entries, err := os.ReadDir(sessionDir)
			if err != nil {
				fmt.Fprintln(cmd.OutOrStdout(), "learn session: no .forge/session/ directory found (set FORGE_SESSION=1 to enable logging)")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "# Session Digest — %s\n\n", time.Now().UTC().Format(time.RFC3339))
			fileCount := 0
			for _, e := range entries {
				if !strings.HasSuffix(e.Name(), ".log") {
					continue
				}
				fileCount++
				data, err := os.ReadFile(filepath.Join(sessionDir, e.Name()))
				if err != nil {
					continue
				}
				lines := strings.Split(string(data), "\n")
				// Emit first 5 lines as preview.
				preview := lines
				if len(preview) > 5 {
					preview = preview[:5]
				}
				fmt.Fprintf(cmd.OutOrStdout(), "## %s (%d lines)\n", e.Name(), len(lines))
				for _, l := range preview {
					if l != "" {
						fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", l)
					}
				}
				fmt.Fprintln(cmd.OutOrStdout())
			}
			if fileCount == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "learn session: no .log files in .forge/session/")
			}
			return nil
		},
	}
}

// NewInstructionsCmd returns `forge learn instructions` (G-034).
func NewInstructionsCmd(root *string, _ *bool) *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "instructions",
		Short: "Propose edits to .forge/instructions/*.instructions.md from recent merged PRs (G-034).",
		RunE: func(cmd *cobra.Command, _ []string) error {
			r, _ := resolveLearnRoot(*root)
			_ = r

			ghPath, err := exec.LookPath("gh")
			if err != nil {
				fmt.Fprintln(cmd.OutOrStdout(), "learn instructions: gh CLI not found; cannot read PR history")
				return nil
			}

			// Fetch recent merged PRs.
			out, err := exec.Command(ghPath, "pr", "list", //nolint:gosec
				"--state=merged", "--limit=10", "--json=title,body").Output()
			if err != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "learn instructions: gh pr list failed: %v\n", err)
				return nil
			}

			type prEntry struct {
				Title string `json:"title"`
				Body  string `json:"body"`
			}
			var prs []prEntry
			if err := json.Unmarshal(out, &prs); err != nil {
				return fmt.Errorf("parse PR list: %w", err)
			}

			// Build proposed update content.
			var sb strings.Builder
			sb.WriteString("# Proposed .forge/instructions update\n\n")
			sb.WriteString(fmt.Sprintf("Generated: %s\n\n", time.Now().UTC().Format(time.RFC3339)))
			sb.WriteString("Based on the following recently merged PRs:\n\n")
			for _, pr := range prs {
				sb.WriteString(fmt.Sprintf("- %s\n", pr.Title))
			}
			sb.WriteString("\n## Proposed changes\n\n")
			sb.WriteString("Review and apply manually to .forge/instructions/*.instructions.md\n")

			proposal := sb.String()

			if dryRun {
				fmt.Fprintln(cmd.OutOrStdout(), proposal)
				return nil
			}

			// Create a PR with the proposal.
			prOut, err := exec.Command(ghPath, "pr", "create", //nolint:gosec
				"--title", "forge learn: update instructions from merged PR history",
				"--body", proposal, "--draft").CombinedOutput()
			if err != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "learn instructions: gh pr create failed: %v\n%s\n",
					err, strings.TrimSpace(string(prOut)))
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "learn instructions: draft PR created: %s\n",
				strings.TrimSpace(string(prOut)))
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print proposal without creating a PR")
	return cmd
}

// NewShareCmd implements G-035: `forge learn share`.
// Opt-in federated convention sharing. When learn.share=true in forge.yaml,
// sends an anonymized payload of convention counts (never code or PII).
func NewShareCmd(root *string, _ *bool) *cobra.Command {
	var optIn bool
	cmd := &cobra.Command{
		Use:   "share",
		Short: "Opt-in federated convention sharing: sends anonymized stats (G-035).",
		Long: "forge learn share opts into anonymous sharing of convention usage counts.\n" +
			"No code, file contents, or PII is ever included.\n" +
			"Toggle with: forge learn share --opt-in / --no-opt-in",
		RunE: func(cmd *cobra.Command, _ []string) error {
			r, _ := resolveLearnRoot(*root)

			// Read current opt-in status from forge.yaml (best-effort).
			forgeYAML := filepath.Join(r, "forge.yaml")
			data, _ := os.ReadFile(forgeYAML)
			currentOptIn := strings.Contains(string(data), "share: true")

			// --opt-in flag overrides.
			if cmd.Flags().Changed("opt-in") {
				if optIn {
					// Write opt-in into forge.yaml.
					var updated string
					if strings.Contains(string(data), "learn:") {
						updated = strings.ReplaceAll(string(data), "learn:", "learn:\n  share: true")
					} else {
						updated = string(data) + "\nlearn:\n  share: true\n"
					}
					if err := os.WriteFile(forgeYAML, []byte(updated), 0o600); err != nil {
						return fmt.Errorf("learn share: write forge.yaml: %w", err)
					}
					fmt.Fprintln(cmd.OutOrStdout(), "learn share: opted IN — anonymized convention counts will be shared.")
					currentOptIn = true
				} else {
					// Remove share flag.
					updated := strings.ReplaceAll(string(data), "  share: true\n", "")
					if err := os.WriteFile(forgeYAML, []byte(updated), 0o600); err != nil {
						return fmt.Errorf("learn share: write forge.yaml: %w", err)
					}
					fmt.Fprintln(cmd.OutOrStdout(), "learn share: opted OUT — no sharing.")
					currentOptIn = false
				}
			}

			if !currentOptIn {
				fmt.Fprintln(cmd.OutOrStdout(), "learn share: sharing is OFF. Use --opt-in to enable.")
				return nil
			}

			// Build anonymized payload.
			convFile := filepath.Join(r, ".forge", "conventions.jsonl")
			var count int
			if f, err := os.Open(convFile); err == nil { //nolint:gosec
				defer f.Close()
				sc := bufio.NewScanner(f)
				for sc.Scan() {
					count++
				}
			}
			payload := map[string]any{
				"schema":           "forge-learn-share/v1",
				"convention_count": count,
				// No code, no file paths, no PII.
			}
			out, _ := json.MarshalIndent(payload, "", "  ")
			fmt.Fprintf(cmd.OutOrStdout(), "learn share: payload (dry-run — no network call in MVP):\n%s\n", out)
			return nil
		},
	}
	cmd.Flags().BoolVar(&optIn, "opt-in", false, "Opt in to anonymous convention sharing")
	cmd.Flags().Bool("no-opt-in", false, "Opt out of anonymous convention sharing")
	return cmd
}
