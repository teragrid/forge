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

// Internal test file (package llmprovider) — has access to unexported symbols.
package llmprovider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// ── readClaudeCodeAPIKey ───────────────────────────────────────────────────────

func TestReadClaudeCodeAPIKey_WellFormed(t *testing.T) {
	dir := t.TempDir()
	claudeDir := filepath.Join(dir, ".claude")
	if err := os.MkdirAll(claudeDir, 0700); err != nil {
		t.Fatal(err)
	}
	cfg := map[string]string{"primaryApiKey": "sk-ant-test-from-claude-config"}
	data, _ := json.Marshal(cfg)
	if err := os.WriteFile(filepath.Join(claudeDir, "config.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", dir)        // Unix
	t.Setenv("USERPROFILE", dir) // Windows
	got := readClaudeCodeAPIKey()
	if got != "sk-ant-test-from-claude-config" {
		t.Errorf("got %q, want sk-ant-test-from-claude-config", got)
	}
}

func TestReadClaudeCodeAPIKey_MissingFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	got := readClaudeCodeAPIKey()
	if got != "" {
		t.Errorf("expected empty string for missing file, got %q", got)
	}
}

func TestReadClaudeCodeAPIKey_NoPrimaryApiKey(t *testing.T) {
	dir := t.TempDir()
	data := []byte(`{"otherField":"value"}`)
	claudeDir := filepath.Join(dir, ".claude")
	if err := os.MkdirAll(claudeDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(claudeDir, "config.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	got := readClaudeCodeAPIKey()
	if got != "" {
		t.Errorf("expected empty string when primaryApiKey absent, got %q", got)
	}
}

func TestReadClaudeCodeAPIKey_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	claudeDir := filepath.Join(dir, ".claude")
	if err := os.MkdirAll(claudeDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(claudeDir, "config.json"), []byte("not-json"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	got := readClaudeCodeAPIKey()
	if got != "" {
		t.Errorf("expected empty string for invalid JSON, got %q", got)
	}
}

// ── detectAnthropicKey priority ───────────────────────────────────────────────

func TestDetectAnthropicKey_EnvTakesPrecedenceOverClaudeConfig(t *testing.T) {
	// Cannot use t.Parallel() with t.Setenv for env vars that affect global state.
	dir := t.TempDir()
	claudeDir := filepath.Join(dir, ".claude")
	if err := os.MkdirAll(claudeDir, 0700); err != nil {
		t.Fatal(err)
	}
	cfg := map[string]string{"primaryApiKey": "sk-ant-from-claude-config"}
	data, _ := json.Marshal(cfg)
	if err := os.WriteFile(filepath.Join(claudeDir, "config.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	t.Setenv("ANTHROPIC_API_KEY", "sk-ant-from-env-var")

	got := detectAnthropicKey()
	if got != "sk-ant-from-env-var" {
		t.Errorf("ANTHROPIC_API_KEY should take precedence; got %q", got)
	}
}

func TestDetectAnthropicKey_FallsBackToClaudeConfig(t *testing.T) {
	dir := t.TempDir()
	claudeDir := filepath.Join(dir, ".claude")
	if err := os.MkdirAll(claudeDir, 0700); err != nil {
		t.Fatal(err)
	}
	cfg := map[string]string{"primaryApiKey": "sk-ant-from-claude-config"}
	data, _ := json.Marshal(cfg)
	if err := os.WriteFile(filepath.Join(claudeDir, "config.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	t.Setenv("ANTHROPIC_API_KEY", "") // explicitly unset

	got := detectAnthropicKey()
	if got != "sk-ant-from-claude-config" {
		t.Errorf("expected key from Claude Code config, got %q", got)
	}
}

// ── AnthropicAdapter.Complete — real HTTP ─────────────────────────────────────

func TestAnthropicAdapter_Complete_OK(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") == "" {
			http.Error(w, "missing key", http.StatusUnauthorized)
			return
		}
		if r.Header.Get("anthropic-version") == "" {
			http.Error(w, "missing version header", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"content": []map[string]any{{"type": "text", "text": "hello from claude"}},
			"usage":   map[string]any{"input_tokens": 12, "output_tokens": 4},
			"model":   "claude-sonnet-4-5-20250514",
		})
	}))
	defer srv.Close()

	a := &AnthropicAdapter{
		apiKey:  "sk-ant-test",
		baseURL: srv.URL,
		client:  srv.Client(),
	}
	resp, err := a.Complete(context.Background(), &Request{
		SystemPrompt: "you are a test assistant",
		UserPrompt:   "say hello",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "hello from claude" {
		t.Errorf("content = %q, want %q", resp.Content, "hello from claude")
	}
	if resp.InputTokens != 12 || resp.OutputTokens != 4 {
		t.Errorf("tokens = %d/%d, want 12/4", resp.InputTokens, resp.OutputTokens)
	}
	if resp.Model != "claude-sonnet-4-5-20250514" {
		t.Errorf("model = %q", resp.Model)
	}
}

func TestAnthropicAdapter_Complete_RespectsReqModel(t *testing.T) {
	t.Parallel()
	var gotModel string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Model string `json:"model"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		gotModel = body.Model
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"content": []map[string]any{{"type": "text", "text": "ok"}},
			"usage":   map[string]any{"input_tokens": 1, "output_tokens": 1},
			"model":   body.Model,
		})
	}))
	defer srv.Close()

	a := &AnthropicAdapter{apiKey: "sk-ant-test", baseURL: srv.URL, client: srv.Client()}
	_, err := a.Complete(context.Background(), &Request{
		UserPrompt: "hi",
		Model:      "claude-opus-4-8-20250514",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotModel != "claude-opus-4-8-20250514" {
		t.Errorf("model sent to API = %q, want claude-opus-4-8-20250514", gotModel)
	}
}

func TestAnthropicAdapter_Complete_HTTP401(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"invalid key"}`, http.StatusUnauthorized)
	}))
	defer srv.Close()

	a := &AnthropicAdapter{apiKey: "bad-key", baseURL: srv.URL, client: srv.Client()}
	_, err := a.Complete(context.Background(), &Request{UserPrompt: "hi"})
	if err == nil {
		t.Fatal("expected error for HTTP 401")
	}
	if !containsStr(err.Error(), "401") {
		t.Errorf("error should mention 401: %v", err)
	}
}

func TestAnthropicAdapter_Complete_NoKey(t *testing.T) {
	t.Parallel()
	a := &AnthropicAdapter{} // zero value — no API key
	_, err := a.Complete(context.Background(), &Request{UserPrompt: "hi"})
	if err == nil {
		t.Fatal("expected error with no API key")
	}
	// Should still be FORGE-4051.
	if !containsStr(err.Error(), "FORGE-4051") {
		t.Errorf("expected FORGE-4051, got: %v", err)
	}
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		}())
}
