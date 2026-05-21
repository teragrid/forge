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

// Package knowledge implements ADR-026: demand-pull, tag-indexed knowledge-base
// injection for forge ship, forge scan, and forge new.
//
// The knowledge base (~163 markdown files, ~500 KB) is never loaded wholesale.
// Instead, a compact JSON index (~20 KB, embedded at compile time) is queried
// per LLM call. Only the top-N matching entries (≤5) are injected as a
// <knowledge> block, adding ≤800 tokens to the prompt budget.
//
// Consumption:
//
//	idx, _ := knowledge.Load()
//	entries := knowledge.Select(idx, "code", "reliability", "go-service", []string{"circuit-breaker"})
//	system   = knowledge.AppendDocsBudgeted(system, entries, maxTokens)
package knowledge

// ForgeIntegration mirrors the forge_integration frontmatter block.
type ForgeIntegration struct {
	ScanFamilies    []string `json:"scan_families"`
	Templates       []string `json:"templates"`
	Conventions     []string `json:"conventions"`
	ShipCheckpoints []string `json:"ship_checkpoints"`
}

// Entry is one knowledge-base document, pre-processed at index-generation time.
type Entry struct {
	// ID is the frontmatter `id` field (e.g. "circuit-breaker").
	ID string `json:"id"`

	// Category is the frontmatter `category` (e.g. "patterns/resilience").
	Category string `json:"category"`

	// Path is the relative path from knowledge-base/ root (e.g. "patterns/resilience/circuit-breaker.md").
	Path string `json:"path"`

	// Intent is the first sentence extracted from the ## Intent section.
	// Kept to ≤200 chars; used as the primary display string.
	Intent string `json:"intent"`

	// Snippet is intent + first paragraph of ## Solution, truncated to ≤300 chars.
	// Injected verbatim into the <knowledge> block.
	Snippet string `json:"snippet"`

	// ForgeIntegration holds the structured routing metadata from frontmatter.
	ForgeIntegration ForgeIntegration `json:"forge_integration"`

	// Tags is the flat list of frontmatter tags.
	Tags []string `json:"tags"`
}

// Index is the in-memory representation of index.json.
type Index struct {
	// Version is the schema version, currently "1".
	Version string `json:"version"`

	// Entries is the full entry list, ordered by category+id.
	Entries []Entry `json:"entries"`
}
