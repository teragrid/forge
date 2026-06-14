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

// Package llmprovider implements DEV-M0-16 and DEV-M0-17: the ILlmProvider
// interface, environment-variable-based provider detection (IDE-config bridge),
// and a mock provider for use in tests.
//
// Detection order:
//  1. ANTHROPIC_API_KEY  → AnthropicAdapter (Claude models)
//  2. OPENAI_API_KEY     → OpenAIAdapter (GPT models)
//  3. Neither present    → ErrNoProvider (FORGE-4050)
//
// No external HTTP calls are made by the adapters in this package; they carry
// only the key and capability metadata. Actual HTTP transport is the caller's
// responsibility, which allows clean separation from HTTP mocking in tests.
package llmprovider

import (
	"context"
	"os"
	"strings"

	"github.com/teragrid/forge/internal/config"
	"github.com/teragrid/forge/internal/errcode"
)

// Reserved error codes (range 4050..4099).
var (
	ErrNoProvider   = errcode.Register(errcode.Code(4050), "no LLM provider detected")
	ErrProviderFail = errcode.Register(errcode.Code(4051), "LLM provider request failed")
	ErrInvalidInput = errcode.Register(errcode.Code(4052), "invalid LLM request")
)

// ── Core types ────────────────────────────────────────────────────────────────

// Capabilities describes what a provider supports.
type Capabilities struct {
	Streaming bool
	MaxTokens int
	Models    []string
}

// Request is a single completion request.
type Request struct {
	// Model is the model identifier (e.g. "claude-3-5-sonnet-20241022").
	// If empty, the provider chooses a sensible default.
	Model        string
	SystemPrompt string
	UserPrompt   string
	// MaxTokens caps the completion length. 0 means provider default.
	MaxTokens int
	// G-140: per-feature token attribution.
	// Capability is the forge capability/verb performing this LLM call.
	Capability string
	// Actor is the user or service initiating this request.
	Actor string
	// CorrelationID links all LLM calls in a single pipeline run.
	CorrelationID string
}

// Response is a completed completion response.
type Response struct {
	Content      string
	InputTokens  int
	OutputTokens int
	Model        string
}

// Provider is the interface that all LLM backends implement.
type Provider interface {
	// Name returns a stable identifier for the provider (e.g. "anthropic").
	Name() string
	// Complete sends a completion request and returns the response.
	Complete(ctx context.Context, req *Request) (*Response, error)
	// Capabilities returns static metadata about the provider's capabilities.
	Capabilities() Capabilities
}

// ── Profile-aware provider wrapper ───────────────────────────────────────────

// profileProvider wraps any Provider and applies the active profile's
// MaxLLMTokenBudget when the caller leaves MaxTokens at zero, and the
// forge.yml llm.model when the request leaves Model empty.
type profileProvider struct {
	inner       Provider
	configModel string // forge.yml llm.model; applied when Request.Model is empty
}

func (p *profileProvider) Name() string               { return p.inner.Name() }
func (p *profileProvider) Capabilities() Capabilities { return p.inner.Capabilities() }

func (p *profileProvider) Complete(ctx context.Context, req *Request) (*Response, error) {
	if req != nil {
		var modified bool
		copied := *req
		if prof := config.ActiveProfile(); prof != nil && prof.MaxLLMTokenBudget > 0 && copied.MaxTokens == 0 {
			copied.MaxTokens = prof.MaxLLMTokenBudget
			modified = true
		}
		if p.configModel != "" && copied.Model == "" {
			copied.Model = p.configModel
			modified = true
		}
		if modified {
			req = &copied
		}
	}
	return p.inner.Complete(ctx, req)
}

// WithActiveProfile wraps p so that every Complete call applies the active
// profile's token budget when the caller leaves MaxTokens at zero.
// Callers that bypass Detect() (e.g. tests, dependency injection) can use
// this to opt in to the same profile-aware behaviour.
func WithActiveProfile(p Provider) Provider { return &profileProvider{inner: p} }

// ── Detection (IDE-config bridge) ────────────────────────────────────────────

// Detect inspects the environment and returns the first available Provider.
// Detection order (highest priority first):
//  0. forge.yml llm.provider  → provider selected by name (credentials still from env)
//  1. ANTHROPIC_API_KEY       → AnthropicAdapter
//  2. OPENAI_API_KEY          → OpenAIAdapter
//  3. GEMINI_API_KEY          → GeminiAdapter
//  4. AZURE_OPENAI_API_KEY    → AzureOpenAIAdapter (requires AZURE_OPENAI_ENDPOINT)
//  5. AWS_BEDROCK_REGION      → BedrockAdapter
//  6. OLLAMA_HOST             → OllamaAdapter (air-gap / local)
//  7. GH_TOKEN / GITHUB_TOKEN / gh config → CopilotProvider (GitHub Copilot plan)
//
// Returns ErrNoProvider (FORGE-4050) if no known credentials are present.
func Detect() (Provider, error) {
	// FORGE_NO_LLM=1 disables auto-detection (used in integration tests to
	// prevent real network calls when no mock provider is injected).
	if os.Getenv("FORGE_NO_LLM") == "1" {
		return nil, errcode.New(ErrNoProvider, "LLM disabled (FORGE_NO_LLM=1)", nil)
	}
	var inner Provider
	var configModel string

	// Layer 0: forge.yml explicit provider preference takes priority over env-var order.
	if root, wdErr := os.Getwd(); wdErr == nil {
		if cfg, cfgErr := config.Load(root, nil); cfgErr == nil {
			configModel = cfg.LLMModel.Raw
			if prov := cfg.LLMProvider.Raw; prov != "" && prov != "auto" {
				inner = detectByName(prov)
			}
		}
	}

	// Env-var auto-detection (used when no explicit provider is configured).
	if inner == nil {
		switch {
		case os.Getenv("ANTHROPIC_API_KEY") != "":
			inner = &AnthropicAdapter{apiKey: os.Getenv("ANTHROPIC_API_KEY")}
		case os.Getenv("OPENAI_API_KEY") != "":
			inner = &OpenAIAdapter{apiKey: os.Getenv("OPENAI_API_KEY")}
		default:
			if p := newGeminiProvider(); p != nil {
				inner = p
			} else if p := newAzureOpenAIProvider(); p != nil {
				inner = p
			} else if p := newBedrockProvider(); p != nil {
				inner = p
			} else if p := newOllamaProvider(); p != nil {
				inner = p
			} else if p := newCopilotProvider(); p != nil {
				inner = p
			}
		}
	}

	if inner == nil {
		return nil, errcode.New(ErrNoProvider,
			"no LLM provider detected: run 'forge config set llm.provider <name>' (e.g. copilot, ollama) "+
				"or set ANTHROPIC_API_KEY, OPENAI_API_KEY, GEMINI_API_KEY, "+
				"AZURE_OPENAI_API_KEY, AWS_BEDROCK_REGION, OLLAMA_HOST, or GH_TOKEN", nil)
	}
	return &profileProvider{inner: inner, configModel: configModel}, nil
}

// detectByName initialises a provider by explicit name, using the same
// credential env-vars as the auto-detection path. Returns nil when the
// provider's credentials are unavailable.
func detectByName(name string) Provider {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "anthropic":
		if k := os.Getenv("ANTHROPIC_API_KEY"); k != "" {
			return &AnthropicAdapter{apiKey: k}
		}
	case "openai":
		if k := os.Getenv("OPENAI_API_KEY"); k != "" {
			return &OpenAIAdapter{apiKey: k}
		}
	case "gemini":
		if p := newGeminiProvider(); p != nil {
			return p
		}
	case "azure", "azure_openai", "azure-openai":
		if p := newAzureOpenAIProvider(); p != nil {
			return p
		}
	case "bedrock", "aws_bedrock", "aws-bedrock":
		if p := newBedrockProvider(); p != nil {
			return p
		}
	case "ollama":
		if p := newOllamaProvider(); p != nil {
			return p
		}
	case "copilot", "github_copilot", "github-copilot":
		if p := newCopilotProvider(); p != nil {
			return p
		}
	}
	return nil
}

// ── Anthropic adapter ─────────────────────────────────────────────────────────

// AnthropicAdapter is a Provider skeleton for the Anthropic Claude API.
// HTTP transport is intentionally not included here; inject a custom transport
// or use a concrete implementation built on top of this adapter.
type AnthropicAdapter struct {
	apiKey string
}

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

// Complete is a stub that returns ErrProviderFail. Replace with a concrete
// HTTP implementation when needed.
func (a *AnthropicAdapter) Complete(_ context.Context, req *Request) (*Response, error) {
	if req == nil {
		return nil, errcode.New(ErrInvalidInput, "request must not be nil", nil)
	}
	return nil, errcode.New(ErrProviderFail,
		"AnthropicAdapter.Complete: HTTP transport not implemented; wire a concrete client", nil)
}

// APIKey returns the raw API key (for use by concrete HTTP clients).
func (a *AnthropicAdapter) APIKey() string { return a.apiKey }

// ── OpenAI adapter ────────────────────────────────────────────────────────────

// OpenAIAdapter is a Provider skeleton for the OpenAI-compatible API.
type OpenAIAdapter struct {
	apiKey string
}

func (o *OpenAIAdapter) Name() string { return "openai" }

func (o *OpenAIAdapter) Capabilities() Capabilities {
	return Capabilities{
		Streaming: true,
		MaxTokens: 128000,
		Models: []string{
			// GPT-4.1 family
			"gpt-4.1",
			"gpt-4.1-mini",
			// GPT-4o family
			"gpt-4o",
			"gpt-4o-mini",
			// o-series reasoning models
			"o4-mini",
			"o3",
			"o1",
		},
	}
}

// Complete is a stub that returns ErrProviderFail. Replace with a concrete
// HTTP implementation when needed.
func (o *OpenAIAdapter) Complete(_ context.Context, req *Request) (*Response, error) {
	if req == nil {
		return nil, errcode.New(ErrInvalidInput, "request must not be nil", nil)
	}
	return nil, errcode.New(ErrProviderFail,
		"OpenAIAdapter.Complete: HTTP transport not implemented; wire a concrete client", nil)
}

// APIKey returns the raw API key (for use by concrete HTTP clients).
func (o *OpenAIAdapter) APIKey() string { return o.apiKey }

// ── Mock provider ─────────────────────────────────────────────────────────────

// MockProvider is a deterministic Provider for use in tests. It never makes
// network calls and returns canned responses.
type MockProvider struct {
	// Response is returned verbatim from Complete when Err is nil and Fn is nil.
	Response *Response
	// Err is returned from Complete when set (takes priority over Fn and Response).
	Err error
	// Fn, when set, is called instead of returning the canned Response. Useful
	// for tests that need to inspect or record the incoming Request.
	Fn func(*Request) (*Response, error)
	// calls tracks the number of Complete invocations.
	calls int
}

func (m *MockProvider) Name() string { return "mock" }

func (m *MockProvider) Capabilities() Capabilities {
	return Capabilities{
		Streaming: false,
		MaxTokens: 4096,
		Models:    []string{"mock-v1"},
	}
}

// Complete returns the canned Response/Err. It records every call.
func (m *MockProvider) Complete(_ context.Context, req *Request) (*Response, error) {
	if req == nil {
		return nil, errcode.New(ErrInvalidInput, "request must not be nil", nil)
	}
	m.calls++
	if m.Err != nil {
		return nil, m.Err
	}
	if m.Fn != nil {
		return m.Fn(req)
	}
	if m.Response != nil {
		return m.Response, nil
	}
	// Sane default when neither Response nor Err is set.
	return &Response{
		Content:      "mock response",
		InputTokens:  10,
		OutputTokens: 5,
		Model:        "mock-v1",
	}, nil
}

// Calls returns the number of times Complete was invoked.
func (m *MockProvider) Calls() int { return m.calls }
