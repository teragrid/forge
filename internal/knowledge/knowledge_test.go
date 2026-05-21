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

package knowledge_test

import (
	"strings"
	"testing"

	"github.com/teragrid/forge/internal/knowledge"
)

// ── selector tests ────────────────────────────────────────────────────────────

func TestScore_ShipCheckpointMatch(t *testing.T) {
	e := knowledge.Entry{
		ID: "circuit-breaker",
		ForgeIntegration: knowledge.ForgeIntegration{
			ShipCheckpoints: []string{"code", "ship"},
		},
	}
	if got := knowledge.Score(e, "code", "", "", nil); got != 3 {
		t.Fatalf("want 3, got %d", got)
	}
}

func TestScore_ScanFamilyMatch(t *testing.T) {
	e := knowledge.Entry{
		ID: "circuit-breaker",
		ForgeIntegration: knowledge.ForgeIntegration{
			ScanFamilies: []string{"reliability"},
		},
	}
	if got := knowledge.Score(e, "", "reliability", "", nil); got != 3 {
		t.Fatalf("want 3, got %d", got)
	}
}

func TestScore_TagOverlap(t *testing.T) {
	e := knowledge.Entry{
		ID:   "circuit-breaker",
		Tags: []string{"circuit-breaker", "resilience"},
	}
	got := knowledge.Score(e, "", "", "", []string{"circuit-breaker", "unrelated"})
	if got != 1 {
		t.Fatalf("want 1, got %d", got)
	}
}

func TestScore_NoMatch(t *testing.T) {
	e := knowledge.Entry{ID: "irrelevant"}
	if got := knowledge.Score(e, "spec", "security", "ts-service", nil); got != 0 {
		t.Fatalf("want 0, got %d", got)
	}
}

func TestSelect_ReturnsTopN(t *testing.T) {
	idx := &knowledge.Index{
		Entries: []knowledge.Entry{
			{ID: "a", ForgeIntegration: knowledge.ForgeIntegration{ScanFamilies: []string{"reliability"}}},
			{ID: "b", ForgeIntegration: knowledge.ForgeIntegration{ScanFamilies: []string{"security"}}},
			{ID: "c", ForgeIntegration: knowledge.ForgeIntegration{ScanFamilies: []string{"reliability"}}},
			{ID: "d"},
		},
	}
	got := knowledge.Select(idx, "", "reliability", "", nil)
	if len(got) != 2 {
		t.Fatalf("want 2, got %d", len(got))
	}
}

func TestSelect_NilIndex(t *testing.T) {
	if got := knowledge.Select(nil, "code", "reliability", "", nil); got != nil {
		t.Fatalf("want nil, got %v", got)
	}
}

func TestSelect_MaxN(t *testing.T) {
	entries := make([]knowledge.Entry, 10)
	for i := range entries {
		entries[i] = knowledge.Entry{
			ID:   "e",
			Tags: []string{"tag"},
		}
	}
	idx := &knowledge.Index{Entries: entries}
	got := knowledge.Select(idx, "", "", "", []string{"tag"})
	if len(got) > knowledge.DefaultMaxN {
		t.Fatalf("want ≤%d, got %d", knowledge.DefaultMaxN, len(got))
	}
}

// ── loader tests ──────────────────────────────────────────────────────────────

func TestAppendDocs_Empty(t *testing.T) {
	orig := "system prompt"
	got := knowledge.AppendDocs(orig, nil)
	if got != orig {
		t.Fatalf("want original unchanged, got %q", got)
	}
}

func TestAppendDocs_ContainsKnowledgeTag(t *testing.T) {
	entries := []knowledge.Entry{
		{ID: "circuit-breaker", Intent: "Prevent cascading failures."},
	}
	got := knowledge.AppendDocs("system", entries)
	if !strings.Contains(got, "<knowledge>") {
		t.Fatalf("expected <knowledge> block in output")
	}
	if !strings.Contains(got, "circuit-breaker") {
		t.Fatalf("expected entry ID in output")
	}
}

func TestAppendDocsBudgeted_TrimsOnOverflow(t *testing.T) {
	entries := []knowledge.Entry{
		{ID: "a", Snippet: strings.Repeat("x", 400)},
		{ID: "b", Snippet: strings.Repeat("y", 400)},
		{ID: "c", Snippet: strings.Repeat("z", 400)},
	}
	// maxTokens=10 forces all entries to be trimmed → returns original system prompt
	got := knowledge.AppendDocsBudgeted("sys", entries, 10)
	if got != "sys" {
		t.Fatalf("expected original sys prompt when budget exhausted, got %q", got)
	}
}

func TestAppendDocsBudgeted_NoLimit(t *testing.T) {
	entries := []knowledge.Entry{{ID: "a", Intent: "test"}}
	got := knowledge.AppendDocsBudgeted("sys", entries, 0)
	if !strings.Contains(got, "<knowledge>") {
		t.Fatalf("expected knowledge block when maxTokens=0 (unlimited)")
	}
}

// ── index load test (uses embedded placeholder) ───────────────────────────────

func TestLoad_ReturnsIndex(t *testing.T) {
	idx, err := knowledge.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if idx == nil {
		t.Fatal("Load() returned nil index")
	}
}
