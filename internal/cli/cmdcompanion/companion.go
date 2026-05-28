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

// Package cmdcompanion implements `forge companion` — zero-setup AI pairing for
// the Forge framework.
//
// forge companion is the recommended first command to run after `forge init`.
// It detects which AI tools are installed (VS Code Copilot, Claude, Cursor,
// Windsurf), generates the matching skill / agent files, and prints a
// copy-paste vibe-coding quick-start guide.
//
// Subcommands:
//
//	forge companion            — detect + install missing skill files (interactive)
//	forge companion install    — alias for forge skill install --for all
//	forge companion update     — regenerate all skill files (picks up KB changes)
//	forge companion status     — show which platforms are configured
//	forge companion guide      — print the vibe-coding quick-start guide
package cmdcompanion

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/teragrid/forge/internal/cli/cmdskill"
	"github.com/teragrid/forge/internal/errcode"
	"github.com/teragrid/forge/internal/verbmeta"
)

// Reserved error codes (range 6650..6699).
var (
	ErrCompanionFailed = errcode.Register(errcode.Code(6650), "companion setup failed")
)

func init() {
	verbmeta.Register(verbmeta.Manifest{
		Verb: "companion",
		Summary: "Zero-setup AI pairing: install the Forge expert persona in every AI tool " +
			"(VS Code Copilot, Claude, Cursor, Windsurf) and get a vibe-coding quick-start.",
		Inputs: []string{
			"[install]        — install skill files for all detected AI platforms",
			"[update]         — regenerate all skill files (refresh KB + templates)",
			"[status]         — show which platforms already have skill files",
			"[guide]          — print the vibe-coding quick-start cheatsheet",
			"--root <path>    — project root (default: current directory)",
			"--for <platform> — target only: copilot, claude, cursor, windsurf, all",
			"--force / -f     — overwrite existing skill files",
			"--yes / -y       — non-interactive; skip confirmation prompt",
		},
		Outputs: []string{
			"stdout: installation summary and quick-start guide",
			".github/chatmodes/forge-expert.chatmode.md (Copilot)",
			".github/instructions/forge-expert.instructions.md (Copilot)",
			".github/prompts/forge-*.prompt.md (Copilot)",
			"CLAUDE.md + .claude/commands/ (Claude)",
			".cursor/rules/forge-expert.mdc (Cursor)",
			".windsurfrules (Windsurf)",
		},
		SideEffects: []string{
			"Creates AI configuration files under .github/, CLAUDE.md, .cursor/, .windsurfrules",
		},
		GatesTouched: []string{},
		ErrorCodes:   []errcode.Code{ErrCompanionFailed},
	})
}

// New returns the top-level `forge companion` cobra command.
func New() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "companion",
		Short: "Zero-setup AI pairing: configure every AI tool as a Forge expert.",
		Long: `forge companion wires the Forge expert persona into every AI coding tool
you have installed — VS Code Copilot, Claude, Cursor, and Windsurf.

After running forge companion, open your AI chat tool and describe what you
want to build in plain English. The assistant will guide you through the full
Forge ship workflow: spec → scaffold → implement → test → review → ship.

  forge companion          — detect + install (all platforms, safe defaults)
  forge companion update   — regenerate to pick up latest KB changes
  forge companion status   — show installed platforms
  forge companion guide    — print the vibe-coding quick-start cheatsheet`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			// Default action: install all platforms (safe — skips existing files).
			return runCompanionInstall(cmd, "", "all", false, false)
		},
	}
	cmd.AddCommand(
		newInstallSubCmd(),
		newUpdateSubCmd(),
		newStatusSubCmd(),
		newGuideSubCmd(),
	)
	return cmd
}

// ── install ───────────────────────────────────────────────────────────────────

func newInstallSubCmd() *cobra.Command {
	var (
		root     string
		platform string
		force    bool
		yes      bool
	)
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install the Forge expert persona for all AI tools.",
		Example: `  forge companion install
  forge companion install --for copilot
  forge companion install --for claude --force`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runCompanionInstall(cmd, root, platform, force, yes)
		},
	}
	cmd.Flags().StringVarP(&root, "root", "r", "", "project root (default: cwd)")
	cmd.Flags().StringVar(&platform, "for", "all", "target platform: copilot, claude, cursor, windsurf, all")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "overwrite existing skill files")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "non-interactive (skip confirmation)")
	return cmd
}

// ── update ────────────────────────────────────────────────────────────────────

func newUpdateSubCmd() *cobra.Command {
	var (
		root     string
		platform string
	)
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Regenerate skill files to pick up new KB entries and template changes.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runCompanionInstall(cmd, root, platform, true /*force*/, true /*yes*/)
		},
	}
	cmd.Flags().StringVarP(&root, "root", "r", "", "project root (default: cwd)")
	cmd.Flags().StringVar(&platform, "for", "all", "target platform: copilot, claude, cursor, windsurf, all")
	return cmd
}

// ── status ────────────────────────────────────────────────────────────────────

func newStatusSubCmd() *cobra.Command {
	var root string
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show which AI platforms have the Forge expert persona installed.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if root == "" {
				var err error
				root, err = os.Getwd()
				if err != nil {
					return errcode.New(ErrCompanionFailed, "cannot determine working directory", err)
				}
			}
			printCompanionStatus(cmd, root)
			return nil
		},
	}
	cmd.Flags().StringVarP(&root, "root", "r", "", "project root (default: cwd)")
	return cmd
}

// ── guide ─────────────────────────────────────────────────────────────────────

func newGuideSubCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "guide",
		Short: "Print the vibe-coding quick-start guide.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			fmt.Fprint(cmd.OutOrStdout(), vibeCodeGuide())
			return nil
		},
	}
	return cmd
}

// ── implementation ─────────────────────────────────────────────────────────────

func runCompanionInstall(cmd *cobra.Command, root, platform string, force, _ bool) error {
	if root == "" {
		var err error
		root, err = os.Getwd()
		if err != nil {
			return errcode.New(ErrCompanionFailed, "cannot determine working directory", err)
		}
	}

	fmt.Fprintln(cmd.OutOrStdout(), "")
	fmt.Fprintln(cmd.OutOrStdout(), "  Forge Companion — AI Pairing Setup")
	fmt.Fprintln(cmd.OutOrStdout(), "  ───────────────────────────────────")
	fmt.Fprintf(cmd.OutOrStdout(), "  Project root : %s\n", root)
	fmt.Fprintf(cmd.OutOrStdout(), "  Platform     : %s\n", platform)
	fmt.Fprintln(cmd.OutOrStdout(), "")

	res, err := cmdskill.RunInstall(root, "forge-expert", platform, force, false)
	if err != nil {
		return errcode.New(ErrCompanionFailed, "skill installation failed", err)
	}

	for _, f := range res.Written {
		fmt.Fprintf(cmd.OutOrStdout(), "  ✓  %s\n", f.RelPath)
	}
	for _, f := range res.Skipped {
		fmt.Fprintf(cmd.OutOrStdout(), "  –  %s  (already installed)\n", f.RelPath)
	}

	if len(res.Written) == 0 && len(res.Skipped) > 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "")
		fmt.Fprintln(cmd.OutOrStdout(), "  Already up to date. Run `forge companion update` to refresh.")
		return nil
	}

	fmt.Fprintln(cmd.OutOrStdout(), "")
	fmt.Fprintln(cmd.OutOrStdout(), "  ✨ Forge AI companion installed!")
	fmt.Fprintln(cmd.OutOrStdout(), "")
	fmt.Fprintln(cmd.OutOrStdout(), "  How to use it:")
	fmt.Fprintln(cmd.OutOrStdout(), "  1. Open VS Code Chat → switch to \"forge-expert\" mode")
	fmt.Fprintln(cmd.OutOrStdout(), "     (or Claude/Cursor/Windsurf — all configured)")
	fmt.Fprintln(cmd.OutOrStdout(), "  2. Describe what you want to build in plain English")
	fmt.Fprintln(cmd.OutOrStdout(), "  3. The assistant runs the full Forge workflow for you")
	fmt.Fprintln(cmd.OutOrStdout(), "")
	fmt.Fprintln(cmd.OutOrStdout(), "  Run `forge companion guide` for vibe-coding examples.")
	fmt.Fprintln(cmd.OutOrStdout(), "")
	return nil
}

// platformFiles lists the indicator files per platform.
var platformFiles = map[string]string{
	"VS Code Copilot": ".github/chatmodes/forge-expert.chatmode.md",
	"Claude":          "CLAUDE.md",
	"Cursor":          ".cursor/rules/forge-expert.mdc",
	"Windsurf":        ".windsurfrules",
}

func printCompanionStatus(cmd *cobra.Command, root string) {
	fmt.Fprintln(cmd.OutOrStdout(), "")
	fmt.Fprintln(cmd.OutOrStdout(), "  Forge Companion — Platform Status")
	fmt.Fprintln(cmd.OutOrStdout(), "  ──────────────────────────────────")
	for platform, rel := range platformFiles {
		path := filepath.Join(root, rel)
		if _, err := os.Stat(path); err == nil {
			fmt.Fprintf(cmd.OutOrStdout(), "  ✓  %-20s  %s\n", platform, rel)
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "  –  %-20s  not installed\n", platform)
		}
	}
	fmt.Fprintln(cmd.OutOrStdout(), "")
	fmt.Fprintln(cmd.OutOrStdout(), "  Run `forge companion install` to configure missing platforms.")
	fmt.Fprintln(cmd.OutOrStdout(), "")
}

// vibeCodeGuide returns the vibe-coding quick-start cheatsheet.
func vibeCodeGuide() string {
	// Cannot use a raw string literal here because the content contains backtick
	// characters (e.g. `forge skill list`).  We build the string via concatenation
	// so the Go parser is not confused by embedded backticks.
	bt := "`" // backtick character
	return "\n" +
		"  ╔══════════════════════════════════════════════════════════════════════╗\n" +
		"  ║           Forge Vibe-Coding Quick-Start Guide                       ║\n" +
		"  ╚══════════════════════════════════════════════════════════════════════╝\n" +
		"\n" +
		"  WHAT IS VIBE-CODING WITH FORGE?\n" +
		"  ─────────────────────────────────────────────────────────────────────\n" +
		"  You describe what you want in plain English.\n" +
		"  The Forge AI companion handles spec → scaffold → implement → test →\n" +
		"  security-check → commit — the complete production-quality workflow.\n" +
		"\n" +
		"  ══════════════════════════════════════════════════════════════════════\n" +
		"  DAILY WORKFLOW PATTERNS\n" +
		"  ══════════════════════════════════════════════════════════════════════\n" +
		"\n" +
		"  ┌─ Feature Workflow ─────────────────────────────────────────────────┐\n" +
		"  │                                                                     │\n" +
		"  │  You:  \"Build a rate-limiter middleware for the API gateway.        │\n" +
		"  │         It should allow 100 req/min per client IP, return 429       │\n" +
		"  │         with Retry-After header, and store counters in Redis.\"      │\n" +
		"  │                                                                     │\n" +
		"  │  AI:   Creates spec → scaffolds middleware → implements with        │\n" +
		"  │        Redis sliding window → writes 9-point test suite →           │\n" +
		"  │        runs security check → commits on feature branch              │\n" +
		"  └─────────────────────────────────────────────────────────────────────┘\n" +
		"\n" +
		"  ┌─ Bugfix Workflow ───────────────────────────────────────────────────┐\n" +
		"  │                                                                     │\n" +
		"  │  You:  \"Fix this panic: goroutine 1 [running]:                      │\n" +
		"  │         runtime error: index out of range [3] with length 3         │\n" +
		"  │         cmdship/pipeline.go:147 +0x2a4\"                             │\n" +
		"  │                                                                     │\n" +
		"  │  AI:   Reads file:147 → writes failing test → traces root cause →  │\n" +
		"  │        applies minimal fix → verifies test passes → regression guard │\n" +
		"  └─────────────────────────────────────────────────────────────────────┘\n" +
		"\n" +
		"  ┌─ Security Scan Workflow ────────────────────────────────────────────┐\n" +
		"  │                                                                     │\n" +
		"  │  You:  \"Scan the last 3 commits for secrets, OWASP Top 10           │\n" +
		"  │         violations, and CVEs in the dependency tree.\"               │\n" +
		"  │                                                                     │\n" +
		"  │  AI:   Checks git diff → scans for hardcoded creds → audits deps →  │\n" +
		"  │        reports: severity / file / line / remediation                │\n" +
		"  └─────────────────────────────────────────────────────────────────────┘\n" +
		"\n" +
		"  ┌─ Morning Standup Workflow ──────────────────────────────────────────┐\n" +
		"  │                                                                     │\n" +
		"  │  You:  \"Summarise what was shipped yesterday, what's in-flight,     │\n" +
		"  │         and any blockers from the current forge ship status.\"        │\n" +
		"  │                                                                     │\n" +
		"  │  AI:   Reads .forge/specs/ + git log + open PRs → formats standup   │\n" +
		"  └─────────────────────────────────────────────────────────────────────┘\n" +
		"\n" +
		"  ┌─ Code Review Workflow ──────────────────────────────────────────────┐\n" +
		"  │                                                                     │\n" +
		"  │  You:  \"Review this PR. Focus on: correctness of the rate-limiter   │\n" +
		"  │         algorithm, error handling, and test coverage gaps.\"          │\n" +
		"  │                                                                     │\n" +
		"  │  AI:   Reads diff → checks spec alignment → identifies gaps →       │\n" +
		"  │        produces inline comments with severity + suggested fixes      │\n" +
		"  └─────────────────────────────────────────────────────────────────────┘\n" +
		"\n" +
		"  ══════════════════════════════════════════════════════════════════════\n" +
		"  TOP 10 DAILY COMMANDS (paste directly into your AI chat)\n" +
		"  ══════════════════════════════════════════════════════════════════════\n" +
		"\n" +
		"  1.  \"Ship <feature description>\"\n" +
		"      → full 6-stage pipeline: spec, scaffold, implement, test, scan, commit\n" +
		"\n" +
		"  2.  \"forge bugfix: <paste error/stacktrace>\"\n" +
		"      → reproduce → root cause → fix → verify → regression guard\n" +
		"\n" +
		"  3.  \"forge scan for secrets and OWASP issues in the current branch\"\n" +
		"      → comprehensive security review before PR\n" +
		"\n" +
		"  4.  \"forge review <file or PR> focusing on <concern>\"\n" +
		"      → AI code review with actionable inline suggestions\n" +
		"\n" +
		"  5.  \"forge test <function/module> using the 9-point framework\"\n" +
		"      → test design → happy/boundary/negative/race/authz/regression\n" +
		"\n" +
		"  6.  \"Upgrade dependencies and fix breaking changes in <package>\"\n" +
		"      → safe upgrade with change-log analysis and migration\n" +
		"\n" +
		"  7.  \"Add a <component type> to this project following Forge conventions\"\n" +
		"      → type-safe scaffold + wired into existing patterns\n" +
		"\n" +
		"  8.  \"Explain what forge <command> does with an example\"\n" +
		"      → learn any Forge verb with real usage context\n" +
		"\n" +
		"  9.  \"What's the forge error code for <description>? Pick the next available.\"\n" +
		"      → error-code assignment from docs/ERROR_CODES.md\n" +
		"\n" +
		"  10. \"Prepare a postmortem for the incident: <brief description>\"\n" +
		"      → structured postmortem with root cause, timeline, action items\n" +
		"\n" +
		"  ══════════════════════════════════════════════════════════════════════\n" +
		"  TIPS\n" +
		"  ══════════════════════════════════════════════════════════════════════\n" +
		"\n" +
		"  • Be specific about constraints: \"Redis\", \"no CGO\", \"must be idempotent\"\n" +
		"  • Mention the target file when fixing bugs: use forge bugfix with the filename\n" +
		"  • Ask for the test design BEFORE the implementation: \"Design the tests first\"\n" +
		"  • Use \"dry-run\" when exploring: \"What would forge scan find here?\"\n" +
		"  • Keep the AI in Forge mode: switch to \"forge-expert\" chat mode in VS Code\n" +
		"\n" +
		"  Run " + bt + "forge skill list" + bt + " to see all installed skill files.\n" +
		"  Run " + bt + "forge companion update" + bt + " to refresh after a forge upgrade.\n" +
		"\n"
}
