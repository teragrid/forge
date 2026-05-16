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

package capability_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/teragrid/forge/internal/capability"
)

// newTestCapability creates a Define()d capability with a simple echo handler.
func newTestCapability(name string) *capability.Capability {
	return capability.Define(name, capability.Spec{
		Description:  "test capability: " + name,
		InputSchema:  `{"type":"object","properties":{"msg":{"type":"string"}}}`,
		OutputSchema: `{"type":"object","properties":{"echo":{"type":"string"}}}`,
		Tags:         []string{"test"},
		Handler: func(_ context.Context, input map[string]any) (map[string]any, error) {
			msg, _ := input["msg"].(string)
			return map[string]any{"echo": msg}, nil
		},
	})
}

// TestDefine_CreatesCapability verifies Define returns a non-nil Capability with correct name.
func TestDefine_CreatesCapability(t *testing.T) {
	t.Parallel()
	c := capability.Define("my-cap", capability.Spec{Description: "test"})
	if c == nil {
		t.Fatal("Define returned nil")
	}
	if c.Name != "my-cap" {
		t.Errorf("Name = %q, want %q", c.Name, "my-cap")
	}
}

// TestCapability_Manifest returns expected fields.
func TestCapability_Manifest(t *testing.T) {
	t.Parallel()
	c := capability.Define("manifest-test", capability.Spec{
		Description:  "a description",
		InputSchema:  `{"type":"object"}`,
		OutputSchema: `{"type":"string"}`,
		Tags:         []string{"alpha", "beta"},
	})
	m := c.Manifest()
	if m["name"] != "manifest-test" {
		t.Errorf("manifest name = %v", m["name"])
	}
	if m["description"] != "a description" {
		t.Errorf("manifest description = %v", m["description"])
	}
	tags, ok := m["tags"].([]string)
	if !ok || len(tags) != 2 {
		t.Errorf("manifest tags = %v", m["tags"])
	}
}

// TestCapability_Execute_HappyPath calls the handler and returns result.
func TestCapability_Execute_HappyPath(t *testing.T) {
	t.Parallel()
	c := newTestCapability("echo-cap-" + t.Name())
	out, err := c.Execute(context.Background(), map[string]any{"msg": "hello"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["echo"] != "hello" {
		t.Errorf("echo = %v, want %q", out["echo"], "hello")
	}
}

// TestCapability_Execute_NilHandler returns error.
func TestCapability_Execute_NilHandler(t *testing.T) {
	t.Parallel()
	c := capability.Define("no-handler", capability.Spec{})
	_, err := c.Execute(context.Background(), nil)
	if err == nil {
		t.Error("expected error from nil handler")
	}
}

// TestCapability_Execute_HandlerError propagates handler errors.
func TestCapability_Execute_HandlerError(t *testing.T) {
	t.Parallel()
	c := capability.Define("err-cap", capability.Spec{
		Handler: func(_ context.Context, _ map[string]any) (map[string]any, error) {
			return nil, fmt.Errorf("handler failed")
		},
	})
	_, err := c.Execute(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "handler failed") {
		t.Errorf("expected handler error, got: %v", err)
	}
}

// TestRegistry_RegisterAndGet verifies Register + Get round-trips.
func TestRegistry_RegisterAndGet(t *testing.T) {
	// Use unique names to avoid conflict with parallel tests.
	name := "reg-cap-" + t.Name()
	c := newTestCapability(name)
	capability.Register(c)

	got := capability.Get(name)
	if got == nil {
		t.Fatal("Get returned nil after Register")
	}
	if got.Name != name {
		t.Errorf("Get().Name = %q, want %q", got.Name, name)
	}
}

// TestRegistry_GetMissing returns nil for unknown name.
func TestRegistry_GetMissing(t *testing.T) {
	t.Parallel()
	if got := capability.Get("does-not-exist-xyz"); got != nil {
		t.Errorf("Get(unknown) = %v, want nil", got)
	}
}

// TestRegistry_ExecuteUnknown returns error.
func TestRegistry_ExecuteUnknown(t *testing.T) {
	t.Parallel()
	_, err := capability.Execute(context.Background(), "nonexistent-cap-xyz", nil)
	if err == nil {
		t.Error("expected error for unknown capability")
	}
}

// TestRegistry_List returns manifests for all registered capabilities.
func TestRegistry_List(t *testing.T) {
	name := "list-cap-" + t.Name()
	c := newTestCapability(name)
	capability.Register(c)

	manifests := capability.List()
	found := false
	for _, m := range manifests {
		if m["name"] == name {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("List() did not include %q", name)
	}
}

// TestRegistry_ExecuteRegistered calls a registered capability end-to-end.
func TestRegistry_ExecuteRegistered(t *testing.T) {
	name := "exec-reg-" + t.Name()
	c := newTestCapability(name)
	capability.Register(c)

	out, err := capability.Execute(context.Background(), name, map[string]any{"msg": "world"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["echo"] != "world" {
		t.Errorf("echo = %v, want %q", out["echo"], "world")
	}
}
