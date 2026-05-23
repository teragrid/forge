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

// Package llmprovider — copilot.go implements the GitHub Copilot LLM adapter.
//
// Activated when a GitHub token is available via (in priority order):
//  1. GH_TOKEN environment variable
//  2. GITHUB_TOKEN environment variable
//  3. gh CLI config file (~/.config/gh/hosts.yml or %APPDATA%\GitHub CLI\hosts.yml)
//
// Uses the GitHub Copilot chat completions API at https://api.githubcopilot.com,
// which exposes an OpenAI-compatible interface. This lets VS Code / GitHub Copilot
// subscribers use their existing Copilot plan — no extra API key required.
//
// Default model: claude-sonnet-4-5 (override via FORGE_COPILOT_MODEL).
package llmprovider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/teragrid/forge/internal/errcode"
)

const (
	copilotAPIBase      = "https://api.githubcopilot.com"
	copilotChatURL      = copilotAPIBase + "/chat/completions"
	copilotDefaultModel = "claude-sonnet-4-5"
)

// CopilotProvider implements Provider using the GitHub Copilot Chat API.
// Any GitHub account with an active Copilot subscription can use this provider.
type CopilotProvider struct {
	token  string
	model  string
	client *http.Client
}

// newCopilotProvider returns a CopilotProvider when a GitHub token is available,
// nil otherwise.
func newCopilotProvider() *CopilotProvider {
	token := detectGitHubToken()
	if token == "" {
		return nil
	}
	model := os.Getenv("FORGE_COPILOT_MODEL")
	if model == "" {
		model = copilotDefaultModel
	}
	return &CopilotProvider{
		token:  token,
		model:  model,
		client: &http.Client{},
	}
}

// detectGitHubToken returns a GitHub OAuth token from the environment or
// the gh CLI config file, in that priority order.
func detectGitHubToken() string {
	if t := os.Getenv("GH_TOKEN"); t != "" {
		return t
	}
	if t := os.Getenv("GITHUB_TOKEN"); t != "" {
		return t
	}
	return readGHConfigToken()
}

// readGHConfigToken reads the oauth_token from the gh CLI hosts config file.
// No subprocess is spawned; the file is read directly.
func readGHConfigToken() string {
	path := ghConfigPath()
	if path == "" {
		return ""
	}
	f, err := os.Open(path) //nolint:gosec // path is constructed from known safe locations
	if err != nil {
		return ""
	}
	defer f.Close()

	// Parse the minimal YAML structure we need.
	// Format:
	//   github.com:
	//       oauth_token: ghp_xxxxx
	//       user: username
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "oauth_token:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1])
			}
		}
	}
	return ""
}

// ghConfigPath returns the path to the gh CLI hosts.yml config file.
func ghConfigPath() string {
	// Respect GH_CONFIG_DIR if set (official gh env var).
	if dir := os.Getenv("GH_CONFIG_DIR"); dir != "" {
		return filepath.Join(dir, "hosts.yml")
	}
	if runtime.GOOS == "windows" {
		appdata := os.Getenv("APPDATA")
		if appdata == "" {
			return ""
		}
		return filepath.Join(appdata, "GitHub CLI", "hosts.yml")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "gh", "hosts.yml")
}

// ── Provider interface ────────────────────────────────────────────────────────

func (c *CopilotProvider) Name() string { return "github-copilot" }

func (c *CopilotProvider) Capabilities() Capabilities {
	return Capabilities{
		Streaming: false,
		MaxTokens: 200000,
		Models: []string{
			"claude-sonnet-4-5",
			"claude-3-7-sonnet",
			"claude-3-5-sonnet",
			"gpt-4o",
			"gpt-4o-mini",
		},
	}
}

// Complete sends a chat completion request to the GitHub Copilot API.
func (c *CopilotProvider) Complete(ctx context.Context, req *Request) (*Response, error) {
	if req == nil {
		return nil, errcode.New(ErrInvalidInput, "request must not be nil", nil)
	}

	model := c.model
	if req.Model != "" {
		model = req.Model
	}

	// Build OpenAI-compatible chat messages.
	type chatMessage struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	messages := make([]chatMessage, 0, 2)
	if req.SystemPrompt != "" {
		messages = append(messages, chatMessage{Role: "system", Content: req.SystemPrompt})
	}
	if req.UserPrompt != "" {
		messages = append(messages, chatMessage{Role: "user", Content: req.UserPrompt})
	}

	type chatRequest struct {
		Model     string        `json:"model"`
		Messages  []chatMessage `json:"messages"`
		MaxTokens int           `json:"max_tokens,omitempty"`
	}
	body := chatRequest{
		Model:     model,
		Messages:  messages,
		MaxTokens: req.MaxTokens,
	}

	raw, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("copilot: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, copilotChatURL, bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("copilot: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.token)
	httpReq.Header.Set("Copilot-Integration-Id", "forge-cli")
	httpReq.Header.Set("Editor-Version", "forge/1.1.6")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("copilot: http: %w", err)
	}
	defer resp.Body.Close()

	respData, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1 MB max
	if err != nil {
		return nil, fmt.Errorf("copilot: read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("copilot: API error %d: %s", resp.StatusCode, string(respData))
	}

	// Parse OpenAI-compatible response.
	var cr struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
		Model string `json:"model"`
	}
	if err := json.Unmarshal(respData, &cr); err != nil {
		return nil, fmt.Errorf("copilot: parse response: %w", err)
	}
	if len(cr.Choices) == 0 || cr.Choices[0].Message.Content == "" {
		return nil, fmt.Errorf("copilot: empty response")
	}

	return &Response{
		Content:      cr.Choices[0].Message.Content,
		InputTokens:  cr.Usage.PromptTokens,
		OutputTokens: cr.Usage.CompletionTokens,
		Model:        cr.Model,
	}, nil
}
