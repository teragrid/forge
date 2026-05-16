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

// Package capability implements G-100: the Capability registry runtime SDK.
//
// Every Capability ships an LLM-readable manifest and the same business logic
// is callable from chat, agents, and external tools without rewrites.
//
// Usage:
//
//	cap := capability.Define("create-user", capability.Spec{
//	    Description: "Create a new user account",
//	    InputSchema:  schema,
//	    Handler:      createUserHandler,
//	})
//	capability.Register(cap)
//	capability.Execute(ctx, "create-user", input)
package capability

import (
	"context"
	"fmt"
	"sync"
)

// Spec defines the metadata and handler for a Capability.
type Spec struct {
	// Description is the human- and LLM-readable description of what this
	// capability does.
	Description string
	// InputSchema is a JSON Schema string describing the expected input.
	InputSchema string
	// OutputSchema is a JSON Schema string describing the output.
	OutputSchema string
	// Tags are arbitrary labels for filtering.
	Tags []string
	// Handler is the Go function that implements the capability.
	Handler func(ctx context.Context, input map[string]any) (map[string]any, error)
}

// Capability is a registered callable unit with an LLM-readable manifest.
type Capability struct {
	Name string
	Spec Spec
}

// Manifest returns the LLM-readable manifest for this capability.
func (c *Capability) Manifest() map[string]any {
	return map[string]any{
		"name":          c.Name,
		"description":   c.Spec.Description,
		"input_schema":  c.Spec.InputSchema,
		"output_schema": c.Spec.OutputSchema,
		"tags":          c.Spec.Tags,
	}
}

// Execute invokes the capability's handler with the given input.
func (c *Capability) Execute(ctx context.Context, input map[string]any) (map[string]any, error) {
	if c.Spec.Handler == nil {
		return nil, fmt.Errorf("capability %q has no handler", c.Name)
	}
	return c.Spec.Handler(ctx, input)
}

// ── Registry ─────────────────────────────────────────────────────────────────

var (
	mu       sync.RWMutex
	registry = map[string]*Capability{}
)

// Define creates a Capability from a name and spec. Does not register it;
// call Register to add it to the global registry.
func Define(name string, spec Spec) *Capability {
	return &Capability{Name: name, Spec: spec}
}

// Register adds c to the global registry. Panics if name already registered.
func Register(c *Capability) {
	mu.Lock()
	defer mu.Unlock()
	if _, exists := registry[c.Name]; exists {
		panic(fmt.Sprintf("capability %q already registered", c.Name))
	}
	registry[c.Name] = c
}

// Execute looks up name in the global registry and calls its handler.
func Execute(ctx context.Context, name string, input map[string]any) (map[string]any, error) {
	mu.RLock()
	c, ok := registry[name]
	mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("capability %q not found", name)
	}
	return c.Execute(ctx, input)
}

// List returns all registered capability manifests — used by LLM agents to
// discover available tools.
func List() []map[string]any {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]map[string]any, 0, len(registry))
	for _, c := range registry {
		out = append(out, c.Manifest())
	}
	return out
}

// Get returns the named capability or nil if not registered.
func Get(name string) *Capability {
	mu.RLock()
	defer mu.RUnlock()
	return registry[name]
}
