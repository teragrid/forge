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

// Package tierrouter — G-044: Cascade mode.
//
// Cascade is a higher-level strategy that composes the file-backed LLM cache
// (exact hit) with the semantic cache (near-miss hit) and the tier router
// (live call). It reduces provider calls by checking caches first, and records
// all live results so future requests can benefit.
//
// Usage:
//
//	c := tierrouter.NewCascade(router, cache, root)
//	result, err := c.Complete(ctx, req, sourcePaths)
package tierrouter

import (
	"context"
	"fmt"

	"github.com/teragrid/forge/internal/llmcache"
	"github.com/teragrid/forge/internal/llmprovider"
)

// Cascade composes exact cache → semantic cache → tier router.
type Cascade struct {
	router *Router
	cache  *llmcache.Cache
	root   string
}

// NewCascade creates a Cascade. cache may be nil to skip caching.
func NewCascade(router *Router, cache *llmcache.Cache, root string) *Cascade {
	return &Cascade{router: router, cache: cache, root: root}
}

// CascadeResult is the result from a Cascade.Complete call.
type CascadeResult struct {
	RouteResult
	// CacheHit is "exact", "semantic", or "" (live call).
	CacheHit string
}

// Complete resolves the request via: exact cache hit → semantic cache hit →
// tier router (live call). sourcePaths are file paths included in the prompt
// that are used for cache invalidation.
func (c *Cascade) Complete(ctx context.Context, req llmprovider.Request, sourcePaths []string) (*CascadeResult, error) {
	systemPrompt := req.SystemPrompt
	userPrompt := req.UserPrompt

	if c.cache != nil {
		// G-043: warn if system-prompt prefix changed.
		llmprovider.WarnPrefixBreak(c.root, systemPrompt)

		// 1. Exact cache hit.
		key := llmcache.Key(req.Model, systemPrompt, userPrompt)
		if entry, ok := c.cache.Get(key, sourcePaths); ok && entry != nil {
			return &CascadeResult{
				RouteResult: RouteResult{Response: entry.Response},
				CacheHit:    "exact",
			}, nil
		}

		// 2. Semantic cache hit (G-042).
		if entry := c.cache.SemanticLookup(req.Model, systemPrompt, userPrompt); entry != nil {
			return &CascadeResult{
				RouteResult: RouteResult{Response: entry.Response},
				CacheHit:    "semantic",
			}, nil
		}
	}

	// 3. Live call through tier router.
	rr, err := c.router.Route(ctx, req, "")
	if err != nil {
		return nil, fmt.Errorf("cascade: live call failed: %w", err)
	}

	// Store result in cache for future requests.
	if c.cache != nil {
		key := llmcache.Key(req.Model, systemPrompt, userPrompt)
		if storeErr := c.cache.StoreWithTokens(key, req.Model, systemPrompt, userPrompt, rr.Response, sourcePaths); storeErr != nil {
			// Non-fatal — log but do not fail the request.
			fmt.Printf("cascade: cache store warning: %v\n", storeErr)
		}
	}

	return &CascadeResult{RouteResult: *rr, CacheHit: ""}, nil
}
