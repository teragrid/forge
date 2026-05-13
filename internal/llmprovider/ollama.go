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

// Package llmprovider — ollama.go implements the Ollama local LLM adapter (M3-13).
//
// Detected when OLLAMA_HOST is set (typically "http://localhost:11434").
// Ollama runs models locally with no external API calls — the primary adapter
// for air-gapped environments.
//
// Default model: llama3.2 (override via OLLAMA_MODEL or Request.Model).
//
// The actual HTTP call to the Ollama /api/generate endpoint is left to a
// concrete client; this adapter carries detection metadata and capability info.
package llmprovider

import (
	"context"
	"os"

	"github.com/teragrid/forge/internal/errcode"
)

// OllamaAdapter is a Provider for locally-running Ollama models.
// Use this adapter in air-gapped environments (M3-13).
type OllamaAdapter struct {
	host  string
	model string
}

// newOllamaProvider returns an OllamaAdapter when OLLAMA_HOST is set, nil otherwise.
func newOllamaProvider() *OllamaAdapter {
	host := os.Getenv("OLLAMA_HOST")
	if host == "" {
		return nil
	}
	model := os.Getenv("OLLAMA_MODEL")
	if model == "" {
		model = "llama3.2"
	}
	return &OllamaAdapter{host: host, model: model}
}

func (o *OllamaAdapter) Name() string { return "ollama" }

func (o *OllamaAdapter) Capabilities() Capabilities {
	return Capabilities{
		Streaming: true,
		MaxTokens: 128000,
		Models: []string{
			"llama3.2",
			"llama3.1",
			"codellama",
			"mistral",
			"gemma2",
			"phi3",
		},
	}
}

// Complete is a stub. Replace with a concrete HTTP client targeting OLLAMA_HOST/api/generate.
func (o *OllamaAdapter) Complete(_ context.Context, req *Request) (*Response, error) {
	if req == nil {
		return nil, errcode.New(ErrInvalidInput, "request must not be nil", nil)
	}
	return nil, errcode.New(ErrProviderFail,
		"OllamaAdapter.Complete: HTTP transport not implemented; wire a concrete Ollama client", nil)
}

// Host returns the Ollama server base URL.
func (o *OllamaAdapter) Host() string { return o.host }

// DefaultModel returns the configured default model name.
func (o *OllamaAdapter) DefaultModel() string { return o.model }
