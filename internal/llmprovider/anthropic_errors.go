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

// anthropic_errors.go — typed classification of Anthropic API error
// responses (fix-checkpoint-llm-quality-and-observability J2;
// dynamic-fault-tolerant-model-selection AC1).
//
// Anthropic returns a JSON error envelope on non-200 responses:
//
//	{"type":"error","error":{"type":"not_found_error","message":"..."}}
//
// Previously Complete()'s error switch branched only on bare HTTP status
// codes (401/429/default) with no JSON body parsing at all, so a 404 (dead
// model id) produced the exact same generic error string as every other
// failure — impossible to distinguish programmatically or act on
// automatically. classifyAnthropicError parses this envelope (when present)
// and combines it with the HTTP status code to produce a typed *errcode.Error
// with a distinct, actionable message per failure class.
package llmprovider

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/teragrid/forge/internal/errcode"
)

// Reserved error codes within the existing llmprovider range (4050-4099,
// see ErrNoProvider/ErrProviderFail/ErrInvalidInput in llmprovider.go).
var (
	// ErrModelNotFound is returned when Anthropic reports a 404 /
	// not_found_error — the model id is stale or deprecated upstream. This
	// is the only failure class that is safe to retry against a different
	// model (dynamic-fault-tolerant-model-selection AC2).
	ErrModelNotFound = errcode.Register(errcode.Code(4053),
		"LLM model not found or deprecated (Anthropic not_found_error)")
	// ErrRateLimited is returned on a 429 / rate_limit_error — transient,
	// not a candidate for model fallback.
	ErrRateLimited = errcode.Register(errcode.Code(4054),
		"LLM provider rate limit exceeded (Anthropic rate_limit_error)")
	// ErrInvalidRequest is returned on a 400 / invalid_request_error — a
	// malformed request, not a model-availability problem.
	ErrInvalidRequest = errcode.Register(errcode.Code(4055),
		"LLM request rejected as invalid (Anthropic invalid_request_error)")
	// ErrAuthFailed is returned on a 401 — API key invalid/expired.
	ErrAuthFailed = errcode.Register(errcode.Code(4056),
		"LLM provider authentication failed")
)

// anthropicErrorBody is the JSON error envelope Anthropic returns on non-200
// responses.
type anthropicErrorBody struct {
	Type  string `json:"type"`
	Error struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

// parseAnthropicError parses raw into the typed error envelope. ok is false
// when the body isn't valid JSON in the expected shape (defensive — callers
// fall back to classifying by HTTP status code alone).
func parseAnthropicError(raw []byte) (errType, message string, ok bool) {
	var body anthropicErrorBody
	if err := json.Unmarshal(raw, &body); err != nil {
		return "", "", false
	}
	if body.Error.Type == "" {
		return "", "", false
	}
	return body.Error.Type, body.Error.Message, true
}

// classifyAnthropicError builds a typed *errcode.Error for a non-200
// Anthropic response, distinguishing not_found_error / rate_limit_error /
// invalid_request_error / auth failures from a generic catch-all — the
// errcode.New(...) pattern already used elsewhere in this file, rather than
// introducing a third error-handling style (bare fmt.Errorf strings).
func classifyAnthropicError(statusCode int, model string, raw []byte) *errcode.Error {
	errType, message, _ := parseAnthropicError(raw)

	switch {
	case statusCode == http.StatusNotFound || errType == "not_found_error":
		hint := fmt.Sprintf(
			"HTTP 404 — model %q not found (permanent: the model id is stale/deprecated upstream, not "+
				"a transient failure). Remove any explicit model: pin in forge.yml / ANTHROPIC_MODEL to "+
				"enable auto-fallback to a known-good model, or update the pin to a currently-valid id.",
			model)
		if message != "" {
			hint += " API message: " + message
		}
		return errcode.New(ErrModelNotFound, hint, fmt.Errorf("anthropic: raw response: %s", string(raw)))

	case statusCode == http.StatusTooManyRequests || errType == "rate_limit_error":
		hint := "HTTP 429 — rate limit exceeded (transient; retry after a short backoff, or reduce concurrency)."
		if message != "" {
			hint += " API message: " + message
		}
		return errcode.New(ErrRateLimited, hint, fmt.Errorf("anthropic: raw response: %s", string(raw)))

	case statusCode == http.StatusUnauthorized || errType == "authentication_error":
		hint := "HTTP 401 — API key invalid or expired. Set ANTHROPIC_API_KEY or re-authenticate with the Claude Code CLI."
		if message != "" {
			hint += " API message: " + message
		}
		return errcode.New(ErrAuthFailed, hint, fmt.Errorf("anthropic: raw response: %s", string(raw)))

	case statusCode == http.StatusBadRequest || errType == "invalid_request_error":
		hint := fmt.Sprintf("HTTP %d — invalid request (permanent; not a model-fallback candidate).", statusCode)
		if message != "" {
			hint += " API message: " + message
		}
		return errcode.New(ErrInvalidRequest, hint, fmt.Errorf("anthropic: raw response: %s", string(raw)))

	default:
		hint := fmt.Sprintf("API error %d", statusCode)
		if message != "" {
			hint += ": " + message
		} else {
			hint += ": " + string(raw)
		}
		return errcode.New(ErrProviderFail, hint, fmt.Errorf("anthropic: raw response: %s", string(raw)))
	}
}
