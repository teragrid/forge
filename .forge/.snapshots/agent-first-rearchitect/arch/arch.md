# Architecture: Forge Framework — LLM-First Rearchitecture

> **Status**: Draft — awaiting review before `forge ship code`
> **Spec**: `.forge/specs/agent-first-rearchitect/spec.md`
> **Target release**: v1.7.0

---

## 1. Mental Model

```
Developer
   │  "Claude, add rate limiting to the API"
   ▼
LLM (Claude / GPT-4o / Gemini / …)
   │  tool_use: forge_ship_spec { description: "add rate limiting" }
   │  — or —
   │  subprocess: forge ship spec "add rate limiting"
   ▼
forge CLI  ──────────────────────────────────────────────────────────┐
   │  JSON response:                                                  │
   │  { ok, status, path, context_summary, next_actions, cost_usd }  │
   ▼                                                                  │
LLM reads response, reasons, issues next command ◄───────────────────┘
```

The LLM is the orchestrator. Forge is a deterministic tool the LLM calls. No agent runtime, no DAG scheduler, no orchestration framework needed — the LLM conversation loop **is** the pipeline.

---

## 2. Component Topology

```
┌──────────────────────────────────────────────────────────────────┐
│  LLM Caller  (Claude / GPT-4o / local model via Ollama / …)     │
│  — subprocess call  OR  MCP tool-use call                        │
└───────────────────────────┬──────────────────────────────────────┘
                            │  stdin: args + env  |  stdout: JSON
┌───────────────────────────▼──────────────────────────────────────┐
│  forge CLI entrypoint  (cmd/forge/main.go)                       │
│  • TTY absent OR FORGE_LLM_MODE=1 OR NO_COLOR=1 → LLM mode      │
│  • TTY present, no flag → human mode (backwards compat)          │
└──┬─────────────┬──────────────┬──────────────┬───────────────────┘
   │             │              │              │
   ▼             ▼              ▼              ▼
[cmdship]   [cmdmcp]       [cmdaudit]    existing verbs
Ship pipeline  MCP server     Audit trail   scan, lint, test, …
checkpoints    (extended)     & cost log
   │             │
   ▼             ▼
[internal/llmresponse]    NEW — builds the JSON envelope:
                          ok, status, path, context_summary,
                          next_actions, remedy, cost_usd, tokens_used
   │
   ├──► [internal/llmbudget]      (existing) — enforces FORGE_BUDGET_USD
   ├──► [internal/tokenledger]    (existing) — tracks tokens per call
   ├──► [internal/audit]          (existing) — records cost + model used
   ├──► [internal/prompttemplates](existing) — system prompts per verb
   └──► [internal/telemetry]      (existing) — OTel spans
```

**New packages** (all under `internal/`):

| Package | Responsibility |
|---|---|
| `llmresponse` | Build the standard JSON envelope; generate `context_summary` (deterministic, no LLM call); compute `next_actions` from checkpoint state; enforce `remedy` presence on errors |

**Modified packages**:

| Package | Change |
|---|---|
| `internal/cli/cmdship` | Wrap all output through `llmresponse.Wrap()` when in LLM mode |
| `internal/cli/cmdmcp` | Add remaining `forge ship` subcommands as MCP tools; align MCP response schema with `llmresponse` envelope |
| `cmd/forge/main.go` | Detect LLM mode (TTY absent / `FORGE_LLM_MODE` / `NO_COLOR`); inject into context |
| `internal/audit` | Add `cost_usd`, `model`, `tokens_used` fields to every log entry |

---

## 3. LLM-Mode JSON Envelope

Activated by: no TTY on stdout, `FORGE_LLM_MODE=1`, `NO_COLOR=1`, or `--json` flag.

### Success response

```jsonc
{
  "ok": true,
  "checkpoint": "spec",           // which forge ship checkpoint ran
  "status": "completed",          // completed | skipped | pending | running
  "path": ".forge/specs/add-rate-limiting/spec.md",
  "context_summary": "Spec 'add-rate-limiting' created. 3 ACs defined. Next: arch.", // ≤500 tokens
  "next_actions": [
    "forge ship arch \"add rate limiting\"",
    "forge ship spec \"add rate limiting\" --dry-run  # re-inspect without changes"
  ],
  "llm_tokens_used": 1240,
  "cost_usd": 0.0037,
  "duration_ms": 1840
}
```

### Error response

```jsonc
{
  "ok": false,
  "checkpoint": "code",
  "status": "failed",
  "error": {
    "code": "FORGE-2001",
    "message": "LLM spend cap exceeded ($2.00 limit, $2.03 used)",
    "remedy": "export FORGE_BUDGET_USD=5.00 && forge ship code \"add rate limiting\""
  },
  "context_summary": "Pipeline stalled at 'code' checkpoint. Spec and arch are complete.",
  "next_actions": [
    "export FORGE_BUDGET_USD=5.00 && forge ship code \"add rate limiting\"",
    "forge ship status"
  ],
  "llm_tokens_used": 0,
  "cost_usd": 0.0
}
```

### Idempotency / skip response

```jsonc
{
  "ok": true,
  "checkpoint": "spec",
  "status": "skipped",
  "path": ".forge/specs/add-rate-limiting/spec.md",
  "context_summary": "Spec already completed. No changes made.",
  "next_actions": ["forge ship arch \"add rate limiting\""]
}
```

Schema source of truth: `internal/llmresponse/envelope.go`. All fields except `path` and `checkpoint` are always present.

---

## 4. MCP Tool Surface

`forge mcp serve` (existing) is extended to expose the full `forge ship` pipeline as MCP tools. This lets Claude / GPT-4o use native tool-use instead of subprocess calls.

### Tool inventory (new/extended)

| MCP Tool | Maps to | Input schema |
|---|---|---|
| `forge_ship_spec` | `forge ship spec` | `{ description: string, name?: string, dry_run?: bool }` |
| `forge_ship_arch` | `forge ship arch` | `{ description: string, name?: string, dry_run?: bool }` |
| `forge_ship_code` | `forge ship code` | `{ description: string, name?: string, dry_run?: bool }` |
| `forge_ship_test` | `forge ship test` | `{ description: string, name?: string }` |
| `forge_ship_breakdown` | `forge ship breakdown` | `{ description: string, name?: string }` |
| `forge_ship_verify` | `forge ship verify` | `{ description: string, name?: string }` |
| `forge_ship_status` | `forge ship status` | `{ name?: string }` |
| `forge_scan_secrets` | `forge scan secrets` | `{ path?: string }` |
| `forge_audit_show` | `forge audit show` | `{ limit?: int }` |
| `forge_spend_status` | `forge spend status` | `{}` |

All tools return the same `llmresponse` JSON envelope. Static schemas published under `docs/mcp/tools.json`.

### LLM interaction flow (Claude example)

```
User:    "Add rate limiting to the API"
Claude:  [tool_use] forge_ship_spec { description: "add rate limiting to the API" }
Forge:   { ok: true, status: "completed", context_summary: "...", next_actions: [...] }
Claude:  [tool_use] forge_ship_arch { description: "add rate limiting to the API" }
Forge:   { ok: true, status: "completed", ... }
Claude:  "I've created the spec and architecture. Here's what was planned: ..."
User:    "Looks good, implement it"
Claude:  [tool_use] forge_ship_code { description: "add rate limiting to the API" }
...
```

---

## 5. `context_summary` Generation

`context_summary` is computed **deterministically** (no LLM call, no extra cost):

```
context_summary = checkpoint_name + " '" + spec_slug + "' " + status_sentence
                + " ACs: " + ac_count + "/" + ac_total
                + " Next: " + next_checkpoint
```

Example: `"spec 'add-rate-limiting' completed. 4 ACs defined (0 checked). Next: arch."`

Rules:
- Max 500 `cl100k_base` tokens, measured at write time; truncated with `…` if over.
- Never includes file paths longer than the basename.
- Never includes raw diff or code — those belong in artifact files the LLM can read separately via `read_file`.

---

## 6. LLM Mode Detection

```
Priority (highest to lowest):
1. --json flag                   → LLM mode
2. FORGE_LLM_MODE=1 env          → LLM mode
3. NO_COLOR=1 env                → LLM mode (JSON, no ANSI)
4. stdout is not a TTY (pipe)    → LLM mode
5. --human flag                  → human mode (overrides all above)
6. stdout is a TTY               → human mode
```

This ensures:
- Subprocess calls from LLMs (no TTY) are always in LLM mode automatically.
- MCP tool calls always in LLM mode (MCP server pipes stdout).
- Existing human developer sessions are unchanged.
- `--human` is an explicit escape hatch for scripts that want human output.

---

## 7. `remedy` Field Contract

Every non-zero exit code **must** include a `remedy`. CI gate (`go test ./internal/llmresponse/...`) asserts this via `TestAllErrorsHaveRemedy`. Remedy content rules:

- Must be a complete, copy-pasteable shell command or a specific file edit instruction.
- Must not say "contact support" or "see documentation" alone — actionable always.
- Should be ≤ 120 characters when possible.
- For budget errors: include the exact `export` + retry command.
- For lint/format errors: include the exact fix command (`gofmt -w`, `goimports -w`, etc.).
- For missing dependency errors: include the exact `go get` command.

---

## 8. Non-Functional Requirements

| NFR | Target | Measurement |
|---|---|---|
| CLI cold start (no LLM) | < 50 ms | `time forge --version` |
| Checkpoint dispatch overhead | < 200 ms (excl. LLM) | `--dry-run --json` benchmark |
| `context_summary` token count | ≤ 500 tokens | unit test, `cl100k_base` |
| MCP tool response schema match | 100% field parity | contract test on every PR |
| `remedy` coverage | 100% of error codes | `TestAllErrorsHaveRemedy` |
| No ANSI in LLM mode | zero bytes matching `\x1b[` | stdout capture test |

---

## 9. Security Threat Model

| Threat | STRIDE | Mitigation |
|---|---|---|
| Prompt injection via spec/arch content | Tampering | `secretrewriter` + `guardrails` applied before LLM calls; `context_summary` is deterministic (no LLM-generated text in control path) |
| LLM-driven path traversal via description arg | Elevation | `fssandbox` validates all paths derived from user input |
| Budget exhaustion by runaway LLM loop | DoS (cost) | `FORGE_BUDGET_USD` hard cap via `llmbudget`; `FORGE-2001` halts cleanly |
| Forged `remedy` commands (if LLM-generated) | Tampering | `remedy` is template-generated from error code registry, not LLM-generated |
| MCP tool schema drift breaks LLM tool-use | Tampering | Static schema published + contract test; schema is the source of truth |
| LLM reads sensitive data via `context_summary` | Info Disclosure | `context_summary` is path-basename only; no file content, no secrets |

---

## 10. Migration & Backwards Compatibility

| Scenario | Behaviour |
|---|---|
| TTY present, no flags | Existing human UX — zero change |
| Pipe / subprocess (no TTY) | **New**: auto-switches to LLM mode JSON |
| `FORGE_LLM_MODE=1` | LLM mode JSON, no prompts |
| `NO_COLOR=1` | LLM mode JSON, no ANSI |
| `--json` flag | LLM mode JSON |
| `--human` flag | Forces human mode regardless |
| MCP tool-use | LLM mode JSON (MCP server owns stdout) |
| Existing scripts using `grep` on output | May break if they depended on human text from piped output — `--human` flag is the fix |

One breaking-change note: piped output (e.g. `forge ship spec "x" | grep "✓"`) switches to JSON in LLM mode. Scripts using text-grep on piped output should add `--human`. This is called out in `BREAKING.md` and `CHANGELOG.md`.

---

## 11. Phased Delivery

| Phase | Scope | Target |
|---|---|---|
| P1 | `llmresponse` envelope, LLM mode detection, `--json` flag wired to all `forge ship` commands | v1.7.0-alpha |
| P2 | `context_summary` deterministic generator, `remedy` field on all error codes, `TestAllErrorsHaveRemedy` gate | v1.7.0-beta |
| P3 | MCP tools for full `forge ship` pipeline, static schema under `docs/mcp/tools.json` | v1.7.0-rc |
| P4 | `cost_usd` + `llm_tokens_used` in every response, budget cap wired to per-call spend | v1.7.0 |
| P5 | `BREAKING.md` + migration note for piped-output scripts; update `forge doctor` to detect affected scripts | v1.7.1 |

---

## 12. ADR Summary

**Status**: Proposed

**Context**: Vibe-coders increasingly interact with Forge through an LLM (Claude, GPT-4o, etc.) rather than directly. The current CLI was designed for humans: ANSI output, interactive prompts, prose error messages. This makes LLM-driven usage fragile and prompt-heavy.

**Decision**: Make LLM the primary consumer. Structured JSON output is the default for any non-TTY context. Every response is self-contained (context, next steps, remedy). MCP tool surface covers the full ship pipeline.

**Consequences**:
- ✓ LLM-driven `forge ship` workflows become robust and token-efficient.
- ✓ No new orchestration runtime needed — the LLM conversation loop is the pipeline.
- ✓ Human UX is fully preserved via TTY autodetect and `--human` flag.
- ✗ Piped shell scripts that parse human-text output must add `--human` (documented in BREAKING.md).
