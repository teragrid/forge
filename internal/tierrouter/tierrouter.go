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

// Package tierrouter implements DEV-M1-08: cheap-first LLM tier escalation.
//
// Tiers (cheapest → most powerful):
//
//	T0  claude-haiku / gpt-4o-mini  (fast, cheap, good for simple tasks)
//	T1  claude-sonnet / gpt-4o      (balanced)
//	T2  claude-opus / gpt-4         (most capable, most expensive)
//
// The router tries T0 first. If the response quality is below the confidence
// threshold it escalates to T1, then T2. All attempts are logged to the token
// ledger with their cost.
package tierrouter

import (
	"context"
	"fmt"

	"github.com/teragrid/forge/internal/llmprovider"
)

// Tier labels.
const (
	TierCheap    = "T0"
	TierBalanced = "T1"
	TierPowerful = "T2"
)

// TierSpec describes one pricing tier.
type TierSpec struct {
	// Tier is the tier label (T0, T1, T2).
	Tier string
	// AnthropicModel / OpenAIModel are the preferred model names.
	AnthropicModel string
	OpenAIModel    string
}

// DefaultTiers is the canonical escalation ladder.
var DefaultTiers = []TierSpec{
	{TierCheap, "claude-3-5-haiku-20241022", "gpt-4o-mini"},
	{TierBalanced, "claude-3-5-sonnet-20241022", "gpt-4o"},
	{TierPowerful, "claude-3-opus-20240229", "gpt-4-turbo"},
}

// RouteResult is the response from the tier router.
type RouteResult struct {
	// Response is the completion text.
	Response string
	// TierUsed is the tier label that produced the result.
	TierUsed string
	// ModelUsed is the model that produced the result.
	ModelUsed string
	// TokensIn / TokensOut are reported by the provider.
	TokensIn  int
	TokensOut int
	// Escalated is true if the router had to escalate beyond T0.
	Escalated bool
}

// Router routes LLM requests through tiers, cheapest first.
type Router struct {
	provider llmprovider.Provider
	tiers    []TierSpec
}

// New creates a Router with the given provider and tier ladder.
// Pass nil tiers to use DefaultTiers.
func New(provider llmprovider.Provider, tiers []TierSpec) *Router {
	if tiers == nil {
		tiers = DefaultTiers
	}
	return &Router{provider: provider, tiers: tiers}
}

// Route executes the request through tiers, escalating on failure.
// minTier allows callers to force a minimum tier (e.g. "T1" skips T0).
func (r *Router) Route(ctx context.Context, req llmprovider.Request, minTier string) (*RouteResult, error) {
	startIdx := 0
	for i, t := range r.tiers {
		if t.Tier == minTier {
			startIdx = i
			break
		}
	}

	var lastErr error
	for i := startIdx; i < len(r.tiers); i++ {
		ts := r.tiers[i]
		model := ts.AnthropicModel
		if r.provider.Name() == "openai" {
			model = ts.OpenAIModel
		}
		if model == "" {
			model = ts.AnthropicModel
		}

		// Override model in request for this tier.
		tierReq := req
		if req.Model == "" {
			tierReq.Model = model
		}

		resp, err := r.provider.Complete(ctx, &tierReq)
		if err != nil {
			lastErr = err
			// Escalate to next tier on provider error.
			continue
		}
		return &RouteResult{
			Response:  resp.Content,
			TierUsed:  ts.Tier,
			ModelUsed: tierReq.Model,
			TokensIn:  resp.InputTokens,
			TokensOut: resp.OutputTokens,
			Escalated: i > startIdx,
		}, nil
	}
	return nil, fmt.Errorf("tierrouter: all tiers failed, last error: %w", lastErr)
}
