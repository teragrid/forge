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

package llmprovider_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/teragrid/forge/internal/llmprovider"
)

// TestWarnPrefixBreak_FirstCall writes kv-prefix.json without warning.
func TestWarnPrefixBreak_FirstCall(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// First call: no prior record, should silently write hash.
	llmprovider.WarnPrefixBreak(root, "system prompt v1")
	path := filepath.Join(root, ".forge", "cache", "kv-prefix.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("kv-prefix.json not written: %v", err)
	}
	var rec struct {
		SystemPromptHash string `json:"system_prompt_hash"`
	}
	if err := json.Unmarshal(data, &rec); err != nil {
		t.Fatalf("invalid JSON in kv-prefix.json: %v", err)
	}
	if rec.SystemPromptHash == "" {
		t.Error("expected non-empty system_prompt_hash")
	}
}

// TestWarnPrefixBreak_SamePrompt is a no-op when prompt unchanged.
func TestWarnPrefixBreak_SamePrompt(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	prompt := "stable system prompt"
	llmprovider.WarnPrefixBreak(root, prompt)
	llmprovider.WarnPrefixBreak(root, prompt)
	// No assertion — verifies no panic and kv-prefix.json remains valid.
	path := filepath.Join(root, ".forge", "cache", "kv-prefix.json")
	if _, err := os.Stat(path); err != nil {
		t.Errorf("kv-prefix.json missing after second call: %v", err)
	}
}

// TestWarnPrefixBreak_HashChanges persists updated hash when prompt changes.
func TestWarnPrefixBreak_HashChanges(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	llmprovider.WarnPrefixBreak(root, "version 1")

	path := filepath.Join(root, ".forge", "cache", "kv-prefix.json")
	data1, _ := os.ReadFile(path)
	var rec1 struct {
		SystemPromptHash string `json:"system_prompt_hash"`
	}
	_ = json.Unmarshal(data1, &rec1)
	hash1 := rec1.SystemPromptHash

	llmprovider.WarnPrefixBreak(root, "version 2 — changed content")
	data2, _ := os.ReadFile(path)
	var rec2 struct {
		SystemPromptHash string `json:"system_prompt_hash"`
	}
	_ = json.Unmarshal(data2, &rec2)
	hash2 := rec2.SystemPromptHash

	if hash1 == hash2 {
		t.Errorf("expected different hashes for different prompts, both = %s", hash1)
	}
}

// TestWarnPrefixBreak_EmptyRoot falls back gracefully (does not panic).
func TestWarnPrefixBreak_EmptyPrompt(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// Should not panic; empty prompt is a valid (if unusual) case.
	llmprovider.WarnPrefixBreak(root, "")
}
