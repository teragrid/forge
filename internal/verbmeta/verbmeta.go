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
// Package verbmeta tracks per-verb manifests so `forge explain <verb>` can
// surface inputs, outputs, side-effects, gates touched, and error codes
// without each verb shipping its own ad-hoc help (DEV-M0-12 / Spec §4 §16.5.4).
//
// Verbs call Register from their package init() so the manifest is built at
// program start and trivially discoverable from any context.
//
// JSON schema validation (DEV-M0-11): each verb may declare its expected top-level
// --json output fields via OutputFields. ValidateJSON checks that all declared fields
// are present in a given JSON object, enabling CI schema-drift detection.
package verbmeta

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
	// OutputFields is the list of top-level JSON keys that the verb's --json
	// output must always include (DEV-M0-11 schema-drift gate).
	OutputFields []string `json:"output_fields,omitempty"`
}

// ValidateJSON checks that all OutputFields declared in m are present as
// top-level keys in the JSON object b. Returns a descriptive error for any
// missing field. Returns nil if OutputFields is empty or b is not a JSON object.
func (m Manifest) ValidateJSON(b []byte) error {
	if len(m.OutputFields) == 0 {
		return nil
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(b, &obj); err != nil {
		return nil // not a JSON object — skip (arrays etc.)
	}
	var missing []string
	for _, f := range m.OutputFields {
		if _, ok := obj[f]; !ok {
			missing = append(missing, f)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("verb %q --json output missing required fields: %v", m.Verb, missing)
	}
	return nil
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

// CLISchema is the JSON Schema (Draft 7) generated for a verb's --json output.
type CLISchema struct {
	Schema      string              `json:"$schema"`
	Title       string              `json:"title"`
	Description string              `json:"description"`
	Type        string              `json:"type"`
	Properties  map[string]property `json:"properties"`
	Required    []string            `json:"required,omitempty"`
}

type property struct {
	Type string `json:"type"`
}

// GenerateCLISchemas writes one JSON Schema file per registered verb into
// dir/.forge/cli-schemas/<verb>.schema.json. It creates the directory if it
// does not exist. G-082: per-command JSON schemas.
func GenerateCLISchemas(root string) error {
	schemaDir := filepath.Join(root, ".forge", "cli-schemas")
	if err := os.MkdirAll(schemaDir, 0o755); err != nil {
		return fmt.Errorf("GenerateCLISchemas mkdir: %w", err)
	}

	for _, m := range All() {
		schema := CLISchema{
			Schema:      "http://json-schema.org/draft-07/schema#",
			Title:       "forge " + m.Verb + " output",
			Description: m.Summary,
			Type:        "object",
			Properties:  make(map[string]property, len(m.OutputFields)),
		}
		for _, f := range m.OutputFields {
			schema.Properties[f] = property{Type: "string"}
			schema.Required = append(schema.Required, f)
		}
		if len(schema.Required) == 0 {
			schema.Required = nil
		}

		data, err := json.MarshalIndent(schema, "", "  ")
		if err != nil {
			return fmt.Errorf("GenerateCLISchemas marshal %s: %w", m.Verb, err)
		}
		path := filepath.Join(schemaDir, m.Verb+".schema.json")
		if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
			return fmt.Errorf("GenerateCLISchemas write %s: %w", m.Verb, err)
		}
	}
	return nil
}
