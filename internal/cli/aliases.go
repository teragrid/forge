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

// Package cli - aliases.go registers deprecated verb aliases (M2-16).
//
// When a verb is renamed the old name must remain functional for at least one
// major version. Each entry in aliases maps the OLD name to the NEW cobra
// command, wrapped in a thin cobra.Command that:
//   - prints a deprecation notice to stderr on every invocation
//   - forwards all args and flags to the canonical command
//
// Usage inside NewRootCommand:
//
//	AddAliases(root, myVerb.New())
//
// ADR-024 (Reversibility Contract) requires aliases stay until v2.0.0.
package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// aliasEntry describes one deprecated verb alias.
type aliasEntry struct {
	// OldName is the deprecated command name users may still type.
	OldName string
	// NewName is the canonical verb that should be used instead.
	NewName string
	// RemovedIn is the version in which the alias will be removed.
	RemovedIn string
}

// knownAliases is the list of all active backward-compat aliases.
// Add a new row here whenever a verb is renamed.
var knownAliases = []aliasEntry{
	// forge check was previously called forge lint in early prototypes.
	{OldName: "lint-check", NewName: "check", RemovedIn: "v2.0.0"},
	// forge scan was previously called forge audit-scan in early prototypes.
	{OldName: "audit-scan", NewName: "scan", RemovedIn: "v2.0.0"},
	// forge generate was previously called forge gen in early prototypes.
	{OldName: "gen", NewName: "generate", RemovedIn: "v2.0.0"},
	// forge docs was previously called forge doc (singular) in early prototypes.
	{OldName: "doc", NewName: "docs", RemovedIn: "v2.0.0"},
	// forge optimize was previously called forge opt in early prototypes.
	{OldName: "opt", NewName: "optimize", RemovedIn: "v2.0.0"},
	// forge adopt was previously called forge bootstrap in early prototypes.
	{OldName: "bootstrap", NewName: "adopt", RemovedIn: "v2.0.0"},
	// G-090 rename map: old top-level verbs → new canonical verbs.
	// forge gdpr → forge audit  (was a separate GDPR-flavoured alias in early prototypes)
	{OldName: "gdpr", NewName: "audit", RemovedIn: "v2.0.0"},
	// forge compliance → forge audit  (compliance was an alias for the same ledger commands)
	{OldName: "compliance", NewName: "audit", RemovedIn: "v2.0.0"},
	// forge migrate-code → forge upgrade
	{OldName: "migrate-code", NewName: "upgrade", RemovedIn: "v2.0.0"},
	// forge generate ai-context → forge context generate
	// forge agents stop --workspace → forge agents stop  (flag change handled in cmdagents)
}

// registerAliases wires all knownAliases into root so that typing the old
// name still works but prints a deprecation notice.
//
// It must be called after all canonical subcommands are added to root.
func registerAliases(root *cobra.Command) {
	for _, a := range knownAliases {
		a := a // capture
		canonical, _, err := root.Find([]string{a.NewName})
		if err != nil || canonical == root {
			// Canonical command not found; skip alias rather than panic.
			continue
		}
		alias := &cobra.Command{
			Use:        a.OldName,
			Short:      fmt.Sprintf("[DEPRECATED] Use `forge %s` instead.", a.NewName),
			Deprecated: fmt.Sprintf("use `forge %s` instead. The `%s` alias will be removed in %s.", a.NewName, a.OldName, a.RemovedIn),
			Hidden:     true,
			// DisableFlagParsing lets the alias forward every flag untouched.
			DisableFlagParsing: true,
			RunE: func(cmd *cobra.Command, args []string) error {
				cmd.PrintErrf("forge: DEPRECATED: `%s` is an alias for `%s`. "+
					"Update your scripts. This alias will be removed in %s.\n",
					a.OldName, a.NewName, a.RemovedIn)
				return canonical.RunE(canonical, args)
			},
		}
		root.AddCommand(alias)
	}
}
