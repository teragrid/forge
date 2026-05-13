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

// Package cmdadopt implements `forge adopt` (DEV-M1-43, spec Â§4).
//
// Adopts an existing project into the Forge framework by scaffolding missing
// framework files without overwriting existing ones. Safe by default
// (--dry-run previews without writing; --apply commits changes).
//
// Detection heuristics:
//
//	go.mod          â†’ Go project
//	package.json    â†’ Node.js project
//	pyproject.toml  â†’ Python project
//	Cargo.toml      â†’ Rust project
//	(none matched)  â†’ generic
package cmdadopt

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/teragrid/forge/internal/errcode"
	"github.com/teragrid/forge/internal/verbmeta"
)

// Reserved error codes (range 4500..4599).
var (
	ErrAdoptFailed = errcode.Register(errcode.Code(4500), "adopt failed")
)

// FileAction describes one file to be created during adoption.
type FileAction struct {
	Path    string `json:"path"`              // relative to project root
	Status  string `json:"status"`            // "create", "exists" (skipped), "skipped" (--skip)
	Preview string `json:"preview,omitempty"` // first 80 chars of content
}

// AdoptResult summarises the adopt run.
type AdoptResult struct {
	Root        string       `json:"root"`
	ProjectType string       `json:"project_type"`
	Mode        string       `json:"mode"` // "dry-run" or "apply"
	Actions     []FileAction `json:"actions"`
	Created     int          `json:"created"`
	Existing    int          `json:"existing"`
	Skipped     int          `json:"skipped"`
}

func init() {
	verbmeta.Register(verbmeta.Manifest{
		Verb:    "adopt",
		Summary: "Adopt an existing project into the Forge framework (DEV-M1-43, spec Â§4).",
		Inputs: []string{
			"--root <path>     â€” project root (default: cwd)",
			"--dry-run         â€” preview files without writing (default)",
			"--apply           â€” write framework files",
			"--skip <file>     â€” skip a specific file (repeatable)",
			"--json            â€” machine-readable output",
		},
		Outputs:      []string{"stdout: list of files to be / that were created"},
		SideEffects:  []string{"with --apply: writes framework files; NEVER overwrites existing"},
		GatesTouched: []string{"Â§4 adopt", "Â§11.1.2 Developer Promises #1â€“#6"},
		ErrorCodes:   []errcode.Code{ErrAdoptFailed},
	})
}

// New returns the cobra command for `forge adopt`.
func New() *cobra.Command {
	var (
		root   string
		apply  bool
		skip   []string
		asJSON bool
	)
	cmd := &cobra.Command{
		Use:   "adopt [--root <path>]",
		Short: "Adopt an existing project into the Forge framework.",
		Long: "forge adopt brings an existing project under Forge management by scaffolding\n" +
			"missing framework files without touching existing ones.\n\n" +
			"Framework files scaffolded:\n" +
			"  .forge/manifest         â€” hygiene / managed file registry\n" +
			"  .forge/hygiene.yml      â€” scratch/managed pattern manifest\n" +
			"  .forge/conventions.json â€” machine-readable convention registry\n" +
			"  AGENTS.md               â€” multi-AI agent context file\n" +
			"  CLAUDE.md               â€” Claude AI context file\n" +
			"  .cursorrules            â€” Cursor IDE rules\n" +
			"  .windsurfrules          â€” Windsurf IDE rules\n\n" +
			"Safe by default: --dry-run previews. Use --apply to write.\n" +
			"Existing files are NEVER overwritten.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if root == "" {
				cwd, err := os.Getwd()
				if err != nil {
					return errcode.New(ErrAdoptFailed, "getwd", err)
				}
				root = cwd
			}
			mode := "dry-run"
			if apply {
				mode = "apply"
			}
			result, err := Run(root, mode, skip)
			if err != nil {
				return errcode.New(ErrAdoptFailed, "run", err)
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
	cmd.Flags().BoolVar(&apply, "apply", false, "write framework files")
	cmd.Flags().StringArrayVar(&skip, "skip", nil, "skip a framework file (repeatable)")
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit JSON")
	return cmd
}

// Run performs the adopt analysis (and optionally writes files).
func Run(root, mode string, skipList []string) (AdoptResult, error) {
	result := AdoptResult{
		Root:        root,
		ProjectType: detectProjectType(root),
		Mode:        mode,
	}

	skipSet := make(map[string]bool, len(skipList))
	for _, s := range skipList {
		skipSet[filepath.Clean(s)] = true
	}

	templates := buildTemplates(root, result.ProjectType)

	for _, t := range templates {
		rel := filepath.Clean(t.path)
		action := FileAction{Path: rel}

		if skipSet[rel] {
			action.Status = "skipped"
			result.Skipped++
			result.Actions = append(result.Actions, action)
			continue
		}

		fullPath := filepath.Join(root, rel)
		if _, err := os.Stat(fullPath); err == nil {
			action.Status = "exists"
			result.Existing++
			result.Actions = append(result.Actions, action)
			continue
		}

		content := t.content
		preview := content
		if len(preview) > 80 {
			preview = preview[:80] + "..."
		}
		action.Status = "create"
		action.Preview = preview
		result.Created++

		if mode == "apply" {
			if err := os.MkdirAll(filepath.Dir(fullPath), 0o750); err != nil {
				return result, fmt.Errorf("mkdir %s: %w", filepath.Dir(fullPath), err)
			}
			if err := os.WriteFile(fullPath, []byte(content), 0o600); err != nil {
				return result, fmt.Errorf("write %s: %w", rel, err)
			}
		}
		result.Actions = append(result.Actions, action)
	}
	return result, nil
}

type templateEntry struct {
	path    string
	content string
}

func buildTemplates(root, projectType string) []templateEntry {
	// Detect module name from go.mod.
	moduleName := "myproject"
	if data, err := os.ReadFile(filepath.Join(root, "go.mod")); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "module ") {
				moduleName = strings.TrimSpace(strings.TrimPrefix(line, "module "))
				break
			}
		}
	}

	ts := time.Now().UTC().Format(time.RFC3339)

	return []templateEntry{
		{
			path: filepath.Join(".forge", "manifest"),
			content: fmt.Sprintf(`{"module":"%s","created_at":"%s","managed_dirs":[".forge","internal","cmd"],"scratch_patterns":["*.tmp","*.bak","dist/","build/"]}`+"\n",
				moduleName, ts),
		},
		{
			path:    filepath.Join(".forge", "hygiene.yml"),
			content: fmt.Sprintf("# Forge hygiene manifest\n# Created: %s\n# Project: %s\n\nscope:\n  module: %s\n  type: %s\n\nmanaged:\n  - .forge/**\n  - internal/**\n  - cmd/**\n\nscratch_patterns:\n  - \"*.tmp\"\n  - \"*.bak\"\n  - dist/\n  - build/\n", ts, moduleName, moduleName, projectType),
		},
		{
			path: filepath.Join(".forge", "conventions.json"),
			content: fmt.Sprintf(`{"version":"1","module":"%s","project_type":"%s","created_at":"%s","conventions":{"errcode_range_per_verb":true,"verbmeta_required":true,"tests_before_ship":true,"audit_log_path":".forge/audit.log"}}`+"\n",
				moduleName, projectType, ts),
		},
		{
			path: "AGENTS.md",
			content: fmt.Sprintf("# Agents Context â€” %s\n\n## Project\n\nModule: `%s`  \nType: %s  \nCreated: %s\n\n## Rules\n\n1. Tests must precede code in every PR.\n2. Never commit secrets. Run `forge scan security --secrets`.\n3. Every error must use `errcode.Register`.\n4. Run `forge ship` before merging.\n5. Waivers go in `.forge/waivers/`.\n",
				moduleName, moduleName, projectType, ts),
		},
		{
			path: "CLAUDE.md",
			content: fmt.Sprintf("# Claude Context â€” %s\n\n> Generated by `forge adopt` on %s\n\n## Project\n\n- Module: `%s`\n- Type: `%s`\n\n## Forge Rules\n\n- Use `errcode.Register` for all errors\n- Use `verbmeta.Register` in every `cmd*` package\n- Run `forge lint` before shipping\n- Never commit secrets\n",
				moduleName, ts, moduleName, projectType),
		},
		{
			path: ".cursorrules",
			content: fmt.Sprintf("# Cursor Rules â€” %s\n# Generated: %s\n\n- Always run `forge lint` before committing\n- Use errcode.Register for error codes\n- Add verbmeta.Register in every cmd* package\n- Do not commit secrets or credentials\n- Tests must precede implementation in all PRs\n",
				moduleName, ts),
		},
		{
			path: ".windsurfrules",
			content: fmt.Sprintf("# Windsurf Rules â€” %s\n# Generated: %s\n\n- Always run `forge lint` before committing\n- Use errcode.Register for error codes\n- Add verbmeta.Register in every cmd* package\n- Do not commit secrets or credentials\n- Tests must precede implementation in all PRs\n",
				moduleName, ts),
		},
	}
}

func detectProjectType(root string) string {
	checks := []struct {
		file string
		kind string
	}{
		{"go.mod", "go"},
		{"package.json", "nodejs"},
		{"pyproject.toml", "python"},
		{"setup.py", "python"},
		{"Cargo.toml", "rust"},
		{"pom.xml", "java"},
	}
	for _, c := range checks {
		if _, err := os.Stat(filepath.Join(root, c.file)); err == nil {
			return c.kind
		}
	}
	return "generic"
}

func renderText(cmd *cobra.Command, r AdoptResult) {
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "forge adopt (%s): %s [%s]\n\n", r.Mode, r.Root, r.ProjectType)
	for _, a := range r.Actions {
		icon := "+"
		switch a.Status {
		case "exists":
			icon = "~"
		case "skipped":
			icon = "-"
		}
		fmt.Fprintf(w, "  %s %s (%s)\n", icon, a.Path, a.Status)
	}
	fmt.Fprintf(w, "\ncreate: %d  existing: %d  skipped: %d\n", r.Created, r.Existing, r.Skipped)
	if r.Mode == "dry-run" && r.Created > 0 {
		fmt.Fprintln(w, "(use --apply to write the framework files)")
	}
}
