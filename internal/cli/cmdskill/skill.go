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

// Package cmdskill implements `forge skill` — install, list, and remove the
// Forge expert role in the current project and/or VS Code workspace.
//
// # What it installs
//
// Running `forge skill install` writes the following files into the project's
// .github/ directory so that GitHub Copilot Chat and VS Code Copilot can pick
// them up automatically:
//
//   - .github/chatmodes/forge-expert.chatmode.md   — VS Code custom chat mode
//     (select "forge-expert" in the chat panel mode picker to unlock /forge-ship,
//     /forge-scan, /forge-bugfix and all other Forge capabilities)
//
//   - .github/instructions/forge-expert.instructions.md — scoped instructions
//     applied to all files in the project, teaching Copilot the Forge workflow
//
//   - .github/prompts/forge-ship.prompt.md    — reusable ship-workflow prompt
//
//   - .github/prompts/forge-scan.prompt.md    — reusable security-scan prompt
//
//   - .github/prompts/forge-bugfix.prompt.md  — reusable bug-fix prompt
//
// After installation users can open VS Code Chat, switch to the "forge-expert"
// chat mode, and type a description to run the Forge ship workflow, scan for
// vulnerabilities, or diagnose bugs — all backed by the Forge knowledge base.
package cmdskill

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/teragrid/forge/internal/errcode"
	"github.com/teragrid/forge/internal/verbmeta"
)

// Reserved error codes (range 6700..6799).
var (
	ErrSkillFailed = errcode.Register(errcode.Code(6700), "skill operation failed")
)

// SkillFile describes a single file that the skill installer manages.
type SkillFile struct {
	RelPath     string `json:"path"`
	Description string `json:"description"`
	Installed   bool   `json:"installed"`
}

// InstallResult is the outcome of `forge skill install`.
type InstallResult struct {
	Root    string      `json:"root"`
	Written []SkillFile `json:"written"`
	Skipped []SkillFile `json:"skipped"`
}

func init() {
	verbmeta.Register(verbmeta.Manifest{
		Verb:    "skill",
		Summary: "Install the Forge expert role into your project and VS Code Copilot.",
		Inputs: []string{
			"install   — write Forge expert chatmode + instructions + prompts",
			"list      — show which Forge skill files are present",
			"remove    — delete all Forge skill files from the project",
			"--root <path>  — project root (default: current directory)",
			"--name <name>  — skill name slug (default: forge-expert)",
			"--force / -f   — overwrite existing files",
			"--dry-run / -d — preview changes without writing",
			"--json / -j    — machine-readable JSON output",
		},
		Outputs: []string{
			"stdout: list of files written / skipped",
			".github/chatmodes/<name>.chatmode.md",
			".github/instructions/<name>.instructions.md",
			".github/prompts/forge-setup.prompt.md",
			".github/prompts/forge-ship.prompt.md",
			".github/prompts/forge-scan.prompt.md",
			".github/prompts/forge-bugfix.prompt.md",
		},
		SideEffects: []string{
			"Creates files under .github/ in the project root",
		},
		GatesTouched: []string{},
	})
}

// New returns the top-level `forge skill` cobra command.
func New() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skill",
		Short: "Install the Forge expert role into your project and VS Code Copilot.",
		Long: `forge skill installs the Forge expert AI role into the current project.

After installation, open VS Code Chat, switch to the "forge-expert" chat mode
(or any name you chose with --name), and describe what you want to build.
The assistant will guide you through forge ship, forge scan, forge bugfix and
all other Forge workflows backed by the Forge knowledge base.

The following files are created under .github/:

  chatmodes/<name>.chatmode.md        VS Code custom chat mode
  instructions/<name>.instructions.md Copilot repo-level instructions
  prompts/forge-ship.prompt.md        Reusable ship-workflow prompt
  prompts/forge-scan.prompt.md        Reusable security-scan prompt
  prompts/forge-bugfix.prompt.md      Reusable bug-fix prompt`,
	}
	cmd.AddCommand(newInstallCmd(), newListCmd(), newRemoveCmd())
	return cmd
}

// ── install ──────────────────────────────────────────────────────────────────

func newInstallCmd() *cobra.Command {
	var (
		root     string
		name     string
		platform string
		force    bool
		dryRun   bool
		jsonOut  bool
	)
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install Forge expert persona, workflow, and KB into your AI chat tools.",
		Example: `  forge skill install
  forge skill install --for claude
  forge skill install --for all --force
  forge skill install --dry-run`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if root == "" {
				var err error
				root, err = os.Getwd()
				if err != nil {
					return errcode.New(ErrSkillFailed, "cannot determine working directory", err)
				}
			}
			if !isValidPlatform(platform) {
				return errcode.New(ErrSkillFailed,
					fmt.Sprintf("unknown platform %q — valid values: copilot, claude, cursor, windsurf, all", platform), nil)
			}
			res, err := runInstall(root, name, platform, force, dryRun)
			if err != nil {
				return err
			}
			if jsonOut {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(res)
			}
			printInstallResult(cmd, res, platform, dryRun)
			return nil
		},
	}
	cmd.Flags().StringVarP(&root, "root", "r", "", "project root (default: current directory)")
	cmd.Flags().StringVarP(&name, "name", "n", "forge-expert", "skill name slug")
	cmd.Flags().StringVar(&platform, "for", PlatformAll, "target platform: copilot, claude, cursor, windsurf, all")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "overwrite existing files")
	cmd.Flags().BoolVarP(&dryRun, "dry-run", "d", false, "preview changes without writing")
	cmd.Flags().BoolVarP(&jsonOut, "json", "j", false, "machine-readable JSON output")
	return cmd
}

func isValidPlatform(p string) bool {
	for _, v := range ValidPlatforms {
		if p == v {
			return true
		}
	}
	return false
}

// ── list ─────────────────────────────────────────────────────────────────────

func newListCmd() *cobra.Command {
	var (
		root    string
		name    string
		jsonOut bool
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "Show which Forge skill files are installed in the project.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if root == "" {
				var err error
				root, err = os.Getwd()
				if err != nil {
					return errcode.New(ErrSkillFailed, "cannot determine working directory", err)
				}
			}
			files := skillFiles(root, name)
			if jsonOut {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(files)
			}
			for _, f := range files {
				status := "✗ missing"
				if f.Installed {
					status = "✓ present"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  %s  %s\n    %s\n", status, f.RelPath, f.Description)
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&root, "root", "r", "", "project root (default: current directory)")
	cmd.Flags().StringVarP(&name, "name", "n", "forge-expert", "skill name slug")
	cmd.Flags().BoolVarP(&jsonOut, "json", "j", false, "machine-readable JSON output")
	return cmd
}

// ── remove ────────────────────────────────────────────────────────────────────

func newRemoveCmd() *cobra.Command {
	var (
		root    string
		name    string
		yes     bool
		jsonOut bool
	)
	cmd := &cobra.Command{
		Use:   "remove",
		Short: "Delete all Forge skill files from the project.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if root == "" {
				var err error
				root, err = os.Getwd()
				if err != nil {
					return errcode.New(ErrSkillFailed, "cannot determine working directory", err)
				}
			}
			files := skillFiles(root, name)
			present := make([]SkillFile, 0, len(files))
			for _, f := range files {
				if f.Installed {
					present = append(present, f)
				}
			}
			if len(present) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no Forge skill files found — nothing to remove")
				return nil
			}
			if !yes {
				fmt.Fprintf(cmd.OutOrStdout(), "will remove %d file(s):\n", len(present))
				for _, f := range present {
					fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", f.RelPath)
				}
				fmt.Fprint(cmd.OutOrStdout(), "confirm? [y/N] ")
				var answer string
				if _, err := fmt.Fscan(cmd.InOrStdin(), &answer); err != nil || (answer != "y" && answer != "Y") {
					fmt.Fprintln(cmd.OutOrStdout(), "aborted")
					return nil
				}
			}
			removed := 0
			for _, f := range present {
				full := filepath.Join(root, f.RelPath)
				if err := os.Remove(full); err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not remove %s: %v\n", f.RelPath, err)
					continue
				}
				removed++
			}
			if jsonOut {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]int{"removed": removed})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "removed %d file(s)\n", removed)
			return nil
		},
	}
	cmd.Flags().StringVarP(&root, "root", "r", "", "project root (default: current directory)")
	cmd.Flags().StringVarP(&name, "name", "n", "forge-expert", "skill name slug")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip confirmation prompt")
	cmd.Flags().BoolVarP(&yes, "force", "f", false, "skip confirmation prompt (alias for --yes)")
	cmd.Flags().BoolVarP(&jsonOut, "json", "j", false, "machine-readable JSON output")
	return cmd
}

// ── core logic ────────────────────────────────────────────────────────────────

func runInstall(root, name, platform string, force, dryRun bool) (*InstallResult, error) {
	res := &InstallResult{Root: root}
	for _, sf := range buildFilesForPlatform(root, name, platform) {
		full := filepath.Join(root, sf.RelPath)
		if _, err := os.Stat(full); err == nil && !force {
			sf.SkillFile.Installed = false
			res.Skipped = append(res.Skipped, sf.SkillFile)
			continue
		}
		sf.SkillFile.Installed = true
		res.Written = append(res.Written, sf.SkillFile)
		if dryRun {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return nil, errcode.New(ErrSkillFailed, "cannot create directory for "+sf.RelPath, err)
		}
		if err := os.WriteFile(full, []byte(sf.content), 0o600); err != nil {
			return nil, errcode.New(ErrSkillFailed, "cannot write "+sf.RelPath, err)
		}
	}
	return res, nil
}

func skillFiles(root, name string) []SkillFile {
	out := make([]SkillFile, 0, 5)
	for _, sf := range buildFilesForPlatform(root, name, PlatformAll) {
		full := filepath.Join(root, sf.RelPath)
		_, err := os.Stat(full)
		sf.Installed = err == nil
		out = append(out, sf.SkillFile)
	}
	return out
}

func printInstallResult(cmd *cobra.Command, res *InstallResult, platform string, dryRun bool) {
	verb := "wrote"
	if dryRun {
		verb = "would write"
	}
	for _, f := range res.Written {
		fmt.Fprintf(cmd.OutOrStdout(), "  ✓ %s  %s — %s\n", verb, f.RelPath, f.Description)
	}
	for _, f := range res.Skipped {
		fmt.Fprintf(cmd.OutOrStdout(), "  – skipped  %s (already exists; use --force to overwrite)\n", f.RelPath)
	}
	if len(res.Written) > 0 && !dryRun {
		fmt.Fprintln(cmd.OutOrStdout())
		fmt.Fprintln(cmd.OutOrStdout(), "Done! Next steps:")
		for i, step := range platformNextSteps(platform) {
			fmt.Fprintf(cmd.OutOrStdout(), "  %d. %s\n", i+1, step)
		}
	}
}

// ── file content builders ─────────────────────────────────────────────────────

// skillFileInternal carries the SkillFile fields plus the file content.
type skillFileInternal struct {
	SkillFile
	content string
}

func buildSkillFiles(root, name string) []skillFileInternal {
	_ = root // reserved for future KB-enriched content
	return []skillFileInternal{
		chatmodeFile(name),
		instructionsFile(name),
		setupPromptFile(),
		shipPromptFile(),
		scanPromptFile(),
		bugfixPromptFile(),
	}
}

func chatmodeFile(name string) skillFileInternal {
	q := "`"
	content := `---
description: Forge AI development expert — ship, scan, bugfix and more with the full Forge workflow
tools:
  - read_file
  - create_file
  - replace_string_in_file
  - run_in_terminal
  - file_search
  - grep_search
  - semantic_search
  - get_errors
---
You are the **Forge expert AI assistant** — an elite software engineer with deep knowledge of the Forge AI development framework.

> **You are running inside VS Code GitHub Copilot.** You ARE the LLM — no
> separate LLM configuration is needed. Every ` + q + `forge` + q + ` CLI command you run
> uses GitHub Copilot automatically via ` + q + `llm.provider = copilot` + q + `.

## Auto-setup (run silently at the start of every session)

Perform these checks once before answering the first request. Do not explain
each step unless something fails.

**1. Ensure Forge CLI is installed**

Run ` + q + `forge --version` + q + `. If the command is not found, install it:

| OS | Command |
|----|---------|
| macOS | ` + q + `brew install teragrid/tap/forge` + q + ` |
| Windows | ` + q + `winget install teragrid.forge` + q + ` |
| Linux / CI | ` + q + `npm install -g @forgeone/cli` + q + ` |
| Go toolchain | ` + q + `go install github.com/teragrid/forge/cmd/forge@latest` + q + ` |

**2. Pin the LLM provider to Copilot and verify scope (no prompt needed)**

Because you are already inside VS Code Copilot, GitHub Copilot is always
available. Run these automatically — **do not ask the user**:

` + "```" + `
forge config set llm.provider copilot
` + "```" + `

Then verify the token has the required ` + q + `copilot` + q + ` OAuth scope:

` + "```" + `
gh auth status
` + "```" + `

If the output shows scopes that do **not** include ` + q + `copilot` + q + `, run:

` + "```" + `
gh auth refresh -s copilot
` + "```" + `

This adds the scope without creating a new token. After refreshing, retry any
` + q + `forge` + q + ` command that previously returned a 401/403 error.

**3. Initialise the project if needed**

If ` + q + `.forge/` + q + ` does not exist, run ` + q + `forge init` + q + ` automatically.

**4. Install the skill files if missing**

If ` + q + `.github/chatmodes/` + name + `.chatmode.md` + q + ` does not exist, run
` + q + `forge skill install` + q + ` automatically.

---

## Your capabilities

| Command | What it does |
|---------|-------------|
| ` + q + `forge ship "description"` + q + ` | Full 6-stage quality gate: spec → arch → test → breakdown → code → ship |
| ` + q + `forge ship spec "description"` + q + ` | Generate a 9-point YAML test spec |
| ` + q + `forge ship arch "description"` + q + ` | Design the system architecture |
| ` + q + `forge ship test "description"` + q + ` | Write tests against the spec |
| ` + q + `forge ship code "description"` + q + ` | Implement the feature |
| ` + q + `forge scan all` + q + ` | Security, secrets, vulnerabilities, and code quality |
| ` + q + `forge scan security` + q + ` | OWASP Top 10 + CVE check |
| ` + q + `forge bugfix --bug "description"` + q + ` | Diagnose and auto-fix a bug with LLM assistance |
| ` + q + `forge new <template> <name>` + q + ` | Scaffold a production-grade project from a template |
| ` + q + `forge init` + q + ` | Initialise Forge in an existing project |
| ` + q + `forge skill install` + q + ` | Install the Forge expert role in VS Code Copilot |
| ` + q + `forge config set llm.model <model>` + q + ` | Override the Copilot model (e.g. gpt-4o, claude-sonnet) |
| ` + q + `forge audit` + q + ` | Audit the AI decision trail |
| ` + q + `forge explain <verb>` + q + ` | Learn what any command does |
| ` + q + `forge check` + q + ` | Validate project conventions and schema contracts |
| ` + q + `forge context generate` + q + ` | Build project context for LLM calls |
| ` + q + `forge undo` + q + ` | Reverse the last reversible Forge operation |

## How to respond to user requests

1. **Clarify scope** if ambiguous (one question maximum).
2. **Run the forge command** directly when possible — you have terminal access.
3. **Walk through output** of each stage if the user wants detail.
4. **Use the Forge KB** — ` + q + `.forge/` + q + ` contains specs, checkpoints, and context. Read relevant files before answering.
5. **Follow Forge test design** before writing any code: happy path, boundaries, negatives, idempotency, concurrency, cross-tenant, backward-compat, data-accuracy, false-positive guard.
6. **Security-first** — flag OWASP Top 10 issues immediately; run ` + q + `forge scan security` + q + ` after any change.

## Forge ship workflow stages

` + "```" + `
[1/6] spec      — 9-point YAML test spec  (.forge/specs/<slug>/spec.yml)
[2/6] arch      — architecture design     (.forge/specs/<slug>/arch.md)
[3/6] test      — test suite scaffolding
[4/6] breakdown — implementation plan
[5/6] code      — implementation
[6/6] ship      — quality gate: format + vet + lint + test + vuln + forge scan
` + "```" + `
`
	return skillFileInternal{
		SkillFile: SkillFile{
			RelPath:     filepath.Join(".github", "chatmodes", name+".chatmode.md"),
			Description: "VS Code custom chat mode — select from the chat mode picker",
		},
		content: content,
	}
}

func instructionsFile(name string) skillFileInternal {
	content := fmt.Sprintf(`---
applyTo: "**"
---
# Forge Expert Instructions

This project uses the **Forge AI development framework**. Apply Forge best practices in all responses.

## Forge project conventions

- All AI changes go through `+"`"+`forge ship`+"`"+` or a Forge checkpoint command.
- Specs live in `+"`"+`.forge/specs/<slug>/spec.yml`+"`"+` — read them before implementing.
- Error codes use `+"`"+`errcode.Register(errcode.Code(NNNN), "message")`+"`"+`. Never reuse codes.
- No CGO (`+"`"+`CGO_ENABLED=0`+"`"+`); no `+"`"+`os.Exit`+"`"+` except in `+"`"+`cmd/forge/main.go`+"`"+`.
- All subprocess execution via `+"`"+`internal/procspawn`+"`"+` (allow-list only).
- All user paths validated via `+"`"+`internal/fssandbox`+"`"+`.
- No secrets in code; use `+"`"+`internal/secretrewriter`+"`"+` for redaction.

## Test design (mandatory before coding)

Before writing any test or fix, enumerate:
1. Happy path — the intended scenario succeeds.
2. Boundary cases — empty/null, zero, max, min, off-by-one.
3. Negative cases — invalid input, unauthorised, wrong tenant.
4. Idempotency / replay — same operation twice.
5. Concurrency / race — two writers, out-of-order arrival.
6. Cross-tenant / authz — user A cannot affect user B.
7. Backward-compat / regression — the original bug must be in the suite.
8. Data-accuracy — real inserts → query back → assert correctness.
9. False-positive guard — a case where the new check MUST NOT trigger.

## LLM calls

Use `+"`"+`internal/llmprovider`+"`"+`:
`+"`"+``+"`"+``+"`"+`go
p, err := llmprovider.Detect()
resp, err := p.Complete(ctx, &llmprovider.Request{...})
`+"`"+``+"`"+``+"`"+`

## Skill: %s

Managed by `+"`"+`forge skill`+"`"+`. Run `+"`"+`forge skill list`+"`"+` to see all installed files.
`, name)
	return skillFileInternal{
		SkillFile: SkillFile{
			RelPath:     filepath.Join(".github", "instructions", name+".instructions.md"),
			Description: "Scoped Copilot instructions applied to all files in the project",
		},
		content: content,
	}
}

func setupPromptFile() skillFileInternal {
	q := "`"
	content := `---
mode: agent
description: Install and configure the Forge framework in this project
tools:
  - run_in_terminal
  - read_file
  - create_file
---
Set up the Forge AI development framework in this project from scratch.

**Step 1 — Install Forge CLI**

Check if Forge is installed:

` + "```" + `
forge --version
` + "```" + `

If not found, detect the OS and run the appropriate installer:

| OS | Command |
|----|---------|
| macOS | ` + q + `brew install teragrid/tap/forge` + q + ` |
| Windows | ` + q + `winget install teragrid.forge` + q + ` |
| Linux / CI | ` + q + `npm install -g @forgeone/cli` + q + ` |
| Go toolchain | ` + q + `go install github.com/teragrid/forge/cmd/forge@latest` + q + ` |

Confirm: ` + q + `forge --version` + q + `

**Step 2 — Pin the LLM provider**

` + "```" + `
gh auth status
` + "```" + `

- If ` + q + `gh` + q + ` is authenticated (or you are running this from VS Code Copilot), run:
  ` + q + `forge config set llm.provider copilot` + q + ` — **no API key needed**

  Then check ` + q + `gh auth status` + q + `. If the token scopes do **not** include ` + q + `copilot` + q + `, run:
  ` + q + `gh auth refresh -s copilot` + q + ` to add the scope without re-authenticating.

- Otherwise set the provider for the environment:
  - ` + q + `forge config set llm.provider openai` + q + `    → requires OPENAI_API_KEY
  - ` + q + `forge config set llm.provider anthropic` + q + `  → requires ANTHROPIC_API_KEY
  - ` + q + `forge config set llm.provider ollama` + q + `     → local, no key needed

> If running this prompt from VS Code Copilot chat, always choose **copilot**.

**Step 3 — Initialise Forge in the project**

` + "```" + `
forge init
` + "```" + `

**Step 4 — Install the Forge expert skill**

` + "```" + `
forge skill install
` + "```" + `

**Step 5 — Verify**

` + "```" + `
forge check
forge skill list
` + "```" + `

Report what was done and confirm the user can now use ` + q + `forge ship` + q + ` commands.
`
	return skillFileInternal{
		SkillFile: SkillFile{
			RelPath:     filepath.Join(".github", "prompts", "forge-setup.prompt.md"),
			Description: "Reusable prompt: install and configure Forge from scratch",
		},
		content: content,
	}
}

func shipPromptFile() skillFileInternal {
	content := `---
mode: agent
description: Run the Forge ship workflow for a new feature
tools:
  - run_in_terminal
  - read_file
  - grep_search
---
Run the Forge ship workflow for the feature described below.

1. Run ` + "`" + `forge ship spec "$DESCRIPTION"` + "`" + ` and show the generated spec.
2. Run ` + "`" + `forge ship arch "$DESCRIPTION"` + "`" + ` and summarise the architecture.
3. Run ` + "`" + `forge ship test "$DESCRIPTION"` + "`" + ` to scaffold the test suite.
4. Run ` + "`" + `forge ship code "$DESCRIPTION"` + "`" + ` to implement the feature.
5. Run ` + "`" + `forge ship "$DESCRIPTION"` + "`" + ` to execute the full quality gate.
6. If the gate fails, diagnose and fix each issue before re-running.

Feature description: $DESCRIPTION
`
	return skillFileInternal{
		SkillFile: SkillFile{
			RelPath:     filepath.Join(".github", "prompts", "forge-ship.prompt.md"),
			Description: "Reusable prompt: run the Forge ship workflow",
		},
		content: content,
	}
}

func scanPromptFile() skillFileInternal {
	content := `---
mode: agent
description: Run a Forge security and quality scan
tools:
  - run_in_terminal
  - read_file
---
Run a full Forge security and quality scan on the current project.

1. Run ` + "`" + `forge scan all` + "`" + ` and summarise the findings by severity.
2. For each HIGH or CRITICAL finding, explain the root cause and suggest a fix.
3. Run ` + "`" + `forge scan security` + "`" + ` specifically for OWASP Top 10 issues.
4. If vulnerabilities are found in dependencies, suggest ` + "`" + `forge upgrade` + "`" + `.
5. Prioritise fixes: critical > high > medium > low > info.
`
	return skillFileInternal{
		SkillFile: SkillFile{
			RelPath:     filepath.Join(".github", "prompts", "forge-scan.prompt.md"),
			Description: "Reusable prompt: run a Forge security and quality scan",
		},
		content: content,
	}
}

func bugfixPromptFile() skillFileInternal {
	content := `---
mode: agent
description: Diagnose and fix a bug using the Forge bugfix workflow
tools:
  - run_in_terminal
  - read_file
  - grep_search
  - replace_string_in_file
---
Diagnose and fix the bug described below using the Forge bugfix workflow.

1. Run ` + "`" + `forge bugfix --bug "$BUG_DESCRIPTION" --apply` + "`" + `.
2. Show the root-cause analysis from the output.
3. Show the patch that was applied.
4. Write a regression test that would have caught this bug.
5. Run ` + "`" + `forge scan security` + "`" + ` to verify the fix does not introduce new issues.
6. Run ` + "`" + `forge ship` + "`" + ` to execute the full quality gate.

Bug description: $BUG_DESCRIPTION
`
	return skillFileInternal{
		SkillFile: SkillFile{
			RelPath:     filepath.Join(".github", "prompts", "forge-bugfix.prompt.md"),
			Description: "Reusable prompt: diagnose and fix a bug with Forge",
		},
		content: content,
	}
}
