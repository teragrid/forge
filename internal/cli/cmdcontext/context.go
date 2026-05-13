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

// Package cmdcontext implements `forge context` (DEV-M1-46, spec Â§4).
//
// Sub-commands:
//
//	generate â€” write .forge/context/snapshot.md from live project files
//	show     â€” print snapshot.md contents
//	budget   â€” estimate token counts for all AI context files
package cmdcontext

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

// Reserved error codes (range 4700..4799).
var (
	ErrContextFailed = errcode.Register(errcode.Code(4700), "context operation failed")
)

// BudgetEntry reports one context file's token estimate.
type BudgetEntry struct {
	File      string `json:"file"`
	Bytes     int    `json:"bytes"`
	EstTokens int    `json:"est_tokens"` // ~4 bytes/token heuristic
	Exists    bool   `json:"exists"`
}

// BudgetResult is the output of `forge context budget`.
type BudgetResult struct {
	Root        string        `json:"root"`
	Files       []BudgetEntry `json:"files"`
	TotalBytes  int           `json:"total_bytes"`
	TotalTokens int           `json:"total_tokens"`
}

func init() {
	verbmeta.Register(verbmeta.Manifest{
		Verb:    "context",
		Summary: "Manage LLM context files and token budgets (spec Â§4, DEV-M1-46).",
		Inputs: []string{
			"generate  â€” write .forge/context/snapshot.md",
			"show      â€” print the snapshot",
			"budget    â€” report token budget for all AI context files",
			"--root <path>",
			"--json    â€” machine-readable output",
		},
		Outputs:      []string{"stdout: context file contents or budget report"},
		SideEffects:  []string{"generate: writes .forge/context/snapshot.md"},
		GatesTouched: []string{"Â§4 context", "Â§5 llmbudget"},
		ErrorCodes:   []errcode.Code{ErrContextFailed},
	})
}

// New returns the cobra command for `forge context`.
func New() *cobra.Command {
	var root string
	var asJSON bool

	cmd := &cobra.Command{
		Use:   "context <generate|show|budget>",
		Short: "Manage LLM context files and token budgets.",
		Long: "forge context manages the AI agent context snapshot and token budgets.\n\n" +
			"Sub-commands:\n" +
			"  generate  â€” write .forge/context/snapshot.md from live project state\n" +
			"  show      â€” print the current snapshot contents\n" +
			"  budget    â€” estimate token usage across all AI context files",
	}

	genCmd := &cobra.Command{
		Use:   "generate",
		Short: "Regenerate .forge/context/snapshot.md from live project state.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			r, err := resolveRoot(root)
			if err != nil {
				return errcode.New(ErrContextFailed, "getwd", err)
			}
			path, err := GenerateSnapshot(r)
			if err != nil {
				return errcode.New(ErrContextFailed, "generate snapshot", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "forge context: snapshot written to %s\n", path)
			return nil
		},
	}
	genCmd.Flags().StringVar(&root, "root", "", "project root")

	showCmd := &cobra.Command{
		Use:   "show",
		Short: "Print the current .forge/context/snapshot.md.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			r, err := resolveRoot(root)
			if err != nil {
				return errcode.New(ErrContextFailed, "getwd", err)
			}
			snapshotPath := filepath.Join(r, ".forge", "context", "snapshot.md")
			data, err := os.ReadFile(snapshotPath)
			if err != nil {
				return errcode.New(ErrContextFailed,
					"snapshot not found; run: forge context generate", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "forge context: %s\n", snapshotPath)
			fmt.Fprintln(cmd.OutOrStdout(), string(data))
			return nil
		},
	}
	showCmd.Flags().StringVar(&root, "root", "", "project root")

	budgetCmd := &cobra.Command{
		Use:   "budget",
		Short: "Report token budget across all AI context files.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			r, err := resolveRoot(root)
			if err != nil {
				return errcode.New(ErrContextFailed, "getwd", err)
			}
			result := Budget(r)
			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}
			renderBudget(cmd, result)
			return nil
		},
	}
	budgetCmd.Flags().StringVar(&root, "root", "", "project root")
	budgetCmd.Flags().BoolVar(&asJSON, "json", false, "emit JSON")

	cmd.AddCommand(genCmd, showCmd, budgetCmd)
	return cmd
}

// GenerateSnapshot writes .forge/context/snapshot.md from project context files.
// Returns the path of the written snapshot.
func GenerateSnapshot(root string) (string, error) {
	outDir := filepath.Join(root, ".forge", "context")
	if err := os.MkdirAll(outDir, 0o750); err != nil {
		return "", fmt.Errorf("mkdir %s: %w", outDir, err)
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "# Project Context Snapshot\n\nGenerated: %s\n\n", time.Now().UTC().Format(time.RFC3339))

	sections := []struct {
		path  string
		title string
	}{
		{"AGENTS.md", "Agents / LLM Instructions"},
		{"README.md", "README"},
		{"docs/ARCHITECTURE_OVERVIEW.md", "Architecture Overview"},
		{".forge/manifest", "Forge Manifest (patterns)"},
		{".forge/hygiene.yml", "Hygiene Config"},
		{".forge/conventions.json", "Conventions"},
		{"ROLLBACK.md", "Rollback Procedures"},
	}
	for _, s := range sections {
		data, err := os.ReadFile(filepath.Join(root, s.path))
		if err != nil {
			continue
		}
		content := strings.TrimSpace(string(data))
		if content == "" {
			continue
		}
		if len(content) > 4000 {
			content = content[:4000] + "\n\n[...truncated]"
		}
		fmt.Fprintf(&sb, "## %s\n\n%s\n\n---\n\n", s.title, content)
	}

	snapshotPath := filepath.Join(outDir, "snapshot.md")
	if err := os.WriteFile(snapshotPath, []byte(sb.String()), 0o600); err != nil {
		return "", fmt.Errorf("write snapshot: %w", err)
	}
	return snapshotPath, nil
}

// Budget estimates token usage for all AI context files.
func Budget(root string) BudgetResult {
	files := []string{
		"AGENTS.md",
		"CLAUDE.md",
		".cursorrules",
		".windsurfrules",
		".forge-conventions",
		".forge/context/snapshot.md",
		"docs/ARCHITECTURE_OVERVIEW.md",
	}
	result := BudgetResult{Root: root}
	for _, f := range files {
		path := filepath.Join(root, f)
		entry := BudgetEntry{File: f}
		data, err := os.ReadFile(path)
		if err == nil {
			entry.Exists = true
			entry.Bytes = len(data)
			entry.EstTokens = len(data) / 4
		}
		result.Files = append(result.Files, entry)
		result.TotalBytes += entry.Bytes
		result.TotalTokens += entry.EstTokens
	}
	return result
}

func renderBudget(cmd *cobra.Command, r BudgetResult) {
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "forge context budget: %s\n\n", r.Root)
	fmt.Fprintf(w, "  %-45s %8s  %8s\n", "File", "Bytes", "~Tokens")
	fmt.Fprintf(w, "  %-45s %8s  %8s\n", strings.Repeat("-", 45), "--------", "--------")
	for _, f := range r.Files {
		if !f.Exists {
			fmt.Fprintf(w, "  %-45s   (missing)\n", f.File)
			continue
		}
		fmt.Fprintf(w, "  %-45s %8d  %8d\n", f.File, f.Bytes, f.EstTokens)
	}
	fmt.Fprintf(w, "\n  Total: %d bytes, ~%d tokens\n", r.TotalBytes, r.TotalTokens)
	if r.TotalTokens > 128000 {
		fmt.Fprintln(w, "  âš  Warning: combined context exceeds 128k token budget for some models")
	}
}

func resolveRoot(root string) (string, error) {
	if root != "" {
		return root, nil
	}
	return os.Getwd()
}
