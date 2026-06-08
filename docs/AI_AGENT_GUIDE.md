# Forge — AI Agent Guide

> **Audience:** AI agents, LLMs, and coding assistants that invoke Forge via
> MCP tools, CLI subprocess calls, or CI pipelines.  
> Human users: start with [GETTING_STARTED.md](../GETTING_STARTED.md).

---

## Contents

1. [What Forge does (capability map)](#1-what-forge-does)
2. [Install (non-interactive)](#2-install-non-interactive)
3. [Verify the install](#3-verify-the-install)
4. [Connect Forge to an AI chat (MCP)](#4-connect-forge-to-an-ai-chat-mcp)
5. [MCP tool reference](#5-mcp-tool-reference)
6. [Full command reference](#6-full-command-reference)
7. [Common agent task patterns](#7-common-agent-task-patterns)
8. [Error handling and codes](#8-error-handling-and-error-codes)
9. [Key file paths and formats](#9-key-file-paths-and-formats)
10. [Environment variables](#10-environment-variables)
11. [Exit codes](#11-exit-codes)

---

## 1. What Forge Does

Forge is a single-binary CLI (`forge`) that wraps these capabilities:

| Capability | Verbs |
|---|---|
| Project scaffolding | `new`, `init`, `templates`, `tsd` |
| Security scanning | `scan secrets`, `scan prompt-injection`, `scan supply-chain`, `scan all` |
| Convention linting | `lint`, `clean` |
| Quality gate / ship pipeline | `ship`, `check` |
| Test generation & execution | `test spec`, `test run`, `test unit`, `test e2e` |
| Bug diagnosis & patching | `bugfix` |
| LLM spend control | `spend set`, `spend status` |
| Audit ledger | `audit show`, `audit verify`, `audit export` |
| Context generation | `context generate` |
| MCP server | `mcp serve`, `mcp info` |
| Skill injection | `skill install`, `skill list`, `skill remove` |
| CI monitoring | `ci watch`, `ci fix` |
| Incident management | `incident new`, `incident triage`, `rollback` |
| Project insights | `insights`, `telemetry status` |
| LLM Q&A | `ask`, `ask error <code>` |
| Learning loop | `learn teach`, `learn session`, `learn instructions` |
| Knowledge base | `forge_kb_search` MCP tool (172 curated entries) |

---

## 2. Install (Non-Interactive)

All install methods are fully non-interactive (no prompts, no browser).

### npm (recommended)

```bash
npm install -g @forgeone/cli
```

**CI / Docker:**

```bash
# No TTY, no sudo
npm install -g @forgeone/cli --no-update-notifier --prefer-offline 2>/dev/null || \
npm install -g @forgeone/cli
```

### go install

```bash
# Requires Go 1.24+
go install github.com/teragrid/forge/cmd/forge@latest
# Binary lands in $(go env GOPATH)/bin/forge
```

### Download binary (no runtime required)

```bash
# Linux amd64
curl -fsSL https://github.com/teragrid/forge/releases/latest/download/forge-linux-amd64 \
  -o /usr/local/bin/forge && chmod +x /usr/local/bin/forge

# macOS arm64 (Apple Silicon)
curl -fsSL https://github.com/teragrid/forge/releases/latest/download/forge-darwin-arm64 \
  -o /usr/local/bin/forge && chmod +x /usr/local/bin/forge

# macOS amd64 (Intel)
curl -fsSL https://github.com/teragrid/forge/releases/latest/download/forge-darwin-amd64 \
  -o /usr/local/bin/forge && chmod +x /usr/local/bin/forge

# Windows (PowerShell)
Invoke-WebRequest -Uri https://github.com/teragrid/forge/releases/latest/download/forge-windows-amd64.exe `
  -OutFile "$env:USERPROFILE\bin\forge.exe"
```

### Verify

```bash
forge version
# forge 1.2.0 (go1.26.3 linux/amd64)
```

---

## 3. Verify the Install

```bash
forge doctor
```

**Expected output (healthy):**

```
✓ git              found (git version 2.44.0)
✓ .gitignore       managed block present
✓ LLM provider     copilot (VS Code credential store)
```

**On failure:** each item shows `FORGE-XXXX` — look up the code with
`forge ask error FORGE-XXXX` or see [ERROR_CODES.md](ERROR_CODES.md).

**LLM provider detection order:**

1. `FORGE_COPILOT_MODEL` / `forge config get llm.model` (explicit model)
2. VS Code GitHub Copilot (credential store)
3. `ANTHROPIC_API_KEY` env var
4. `OPENAI_API_KEY` env var
5. No provider — verbs that call an LLM will fail with `FORGE-2000`

**Set an explicit model:**

```bash
forge config set llm.model gpt-4o       # persists to forge.config.yml
forge config set llm.model claude-opus-4-5
# override for one command:
forge scan all --model gpt-4o
```

---

## 4. Connect Forge to an AI Chat (MCP)

Forge exposes a [Model Context Protocol](https://modelcontextprotocol.io) stdio
server. Wiring it into your AI chat tool lets the assistant call Forge directly.

### VS Code / GitHub Copilot

The repo ships `.vscode/settings.json` already configured. Open this repo and
Forge tools appear in Copilot Chat automatically. For other projects, copy:

```json
{
  "mcp": {
    "servers": {
      "forge": {
        "type": "stdio",
        "command": "forge",
        "args": ["mcp", "serve"]
      }
    }
  }
}
```

### Claude Desktop

```json
// ~/Library/Application Support/Claude/claude_desktop_config.json  (macOS)
// %APPDATA%\Claude\claude_desktop_config.json  (Windows)
{
  "mcpServers": {
    "forge": {
      "command": "forge",
      "args": ["mcp", "serve"]
    }
  }
}
```

### Cursor

```json
// .cursor/mcp.json
{
  "mcpServers": {
    "forge": {
      "command": "forge",
      "args": ["mcp", "serve"]
    }
  }
}
```

### Windsurf

```json
// ~/.codeium/windsurf/mcp_config.json
{
  "mcpServers": {
    "forge": {
      "command": "forge",
      "args": ["mcp", "serve"]
    }
  }
}
```

**Print all configs at once:**

```bash
forge mcp info
```

### Test the MCP server (JSON-RPC)

```bash
# Send an initialize + tools/list handshake and inspect the response
printf '%s\n%s\n' \
  '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"0"}}}' \
  '{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}' \
  | forge mcp serve
```

**Expected:** response contains `"protocolVersion":"2024-11-05"` and an array
of **10 tools**: `forge_kb_search`, `forge_get_workflow`, `forge_get_standards`,
`forge_run`, `forge_ship_checkpoint`, `forge_get_errors`, `forge_set_budget`,
`forge_list_specs`, `forge_get_spec`, `forge_check_health`.

---

## 5. MCP Tool Reference

All tools use JSON-RPC 2.0 over stdio. The MCP server reads one request per
line on stdin and writes one JSON response per line on stdout.

---

### `forge_kb_search`

Search the 172-entry Forge knowledge base for architecture patterns,
compliance standards, and best practices.

**Input schema:**

```json
{
  "query": "<text>",     // required — natural-language or keyword search
  "limit": 5             // optional — max results, default 5, max 20
}
```

**Example call (via MCP `tools/call`):**

```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "method": "tools/call",
  "params": {
    "name": "forge_kb_search",
    "arguments": { "query": "rate limiting API", "limit": 3 }
  }
}
```

**Example result:**

```json
{
  "content": [{
    "type": "text",
    "text": "## rate-limiting-api\n\nApply token-bucket rate limiting at the API gateway layer...\n\n**Tags:** security, api, performance\n**Relevance:** 0.91"
  }]
}
```

---

### `forge_get_workflow`

Get the step-by-step Forge workflow for any verb.

**Input schema:**

```json
{
  "verb": "<forge-verb>"   // required — e.g. "ship", "scan", "bugfix"
}
```

**Example:**

```json
{ "name": "forge_get_workflow", "arguments": { "verb": "ship" } }
```

**Returns:** Markdown string with numbered steps, flags, and checkpoint
descriptions for the requested verb.

---

### `forge_get_standards`

Read the coding standards and conventions configured for the current project.

**Input schema:** `{}` (no parameters)

**Returns:** Concatenated contents of:
- `.forge/instructions/*.md`  
- `AGENTS.md` (if present)  
- `forge.config.yml` (relevant sections)

Use this before generating or reviewing code to ensure suggestions align with
project conventions.

---

### `forge_run`

Execute any Forge verb and return its output. Unsafe verbs are denied
(`ship --apply`, `audit erase`, `learn promote`, `bundle`, `plugin`).

**Input schema:**

```json
{
  "verb": "<forge-verb>",     // required — the verb name, e.g. "scan"
  "args": ["<arg1>", ...]     // optional — additional arguments/flags
}
```

**Allowed verbs (deny-list excluded):**

`scan`, `lint`, `check`, `doctor`, `version`, `config`, `audit show`,
`audit verify`, `test spec`, `test run`, `spend status`, `context generate`,
`telemetry status`, `insights`, `mcp info`, `skill list`, `bugfix` (dry-run),
`ask`, `explain`

**Example — scan for secrets:**

```json
{ "name": "forge_run", "arguments": { "verb": "scan", "args": ["secrets", "--json"] } }
```

**Example — generate test spec:**

```json
{ "name": "forge_run", "arguments": { "verb": "test", "args": ["spec", "rate limiting"] } }
```

**Returns:** `{ "stdout": "...", "stderr": "...", "exitCode": 0 }`

---

### `forge_ship_checkpoint`

Run a single `forge ship` checkpoint and return a structured JSON envelope.

**Input schema:**

```json
{
  "checkpoint": "<name>",  // required — e.g. "spec", "arch", "test", "code", "ship"
  "feature": "<slug>",    // required — spec slug under .forge/specs/
  "dry_run": false        // optional — default false
}
```

**Returns:** `{ "ok": true, "checkpoint": "code", "status": "completed", "context_summary": "...", "next_actions": [...], "cost_usd": 0.012, "duration_ms": 4200 }`

---

### `forge_get_errors`

Retrieve the last forge error list with structured code and remedy.

**Input schema:** `{}` (no parameters)

**Returns:** Array of `{ "code": "FORGE-2001", "message": "...", "remedy": "set FORGE_BUDGET_USD=<amount>" }`

---

### `forge_set_budget`

Set the `FORGE_BUDGET_USD` per-invocation LLM spend cap for the current session.

**Input schema:**

```json
{
  "usd": 0.50   // required — cap in USD; 0 = unlimited
}
```

---

### `forge_list_specs`

List all specs under `.forge/specs/` with status and checkpoint progress.

**Input schema:** `{}` (no parameters)

**Returns:** Array of `{ "slug": "rate-limiter", "status": "draft", "progress": "2/7" }`

---

### `forge_get_spec`

Read the full content of any spec file by slug and file name.

**Input schema:**

```json
{
  "slug": "<spec-slug>",      // required — e.g. "rate-limiter"
  "file": "spec.md"           // optional — default "spec.md"
}
```

---

### `forge_check_health`

Run `forge doctor` and return structured health results.

**Input schema:** `{}` (no parameters)

**Returns:** `{ "ok": true, "checks": [ { "name": "llm-mode", "status": "advisory", "message": "..." } ] }`

---

All commands run against the project in the current working directory unless
otherwise noted. Flags marked `†` are available on every verb.

### Global flags `†`

| Flag | Description |
|---|---|
| `--model <model>` | LLM model override for this invocation (e.g. `gpt-4o`, `claude-sonnet-4-5`) |
| `--budget-usd <n>` | Hard LLM spend cap in USD for this invocation (`0` = unlimited) |
| `--profile <name>` | Load a named config profile from `forge.config.yml` |
| `--json` | Machine-readable JSON output (supported by most verbs; auto-enabled when stdout is non-TTY) |
| `--human` | Force human-readable output even when piped or `FORGE_LLM_MODE=1` is set |
| `--quiet` | Suppress informational output; only print findings/errors |

**LLM-first mode (v1.7.0+):** Set `FORGE_LLM_MODE=1` in the shell environment once to enable JSON envelopes on all verbs and auto-approve all `forge ship` interactive gates — no per-command flags needed.

```bash
export FORGE_LLM_MODE=1
forge ship code   # emits JSON envelope; no y/N prompts
```

---

### Setup & health

```bash
forge version                        # print version + build info (exit 0)
forge doctor                         # check prerequisites; FORGE-1000..1099
forge init                           # add Forge to an existing project
forge init --minimal                 # inject KB + ship workflow only (no CI rewrite)
forge config set <key> <value>       # persist config to forge.config.yml
forge config get <key>               # read one config value (exit 1 if missing)
forge config show                    # print full resolved config as YAML
forge explain <verb>                 # plain-English description of any verb
forge explain --json                 # machine-readable manifest of all verbs
```

### Scaffolding

```bash
forge new <template> <name>          # classic mode — built-in template
forge new "<description>"            # TSD mode — reads .forge/tsd.yml
forge new --tsd <file> "<desc>"      # TSD mode — explicit TSD file
forge tsd init                       # interactive wizard → .forge/tsd.yml
forge tsd validate                   # lint the TSD file
forge templates list                 # list available templates
```

### Scanning (exits 1 when findings present)

```bash
forge scan all                       # run every scanner
forge scan secrets                   # secret / credential leak (FORGE-1400..1499)
forge scan secrets --json            # JSON array of findings
forge scan security --secrets        # alias; adds --min-confidence medium by default
forge scan security --json --min-confidence medium
forge scan prompt-injection          # AI app prompt-injection patterns
forge scan supply-chain              # known-vulnerable package versions
forge scan all --exit-zero           # always exit 0 (advisory mode)
```

**JSON finding schema:**

```json
{
  "rule_id": "FORGE-1401",
  "severity": "error",           // "error" | "warning" | "info"
  "file": "config/secrets.go",
  "line": 42,
  "message": "Hardcoded API key detected",
  "fix": "Move to environment variable or secret manager"
}
```

### Linting & quality

```bash
forge lint                           # convention violations (FORGE-1500..1599)
forge lint --json                    # JSON array
forge check                          # pre-flight: typecheck + lint + test fast-path
forge clean                          # remove AI-generated junk (placeholder comments, dead TODOs)
```

### Ship pipeline

```bash
forge ship                           # 6-stage gate: spec→arch→test→breakdown→code→ship
forge ship --dry-run                 # preview all stages without side effects (exit 0/1)
forge ship <feature>                 # auto-creates feature/<slug> branch then runs gate
forge ship --resume                  # continue from last failed checkpoint
forge ship --no-branch               # skip branch creation; use current branch
```

Checkpoint names and exit codes:

| Checkpoint | Exits 1 when… |
|---|---|
| `spec` | generated code diverges from the requested feature |
| `arch` | architecture doc / OpenAPI contract missing or invalid |
| `test` | any test suite fails |
| `breakdown` | logic gaps or missing error handling detected |
| `code` | code quality below threshold |
| `ship` | secrets present, lint failures, or security violations |

### Testing

```bash
forge test unit                      # run unit tests
forge test integration               # run integration tests
forge test e2e                       # run end-to-end tests
forge test spec "<feature>"          # generate 9-case YAML spec → .forge/specs/<feature>/spec.yml
forge test run --spec <path>         # execute families declared in spec.yml
forge test run --feature <name>      # same; resolves .forge/specs/<name>/spec.yml
forge test run --spec <path> --dry-run  # print plan without running
```

**spec.yml schema (generated by `forge test spec`):**

```yaml
feature: rate limiting
families:
  happy_path:    { description: "...", run: "..." }
  boundary:      { description: "...", run: "..." }
  negative:      { description: "...", run: "..." }
  idempotency:   { description: "...", run: "..." }
  concurrency:   { description: "...", run: "..." }
  authz:         { description: "...", run: "..." }
  regression:    { description: "...", run: "..." }
  data_accuracy: { description: "...", run: "..." }
  false_positive:{ description: "...", run: "..." }
```

### Bug fixing

```bash
forge bugfix --bug "<description>"            # dry-run diagnosis + patch plan
forge bugfix --bug "<description>" --apply    # write patch + regression test to disk
forge bugfix --bug - < crash.txt             # read description from stdin
forge bugfix --test "TestName"               # fix the root cause of a failing test
forge bugfix --finding <id>                  # fix a specific scan finding
forge bugfix --bug "..." --stack "$(cat crash.log)" --file handler.go --apply
```

### Spend & LLM

```bash
forge spend set --daily 2.00 --monthly 30.00  # set hard limits
forge spend status                             # show current usage
forge spend reset                              # reset counters
forge <any> --budget-usd 0.50                 # cap this invocation
```

### Audit ledger

```bash
forge audit show                     # print audit log (JSONL, pretty-printed)
forge audit show --json              # raw JSONL
forge audit verify                   # hash-chain integrity check (exit 0 = ok, 1 = tampered)
forge audit export --format pdf      # export for compliance review
forge audit erase --id <entry-id>    # GDPR erasure (only in regulated modes)
```

### Context & knowledge

```bash
forge context generate               # write .forge/context/snapshot.md
forge ask "<question>"               # LLM Q&A about the project
forge ask error FORGE-1001           # look up an error code
forge explain <verb>                 # plain-English description + flags
forge explain --json                 # machine-readable manifest of every verb
```

### MCP & skills

```bash
forge mcp serve                      # start MCP stdio server (reads JSON-RPC from stdin)
forge mcp info                       # print ready-to-paste config for VS Code, Claude, Cursor, Windsurf

forge skill install --for copilot    # inject expert-role files into project (VS Code Copilot)
forge skill install --for claude     # Claude (writes CLAUDE.md + .claude/commands/)
forge skill install --for cursor     # Cursor (writes .cursor/rules/*.mdc)
forge skill install --for windsurf   # Windsurf (writes .windsurfrules)
forge skill install --for all        # all four platforms
forge skill install --dry-run        # preview without writing
forge skill list                     # show installed skill files
forge skill remove --force           # remove all skill files without prompting
```

### Incidents & rollback

```bash
forge incident new --id INC-001 --title "<title>" --severity S1
forge incident triage INC-001
forge rollback --advise
```

### Insights & telemetry

```bash
forge insights                       # local usage statistics rollup
forge insights cli                   # find unused verbs
forge telemetry status               # show opt-in/out status
forge telemetry enable
forge telemetry disable
```

---

## 7. Common Agent Task Patterns

### Scan a repo and return structured findings

```bash
cd /path/to/project
forge scan all --json
```

Parse the JSON array. Each finding has `rule_id`, `severity`, `file`, `line`,
`message`, `fix`.

### Gate a PR (non-interactive CI check)

```bash
forge ship --dry-run
# exit 0 → all checks pass; exit 1 → print failures and block merge
```

### Add Forge to a project and scan it in one shot

```bash
forge init --minimal   # idempotent; safe to run on already-initialized projects
forge scan all --json
```

### Generate a test spec then review it

```bash
forge test spec "user authentication"
# Writes: .forge/specs/user-authentication/spec.yml
cat .forge/specs/user-authentication/spec.yml
# Edit as needed, then:
forge test run --feature "user authentication" --dry-run
```

### Fix a bug end-to-end

```bash
# 1. Diagnose (dry-run by default)
forge bugfix --bug "Payment total wrong when discount applied"

# 2. Review the proposed patch (printed to stdout)

# 3. Apply (writes to disk + audit log)
forge bugfix --bug "Payment total wrong when discount applied" --apply
```

### Verify project context for another LLM call

```bash
forge context generate
cat .forge/context/snapshot.md
# Use snapshot.md content as context in your own LLM call
```

### Use the knowledge base from an agent

Via MCP tool call:
```json
{
  "name": "forge_kb_search",
  "arguments": { "query": "multi-tenant row-level security", "limit": 5 }
}
```

Direct CLI:
```bash
forge ask "what are the best practices for multi-tenant row-level security?"
```

### Install Forge in a GitHub Actions workflow

```yaml
- name: Install Forge
  run: npm install -g @forgeone/cli --no-update-notifier

- name: Forge quality gate
  run: |
    forge doctor
    forge scan all --json > forge-findings.json
    forge ship --dry-run
  env:
    OPENAI_API_KEY: ${{ secrets.OPENAI_API_KEY }}
```

### Check audit integrity in CI

```bash
forge audit verify   # exit 0 = clean; exit 1 = hash mismatch (log was modified)
```

---

## 8. Error Handling and Error Codes

All errors follow the `FORGE-XXXX` format. Error code ranges by verb:

| Range | Verb / area |
|---|---|
| `FORGE-1000..1099` | `doctor` |
| `FORGE-1100..1199` | `new` |
| `FORGE-1400..1499` | `scan` |
| `FORGE-1500..1599` | `lint` |
| `FORGE-1600..1699` | `ship` |
| `FORGE-2000..2099` | `config`, `upgrade` |
| `FORGE-2400..2499` | `spend` |
| `FORGE-3400..3499` | `audit` |
| `FORGE-3600..3699` | `eval` |
| `FORGE-3900..3999` | `insights` |
| `FORGE-4000..4099` | `incident` |
| `FORGE-4100..4199` | `telemetry` |
| `FORGE-4300..4399` | `test` |
| `FORGE-4900..4999` | `ask` |
| `FORGE-5200..5299` | `learn` |
| `FORGE-6100..6199` | `backup` |
| `FORGE-6200..6299` | `ci` |
| `FORGE-6500..6599` | `tsd` |
| `FORGE-6550..6599` | `bugfix` |

**Look up any code:**

```bash
forge ask error FORGE-2000
# Prints: cause, likely fix, related commands
```

**Full catalogue:** [docs/ERROR_CODES.md](ERROR_CODES.md)

**Common errors an agent will encounter:**

| Code | Cause | Fix |
|---|---|---|
| `FORGE-2000` | No LLM provider configured | Set `ANTHROPIC_API_KEY` or `OPENAI_API_KEY` env var |
| `FORGE-1001` | `forge doctor` — git not found | Install git |
| `FORGE-1401` | `scan secrets` — hardcoded credential | Move secret to env var; add file to `.gitignore` |
| `FORGE-1501` | `lint` — missing test file | Create `<file>_test.<ext>` alongside source |
| `FORGE-4001` | `doctor` — no `.gitignore` | Run `forge init --minimal` |

---

## 9. Key File Paths and Formats

```
<project-root>/
├── forge.config.yml              # Forge project config (created by forge init)
├── AGENTS.md                     # Coding-assistant instructions
└── .forge/
    ├── audit.log                 # Hash-chained JSONL audit ledger (append-only)
    ├── tsd.yml                   # Tech Stack Decision (written by forge tsd init)
    ├── context/
    │   └── snapshot.md           # LLM-readable project context (forge context generate)
    ├── instructions/
    │   └── *.md                  # Project coding standards (read by forge_get_standards)
    ├── specs/
    │   └── <feature>/
    │       └── spec.yml          # 9-case test spec (forge test spec)
    └── learned/
        └── gotchas.jsonl         # CI failure lessons (forge ci fix)
```

**`forge.config.yml` schema (minimal):**

```yaml
project:
  name: my-service
  type: ts-service      # ts-service | next-app | go-service | ...
llm:
  model: gpt-4o         # optional; detected from env if absent
spend:
  daily_usd: 2.00
  monthly_usd: 30.00
```

**`audit.log` entry schema:**

```jsonl
{"ts":"2025-05-01T12:00:00Z","verb":"ship","stage":"code","action":"codemod:fix-null-check","file":"handler.go","hash":"<sha256>","prev":"<prev-hash>"}
```

---

## 10. Environment Variables

| Variable | Effect |
|---|---|
| `OPENAI_API_KEY` | OpenAI API key; auto-detected by Forge |
| `ANTHROPIC_API_KEY` | Anthropic API key; auto-detected by Forge |
| `FORGE_BUDGET_USD` | Per-invocation LLM spend cap (overridden by `--budget-usd`) |
| `FORGE_COPILOT_MODEL` | Default LLM model (overridden by `forge config set llm.model`) |
| `FORGE_SKIP_SCAN` | Set to `1` to skip `forge scan security` in pre-push hook |
| `FORGE_SKIP_LINT` | Set to `1` to skip `forge lint` in pre-push hook |
| `FORGE_SKIP_CHECK` | Set to `1` to skip `forge check` in pre-push hook |
| `FORGE_SKIP_QA` | Set to `1` to skip the real-command QA suite in pre-push hook |
| `SKIP_PRE_PUSH` | Set to `1` to bypass all pre-push checks (emergency only) |
| `FORGE_BIN` | Path to forge binary (used by hook scripts, default: `forge`) |
| `CGO_ENABLED` | Must be `0` for forge itself; enforced by `ADR-001` |

---

## 11. Exit Codes

| Code | Meaning |
|---|---|
| `0` | Success. For scanners: no findings. For `ship --dry-run`: all checks pass. |
| `1` | Failure. Findings present, stage failed, or validation error. |
| `2` | Fatal. Binary misconfigured, missing required dependency, or I/O error. |

To force scanners to always exit 0 (advisory mode):

```bash
forge scan all --exit-zero
forge scan secrets --exit-zero
```

To get machine-readable output regardless of exit code:

```bash
forge scan all --json; true   # capture JSON, ignore exit code in script
```
