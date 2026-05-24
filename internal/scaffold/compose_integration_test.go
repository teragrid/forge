//go:build integration

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

package scaffold_test

import (
	"testing"

	"github.com/teragrid/forge/internal/scaffold"
)

// TEST-COMP-10: multiple real modules composed correctly (integration).
func TestCompose_IntegrationMultiModule(t *testing.T) {
	root := t.TempDir()
	makeModule(t, root, "core/rbac", map[string]string{
		"rbac.go":      "package rbac\n",
		".gitignore":   "*.bin\n",
		".env.example": "RBAC_KEY=secret\n",
	})
	makeModule(t, root, "frontend/nextjs-15-supabase", map[string]string{
		"app/page.tsx": "export default function Home(){}\n",
		".env.example": "NEXT_PUBLIC_SUPABASE_URL=\n",
		".gitignore":   ".next/\n",
	})

	result, err := scaffold.Compose(
		[]string{"core/rbac", "frontend/nextjs-15-supabase"},
		scaffold.CompositionOptions{ModulesRoot: root},
	)
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}
	for _, f := range []string{"rbac.go", "app/page.tsx", ".gitignore", ".env.example"} {
		if _, ok := result.Files[f]; !ok {
			t.Errorf("expected %q in result", f)
		}
	}
}

// TEST-COMP-17: real core/mcp-server scaffold reads from forge-knowledge/ (integration).
func TestCompose_Integration_MCPServer_RealScaffold(t *testing.T) {
	// This test reads from the actual forge-knowledge/templates directory.
	// Path is relative to internal/scaffold/ where this package lives.
	result, err := scaffold.Compose(
		[]string{"core/mcp-server"},
		scaffold.CompositionOptions{
			ModulesRoot: "../../forge-knowledge/templates",
			SkipMissing: false,
		},
	)
	if err != nil {
		t.Fatalf("Compose core/mcp-server from real templates: %v", err)
	}

	wantFiles := []string{
		"cmd/mcp/main.go.tmpl",
		"internal/mcpserver/server.go.tmpl",
		"internal/mcpserver/tools.go.tmpl",
		"internal/mcpserver/tools_test.go.tmpl",
		".vscode/settings.json.tmpl",
		".env.example",
	}
	for _, f := range wantFiles {
		if _, ok := result.Files[f]; !ok {
			t.Errorf("real core/mcp-server scaffold missing %q", f)
		}
	}
}

// TEST-COMP-18: real core/mcp-server Python scaffold selected with Language=python.
func TestCompose_Integration_MCPServer_PythonScaffold(t *testing.T) {
	result, err := scaffold.Compose(
		[]string{"core/mcp-server"},
		scaffold.CompositionOptions{
			ModulesRoot: "../../forge-knowledge/templates",
			Language:    "python",
			SkipMissing: false,
		},
	)
	if err != nil {
		t.Fatalf("Compose core/mcp-server Python from real templates: %v", err)
	}

	wantFiles := []string{
		"mcp_server.py.tmpl",
		"tools/__init__.py.tmpl",
		"requirements-mcp.txt",
		".vscode/settings.json.tmpl",
		".env.example",
	}
	for _, f := range wantFiles {
		if _, ok := result.Files[f]; !ok {
			t.Errorf("real core/mcp-server Python scaffold missing %q", f)
		}
	}
	if _, ok := result.Files["cmd/mcp/main.go.tmpl"]; ok {
		t.Error("Go entry point must not be present when Language=python")
	}
}
