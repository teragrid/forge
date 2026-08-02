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
	// model is left empty at construction when FORGE_COPILOT_MODEL is unset —
	// resolveModel() fills it in lazily from the live catalog (or
	// copilotDefaultModel as a last resort) on first use, rather than baking
	// in a possibly-stale default here. See TestCopilotProvider_ResolveModel*
	// for the resolution behavior itself.
	if p.model != "" {
		t.Errorf("model = %q, want empty (unresolved until first use)", p.model)
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
	// Redirect HOME so detectAnthropicKey cannot read ~/.claude/config.json on this machine.
	emptyHome := t.TempDir()
	t.Setenv("HOME", emptyHome)
	t.Setenv("USERPROFILE", emptyHome)
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

// -- pickDefaultModel / resolveModel (2026-08-02: stale-default regression) --
//
// Regression coverage for a real incident: the previous hardcoded
// copilotDefaultModel ("claude-sonnet-4-5-20250514") had gone dead on
// GitHub's Copilot backend. Unlike the /chat/completions HTTP-400 case
// already covered above, an unknown *default* model doesn't error at all —
// the API silently substitutes a different model (observed: gpt-4.1, whose
// non-streaming output got cut short well under the requested token budget
// on real prompts) and returns HTTP 200, so the existing fallback-on-400
// path never even triggers. pickDefaultModel/resolveModel close this gap by
// preferring the live /models catalog over any hardcoded constant whenever
// it's reachable.

func TestPickDefaultModel_PrefersEnabledAnthropicSonnet(t *testing.T) {
	infos := []CopilotModelInfo{
		{ID: "gpt-4.1", Vendor: "Azure OpenAI", Enabled: true, PickerEnabled: true},
		{ID: "claude-opus-4.5", Vendor: "Anthropic", Enabled: true, PickerEnabled: true},
		{ID: "claude-sonnet-5", Vendor: "Anthropic", Enabled: true, PickerEnabled: true},
	}
	got := pickDefaultModel(infos, "fallback")
	if got != "claude-sonnet-5" {
		t.Errorf("pickDefaultModel() = %q, want %q (enabled Anthropic sonnet preferred)", got, "claude-sonnet-5")
	}
}

// TestPickDefaultModel_PrefersHighestVersion_NotFirstInResponseOrder locks a
// real observed shape: GitHub's /models response order is not sorted by
// version (e.g. "claude-sonnet-4.6" appeared before "claude-sonnet-5" in a
// real catalog snapshot on 2026-08-02). Picking the first match in response
// order would silently default to an older Sonnet even when a newer one is
// available and enabled.
func TestPickDefaultModel_PrefersHighestVersion_NotFirstInResponseOrder(t *testing.T) {
	infos := []CopilotModelInfo{
		{ID: "claude-sonnet-4.6", Vendor: "Anthropic", Enabled: true, PickerEnabled: true},
		{ID: "claude-sonnet-4.5", Vendor: "Anthropic", Enabled: true, PickerEnabled: true},
		{ID: "claude-sonnet-5", Vendor: "Anthropic", Enabled: true, PickerEnabled: true},
	}
	got := pickDefaultModel(infos, "fallback")
	if got != "claude-sonnet-5" {
		t.Errorf("pickDefaultModel() = %q, want %q (highest version, regardless of catalog order)", got, "claude-sonnet-5")
	}
}

func TestModelVersionRank(t *testing.T) {
	cases := []struct {
		id   string
		want float64
	}{
		{"claude-sonnet-5", 5},
		{"claude-sonnet-4.6", 4.6},
		{"gpt-5.6-luna", 5.6},
		{"claude-opus-4.8-fast", 4.8},
		{"text-embedding-3-small", 3},
		{"no-version-here", 0},
	}
	for _, c := range cases {
		if got := modelVersionRank(c.id); got != c.want {
			t.Errorf("modelVersionRank(%q) = %v, want %v", c.id, got, c.want)
		}
	}
}

func TestPickDefaultModel_SkipsDisabledSonnet_FallsBackToEnabledAnthropic(t *testing.T) {
	infos := []CopilotModelInfo{
		// Present in the catalog but gated off for this account — must not be picked.
		{ID: "claude-sonnet-5", Vendor: "Anthropic", Enabled: false, PickerEnabled: true},
		{ID: "claude-opus-4.5", Vendor: "Anthropic", Enabled: true, PickerEnabled: true},
	}
	got := pickDefaultModel(infos, "fallback")
	if got != "claude-opus-4.5" {
		t.Errorf("pickDefaultModel() = %q, want %q (falls back to any enabled Anthropic model)", got, "claude-opus-4.5")
	}
}

func TestPickDefaultModel_NoAnthropicAvailable_FallsBackToAnyEnabled(t *testing.T) {
	infos := []CopilotModelInfo{
		{ID: "claude-sonnet-5", Vendor: "Anthropic", Enabled: false, PickerEnabled: true},
		{ID: "gpt-5.4", Vendor: "Azure OpenAI", Enabled: true, PickerEnabled: true},
	}
	got := pickDefaultModel(infos, "fallback")
	if got != "gpt-5.4" {
		t.Errorf("pickDefaultModel() = %q, want %q (falls back to any enabled model)", got, "gpt-5.4")
	}
}

func TestPickDefaultModel_NothingEnabled_FallsBackToFirstEntry(t *testing.T) {
	infos := []CopilotModelInfo{
		{ID: "claude-opus-5", Vendor: "Anthropic", Enabled: false, PickerEnabled: true},
		{ID: "gpt-5.5", Vendor: "Azure OpenAI", Enabled: false, PickerEnabled: true},
	}
	got := pickDefaultModel(infos, "fallback")
	if got != "claude-opus-5" {
		t.Errorf("pickDefaultModel() = %q, want %q (first entry when nothing is usable)", got, "claude-opus-5")
	}
}

func TestPickDefaultModel_PickerDisabled_TreatedAsUnusable(t *testing.T) {
	infos := []CopilotModelInfo{
		{ID: "claude-sonnet-5", Vendor: "Anthropic", Enabled: true, PickerEnabled: false},
		{ID: "gpt-5.4", Vendor: "Azure OpenAI", Enabled: true, PickerEnabled: true},
	}
	got := pickDefaultModel(infos, "fallback")
	if got != "gpt-5.4" {
		t.Errorf("pickDefaultModel() = %q, want %q (picker-disabled Anthropic model skipped)", got, "gpt-5.4")
	}
}

func TestPickDefaultModel_EmptyCatalog_ReturnsStaticFallback(t *testing.T) {
	got := pickDefaultModel(nil, "static-fallback")
	if got != "static-fallback" {
		t.Errorf("pickDefaultModel(nil) = %q, want the static fallback unchanged", got)
	}
}

// mockRichModelsServer is like mockModelsServer but includes vendor/policy/
// model_picker_enabled, the fields pickDefaultModel actually needs.
func mockRichModelsServer(t *testing.T, infos []CopilotModelInfo) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/models" {
			http.NotFound(w, r)
			return
		}
		type policy struct {
			State string `json:"state"`
		}
		type model struct {
			ID                 string  `json:"id"`
			Vendor             string  `json:"vendor"`
			ModelPickerEnabled bool    `json:"model_picker_enabled"`
			Policy             *policy `json:"policy,omitempty"`
		}
		data := make([]model, 0, len(infos))
		for _, info := range infos {
			m := model{ID: info.ID, Vendor: info.Vendor, ModelPickerEnabled: info.PickerEnabled}
			state := "disabled"
			if info.Enabled {
				state = "enabled"
			}
			m.Policy = &policy{State: state}
			data = append(data, m)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"object": "list", "data": data})
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestCopilotProvider_ResolveModel_UsesLiveCatalogWhenNoOverride(t *testing.T) {
	srv := mockRichModelsServer(t, []CopilotModelInfo{
		{ID: "gpt-4.1", Vendor: "Azure OpenAI", Enabled: true, PickerEnabled: true},
		{ID: "claude-sonnet-5", Vendor: "Anthropic", Enabled: true, PickerEnabled: true},
	})
	p := &CopilotProvider{token: "ghp_test", baseURL: srv.URL, client: srv.Client()} // model left empty: no override
	if got := p.resolveModel(); got != "claude-sonnet-5" {
		t.Errorf("resolveModel() = %q, want %q (live catalog preferred over static default)", got, "claude-sonnet-5")
	}
}

func TestCopilotProvider_ResolveModel_ExplicitOverrideWinsOverLiveCatalog(t *testing.T) {
	srv := mockRichModelsServer(t, []CopilotModelInfo{
		{ID: "claude-sonnet-5", Vendor: "Anthropic", Enabled: true, PickerEnabled: true},
	})
	p := &CopilotProvider{token: "ghp_test", model: "gpt-4o", baseURL: srv.URL, client: srv.Client()}
	if got := p.resolveModel(); got != "gpt-4o" {
		t.Errorf("resolveModel() = %q, want explicit override %q (must not fetch or override it)", got, "gpt-4o")
	}
}

func TestCopilotProvider_ResolveModel_FallsBackToStaticWhenCatalogUnreachable(t *testing.T) {
	p := &CopilotProvider{token: "ghp_test", baseURL: "http://127.0.0.1:1", client: &http.Client{}}
	if got := p.resolveModel(); got != copilotDefaultModel {
		t.Errorf("resolveModel() = %q, want static fallback %q when /models is unreachable", got, copilotDefaultModel)
	}
}

func TestCopilotProvider_ResolveModel_CachedAcrossCalls(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"object": "list", "data": []map[string]any{
			{"id": "claude-sonnet-5", "vendor": "Anthropic", "model_picker_enabled": true, "policy": map[string]string{"state": "enabled"}},
		}})
	}))
	t.Cleanup(srv.Close)

	p := &CopilotProvider{token: "ghp_test", baseURL: srv.URL, client: srv.Client()}
	for i := 0; i < 3; i++ {
		if got := p.resolveModel(); got != "claude-sonnet-5" {
			t.Errorf("call %d: resolveModel() = %q, want %q", i, got, "claude-sonnet-5")
		}
	}
	if calls != 1 {
		t.Errorf("/models called %d times, want exactly 1 (resolveOnce should cache)", calls)
	}
}

// -- Complete() model-unavailable fallback -------------------------------------
//
// Regression coverage for a live incident (2026-07-12): a stale forge.yml
// llm.model value naming a model GitHub Copilot's /models endpoint listed but
// /chat/completions rejected (HTTP 400) caused every forge ship LLM call to
// fail outright, because req.Model being non-empty (merely a soft config
// default applied by profileProvider.Complete, not a genuine per-call
// requirement) was mistaken for pinned user intent and suppressed the
// built-in fallback-to-copilotKnownModels recovery path entirely.

// mockChatServer returns a /chat/completions handler that returns HTTP 400
// "model unavailable" for any model in badModels, and a successful completion
// for any other model.
func mockChatServer(t *testing.T, badModels map[string]bool) (*httptest.Server, *[]string) {
	t.Helper()
	var seen []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/chat/completions" {
			http.NotFound(w, r)
			return
		}
		var body struct {
			Model string `json:"model"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		seen = append(seen, body.Model)
		if badModels[body.Model] {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":{"message":"The requested model is not supported."}}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"model":   body.Model,
			"choices": []map[string]any{{"message": map[string]string{"content": "ok"}}},
			"usage":   map[string]int{"prompt_tokens": 1, "completion_tokens": 1},
		})
	}))
	t.Cleanup(srv.Close)
	return srv, &seen
}

func TestCopilotProvider_Complete_FallsBackWhenModelUnpinned(t *testing.T) {
	// req.Model is set (as profileProvider.Complete would do from a stale
	// forge.yml llm.model default) but ModelPinned is false -- the fallback
	// to copilotKnownModels[0] must still run and succeed.
	badPrimary := copilotKnownModels[0] + "-does-not-exist"
	srv, seen := mockChatServer(t, map[string]bool{badPrimary: true})

	p := &CopilotProvider{
		token:   "ghp_test",
		model:   copilotDefaultModel,
		baseURL: srv.URL,
		client:  srv.Client(),
	}

	resp, err := p.Complete(context.Background(), &Request{
		Model:       badPrimary,
		ModelPinned: false,
		UserPrompt:  "hi",
	})
	if err != nil {
		t.Fatalf("Complete() error = %v, want fallback to succeed", err)
	}
	if resp.Content != "ok" {
		t.Errorf("Content = %q, want %q", resp.Content, "ok")
	}
	if len(*seen) < 2 {
		t.Fatalf("expected at least 2 attempts (primary + fallback), got %v", *seen)
	}
	if (*seen)[0] != badPrimary {
		t.Errorf("first attempt model = %q, want %q", (*seen)[0], badPrimary)
	}
}

func TestCopilotProvider_Complete_DoesNotFallBackWhenModelPinned(t *testing.T) {
	// ModelPinned=true means the caller genuinely requires this exact model
	// -- must fail fast with an actionable error, not silently substitute.
	badPrimary := "some-pinned-model"
	srv, seen := mockChatServer(t, map[string]bool{badPrimary: true})

	p := &CopilotProvider{
		token:   "ghp_test",
		model:   copilotDefaultModel,
		baseURL: srv.URL,
		client:  srv.Client(),
	}

	_, err := p.Complete(context.Background(), &Request{
		Model:       badPrimary,
		ModelPinned: true,
		UserPrompt:  "hi",
	})
	if err == nil {
		t.Fatal("Complete() error = nil, want an error (pinned model unavailable)")
	}
	if !strings.Contains(err.Error(), badPrimary) {
		t.Errorf("error = %v, want it to mention the pinned model %q", err, badPrimary)
	}
	if len(*seen) != 1 {
		t.Errorf("expected exactly 1 attempt (no fallback for a pinned model), got %v", *seen)
	}
}

func TestCopilotProvider_Complete_SucceedsOnFirstTry(t *testing.T) {
	srv, seen := mockChatServer(t, nil)
	p := &CopilotProvider{
		token:   "ghp_test",
		model:   copilotDefaultModel,
		baseURL: srv.URL,
		client:  srv.Client(),
	}
	resp, err := p.Complete(context.Background(), &Request{UserPrompt: "hi"})
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if resp.Content != "ok" {
		t.Errorf("Content = %q, want %q", resp.Content, "ok")
	}
	if len(*seen) != 1 {
		t.Errorf("expected exactly 1 attempt on success, got %v", *seen)
	}
}

// TestProfileProvider_ConfigModel_NotPinned locks the actual root-cause fix:
// profileProvider.Complete fills Request.Model from forge.yml's llm.model
// (configModel) as a soft default, and must NOT also set ModelPinned -- doing
// so would re-introduce the exact live incident this test guards against
// (a stale configured model permanently defeating CopilotProvider's
// fallback-to-copilotKnownModels recovery).
func TestProfileProvider_ConfigModel_NotPinned(t *testing.T) {
	var received *Request
	inner := &MockProvider{Fn: func(r *Request) (*Response, error) {
		received = r
		return &Response{Content: "ok"}, nil
	}}
	p := &profileProvider{inner: inner, configModel: "claude-sonnet-4-6"}

	_, err := p.Complete(context.Background(), &Request{UserPrompt: "hi"})
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if received == nil {
		t.Fatal("inner provider did not receive a request")
	}
	if received.Model != "claude-sonnet-4-6" {
		t.Errorf("Model = %q, want configModel %q", received.Model, "claude-sonnet-4-6")
	}
	if received.ModelPinned {
		t.Error("ModelPinned = true, want false — a config-file default must not suppress fallback")
	}
}

// TestProfileProvider_ConfigModel_DoesNotOverrideExplicitModel confirms a
// caller-supplied Model (e.g. from tierrouter's own tier selection) still
// wins over the config default, and its ModelPinned value passes through
// unchanged (profileProvider must not touch it either way).
func TestProfileProvider_ConfigModel_DoesNotOverrideExplicitModel(t *testing.T) {
	var received *Request
	inner := &MockProvider{Fn: func(r *Request) (*Response, error) {
		received = r
		return &Response{Content: "ok"}, nil
	}}
	p := &profileProvider{inner: inner, configModel: "claude-sonnet-4-6"}

	_, err := p.Complete(context.Background(), &Request{
		Model:       "claude-opus-4-8-20250514",
		ModelPinned: true,
		UserPrompt:  "hi",
	})
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if received.Model != "claude-opus-4-8-20250514" {
		t.Errorf("Model = %q, want caller's explicit model unchanged", received.Model)
	}
	if !received.ModelPinned {
		t.Error("ModelPinned = false, want true — caller's own value must pass through unchanged")
	}
}
