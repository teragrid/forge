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

// Package llmcache — G-042: embedding-similarity semantic cache tier.
//
// SemanticLookup adds a second pass after the exact-key miss in the file-backed
// cache: it computes a simple bag-of-words Jaccard similarity between the
// current prompt and all cached prompts, and returns the highest-similarity
// entry if it exceeds the SemanticThreshold. This avoids costly provider calls
// when a near-identical prompt has been answered before.
//
// NOTE: This uses a zero-dependency heuristic (token Jaccard) rather than
// embedding vectors, keeping CGO_ENABLED=0 compliance. A vector-based tier
// can be added later via ADR.
package llmcache

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// SemanticThreshold is the minimum Jaccard similarity score [0,1] for a
// semantic cache hit. Default 0.85 (very similar, not identical).
const SemanticThreshold = 0.85

// SemanticEntry is an on-disk record augmented with its prompt fingerprint.
type SemanticEntry struct {
	CacheEntry
	// PromptTokens is the bag-of-words token set used for similarity matching.
	PromptTokens []string `json:"prompt_tokens,omitempty"`
}

// SemanticLookup searches the cache for an entry whose prompt is highly
// similar to (model+system+user) using token Jaccard similarity. Returns nil
// if no match exceeds SemanticThreshold. This is called only after an exact
// key miss.
func (c *Cache) SemanticLookup(model, systemPrompt, userPrompt string) *CacheEntry {
	combined := systemPrompt + " " + userPrompt
	queryTokens := tokenSet(combined)
	if len(queryTokens) == 0 {
		return nil
	}

	entries, err := os.ReadDir(c.dir)
	if err != nil {
		return nil
	}

	var best *CacheEntry
	bestScore := 0.0

	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(c.dir, e.Name()))
		if err != nil {
			continue
		}
		var se SemanticEntry
		if err := json.Unmarshal(data, &se); err != nil {
			continue
		}
		if se.Model != model {
			continue
		}
		if len(se.PromptTokens) == 0 {
			continue
		}
		score := jaccardSim(queryTokens, tokenSetFromSlice(se.PromptTokens))
		if score > bestScore && score >= SemanticThreshold {
			bestScore = score
			entry := se.CacheEntry
			best = &entry
		}
	}
	return best
}

// StoreWithTokens writes an entry alongside its prompt token fingerprint so
// that SemanticLookup can match it later.
func (c *Cache) StoreWithTokens(key, model, systemPrompt, userPrompt, response string, sourcePaths []string) error {
	combined := systemPrompt + " " + userPrompt
	tokens := tokenSlice(combined)
	se := SemanticEntry{
		CacheEntry: CacheEntry{
			Key:         key,
			Model:       model,
			Response:    response,
			SourcePaths: sourcePaths,
		},
		PromptTokens: tokens,
	}
	data, err := json.MarshalIndent(se, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(c.dir, key+".json"), data, 0o600)
}

// ── helpers ──────────────────────────────────────────────────────────────────

func tokenSet(s string) map[string]bool {
	out := map[string]bool{}
	for _, w := range strings.Fields(s) {
		w = strings.ToLower(strings.Trim(w, ".,;:!?\"'()[]{}"))
		if w != "" {
			out[w] = true
		}
	}
	return out
}

func tokenSetFromSlice(sl []string) map[string]bool {
	out := map[string]bool{}
	for _, w := range sl {
		out[w] = true
	}
	return out
}

func tokenSlice(s string) []string {
	set := tokenSet(s)
	out := make([]string, 0, len(set))
	for w := range set {
		out = append(out, w)
	}
	return out
}

func jaccardSim(a, b map[string]bool) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 1.0
	}
	intersection := 0
	for w := range a {
		if b[w] {
			intersection++
		}
	}
	union := len(a) + len(b) - intersection
	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}
