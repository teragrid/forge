// Package plugin defines the Forge plugin contract and an in-process
// loader (M2 scaffold). The wazero-backed WASM runtime is gated behind
// the `forge_wasm` build tag and arrives in M2.2.
//
// A plugin is anything that:
//   - declares a Manifest (name, version, kind, capabilities)
//   - implements one of the Plugin interfaces (Scanner, Codemod, Provider)
//   - is registered by build (in-process) or discovered at runtime (WASM)
package plugin

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

// Kind classifies what the plugin does.
type Kind string

const (
	KindScanner  Kind = "scanner"
	KindCodemod  Kind = "codemod"
	KindProvider Kind = "provider"
	KindTemplate Kind = "template"
)

// Manifest is the plugin self-description (mirrors `.forge/plugin.toml`).
type Manifest struct {
	Name         string   `json:"name"`
	Version      string   `json:"version"`
	Kind         Kind     `json:"kind"`
	Author       string   `json:"author,omitempty"`
	Summary      string   `json:"summary,omitempty"`
	Capabilities []string `json:"capabilities,omitempty"` // e.g. "fs:read", "net:http"
	Forge        string   `json:"forge_version,omitempty"`
}

// Validate returns an error if the manifest is incomplete or malformed.
func (m Manifest) Validate() error {
	if m.Name == "" {
		return fmt.Errorf("plugin manifest: name is required")
	}
	if m.Version == "" {
		return fmt.Errorf("plugin manifest %q: version is required", m.Name)
	}
	switch m.Kind {
	case KindScanner, KindCodemod, KindProvider, KindTemplate:
	default:
		return fmt.Errorf("plugin manifest %q: unknown kind %q", m.Name, m.Kind)
	}
	return nil
}

// Plugin is the minimal interface every plugin must satisfy.
// Specialized interfaces (Scanner, Codemod, Provider) embed it.
type Plugin interface {
	Manifest() Manifest
}

// Scanner is the interface exposed by `forge scan` plugins.
type Scanner interface {
	Plugin
	Scan(ctx context.Context, root string) ([]Finding, error)
}

// Finding is the cross-plugin result schema for scanners.
type Finding struct {
	File   string `json:"file"`
	Line   int    `json:"line"`
	Rule   string `json:"rule"`
	Match  string `json:"match"`
	Detail string `json:"detail,omitempty"`
}

// Codemod is the interface exposed by `forge upgrade` plugins.
type Codemod interface {
	Plugin
	Apply(ctx context.Context, root string, dryRun bool) (Result, error)
}

// Result describes one codemod's outcome.
type Result struct {
	Files   []string `json:"files"`
	Changed int      `json:"changed"`
	DryRun  bool     `json:"dry_run"`
	Detail  string   `json:"detail,omitempty"`
}

// Registry is a process-wide store of in-tree plugins. Thread-safe;
// safe to call from init().
type Registry struct {
	mu      sync.RWMutex
	entries map[string]Plugin
}

var defaultRegistry = NewRegistry()

// Default returns the package-wide registry.
func Default() *Registry { return defaultRegistry }

// NewRegistry returns an empty registry (use Default() for the global one).
func NewRegistry() *Registry {
	return &Registry{entries: map[string]Plugin{}}
}

// Register adds a plugin. Panics on duplicate name (same as errcode policy).
func (r *Registry) Register(p Plugin) {
	m := p.Manifest()
	if err := m.Validate(); err != nil {
		panic("plugin.Register: " + err.Error())
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.entries[m.Name]; ok {
		panic("plugin.Register: duplicate plugin name " + m.Name)
	}
	r.entries[m.Name] = p
}

// Lookup returns the plugin with the given name.
func (r *Registry) Lookup(name string) (Plugin, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.entries[name]
	return p, ok
}

// All returns every registered plugin, sorted by name.
func (r *Registry) All() []Plugin {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Plugin, 0, len(r.entries))
	for _, p := range r.entries {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Manifest().Name < out[j].Manifest().Name })
	return out
}

// ByKind filters the registry to the given kind.
func (r *Registry) ByKind(k Kind) []Plugin {
	all := r.All()
	out := make([]Plugin, 0, len(all))
	for _, p := range all {
		if p.Manifest().Kind == k {
			out = append(out, p)
		}
	}
	return out
}

// Reset clears the registry (test-only helper).
func (r *Registry) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries = map[string]Plugin{}
}
