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

// llmpipe.go â€” LLM integration layer wiring llmprovider, secretrewriter, and
// tokenledger into a single callable unit for the forge ship checkpoint pipeline.
//
// Design
// â”€â”€â”€â”€â”€â”€
// Each forge ship checkpoint that benefits from AI assistance calls
// (*LLMPipe).Invoke. The pipe transparently:
//
//  1. Scrubs secrets from every prompt before dispatch (secretrewriter).
//  2. Calls the provider's Complete method (llmprovider.Provider).
//  3. Records per-call token usage in the JSONL ledger (tokenledger).
//
// All methods on a nil *LLMPipe are no-ops returning ("", nil) so callers
// can degrade gracefully when no provider is configured, without branching.
//
// Provider selection (via llmprovider.Detect):
//
//	ANTHROPIC_API_KEY â†’ AnthropicAdapter (Claude models)
//	OPENAI_API_KEY    â†’ OpenAIAdapter (GPT models)
//	neither           â†’ nil *LLMPipe (dry-run mode)
package cmdship

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/teragrid/forge/internal/llmprovider"
	"github.com/teragrid/forge/internal/secretrewriter"
	"github.com/teragrid/forge/internal/tokenledger"
)

// LLMPipe bundles an LLM Provider, a secret Rewriter, and a token Ledger.
type LLMPipe struct {
	provider llmprovider.Provider
	rewriter *secretrewriter.Rewriter
	ledger   *tokenledger.Ledger
}

// newLLMPipe detects the active LLM provider from the environment and returns
// an initialized *LLMPipe. Returns nil (not an error) if no provider is
// configured so callers silently fall back to structural dry-run behavior.
func newLLMPipe(root string) *LLMPipe {
	p, err := llmprovider.Detect()
	if err != nil {
		return nil
	}
	return newLLMPipeWithProvider(p, root)
}

// newLLMPipeWithProvider creates an LLMPipe backed by the given provider.
// Intended for tests â€” inject a *llmprovider.MockProvider to exercise LLM
// code paths without network calls.
func newLLMPipeWithProvider(p llmprovider.Provider, root string) *LLMPipe {
	return &LLMPipe{
		provider: p,
		rewriter: secretrewriter.New(),
		ledger:   tokenledger.New(filepath.Join(root, tokenledger.DefaultPath)),
	}
}

// Invoke sends a completion request to the provider after scrubbing secrets
// from both the system and user prompts, then records token usage in the ledger.
//
// Returns ("", nil) when called on a nil *LLMPipe (no provider configured).
// model may be "" to let the provider choose its default.
func (p *LLMPipe) Invoke(operation, model, system, user string, maxTokens int) (string, error) {
	if p == nil {
		return "", nil
	}
	sys := p.rewriter.Rewrite(system).Text
	usr := p.rewriter.Rewrite(user).Text

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	req := &llmprovider.Request{
		Model:        model,
		SystemPrompt: sys,
		UserPrompt:   usr,
		MaxTokens:    maxTokens,
	}
	resp, err := p.provider.Complete(ctx, req)
	if err != nil {
		return "", err
	}
	// Best-effort ledger append â€” a write failure never blocks the pipeline.
	_ = p.ledger.Append(tokenledger.Entry{
		Model:        resp.Model,
		InputTokens:  resp.InputTokens,
		OutputTokens: resp.OutputTokens,
		CostUSD:      estimateCost(resp.Model, resp.InputTokens, resp.OutputTokens),
		Operation:    operation,
	})
	return resp.Content, nil
}

// ProviderName returns the active provider name or "none" for a nil receiver.
func (p *LLMPipe) ProviderName() string {
	if p == nil {
		return "none"
	}
	return p.provider.Name()
}

// shipMessage returns the top-level status message for a ship run.
func shipMessage(pipe *LLMPipe) string {
	if pipe == nil {
		return "no LLM provider detected; set ANTHROPIC_API_KEY or OPENAI_API_KEY for AI-driven checkpoints"
	}
	return "LLM provider: " + pipe.ProviderName() + " (M1 HTTP transport pending; structural checkpoints active)"
}

// llmErrNote converts an LLM error into a concise user-facing note.
func llmErrNote(err error) string {
	if err == nil {
		return ""
	}
	s := err.Error()
	switch {
	case strings.Contains(s, "transport not implemented") ||
		strings.Contains(s, "FORGE-4051"):
		return "M1 HTTP transport pending"
	case strings.Contains(s, "FORGE-4050") ||
		strings.Contains(s, "no ANTHROPIC_API_KEY"):
		return "no provider configured"
	default:
		if len(s) > 80 {
			return s[:77] + "..."
		}
		return s
	}
}

// estimateCost returns an approximate USD cost for a completion call.
// Values are indicative only â€” intended for token-budget awareness, not billing.
func estimateCost(model string, inputTokens, outputTokens int) float64 {
	type rate struct{ in, out float64 } // USD per 1M tokens
	rates := map[string]rate{
		"claude-3-5-sonnet-20241022": {3.0, 15.0},
		"claude-3-5-haiku-20241022":  {0.8, 4.0},
		"claude-3-opus-20240229":     {15.0, 75.0},
		"gpt-4o":                     {2.5, 10.0},
		"gpt-4o-mini":                {0.15, 0.60},
		"gpt-4-turbo":                {10.0, 30.0},
	}
	r, ok := rates[model]
	if !ok {
		r = rate{3.0, 15.0} // conservative fallback
	}
	return (float64(inputTokens)*r.in + float64(outputTokens)*r.out) / 1_000_000
}

// â”€â”€ LLM generation helpers â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

// specStub returns a template spec Markdown file for the given description.
func specStub(description string) string {
	return fmt.Sprintf(
		"# Spec: %s\n\n"+
			"## What\n%s\n\n"+
			"## Why\n<!-- fill in business rationale -->\n\n"+
			"## Acceptance Criteria\n- [ ] <!-- add at least one criterion (Given/When/Then) -->\n\n"+
			"## Non-Functional Requirements\n- [ ] <!-- latency / throughput / availability -->\n\n"+
			"## Out of scope\n<!-- list explicit exclusions -->\n",
		description, description,
	)
}

// generateTestStubs asks the LLM to produce failing test stubs for the feature
// and writes them to .forge/specs/<slug>/test-stubs.md.
// Returns ("", nil) when description is empty or pipe is nil.
func generateTestStubs(root, description string, pipe *LLMPipe) (string, error) {
	if description == "" || pipe == nil {
		return "", nil
	}
	slug := slugify(description)
	specsDir := filepath.Join(root, ".forge", "specs", slug)

	// Include spec content as context if available.
	specContent := ""
	if data, err := os.ReadFile(filepath.Join(specsDir, "spec.md")); err == nil {
		specContent = string(data)
	}

	var userPrompt strings.Builder
	userPrompt.WriteString(fmt.Sprintf("Feature: %s\n\n", description))
	if specContent != "" {
		userPrompt.WriteString("Spec:\n")
		userPrompt.WriteString(specContent)
		userPrompt.WriteString("\n\n")
	}
	userPrompt.WriteString(
		"Generate failing unit test stubs in Go that will pass only after the feature is " +
			"implemented. Use table-driven tests. Include: happy path, boundary cases, and " +
			"at least one negative case. Wrap code in a ```go block.",
	)

	content, err := pipe.Invoke(
		"ship:test:generate", "",
		"You are a senior Go QA engineer generating test stubs for a TDD workflow. "+
			"Write idiomatic Go tests using the standard testing package. "+
			"Use t.Parallel() and t.TempDir() where appropriate.",
		userPrompt.String(),
		3000,
	)
	if err != nil {
		return "", err
	}
	if content == "" {
		return "", nil
	}
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		return content, nil // content available but can't write; not a hard error
	}
	_ = os.WriteFile(filepath.Join(specsDir, "test-stubs.md"), []byte(content), 0o600)
	return content, nil
}

// generateBreakdown asks the LLM to decompose the spec into an atomic task list
// and writes the result to .forge/specs/<slug>/breakdown.md.
// Returns ("", nil) when description is empty or pipe is nil.
func generateBreakdown(root, description string, pipe *LLMPipe) (string, error) {
	if description == "" || pipe == nil {
		return "", nil
	}
	slug := slugify(description)
	specsDir := filepath.Join(root, ".forge", "specs", slug)

	// Load spec for context.
	specContent := ""
	if data, err := os.ReadFile(filepath.Join(specsDir, "spec.md")); err == nil {
		specContent = string(data)
	}

	var userPrompt strings.Builder
	userPrompt.WriteString(fmt.Sprintf("Feature: %s\n\n", description))
	if specContent != "" {
		userPrompt.WriteString("Spec:\n")
		userPrompt.WriteString(specContent)
		userPrompt.WriteString("\n\n")
	}
	userPrompt.WriteString(
		"Decompose this feature into an atomic, ordered task list. " +
			"For each task include: task ID, title, description, estimated effort (XS/S/M/L), " +
			"dependencies, and acceptance criteria. Format as Markdown.",
	)

	content, err := pipe.Invoke(
		"ship:breakdown:generate", "",
		"You are a delivery lead decomposing a feature spec into an atomic task list "+
			"for a development team. Each task should be completable in â‰¤ 1 day.",
		userPrompt.String(),
		3000,
	)
	if err != nil {
		return "", err
	}
	if content == "" {
		return "", nil
	}
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		return content, nil
	}
	_ = os.WriteFile(filepath.Join(specsDir, "breakdown.md"), []byte(content), 0o600)
	return content, nil
}

// generateCodePlan asks the LLM for a step-by-step code implementation plan
// derived from the spec and breakdown, then writes it to
// .forge/specs/<slug>/code-plan.md.
// Returns ("", nil) when description is empty, pipe is nil, or no context exists.
func generateCodePlan(root, description string, pipe *LLMPipe) (string, error) {
	if description == "" || pipe == nil {
		return "", nil
	}
	slug := slugify(description)
	specsDir := filepath.Join(root, ".forge", "specs", slug)

	var sb strings.Builder
	if data, err := os.ReadFile(filepath.Join(specsDir, "spec.md")); err == nil {
		sb.WriteString("Spec:\n")
		sb.Write(data)
		sb.WriteString("\n\n")
	}
	if data, err := os.ReadFile(filepath.Join(specsDir, "breakdown.md")); err == nil {
		sb.WriteString("Breakdown:\n")
		sb.Write(data)
		sb.WriteString("\n\n")
	}
	if sb.Len() == 0 {
		return "", nil // nothing to ground the code plan in
	}

	content, err := pipe.Invoke(
		"ship:code:generate", "",
		"You are a senior software engineer. Given a feature spec and task breakdown, "+
			"produce a step-by-step code implementation plan. "+
			"For each task: which files to create/modify, key function signatures, "+
			"data structures, and test strategy. Format as Markdown.",
		fmt.Sprintf("Feature: %s\n\n%s", description, sb.String()),
		4000,
	)
	if err != nil {
		return "", err
	}
	if content == "" {
		return "", nil
	}
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		return content, nil
	}
	_ = os.WriteFile(filepath.Join(specsDir, "code-plan.md"), []byte(content), 0o600)
	return content, nil
}
