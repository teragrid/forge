package plugin

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// DiscoveryPath is the default project-relative path for the plugin
// discovery file. JSON is used because the codebase ships zero non-stdlib
// deps (no TOML library); the on-disk name intentionally uses a .json
// extension to avoid confusion with the WASM-era .forge/plugin.toml
// (which arrives in DEV-M2-05 behind the forge_wasm build tag).
const DiscoveryPath = ".forge/plugins.json"

// ExternalPlugin is a declared-but-not-loaded external plugin entry.
// Its manifest is read from DiscoveryPath; its callable body is a stub
// that returns ErrNotLoaded until the WASM runtime (DEV-M2-05) replaces
// the stub with a real host-function binding.
type ExternalPlugin struct {
	manifest Manifest
}

// Manifest implements Plugin.
func (e *ExternalPlugin) Manifest() Manifest { return e.manifest }

// ErrNotLoaded is returned by any ExternalPlugin method that would
// require an actual WASM host call. This will be wired in DEV-M2-05.
var ErrNotLoaded = fmt.Errorf("plugin not loaded: WASM runtime not yet available (see DEV-M2-05)")

// DiscoverFile reads a JSON discovery file at path and registers each
// declared plugin manifest into r as an ExternalPlugin. Already-registered
// names are silently skipped (in-tree built-ins take precedence). Returns
// the count of newly registered plugins.
//
// File format:
//
//	[
//	  {"name":"my-scanner","version":"1.0.0","kind":"scanner","summary":"..."},
//	  ...
//	]
func DiscoverFile(path string, r *Registry) (int, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("plugin discovery: read %s: %w", path, err)
	}
	var manifests []Manifest
	if err := json.Unmarshal(b, &manifests); err != nil {
		return 0, fmt.Errorf("plugin discovery: parse %s: %w", path, err)
	}
	var n int
	for i, m := range manifests {
		if err := m.Validate(); err != nil {
			return n, fmt.Errorf("plugin discovery %s entry %d: %w", path, i, err)
		}
		if _, exists := r.Lookup(m.Name); exists {
			continue // built-in takes precedence
		}
		r.Register(&ExternalPlugin{manifest: m})
		n++
	}
	return n, nil
}

// Discover loads plugins from root/.forge/plugins.json into the default
// registry. If the file is absent it is a no-op. This is called once at
// CLI startup from root.go before any verb runs.
func Discover(root string) (int, error) {
	return DiscoverFile(filepath.Join(root, DiscoveryPath), Default())
}
