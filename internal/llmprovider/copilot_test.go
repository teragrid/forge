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

// Internal package test for copilot.go — accesses unexported helpers.
package llmprovider

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCopilotProvider_GHToken(t *testing.T) {
	t.Setenv("GH_TOKEN", "ghp_test_token")
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("FORGE_COPILOT_MODEL", "")

	p := newCopilotProvider()
	if p == nil {
		t.Fatal("expected non-nil provider when GH_TOKEN is set")
	}
	if p.Name() != "github-copilot" {
		t.Errorf("Name() = %q, want %q", p.Name(), "github-copilot")
	}
	if p.token != "ghp_test_token" {
		t.Errorf("token = %q, want %q", p.token, "ghp_test_token")
	}
	if p.model != copilotDefaultModel {
		t.Errorf("model = %q, want %q", p.model, copilotDefaultModel)
	}
}

func TestCopilotProvider_GithubTokenFallback(t *testing.T) {
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "ghp_fallback")
	t.Setenv("GH_CONFIG_DIR", "") // prevent config file fallback

	p := newCopilotProvider()
	if p == nil {
		t.Fatal("expected non-nil provider when GITHUB_TOKEN is set")
	}
	if p.token != "ghp_fallback" {
		t.Errorf("token = %q, want %q", p.token, "ghp_fallback")
	}
}

func TestCopilotProvider_ModelOverride(t *testing.T) {
	t.Setenv("GH_TOKEN", "ghp_anything")
	t.Setenv("FORGE_COPILOT_MODEL", "gpt-4o")

	p := newCopilotProvider()
	if p == nil {
		t.Fatal("expected non-nil provider")
	}
	if p.model != "gpt-4o" {
		t.Errorf("model = %q, want gpt-4o", p.model)
	}
}

func TestCopilotProvider_NoToken_ReturnsNil(t *testing.T) {
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "")
	// Point to a non-existent config dir so file fallback also fails.
	tmp := t.TempDir()
	t.Setenv("GH_CONFIG_DIR", filepath.Join(tmp, "does_not_exist"))

	p := newCopilotProvider()
	if p != nil {
		t.Errorf("expected nil provider when no token available, got %+v", p)
	}
}

func TestCopilotProvider_Capabilities(t *testing.T) {
	t.Setenv("GH_TOKEN", "ghp_test")

	p := newCopilotProvider()
	if p == nil {
		t.Fatal("provider must not be nil")
	}
	caps := p.Capabilities()
	if caps.MaxTokens == 0 {
		t.Error("MaxTokens must not be zero")
	}
	if len(caps.Models) == 0 {
		t.Error("Models list must not be empty")
	}
	found := false
	for _, m := range caps.Models {
		if m == copilotDefaultModel {
			found = true
		}
	}
	if !found {
		t.Errorf("default model %q not in Capabilities.Models", copilotDefaultModel)
	}
}

func TestCopilotProvider_NilRequest(t *testing.T) {
	t.Setenv("GH_TOKEN", "ghp_test")

	p := newCopilotProvider()
	if p == nil {
		t.Fatal("provider must not be nil")
	}
	_, err := p.Complete(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil request")
	}
	if !strings.Contains(err.Error(), "FORGE-4052") {
		t.Errorf("expected FORGE-4052 in error, got: %v", err)
	}
}

func TestReadGHConfigToken_WellFormed(t *testing.T) {
	tmp := t.TempDir()
	hostsYML := "github.com:\n    oauth_token: ghp_from_file\n    user: testuser\n    git_protocol: https\n"
	path := filepath.Join(tmp, "hosts.yml")
	if err := os.WriteFile(path, []byte(hostsYML), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GH_CONFIG_DIR", tmp)

	token := readGHConfigToken()
	if token != "ghp_from_file" {
		t.Errorf("token = %q, want %q", token, "ghp_from_file")
	}
}

func TestReadGHConfigToken_MissingFile(t *testing.T) {
	t.Setenv("GH_CONFIG_DIR", filepath.Join(t.TempDir(), "noexist"))

	token := readGHConfigToken()
	if token != "" {
		t.Errorf("expected empty token for missing file, got %q", token)
	}
}

func TestReadGHConfigToken_NoOAuthToken(t *testing.T) {
	tmp := t.TempDir()
	hostsYML := "github.com:\n    user: testuser\n    git_protocol: https\n"
	path := filepath.Join(tmp, "hosts.yml")
	if err := os.WriteFile(path, []byte(hostsYML), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GH_CONFIG_DIR", tmp)

	token := readGHConfigToken()
	if token != "" {
		t.Errorf("expected empty token when oauth_token absent, got %q", token)
	}
}

func TestDetect_CopilotPickedUp(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("AZURE_OPENAI_API_KEY", "")
	t.Setenv("AWS_BEDROCK_REGION", "")
	t.Setenv("OLLAMA_HOST", "")
	t.Setenv("GH_TOKEN", "ghp_detect_test")

	p, err := Detect()
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if p.Name() != "github-copilot" {
		t.Errorf("Detect() provider = %q, want %q", p.Name(), "github-copilot")
	}
}
