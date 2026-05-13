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

// Package prompttemplates provides per-verb LLM prompt templates (M3-26).
//
// Templates are Go text/template strings embedded in this package. Each verb
// has a system prompt and a user-turn template. Callers render the template
// with a TemplateData struct and pass the result to the LLM provider.
//
// Usage:
//
//	tmpl, err := prompttemplates.Load("review")
//	if err != nil { ... }
//	prompt, err := tmpl.Render(prompttemplates.TemplateData{
//	    Context: contextSnapshot,
//	    UserInput: diff,
//	})
//
// Templates are kept in sync with docs/ARCHITECTURE.md §5 "Prompt Design".
package prompttemplates

import (
	"bytes"
	"fmt"
	"text/template"
)

// TemplateData is the data passed to every verb prompt template.
type TemplateData struct {
	// Context is the project context snapshot (from forge context generate).
	Context string
	// UserInput is the verb-specific user-supplied content (e.g. diff, question).
	UserInput string
	// Files is a list of relevant file paths.
	Files []string
	// Extra holds verb-specific extra data.
	Extra map[string]string
}

// Prompt contains the rendered system and user prompts for one LLM call.
type Prompt struct {
	System string
	User   string
}

// Template is a compiled per-verb prompt template.
type Template struct {
	verb   string
	system *template.Template
	user   *template.Template
}

// Render executes the template with data and returns the rendered Prompt.
func (t *Template) Render(data TemplateData) (*Prompt, error) {
	var sys, usr bytes.Buffer
	if err := t.system.Execute(&sys, data); err != nil {
		return nil, fmt.Errorf("prompttemplates: render system prompt for %q: %w", t.verb, err)
	}
	if err := t.user.Execute(&usr, data); err != nil {
		return nil, fmt.Errorf("prompttemplates: render user prompt for %q: %w", t.verb, err)
	}
	return &Prompt{System: sys.String(), User: usr.String()}, nil
}

// Load returns the compiled Template for verb. Returns an error if the verb
// has no registered template.
func Load(verb string) (*Template, error) {
	entry, ok := registry[verb]
	if !ok {
		return nil, fmt.Errorf("prompttemplates: no template registered for verb %q", verb)
	}
	sysTmpl, err := template.New(verb + "/system").Parse(entry.system)
	if err != nil {
		return nil, fmt.Errorf("prompttemplates: parse system template for %q: %w", verb, err)
	}
	userTmpl, err := template.New(verb + "/user").Parse(entry.user)
	if err != nil {
		return nil, fmt.Errorf("prompttemplates: parse user template for %q: %w", verb, err)
	}
	return &Template{verb: verb, system: sysTmpl, user: userTmpl}, nil
}

// Verbs returns the list of verbs that have registered templates.
func Verbs() []string {
	out := make([]string, 0, len(registry))
	for v := range registry {
		out = append(out, v)
	}
	return out
}

type entry struct{ system, user string }

// registry maps verb name → (system, user) template strings.
var registry = map[string]entry{
	"ask": {
		system: `You are Forge, an expert software engineering assistant.
You have access to the following project context:
<context>
{{.Context}}
</context>
Answer questions accurately and concisely. When referencing code, use fenced code blocks.
If you are unsure, say so rather than guessing.`,
		user: `{{.UserInput}}`,
	},

	"review": {
		system: `You are an expert code reviewer with deep knowledge of security,
performance, correctness, maintainability, and engineering best practices.

Project context:
<context>
{{.Context}}
</context>

Review the diff provided by the user. For each finding, output a JSON object with:
  - "file": the file path
  - "line": the line number (best estimate)
  - "severity": "error" | "warning" | "suggestion"
  - "category": "security" | "correctness" | "performance" | "style" | "maintainability"
  - "message": a concise description of the issue
  - "suggestion": an optional concrete fix

Wrap the JSON array in <findings>...</findings> tags.
Focus on actionable issues. Do not repeat trivial style nits.`,
		user: `Review this diff:

<diff>
{{.UserInput}}
</diff>`,
	},

	"fix": {
		system: `You are an expert software engineer. Your job is to fix code issues.
You will be given a finding from the forge scan system and the relevant code context.
Produce a minimal, safe patch that addresses the issue without changing unrelated code.
Output only the corrected code (no explanation unless asked).`,
		user: `Fix the following issue:

Finding: {{index .Extra "finding"}}
File: {{index .Extra "file"}}

Code context:
<code>
{{.UserInput}}
</code>`,
	},

	"scan": {
		system: `You are a security and code-quality expert. You analyse code for issues
including: secrets, auth/authz flaws, prompt injection, supply-chain risks,
accessibility gaps, performance anti-patterns, and cost inefficiencies.

Project context:
<context>
{{.Context}}
</context>

For each issue found, output a JSON object matching the forge Finding schema:
{
  "rule_id": "FORGE-XXXX",
  "family": "security|correctness|performance|accessibility|cost",
  "severity": "error|warning|info",
  "file": "<path>",
  "line": <number>,
  "message": "<description>",
  "fix_hint": "<optional suggestion>"
}

Wrap all findings in a JSON array: {"findings": [...]}`,
		user: `Analyse the following files for issues:
{{range .Files}}- {{.}}
{{end}}
{{if .UserInput}}Additional context: {{.UserInput}}{{end}}`,
	},

	"ship": {
		system: `You are a senior engineer running the forge ship workflow.
Your role is to orchestrate the 5-checkpoint ship process:
1. spec — verify spec is present and approved
2. test — verify tests precede production code
3. breakdown — verify tasks are tracked
4. code — verify per-task diff loop is complete
5. ship — run final gates (scan, lint, hygiene, secrets)

Report your assessment for each checkpoint in JSON:
{"checkpoint": "<name>", "status": "pass|fail|skip", "reason": "<detail>"}`,
		user: `Run ship checkpoints for this project.
Context:
<context>
{{.Context}}
</context>
{{if .UserInput}}User request: {{.UserInput}}{{end}}`,
	},

	"optimize": {
		system: `You are a performance and cost optimization expert for AI-powered systems.
Analyse the provided code and token-spend data to identify:
1. LLM call hot-paths that can be cached or batched
2. Prompt redundancy (repeated context)
3. Model tier mismatches (using expensive models for cheap tasks)
4. Token budget overruns

Output recommendations as JSON:
{"recommendations": [{"area": "...", "impact": "high|medium|low", "action": "..."}]}`,
		user: `Optimize the following:
<context>
{{.Context}}
</context>
{{if .UserInput}}Additional input: {{.UserInput}}{{end}}`,
	},

	"learn": {
		system: `You are a learning-loop analyst for Forge. You analyse anonymised
prompt/outcome pairs to identify patterns: common errors, successful strategies,
and opportunities to improve default instructions packs.

Output a structured analysis in JSON:
{
  "top_errors": [{"pattern": "...", "count": N, "suggested_fix": "..."}],
  "winning_patterns": [{"description": "...", "frequency": "high|medium|low"}],
  "instruction_pack_suggestions": ["..."]
}`,
		user: `Analyse the following usage data:
{{.UserInput}}`,
	},
}
