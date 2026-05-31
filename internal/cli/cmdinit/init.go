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

// Package cmdinit implements `forge init` — idiomatic project initialisation
// for the current directory (analogous to `git init` / `npm init`).
//
// Auto-detects the template to apply:
//   - package.json present  → ts-service
//   - go.mod present        → go-service
//   - (default)             → ts-service
//
// For polyglot monorepos both files may be present; --template overrides.
package cmdinit

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/teragrid/forge/internal/codemod"
	"github.com/teragrid/forge/internal/errcode"
	"github.com/teragrid/forge/internal/knowledge"
	"github.com/teragrid/forge/internal/scaffold"
	"github.com/teragrid/forge/internal/verbmeta"
)

// Reserved error codes (range 5100..5199).
var (
	ErrInitFailed     = errcode.Register(errcode.Code(5100), "forge init failed")
	ErrInitUsage      = errcode.Register(errcode.Code(5101), "invalid usage of `forge init`")
	ErrKnowledgeWrite = errcode.Register(errcode.Code(5102), "knowledge injection failed")
)

func init() {
	verbmeta.Register(verbmeta.Manifest{
		Verb: "init",
		Summary: "Initialise the current directory as a Forge project (like git init / npm init).\n" +
			"Auto-detects the best template from existing files.",
		Inputs: []string{
			"--template <name>  (override auto-detection; default: auto)",
			"--name <name>      (project name; default: current directory name)",
			"--minimal          (inject only forge runtime files + knowledge index into an existing project)",
			"--force            (overwrite files that already exist; also overwrites drifted forge-managed blocks)",
			"--json             (machine-readable output)",
		},
		Outputs: []string{
			"<cwd>/* — rendered template files",
			"stdout  — created file list (text or JSON)",
		},
		SideEffects: []string{
			"writes Forge scaffolding into the current directory",
			"always injects forge blocks into .gitignore, .gitleaks.toml, .pre-commit-config.yaml, .github/dependabot.yml (safe: user content preserved)",
			"--minimal: writes .forge/ config + AGENTS.md + knowledge-index.json + global.instructions.md",
		},
		GatesTouched: []string{"§4 scaffold"},
		ErrorCodes:   []errcode.Code{ErrInitFailed, ErrInitUsage, ErrKnowledgeWrite},
	})
}

// New returns the cobra command for `forge init`.
func New(forgeVersion string) *cobra.Command {
	var (
		tmplFlag string
		name     string
		force    bool
		minimal  bool
		asJSON   bool
	)

	cmd := &cobra.Command{
		Use:   "init [path]",
		Short: "Initialise a directory as a Forge project.",
		Long: "forge init applies a scaffold template to a directory (default: current directory).\n\n" +
			"Quick start for an existing project (run from inside it):\n" +
			"  forge init --minimal\n\n" +
			"--minimal wires forge into an existing project with minimal footprint.\n" +
			"The project name defaults to the current directory name — no --name flag needed.\n" +
			"Writes only 7 files: forge.config.yml, .forge/hygiene.yml, .forge/conventions.json,\n" +
			".forge/manifest, AGENTS.md, .forge/knowledge-index.json, .forge/instructions/global.instructions.md\n\n" +
			"forge init always injects forge-managed baseline files (safe — user content is never overwritten):\n" +
			"  .gitignore              — inserts/refreshes # forge:gitignore:start … # forge:gitignore:end\n" +
			"  .gitleaks.toml          — creates with forge baseline rules (if absent)\n" +
			"  .pre-commit-config.yaml — creates with baseline hygiene hooks (if absent)\n" +
			"  .github/dependabot.yml  — creates with auto-detected ecosystems (if absent)\n" +
			"Use --force to also overwrite managed blocks that have drifted.\n\n" +
			"Template auto-detection order (without --minimal):\n" +
			"  1. --template flag\n" +
			"  2. package.json detected → ts-service\n" +
			"  3. go.mod detected       → go-service\n" +
			"  4. default               → ts-service\n\n" +
			"Available templates: " + strings.Join(scaffold.AvailableTemplates(), ", "),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var target string
			if len(args) == 1 && args[0] != "" {
				abs, err := filepath.Abs(args[0])
				if err != nil {
					return errcode.New(ErrInitFailed, "cannot resolve path", err)
				}
				target = abs
			} else {
				cwd, err := os.Getwd()
				if err != nil {
					return errcode.New(ErrInitFailed, "cannot determine current directory", err)
				}
				target = cwd
			}

			// Determine project name.
			if name == "" {
				name = filepath.Base(target)
			}

			// --minimal: wire forge into an existing project with minimal footprint.
			if minimal {
				return runMinimal(cmd, target, name, forgeVersion, asJSON, force)
			}

			// Auto-detect template if not explicitly provided.
			tmpl := tmplFlag
			if tmpl == "" {
				tmpl = detectTemplate(target)
			}

			// Validate the template exists.
			available := scaffold.AvailableTemplates()
			if !contains(available, tmpl) {
				return errcode.New(ErrInitUsage,
					fmt.Sprintf("unknown template %q; available: %s", tmpl, strings.Join(available, ", ")),
					nil)
			}

			res, err := scaffold.Render(scaffold.Options{
				Template: tmpl,
				Target:   target,
				Vars: scaffold.Vars{
					Name:     name,
					ForgeVer: forgeVersion,
				},
				Force: force,
			})
			if err != nil {
				return err
			}

			// Always inject forge marker blocks and baseline files (safe: user content preserved).
			var mergeWarnings []string
			{
				merged, warnings, mergeErr := runMergeCodemods(target, force)
				if mergeErr != nil {
					return mergeErr
				}
				res.Files = append(res.Files, merged...)
				sort.Strings(res.Files)
				mergeWarnings = warnings
			}

			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(res)
			}

			fmt.Fprintf(cmd.OutOrStdout(),
				"initialised %q with template %q (%d files)\n", res.Target, res.Template, len(res.Files))
			for _, f := range res.Files {
				fmt.Fprintf(cmd.OutOrStdout(), "  + %s\n", f)
			}
			for _, w := range mergeWarnings {
				fmt.Fprintln(cmd.OutOrStdout(), w)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "\nNext steps:")
			switch tmpl {
			case "ts-service":
				fmt.Fprintln(cmd.OutOrStdout(), "  npm install")
				fmt.Fprintln(cmd.OutOrStdout(), "  npm run dev")
			case "go-service":
				fmt.Fprintln(cmd.OutOrStdout(), "  go mod tidy")
				fmt.Fprintln(cmd.OutOrStdout(), "  go test ./...")
			default:
				fmt.Fprintln(cmd.OutOrStdout(), "  forge doctor")
			}
			// Nudge: auto-pairing with AI tools.
			fmt.Fprintln(cmd.OutOrStdout(), "\n→ Pair AI with this project (VS Code, Claude, Cursor, Windsurf):")
			fmt.Fprintln(cmd.OutOrStdout(), "    forge companion")
			return nil
		},
	}

	cmd.Flags().StringVarP(&tmplFlag, "template", "t", "", "template to apply (default: auto-detect)")
	cmd.Flags().StringVarP(&name, "name", "n", "", "project name (default: current directory name \u2014 no flag needed when running from the project root)")
	cmd.Flags().BoolVarP(&minimal, "minimal", "m", false, "inject only forge runtime files + knowledge index (safe for existing projects; name auto-detected from current directory)")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "overwrite files that already exist; also overwrites drifted forge-managed blocks (.gitignore, .gitleaks.toml)")
	cmd.Flags().BoolVarP(&asJSON, "json", "j", false, "emit machine-readable JSON")
	return cmd
}

// runMinimal runs `forge init --minimal`: renders the minimal template then
// injects the knowledge index, global LLM instructions, and forge baseline
// files (.gitignore block, .gitleaks.toml, .pre-commit-config.yaml,
// .github/dependabot.yml) into the project.
// --force is implied for the scaffold (existing projects are always non-empty)
// and is forwarded to codemods for overwriting drifted managed blocks.
func runMinimal(cmd *cobra.Command, target, name, forgeVersion string, asJSON, force bool) error {
	res, err := scaffold.Render(scaffold.Options{
		Template: "minimal",
		Target:   target,
		Vars:     scaffold.Vars{Name: name, ForgeVer: forgeVersion},
		Force:    true, // existing projects are always non-empty
	})
	if err != nil {
		return err
	}

	// Inject knowledge index and LLM instructions.
	extra, err := injectKnowledge(target)
	if err != nil {
		return err
	}
	res.Files = append(res.Files, extra...)

	// Always inject forge marker blocks and baseline files (safe: user content preserved).
	var mergeWarnings []string
	{
		merged, warnings, mergeErr := runMergeCodemods(target, force)
		if mergeErr != nil {
			return mergeErr
		}
		res.Files = append(res.Files, merged...)
		mergeWarnings = warnings
	}

	sort.Strings(res.Files)

	if asJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(res)
	}

	fmt.Fprintf(cmd.OutOrStdout(),
		"wired forge into %q (%d files)\n", res.Target, len(res.Files))
	for _, f := range res.Files {
		fmt.Fprintf(cmd.OutOrStdout(), "  + %s\n", f)
	}
	for _, w := range mergeWarnings {
		fmt.Fprintln(cmd.OutOrStdout(), w)
	}
	fmt.Fprintln(cmd.OutOrStdout(), "\nNext steps:")
	fmt.Fprintln(cmd.OutOrStdout(), "  forge doctor              # validate forge.config.yml")
	fmt.Fprintln(cmd.OutOrStdout(), "  forge ship \"<feature>\"   # start the 6-checkpoint pipeline")
	fmt.Fprintln(cmd.OutOrStdout(), "  forge lint                # check conventions")
	// Nudge: auto-pairing with AI tools.
	fmt.Fprintln(cmd.OutOrStdout(), "\n→ Pair AI with this project (VS Code, Claude, Cursor, Windsurf):")
	fmt.Fprintln(cmd.OutOrStdout(), "    forge companion")
	return nil
}

// runMergeCodemods applies codemods to inject forge marker blocks and baseline
// files into an existing project. Codemods are applied in a fixed order.
// Files already containing correct forge blocks are unchanged (idempotent).
// Returns the list of touched files and any non-fatal drift warnings.
func runMergeCodemods(target string, force bool) ([]string, []string, error) {
	// Codemods applied in merge mode, in declaration order:
	//   gitignore-marker  — appends/refreshes forge block in .gitignore (always safe)
	//   gitleaks-baseline — creates .gitleaks.toml if absent
	//   dependabot-baseline — creates .github/dependabot.yml if absent
	//   pre-commit-baseline — creates .pre-commit-config.yaml if absent
	codemodNames := []string{
		"gitignore-marker",
		"gitleaks-baseline",
		"dependabot-baseline",
		"pre-commit-baseline",
	}

	reg := codemod.Default()
	var mergedFiles, warnings []string

	for _, cmName := range codemodNames {
		cm, ok := reg.Lookup(cmName)
		if !ok {
			continue
		}

		var rep codemod.Report
		var err error

		if force {
			if fc, ok := cm.(codemod.ForcedCodemod); ok {
				rep, err = fc.ApplyForce(target, false)
			} else {
				rep, err = cm.Apply(target, false)
			}
		} else {
			rep, err = cm.Apply(target, false)
		}

		if errors.Is(err, codemod.ErrManagedBlockDrift) {
			warnings = append(warnings,
				fmt.Sprintf("  ~ %s: managed block has drifted — rerun with --force to overwrite", cmName))
			continue
		}
		if err != nil {
			return mergedFiles, warnings, fmt.Errorf("codemod %s: %w", cmName, err)
		}
		if rep.Changed > 0 {
			mergedFiles = append(mergedFiles, rep.Files...)
		}
	}

	return mergedFiles, warnings, nil
}

// injectKnowledge writes .forge/knowledge-index.json (dumped from the embedded
// knowledge index) and .forge/instructions/global.instructions.md into target.
func injectKnowledge(target string) ([]string, error) {
	forgeDir := filepath.Join(target, ".forge")
	instrDir := filepath.Join(forgeDir, "instructions")
	if err := os.MkdirAll(instrDir, 0o755); err != nil { //nolint:gosec
		return nil, errcode.New(ErrKnowledgeWrite, "mkdir .forge/instructions", err)
	}

	// Load the embedded knowledge index (degrades to empty index in public builds).
	idx, _ := knowledge.Load() //nolint:errcheck // empty index is valid
	if idx == nil {
		idx = &knowledge.Index{Version: "1"}
	}

	// 1. Write .forge/knowledge-index.json
	idxData, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return nil, errcode.New(ErrKnowledgeWrite, "marshal knowledge index", err)
	}
	kPath := filepath.Join(forgeDir, "knowledge-index.json")
	if err := os.WriteFile(kPath, idxData, 0o600); err != nil {
		return nil, errcode.New(ErrKnowledgeWrite, "write knowledge-index.json", err)
	}

	// 2. Write .forge/instructions/global.instructions.md
	instrPath := filepath.Join(instrDir, "global.instructions.md")
	if err := os.WriteFile(instrPath, []byte(buildGlobalInstructions(idx)), 0o600); err != nil {
		return nil, errcode.New(ErrKnowledgeWrite, "write global.instructions.md", err)
	}

	return []string{
		".forge/knowledge-index.json",
		".forge/instructions/global.instructions.md",
	}, nil
}

// buildGlobalInstructions renders the global LLM instructions file content,
// embedding a summary of knowledge categories from the index.
func buildGlobalInstructions(idx *knowledge.Index) string {
	var sb strings.Builder
	sb.WriteString("# Global LLM Instructions\n")
	sb.WriteString("# Wired by `forge init --minimal`. Consumed by GitHub Copilot, Cursor, Claude, Windsurf.\n")
	sb.WriteString("# Run `forge context generate` to refresh the knowledge index.\n\n")

	sb.WriteString("## Forge Ship Workflow\n\n")
	sb.WriteString("`forge ship \"<feature>\"` runs 6 checkpoints: spec → arch → test → breakdown → code → ship.\n\n")
	sb.WriteString("| Checkpoint | What happens |\n")
	sb.WriteString("|---|---|\n")
	sb.WriteString("| spec | Generate/validate feature spec in `.forge/specs/<slug>/spec.md` |\n")
	sb.WriteString("| arch | Multi-role architecture debate → ADR |\n")
	sb.WriteString("| test | Generate failing TDD test stubs (tests before code) |\n")
	sb.WriteString("| breakdown | Decompose spec into atomic tasks |\n")
	sb.WriteString("| code | Generate/iterate code until tests pass |\n")
	sb.WriteString("| ship | Hygiene + scan + lint readiness check |\n\n")

	sb.WriteString("## Mandatory Rules\n\n")
	sb.WriteString("1. Write failing tests BEFORE production code (`forge ship` enforces this).\n")
	sb.WriteString("2. Run `forge scan security` before every PR.\n")
	sb.WriteString("3. Run `forge lint` before committing.\n")
	sb.WriteString("4. Never hardcode secrets — use environment variables.\n")
	sb.WriteString("5. SQL queries use parameterised placeholders only, never string concatenation.\n\n")

	// Emit knowledge category summary when the index is non-empty.
	if idx != nil && len(idx.Entries) > 0 {
		sb.WriteString("## Injected Knowledge Categories\n\n")
		sb.WriteString("Full index: `.forge/knowledge-index.json` — top-5 entries are auto-selected\n")
		sb.WriteString("by `forge ship` for every LLM call based on checkpoint + domain tags.\n\n")

		// Group entry IDs by category.
		catMap := map[string][]string{}
		for _, e := range idx.Entries {
			catMap[e.Category] = append(catMap[e.Category], e.ID)
		}
		cats := make([]string, 0, len(catMap))
		for c := range catMap {
			cats = append(cats, c)
		}
		sort.Strings(cats)
		for _, cat := range cats {
			ids := catMap[cat]
			sort.Strings(ids)
			fmt.Fprintf(&sb, "- **%s**: %s\n", cat, strings.Join(ids, ", "))
		}
		sb.WriteByte('\n')
	} else {
		sb.WriteString("## Injected Knowledge\n\n")
		sb.WriteString("Knowledge index is empty (public build). Run `forge context generate` to populate.\n\n")
	}

	return sb.String()
}

// detectTemplate infers the best-fit template from files already in dir.
func detectTemplate(dir string) string {
	if fileExists(filepath.Join(dir, "package.json")) {
		return "ts-service"
	}
	if fileExists(filepath.Join(dir, "go.mod")) {
		return "go-service"
	}
	return "ts-service"
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func contains(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}
