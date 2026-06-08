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

// Package llmresponse implements the standard JSON envelope emitted by all
// forge commands when running in LLM mode (FORGE_LLM_MODE=1, --json flag,
// NO_COLOR=1, or non-TTY stdout).
//
// Design: every response is self-contained — context_summary, next_actions,
// and remedy give an LLM everything it needs to continue without reading files
// or consulting docs.
package llmresponse

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// Status values for Response.Status.
const (
	StatusCompleted = "completed"
	StatusSkipped   = "skipped"
	StatusPending   = "pending"
	StatusRunning   = "running"
	StatusFailed    = "failed"
)

// Response is the standard JSON envelope emitted in LLM mode.
// All fields except Path and Checkpoint are always present (never omitted).
type Response struct {
	OK             bool         `json:"ok"`
	Checkpoint     string       `json:"checkpoint,omitempty"`
	Status         string       `json:"status"`
	Path           string       `json:"path,omitempty"`
	ContextSummary string       `json:"context_summary"`
	NextActions    []string     `json:"next_actions"`
	LLMTokensUsed  int          `json:"llm_tokens_used"`
	CostUSD        float64      `json:"cost_usd"`
	DurationMS     int64        `json:"duration_ms"`
	Error          *ErrorDetail `json:"error,omitempty"`
	// GateAutoApproved is set when an interactive gate was suppressed in LLM mode.
	GateAutoApproved bool `json:"gate_auto_approved,omitempty"`
}

// ErrorDetail carries structured error information for LLM consumption.
type ErrorDetail struct {
	Code    string `json:"code"`    // FORGE-XXXX
	Message string `json:"message"` // human-readable description
	// Remedy is a complete, copy-pasteable shell command or edit instruction
	// that resolves this error. Always present when ok=false.
	Remedy string `json:"remedy"`
}

type contextKey struct{}

// llmModeKey is the context key for LLM mode flag.
var llmModeKey = contextKey{}

// WithLLMMode returns a context with the LLM mode flag set.
func WithLLMMode(ctx context.Context, enabled bool) context.Context {
	return context.WithValue(ctx, llmModeKey, enabled)
}

// IsLLMMode reports whether the context has LLM mode enabled.
func IsLLMMode(ctx context.Context) bool {
	v, _ := ctx.Value(llmModeKey).(bool)
	return v
}

// Options configures a Wrap call.
type Options struct {
	Checkpoint     string
	Status         string
	Path           string
	ContextSummary string
	NextActions    []string
	LLMTokensUsed  int
	CostUSD        float64
	StartTime      time.Time
	// Err, if non-nil, sets ok=false and populates Error field.
	Err error
	// GateAutoApproved records that a prompt was suppressed.
	GateAutoApproved bool
}

// Wrap builds a Response from options. If opts.Err is non-nil the response has
// ok=false; otherwise ok=true.
func Wrap(opts Options) Response {
	r := Response{
		OK:               opts.Err == nil,
		Checkpoint:       opts.Checkpoint,
		Status:           opts.Status,
		Path:             opts.Path,
		ContextSummary:   opts.ContextSummary,
		NextActions:      opts.NextActions,
		LLMTokensUsed:    opts.LLMTokensUsed,
		CostUSD:          opts.CostUSD,
		GateAutoApproved: opts.GateAutoApproved,
	}
	if r.NextActions == nil {
		r.NextActions = []string{}
	}
	if !opts.StartTime.IsZero() {
		r.DurationMS = time.Since(opts.StartTime).Milliseconds()
	}
	if opts.Err != nil {
		r.Status = StatusFailed
		r.Error = errorDetailFromErr(opts.Err)
	}
	return r
}

// errorDetailFromErr extracts an ErrorDetail from an error, populating
// remedy from the errcode registry when available.
func errorDetailFromErr(err error) *ErrorDetail {
	if err == nil {
		return nil
	}
	d := &ErrorDetail{
		Code:    "FORGE-0000",
		Message: err.Error(),
	}
	// Try to extract forge error code via interface.
	type forgeErr interface {
		ForgeCode() string
		ForgeRemedy() string
	}
	if fe, ok := err.(forgeErr); ok {
		d.Code = fe.ForgeCode()
		d.Remedy = fe.ForgeRemedy()
	}
	if d.Remedy == "" {
		d.Remedy = fmt.Sprintf("forge doctor  # run diagnostics, then retry")
	}
	return d
}

// Write serialises r as a single JSON line to w.
func (r Response) Write(w io.Writer) error {
	b, err := json.Marshal(r)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(w, string(b))
	return err
}
