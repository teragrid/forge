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

package llmprovider_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/teragrid/forge/internal/config"
	"github.com/teragrid/forge/internal/llmprovider"
)

// ── Provider interface contract ───────────────────────────────────────────────

// TestProviderInterface verifies that all three concrete types satisfy Provider.
func TestProviderInterface(t *testing.T) {
	t.Parallel()
	// Compile-time assertions via type assertion.
	var _ llmprovider.Provider = &llmprovider.AnthropicAdapter{}
	var _ llmprovider.Provider = &llmprovider.OpenAIAdapter{}
	var _ llmprovider.Provider = &llmprovider.MockProvider{}
}

// ── MockProvider happy path ───────────────────────────────────────────────────

func TestMock_DefaultResponse(t *testing.T) {
	t.Parallel()
	m := &llmprovider.MockProvider{}
	resp, err := m.Complete(context.Background(), &llmprovider.Request{
		UserPrompt: "hello",
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if resp.Content == "" {
		t.Error("expected non-empty content")
	}
}

func TestMock_CannedResponse(t *testing.T) {
	t.Parallel()
	m := &llmprovider.MockProvider{
		Response: &llmprovider.Response{Content: "the answer is 42", Model: "mock-v1"},
	}
	resp, err := m.Complete(context.Background(), &llmprovider.Request{UserPrompt: "q"})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if resp.Content != "the answer is 42" {
		t.Errorf("unexpected content: %q", resp.Content)
	}
}

func TestMock_CannedError(t *testing.T) {
	t.Parallel()
	m := &llmprovider.MockProvider{Err: fmt.Errorf("FORGE-4051 provider failed")}
	_, err := m.Complete(context.Background(), &llmprovider.Request{UserPrompt: "q"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestMock_CallCount(t *testing.T) {
	t.Parallel()
	m := &llmprovider.MockProvider{}
	for i := 0; i < 5; i++ {
		_, _ = m.Complete(context.Background(), &llmprovider.Request{UserPrompt: "x"})
	}
	if m.Calls() != 5 {
		t.Errorf("expected 5 calls, got %d", m.Calls())
	}
}

// ── Nil request guard ─────────────────────────────────────────────────────────

func TestMock_NilRequest(t *testing.T) {
	t.Parallel()
	m := &llmprovider.MockProvider{}
	_, err := m.Complete(context.Background(), nil)
	if err == nil {
		t.Fatal("expected ErrInvalidInput for nil request")
	}
	if !strings.Contains(err.Error(), "FORGE-4052") {
		t.Fatalf("expected FORGE-4052, got: %v", err)
	}
}

func TestAnthropicAdapter_NilRequest(t *testing.T) {
	t.Parallel()
	a := &llmprovider.AnthropicAdapter{}
	_, err := a.Complete(context.Background(), nil)
	if err == nil {
		t.Fatal("expected ErrInvalidInput")
	}
	if !strings.Contains(err.Error(), "FORGE-4052") {
		t.Fatalf("expected FORGE-4052, got: %v", err)
	}
}

// ── Adapter metadata ──────────────────────────────────────────────────────────

func TestAnthropicAdapter_Name(t *testing.T) {
	t.Parallel()
	a := &llmprovider.AnthropicAdapter{}
	if a.Name() != "anthropic" {
		t.Errorf("unexpected name: %q", a.Name())
	}
}

func TestOpenAIAdapter_Name(t *testing.T) {
	t.Parallel()
	o := &llmprovider.OpenAIAdapter{}
	if o.Name() != "openai" {
		t.Errorf("unexpected name: %q", o.Name())
	}
}

func TestAnthropicAdapter_Capabilities(t *testing.T) {
	t.Parallel()
	a := &llmprovider.AnthropicAdapter{}
	caps := a.Capabilities()
	if caps.MaxTokens == 0 {
		t.Error("MaxTokens must not be zero")
	}
	if len(caps.Models) == 0 {
		t.Error("Models list must not be empty")
	}
}

// ── Detect ────────────────────────────────────────────────────────────────────

func TestDetect_NoEnvVars(t *testing.T) {
	// Cannot use t.Parallel() with t.Setenv
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")

	_, err := llmprovider.Detect()
	if err == nil {
		t.Fatal("expected ErrNoProvider")
	}
	if !strings.Contains(err.Error(), "FORGE-4050") {
		t.Fatalf("expected FORGE-4050, got: %v", err)
	}
}

func TestDetect_AnthropicKeyPresent(t *testing.T) {
	// Cannot use t.Parallel() with t.Setenv
	t.Setenv("ANTHROPIC_API_KEY", "sk-ant-test-key-12345678901234567890")
	t.Setenv("OPENAI_API_KEY", "")

	p, err := llmprovider.Detect()
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if p.Name() != "anthropic" {
		t.Errorf("expected anthropic, got %q", p.Name())
	}
}

func TestDetect_OpenAIKeyPresent(t *testing.T) {
	// Cannot use t.Parallel() with t.Setenv
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "sk-test-key-12345678901234567890abcdef")

	p, err := llmprovider.Detect()
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if p.Name() != "openai" {
		t.Errorf("expected openai, got %q", p.Name())
	}
}

func TestDetect_AnthropicTakesPrecedence(t *testing.T) {
	// Cannot use t.Parallel() with t.Setenv
	t.Setenv("ANTHROPIC_API_KEY", "sk-ant-test-key-12345678901234567890")
	t.Setenv("OPENAI_API_KEY", "sk-test-key-12345678901234567890abcdef")

	p, err := llmprovider.Detect()
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if p.Name() != "anthropic" {
		t.Errorf("ANTHROPIC_API_KEY should take precedence; got %q", p.Name())
	}
}

// ── False-positive: stub adapters return ErrProviderFail, not nil ─────────────

func TestAnthropicAdapter_CompleteReturnsStubError(t *testing.T) {
	t.Parallel()
	a := &llmprovider.AnthropicAdapter{}
	_, err := a.Complete(context.Background(), &llmprovider.Request{UserPrompt: "test"})
	if err == nil {
		t.Fatal("stub adapter must not succeed without HTTP transport")
	}
	if !strings.Contains(err.Error(), "FORGE-4051") {
		t.Fatalf("expected FORGE-4051, got: %v", err)
	}
}

// ── WithActiveProfile / profileProvider ───────────────────────────────────────

// capturingProvider records the last Request passed to Complete.
type capturingProvider struct {
	lastReq *llmprovider.Request
}

func (c *capturingProvider) Name() string { return "capture" }
func (c *capturingProvider) Capabilities() llmprovider.Capabilities {
	return llmprovider.Capabilities{Models: []string{"capture-v1"}}
}
func (c *capturingProvider) Complete(_ context.Context, req *llmprovider.Request) (*llmprovider.Response, error) {
	c.lastReq = req
	return &llmprovider.Response{Content: "ok", Model: "capture-v1"}, nil
}

// TestWithActiveProfile_AppliesBudgetWhenMaxTokensZero verifies that when a
// profile is active and MaxTokens == 0, the profile's budget is applied.
func TestWithActiveProfile_AppliesBudgetWhenMaxTokensZero(t *testing.T) {
	// Cannot run parallel: mutates package-level profile state.
	fast, _ := config.GetProfile(config.ProfileFast)
	config.SetActiveProfile(fast)
	defer config.SetActiveProfile(config.Profile{}) // reset

	inner := &capturingProvider{}
	wrapped := llmprovider.WithActiveProfile(inner)

	req := &llmprovider.Request{UserPrompt: "hello", MaxTokens: 0}
	if _, err := wrapped.Complete(context.Background(), req); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if inner.lastReq == nil {
		t.Fatal("inner provider never called")
	}
	if inner.lastReq.MaxTokens != fast.MaxLLMTokenBudget {
		t.Errorf("MaxTokens: got %d, want %d (fast budget)", inner.lastReq.MaxTokens, fast.MaxLLMTokenBudget)
	}
	// Original request must not be mutated.
	if req.MaxTokens != 0 {
		t.Error("original request was mutated — profileProvider must copy")
	}
}

// TestWithActiveProfile_RespectsExplicitMaxTokens verifies that a non-zero
// MaxTokens set by the caller is never overridden by the profile.
func TestWithActiveProfile_RespectsExplicitMaxTokens(t *testing.T) {
	// Cannot run parallel: mutates package-level profile state.
	paranoid, _ := config.GetProfile(config.ProfileParanoid)
	config.SetActiveProfile(paranoid)
	defer config.SetActiveProfile(config.Profile{}) // reset

	inner := &capturingProvider{}
	wrapped := llmprovider.WithActiveProfile(inner)

	const explicitBudget = 512
	req := &llmprovider.Request{UserPrompt: "hello", MaxTokens: explicitBudget}
	if _, err := wrapped.Complete(context.Background(), req); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if inner.lastReq.MaxTokens != explicitBudget {
		t.Errorf("MaxTokens: got %d, want %d (caller's explicit value)", inner.lastReq.MaxTokens, explicitBudget)
	}
}

// TestWithActiveProfile_NoProfileNoBudget verifies that when no profile is
// active, MaxTokens is forwarded unchanged.
func TestWithActiveProfile_NoProfileNoBudget(t *testing.T) {
	// Cannot run parallel: mutates package-level profile state.
	config.SetActiveProfile(config.Profile{}) // ensure zero-value / cleared

	inner := &capturingProvider{}
	wrapped := llmprovider.WithActiveProfile(inner)

	req := &llmprovider.Request{UserPrompt: "hello", MaxTokens: 0}
	if _, err := wrapped.Complete(context.Background(), req); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if inner.lastReq.MaxTokens != 0 {
		t.Errorf("expected MaxTokens 0 (no profile), got %d", inner.lastReq.MaxTokens)
	}
}

// TestWithActiveProfile_DelegatesName verifies that Name() delegates to the inner provider.
func TestWithActiveProfile_DelegatesName(t *testing.T) {
	t.Parallel()
	inner := &capturingProvider{}
	wrapped := llmprovider.WithActiveProfile(inner)
	if wrapped.Name() != "capture" {
		t.Errorf("Name: got %q, want %q", wrapped.Name(), "capture")
	}
}
