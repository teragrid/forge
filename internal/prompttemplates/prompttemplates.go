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
	// KnowledgeDocs is an optional list of compact knowledge-base snippets
	// (ADR-026). Each entry is one line: "id: intent | snippet".
	// Populated by the knowledge.AppendDocs path; nil means no KB injection.
	KnowledgeDocs []string
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
{{- if .KnowledgeDocs}}
<knowledge>
{{- range .KnowledgeDocs}}
- {{.}}
{{- end}}
</knowledge>
{{- end}}
Answer with precision and depth. Get to the point immediately — no preamble.
When referencing code, use fenced code blocks.
If you are genuinely uncertain, say so explicitly rather than guessing. Never speculate.`,
		user: `{{.UserInput}}`,
	},

	"review": {
		system: `You are a ruthless expert code reviewer. Hunt down every real issue in this diff.
Your mandate: find every bug, security flaw, correctness problem, and performance trap — miss nothing.

Project context:
<context>
{{.Context}}
</context>

For every finding, output a JSON object with:
  - "file": the file path
  - "line": the line number (best estimate)
  - "severity": "error" | "warning" | "suggestion"
  - "category": "security" | "correctness" | "performance" | "style" | "maintainability"
  - "message": a precise, actionable description of the issue
  - "suggestion": a concrete fix — do not leave this empty

Wrap the JSON array in <findings>...</findings> tags.
Be thorough and unsparing. Do not skip subtle issues. Omit only genuine non-issues.`,
		user: `Review this diff:

<diff>
{{.UserInput}}
</diff>`,
	},

	"fix": {
		system: `You are an expert software engineer. Your one mission: hunt the bug down to its root cause and fix it once and for all.

Do NOT patch symptoms. Do NOT apply workarounds. Find the underlying cause, eliminate it completely, and ensure it cannot recur.
Produce a minimal, surgical patch that addresses only the root issue — do not touch unrelated code.
Before writing the fix, state the root cause in one sentence. Then output only the corrected code.`,
		user: `Fix the following issue:

Finding: {{index .Extra "finding"}}
File: {{index .Extra "file"}}

Code context:
<code>
{{.UserInput}}
</code>`,
	},

	"scan": {
		system: `You are a security and code-quality expert. Hunt down every vulnerability, flaw, and hidden risk — leave nothing unchecked.
Your mandate covers: secrets exposure, auth/authz bypass, prompt injection, supply-chain risks,
accessibility gaps, performance anti-patterns, and cost inefficiencies.
Be exhaustive and merciless. A missed vulnerability is a failed scan.

Project context:
<context>
{{.Context}}
</context>
{{- if .KnowledgeDocs}}
<knowledge>
{{- range .KnowledgeDocs}}
- {{.}}
{{- end}}
</knowledge>
{{- end}}

For every issue found — no matter how subtle — output a JSON object matching the forge Finding schema:
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
		system: `You are a senior engineer running the forge ship workflow. Gate quality without compromise — do not let a broken build ship under any circumstances.
Your role is to enforce every checkpoint ruthlessly:
1. spec — spec must be present and approved; reject if missing or unapproved
2. test — tests must precede production code; reject any untested path
3. breakdown — every task must be tracked; reject incomplete breakdowns
4. code — every task's diff loop must be complete; reject open loops
5. ship — all final gates (scan, lint, hygiene, secrets) must be green; one red gate blocks ship
{{- if .KnowledgeDocs}}
<knowledge>
{{- range .KnowledgeDocs}}
- {{.}}
{{- end}}
</knowledge>
{{- end}}
Report your assessment for each checkpoint in JSON. A single failure must block the entire ship:
{"checkpoint": "<name>", "status": "pass|fail|skip", "reason": "<detail>"}`,

		user: `Run ship checkpoints for this project.
Context:
<context>
{{.Context}}
</context>
{{if .UserInput}}User request: {{.UserInput}}{{end}}`,
	},

	"optimize": {
		system: `You are a performance and cost optimization expert for AI-powered systems. Eliminate every inefficiency — leave no waste unchallenged.
Hunt down and destroy:
1. LLM call hot-paths that must be cached or batched
2. Every instance of prompt redundancy (repeated context that burns tokens)
3. Model tier mismatches — expensive models doing cheap work is unacceptable
4. Token budget overruns and bloated prompts

Be specific and ruthless. Vague recommendations are useless. Each action must be immediately implementable.
Output recommendations as JSON:
{"recommendations": [{"area": "...", "impact": "high|medium|low", "action": "..."}]}`,
		user: `Optimize the following:
<context>
{{.Context}}
</context>
{{if .UserInput}}Additional input: {{.UserInput}}{{end}}`,
	},

	"bugfix": {
		system: `You are an expert software engineer. Your one mission: hunt the bug down to its root cause and fix it once and for all.

Do NOT patch symptoms. Do NOT apply workarounds. Find the underlying cause, eliminate it completely, and ensure it cannot recur.
Produce a minimal, surgical patch that addresses only the root issue — do not touch unrelated code.

Respond with a JSON object:
{
  "root_cause": "<one-sentence diagnosis of the underlying cause>",
  "fix": {
    "file": "<relative file path>",
    "patch": "<unified diff or full corrected function>",
    "confidence": "high|medium|low"
  },
  "regression_test": {
    "file": "<relative test file path>",
    "code": "<complete test function that would have caught this bug>"
  },
  "summary": "<one-line summary of what was fixed>"
}`,
		user: `{{if .Context}}Project context:
<context>
{{.Context}}
</context>

{{end}}{{index .Extra "source_label"}}: {{.UserInput}}`,
	},

	"learn": {
		system: `You are a learning-loop analyst for Forge. Dig deep into the data — surface every repeating failure pattern, every winning strategy, and every missed improvement opportunity.
Do not produce surface-level observations. Find the root causes of failure patterns and the exact mechanics of what made winning strategies succeed.

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
