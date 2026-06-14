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
// Default model: claude-sonnet-4-5-20250514 (override via ANTHROPIC_MODEL or
// Request.Model / forge.yml llm.model).
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

	"github.com/teragrid/forge/internal/errcode"
)

const (
	anthropicAPIBase      = "https://api.anthropic.com/v1"
	anthropicAPIVersion   = "2023-06-01"
	anthropicDefaultModel = "claude-sonnet-4-5-20250514"
)

// AnthropicAdapter implements Provider using the Anthropic Claude API.
type AnthropicAdapter struct {
	apiKey  string
	model   string
	baseURL string       // empty → anthropicAPIBase; overridable in tests
	client  *http.Client // nil → http.DefaultClient
}

// newAnthropicProvider returns an AnthropicAdapter when an API key can be found,
// nil otherwise.  Key sources are checked in priority order by detectAnthropicKey.
func newAnthropicProvider() *AnthropicAdapter {
	key := detectAnthropicKey()
	if key == "" {
		return nil
	}
	model := os.Getenv("ANTHROPIC_MODEL")
	if model == "" {
		model = anthropicDefaultModel
	}
	return &AnthropicAdapter{apiKey: key, model: model}
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

func (a *AnthropicAdapter) Capabilities() Capabilities {
	return Capabilities{
		Streaming: true,
		MaxTokens: 200000,
		Models: []string{
			// Claude 4 family
			"claude-opus-4-8-20250514",
			"claude-sonnet-4-5-20250514",
			"claude-haiku-4-5-20251001",
			// Claude 3.7
			"claude-3-7-sonnet-20250219",
			// Claude 3.5
			"claude-3-5-sonnet-20241022",
			"claude-3-5-haiku-20241022",
		},
	}
}

// APIKey returns the raw API key (for callers that need it directly).
func (a *AnthropicAdapter) APIKey() string { return a.apiKey }

// Complete sends a chat request to the Anthropic Messages API and returns the
// assistant's response.
func (a *AnthropicAdapter) Complete(ctx context.Context, req *Request) (*Response, error) {
	if req == nil {
		return nil, errcode.New(ErrInvalidInput, "request must not be nil", nil)
	}
	if a.apiKey == "" {
		return nil, errcode.New(ErrProviderFail,
			"anthropic: no API key — set ANTHROPIC_API_KEY or authenticate with Claude Code CLI", nil)
	}

	model := a.model
	if model == "" {
		model = anthropicDefaultModel
	}
	if req.Model != "" {
		model = req.Model
	}

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
		return nil, fmt.Errorf("anthropic: marshal request: %w", err)
	}

	baseURL := a.baseURL
	if baseURL == "" {
		baseURL = anthropicAPIBase
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/messages", bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("anthropic: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", a.apiKey)
	httpReq.Header.Set("anthropic-version", anthropicAPIVersion)

	client := a.client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("anthropic: http: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	respData, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1 MB max
	if err != nil {
		return nil, fmt.Errorf("anthropic: read response: %w", err)
	}

	switch resp.StatusCode {
	case http.StatusOK:
		// handled below
	case http.StatusUnauthorized:
		return nil, fmt.Errorf(
			"anthropic: HTTP 401 — API key invalid or expired.\n"+
				"Set ANTHROPIC_API_KEY or re-authenticate with the Claude Code CLI.\n"+
				"(raw: %s)", string(respData))
	case http.StatusTooManyRequests:
		return nil, fmt.Errorf("anthropic: HTTP 429 — rate limit exceeded: %s", string(respData))
	default:
		return nil, fmt.Errorf("anthropic: API error %d: %s", resp.StatusCode, string(respData))
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
		Model string `json:"model"`
	}
	if err := json.Unmarshal(respData, &ar); err != nil {
		return nil, fmt.Errorf("anthropic: parse response: %w", err)
	}

	var content string
	for _, block := range ar.Content {
		if block.Type == "text" {
			content += block.Text
		}
	}
	if content == "" {
		return nil, fmt.Errorf("anthropic: empty response from API")
	}

	return &Response{
		Content:      content,
		InputTokens:  ar.Usage.InputTokens,
		OutputTokens: ar.Usage.OutputTokens,
		Model:        ar.Model,
	}, nil
}
