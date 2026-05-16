// Copyright 2024 The Forge Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// G-103: first-class LLM adapter contract — typed prompts, streaming,
// structured outputs, and tool calling.
package llmprovider

import "context"

// ── Tool calling ─────────────────────────────────────────────────────────────

// Tool describes a function the LLM may invoke during a completion.
type Tool struct {
	// Name is the stable identifier for the tool (no spaces).
	Name string
	// Description is the LLM-readable description of what the tool does.
	Description string
	// InputSchema is the JSON Schema string describing the tool's input object.
	InputSchema string
}

// ToolCall is a tool invocation requested by the LLM.
type ToolCall struct {
	// ToolName is the name of the tool the LLM wants to call.
	ToolName string
	// Input is the tool's input as a parsed JSON object.
	Input map[string]any
	// CallID is an opaque identifier from the provider for pairing
	// the call with a ToolResult.
	CallID string
}

// ToolResult is the host's response to a ToolCall.
type ToolResult struct {
	// CallID must match ToolCall.CallID.
	CallID string
	// Output is the result of calling the tool (arbitrary string or JSON).
	Output string
	// Error is non-empty if the tool invocation failed.
	Error string
}

// ── Streaming ────────────────────────────────────────────────────────────────

// StreamChunk is one delta emitted during a streaming completion.
type StreamChunk struct {
	// Delta is the incremental text fragment.
	Delta string
	// Done is true for the final chunk; subsequent reads return io.EOF.
	Done bool
	// InputTokens and OutputTokens are populated only on the final chunk.
	InputTokens  int
	OutputTokens int
}

// ── Structured output ────────────────────────────────────────────────────────

// StructuredRequest wraps a Request with additional constraints for providers
// that support native structured-output mode (e.g. OpenAI response_format).
type StructuredRequest struct {
	Request
	// ResponseSchema is a JSON Schema string describing the expected response
	// object. Providers that support structured output will enforce this.
	ResponseSchema string
	// Tools is the list of tools the LLM may call during this completion.
	Tools []Tool
	// ToolResults are the results of previous ToolCalls to include in context.
	ToolResults []ToolResult
}

// StructuredResponse extends Response with tool-calling results.
type StructuredResponse struct {
	Response
	// ToolCalls contains any tool invocations requested by the LLM.
	ToolCalls []ToolCall
	// StopReason is "end_turn", "tool_use", "max_tokens", etc.
	StopReason string
}

// ── Extended adapter interface ────────────────────────────────────────────────

// AdvancedProvider extends Provider with streaming and tool-calling methods.
// Adapters may optionally implement this interface; callers should type-assert
// before using advanced features.
type AdvancedProvider interface {
	Provider
	// Stream starts a streaming completion. The returned channel receives
	// StreamChunks in order; the channel is closed after the final chunk.
	Stream(ctx context.Context, req *Request) (<-chan StreamChunk, error)
	// CompleteStructured sends a completion that may include tool calls and/or
	// return a structured JSON output.
	CompleteStructured(ctx context.Context, req *StructuredRequest) (*StructuredResponse, error)
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// Supports reports whether provider implements AdvancedProvider.
func Supports(p Provider) bool {
	_, ok := p.(AdvancedProvider)
	return ok
}

// G-046: Streaming early-stop for structured outputs.
// StreamUntilComplete reads from a streaming channel and returns the
// accumulated text. It stops early if stopFn returns true for a given
// accumulated string (e.g. when a valid JSON object is detected).
// If stopFn is nil, all chunks are consumed until Done.
func StreamUntilComplete(
	ctx context.Context,
	ch <-chan StreamChunk,
	stopFn func(accumulated string) bool,
) (string, int, int, error) {
	var accumulated string
	var inputTokens, outputTokens int
	for {
		select {
		case <-ctx.Done():
			return accumulated, inputTokens, outputTokens, ctx.Err()
		case chunk, ok := <-ch:
			if !ok {
				return accumulated, inputTokens, outputTokens, nil
			}
			accumulated += chunk.Delta
			if chunk.InputTokens > 0 {
				inputTokens = chunk.InputTokens
			}
			if chunk.OutputTokens > 0 {
				outputTokens = chunk.OutputTokens
			}
			if chunk.Done {
				return accumulated, inputTokens, outputTokens, nil
			}
			// Early-stop: invoke the predicate after each delta.
			if stopFn != nil && stopFn(accumulated) {
				return accumulated, inputTokens, outputTokens, nil
			}
		}
	}
}
