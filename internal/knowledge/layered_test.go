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

// layered_test.go — tests for P1 3-layer knowledge base (LoadLayered).

package knowledge

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoadLayered_HappyPath verifies LoadLayered returns global entries
// when no project/user layers exist.
func TestLoadLayered_HappyPath(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	idx, err := LoadLayered(root)
	if err != nil {
		t.Fatalf("LoadLayered: %v", err)
	}
	if idx == nil {
		t.Fatal("expected non-nil Index")
	}
	// Global embedded index should have at least one entry.
	global, _ := Load()
	if global != nil && len(global.Entries) > 0 && len(idx.Entries) == 0 {
		t.Error("expected project index to include global entries when no layers override them")
	}
}

// TestLoadLayered_ProjectLayerOverridesGlobal verifies that a project-layer
// entry with the same ID shadows the global entry.
func TestLoadLayered_ProjectLayerOverridesGlobal(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	kbDir := filepath.Join(root, ".forge", "kb")
	if err := os.MkdirAll(kbDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Write a project-layer entry.
	md := "---\nid: circuit-breaker\ncategory: patterns/resilience\nintent: project-level override\n---\n# CB\n"
	if err := os.WriteFile(filepath.Join(kbDir, "circuit-breaker.md"), []byte(md), 0o600); err != nil {
		t.Fatal(err)
	}

	idx, err := LoadLayered(root)
	if err != nil {
		t.Fatalf("LoadLayered: %v", err)
	}
	for _, e := range idx.Entries {
		if e.ID == "circuit-breaker" {
			if e.Intent != "project-level override" {
				t.Errorf("expected project-level intent, got %q", e.Intent)
			}
			return
		}
	}
	t.Error("entry circuit-breaker not found in merged index")
}

// TestLoadLayered_NewProjectEntry verifies that an entry present only in the
// project layer appears in the merged index.
func TestLoadLayered_NewProjectEntry(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	kbDir := filepath.Join(root, ".forge", "kb")
	if err := os.MkdirAll(kbDir, 0o755); err != nil {
		t.Fatal(err)
	}

	md := "---\nid: my-custom-pattern\ncategory: custom\nintent: unique project entry\ntags: foo, bar\n---\n# Body\n"
	if err := os.WriteFile(filepath.Join(kbDir, "my-custom-pattern.md"), []byte(md), 0o600); err != nil {
		t.Fatal(err)
	}

	idx, err := LoadLayered(root)
	if err != nil {
		t.Fatalf("LoadLayered: %v", err)
	}
	for _, e := range idx.Entries {
		if e.ID == "my-custom-pattern" {
			if len(e.Tags) < 2 {
				t.Errorf("expected >=2 tags, got %v", e.Tags)
			}
			return
		}
	}
	t.Error("custom project entry not found in merged index")
}

// TestLoadLayered_MissingProjectDir is a no-op when no .forge/kb/ dir exists.
func TestLoadLayered_MissingProjectDir(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// No .forge/kb/ directory — should not error.
	if _, err := LoadLayered(root); err != nil {
		t.Fatalf("LoadLayered should not error on missing layer dirs: %v", err)
	}
}

// TestParseKBMarkdown_NoFrontmatter verifies a plain markdown file uses the
// filename stem as ID and the full content as Snippet.
func TestParseKBMarkdown_NoFrontmatter(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "my-idea.md")
	content := "Some text without frontmatter."
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	e, ok := parseKBMarkdown(path)
	if !ok {
		t.Fatal("parseKBMarkdown returned false for a valid file")
	}
	if e.ID != "my-idea" {
		t.Errorf("ID = %q, want %q", e.ID, "my-idea")
	}
	if e.Snippet != content {
		t.Errorf("Snippet mismatch: got %q", e.Snippet)
	}
}
