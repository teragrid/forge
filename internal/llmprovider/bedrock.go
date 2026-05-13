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

// Package llmprovider — bedrock.go implements the AWS Bedrock adapter (M3-09).
//
// Detected when AWS_BEDROCK_REGION is set (along with standard AWS credentials
// AWS_ACCESS_KEY_ID / AWS_SECRET_ACCESS_KEY or an ambient IAM role).
//
// Default model: anthropic.claude-3-5-sonnet-20241022-v2:0
// Override via AWS_BEDROCK_MODEL env var or Request.Model.
//
// The actual AWS Bedrock API call (SigV4-signed HTTPS) is left to a concrete
// client; this adapter carries only the metadata required for detection and
// capability reporting.
package llmprovider

import (
	"context"
	"os"

	"github.com/teragrid/forge/internal/errcode"
)

// BedrockAdapter is a Provider skeleton for AWS Bedrock.
// HTTP transport (SigV4 signing) is intentionally not included here.
type BedrockAdapter struct {
	region string
	model  string
}

// newBedrockProvider returns a BedrockAdapter if AWS_BEDROCK_REGION is set,
// or nil otherwise.
func newBedrockProvider() *BedrockAdapter {
	region := os.Getenv("AWS_BEDROCK_REGION")
	if region == "" {
		return nil
	}
	model := os.Getenv("AWS_BEDROCK_MODEL")
	if model == "" {
		model = "anthropic.claude-3-5-sonnet-20241022-v2:0"
	}
	return &BedrockAdapter{region: region, model: model}
}

func (b *BedrockAdapter) Name() string { return "bedrock" }

func (b *BedrockAdapter) Capabilities() Capabilities {
	return Capabilities{
		Streaming: true,
		MaxTokens: 200000,
		Models: []string{
			"anthropic.claude-3-5-sonnet-20241022-v2:0",
			"anthropic.claude-3-5-haiku-20241022-v1:0",
			"anthropic.claude-3-opus-20240229-v1:0",
			"amazon.titan-text-express-v1",
			"meta.llama3-70b-instruct-v1:0",
		},
	}
}

// Complete is a stub that returns ErrProviderFail. Replace with a concrete
// Bedrock InvokeModel HTTP implementation when needed.
func (b *BedrockAdapter) Complete(_ context.Context, req *Request) (*Response, error) {
	if req == nil {
		return nil, errcode.New(ErrInvalidInput, "request must not be nil", nil)
	}
	return nil, errcode.New(ErrProviderFail,
		"BedrockAdapter.Complete: HTTP transport not implemented; wire a concrete SigV4 client", nil)
}

// Region returns the AWS region the adapter targets.
func (b *BedrockAdapter) Region() string { return b.region }

// DefaultModel returns the configured default model ID.
func (b *BedrockAdapter) DefaultModel() string { return b.model }
