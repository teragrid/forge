// Package verbmeta tracks per-verb manifests so `forge explain <verb>` can
// surface inputs, outputs, side-effects, gates touched, and error codes
// without each verb shipping its own ad-hoc help (DEV-M0-12 / Spec §4 §16.5.4).
//
// Verbs call Register from their package init() so the manifest is built at
// program start and trivially discoverable from any context.
package verbmeta

import (
	"sort"
	"sync"

	"github.com/teragrid/forge/internal/errcode"
)

// Manifest is the introspection record for a single verb.
type Manifest struct {
	Verb         string         `json:"verb"`
	Summary      string         `json:"summary"`
	Inputs       []string       `json:"inputs"`
	Outputs      []string       `json:"outputs"`
	SideEffects  []string       `json:"side_effects"`
	GatesTouched []string       `json:"gates_touched,omitempty"`
	ErrorCodes   []errcode.Code `json:"error_codes,omitempty"`
	SchemaURI    string         `json:"schema_uri,omitempty"`
}

var (
	mu        sync.RWMutex
	manifests = map[string]Manifest{}
)

// Register records a manifest. Duplicates panic so verb-name collisions fail
// at program start, never at runtime (DEV-M0-12 TC-12-02 regression).
func Register(m Manifest) {
	if m.Verb == "" {
		panic("verbmeta.Register: empty verb")
	}
	mu.Lock()
	defer mu.Unlock()
	if _, ok := manifests[m.Verb]; ok {
		panic("verbmeta.Register: duplicate verb " + m.Verb)
	}
	// Normalise to non-nil slices so JSON output is stable (DEV-M0-12 TC-12-03
	// false-positive guard: no-op verbs still emit a manifest).
	if m.Inputs == nil {
		m.Inputs = []string{}
	}
	if m.Outputs == nil {
		m.Outputs = []string{}
	}
	if m.SideEffects == nil {
		m.SideEffects = []string{}
	}
	manifests[m.Verb] = m
}

// Lookup returns the manifest by verb name and a found flag.
func Lookup(verb string) (Manifest, bool) {
	mu.RLock()
	defer mu.RUnlock()
	m, ok := manifests[verb]
	return m, ok
}

// All returns a sorted snapshot of every registered manifest.
func All() []Manifest {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]Manifest, 0, len(manifests))
	for _, m := range manifests {
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Verb < out[j].Verb })
	return out
}

// Reset clears the registry. Test-only.
func Reset() {
	mu.Lock()
	defer mu.Unlock()
	manifests = map[string]Manifest{}
}
