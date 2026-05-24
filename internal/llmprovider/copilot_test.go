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

// Internal package test for copilot.go -- accesses unexported helpers.
package llmprovider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
	t.Setenv("GH_CONFIG_DIR", "")

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
	tmp := t.TempDir()
	t.Setenv("GH_CONFIG_DIR", filepath.Join(tmp, "does_not_exist"))
	// Block `gh auth token` subprocess so the test is hermetic.
	t.Setenv("PATH", tmp)

	p := newCopilotProvider()
	if p != nil {
		t.Errorf("expected nil provider when no token available, got %+v", p)
	}
}

// -- Dynamic model list -------------------------------------------------------

func mockModelsServer(t *testing.T, ids []string) (*httptest.Server, *int) {
	t.Helper()
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/models" {
			http.NotFound(w, r)
			return
		}
		calls++
		type model struct {
			ID     string `json:"id"`
			Object string `json:"object"`
		}
		type resp struct {
			Object string  `json:"object"`
			Data   []model `json:"data"`
		}
		data := make([]model, 0, len(ids))
		for _, id := range ids {
			data = append(data, model{ID: id, Object: "model"})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp{Object: "list", Data: data})
	}))
	t.Cleanup(srv.Close)
	return srv, &calls
}

func TestCopilotProvider_Capabilities_LiveModels(t *testing.T) {
	liveModels := []string{"claude-sonnet-4-5", "gpt-4o", "o3"}
	srv, calls := mockModelsServer(t, liveModels)

	p := &CopilotProvider{
		token:   "ghp_test",
		model:   copilotDefaultModel,
		baseURL: srv.URL,
		client:  srv.Client(),
	}

	caps := p.Capabilities()
	if *calls != 1 {
		t.Errorf("/models called %d times, want 1", *calls)
	}
	if len(caps.Models) != len(liveModels) {
		t.Errorf("Models = %v, want %v", caps.Models, liveModels)
	}
	for i, id := range liveModels {
		if caps.Models[i] != id {
			t.Errorf("Models[%d] = %q, want %q", i, caps.Models[i], id)
		}
	}
	if caps.MaxTokens == 0 {
		t.Error("MaxTokens must not be zero")
	}
}

func TestCopilotProvider_Capabilities_FallbackOnError(t *testing.T) {
	// Port 1 is refused; this tests the unreachable-endpoint code path.
	p := &CopilotProvider{
		token:   "ghp_test",
		model:   copilotDefaultModel,
		baseURL: "http://127.0.0.1:1",
		client:  &http.Client{},
	}

	caps := p.Capabilities()
	if len(caps.Models) == 0 {
		t.Fatal("expected fallback models when endpoint is unreachable")
	}
	found := false
	for _, m := range caps.Models {
		if m == copilotDefaultModel {
			found = true
		}
	}
	if !found {
		t.Errorf("default model %q not in fallback list %v", copilotDefaultModel, caps.Models)
	}
}

func TestCopilotProvider_Capabilities_CachedAfterFirstCall(t *testing.T) {
	srv, calls := mockModelsServer(t, []string{"gpt-4o"})

	p := &CopilotProvider{
		token:   "ghp_test",
		model:   copilotDefaultModel,
		baseURL: srv.URL,
		client:  srv.Client(),
	}

	for i := 0; i < 3; i++ {
		_ = p.Capabilities()
	}
	if *calls != 1 {
		t.Errorf("/models called %d times, want exactly 1 (cached)", *calls)
	}
}

func TestCopilotProvider_Capabilities_FallbackOnNonOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	t.Cleanup(srv.Close)

	p := &CopilotProvider{
		token:   "ghp_bad",
		model:   copilotDefaultModel,
		baseURL: srv.URL,
		client:  srv.Client(),
	}

	caps := p.Capabilities()
	if len(caps.Models) == 0 {
		t.Fatal("expected fallback models on non-200 response")
	}
}

// -- Nil request guard --------------------------------------------------------

func TestCopilotProvider_NilRequest(t *testing.T) {
	p := &CopilotProvider{token: "ghp_test", model: copilotDefaultModel, baseURL: copilotAPIBase, client: &http.Client{}}
	_, err := p.Complete(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil request")
	}
	if !strings.Contains(err.Error(), "FORGE-4052") {
		t.Errorf("expected FORGE-4052 in error, got: %v", err)
	}
}

// -- gh config file parsing ---------------------------------------------------

func TestReadGHConfigToken_WellFormed(t *testing.T) {
	tmp := t.TempDir()
	hostsYML := "github.com:\n    oauth_token: ghp_from_file\n    user: testuser\n    git_protocol: https\n"
	if err := os.WriteFile(filepath.Join(tmp, "hosts.yml"), []byte(hostsYML), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GH_CONFIG_DIR", tmp)

	if got := readGHConfigToken(); got != "ghp_from_file" {
		t.Errorf("token = %q, want %q", got, "ghp_from_file")
	}
}

func TestReadGHConfigToken_MissingFile(t *testing.T) {
	t.Setenv("GH_CONFIG_DIR", filepath.Join(t.TempDir(), "noexist"))
	if got := readGHConfigToken(); got != "" {
		t.Errorf("expected empty token for missing file, got %q", got)
	}
}

func TestReadGHConfigToken_NoOAuthToken(t *testing.T) {
	tmp := t.TempDir()
	hostsYML := "github.com:\n    user: testuser\n    git_protocol: https\n"
	if err := os.WriteFile(filepath.Join(tmp, "hosts.yml"), []byte(hostsYML), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GH_CONFIG_DIR", tmp)

	if got := readGHConfigToken(); got != "" {
		t.Errorf("expected empty token when oauth_token absent, got %q", got)
	}
}

// -- Detect() integration -----------------------------------------------------

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
