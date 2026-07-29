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
			"model":   AnthropicDefaultModel,
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
	if resp.Model != AnthropicDefaultModel {
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
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
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

// ── stop_reason plumbing + cost-efficient max_tokens continuation ─────────────
//
// Test Design (root cause: checkSpec/checkArch requested completions with
// MaxTokens well below what the prompted document shape needs, and forge had
// no visibility into *why* a response was cut short — attemptComplete parsed
// the Anthropic response but discarded stop_reason entirely. The downstream
// truncation heuristic (artefact_validate.go's looksComplete) then retried at
// the *same* budget, so a genuinely budget-truncated response failed
// identically on retry. The fix must ALSO not make cost worse — forge ship's
// whole point is to control LLM spend — so truncation recovery continues the
// existing (already-paid-for) output via an assistant-prefill turn instead of
// throwing it away and re-generating the entire document from scratch.):
//  1. Happy path — stop_reason "end_turn" is surfaced on Response, no continuation
//  2. Happy path — stop_reason "max_tokens" triggers a continuation call that
//     replays the truncated text as an assistant turn, and the concatenated
//     (first + continuation) content is returned
//  3. Cost — the continuation call's message list is exactly [user, assistant
//     prefill] and uses the SAME MaxTokens as the original (never doubled/
//     inflated) — the remainder needed is typically small, not a full budget
//  4. Cost — token counts are summed across both calls, so the ledger sees
//     the true total cost rather than only the (larger) final call
//  5. Negative — the continuation itself also hits max_tokens → the
//     concatenated (still-truncated) response is returned rather than erroring
//  6. Negative — MaxTokens==0 (provider default) → no continuation attempted
//     (no budget context to replay against)
//  7. Idempotency — a stop_reason "end_turn" response is never continued
//  8. False-positive guard — a non-"max_tokens" stop_reason (e.g.
//     "stop_sequence") never triggers a continuation
//  9. Boundary — first.Content == "" (nothing to prefill) → no continuation

func TestAnthropicAdapter_Complete_SurfacesTruncated(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"content":     []map[string]any{{"type": "text", "text": "complete answer."}},
			"usage":       map[string]any{"input_tokens": 10, "output_tokens": 5},
			"model":       AnthropicDefaultModel,
			"stop_reason": "end_turn",
		})
	}))
	defer srv.Close()

	a := &AnthropicAdapter{apiKey: "sk-ant-test", baseURL: srv.URL, client: srv.Client()}
	resp, err := a.Complete(context.Background(), &Request{UserPrompt: "hi", MaxTokens: 100})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Truncated {
		t.Error("Truncated = true, want false for stop_reason \"end_turn\"")
	}
	if resp.Content != "complete answer." {
		t.Errorf("content = %q", resp.Content)
	}
}

func TestAnthropicAdapter_Complete_ContinuesOnMaxTokens(t *testing.T) {
	t.Parallel()
	var requests []struct {
		MaxTokens int
		Messages  []anthropicMessage
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			MaxTokens int                `json:"max_tokens"`
			Messages  []anthropicMessage `json:"messages"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		requests = append(requests, struct {
			MaxTokens int
			Messages  []anthropicMessage
		}{body.MaxTokens, body.Messages})

		w.Header().Set("Content-Type", "application/json")
		if len(requests) == 1 {
			// First attempt: cut off by the token budget.
			_ = json.NewEncoder(w).Encode(map[string]any{
				"content":     []map[string]any{{"type": "text", "text": "this response got cut off mid"}},
				"usage":       map[string]any{"input_tokens": 10, "output_tokens": 100},
				"model":       AnthropicDefaultModel,
				"stop_reason": "max_tokens",
			})
			return
		}
		// Continuation: finishes the remainder naturally.
		_ = json.NewEncoder(w).Encode(map[string]any{
			"content":     []map[string]any{{"type": "text", "text": "-sentence, now finished."}},
			"usage":       map[string]any{"input_tokens": 120, "output_tokens": 8},
			"model":       AnthropicDefaultModel,
			"stop_reason": "end_turn",
		})
	}))
	defer srv.Close()

	a := &AnthropicAdapter{apiKey: "sk-ant-test", baseURL: srv.URL, client: srv.Client()}
	resp, err := a.Complete(context.Background(), &Request{UserPrompt: "hi", MaxTokens: 100})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(requests) != 2 {
		t.Fatalf("expected 2 requests (original + continuation), got %d", len(requests))
	}
	// 3. Cost — same MaxTokens on the continuation, never inflated.
	if requests[1].MaxTokens != 100 {
		t.Errorf("continuation MaxTokens = %d, want 100 (same as original, not doubled)", requests[1].MaxTokens)
	}
	// 3. Cost — the continuation replays [user, assistant-prefill], not a
	// fresh single-user-turn request (which would re-ask from scratch).
	if len(requests[1].Messages) != 2 {
		t.Fatalf("continuation should send 2 messages (user + assistant prefill), got %d", len(requests[1].Messages))
	}
	if requests[1].Messages[0].Role != "user" || requests[1].Messages[0].Content != "hi" {
		t.Errorf("continuation messages[0] = %+v, want the original user prompt", requests[1].Messages[0])
	}
	if requests[1].Messages[1].Role != "assistant" || requests[1].Messages[1].Content != "this response got cut off mid" {
		t.Errorf("continuation messages[1] = %+v, want the truncated text replayed as an assistant turn", requests[1].Messages[1])
	}
	// 2. The final content is the concatenation, not just the continuation.
	want := "this response got cut off mid" + "-sentence, now finished."
	if resp.Content != want {
		t.Errorf("content = %q, want %q", resp.Content, want)
	}
	// 4. Cost — token usage is summed across both calls.
	if resp.InputTokens != 130 {
		t.Errorf("InputTokens = %d, want 130 (10 + 120, summed across both calls)", resp.InputTokens)
	}
	if resp.OutputTokens != 108 {
		t.Errorf("OutputTokens = %d, want 108 (100 + 8, summed across both calls)", resp.OutputTokens)
	}
	if resp.Truncated {
		t.Error("Truncated = true, want false — continuation finished with stop_reason \"end_turn\"")
	}
}

func TestAnthropicAdapter_Complete_ContinuationStillTruncated_ReturnsConcatenated(t *testing.T) {
	t.Parallel()
	var requests int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Content-Type", "application/json")
		// Every attempt (original + continuation) is still cut off.
		text := "chunk"
		if requests > 1 {
			text = "-more"
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"content":     []map[string]any{{"type": "text", "text": text}},
			"usage":       map[string]any{"input_tokens": 10, "output_tokens": 100},
			"model":       AnthropicDefaultModel,
			"stop_reason": "max_tokens",
		})
	}))
	defer srv.Close()

	a := &AnthropicAdapter{apiKey: "sk-ant-test", baseURL: srv.URL, client: srv.Client()}
	resp, err := a.Complete(context.Background(), &Request{UserPrompt: "hi", MaxTokens: 100})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if requests != 2 {
		t.Fatalf("expected original + one continuation attempt, got %d requests", requests)
	}
	// 5. Negative — the continuation also truncated; we still return the
	// concatenated (still-truncated) response — the caller's own J8/J9
	// completeness check decides what to do with it — rather than erroring.
	if resp.Content != "chunk-more" {
		t.Errorf("content = %q, want the concatenated (still-truncated) response body", resp.Content)
	}
	if !resp.Truncated {
		t.Error("Truncated = false, want true — still cut off after the continuation attempt")
	}
}

func TestAnthropicAdapter_Complete_NoContinuationWhenMaxTokensZero(t *testing.T) {
	t.Parallel()
	var requests int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"content":     []map[string]any{{"type": "text", "text": "cut off"}},
			"usage":       map[string]any{"input_tokens": 10, "output_tokens": 8096},
			"model":       AnthropicDefaultModel,
			"stop_reason": "max_tokens",
		})
	}))
	defer srv.Close()

	a := &AnthropicAdapter{apiKey: "sk-ant-test", baseURL: srv.URL, client: srv.Client()}
	// 6. Boundary — MaxTokens==0 means "provider default"; there is no
	// caller-specified budget context to replay, so no continuation should
	// be attempted.
	_, err := a.Complete(context.Background(), &Request{UserPrompt: "hi"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if requests != 1 {
		t.Errorf("expected exactly 1 request when MaxTokens==0, got %d", requests)
	}
}

func TestAnthropicAdapter_Complete_NoContinuationOnOtherStopReasons(t *testing.T) {
	t.Parallel()
	var requests int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"content":     []map[string]any{{"type": "text", "text": "stopped on a sequence"}},
			"usage":       map[string]any{"input_tokens": 10, "output_tokens": 20},
			"model":       AnthropicDefaultModel,
			"stop_reason": "stop_sequence",
		})
	}))
	defer srv.Close()

	a := &AnthropicAdapter{apiKey: "sk-ant-test", baseURL: srv.URL, client: srv.Client()}
	// 8. False-positive guard — "stop_sequence" is a normal, intentional
	// completion shape, not a truncation signal; must never trigger a
	// continuation call (which would waste a full extra request).
	_, err := a.Complete(context.Background(), &Request{UserPrompt: "hi", MaxTokens: 100})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if requests != 1 {
		t.Errorf("expected exactly 1 request for stop_reason=stop_sequence, got %d", requests)
	}
}

// 7. Idempotency — TestAnthropicAdapter_Complete_OK (existing test above)
// already asserts a single successful end_turn call returns cleanly; calling
// continueOnMaxTokens directly on an end_turn response is also covered here
// as a unit-level guard against a future regression that removes the
// stop_reason check.
func TestContinueOnMaxTokens_NoOpForEndTurn(t *testing.T) {
	t.Parallel()
	a := &AnthropicAdapter{apiKey: "sk-ant-test"}
	first := &Response{Content: "done", Truncated: false}
	got := a.continueOnMaxTokens(context.Background(), &Request{MaxTokens: 100}, AnthropicDefaultModel, first)
	if got != first {
		t.Error("continueOnMaxTokens should return the same pointer unchanged for a non-truncated response")
	}
}

// 9. Boundary — nothing to prefill.
func TestContinueOnMaxTokens_NoOpForEmptyContent(t *testing.T) {
	t.Parallel()
	a := &AnthropicAdapter{apiKey: "sk-ant-test"}
	first := &Response{Content: "", Truncated: true}
	got := a.continueOnMaxTokens(context.Background(), &Request{MaxTokens: 100}, AnthropicDefaultModel, first)
	if got != first {
		t.Error("continueOnMaxTokens should return the same pointer unchanged when there is no content to prefill")
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
