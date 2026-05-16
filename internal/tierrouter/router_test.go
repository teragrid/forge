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

// Gap tests for G-042: tier router model-tier rules.
package tierrouter_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/teragrid/forge/internal/llmcache"
	"github.com/teragrid/forge/internal/llmprovider"
	"github.com/teragrid/forge/internal/tierrouter"
)

// ── G-042: TestTierRouter_ModelTierRules ──────────────────────────────────────

// TestTierRouter_DefaultTiersCount verifies DefaultTiers has exactly 3 tiers.
func TestTierRouter_DefaultTiersCount(t *testing.T) {
	t.Parallel()
	if len(tierrouter.DefaultTiers) != 3 {
		t.Fatalf("expected 3 default tiers, got %d", len(tierrouter.DefaultTiers))
	}
}

// TestTierRouter_DefaultTierLabels verifies the default tier ladder labels.
func TestTierRouter_DefaultTierLabels(t *testing.T) {
	t.Parallel()
	want := []string{tierrouter.TierCheap, tierrouter.TierBalanced, tierrouter.TierPowerful}
	for i, ts := range tierrouter.DefaultTiers {
		if ts.Tier != want[i] {
			t.Errorf("tier[%d]: want %q, got %q", i, want[i], ts.Tier)
		}
	}
}

// TestTierRouter_T0UsesHaikuOrMini verifies that T0 uses cheap model names.
func TestTierRouter_T0UsesHaikuOrMini(t *testing.T) {
	t.Parallel()
	t0 := tierrouter.DefaultTiers[0]
	if t0.Tier != tierrouter.TierCheap {
		t.Fatalf("DefaultTiers[0] should be TierCheap")
	}
	if !strings.Contains(t0.AnthropicModel, "haiku") {
		t.Errorf("T0 Anthropic model should contain 'haiku', got %q", t0.AnthropicModel)
	}
	if !strings.Contains(t0.OpenAIModel, "mini") {
		t.Errorf("T0 OpenAI model should contain 'mini', got %q", t0.OpenAIModel)
	}
}

// TestTierRouter_T2UsesPowerfulModel verifies that T2 uses powerful model names.
func TestTierRouter_T2UsesPowerfulModel(t *testing.T) {
	t.Parallel()
	t2 := tierrouter.DefaultTiers[2]
	if t2.Tier != tierrouter.TierPowerful {
		t.Fatalf("DefaultTiers[2] should be TierPowerful")
	}
	if !strings.Contains(t2.AnthropicModel, "opus") && !strings.Contains(t2.AnthropicModel, "sonnet") {
		t.Errorf("T2 Anthropic model should be opus/sonnet, got %q", t2.AnthropicModel)
	}
}

// TestTierRouter_T0RespondsWithMockProvider verifies that a simple request
// is served by T0 when the mock provider succeeds on the first call.
func TestTierRouter_T0RespondsWithMockProvider(t *testing.T) {
	t.Parallel()
	mock := &llmprovider.MockProvider{
		Response: &llmprovider.Response{
			Content:      "hello from mock",
			InputTokens:  5,
			OutputTokens: 3,
			Model:        "mock-v1",
		},
	}
	router := tierrouter.New(mock, nil)
	res, err := router.Route(context.Background(), llmprovider.Request{
		UserPrompt: "Say hello",
	}, "")
	if err != nil {
		t.Fatalf("Route: %v", err)
	}
	if res.Response != "hello from mock" {
		t.Errorf("response: want %q, got %q", "hello from mock", res.Response)
	}
	if res.TierUsed != tierrouter.TierCheap {
		t.Errorf("tier: want %q, got %q", tierrouter.TierCheap, res.TierUsed)
	}
	if res.Escalated {
		t.Error("should not have escalated from T0")
	}
}

// TestTierRouter_EscalatesOnProviderError verifies that when T0 fails,
// the router escalates to T1 and uses T1's response.
func TestTierRouter_EscalatesOnProviderError(t *testing.T) {
	t.Parallel()
	callCount := 0
	mock := &llmprovider.MockProvider{}
	// First call fails, second succeeds.
	_ = mock // We'll use a custom approach: override via custom tiers with the mock.

	// Build custom tiers pointing to the same mock.
	// The mock's Complete will return an error on the first call and succeed on
	// the second call. We simulate this with a wrappedProvider.
	wrapped := &errorOnFirstProvider{
		inner:       mock,
		failOnFirst: true,
		calls:       &callCount,
	}
	router := tierrouter.New(wrapped, []tierrouter.TierSpec{
		{Tier: tierrouter.TierCheap, AnthropicModel: "haiku"},
		{Tier: tierrouter.TierBalanced, AnthropicModel: "sonnet"},
	})
	res, err := router.Route(context.Background(), llmprovider.Request{
		UserPrompt: "Say hello",
	}, "")
	if err != nil {
		t.Fatalf("Route should escalate and succeed: %v", err)
	}
	if res.TierUsed != tierrouter.TierBalanced {
		t.Errorf("tier: want %q, got %q", tierrouter.TierBalanced, res.TierUsed)
	}
	if !res.Escalated {
		t.Error("Escalated should be true")
	}
}

// TestTierRouter_AllTiersFailReturnsError verifies that when all tiers fail,
// an error is returned.
func TestTierRouter_AllTiersFailReturnsError(t *testing.T) {
	t.Parallel()
	failing := &llmprovider.MockProvider{Err: errors.New("all tiers out")}
	router := tierrouter.New(failing, []tierrouter.TierSpec{
		{Tier: tierrouter.TierCheap, AnthropicModel: "haiku"},
		{Tier: tierrouter.TierBalanced, AnthropicModel: "sonnet"},
	})
	_, err := router.Route(context.Background(), llmprovider.Request{
		UserPrompt: "test",
	}, "")
	if err == nil {
		t.Fatal("expected error when all tiers fail, got nil")
	}
}

// TestTierRouter_MinTierSkipsT0 verifies that passing minTier="T1" skips T0.
func TestTierRouter_MinTierSkipsT0(t *testing.T) {
	t.Parallel()
	callCount := 0
	mock := &countingProvider{calls: &callCount}
	router := tierrouter.New(mock, nil)
	res, err := router.Route(context.Background(), llmprovider.Request{
		UserPrompt: "test",
	}, tierrouter.TierBalanced)
	if err != nil {
		t.Fatalf("Route: %v", err)
	}
	if res.TierUsed != tierrouter.TierBalanced {
		t.Errorf("tier: want %q, got %q", tierrouter.TierBalanced, res.TierUsed)
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

// errorOnFirstProvider fails on the first Complete call, succeeds on subsequent ones.
type errorOnFirstProvider struct {
	inner       llmprovider.Provider
	failOnFirst bool
	calls       *int
}

func (e *errorOnFirstProvider) Name() string { return "error-on-first" }
func (e *errorOnFirstProvider) Capabilities() llmprovider.Capabilities {
	return llmprovider.Capabilities{MaxTokens: 4096, Models: []string{"mock"}}
}
func (e *errorOnFirstProvider) Complete(ctx context.Context, req *llmprovider.Request) (*llmprovider.Response, error) {
	*e.calls++
	if *e.calls == 1 {
		return nil, errors.New("first call fails")
	}
	return &llmprovider.Response{Content: "escalated response", Model: "mock"}, nil
}

// countingProvider counts Complete calls and always succeeds.
type countingProvider struct {
	calls *int
}

func (c *countingProvider) Name() string { return "counting" }
func (c *countingProvider) Capabilities() llmprovider.Capabilities {
	return llmprovider.Capabilities{MaxTokens: 4096, Models: []string{"mock"}}
}
func (c *countingProvider) Complete(_ context.Context, req *llmprovider.Request) (*llmprovider.Response, error) {
	*c.calls++
	return &llmprovider.Response{Content: "ok", Model: req.Model}, nil
}

// ── G-044: TestCascade ────────────────────────────────────────────────────────

// TestCascade_ExactHit verifies that Cascade returns an "exact" cache hit
// without making a live provider call when the exact prompt is cached.
func TestCascade_ExactHit(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cache, err := llmcache.Open(dir)
	if err != nil {
		t.Fatalf("Open cache: %v", err)
	}

	calls := 0
	mock := &countingProvider{calls: &calls}
	router := tierrouter.New(mock, nil)
	cascade := tierrouter.NewCascade(router, cache, "")

	req := llmprovider.Request{Model: "mock", SystemPrompt: "sys", UserPrompt: "hello world"}
	key := llmcache.Key(req.Model, req.SystemPrompt, req.UserPrompt)
	if err := cache.Store(key, req.Model, "cached response", nil); err != nil {
		t.Fatalf("cache.Store: %v", err)
	}

	res, err := cascade.Complete(context.Background(), req, nil)
	if err != nil {
		t.Fatalf("Cascade.Complete: %v", err)
	}
	if res.CacheHit != "exact" {
		t.Errorf("CacheHit: want %q, got %q", "exact", res.CacheHit)
	}
	if res.Response != "cached response" {
		t.Errorf("Response: want %q, got %q", "cached response", res.Response)
	}
	if calls != 0 {
		t.Errorf("provider calls: want 0 (cache hit), got %d", calls)
	}
}

// TestCascade_LiveCallOnMiss verifies that Cascade makes a live provider call
// when neither exact nor semantic cache has a match, and stores the result.
func TestCascade_LiveCallOnMiss(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cache, err := llmcache.Open(dir)
	if err != nil {
		t.Fatalf("Open cache: %v", err)
	}

	calls := 0
	mock := &countingProvider{calls: &calls}
	router := tierrouter.New(mock, nil)
	cascade := tierrouter.NewCascade(router, cache, "")

	req := llmprovider.Request{Model: "mock", SystemPrompt: "sys", UserPrompt: "brand new prompt no match"}
	res, err := cascade.Complete(context.Background(), req, nil)
	if err != nil {
		t.Fatalf("Cascade.Complete: %v", err)
	}
	if res.CacheHit != "" {
		t.Errorf("CacheHit: want %q (live call), got %q", "", res.CacheHit)
	}
	if calls != 1 {
		t.Errorf("provider calls: want 1, got %d", calls)
	}

	// Second call with same prompt: should now be exact cache hit.
	calls = 0
	res2, err := cascade.Complete(context.Background(), req, nil)
	if err != nil {
		t.Fatalf("Cascade.Complete second: %v", err)
	}
	if res2.CacheHit != "exact" {
		t.Errorf("second call CacheHit: want %q, got %q", "exact", res2.CacheHit)
	}
	if calls != 0 {
		t.Errorf("second call provider calls: want 0, got %d", calls)
	}
}

// TestCascade_NilCacheStillWorks verifies that Cascade works correctly when
// no cache is provided (nil), always making live calls.
func TestCascade_NilCacheStillWorks(t *testing.T) {
	t.Parallel()
	calls := 0
	mock := &countingProvider{calls: &calls}
	router := tierrouter.New(mock, nil)
	cascade := tierrouter.NewCascade(router, nil, "")

	req := llmprovider.Request{Model: "mock", SystemPrompt: "sys", UserPrompt: "test"}
	_, err := cascade.Complete(context.Background(), req, nil)
	if err != nil {
		t.Fatalf("Cascade.Complete (nil cache): %v", err)
	}
	if calls != 1 {
		t.Errorf("provider calls: want 1, got %d", calls)
	}
}
