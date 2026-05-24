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
//  4. VS Code GitHub Copilot extension hosts.json (%APPDATA%\GitHub Copilot\hosts.json on Windows)
//  5. `gh auth token` subprocess (covers OS-keychain / credential-helper storage)
//
// Uses the GitHub Copilot chat completions API at https://api.githubcopilot.com,
// which exposes an OpenAI-compatible interface. This lets VS Code / GitHub Copilot
// subscribers use their existing Copilot plan — no extra API key required.
//
// The list of available models is fetched dynamically from GET /models on the
// first call to Capabilities(), so new Copilot models appear automatically.
// If the endpoint is unreachable the known-good fallback list is used instead.
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
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/teragrid/forge/internal/errcode"
)

const (
	copilotAPIBase      = "https://api.githubcopilot.com"
	copilotDefaultModel = "claude-sonnet-4-5"
)

// copilotKnownModels is the static fallback used when the /models endpoint is
// unreachable (air-gap, token not yet valid, etc.).
var copilotKnownModels = []string{
	"claude-sonnet-4-5",
	"claude-3-7-sonnet",
	"claude-3-5-sonnet",
	"gpt-4o",
	"gpt-4o-mini",
	"o3-mini",
}

// CopilotProvider implements Provider using the GitHub Copilot Chat API.
// Any GitHub account with an active Copilot subscription can use this provider.
type CopilotProvider struct {
	token        string
	model        string
	baseURL      string // overridable in tests
	client       *http.Client
	cachedModels []string
	modelsOnce   sync.Once
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
		token:   token,
		model:   model,
		baseURL: copilotAPIBase,
		client:  &http.Client{},
	}
}

// loadModels fetches the list of models from GET {baseURL}/models and caches
// the result. Called lazily (once) from Capabilities(). Falls back silently.
func (c *CopilotProvider) loadModels() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/models", nil)
	if err != nil {
		return
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Copilot-Integration-Id", "forge-cli")
	req.Header.Set("Editor-Version", "forge/1.2.1")
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<16)) // 64 KB max
	if err != nil {
		return
	}

	var body struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &body); err != nil {
		return
	}

	models := make([]string, 0, len(body.Data))
	for _, m := range body.Data {
		if m.ID != "" {
			models = append(models, m.ID)
		}
	}
	if len(models) > 0 {
		c.cachedModels = models
	}
}

// detectGitHubToken returns a GitHub OAuth token from the environment or
// config files, in priority order:
//  1. GH_TOKEN / GITHUB_TOKEN environment variables
//  2. gh CLI hosts.yml (plain-text storage)
//  3. VS Code GitHub Copilot extension hosts.json
//  4. `gh auth token` subprocess (OS keychain / credential helper)
func detectGitHubToken() string {
	if t := os.Getenv("GH_TOKEN"); t != "" {
		return t
	}
	if t := os.Getenv("GITHUB_TOKEN"); t != "" {
		return t
	}
	// Try the gh CLI config file first (plain-text storage).
	if t := readGHConfigToken(); t != "" {
		return t
	}
	// VS Code GitHub Copilot extension stores its token separately from gh CLI.
	if t := readVSCodeCopilotToken(); t != "" {
		return t
	}
	// Modern gh CLI may store the token in the OS keychain instead of the
	// config file. Run `gh auth token` as a last resort; it works regardless
	// of the storage backend (plaintext, keychain, credential helper).
	return runGHAuthToken()
}

// runGHAuthToken spawns `gh auth token --hostname github.com` with a short
// timeout. Returns the trimmed token string, or "" when gh is not installed
// or the user is not authenticated.
func runGHAuthToken() string {
	ghPath, err := exec.LookPath("gh")
	if err != nil {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, ghPath, "auth", "token", "--hostname", "github.com").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
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

// readVSCodeCopilotToken reads the oauth_token from the VS Code GitHub Copilot
// extension's hosts.json file. This covers users who have the VS Code extension
// installed but do not use the gh CLI.
// Format: {"github.com:": {"user": "username", "oauth_token": "ghu_XXXXX"}}
// Note: the key uses a trailing colon ("github.com:").
func readVSCodeCopilotToken() string {
	path := vscodeCopilotConfigPath()
	if path == "" {
		return ""
	}
	data, err := os.ReadFile(path) //nolint:gosec // path is constructed from known safe locations
	if err != nil {
		return ""
	}
	var hosts map[string]map[string]string
	if err := json.Unmarshal(data, &hosts); err != nil {
		return ""
	}
	for _, entry := range hosts {
		if t := entry["oauth_token"]; t != "" {
			return t
		}
	}
	return ""
}

// vscodeCopilotConfigPath returns the path to the VS Code GitHub Copilot
// extension's hosts.json file, or "" if the path cannot be determined.
func vscodeCopilotConfigPath() string {
	if runtime.GOOS == "windows" {
		appdata := os.Getenv("APPDATA")
		if appdata == "" {
			return ""
		}
		return filepath.Join(appdata, "GitHub Copilot", "hosts.json")
	}
	if runtime.GOOS == "darwin" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		return filepath.Join(home, "Library", "Application Support", "GitHub Copilot", "hosts.json")
	}
	// Linux
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "github-copilot", "hosts.json")
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

// Capabilities fetches the live model list from the Copilot API on first call
// (cached via sync.Once) and returns it. Falls back to copilotKnownModels if
// the endpoint is unreachable.
func (c *CopilotProvider) Capabilities() Capabilities {
	c.modelsOnce.Do(c.loadModels)
	models := c.cachedModels
	if len(models) == 0 {
		models = copilotKnownModels
	}
	return Capabilities{
		Streaming: false,
		MaxTokens: 200000,
		Models:    models,
	}
}

// Complete sends a chat completion request to the GitHub Copilot API.
// On HTTP 400 with a model-availability error, it automatically retries with
// the next model from copilotKnownModels (unless req.Model is explicitly set).
func (c *CopilotProvider) Complete(ctx context.Context, req *Request) (*Response, error) {
	if req == nil {
		return nil, errcode.New(ErrInvalidInput, "request must not be nil", nil)
	}

	primaryModel := c.model
	if req.Model != "" {
		primaryModel = req.Model
	}

	// Build OpenAI-compatible chat messages.
	type chatMessage struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	type chatRequest struct {
		Model     string        `json:"model"`
		Messages  []chatMessage `json:"messages"`
		MaxTokens int           `json:"max_tokens,omitempty"`
	}

	messages := make([]chatMessage, 0, 2)
	if req.SystemPrompt != "" {
		messages = append(messages, chatMessage{Role: "system", Content: req.SystemPrompt})
	}
	if req.UserPrompt != "" {
		messages = append(messages, chatMessage{Role: "user", Content: req.UserPrompt})
	}

	// attempt sends one request with the given model.
	// Returns (response, shouldRetry, error): shouldRetry is true only on a
	// model-availability HTTP 400 so the caller can try the next model.
	attempt := func(model string) (*Response, bool, error) {
		body := chatRequest{Model: model, Messages: messages, MaxTokens: req.MaxTokens}
		raw, err := json.Marshal(body)
		if err != nil {
			return nil, false, fmt.Errorf("copilot: marshal request: %w", err)
		}
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(raw))
		if err != nil {
			return nil, false, fmt.Errorf("copilot: create request: %w", err)
		}
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Authorization", "Bearer "+c.token)
		httpReq.Header.Set("Copilot-Integration-Id", "forge-cli")
		httpReq.Header.Set("Editor-Version", "forge/1.2.1")

		resp, err := c.client.Do(httpReq)
		if err != nil {
			return nil, false, fmt.Errorf("copilot: http: %w", err)
		}
		defer resp.Body.Close() //nolint:errcheck

		respData, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1 MB max
		if err != nil {
			return nil, false, fmt.Errorf("copilot: read response: %w", err)
		}
		switch resp.StatusCode {
		case http.StatusOK:
			// fall through to parse response below
		case http.StatusBadRequest:
			// Retry with next model when the API signals a model-availability error.
			// Example body: {"error":{"message":"The requested model is not supported..."}}
			if strings.Contains(strings.ToLower(string(respData)), "model") {
				return nil, true, fmt.Errorf("copilot: model %q unavailable (HTTP 400): %s", model, string(respData))
			}
			return nil, false, fmt.Errorf("copilot: API error %d: %s", resp.StatusCode, string(respData))
		case http.StatusUnauthorized, http.StatusForbidden:
			// The most common cause is a GitHub token that exists but lacks the
			// 'copilot' OAuth scope (e.g. a token created before Copilot support
			// was added, or a fine-grained PAT without the scope).
			// Running `gh auth refresh -s copilot` re-authenticates and adds the
			// scope without creating a new token.
			return nil, false, fmt.Errorf(
				"copilot: API returned HTTP %d — your GitHub token may be missing the 'copilot' scope.\n"+
					"Run: gh auth refresh -s copilot\n"+
					"Then retry. (raw: %s)",
				resp.StatusCode, string(respData))
		default:
			return nil, false, fmt.Errorf("copilot: API error %d: %s", resp.StatusCode, string(respData))
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
			return nil, false, fmt.Errorf("copilot: parse response: %w", err)
		}
		if len(cr.Choices) == 0 || cr.Choices[0].Message.Content == "" {
			return nil, false, fmt.Errorf("copilot: empty response")
		}
		return &Response{
			Content:      cr.Choices[0].Message.Content,
			InputTokens:  cr.Usage.PromptTokens,
			OutputTokens: cr.Usage.CompletionTokens,
			Model:        cr.Model,
		}, false, nil
	}

	// Try the primary model first.
	if resp, retry, err := attempt(primaryModel); !retry {
		return resp, err
	}

	// Model-availability error: do NOT silently fall back when the caller explicitly
	// requested a specific model (req.Model != ""), so user intent is honoured.
	if req.Model != "" {
		return nil, fmt.Errorf(
			"copilot: model %q returned HTTP 400; set FORGE_COPILOT_MODEL to a supported model",
			primaryModel)
	}

	// Iterate through known fallback models.
	for _, fallback := range copilotKnownModels {
		if fallback == primaryModel {
			continue
		}
		if resp, retry, err := attempt(fallback); !retry {
			return resp, err
		}
		// fallback also model-unavailable; try next
	}
	return nil, fmt.Errorf(
		"copilot: no available model for this account; tried %q and all fallbacks — "+
			"run: forge config list-models",
		primaryModel)
}
