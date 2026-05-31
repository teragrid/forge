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

// llmpipe_tier_test.go — tests for P1 complexity-based tier routing in LLMPipe.

package cmdship

import (
	"context"
	"testing"

	"github.com/teragrid/forge/internal/llmprovider"
	"github.com/teragrid/forge/internal/tierrouter"
)

// captureProvider records which model was used in the last Complete call.
type captureProvider struct {
	lastModel string
}

func (p *captureProvider) Name() string { return "anthropic" }
func (p *captureProvider) Complete(_ context.Context, req *llmprovider.Request) (*llmprovider.Response, error) {
	p.lastModel = req.Model
	return &llmprovider.Response{Content: "ok", Model: req.Model}, nil
}
func (p *captureProvider) Capabilities() llmprovider.Capabilities { return llmprovider.Capabilities{} }

// TestSetComplexityTier_MapsToCorrectMinTier verifies the complexity → minTier mapping.
func TestSetComplexityTier_MapsToCorrectMinTier(t *testing.T) {
	t.Parallel()
	tests := []struct {
		ct      ComplexityTier
		wantMin string
	}{
		{ComplexityNano, tierrouter.TierCheap},
		{ComplexityMicro, tierrouter.TierCheap},
		{ComplexityStandard, tierrouter.TierBalanced},
		{ComplexityComplex, tierrouter.TierPowerful},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(string(tc.ct), func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			cp := &captureProvider{}
			pipe := newLLMPipeWithProvider(cp, root)
			pipe.SetComplexityTier(tc.ct)
			if got := pipe.minTier; got != tc.wantMin {
				t.Errorf("minTier = %q, want %q", got, tc.wantMin)
			}
		})
	}
}

// TestSetComplexityTier_RouterNotNil verifies router is initialised in constructor.
func TestSetComplexityTier_RouterNotNil(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	cp := &captureProvider{}
	pipe := newLLMPipeWithProvider(cp, root)
	if pipe.router == nil {
		t.Fatal("expected router to be non-nil after newLLMPipeWithProvider")
	}
}

// TestInvoke_UsesRouterWhenMinTierSet verifies Invoke calls router.Route when
// minTier is set (i.e., SetComplexityTier has been called).
// For nano/micro the router starts at T0 (haiku/gpt-4o-mini).
func TestInvoke_UsesRouterWhenMinTierSet(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	cp := &captureProvider{}
	pipe := newLLMPipeWithProvider(cp, root)
	pipe.SetComplexityTier(ComplexityNano)

	_, err := pipe.Invoke("test-op", "", "system", "user", 100)
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	// T0 model for anthropic is claude-3-5-haiku-20241022.
	if got := cp.lastModel; got != "claude-3-5-haiku-20241022" {
		t.Errorf("model used = %q, want T0 haiku model", got)
	}
}

// TestInvoke_FallbackToDirectWhenRouterNil ensures Invoke still works when
// router is nil (defensive nil-safety guard).
func TestInvoke_FallbackToDirectWhenRouterNil(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	cp := &captureProvider{}
	// Use the proper constructor so rewriter/ledger are initialised,
	// then nil out the router to exercise the fallback path.
	pipe := newLLMPipeWithProvider(cp, root)
	pipe.router = nil
	_, err := pipe.Invoke("test-op", "some-model", "system", "user", 100)
	if err != nil {
		t.Fatalf("Invoke with nil router: %v", err)
	}
	if cp.lastModel != "some-model" {
		t.Errorf("expected model=some-model, got %q", cp.lastModel)
	}
}
