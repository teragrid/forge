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

// G-072: Privacy invariant tests — quarantined + scratch files must never
// appear in LLM context bundles.
package cmdcontext_test

import (
	"testing"

	"github.com/teragrid/forge/internal/cli/cmdcontext"
)

// TestIsPrivatePath_LLMScratch verifies that .forge/llm-scratch/ is excluded. G-072.
func TestIsPrivatePath_LLMScratch(t *testing.T) {
	t.Parallel()
	cases := []string{
		".forge/llm-scratch/task-abc/output.txt",
		".forge/llm-scratch/",
		".forge/llm-scratch/nested/deep/file.json",
	}
	for _, c := range cases {
		if !cmdcontext.IsPrivatePath(c) {
			t.Errorf("IsPrivatePath(%q) = false, want true — llm-scratch must be private", c)
		}
	}
}

// TestIsPrivatePath_Trash verifies that .forge/trash/ is excluded. G-072.
func TestIsPrivatePath_Trash(t *testing.T) {
	t.Parallel()
	cases := []string{
		".forge/trash/1234567890/main.go",
		".forge/trash/",
		".forge/trash/run-id/nested/file.ts",
	}
	for _, c := range cases {
		if !cmdcontext.IsPrivatePath(c) {
			t.Errorf("IsPrivatePath(%q) = false, want true — trash must be private", c)
		}
	}
}

// TestIsPrivatePath_OtherQuarantined verifies all quarantined dirs are excluded. G-072.
func TestIsPrivatePath_OtherQuarantined(t *testing.T) {
	t.Parallel()
	cases := []string{
		".forge/outbox/event.json",
		".forge/session/state.json",
		".forge/cache/kv.db",
		".forge/scan-history/report.json",
	}
	for _, c := range cases {
		if !cmdcontext.IsPrivatePath(c) {
			t.Errorf("IsPrivatePath(%q) = false, want true", c)
		}
	}
}

// TestIsPrivatePath_AllowedPaths verifies that legitimate project files are NOT excluded.
func TestIsPrivatePath_AllowedPaths(t *testing.T) {
	t.Parallel()
	cases := []string{
		"README.md",
		"src/main.go",
		"internal/api/handler.go",
		".forge/hygiene.yml",
		".forge/manifest.yaml",
		".forge/cli-schemas/scan.schema.json",
		"docs/ARCHITECTURE.md",
	}
	for _, c := range cases {
		if cmdcontext.IsPrivatePath(c) {
			t.Errorf("IsPrivatePath(%q) = true, want false — must be included in context bundles", c)
		}
	}
}

// TestIsPrivatePath_RootForgeDir verifies that .forge/ itself is not private (only its subdirs are).
func TestIsPrivatePath_ForgeRootNotPrivate(t *testing.T) {
	t.Parallel()
	// The .forge directory root is not quarantined — only specific subdirs.
	if cmdcontext.IsPrivatePath(".forge") {
		t.Error("IsPrivatePath(\".forge\") = true, want false — the .forge root itself is not quarantined")
	}
}

// TestIsPrivatePath_NoPathTraversal ensures that path traversal tricks do not
// accidentally expose quarantined content by bypassing the prefix check.
func TestIsPrivatePath_NoPathTraversal(t *testing.T) {
	t.Parallel()
	// These paths are canonically in quarantined dirs, even with traversal components.
	traversalCases := []string{
		".forge/./llm-scratch/file.txt",
		".forge/../.forge/llm-scratch/file.txt",
		".forge/llm-scratch/../llm-scratch/file.txt",
	}
	for _, c := range traversalCases {
		if !cmdcontext.IsPrivatePath(c) {
			t.Errorf("IsPrivatePath(%q) = false — path traversal must not bypass privacy check", c)
		}
	}
}
