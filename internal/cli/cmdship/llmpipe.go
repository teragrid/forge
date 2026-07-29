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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/teragrid/forge/internal/contextbudgeter"
	"github.com/teragrid/forge/internal/knowledge"
	"github.com/teragrid/forge/internal/llmprovider"
	"github.com/teragrid/forge/internal/secretrewriter"
	"github.com/teragrid/forge/internal/tierrouter"
	"github.com/teragrid/forge/internal/tokenledger"
)

// LLMPipe bundles an LLM Provider, a secret Rewriter, a token Ledger, and
// an optional tier router for complexity-driven model selection.
type LLMPipe struct {
	provider llmprovider.Provider
	rewriter *secretrewriter.Rewriter
	ledger   *tokenledger.Ledger
	// router is used for complexity-based tier routing (T0→T1→T2 escalation).
	// A nil router falls back to direct provider calls.
	router *tierrouter.Router
	// minTier is the minimum tier forced by complexity classification.
	// Empty string means use T0 (cheapest available tier).
	minTier string
	// root is the project root used for layered knowledge-base loading.
	root string
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

// newLLMPipeInteractive is like newLLMPipe but prompts the user for an API key
// when no provider is detected and dryRun is false. In dry-run mode it falls
// back to nil silently, matching the old behaviour.
func newLLMPipeInteractive(root string, dryRun bool) *LLMPipe {
	if dryRun {
		return newLLMPipe(root)
	}
	p, err := llmprovider.DetectOrPrompt(nil, nil) // nil → os.Stdin / os.Stderr
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
		router:   tierrouter.New(p, nil), // P1: cheap-first tier escalation
		root:     root,
	}
}

// SetComplexityTier maps a ComplexityTier to the minimum tier for LLM routing.
// nano/micro → T0 (cheap), standard → T1 (balanced), complex → T2 (powerful).
// Call this once per pipeline run after classifyComplexity is known.
func (p *LLMPipe) SetComplexityTier(ct ComplexityTier) {
	if p == nil {
		return
	}
	switch ct {
	case ComplexityComplex:
		p.minTier = tierrouter.TierPowerful
	case ComplexityStandard:
		p.minTier = tierrouter.TierBalanced
	default: // nano, micro
		p.minTier = tierrouter.TierCheap
	}
}

// Invoke sends a completion request to the provider after scrubbing secrets
// from both the system and user prompts, then records token usage in the ledger.
//
// Returns ("", nil) when called on a nil *LLMPipe (no provider configured).
// model may be "" to let the provider choose its default.
func (p *LLMPipe) Invoke(operation, model, system, user string, maxTokens int) (string, error) {
	content, _, err := p.InvokeChecked(operation, model, system, user, maxTokens)
	return content, err
}

// InvokeChecked is Invoke plus the provider's own truncation signal
// (llmprovider.Response.Truncated / tierrouter.RouteResult.Truncated): true
// when the completion was cut off by hitting maxTokens rather than finishing
// naturally. Callers that write LLM output to disk as a checkpoint artefact
// (see generateWithValidation) should treat truncated == true as authoritative
// — it comes from the API's own stop/finish reason, not a guess from the
// shape of the returned text.
func (p *LLMPipe) InvokeChecked(operation, model, system, user string, maxTokens int) (content string, truncated bool, err error) {
	if p == nil {
		return "", false, nil
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

	// P1: use the tier router when available to select the right model tier
	// based on complexity (SetComplexityTier must be called before Invoke).
	var (
		respModel    string
		inputTokens  int
		outputTokens int
	)
	if p.router != nil {
		result, routeErr := p.router.Route(ctx, *req, p.minTier)
		if routeErr != nil {
			return "", false, routeErr
		}
		content = result.Response
		respModel = result.ModelUsed
		inputTokens = result.TokensIn
		outputTokens = result.TokensOut
		truncated = result.Truncated
	} else {
		resp, respErr := p.provider.Complete(ctx, req)
		if respErr != nil {
			return "", false, respErr
		}
		content = resp.Content
		respModel = resp.Model
		inputTokens = resp.InputTokens
		outputTokens = resp.OutputTokens
		truncated = resp.Truncated
	}

	// Best-effort ledger append — a write failure never blocks the pipeline.
	_ = p.ledger.Append(tokenledger.Entry{
		Model:        respModel,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		CostUSD:      estimateCost(respModel, inputTokens, outputTokens),
		Operation:    operation,
	})
	return content, truncated, nil
}

// InvokeWithKnowledge enriches the system prompt with relevant knowledge-base
// entries (ADR-026) before calling Invoke. Knowledge injection is best-effort:
// any failure (e.g. missing index file) falls back to plain Invoke.
//
// checkpoint, family, tmpl, and tags are forwarded to knowledge.Select to
// score entries. Only entries above MinScore are appended, and
// AppendDocsBudgeted ensures the total prompt stays within the available input
// capacity (total context window − output budget − current prompt estimate).
func (p *LLMPipe) InvokeWithKnowledge(operation, model, system, user string, maxTokens int, checkpoint, family, tmpl string, tags []string) (string, error) {
	content, _, err := p.InvokeWithKnowledgeChecked(operation, model, system, user, maxTokens, checkpoint, family, tmpl, tags)
	return content, err
}

// InvokeWithKnowledgeChecked is InvokeWithKnowledge plus the truncation
// signal — see InvokeChecked.
func (p *LLMPipe) InvokeWithKnowledgeChecked(operation, model, system, user string, maxTokens int, checkpoint, family, tmpl string, tags []string) (content string, truncated bool, err error) {
	if p == nil {
		return "", false, nil
	}
	// P1: use layered KB (project > user > embedded) when root is known.
	var idx *knowledge.Index
	if p.root != "" {
		idx, err = knowledge.LoadLayered(p.root)
	} else {
		idx, err = knowledge.Load()
	}
	if err != nil {
		// Graceful degradation: log nothing (no PII risk), proceed without KB.
		return p.InvokeChecked(operation, model, system, user, maxTokens)
	}
	entries := knowledge.Select(idx, checkpoint, family, tmpl, tags)

	// P0 bug fix: compute the true available input capacity for KB injection.
	// maxTokens is the *output* budget; the KB must fit in the *remaining input* space.
	// kbBudget = total context window − output reserve − current prompt estimate.
	const minKBBudget = 200 // below this it is not worth injecting KB
	caps := p.provider.Capabilities()
	promptTokens := contextbudgeter.EstimateTokens(system) + contextbudgeter.EstimateTokens(user)
	kbBudget := caps.MaxTokens - maxTokens - promptTokens
	if kbBudget < minKBBudget {
		kbBudget = 0 // prompts already fill the window; skip KB injection
	}

	enriched := knowledge.AppendDocsBudgeted(system, entries, kbBudget)
	return p.InvokeChecked(operation, model, enriched, user, maxTokens)
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
		return "tip: set ANTHROPIC_API_KEY or OPENAI_API_KEY to unlock AI-driven spec generation, arch docs, and code plans"
	}
	return "LLM provider: " + pipe.ProviderName()
}

// extractAPIErrorMessage pulls the human-readable "message" field out of a
// provider error body shaped like {"error":{"message":"..."}} or
// {"error":{"type":"...","message":"..."}} (the shape used by both Anthropic
// and GitHub Copilot/OpenAI-compatible APIs). Returns "" if the string isn't
// JSON or has no such field, so callers can fall back to the raw text.
func extractAPIErrorMessage(s string) string {
	start := strings.IndexByte(s, '{')
	if start < 0 {
		return ""
	}
	var body struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
		Message string `json:"message"` // some providers put it at the top level
	}
	if err := json.Unmarshal([]byte(s[start:]), &body); err != nil {
		return ""
	}
	if body.Error.Message != "" {
		return body.Error.Message
	}
	return body.Message
}

// llmErrNote converts an LLM error into a concise user-facing note for the
// single-line checkpoint summary. Prefers the API's own "message" field
// (the actual root cause, e.g. "Your credit balance is too low...") over a
// blind character truncation, which previously often cut the message off
// exactly before the useful part (e.g. "...last error: copilot: model
// \"claude-sonnet-4-6\" ..." with the real reason never shown). For the full,
// untruncated error — e.g. when writing to .forge/learned/*.jsonl for later
// diagnosis by a human or an LLM driving forge — use llmErrFull instead.
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
		if msg := extractAPIErrorMessage(s); msg != "" {
			if len(msg) > 160 {
				return msg[:157] + "..."
			}
			return msg
		}
		if len(s) > 160 {
			return s[:157] + "..."
		}
		return s
	}
}

// llmErrFull returns the complete, untruncated error text — for persisted
// diagnostics (.forge/learned/*.jsonl) where losing the root cause to
// truncation defeats the entire purpose of the log. Unlike llmErrNote, this
// is never shortened.
func llmErrFull(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
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
			"## Status Summary\n"+
			"- Lifecycle: Draft\n"+
			"- Version Scope: PATCH (default; confirm before release)\n"+
			"- Owner: <!-- team/person -->\n"+
			"- Last Updated: <!-- YYYY-MM-DD -->\n"+
			"- Checkpoint Progress: 1/7\n\n"+
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
// specName, when non-empty, overrides the slug derived from description — the
// caller (checkTest) already resolves --name/-n vs. the description-derived
// slug, and must pass the same value here so the two agree on the directory.
// Returns ("", nil) when description is empty or pipe is nil.
func generateTestStubs(root, description, specName string, pipe *LLMPipe) (string, error) {
	if description == "" || pipe == nil {
		return "", nil
	}
	slug := specName
	if slug == "" {
		slug = slugify(description)
	}
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
	// J4 (fix-checkpoint-llm-quality-and-observability): use the project's
	// actually-detected language/framework, not a hardcoded Go assumption —
	// confirmed dogfooding this checkpoint on a TypeScript/Jest project
	// produced Go net/http test stubs that couldn't even compile there.
	fw := detectTestFramework(root)
	lang, langLabel, codeFence, tddHint := testStubLanguageProfile(fw)

	testInstruction := fmt.Sprintf(
		"Generate failing unit test stubs in %s that will pass only after the feature is "+
			"implemented. Include: happy path, boundary cases, and at least one negative case. "+
			"For each API operation defined in the OpenAPI contract include a corresponding test case. "+
			"Wrap code in a ```%s block.", langLabel, codeFence)
	if apiStyle == "supabase-rpc" {
		testInstruction += " For Supabase RPC operations (/rest/v1/rpc/{fn}), include tests that " +
			"verify the RPC returns the correct JSON shape defined in the OpenAPI response schema, " +
			"and cover the database-function behaviour via an integration test with a test schema."
	}
	userPrompt.WriteString(testInstruction)

	genFn := func() (string, bool, error) {
		return pipe.InvokeWithKnowledgeChecked(
			"ship:test:generate", "",
			fmt.Sprintf("You are a senior %s QA engineer generating test stubs for a TDD workflow. %s",
				langLabel, tddHint),
			userPrompt.String(),
			// 5000: was 3000 — a stub suite covering happy/boundary/negative
			// cases per OpenAPI operation routinely exceeds that. See the
			// ship:spec:review budget comment (ship.go) for the same
			// undersized-budget root cause.
			5000,
			"test", "testing", lang,
			[]string{"openapi", "tdd", "test-stubs"},
		)
	}
	// J8/J9: this write path previously had no truncation/preamble check at
	// all (unlike spec.md/arch.md) — a cut-off or preamble-prefixed response
	// would be written to test-stubs.md as if it succeeded.
	content, complete, err := generateWithValidation(genFn)
	if err != nil {
		return "", err
	}
	if content == "" {
		return "", nil
	}
	if !complete {
		return "", fmt.Errorf("test stub generation truncated/incomplete after retry")
	}
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		return content, nil // content available but can't write; not a hard error
	}
	_ = os.WriteFile(filepath.Join(specsDir, "test-stubs.md"), []byte(content), 0o600)
	return content, nil
}

// testStubLanguageProfile maps a detected TestFrameworkContext to the prompt
// language name, knowledge-tag slug, code-fence language, and a short
// framework-specific TDD hint used to build generateTestStubs' prompt (J4).
// Defaults to TypeScript/Jest — the project's original G-006 assumption —
// when no language was detected, matching writeTestArtifactsWithContext's
// own default branch.
func testStubLanguageProfile(fw TestFrameworkContext) (lang, langLabel, codeFence, tddHint string) {
	switch fw.Language {
	case "go":
		return "go", "Go", "go",
			"Write idiomatic Go tests using the standard testing package. Use t.Parallel() and t.TempDir() where appropriate."
	case "python":
		return "python", "Python", "python",
			"Write idiomatic Python tests using pytest. Use fixtures where appropriate."
	case "java":
		return "java", "Java", "java",
			"Write idiomatic Java tests using JUnit 5. Use @TempDir where appropriate."
	default:
		return "typescript", "TypeScript", "typescript",
			"Write idiomatic TypeScript tests using Jest. Tests must compile but fail at runtime until implemented."
	}
}

// generateBreakdown asks the LLM to decompose the spec into an atomic task list
// and writes the result to .forge/specs/<slug>/breakdown.md.
// specName, when non-empty, overrides the slug derived from description — see
// generateTestStubs for why the caller-resolved slug must be threaded through.
// Returns ("", nil) when description is empty or pipe is nil.
func generateBreakdown(root, description, specName string, pipe *LLMPipe) (string, error) {
	if description == "" || pipe == nil {
		return "", nil
	}
	slug := specName
	if slug == "" {
		slug = slugify(description)
	}
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

	genFn := func() (string, bool, error) {
		return pipe.InvokeWithKnowledgeChecked(
			"ship:breakdown:generate", "",
			"You are a delivery lead decomposing a feature spec into an atomic task list "+
				"for a development team. Each task should be completable in ≤ 1 day.",
			userPrompt.String(),
			// 12000: was 6000 (before that, 3000) — a task-by-task breakdown
			// (ID/title/description/effort/dependencies/AC per task) for a
			// non-trivial feature routinely exceeds that; the token ledger
			// shows this call landing at *exactly* the configured budget on
			// every real run observed (3000 on 2026-07-13/14, 6000 on
			// 2026-07-23, twice in a row including the generateWithValidation
			// retry) — i.e. this checkpoint has never once finished under its
			// own budget, only ever been cut off by it. Note: the
			// AnthropicAdapter.continueOnMaxTokens continuation (see
			// anthropic.go) did not visibly engage on the 2026-07-23 runs
			// (no "continued generation" stderr message, and output_tokens
			// landed at exactly the budget rather than the budget plus a
			// partial continuation) — left as an open question for this call
			// path specifically; raising the base budget is the safe fix
			// regardless of whether that's resolved.
			12000,
			"breakdown", "", "dab",
			[]string{"openapi", "task-decomposition"},
		)
	}
	// J8/J9: this write path previously had no truncation/preamble check at
	// all (unlike spec.md/arch.md) — a cut-off or preamble-prefixed response
	// would be written to breakdown.md as if it succeeded.
	content, complete, err := generateWithValidation(genFn)
	if err != nil {
		return "", err
	}
	if content == "" {
		return "", nil
	}
	if !complete {
		return "", fmt.Errorf("breakdown generation truncated/incomplete after retry")
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
// specName, when non-empty, overrides the slug derived from description — see
// generateTestStubs for why the caller-resolved slug must be threaded through.
// Returns ("", nil) when description is empty, pipe is nil, or no context exists.
func generateCodePlan(root, description, specName string, pipe *LLMPipe) (string, error) {
	if description == "" || pipe == nil {
		return "", nil
	}
	slug := specName
	if slug == "" {
		slug = slugify(description)
	}
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

// ── L1: Stripped Debate Overhead ──────────────────────────────────────────────

// InvokeDebateRound runs a single lean debate turn for one role persona.
// Unlike InvokeWithKnowledge, this method intentionally skips KB lookup and
// steering injection — it uses only a minimal persona preamble (~150 tokens)
// plus a compact debate instruction (~100 tokens) per turn.
//
// This reduces per-debate-call cost by ~51% vs full InvokeWithKnowledge
// (~47,700 tokens saved per feature across all debate rounds).
func (p *LLMPipe) InvokeDebateRound(operation, model, persona, artefact, priorConcerns string, maxTokens int) (string, error) {
	if p == nil {
		return "", nil
	}
	system := buildDebateSystemPrompt(persona)
	user := buildDebateUserPrompt(artefact, priorConcerns)
	return p.Invoke(operation, model, system, user, maxTokens)
}

// buildDebateSystemPrompt returns a minimal persona system prompt (~150 tokens).
func buildDebateSystemPrompt(persona string) string {
	return "You are " + persona + ". Review the artefact and raise " +
		"concrete concerns from your area of expertise. Be specific, brief, " +
		"and actionable. Focus only on gaps that could cause implementation risk."
}

// buildDebateUserPrompt constructs the compact debate user message.
func buildDebateUserPrompt(artefact, priorConcerns string) string {
	var sb strings.Builder
	sb.WriteString("## Artefact\n")
	sb.WriteString(artefact)
	if priorConcerns != "" {
		sb.WriteString("\n\n## Prior Concerns\n")
		sb.WriteString(priorConcerns)
	}
	sb.WriteString("\n\n## Your review:")
	return sb.String()
}

// ── L3: Model Tier Resolution ──────────────────────────────────────────────────

// resolveModel selects the appropriate model for the given tier.
// If requestedModel is non-empty it is returned unchanged (caller wins).
// Otherwise the provider's capability list is used: Models[0] = most capable
// (tier-1 / heavy), Models[last] = fastest / cheapest (tier-2 / fast).
func (p *LLMPipe) resolveModel(requestedModel, tier string) string {
	if requestedModel != "" {
		return requestedModel
	}
	if p == nil {
		return ""
	}
	caps := p.provider.Capabilities()
	if len(caps.Models) == 0 {
		return ""
	}
	switch tier {
	case ModelTierHeavy:
		return caps.Models[0] // most capable model
	case ModelTierFast:
		return caps.Models[len(caps.Models)-1] // fastest / cheapest model
	}
	return ""
}

// InvokeForPhase is like Invoke but resolves the model via the phase's ModelTier
// before dispatching. Use this inside phase execution loops.
func (p *LLMPipe) InvokeForPhase(operation, system, user string, maxTokens int, tier string) (string, error) {
	return p.Invoke(operation, p.resolveModel("", tier), system, user, maxTokens)
}

// InvokeWithKnowledgeForPhase is like InvokeWithKnowledge but resolves the
// model from the phase's ModelTier.
func (p *LLMPipe) InvokeWithKnowledgeForPhase(operation, system, user string, maxTokens int, tier, checkpoint, family, tmpl string, tags []string) (string, error) {
	return p.InvokeWithKnowledge(operation, p.resolveModel("", tier), system, user, maxTokens, checkpoint, family, tmpl, tags)
}

// ── L7: Prefix Cache Exploit ──────────────────────────────────────────────────

// FrozenKBEntries holds KB entries pre-selected once at the start of a
// checkpoint so that all sub-calls reuse the same selection and benefit from
// provider-side prefix caching.
type FrozenKBEntries struct {
	entries []knowledge.Entry
}

// FreezeKBSelection pre-selects KB entries once for the given query parameters.
// Pass the returned *FrozenKBEntries to InvokeWithFrozenKB for all sub-calls
// within a single checkpoint.  This prevents repeated re-selection (cache miss)
// and saves ~$0.05–0.09 per feature across 17 debate sub-calls.
func (p *LLMPipe) FreezeKBSelection(checkpoint, family, tmpl string, tags []string) *FrozenKBEntries {
	if p == nil {
		return &FrozenKBEntries{}
	}
	idx, err := knowledge.Load()
	if err != nil {
		return &FrozenKBEntries{}
	}
	return &FrozenKBEntries{entries: knowledge.Select(idx, checkpoint, family, tmpl, tags)}
}

// InvokeWithFrozenKB is like InvokeWithKnowledge but uses pre-selected KB
// entries.  The KB budget is computed the same way as InvokeWithKnowledge.
func (p *LLMPipe) InvokeWithFrozenKB(operation, model, system, user string, maxTokens int, frozen *FrozenKBEntries) (string, error) {
	if p == nil {
		return "", nil
	}
	var entries []knowledge.Entry
	if frozen != nil {
		entries = frozen.entries
	}
	const minKBBudget = 200
	caps := p.provider.Capabilities()
	promptTokens := contextbudgeter.EstimateTokens(system) + contextbudgeter.EstimateTokens(user)
	kbBudget := caps.MaxTokens - maxTokens - promptTokens
	if kbBudget < minKBBudget {
		kbBudget = 0
	}
	enriched := knowledge.AppendDocsBudgeted(system, entries, kbBudget)
	return p.Invoke(operation, model, enriched, user, maxTokens)
}

// ── P1: Adaptive Token Budget ─────────────────────────────────────────────────

// ScaledBudget multiplies base by the complexity-tier multiplier and returns
// the result, clamped to a minimum of 256 tokens.
//
// Multipliers per tier (RFC-005 §3.2):
//
//	nano     → 0.7× (config change / UI tweak)
//	micro    → 1.0× (new endpoint + CRUD)
//	standard → 1.5× (new service / schema migration)
//	complex  → 2.0× (cross-service epic / compliance-sensitive)
//
// Pass the result as maxTokens to any Invoke variant so that LLM output
// budgets auto-scale with feature complexity rather than using hard-coded
// per-checkpoint constants.
func ScaledBudget(base int, tier ComplexityTier) int {
	multipliers := map[ComplexityTier]float64{
		ComplexityNano:     0.7,
		ComplexityMicro:    1.0,
		ComplexityStandard: 1.5,
		ComplexityComplex:  2.0,
	}
	m, ok := multipliers[tier]
	if !ok {
		m = 1.0
	}
	result := int(float64(base) * m)
	const minBudget = 256
	if result < minBudget {
		return minBudget
	}
	return result
}
