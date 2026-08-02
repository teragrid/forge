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
// Default model: claude-sonnet-4-5-20250514 (override via FORGE_COPILOT_MODEL).
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
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/teragrid/forge/internal/errcode"
)

const (
	copilotAPIBase = "https://api.githubcopilot.com"
	// copilotDefaultModel is the last-resort static default, used only when
	// the live /models endpoint is unreachable (air-gap, token not yet
	// valid, network outage) and no explicit model was configured — see
	// resolveModel(). GitHub periodically rotates/retires model snapshot
	// IDs (root-caused 2026-08-02: the previous hardcoded default,
	// "claude-sonnet-4-5-20250514", had gone dead — GitHub's Copilot API
	// does not error on an unknown model id, it silently substitutes a
	// different one (observed: fell back to "gpt-4.1-2025-04-14", whose
	// non-streaming output was cut short — finish_reason "length" — well
	// under the requested token budget on real prompts), so a stale
	// hardcoded ID doesn't fail loudly, it silently degrades output
	// quality). resolveModel() prefers the live list whenever it's
	// reachable specifically to avoid depending on this constant staying
	// fresh; this value only matters in the true offline fallback path.
	copilotDefaultModel = "claude-sonnet-5"
)

// copilotKnownModels is the static fallback used when the /models endpoint is
// unreachable (air-gap, token not yet valid, etc.). Snapshot of currently
// live, enabled model ids as of 2026-08-02 (see copilotDefaultModel's
// comment for why this list drifting out of date is expected and
// tolerated — the live /models endpoint is the authoritative source and is
// preferred whenever reachable). Refresh via `forge llm list --refresh`
// (prints the live catalog) if this ever needs updating by hand.
var copilotKnownModels = []string{
	"claude-sonnet-5",
	"claude-sonnet-4.6",
	"claude-sonnet-4.5",
	"claude-opus-4.5",
	"claude-haiku-4.5",
	"gpt-5.4",
	"gpt-5.4-mini",
	"gpt-4.1-2025-04-14",
	"gpt-4.1",
}

// CopilotModelInfo is the subset of GET /models metadata resolveModel needs
// to pick a good default — richer than the plain []string Capabilities()
// exposes (which only needs IDs).
type CopilotModelInfo struct {
	ID            string
	Vendor        string
	Enabled       bool // policy.state == "enabled" (or no policy gate at all)
	PickerEnabled bool // model_picker_enabled
}

// CopilotProvider implements Provider using the GitHub Copilot Chat API.
// Any GitHub account with an active Copilot subscription can use this provider.
type CopilotProvider struct {
	token   string
	model   string // explicit override (FORGE_COPILOT_MODEL); empty means "resolve dynamically"
	baseURL string // overridable in tests
	client  *http.Client

	cachedModels     []string
	cachedModelInfos []CopilotModelInfo
	modelsOnce       sync.Once

	resolvedDefault    string
	resolvedDefaultSet bool
	resolveOnce        sync.Once
}

// newCopilotProvider returns a CopilotProvider when a GitHub token is available,
// nil otherwise. model is left empty when FORGE_COPILOT_MODEL is unset —
// resolveModel() fills it in lazily from the live model catalog on first use
// rather than baking in a possibly-stale default at construction time.
func newCopilotProvider() *CopilotProvider {
	token := detectGitHubToken()
	if token == "" {
		return nil
	}
	return &CopilotProvider{
		token:   token,
		model:   os.Getenv("FORGE_COPILOT_MODEL"),
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
			ID                 string `json:"id"`
			Vendor             string `json:"vendor"`
			ModelPickerEnabled bool   `json:"model_picker_enabled"`
			Policy             *struct {
				State string `json:"state"`
			} `json:"policy"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &body); err != nil {
		return
	}

	models := make([]string, 0, len(body.Data))
	infos := make([]CopilotModelInfo, 0, len(body.Data))
	for _, m := range body.Data {
		if m.ID == "" {
			continue
		}
		models = append(models, m.ID)
		// No policy block at all means the model isn't behind an
		// account/org enable-toggle — treat as enabled by default (matches
		// e.g. the exec-agent-* / embedding entries in the live catalog,
		// which carry no "policy" key and are usable as-is).
		enabled := m.Policy == nil || m.Policy.State == "enabled"
		infos = append(infos, CopilotModelInfo{
			ID:            m.ID,
			Vendor:        m.Vendor,
			Enabled:       enabled,
			PickerEnabled: m.ModelPickerEnabled,
		})
	}
	if len(models) > 0 {
		c.cachedModels = models
		c.cachedModelInfos = infos
	}
}

// resolveModel returns the model id to use for a request that left
// req.Model empty: an explicit FORGE_COPILOT_MODEL override if set,
// otherwise a live-catalog-derived default (cached for the lifetime of this
// provider instance via resolveOnce), falling back to the static
// copilotDefaultModel constant only when the live /models call itself is
// unreachable. See copilotDefaultModel's doc comment for why preferring
// live data over any hardcoded constant matters here.
func (c *CopilotProvider) resolveModel() string {
	if c.model != "" {
		return c.model
	}
	c.resolveOnce.Do(func() {
		c.modelsOnce.Do(c.loadModels)
		c.resolvedDefault = pickDefaultModel(c.cachedModelInfos, copilotDefaultModel)
		c.resolvedDefaultSet = true
	})
	return c.resolvedDefault
}

// pickDefaultModel chooses a sensible default from a live GET /models
// catalog: prefer an enabled, picker-enabled Anthropic "sonnet"-family
// model (forge's own generation prompts are tuned against Claude, and
// "sonnet" is the quality/cost sweet spot vs. "opus"/"haiku"), then any
// enabled+picker-enabled Anthropic model, then any enabled+picker-enabled
// model regardless of vendor, then just the first live entry. Returns
// fallback unchanged when infos is empty (live fetch failed/unreachable).
func pickDefaultModel(infos []CopilotModelInfo, fallback string) string {
	if len(infos) == 0 {
		return fallback
	}
	usable := func(m CopilotModelInfo) bool { return m.Enabled && m.PickerEnabled }

	// bestByVersion returns the highest-version match (by modelVersionRank)
	// among infos passing keep, or "" if none match.
	bestByVersion := func(keep func(CopilotModelInfo) bool) string {
		best, bestRank := "", -1.0
		for _, m := range infos {
			if !keep(m) {
				continue
			}
			if rank := modelVersionRank(m.ID); best == "" || rank > bestRank {
				best, bestRank = m.ID, rank
			}
		}
		return best
	}

	if id := bestByVersion(func(m CopilotModelInfo) bool {
		return usable(m) && strings.EqualFold(m.Vendor, "Anthropic") && strings.Contains(strings.ToLower(m.ID), "sonnet")
	}); id != "" {
		return id
	}
	if id := bestByVersion(func(m CopilotModelInfo) bool {
		return usable(m) && strings.EqualFold(m.Vendor, "Anthropic")
	}); id != "" {
		return id
	}
	if id := bestByVersion(usable); id != "" {
		return id
	}
	return infos[0].ID
}

// modelVersionRank extracts a comparable version number from a Copilot
// model id's trailing digits/dots (e.g. "claude-sonnet-4.6" → 4.6,
// "claude-sonnet-5" → 5, "gpt-5.6-luna" → 5.6, ignoring the trailing
// non-numeric codename), so pickDefaultModel prefers the newest matching
// model instead of whichever happens to appear first in the API's response
// order (observed to not be strictly newest-last or newest-first).
// Returns 0 for ids with no trailing version number.
func modelVersionRank(id string) float64 {
	m := modelVersionPattern.FindStringSubmatch(id)
	if m == nil {
		return 0
	}
	v, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0
	}
	return v
}

var modelVersionPattern = regexp.MustCompile(`(\d+(?:\.\d+)?)(?:-[a-zA-Z]+)?$`)

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

// LiveModels returns the rich GET /models catalog (id/vendor/enabled/
// picker-enabled) for `forge llm list` to render, plus whether it came from
// a live network fetch (true) or the static copilotKnownModels fallback
// (false, when the endpoint was unreachable). Unlike Capabilities().Models
// (plain IDs, kept minimal for the generic Provider interface), this is
// Copilot-specific and exported directly on *CopilotProvider.
func (c *CopilotProvider) LiveModels() (infos []CopilotModelInfo, live bool) {
	c.modelsOnce.Do(c.loadModels)
	if len(c.cachedModelInfos) > 0 {
		return c.cachedModelInfos, true
	}
	fallback := make([]CopilotModelInfo, 0, len(copilotKnownModels))
	for _, id := range copilotKnownModels {
		fallback = append(fallback, CopilotModelInfo{ID: id, Enabled: true, PickerEnabled: true})
	}
	return fallback, false
}

// ResolvedModel exposes resolveModel() for `forge llm list` to show which
// model an unconfigured (no FORGE_COPILOT_MODEL / forge.yml llm.model)
// request would actually use, without spending a real completion call to
// find out.
func (c *CopilotProvider) ResolvedModel() string { return c.resolveModel() }

// Complete sends a chat completion request to the GitHub Copilot API.
// On HTTP 400 with a model-availability error, it automatically retries with
// the next model from copilotKnownModels (unless req.Model is explicitly set).
func (c *CopilotProvider) Complete(ctx context.Context, req *Request) (*Response, error) {
	if req == nil {
		return nil, errcode.New(ErrInvalidInput, "request must not be nil", nil)
	}

	primaryModel := c.resolveModel()
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
		// Stream is explicitly false (not omitempty — always present in the
		// request body) so the API never has to guess a default for it;
		// Capabilities() declares Streaming: false and Complete() below only
		// ever parses a single-JSON-object response, never SSE frames.
		Stream bool `json:"stream"`
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
				FinishReason string `json:"finish_reason"`
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
			Truncated:    cr.Choices[0].FinishReason == "length",
		}, false, nil
	}

	// Try the primary model first.
	if resp, retry, err := attempt(primaryModel); !retry {
		return resp, err
	}

	// Model-availability error: do NOT silently fall back when the caller
	// pinned a specific model (req.ModelPinned), so genuine user intent is
	// honoured. A model merely defaulted from forge.yml's llm.model /
	// FORGE_COPILOT_MODEL (req.Model set but ModelPinned false, via
	// profileProvider.Complete) is NOT pinned intent -- it must still fall
	// back, otherwise a stale/unavailable configured model permanently
	// breaks every call with no recovery path.
	if req.ModelPinned {
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
