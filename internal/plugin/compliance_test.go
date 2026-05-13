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

// compliance_test.go — plugin compliance test runner (M2-03).
//
// These tests enumerate every plugin registered in the Default() registry and
// verify that each one satisfies the Plugin contract:
//
//  1. Manifest.Validate() returns nil (name, version, kind all set).
//  2. The plugin implements at least one of Scanner, Codemod, or a known kind.
//  3. Kind is a recognised constant (KindScanner, KindCodemod, KindProvider,
//     KindTemplate).
//
// Plugins that fail any check are reported individually so CI shows the full
// list of failures rather than stopping at the first.

package plugin

import (
	"testing"
)

// TestComplianceManifestValid checks that every registered plugin has a
// non-empty Name, non-empty Version, and a recognised Kind.
func TestComplianceManifestValid(t *testing.T) {
	t.Helper()
	plugins := Default().All()
	if len(plugins) == 0 {
		t.Skip("no plugins registered — skipping compliance tests")
	}
	for _, p := range plugins {
		p := p
		m := p.Manifest()
		t.Run(m.Name, func(t *testing.T) {
			if err := m.Validate(); err != nil {
				t.Errorf("manifest.Validate() = %v; want nil", err)
			}
		})
	}
}

// TestComplianceKindKnown ensures every plugin's Kind is one of the defined
// constants so the plugin system can route it correctly.
func TestComplianceKindKnown(t *testing.T) {
	t.Helper()
	known := map[Kind]bool{
		KindScanner:  true,
		KindCodemod:  true,
		KindProvider: true,
		KindTemplate: true,
	}
	for _, p := range Default().All() {
		p := p
		m := p.Manifest()
		t.Run(m.Name, func(t *testing.T) {
			if !known[m.Kind] {
				t.Errorf("kind %q is not a recognised constant", m.Kind)
			}
		})
	}
}

// TestComplianceInterfaceImplemented checks that every plugin implements at
// least one of the specialised interfaces (Scanner, Codemod) if its Kind
// implies it should.
func TestComplianceInterfaceImplemented(t *testing.T) {
	t.Helper()
	for _, p := range Default().All() {
		p := p
		m := p.Manifest()
		t.Run(m.Name, func(t *testing.T) {
			switch m.Kind {
			case KindScanner:
				if _, ok := p.(Scanner); !ok {
					t.Errorf("plugin %q has Kind=scanner but does not implement Scanner interface", m.Name)
				}
			case KindCodemod:
				if _, ok := p.(Codemod); !ok {
					t.Errorf("plugin %q has Kind=codemod but does not implement Codemod interface", m.Name)
				}
			}
			// KindProvider and KindTemplate are currently only manifest-level;
			// no interface check required in v1.
		})
	}
}

// TestComplianceRegistryLookup verifies that every plugin can be looked up by
// name from the registry after registration.
func TestComplianceRegistryLookup(t *testing.T) {
	t.Helper()
	reg := Default()
	for _, p := range reg.All() {
		p := p
		name := p.Manifest().Name
		t.Run(name, func(t *testing.T) {
			got, ok := reg.Lookup(name)
			if !ok {
				t.Errorf("Lookup(%q) = _, false; want true", name)
				return
			}
			if got.Manifest().Name != name {
				t.Errorf("Lookup(%q).Manifest().Name = %q; want %q", name, got.Manifest().Name, name)
			}
		})
	}
}
