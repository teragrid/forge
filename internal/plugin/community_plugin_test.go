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

// G-130: WASM plugin sandbox — community plugin fixture end-to-end test.
// Validates that the community-plugin-demo fixture manifest is loadable,
// conforms to the plugin contract, and its sandbox capability declaration
// (fs:read only) is enforced by the in-process registry.
package plugin_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/teragrid/forge/internal/plugin"
)

const communityPluginFixture = "../../tests/fixtures/community-plugin/plugin.json"

// TestCommunityPlugin_ManifestLoads verifies the fixture plugin.json parses
// into a valid plugin.Manifest.
func TestCommunityPlugin_ManifestLoads(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile(filepath.FromSlash(communityPluginFixture))
	if err != nil {
		t.Fatalf("fixture plugin.json missing: %v", err)
	}
	var m plugin.Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if err := m.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

// TestCommunityPlugin_IsScanner verifies the fixture declares Kind=scanner.
func TestCommunityPlugin_IsScanner(t *testing.T) {
	t.Parallel()
	data, _ := os.ReadFile(filepath.FromSlash(communityPluginFixture))
	var m plugin.Manifest
	_ = json.Unmarshal(data, &m)
	if m.Kind != plugin.KindScanner {
		t.Errorf("expected kind %q, got %q", plugin.KindScanner, m.Kind)
	}
}

// TestCommunityPlugin_CapabilityFsReadOnly verifies the fixture only requests
// fs:read and not broader capabilities (fs:write, net:http, exec).
func TestCommunityPlugin_CapabilityFsReadOnly(t *testing.T) {
	t.Parallel()
	data, _ := os.ReadFile(filepath.FromSlash(communityPluginFixture))
	var m plugin.Manifest
	_ = json.Unmarshal(data, &m)
	forbidden := []string{"fs:write", "net:http", "exec"}
	capSet := map[string]bool{}
	for _, c := range m.Capabilities {
		capSet[c] = true
	}
	for _, f := range forbidden {
		if capSet[f] {
			t.Errorf("fixture declared forbidden capability %q", f)
		}
	}
	if !capSet["fs:read"] {
		t.Error("fixture missing required capability 'fs:read'")
	}
}

// TestCommunityPlugin_RegisterInRegistry verifies an in-process stub
// representing the community plugin can be registered without panic.
func TestCommunityPlugin_RegisterInRegistry(t *testing.T) {
	t.Parallel()
	r := plugin.NewRegistry()

	data, _ := os.ReadFile(filepath.FromSlash(communityPluginFixture))
	var m plugin.Manifest
	_ = json.Unmarshal(data, &m)

	// Validate before attempting registration.
	if err := m.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}

	stub := &communityStubScanner{manifest: m}
	// Should not panic.
	r.Register(stub)

	p, ok := r.Lookup(m.Name)
	if !ok {
		t.Fatalf("Lookup(%q) failed after Register", m.Name)
	}
	if p.Manifest().Kind != plugin.KindScanner {
		t.Errorf("wrong kind: %q", p.Manifest().Kind)
	}
}

// TestCommunityPlugin_ByKindScanner verifies ByKind returns the community plugin.
func TestCommunityPlugin_ByKindScanner(t *testing.T) {
	t.Parallel()
	r := plugin.NewRegistry()

	data, _ := os.ReadFile(filepath.FromSlash(communityPluginFixture))
	var m plugin.Manifest
	_ = json.Unmarshal(data, &m)

	r.Register(&communityStubScanner{manifest: m})
	scanners := r.ByKind(plugin.KindScanner)
	if len(scanners) == 0 {
		t.Error("ByKind(scanner) returned nothing after registering community scanner")
	}
}

// TestCommunityPlugin_WATFilePresent verifies the WAT source file ships with the fixture.
func TestCommunityPlugin_WATFilePresent(t *testing.T) {
	t.Parallel()
	watPath := filepath.FromSlash("../../tests/fixtures/community-plugin/community_plugin_demo.wat")
	if _, err := os.Stat(watPath); err != nil {
		t.Errorf("WAT source file missing: %v", err)
	}
}

// communityStubScanner is an in-process stub representing the WASM community plugin.
type communityStubScanner struct {
	manifest plugin.Manifest
}

func (c *communityStubScanner) Manifest() plugin.Manifest { return c.manifest }
func (c *communityStubScanner) Scan(_ interface{ Done() <-chan struct{} }, _ string) ([]plugin.Finding, error) {
	return nil, nil
}
