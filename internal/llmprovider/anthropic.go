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

// Package llmprovider — anthropic.go implements the Anthropic Claude API adapter.
//
// Activated when an Anthropic API key is available via (in priority order):
//  1. ANTHROPIC_API_KEY environment variable
//  2. ~/.claude/config.json primaryApiKey (Claude Code CLI — no extra setup needed)
//
// The second source means users who have Claude Code installed and authenticated
// can run forge commands without setting any extra environment variable — the same
// credential that powers their Claude Code session is reused automatically.
//
// Default model: AnthropicDefaultModel (override via ANTHROPIC_MODEL or
// Request.Model / forge.yml llm.model). AnthropicDefaultModel is the single
// source of truth for the default Anthropic model id — see J1/J2
// (fix-checkpoint-llm-quality-and-observability): a stale hardcoded model id
// duplicated across 5 call sites previously caused every Anthropic call to
// 404. Live model discovery (loadModels, mirroring CopilotProvider) and
// automatic fallback-on-404 retry (dynamic-fault-tolerant-model-selection)
// make this class of bug self-healing going forward.
package llmprovider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/teragrid/forge/internal/errcode"
)

const (
	anthropicAPIBase    = "https://api.anthropic.com/v1"
	anthropicAPIVersion = "2023-06-01"

	// AnthropicDefaultModel is the single source of truth for the default
	// Anthropic model id used when no explicit model is configured. Updated
	// 2026-07-14: confirmed operational via `forge doctor --llm` returning
	// HTTP 429 (rate-limited, not 404) — the prior id
	// (claude-sonnet-4-5-20250514, added v1.7.1) was silently deprecated
	// upstream and 404'd on every call. Every other package that previously
	// hardcoded its own copy of this literal (anthropic_test.go, llmpipe.go,
	// tierrouter.go) now references this constant instead (J1).
	AnthropicDefaultModel = "claude-sonnet-5"
)

// anthropicKnownModels is the static fallback model list used when live
// discovery (loadModels) is unreachable and no cached discovery result
// exists — mirrors CopilotProvider's copilotKnownModels. Also used as the
// fallback-retry ladder when the primary model 404s
// (dynamic-fault-tolerant-model-selection AC2).
var anthropicKnownModels = []string{
	AnthropicDefaultModel,
	"claude-opus-4-8-20250514",
	"claude-haiku-4-5-20251001",
	"claude-3-7-sonnet-20250219",
	"claude-3-5-sonnet-20241022",
	"claude-3-5-haiku-20241022",
}

// AnthropicAdapter implements Provider using the Anthropic Claude API.
type AnthropicAdapter struct {
	apiKey string
	model  string
	// pinned is true when model came from an explicit ANTHROPIC_MODEL env var
	// (as opposed to falling back to AnthropicDefaultModel) — mirrors
	// Request.ModelPinned: an explicit pin disables silent fallback
	// substitution on a 404 (dynamic-fault-tolerant-model-selection AC4).
	pinned  bool
	baseURL string       // empty → anthropicAPIBase; overridable in tests
	client  *http.Client // nil → http.DefaultClient

	cachedModels []string
	modelsOnce   sync.Once
}

// newAnthropicProvider returns an AnthropicAdapter when an API key can be found,
// nil otherwise.  Key sources are checked in priority order by detectAnthropicKey.
func newAnthropicProvider() *AnthropicAdapter {
	key := detectAnthropicKey()
	if key == "" {
		return nil
	}
	model := os.Getenv("ANTHROPIC_MODEL")
	pinned := model != ""
	if model == "" {
		model = AnthropicDefaultModel
	}
	return &AnthropicAdapter{apiKey: key, model: model, pinned: pinned}
}

// detectAnthropicKey returns the first Anthropic API key found from:
//  1. ANTHROPIC_API_KEY environment variable
//  2. ~/.claude/config.json primaryApiKey (Claude Code CLI credential store)
func detectAnthropicKey() string {
	if k := os.Getenv("ANTHROPIC_API_KEY"); k != "" {
		return k
	}
	return readClaudeCodeAPIKey()
}

// readClaudeCodeAPIKey reads the primaryApiKey from Claude Code's config.json.
// Claude Code stores its API key at ~/.claude/config.json, which is a plain
// JSON file (not the OAuth credential store) and safe to read without elevated
// permissions.
func readClaudeCodeAPIKey() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(home, ".claude", "config.json")) //nolint:gosec
	if err != nil {
		return ""
	}
	var cfg struct {
		PrimaryAPIKey string `json:"primaryApiKey"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return ""
	}
	return cfg.PrimaryAPIKey
}

// ── Provider interface ─────────────────────────────────────────────────────────

func (a *AnthropicAdapter) Name() string { return "anthropic" }

// Capabilities fetches the live model list from the Anthropic /v1/models
// endpoint on first call (cached via sync.Once, mirroring CopilotProvider),
// falling back to a fresh on-disk cache, then a stale on-disk cache, then the
// hardcoded anthropicKnownModels list if discovery has never succeeded
// (dynamic-fault-tolerant-model-selection AC5/AC6).
func (a *AnthropicAdapter) Capabilities() Capabilities {
	a.modelsOnce.Do(a.loadModels)
	models := a.cachedModels
	if len(models) == 0 {
		models = anthropicKnownModels
	}
	return Capabilities{
		Streaming: true,
		MaxTokens: 200000,
		Models:    models,
	}
}

// APIKey returns the raw API key (for callers that need it directly).
func (a *AnthropicAdapter) APIKey() string { return a.apiKey }

// anthropicAttemptResult is the outcome of one Complete() attempt against a
// single model.
type anthropicAttemptResult struct {
	resp      *Response
	retryable bool // true only for a not_found_error (model gone) — safe to retry a different model
	err       error
}

// Complete sends a chat request to the Anthropic Messages API and returns the
// assistant's response. On a not_found_error (model deprecated/unknown) it
// automatically retries once against a different, currently-valid model —
// mirroring CopilotProvider.Complete's attempt()/fallback-list structure —
// unless the caller (or ANTHROPIC_MODEL/forge.yml) explicitly pinned a model,
// in which case it fails loudly naming the dead model id instead of silently
// substituting (dynamic-fault-tolerant-model-selection AC2-AC4).
func (a *AnthropicAdapter) Complete(ctx context.Context, req *Request) (*Response, error) {
	if req == nil {
		return nil, errcode.New(ErrInvalidInput, "request must not be nil", nil)
	}
	if a.apiKey == "" {
		return nil, errcode.New(ErrProviderFail,
			"anthropic: no API key — set ANTHROPIC_API_KEY or authenticate with Claude Code CLI", nil)
	}

	primaryModel := a.model
	if primaryModel == "" {
		primaryModel = AnthropicDefaultModel
	}
	pinned := a.pinned
	if req.Model != "" {
		primaryModel = req.Model
		pinned = req.ModelPinned
	}

	attempt := func(model string) anthropicAttemptResult {
		resp, retryable, err := a.attemptComplete(ctx, req, model)
		return anthropicAttemptResult{resp: resp, retryable: retryable, err: err}
	}

	first := attempt(primaryModel)
	if !first.retryable {
		return first.resp, first.err
	}

	if pinned {
		return nil, fmt.Errorf(
			"anthropic: model %q not found (HTTP 404) — this model is explicitly pinned "+
				"(forge.yml llm.model / ANTHROPIC_MODEL), so forge will NOT silently substitute a "+
				"different model. Remove the pin to enable auto-fallback, or update it to a "+
				"currently-valid model id (e.g. %s): %w", primaryModel, AnthropicDefaultModel, first.err)
	}

	// Fallback: iterate the discovered + known-good model list, skipping the
	// primary model (already tried) and any duplicates.
	tried := map[string]bool{primaryModel: true}
	lastErr := first.err
	for _, fallback := range a.Capabilities().Models {
		if tried[fallback] {
			continue
		}
		tried[fallback] = true
		res := attempt(fallback)
		if !res.retryable {
			if res.err == nil {
				fmt.Fprintf(os.Stderr,
					"forge: anthropic model %q unavailable (404) — fell back to %q\n", primaryModel, fallback)
			}
			return res.resp, res.err
		}
		lastErr = res.err
	}
	return nil, fmt.Errorf(
		"anthropic: no available model; tried %q and all fallbacks — last error: %w", primaryModel, lastErr)
}

// attemptComplete sends one request with the given model. retryable is true
// only when the response is classified as a not_found_error (AC1/AC2) — the
// only failure mode safe to retry against a different model.
func (a *AnthropicAdapter) attemptComplete(ctx context.Context, req *Request, model string) (resp *Response, retryable bool, err error) {
	// Build Messages API request body.
	type message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	type requestBody struct {
		Model     string    `json:"model"`
		MaxTokens int       `json:"max_tokens"`
		System    string    `json:"system,omitempty"`
		Messages  []message `json:"messages"`
	}

	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 8096
	}

	body := requestBody{
		Model:     model,
		MaxTokens: maxTokens,
		System:    req.SystemPrompt,
		Messages:  []message{{Role: "user", Content: req.UserPrompt}},
	}

	raw, err := json.Marshal(body)
	if err != nil {
		return nil, false, fmt.Errorf("anthropic: marshal request: %w", err)
	}

	baseURL := a.baseURL
	if baseURL == "" {
		baseURL = anthropicAPIBase
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/messages", bytes.NewReader(raw))
	if err != nil {
		return nil, false, fmt.Errorf("anthropic: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", a.apiKey)
	httpReq.Header.Set("anthropic-version", anthropicAPIVersion)

	client := a.client
	if client == nil {
		client = http.DefaultClient
	}
	httpResp, err := client.Do(httpReq)
	if err != nil {
		return nil, false, fmt.Errorf("anthropic: http: %w", err)
	}
	defer httpResp.Body.Close() //nolint:errcheck

	respData, err := io.ReadAll(io.LimitReader(httpResp.Body, 1<<20)) // 1 MB max
	if err != nil {
		return nil, false, fmt.Errorf("anthropic: read response: %w", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		cerr := classifyAnthropicError(httpResp.StatusCode, model, respData)
		return nil, cerr.Code == ErrModelNotFound, cerr
	}

	// Parse Messages API response.
	var ar struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
		Model      string `json:"model"`
		StopReason string `json:"stop_reason"`
	}
	if err := json.Unmarshal(respData, &ar); err != nil {
		return nil, false, fmt.Errorf("anthropic: parse response: %w", err)
	}

	var content string
	for _, block := range ar.Content {
		if block.Type == "text" {
			content += block.Text
		}
	}
	if content == "" {
		return nil, false, fmt.Errorf("anthropic: empty response from API")
	}

	return &Response{
		Content:      content,
		InputTokens:  ar.Usage.InputTokens,
		OutputTokens: ar.Usage.OutputTokens,
		Model:        ar.Model,
		Truncated:    ar.StopReason == "max_tokens",
	}, false, nil
}
