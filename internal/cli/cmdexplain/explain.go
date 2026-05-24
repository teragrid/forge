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
// Package cmdexplain implements `forge explain` (DEV-M0-12). With no arg it
// lists every registered verb grouped by category; with one arg it prints that
// verb's manifest with examples and a "what to try next" hint.
package cmdexplain

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/teragrid/forge/internal/errcode"
	"github.com/teragrid/forge/internal/verbmeta"
)

// Reserved error codes (range 1400..1499).
var (
	ErrUnknownVerb = errcode.Register(errcode.Code(1400), "unknown verb passed to `forge explain`")
)

func init() {
	verbmeta.Register(verbmeta.Manifest{
		Verb:    "explain",
		Summary: "Print the manifest for a verb (inputs, outputs, side-effects, error codes).",
		Inputs: []string{
			"<verb> (optional; if absent, list all verbs)",
			"--json (machine-readable output)",
		},
		Outputs:     []string{"stdout: per-verb manifest (text or JSON)"},
		SideEffects: []string{},
	})
}

// New returns the cobra command.
func New() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "explain [verb]",
		Short: "Show what a verb does (inputs, outputs, side-effects).",
		Long: strings.TrimSpace(`
forge explain prints the manifest for any verb: its inputs, outputs, side-effects,
quality gates it touches, and every error code it can produce.

With no argument it lists all registered verbs grouped by category so you can
discover what forge can do.

Examples:
  forge explain                 # list all verbs by category
  forge explain ship            # what does forge ship do?
  forge explain bugfix          # flags, side-effects, and error codes for forge bugfix
  forge explain --json bugfix   # machine-readable manifest

Tip: every command also accepts --help for its full flag reference:
  forge ship --help
  forge bugfix --help
  forge test spec --help
`),
		Example: `  forge explain                # list all verbs grouped by category
  forge explain ship           # manifest for forge ship
  forge explain bugfix         # manifest for forge bugfix
  forge explain --json scan    # machine-readable JSON manifest`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				all := verbmeta.All()
				if asJSON {
					enc := json.NewEncoder(cmd.OutOrStdout())
					enc.SetIndent("", "  ")
					return enc.Encode(all)
				}
				renderAllGrouped(cmd.OutOrStdout(), all)
				return nil
			}
			m, ok := verbmeta.Lookup(args[0])
			if !ok {
				known := []string{}
				for _, v := range verbmeta.All() {
					known = append(known, v.Verb)
				}
				return errcode.Newf(ErrUnknownVerb, nil,
					"unknown verb %q; known: %s", args[0], strings.Join(known, ", "))
			}
			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(m)
			}
			renderText(cmd, m)
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit machine-readable JSON")
	return cmd
}

// verbGroups maps category labels to the verb names in that category.
// Any verb not listed here falls under "Other".
var verbGroups = []struct {
	label string
	verbs []string
}{
	{"Getting started", []string{"version", "doctor", "new", "init", "tsd", "templates"}},
	{"Daily workflow", []string{"ship", "scan", "lint", "clean", "fix", "hygiene"}},
	{"Testing", []string{"test", "eval", "fixtures"}},
	{"Config & LLM", []string{"config", "spend", "context", "agents"}},
	{"Bug & incident response", []string{"bugfix", "incident", "postmortem", "ci"}},
	{"Knowledge & learning", []string{"learn", "explain", "ask", "docs", "insights"}},
	{"Audit & compliance", []string{"audit", "review", "report", "sla"}},
	{"Project evolution", []string{"upgrade", "migrate", "generate", "check", "optimize", "add", "adopt", "eject"}},
	{"Delivery & ops", []string{"deploy", "rollback", "backup", "undo", "plugin"}},
	{"Meta", []string{"telemetry"}},
}

// renderAllGrouped prints all verbs grouped by category.
func renderAllGrouped(w io.Writer, all []verbmeta.Manifest) {
	// Index for fast lookup.
	byVerb := make(map[string]verbmeta.Manifest, len(all))
	for _, m := range all {
		byVerb[m.Verb] = m
	}

	printed := make(map[string]bool)
	total := len(all)

	fmt.Fprintf(w, "Forge verbs (%d total)\n", total)
	fmt.Fprintf(w, "Run `forge explain <verb>` for full inputs, outputs, and error codes.\n")
	fmt.Fprintf(w, "Run `forge <verb> --help` for all flags.\n\n")

	for _, g := range verbGroups {
		var rows []verbmeta.Manifest
		for _, v := range g.verbs {
			if m, ok := byVerb[v]; ok {
				rows = append(rows, m)
				printed[v] = true
			}
		}
		if len(rows) == 0 {
			continue
		}
		fmt.Fprintf(w, "  %s\n", strings.ToUpper(g.label))
		for _, m := range rows {
			fmt.Fprintf(w, "    %-14s %s\n", m.Verb, m.Summary)
		}
		fmt.Fprintln(w)
	}

	// Anything not in a named group goes to "Other".
	var other []verbmeta.Manifest
	for _, m := range all {
		if !printed[m.Verb] {
			other = append(other, m)
		}
	}
	if len(other) > 0 {
		fmt.Fprintf(w, "  OTHER\n")
		for _, m := range other {
			fmt.Fprintf(w, "    %-14s %s\n", m.Verb, m.Summary)
		}
		fmt.Fprintln(w)
	}
}

func renderText(cmd *cobra.Command, m verbmeta.Manifest) {
	w := cmd.OutOrStdout()
	sep := strings.Repeat("─", 56)
	fmt.Fprintf(w, "%s\n", sep)
	fmt.Fprintf(w, "forge %s\n", m.Verb)
	fmt.Fprintf(w, "%s\n\n", sep)
	fmt.Fprintf(w, "  %s\n\n", m.Summary)
	bullets(w, "inputs", m.Inputs)
	bullets(w, "outputs", m.Outputs)
	bullets(w, "side-effects", m.SideEffects)
	if len(m.GatesTouched) > 0 {
		bullets(w, "gates touched", m.GatesTouched)
	}
	if len(m.ErrorCodes) > 0 {
		strs := make([]string, len(m.ErrorCodes))
		for i, c := range m.ErrorCodes {
			strs[i] = fmt.Sprintf("FORGE-%d — %s", c, errcode.Description(c))
		}
		bullets(w, "error codes", strs)
	}
	fmt.Fprintf(w, "\nNext steps:\n")
	fmt.Fprintf(w, "  forge %s --help          full flag reference\n", m.Verb)
	fmt.Fprintf(w, "  forge explain --json %s  machine-readable manifest\n", m.Verb)
}

func bullets(w io.Writer, label string, items []string) {
	if len(items) == 0 {
		return
	}
	fmt.Fprintf(w, "%s:\n", label)
	for _, it := range items {
		fmt.Fprintf(w, "  - %s\n", it)
	}
	fmt.Fprintln(w)
}
