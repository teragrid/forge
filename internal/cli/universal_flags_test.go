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

// Package cli — G-080 universal flags audit.
//
// Every top-level verb registered on the root command must expose the nine
// universal flags listed in docs/VERBS.md §Universal Flags. Persistent flags
// on the root command are inherited by all children (pflag AddFlagSet skips
// duplicates so child-local re-declarations shadow the inherited version
// without conflict). This test therefore covers both cases.
package cli

import (
	"testing"

	"github.com/spf13/cobra"
)

// universalFlags lists the nine flag names that every forge verb must expose
// either via its own local declaration or via root PersistentFlags inheritance.
var universalFlags = []string{
	"json",
	"yes",
	"dry-run",
	"explain",
	"workspace",
	"no-color",
	"quiet",
	"verbose",
	"profile",
}

// TestUniversalFlagsOnAllVerbs iterates every direct child of the root command
// (i.e. every top-level verb) and asserts that each universal flag is reachable
// via cmd.Flags().Lookup, which searches both local and inherited flag sets.
func TestUniversalFlagsOnAllVerbs(t *testing.T) {
	root := NewRootCommand("test")

	for _, verb := range root.Commands() {
		verb := verb // capture
		t.Run(verb.Name(), func(t *testing.T) {
			// Trigger Cobra's lazy persistent-flag merge so that
			// inherited flags are visible via verb.Flags().Lookup.
			_ = verb.InheritedFlags()

			for _, flag := range universalFlags {
				if verb.Flags().Lookup(flag) == nil {
					t.Errorf("verb %q is missing universal flag --%s", verb.Name(), flag)
				}
			}
		})
	}
}

// TestUniversalFlagsExistOnRoot is a fast sanity check that the root command's
// PersistentFlags contain all nine universal flags before any merging occurs.
func TestUniversalFlagsExistOnRoot(t *testing.T) {
	root := NewRootCommand("test")
	for _, flag := range universalFlags {
		if root.PersistentFlags().Lookup(flag) == nil {
			t.Errorf("root PersistentFlags missing --%s; add it in NewRootCommand", flag)
		}
	}
}

// TestUniversalFlagTypes verifies the Go types of the universal flags on root
// so that the schema stays stable across refactors.
func TestUniversalFlagTypes(t *testing.T) {
	root := NewRootCommand("test")
	pf := root.PersistentFlags()

	boolFlags := []string{"json", "yes", "dry-run", "explain", "no-color", "quiet"}
	for _, name := range boolFlags {
		f := pf.Lookup(name)
		if f == nil {
			t.Errorf("root PersistentFlags missing --%s", name)
			continue
		}
		if f.Value.Type() != "bool" {
			t.Errorf("--%s: want type bool, got %s", name, f.Value.Type())
		}
	}

	stringFlags := []string{"workspace", "verbose", "profile"}
	for _, name := range stringFlags {
		f := pf.Lookup(name)
		if f == nil {
			t.Errorf("root PersistentFlags missing --%s", name)
			continue
		}
		if got := f.Value.Type(); got != "string" {
			t.Errorf("--%s: want type string, got %s", name, got)
		}
	}
}

// TestUniversalFlagShorthandConventions checks that the two flags with
// standard UNIX shorthands expose them correctly.
func TestUniversalFlagShorthandConventions(t *testing.T) {
	root := NewRootCommand("test")
	pf := root.PersistentFlags()

	cases := []struct{ flag, shorthand string }{
		{"yes", "y"},
		{"quiet", "q"},
	}
	for _, tc := range cases {
		f := pf.Lookup(tc.flag)
		if f == nil {
			t.Errorf("root PersistentFlags missing --%s", tc.flag)
			continue
		}
		// Cobra stores the shorthand string in the flag; cobra wraps pflag
		// so we look it up via ShorthandLookup.
		if pf.ShorthandLookup(tc.shorthand) == nil {
			t.Errorf("--%s shorthand -%s not registered", tc.flag, tc.shorthand)
		}
	}
}

// verifyVerbExposesFlag is a helper used by table-driven checks.
func verifyVerbExposesFlag(t *testing.T, root *cobra.Command, verbName, flagName string) {
	t.Helper()
	cmd, _, err := root.Find([]string{verbName})
	if err != nil || cmd == nil || cmd == root {
		t.Skipf("verb %q not found in command tree", verbName)
		return
	}
	_ = cmd.InheritedFlags()
	if cmd.Flags().Lookup(flagName) == nil {
		t.Errorf("verb %q missing flag --%s", verbName, flagName)
	}
}

// TestKeyVerbsHaveJSONFlag spot-checks a representative set of verbs for --json.
func TestKeyVerbsHaveJSONFlag(t *testing.T) {
	root := NewRootCommand("test")
	keyVerbs := []string{
		"scan", "ship", "fix", "audit", "eval", "learn",
		"deploy", "undo", "agents", "version",
	}
	for _, v := range keyVerbs {
		v := v
		t.Run(v, func(t *testing.T) {
			verifyVerbExposesFlag(t, root, v, "json")
		})
	}
}
