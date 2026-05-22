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

// llmpipe.go — LLM integration layer wiring llmprovider, secretrewriter, and
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
//	ANTHROPIC_API_KEY → AnthropicAdapter (Claude models)
//	OPENAI_API_KEY    → OpenAIAdapter (GPT models)
//	neither           → nil *LLMPipe (dry-run mode)
package cmdship

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/teragrid/forge/internal/knowledge"
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
// Intended for tests — inject a *llmprovider.MockProvider to exercise LLM
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
	// Best-effort ledger append — a write failure never blocks the pipeline.
	_ = p.ledger.Append(tokenledger.Entry{
		Model:        resp.Model,
		InputTokens:  resp.InputTokens,
		OutputTokens: resp.OutputTokens,
		CostUSD:      estimateCost(resp.Model, resp.InputTokens, resp.OutputTokens),
		Operation:    operation,
	})
	return resp.Content, nil
}

// InvokeWithKnowledge enriches the system prompt with relevant knowledge-base
// entries (ADR-026) before calling Invoke. Knowledge injection is best-effort:
// any failure (e.g. missing index file) falls back to plain Invoke.
//
// checkpoint, family, tmpl, and tags are forwarded to knowledge.Select to
// score entries. Only entries above MinScore are appended, and
// AppendDocsBudgeted ensures the total prompt stays within maxTokens.
func (p *LLMPipe) InvokeWithKnowledge(operation, model, system, user string, maxTokens int, checkpoint, family, tmpl string, tags []string) (string, error) {
	if p == nil {
		return "", nil
	}
	idx, err := knowledge.Load()
	if err != nil {
		// Graceful degradation: log nothing (no PII risk), proceed without KB.
		return p.Invoke(operation, model, system, user, maxTokens)
	}
	entries := knowledge.Select(idx, checkpoint, family, tmpl, tags)
	enriched := knowledge.AppendDocsBudgeted(system, entries, maxTokens)
	return p.Invoke(operation, model, enriched, user, maxTokens)
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
// Values are indicative only — intended for token-budget awareness, not billing.
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
	// Include OpenAPI contract when available — enables typed test stubs per operation.
	apiStyle := "rest"
	if data, err := os.ReadFile(filepath.Join(specsDir, "openapi.yaml")); err == nil {
		apiStyle = detectAPIStyle(string(data))
		userPrompt.WriteString("OpenAPI Contract:\n```yaml\n")
		userPrompt.Write(data)
		userPrompt.WriteString("\n```\n\n")
	}
	testInstruction := "Generate failing unit test stubs in Go that will pass only after the feature is " +
		"implemented. Use table-driven tests. Include: happy path, boundary cases, and " +
		"at least one negative case. For each API operation defined in the OpenAPI contract " +
		"include a corresponding test case. Wrap code in a ```go block."
	if apiStyle == "supabase-rpc" {
		testInstruction += " For Supabase RPC operations (/rest/v1/rpc/{fn}), include tests that " +
			"verify the RPC returns the correct JSON shape defined in the OpenAPI response schema, " +
			"and cover the database-function behaviour via an integration test with a test schema."
	}
	userPrompt.WriteString(testInstruction)

	content, err := pipe.InvokeWithKnowledge(
		"ship:test:generate", "",
		"You are a senior Go QA engineer generating test stubs for a TDD workflow. "+
			"Write idiomatic Go tests using the standard testing package. "+
			"Use t.Parallel() and t.TempDir() where appropriate.",
		userPrompt.String(),
		3000,
		"test", "testing", "go-service",
		[]string{"openapi", "tdd", "test-stubs"},
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
	// Include OpenAPI contract when available — enables typed task breakdown per endpoint.
	breakdownAPIStyle := "rest"
	if data, err := os.ReadFile(filepath.Join(specsDir, "openapi.yaml")); err == nil {
		breakdownAPIStyle = detectAPIStyle(string(data))
		userPrompt.WriteString("OpenAPI Contract:\n```yaml\n")
		userPrompt.Write(data)
		userPrompt.WriteString("\n```\n\n")
	}
	breakdownInstruction := "Decompose this feature into an atomic, ordered task list. " +
		"For each task include: task ID, title, description, estimated effort (XS/S/M/L), " +
		"dependencies, and acceptance criteria. " +
		"If an OpenAPI contract is provided, include a task for implementing each API operation " +
		"and a task for OpenAPI schema validation. Format as Markdown."
	if breakdownAPIStyle == "supabase-rpc" {
		breakdownInstruction += " For Supabase RPC operations: include tasks to create the PostgreSQL " +
			"function, grant execute permissions, add RLS policies, and write an integration test " +
			"that calls the function via the Supabase client."
	}
	userPrompt.WriteString(breakdownInstruction)

	content, err := pipe.InvokeWithKnowledge(
		"ship:breakdown:generate", "",
		"You are a delivery lead decomposing a feature spec into an atomic task list "+
			"for a development team. Each task should be completable in ≤ 1 day.",
		userPrompt.String(),
		3000,
		"breakdown", "", "dab",
		[]string{"openapi", "task-decomposition"},
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

	// G-007: also write tasks.md — machine-parseable checkbox list alongside breakdown.md.
	tasksContent := extractTasksMD(description, content)
	_ = os.WriteFile(filepath.Join(specsDir, "tasks.md"), []byte(tasksContent), 0o600)

	return content, nil
}

// extractTasksMD converts an LLM-generated breakdown into a machine-parseable
// tasks.md with a `- [ ] T-NNN: <title>` checkbox per task.
// If the LLM produced numbered lines (1., 2., …) those are extracted.
// Otherwise the whole breakdown is treated as a single task.
func extractTasksMD(feature, breakdown string) string {
	var sb strings.Builder
	sb.WriteString("# Tasks: ")
	sb.WriteString(feature)
	sb.WriteString("\n\n")

	// Try to extract numbered lines.
	lines := strings.Split(breakdown, "\n")
	taskNum := 0
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Match: "1. Foo" or "1) Foo" or "## Task 1" or "**T-001**" or "- " as task bullet.
		isTask := false
		title := ""
		if len(line) > 3 {
			if (line[1] == '.' || line[1] == ')') && line[0] >= '1' && line[0] <= '9' {
				isTask = true
				title = strings.TrimSpace(line[2:])
			} else if strings.HasPrefix(line, "## ") || strings.HasPrefix(line, "### ") {
				isTask = true
				title = strings.TrimPrefix(strings.TrimPrefix(line, "### "), "## ")
			}
		}
		if isTask && title != "" {
			taskNum++
			sb.WriteString(fmt.Sprintf("- [ ] T-%03d: %s\n", taskNum, title))
		}
	}

	if taskNum == 0 {
		// Fallback: treat the whole breakdown as one task.
		taskNum++
		sb.WriteString(fmt.Sprintf("- [ ] T-%03d: Implement %s\n", taskNum, feature))
	}

	return sb.String()
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
	// Include OpenAPI contract when available — enables typed implementation plan per endpoint.
	codeAPIStyle := "rest"
	if data, err := os.ReadFile(filepath.Join(specsDir, "openapi.yaml")); err == nil {
		codeAPIStyle = detectAPIStyle(string(data))
		sb.WriteString("OpenAPI Contract:\n```yaml\n")
		sb.Write(data)
		sb.WriteString("\n```\n\n")
	}
	if sb.Len() == 0 {
		return "", nil // nothing to ground the code plan in
	}

	codeSystem := "You are a senior software engineer. Given a feature spec and task breakdown, " +
		"produce a step-by-step code implementation plan. " +
		"For each task: which files to create/modify, key function signatures, " +
		"data structures, and test strategy. Format as Markdown."
	if codeAPIStyle == "supabase-rpc" {
		codeSystem += " For Supabase RPC endpoints: show the CREATE OR REPLACE FUNCTION SQL, " +
			"the GRANT EXECUTE statement, and the Go/TypeScript client call using the " +
			"Supabase client's .rpc() method with typed parameters matching the OpenAPI schema."
	}

	content, err := pipe.InvokeWithKnowledge(
		"ship:code:generate", "",
		codeSystem,
		fmt.Sprintf("Feature: %s\n\n%s", description, sb.String()),
		4000,
		"code", "", "go-service",
		[]string{"openapi", "implementation-plan"},
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
