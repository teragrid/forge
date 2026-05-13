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

// Package llmprovider — azure.go implements the Azure OpenAI adapter (M3-09).
//
// Detected when AZURE_OPENAI_API_KEY and AZURE_OPENAI_ENDPOINT are both set.
// The endpoint must be in the form:
//
//	https://<resource>.openai.azure.com
//
// The deployment name (model alias) defaults to "gpt-4o" unless overridden
// via the AZURE_OPENAI_DEPLOYMENT env var or Request.Model.
package llmprovider

import (
	"context"
	"os"

	"github.com/teragrid/forge/internal/errcode"
)

// AzureOpenAIAdapter is a Provider skeleton for the Azure OpenAI Service.
// HTTP transport is intentionally not included; inject a concrete client.
type AzureOpenAIAdapter struct {
	apiKey     string
	endpoint   string
	deployment string
}

// newAzureOpenAIProvider returns an AzureOpenAIAdapter if the required env vars
// are set, or nil otherwise.
func newAzureOpenAIProvider() *AzureOpenAIAdapter {
	key := os.Getenv("AZURE_OPENAI_API_KEY")
	endpoint := os.Getenv("AZURE_OPENAI_ENDPOINT")
	if key == "" || endpoint == "" {
		return nil
	}
	deployment := os.Getenv("AZURE_OPENAI_DEPLOYMENT")
	if deployment == "" {
		deployment = "gpt-4o"
	}
	return &AzureOpenAIAdapter{apiKey: key, endpoint: endpoint, deployment: deployment}
}

func (a *AzureOpenAIAdapter) Name() string { return "azure-openai" }

func (a *AzureOpenAIAdapter) Capabilities() Capabilities {
	return Capabilities{
		Streaming: true,
		MaxTokens: 128000,
		Models: []string{
			"gpt-4o",
			"gpt-4o-mini",
			"gpt-4-turbo",
			"gpt-4",
			"gpt-35-turbo",
		},
	}
}

// Complete is a stub that returns ErrProviderFail. Replace with a concrete
// Azure OpenAI HTTP implementation when needed.
func (a *AzureOpenAIAdapter) Complete(_ context.Context, req *Request) (*Response, error) {
	if req == nil {
		return nil, errcode.New(ErrInvalidInput, "request must not be nil", nil)
	}
	return nil, errcode.New(ErrProviderFail,
		"AzureOpenAIAdapter.Complete: HTTP transport not implemented; wire a concrete client", nil)
}

// APIKey returns the raw API key (for use by concrete HTTP clients).
func (a *AzureOpenAIAdapter) APIKey() string { return a.apiKey }

// Endpoint returns the Azure OpenAI resource endpoint.
func (a *AzureOpenAIAdapter) Endpoint() string { return a.endpoint }

// Deployment returns the model deployment name.
func (a *AzureOpenAIAdapter) Deployment() string { return a.deployment }
