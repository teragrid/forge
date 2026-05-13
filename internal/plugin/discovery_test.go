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
package plugin

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// --- DiscoverFile tests ---

// TC-DISC-01 (happy + data-accuracy): valid file registers all entries.
func TestDiscoverFile_Happy(t *testing.T) {
	dir := t.TempDir()
	manifests := []Manifest{
		{Name: "ext-scanner-a", Version: "1.0.0", Kind: KindScanner, Summary: "A"},
		{Name: "ext-scanner-b", Version: "2.0.0", Kind: KindCodemod, Summary: "B"},
	}
	write(t, dir, manifests)
	r := NewRegistry()
	n, err := DiscoverFile(filepath.Join(dir, "plugins.json"), r)
	if err != nil {
		t.Fatalf("DiscoverFile: %v", err)
	}
	if n != 2 {
		t.Errorf("want 2 registered, got %d", n)
	}
	for _, m := range manifests {
		p, ok := r.Lookup(m.Name)
		if !ok {
			t.Errorf("missing %q", m.Name)
		}
		if p.Manifest().Version != m.Version {
			t.Errorf("%q version: want %s got %s", m.Name, m.Version, p.Manifest().Version)
		}
	}
}

// TC-DISC-02 (boundary): absent file is a no-op (returns 0, nil).
func TestDiscoverFile_MissingFile(t *testing.T) {
	n, err := DiscoverFile("/no/such/path/plugins.json", NewRegistry())
	if err != nil {
		t.Fatalf("expected nil for missing file, got %v", err)
	}
	if n != 0 {
		t.Errorf("expected n=0, got %d", n)
	}
}

// TC-DISC-03 (negative): malformed JSON returns error.
func TestDiscoverFile_BadJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "plugins.json")
	if err := os.WriteFile(path, []byte("{ not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := DiscoverFile(path, NewRegistry()); err == nil {
		t.Fatal("expected error for bad JSON")
	}
}

// TC-DISC-04 (negative): entry with missing name returns error.
func TestDiscoverFile_InvalidManifest(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, []Manifest{{Name: "", Version: "1.0.0", Kind: KindScanner}})
	if _, err := DiscoverFile(filepath.Join(dir, "plugins.json"), NewRegistry()); err == nil {
		t.Fatal("expected error for manifest with empty name")
	}
}

// TC-DISC-05 (false-positive guard): already-registered built-in is NOT overwritten.
func TestDiscoverFile_BuiltinPrecedence(t *testing.T) {
	dir := t.TempDir()
	r := NewRegistry()
	builtin := &stubPlugin{m: Manifest{Name: "builtin", Version: "9.9.9", Kind: KindScanner}}
	r.Register(builtin)
	// External entry with same name but different version.
	write(t, dir, []Manifest{{Name: "builtin", Version: "1.0.0", Kind: KindScanner}})
	n, err := DiscoverFile(filepath.Join(dir, "plugins.json"), r)
	if err != nil {
		t.Fatalf("DiscoverFile: %v", err)
	}
	if n != 0 {
		t.Errorf("want 0 newly registered (skip), got %d", n)
	}
	p, _ := r.Lookup("builtin")
	if p.Manifest().Version != "9.9.9" {
		t.Errorf("built-in overwritten; version = %s", p.Manifest().Version)
	}
}

// TC-DISC-06 (idempotency): calling DiscoverFile twice with the same file is safe.
func TestDiscoverFile_Idempotent(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, []Manifest{{Name: "e", Version: "1.0.0", Kind: KindScanner}})
	path := filepath.Join(dir, "plugins.json")
	r := NewRegistry()
	if _, err := DiscoverFile(path, r); err != nil {
		t.Fatalf("first: %v", err)
	}
	// Second call: the name is already registered, so it is skipped (n=0).
	n, err := DiscoverFile(path, r)
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if n != 0 {
		t.Errorf("second call should skip all, got n=%d", n)
	}
}

// TC-DISC-07 (data-accuracy): ExternalPlugin.Manifest() round-trips the JSON data.
func TestExternalPlugin_ManifestRoundTrip(t *testing.T) {
	m := Manifest{
		Name:         "rt-plugin",
		Version:      "3.2.1",
		Kind:         KindTemplate,
		Author:       "acme",
		Summary:      "Round-trip test",
		Capabilities: []string{"fs:read"},
	}
	ep := &ExternalPlugin{manifest: m}
	got := ep.Manifest()
	if got.Name != m.Name || got.Version != m.Version || got.Kind != m.Kind ||
		got.Author != m.Author || len(got.Capabilities) != len(m.Capabilities) {
		t.Errorf("manifest mismatch: got %+v", got)
	}
}

// TC-DISC-08 (data-accuracy): Discover reads from root/.forge/plugins.json.
func TestDiscover_ReadsFromRoot(t *testing.T) {
	dir := t.TempDir()
	dotForge := filepath.Join(dir, ".forge")
	if err := os.MkdirAll(dotForge, 0o755); err != nil {
		t.Fatal(err)
	}
	manifests := []Manifest{{Name: "disc-test", Version: "1.0.0", Kind: KindScanner}}
	body, _ := json.Marshal(manifests)
	if err := os.WriteFile(filepath.Join(dotForge, "plugins.json"), body, 0o600); err != nil {
		t.Fatal(err)
	}
	// Temporarily replace Default() with a clean registry for isolation.
	old := defaultRegistry
	defaultRegistry = NewRegistry()
	t.Cleanup(func() { defaultRegistry = old })

	n, err := Discover(dir)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if n != 1 {
		t.Errorf("want 1, got %d", n)
	}
	if _, ok := Default().Lookup("disc-test"); !ok {
		t.Error("disc-test not found in Default()")
	}
}

// --- helpers ---

func write(t *testing.T, dir string, ms []Manifest) {
	t.Helper()
	body, err := json.Marshal(ms)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "plugins.json"), body, 0o600); err != nil {
		t.Fatal(err)
	}
}

type stubPlugin struct{ m Manifest }

func (s *stubPlugin) Manifest() Manifest { return s.m }
