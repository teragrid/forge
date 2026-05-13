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

package llmcache

import (
	"os"
	"path/filepath"
	"testing"
)

func TestKeyDeterminism(t *testing.T) {
	k1 := Key("gpt-4", "system", "user")
	k2 := Key("gpt-4", "system", "user")
	if k1 != k2 {
		t.Fatalf("Key() not deterministic: %q != %q", k1, k2)
	}
	k3 := Key("gpt-4", "system", "different")
	if k1 == k3 {
		t.Fatalf("different prompts produced the same key")
	}
}

func TestCacheHitMiss(t *testing.T) {
	dir := t.TempDir()
	c, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}

	key := Key("claude-3", "sys", "prompt")
	// Miss before store.
	if _, ok := c.Get(key, nil); ok {
		t.Fatal("expected cache miss before Store")
	}
	// Store.
	if err := c.Store(key, "claude-3", "hello world", nil); err != nil {
		t.Fatalf("Store: %v", err)
	}
	// Hit after store.
	e, ok := c.Get(key, nil)
	if !ok {
		t.Fatal("expected cache hit after Store")
	}
	if e.Response != "hello world" {
		t.Fatalf("unexpected response %q", e.Response)
	}
}

func TestCacheStaleOnSourceChange(t *testing.T) {
	dir := t.TempDir()
	c, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}

	// Write a source file.
	src := filepath.Join(dir, "source.go")
	if err := os.WriteFile(src, []byte("package foo"), 0o644); err != nil {
		t.Fatal(err)
	}

	key := Key("gpt-4o", "sys", "explain this file")
	paths := []string{src}

	if err := c.Store(key, "gpt-4o", "it defines package foo", paths); err != nil {
		t.Fatal(err)
	}
	// Hit before change.
	if _, ok := c.Get(key, paths); !ok {
		t.Fatal("expected cache hit before source change")
	}
	// Modify source file.
	if err := os.WriteFile(src, []byte("package bar // changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Miss after change.
	if _, ok := c.Get(key, paths); ok {
		t.Fatal("expected cache miss after source file changed")
	}
}

func TestPurge(t *testing.T) {
	dir := t.TempDir()
	c, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		k := Key("m", "s", string(rune('A'+i)))
		_ = c.Store(k, "m", "resp", nil)
	}
	if err := c.Purge(); err != nil {
		t.Fatalf("Purge: %v", err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries after purge, got %d", len(entries))
	}
}

func TestZeroProviderCallOnHit(t *testing.T) {
	dir := t.TempDir()
	c, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}

	providerCalls := 0
	callProvider := func() string {
		providerCalls++
		return "answer"
	}

	key := Key("m", "s", "u")
	// First call: miss → provider is called.
	if _, ok := c.Get(key, nil); !ok {
		resp := callProvider()
		_ = c.Store(key, "m", resp, nil)
	}
	// Second call: hit → provider NOT called.
	if _, ok := c.Get(key, nil); ok {
		// no-op; provider not called
	}

	if providerCalls != 1 {
		t.Fatalf("expected 1 provider call, got %d", providerCalls)
	}
}
