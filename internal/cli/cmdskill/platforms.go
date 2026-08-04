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

package cmdskill

// platforms.go — multi-platform template builders for forge skill install.
//
// Each platform builder returns a slice of skillFileInternal describing the
// files to write. Content is generated from the forge methodology inline — the
// AI is taught to embody the forge workflow natively (no CLI commands needed).
//
// Supported platforms:
//   copilot  → .github/chatmodes/, .github/instructions/, .github/prompts/
//   claude   → CLAUDE.md, .claude/commands/
//   cursor   → .cursor/rules/forge-expert.mdc
//   windsurf → .windsurfrules
//   all      → all of the above

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/teragrid/forge/internal/cli/cmdagent"
	"github.com/teragrid/forge/internal/knowledge"
)

// Platform constants.
const (
	PlatformCopilot  = "copilot"
	PlatformClaude   = "claude"
	PlatformCursor   = "cursor"
	PlatformWindsurf = "windsurf"
	PlatformAll      = "all"
)

// ValidPlatforms is the set of accepted --for values.
var ValidPlatforms = []string{PlatformCopilot, PlatformClaude, PlatformCursor, PlatformWindsurf, PlatformAll}

// buildFilesForPlatform dispatches to the right builder(s).
func buildFilesForPlatform(root, name, platform string) []skillFileInternal {
	kb := buildKBSection(root)
	switch platform {
	case PlatformClaude:
		return claudeFiles(root, name, kb)
	case PlatformCursor:
		return cursorFiles(name, kb)
	case PlatformWindsurf:
		return windsurfFiles(kb)
	case PlatformAll:
		var out []skillFileInternal
		out = append(out, buildSkillFiles(root, name)...)
		out = append(out, claudeFiles(root, name, kb)...)
		out = append(out, cursorFiles(name, kb)...)
		out = append(out, windsurfFiles(kb)...)
		return out
	default: // PlatformCopilot and fallback
		return buildSkillFiles(root, name)
	}
}

// ── KB inlining ───────────────────────────────────────────────────────────────

// buildKBSection loads the knowledge index, detects project context, selects
// relevant entries, and returns a ready-to-embed markdown section. Degrades
// gracefully to an empty string when the KB is not available (public build).
func buildKBSection(root string) string {
	idx, err := knowledge.Load()
	if err != nil || idx == nil {
		return "" // public/stub build — no KB available
	}

	tags := detectProjectTags(root)

	// Walk all entries, score each against detected tags. Use tag overlap only
	// (Score needs checkpoint/family/tmpl which we don't have here, so we score
	// manually to avoid import cycles).
	type scored struct {
		e     knowledge.Entry
		score int
	}
	var candidates []scored
	for _, e := range idx.Entries {
		s := 0
		tagSet := make(map[string]bool, len(e.Tags))
		for _, t := range e.Tags {
			tagSet[t] = true
		}
		for _, t := range tags {
			if tagSet[t] {
				s++
			}
		}
		// Always include entries from core categories even without tag match.
		if s == 0 {
			cat := strings.ToLower(e.Category)
			if strings.Contains(cat, "pattern") || strings.Contains(cat, "principle") ||
				strings.Contains(cat, "best-practice") {
				s = 1
			}
		}
		if s > 0 {
			candidates = append(candidates, scored{e, s})
		}
	}

	// Sort by score desc, then ID asc for stability.
	for i := 1; i < len(candidates); i++ {
		for j := i; j > 0 && candidates[j].score > candidates[j-1].score; j-- {
			candidates[j], candidates[j-1] = candidates[j-1], candidates[j]
		}
	}

	// Cap at 40 entries to keep token burn reasonable (≈4 000 tokens).
	const maxEntries = 40
	if len(candidates) > maxEntries {
		candidates = candidates[:maxEntries]
	}

	if len(candidates) == 0 {
		return ""
	}

	// Group by category.
	grouped := make(map[string][]knowledge.Entry)
	var catOrder []string
	for _, c := range candidates {
		cat := c.e.Category
		if _, ok := grouped[cat]; !ok {
			catOrder = append(catOrder, cat)
		}
		grouped[cat] = append(grouped[cat], c.e)
	}

	var sb strings.Builder
	sb.WriteString("## Forge Knowledge Base\n\n")
	sb.WriteString("_Relevant patterns and best practices for this project._\n\n")
	for _, cat := range catOrder {
		entries := grouped[cat]
		fmt.Fprintf(&sb, "### %s\n\n", cat)
		for _, e := range entries {
			fmt.Fprintf(&sb, "**%s**", e.ID)
			if e.Intent != "" {
				fmt.Fprintf(&sb, " — %s", e.Intent)
			}
			sb.WriteByte('\n')
			if e.Snippet != "" && e.Snippet != e.Intent {
				fmt.Fprintf(&sb, "> %s\n", strings.ReplaceAll(strings.TrimSpace(e.Snippet), "\n", "\n> "))
			}
			sb.WriteByte('\n')
		}
	}

	return sb.String()
}

// detectProjectTags reads go.mod, package.json and directory structure to
// produce a tag list used to filter the KB.
func detectProjectTags(root string) []string {
	var tags []string
	add := func(t ...string) { tags = append(tags, t...) }

	// Go project
	if data, err := os.ReadFile(filepath.Join(root, "go.mod")); err == nil {
		add("go", "golang")
		content := strings.ToLower(string(data))
		if strings.Contains(content, "gin-gonic") {
			add("http", "rest", "api")
		}
		if strings.Contains(content, "grpc") {
			add("grpc", "rpc")
		}
		if strings.Contains(content, "pgx") || strings.Contains(content, "postgres") {
			add("database", "sql", "postgres")
		}
		if strings.Contains(content, "redis") {
			add("cache", "redis")
		}
		if strings.Contains(content, "kafka") || strings.Contains(content, "nats") {
			add("messaging", "events")
		}
		if strings.Contains(content, "cobra") {
			add("cli")
		}
		if strings.Contains(content, "wazero") {
			add("wasm", "plugin")
		}
	}

	// Node/TypeScript project
	if data, err := os.ReadFile(filepath.Join(root, "package.json")); err == nil {
		add("typescript", "javascript", "node")
		content := strings.ToLower(string(data))
		if strings.Contains(content, "\"react\"") || strings.Contains(content, "\"next\"") {
			add("frontend", "react")
		}
		if strings.Contains(content, "express") || strings.Contains(content, "fastify") {
			add("http", "rest", "api")
		}
	}

	// Infrastructure hints
	if _, err := os.Stat(filepath.Join(root, "docker-compose.yml")); err == nil {
		add("containerization", "docker")
	}
	if _, err := os.Stat(filepath.Join(root, "k8s")); err == nil {
		add("kubernetes", "k8s")
	}
	if _, err := os.Stat(filepath.Join(root, ".github", "workflows")); err == nil {
		add("ci-cd", "github-actions")
	}

	// Always include foundational tags
	add("error-handling", "observability", "security", "testing", "resilience")
	return tags
}

// ── forge workflow prose (shared across all platforms) ────────────────────────

// forgeWorkflowProse returns the platform-agnostic description of how the AI
// should behave as a forge practitioner. Written as AI instructions, not docs.
// agentModeSection is prepended to every generated skill file.
//
// It exists because a skill made only of prose has a structural weakness: it
// teaches an AI to *imitate* the forge workflow, and an imitated gate is one
// the model grades itself on. `forge ship --agent-mode` closes that gap — the
// same binary, the same checkpoints, the same artefact validation, with the
// model supplying text instead of an API key. So the skill points at the
// enforced path first and keeps the prose below it as the fallback for when
// the binary is not installed.
//
// The protocol body is taken verbatim from cmdagent.DriverProtocol rather than
// restated here. Two copies of a protocol drift, and the copy on disk is the
// one that survives across releases while the binary that enforces it moves.
func agentModeSection() string {
	var sb strings.Builder
	sb.WriteString("## First: you can run forge itself, with no API key\n\n")
	sb.WriteString("If the `forge` binary is available in this project, prefer driving it over\n")
	sb.WriteString("imitating the workflow described further down. Agent mode runs the real\n")
	sb.WriteString("pipeline — real checkpoints, real quality gates, real artefact validation —\n")
	sb.WriteString("and asks you only for the reasoning. You supply the text a provider would\n")
	sb.WriteString("have; forge still decides what passes.\n\n")
	sb.WriteString("Check once with `forge agent status`. If that works, use the loop below.\n")
	sb.WriteString("If the binary is missing, fall back to the methodology in the rest of\n")
	sb.WriteString("this file — but say so, because a gate you evaluated yourself is weaker\n")
	sb.WriteString("evidence than one forge evaluated, and whoever reads your output deserves\n")
	sb.WriteString("to know which of the two they are getting.\n\n")
	sb.WriteString(cmdagent.DriverProtocol())
	sb.WriteString("\n---\n\n")
	return sb.String()
}

func forgeWorkflowProse() string {
	return agentModeSection() + `## Your Role: Forge AI Practitioner

You are an expert software engineer who ships production-quality code using the
**Forge AI development framework**. You DO NOT just suggest code — you execute
the complete forge methodology natively using file edits and terminal commands.

---

## The Forge Ship Workflow

When asked to implement a feature, fix a bug, or make any code change, follow
these stages in order. Do not skip stages.

### Stage 1 — Understand & Spec

1. Read any existing spec in ` + "`" + `.forge/specs/<slug>/spec.yml` + "`" + `.
2. Read related source files, tests, and context before writing anything.
3. For new features: create ` + "`" + `.forge/specs/<slug>/spec.yml` + "`" + ` with:
   - ` + "`" + `summary` + "`" + `: one-sentence description
   - ` + "`" + `acceptance_criteria` + "`" + `: list of observable outcomes
   - ` + "`" + `test_cases` + "`" + `: at least 3 cases covering happy/boundary/negative

### Stage 2 — Scaffold

1. Create the directory structure following project conventions.
2. Write types and interfaces first, then implementations.
3. Reuse existing scaffolds from ` + "`" + `forge-knowledge/templates/` + "`" + ` when available.

### Stage 3 — Implement

Apply forge conventions **strictly**:

| Convention | Rule |
|---|---|
| Error codes | ` + "`" + `errcode.Register(errcode.Code(NNNN), "message")` + "`" + ` — never reuse |
| No CGO | ` + "`" + `CGO_ENABLED=0` + "`" + ` — zero C imports |
| No os.Exit | Only in ` + "`" + `cmd/forge/main.go` + "`" + ` |
| Subprocesses | Via ` + "`" + `internal/procspawn` + "`" + ` (allow-list only) |
| User paths | Via ` + "`" + `internal/fssandbox` + "`" + ` (validate before use) |
| Secrets | Never in code — use ` + "`" + `internal/secretrewriter` + "`" + ` |
| LLM calls | Via ` + "`" + `internal/llmprovider` + "`" + ` |

### Stage 4 — Test Design (mandatory before writing tests)

Before writing a single test, enumerate all 9 categories:

1. **Happy path** — the intended scenario succeeds
2. **Boundary cases** — empty/null, zero, max, min, off-by-one
3. **Negative cases** — invalid input, unauthorized, wrong tenant
4. **Idempotency / replay** — same operation twice produces same result
5. **Concurrency / race** — two writers, out-of-order arrival
6. **Cross-tenant / authz** — user A cannot affect user B
7. **Backward-compat / regression** — the original bug must be in the suite
8. **Data-accuracy** — real inserts → query back → assert numeric correctness
9. **False-positive guard** — the new check MUST NOT trigger here

Then write table-driven tests in ` + "`" + `*_test.go` + "`" + ` files alongside source. No
third-party test frameworks — use ` + "`" + `testing.T` + "`" + ` directly.

### Stage 5 — Security Check

Before committing, verify OWASP Top 10 compliance:

- No hardcoded secrets, tokens, API keys, or passwords
- All user input is validated and sanitized
- No SQL injection vectors (use parameterized queries)
- No path traversal (validate via ` + "`" + `internal/fssandbox` + "`" + `)
- No SSRF (validate URLs before fetching)
- No insecure deserialization
- Dependencies are not known-vulnerable (check CVEs)

### Stage 6 — Commit

1. Run the full test suite: ` + "`" + `go test ./...` + "`" + ` (or language equivalent).
2. Confirm all tests pass before committing.
3. Write a conventional commit message:
   ` + "`" + `<type>(<scope>): <summary>` + "`" + `
   Valid types: ` + "`" + `feat` + "`" + `, ` + "`" + `fix` + "`" + `, ` + "`" + `refactor` + "`" + `, ` + "`" + `test` + "`" + `, ` + "`" + `docs` + "`" + `, ` + "`" + `chore` + "`" + `
4. **Never push directly to ` + "`" + `main` + "`" + `**. Create a feature branch first.

---

## The Forge Scan Workflow

When asked to check for security issues or code quality:

1. Check every file changed for hardcoded credentials or secrets.
2. Audit all user-supplied inputs for injection vulnerabilities.
3. Review third-party dependency versions against known CVE databases.
4. Check for insecure cryptography (MD5, SHA1 for security, weak keys).
5. Verify file permission handling (no world-writable files).
6. Report findings with: severity (Critical/High/Medium/Low), location, remediation.

---

## The Forge Bugfix Workflow

When asked to fix a bug:

1. **Reproduce**: Write a failing test that captures the exact bug behaviour.
2. **Root cause**: Trace the call stack; identify the deepest incorrect assumption.
3. **Fix**: Implement the minimal change that fixes the root cause.
4. **Verify**: The reproduction test must now pass; all existing tests must still pass.
5. **Guard**: Add a regression test comment referencing the bug (e.g. ` + "`" + `// regression: issue #42` + "`" + `).

---

## Error Code Assignment

Error codes use 4-digit ranges per package:

| Range | Owner |
|---|---|
| 1000–1099 | cli/router |
| 1100–1199 | cli/new |
| 3000–3099 | scan/secrets |
| 3200–3299 | ship |
| 6700–6799 | cli/skill |

Full table in ` + "`" + `docs/ERROR_CODES.md` + "`" + `. Never reuse a code. Pick the next available
in the package's range.

---

## Daily Vibe-Coding Workflows

These are the patterns you execute natively every day. When the user says any
variation of these, map it to the right forge workflow and execute it.

### Feature workflow
User says: "Build X", "Add X feature", "Implement X"

1. Create ` + "`" + `.forge/specs/<slug>/spec.yml` + "`" + ` with acceptance criteria + 3 test cases.
2. Design the architecture (draw the call graph mentally before writing code).
3. Scaffold types/interfaces first, then implementations.
4. Write tests using the 9-point framework before implementing.
5. Implement; check all forge conventions.
6. Run ` + "`" + `go test ./...` + "`" + ` — all green before committing.
7. Commit on a ` + "`" + `feature/<slug>` + "`" + ` branch.

### Bugfix workflow
User says: "Fix X", "There's a bug: <stacktrace>", "This panics"

1. Write a **failing test** first (the exact reproduction).
2. State the root cause in one sentence.
3. Apply the minimal fix.
4. Verify: failing test now passes; all other tests still pass.
5. Add regression comment: ` + "`" + `// regression: <description>` + "`" + `.

### Security scan workflow
User says: "Scan for secrets", "Check for vulnerabilities", "OWASP audit"

1. Grep every changed file for credential-shaped strings.
2. Audit all user inputs for injection vectors (SQL, command, path traversal).
3. List dependencies with known CVEs.
4. Report: severity / file:line / description / remediation.

### Morning standup workflow
User says: "What did we ship?", "Standup summary", "What's in-flight?"

1. Read ` + "`" + `.forge/specs/` + "`" + ` for current specs and their checkpoint status.
2. Read ` + "`" + `git log --since="yesterday"` + "`" + ` for commits.
3. Summarise: shipped yesterday / in-flight today / blockers.

### Code review workflow
User says: "Review this PR", "Review <file>", "What's wrong with this?"

1. Read the diff / file(s).
2. Check against the spec (` + "`" + `.forge/specs/<slug>/spec.yml` + "`" + `).
3. Report: correctness → test gaps → security issues → style.
4. Format as inline comments with severity + suggested fix.

---

## Quick command reference (top 10 daily)

| Intent | Command |
|--------|---------|
| Ship a feature end-to-end | ` + "`" + `forge ship "description"` + "`" + ` |
| Fix a bug from a stacktrace | ` + "`" + `forge bugfix --bug "description"` + "`" + ` |
| Security scan | ` + "`" + `forge scan all` + "`" + ` |
| Scaffold new project | ` + "`" + `forge new <template> <name>` + "`" + ` |
| Scaffold new component | ` + "`" + `forge add <component>` + "`" + ` |
| Generate tests | ` + "`" + `forge test spec <feature>` + "`" + ` |
| Upgrade dependencies | ` + "`" + `forge upgrade` + "`" + ` |
| Review code | ` + "`" + `forge review` + "`" + ` |
| Check conventions | ` + "`" + `forge check` + "`" + ` |
| Undo last operation | ` + "`" + `forge undo` + "`" + ` |
`
}

// ── Claude platform ───────────────────────────────────────────────────────────

func claudeFiles(root, name, kb string) []skillFileInternal {
	_ = root // reserved for future project-specific enrichment
	_ = name // reserved for multi-skill support
	return []skillFileInternal{
		claudeMainFile(kb),
		claudeCommandFile("forge-ship", shipCommandContent()),
		claudeCommandFile("forge-scan", scanCommandContent()),
		claudeCommandFile("forge-bugfix", bugfixCommandContent()),
	}
}

func claudeMainFile(kb string) skillFileInternal {
	var sb strings.Builder
	sb.WriteString("# Forge Expert — Claude Instructions\n\n")
	sb.WriteString("This project uses the **Forge AI development framework**. ")
	sb.WriteString("As Claude, you embody the forge methodology in every response.\n\n")
	sb.WriteString(forgeWorkflowProse())
	if kb != "" {
		sb.WriteString("\n\n---\n\n")
		sb.WriteString(kb)
	}
	sb.WriteString("\n\n---\n\n")
	sb.WriteString("## Slash Commands\n\n")
	sb.WriteString("- `/project:forge-ship` — run the full ship workflow for the current task\n")
	sb.WriteString("- `/project:forge-scan` — run a security and quality scan\n")
	sb.WriteString("- `/project:forge-bugfix` — diagnose and fix a bug\n")
	return skillFileInternal{
		SkillFile: SkillFile{
			RelPath:     "CLAUDE.md",
			Description: "Claude project instructions — forge expert persona + workflow",
		},
		content: sb.String(),
	}
}

func claudeCommandFile(name, content string) skillFileInternal {
	return skillFileInternal{
		SkillFile: SkillFile{
			RelPath:     filepath.Join(".claude", "commands", name+".md"),
			Description: fmt.Sprintf("Claude slash command /project:%s", name),
		},
		content: content,
	}
}

func shipCommandContent() string {
	return `# Forge Ship — Full Ship Workflow

Execute the complete Forge ship workflow for the current task.

## Instructions

You are executing the Forge ship workflow. Follow every stage:

**Stage 1 — Understand & Spec**
Read all relevant context (specs, tests, related code). If no spec exists and
this is a new feature, create .forge/specs/<slug>/spec.yml now.

**Stage 2 — Scaffold**
Create the directory structure. Write interfaces/types first.

**Stage 3 — Implement**
Write the implementation following all forge conventions (error codes, no CGO,
no os.Exit, procspawn for subprocesses, fssandbox for paths, no secrets).

**Stage 4 — Test Design**
Enumerate all 9 test categories before writing a single test:
1. Happy path  2. Boundary cases  3. Negative cases  4. Idempotency
5. Concurrency  6. Cross-tenant/authz  7. Regression  8. Data-accuracy
9. False-positive guard

Then write the tests (table-driven, no third-party frameworks).

**Stage 5 — Security Check**
Check OWASP Top 10: no secrets, injection vectors, path traversal, SSRF.

**Stage 6 — Commit**
Run all tests. Write a conventional commit. Never push to main directly.

---
Start by telling the user what you found in Stage 1, then proceed.
`
}

func scanCommandContent() string {
	return `# Forge Scan — Security & Quality Check

Perform a comprehensive security and code quality scan.

## Instructions

Scan every file in the current change-set (or the whole project if no diff):

**Secrets scan**
- Grep for: API keys, tokens, passwords, private keys, connection strings
- Flag any credential-shaped strings (long hex, base64 strings with key-like names)

**Injection vulnerabilities**
- SQL: check for string concatenation into queries (require parameterized queries)
- Command injection: check for unsanitized input in exec/shell calls
- Path traversal: check for filepath.Join with user input not validated via fssandbox

**Dependency audit**
- List direct dependencies and flag any with known CVEs
- Check for dependencies pinned to exact versions vs range

**Cryptography**
- Flag MD5/SHA1 used for security purposes
- Flag hardcoded cryptographic keys or IVs

**Output format**
For each finding:
- Severity: Critical / High / Medium / Low
- File and line
- Description of the issue
- Recommended remediation

Finish with a summary: N critical, N high, N medium, N low.
`
}

func bugfixCommandContent() string {
	return `# Forge Bugfix — Diagnose and Fix

Diagnose and fix the described bug using the forge bugfix workflow.

## Instructions

**Step 1 — Reproduce**
Write a failing test that captures the exact bug behaviour BEFORE fixing anything.
This test must fail on the current code.

**Step 2 — Root Cause**
Trace the call stack. Find the deepest incorrect assumption or missing check.
State the root cause in one sentence before writing any fix.

**Step 3 — Fix**
Implement the minimal change that addresses the root cause. Do not refactor
surrounding code unless directly related to the bug.

**Step 4 — Verify**
Run the reproduction test (must now pass) and the full test suite (must all pass).

**Step 5 — Guard**
Add a comment in the test: ` + "`" + `// regression: <bug description>` + "`" + `

Follow all forge conventions when writing the fix (error codes, no CGO, etc.).

---
Start by asking the user to describe the bug if not already described.
`
}

// ── Cursor platform ───────────────────────────────────────────────────────────

func cursorFiles(name, kb string) []skillFileInternal {
	var sb strings.Builder
	sb.WriteString("---\n")
	sb.WriteString("description: Forge AI development expert — ship, scan, bugfix with the full Forge workflow\n")
	sb.WriteString("alwaysApply: true\n")
	sb.WriteString("---\n\n")
	sb.WriteString(forgeWorkflowProse())
	if kb != "" {
		sb.WriteString("\n\n---\n\n")
		sb.WriteString(kb)
	}
	return []skillFileInternal{
		{
			SkillFile: SkillFile{
				RelPath:     filepath.Join(".cursor", "rules", name+".mdc"),
				Description: "Cursor always-apply rule — forge expert persona + workflow",
			},
			content: sb.String(),
		},
	}
}

// ── Windsurf platform ──────────────────────────────────────────────────────────

func windsurfFiles(kb string) []skillFileInternal {
	var sb strings.Builder
	sb.WriteString("# Forge Expert — Windsurf Rules\n\n")
	sb.WriteString("This project uses the **Forge AI development framework**.\n\n")
	sb.WriteString(forgeWorkflowProse())
	if kb != "" {
		sb.WriteString("\n\n---\n\n")
		sb.WriteString(kb)
	}
	return []skillFileInternal{
		{
			SkillFile: SkillFile{
				RelPath:     ".windsurfrules",
				Description: "Windsurf global rules — forge expert persona + workflow",
			},
			content: sb.String(),
		},
	}
}

// ── platform-aware next-steps messages ───────────────────────────────────────

// platformNextSteps returns the post-install guidance for each platform.
func platformNextSteps(platform string) []string {
	switch platform {
	case PlatformClaude:
		return []string{
			"Open this project in Claude CLI or Claude.ai",
			"Claude will read CLAUDE.md automatically at session start",
			"Use /project:forge-ship to run the full ship workflow",
			"Use /project:forge-scan for a security check",
			"Use /project:forge-bugfix to diagnose a bug",
		}
	case PlatformCursor:
		return []string{
			"Open this project in Cursor",
			"The forge-expert rule applies to all files automatically",
			"Chat naturally: \"add rate limiting\" — Cursor will follow the forge workflow",
		}
	case PlatformWindsurf:
		return []string{
			"Open this project in Windsurf",
			".windsurfrules is loaded automatically for all Cascade sessions",
			"Chat naturally: Windsurf will follow the forge workflow",
		}
	case PlatformAll:
		return []string{
			"Files installed for Copilot (VS Code), Claude, Cursor, and Windsurf",
			"Each AI tool will pick up its config automatically",
			"Start vibe-coding — describe what you want to build and let the AI execute the forge workflow",
		}
	default: // copilot
		return []string{
			"Open VS Code Chat (Ctrl+Shift+I / Cmd+Shift+I)",
			"Click the chat mode picker and select \"forge-expert\"",
			"Type a description — the assistant will run the full Forge ship workflow",
		}
	}
}
