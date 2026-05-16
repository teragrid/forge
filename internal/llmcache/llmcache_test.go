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
	if _, ok := c.Get(key, nil); !ok {
		t.Fatal("second call: expected cache hit but got miss")
	}

	if providerCalls != 1 {
		t.Fatalf("expected 1 provider call, got %d", providerCalls)
	}
}


// G-041: TestCache_HitRateReported verifies that HitRate() correctly reflects
// the ratio of cache hits to total lookups. This validates the forge insights
// cache hit metric source.
func TestCache_HitRateReported(t *testing.T) {
	// Reset global counters so previous test runs don't interfere.
	ResetStats()

	dir := t.TempDir()
	c, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}

	// No lookups yet -- HitRate should be 0.
	if got := HitRate(); got != 0 {
		t.Errorf("HitRate before any lookup: want 0, got %g", got)
	}

	key := Key("model", "sys", "user-g041")

	// First lookup: miss.
	c.GetWithStats(key, nil)
	if got := HitRate(); got != 0 {
		t.Errorf("HitRate after first miss: want 0, got %g", got)
	}

	// Store a value.
	_ = c.Store(key, "model", "cached-response", nil)

	// Second lookup: hit.
	c.GetWithStats(key, nil)
	// 1 hit / 2 total = 0.5
	want := 0.5
	if got := HitRate(); got != want {
		t.Errorf("HitRate after 1 hit 1 miss: want %g, got %g", want, got)
	}

	// Third lookup: hit.
	c.GetWithStats(key, nil)
	// 2 hits / 3 total ~0.6667
	if got := HitRate(); got <= 0.6 || got > 0.7 {
		t.Errorf("HitRate after 2 hits 1 miss: want ~0.667, got %g", got)
	}
}

// TestCache_HitRate_ZeroWhenNoLookups verifies HitRate returns 0 when no
// lookups have been made.
func TestCache_HitRate_ZeroWhenNoLookups(t *testing.T) {
	ResetStats()
	if got := HitRate(); got != 0 {
		t.Errorf("HitRate with zero lookups: want 0, got %g", got)
	}
}

// ── G-042: SemanticLookup ─────────────────────────────────────────────────────

// TestSemanticLookup_HitOnSimilarPrompt verifies that a prompt highly similar
// to a cached prompt (Jaccard ≥ SemanticThreshold) is returned as a hit.
func TestSemanticLookup_HitOnSimilarPrompt(t *testing.T) {
	dir := t.TempDir()
	c, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}

	model := "test-model"
	sys := "You are a helpful assistant for Go programming."
	user := "Explain the purpose of the context package in Go standard library."
	key := Key(model, sys, user)

	if err := c.StoreWithTokens(key, model, sys, user, "context manages deadlines", nil); err != nil {
		t.Fatalf("StoreWithTokens: %v", err)
	}

	// Slightly different wording, but high token overlap.
	got := c.SemanticLookup(model, sys, "Explain the purpose of context package in Go.")
	if got == nil {
		t.Fatal("SemanticLookup: expected hit for similar prompt, got nil")
	}
	if got.Response != "context manages deadlines" {
		t.Errorf("SemanticLookup: response = %q, want %q", got.Response, "context manages deadlines")
	}
}

// TestSemanticLookup_MissOnDissimilarPrompt verifies that a completely
// different prompt does not produce a false hit.
func TestSemanticLookup_MissOnDissimilarPrompt(t *testing.T) {
	dir := t.TempDir()
	c, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}

	model := "test-model"
	sys := "You are a helpful assistant."
	user := "Explain channels in Go."
	key := Key(model, sys, user)

	if err := c.StoreWithTokens(key, model, sys, user, "channels enable goroutine communication", nil); err != nil {
		t.Fatalf("StoreWithTokens: %v", err)
	}

	// Completely different topic — should not match.
	got := c.SemanticLookup(model, sys, "How do I deploy a Python web application to AWS?")
	if got != nil {
		t.Errorf("SemanticLookup: expected nil for dissimilar prompt, got %+v", got)
	}
}

// TestSemanticLookup_ModelIsolation verifies that entries cached under one
// model are not returned for a different model.
func TestSemanticLookup_ModelIsolation(t *testing.T) {
	dir := t.TempDir()
	c, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}

	sys := "You are a helpful assistant for Go programming."
	user := "Explain the purpose of the context package in Go standard library."
	keyA := Key("model-a", sys, user)

	if err := c.StoreWithTokens(keyA, "model-a", sys, user, "model-a response", nil); err != nil {
		t.Fatalf("StoreWithTokens: %v", err)
	}

	// Same-ish prompt but for model-b → should not match model-a's entry.
	got := c.SemanticLookup("model-b", sys, "Explain the purpose of context package in Go.")
	if got != nil {
		t.Errorf("SemanticLookup: expected nil (model isolation), got %+v", got)
	}
}
