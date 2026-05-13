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

// ── Detection (IDE-config bridge) ────────────────────────────────────────────

// Detect inspects the environment and returns the first available Provider.
// Detection order (highest priority first):
//  1. ANTHROPIC_API_KEY       → AnthropicAdapter
//  2. OPENAI_API_KEY          → OpenAIAdapter
//  3. GEMINI_API_KEY          → GeminiAdapter
//  4. AZURE_OPENAI_API_KEY    → AzureOpenAIAdapter (requires AZURE_OPENAI_ENDPOINT)
//  5. AWS_BEDROCK_REGION      → BedrockAdapter
//  6. OLLAMA_HOST             → OllamaAdapter (air-gap / local)
//
// Returns ErrNoProvider (FORGE-4050) if no known credentials are present.
func Detect() (Provider, error) {
	if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
		return &AnthropicAdapter{apiKey: key}, nil
	}
	if key := os.Getenv("OPENAI_API_KEY"); key != "" {
		return &OpenAIAdapter{apiKey: key}, nil
	}
	if p := newGeminiProvider(); p != nil {
		return p, nil
	}
	if p := newAzureOpenAIProvider(); p != nil {
		return p, nil
	}
	if p := newBedrockProvider(); p != nil {
		return p, nil
	}
	if p := newOllamaProvider(); p != nil {
		return p, nil
	}
	return nil, errcode.New(ErrNoProvider,
		"no ANTHROPIC_API_KEY, OPENAI_API_KEY, GEMINI_API_KEY, AZURE_OPENAI_API_KEY, AWS_BEDROCK_REGION, or OLLAMA_HOST found in environment", nil)
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
			"claude-3-5-sonnet-20241022",
			"claude-3-5-haiku-20241022",
			"claude-3-opus-20240229",
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
			"gpt-4o",
			"gpt-4o-mini",
			"gpt-4-turbo",
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
	// Response is returned verbatim from Complete when Err is nil.
	Response *Response
	// Err is returned from Complete when set.
	Err error
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
