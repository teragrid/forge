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
	// AnthropicModel / OpenAIModel / GeminiModel are the preferred model names
	// for each provider family.
	AnthropicModel string
	OpenAIModel    string
	GeminiModel    string
}

// DefaultTiers is the canonical escalation ladder.
//
// TierBalanced's AnthropicModel references llmprovider.AnthropicDefaultModel
// (the single source of truth for the default Anthropic model id — J1/J2)
// instead of its own duplicated literal, so a future default-model update
// only needs to change one place.
var DefaultTiers = []TierSpec{
	{TierCheap, "claude-haiku-4-5-20251001", "gpt-4o-mini", "gemini-2.0-flash"},
	{TierBalanced, llmprovider.AnthropicDefaultModel, "gpt-4o", "gemini-2.5-flash-preview-05-20"},
	{TierPowerful, "claude-opus-4-8-20250514", "gpt-4.1", "gemini-2.5-pro-preview-06-05"},
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
	// Truncated mirrors llmprovider.Response.Truncated: true when the
	// provider's own stop/finish reason says the completion was cut off by
	// MaxTokens rather than finishing naturally.
	Truncated bool
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
		var model string
		switch r.provider.Name() {
		case "openai", "azure-openai":
			// Azure OpenAI uses the same model family as OpenAI.
			model = ts.OpenAIModel
		case "gemini":
			model = ts.GeminiModel
		case "bedrock":
			// Bedrock model IDs share the Anthropic model names (caller must
			// use the full bedrock ARN format if needed, but the base name is the
			// same Anthropic versioned string).
			model = ts.AnthropicModel
		case "github-copilot", "ollama":
			// These providers have their own model selection / defaults; send
			// an empty model string so the provider uses its configured default.
			model = ""
		default:
			// anthropic and any future Anthropic-compatible providers.
			model = ts.AnthropicModel
		}
		if model == "" && ts.AnthropicModel != "" && r.provider.Name() != "github-copilot" && r.provider.Name() != "ollama" {
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
			Truncated: resp.Truncated,
		}, nil
	}
	return nil, fmt.Errorf("tierrouter: all tiers failed, last error: %w", lastErr)
}
