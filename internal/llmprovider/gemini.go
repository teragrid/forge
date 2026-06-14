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

// Package llmprovider — gemini.go implements the Google Gemini LLM provider (M2-04).
//
// Activated when GEMINI_API_KEY is set in the environment.
// Uses the Gemini REST API (v1beta, generateContent endpoint).
//
// This is a minimal implementation that matches the Provider interface.
// It does NOT require an external SDK — only the standard library.
package llmprovider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

const geminiAPIBase = "https://generativelanguage.googleapis.com/v1beta/models"
const geminiDefaultModel = "gemini-2.0-flash"

// GeminiProvider implements Provider using the Google Gemini REST API.
type GeminiProvider struct {
	apiKey string
	model  string
	client *http.Client
}

// newGeminiProvider constructs a GeminiProvider from the environment.
// Returns nil if GEMINI_API_KEY is not set.
func newGeminiProvider() *GeminiProvider {
	key := os.Getenv("GEMINI_API_KEY")
	if key == "" {
		return nil
	}
	model := os.Getenv("GEMINI_MODEL")
	if model == "" {
		model = geminiDefaultModel
	}
	return &GeminiProvider{
		apiKey: key,
		model:  model,
		client: &http.Client{},
	}
}

// geminiRequest is the request body for the Gemini generateContent API.
type geminiRequest struct {
	SystemInstruction *geminiContent  `json:"system_instruction,omitempty"`
	Contents          []geminiContent `json:"contents"`
	GenerationConfig  geminiGenConfig `json:"generationConfig"`
}

type geminiContent struct {
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiGenConfig struct {
	MaxOutputTokens int `json:"maxOutputTokens,omitempty"`
}

// geminiResponse is the response from the Gemini generateContent API.
type geminiResponse struct {
	Candidates []struct {
		Content geminiContent `json:"content"`
	} `json:"candidates"`
	UsageMetadata struct {
		PromptTokenCount     int `json:"promptTokenCount"`
		CandidatesTokenCount int `json:"candidatesTokenCount"`
	} `json:"usageMetadata"`
}

// Complete sends a request to the Gemini generateContent API and returns a Response.
func (g *GeminiProvider) Complete(ctx context.Context, req *Request) (*Response, error) {
	model := g.model
	if req.Model != "" {
		model = req.Model
	}
	url := fmt.Sprintf("%s/%s:generateContent?key=%s", geminiAPIBase, model, g.apiKey)

	body := geminiRequest{
		Contents: []geminiContent{
			{Parts: []geminiPart{{Text: req.UserPrompt}}},
		},
		GenerationConfig: geminiGenConfig{
			MaxOutputTokens: req.MaxTokens,
		},
	}
	if req.SystemPrompt != "" {
		body.SystemInstruction = &geminiContent{
			Parts: []geminiPart{{Text: req.SystemPrompt}},
		}
	}

	raw, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("gemini: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("gemini: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := g.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("gemini: http: %w", err)
	}
	defer resp.Body.Close()

	respData, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1 MB max
	if err != nil {
		return nil, fmt.Errorf("gemini: read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gemini: API error %d: %s", resp.StatusCode, string(respData))
	}

	var gr geminiResponse
	if err := json.Unmarshal(respData, &gr); err != nil {
		return nil, fmt.Errorf("gemini: parse response: %w", err)
	}
	if len(gr.Candidates) == 0 || len(gr.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("gemini: empty response")
	}

	content := gr.Candidates[0].Content.Parts[0].Text
	return &Response{
		Content:      content,
		InputTokens:  gr.UsageMetadata.PromptTokenCount,
		OutputTokens: gr.UsageMetadata.CandidatesTokenCount,
		Model:        model,
	}, nil
}

// Name returns the provider name.
func (g *GeminiProvider) Name() string { return "gemini" }

// Capabilities returns Gemini capability metadata.
func (g *GeminiProvider) Capabilities() Capabilities {
	return Capabilities{
		Streaming: false,
		MaxTokens: 1048576,
		Models: []string{
			// Gemini 2.5 family
			"gemini-2.5-pro-preview-06-05",
			"gemini-2.5-flash-preview-05-20",
			// Gemini 2.0 family
			"gemini-2.0-flash",
			"gemini-2.0-flash-lite",
			// Gemini 1.5 family (still available)
			"gemini-1.5-pro",
			"gemini-1.5-flash",
		},
	}
}
