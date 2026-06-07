# Spec: Rearchitect Forge Framework for LLM-First Workflows

## Status Summary
- Lifecycle: Draft
- Version Scope: MINOR (v1.7.0)
- Owner: forge-core
- Last Updated: 2026-06-08
- Checkpoint Progress: 1/7

## What

Rearchitect the Forge CLI so that **LLMs acting as vibe-coders** (e.g. Claude, GPT-4o, Gemini running inside an AI coding assistant) are the primary, first-class consumers of every command. A developer interacts with Forge indirectly — they describe intent to their LLM, and the LLM drives `forge` commands on their behalf.

This is distinct from "agent framework" thinking. There is no orchestration runtime to install. The LLM simply calls `forge` as a subprocess (or via MCP tool), reads the JSON response, reasons over it, and issues the next command — exactly like a human would use a CLI, except the reader is a language model, not eyes.

Key pillars:
1. **Structured, parseable output by default** — every command emits clean JSON when stdout is not a TTY or `FORGE_LLM_MODE=1` is set. No ANSI, no spinners, no progress bars in the machine path. LLMs must not have to parse decorated text.
2. **Self-describing responses** — every JSON response includes a `next_actions` array of plain-English suggestions the LLM can directly issue as the next command. The LLM never needs to remember command syntax.
3. **Inline context budget** — responses include a `context_summary` field with the minimum state an LLM needs to continue the conversation without re-reading the filesystem. Keeps prompt windows lean.
4. **Idempotent, resumable checkpoints** — every `forge ship` checkpoint is safe to re-run; the response indicates whether work was skipped (already done), applied, or failed. LLMs can retry without side effects.
5. **Error messages written for LLMs** — errors include a `remedy` field: a concrete, copy-pasteable command or edit that resolves the error. The LLM can act on it immediately without looking up docs.
6. **MCP-native tool surface** — all forge verbs are exposed as MCP tools (already started in v1.5.x). This spec completes the surface so a Claude or GPT tool-use session can drive the full `forge ship` pipeline without subprocess calls.
7. **Spend transparency** — every response reports `llm_tokens_used` and `cost_usd` for the operation so the LLM (or its enclosing agent loop) can respect user-set budgets.

## Why

Today, when a developer says *"Claude, use forge to ship this feature"*, Claude must:
- Guess command flags from memory (forge's help output is human-prose).
- Parse ANSI-decorated stdout to know whether a checkpoint succeeded.
- Re-read `.forge/specs/*/` files to reconstruct context across turns.
- Handle interactive `y/n` prompts that block subprocess execution.
- Have no way to know what to do next after a command — it must guess.

This creates a fragile, prompt-heavy integration. The LLM-first rearchitecture removes all of that friction: every forge command becomes a clean function call that returns structured data the LLM can reason over directly, with the next step embedded in the response.

The net result: a developer can hand their entire `forge ship` workflow to Claude (or any other LLM coding assistant) and the LLM drives it end-to-end with minimal prompt overhead and zero ambiguity.

## Acceptance Criteria

- [ ] Given Claude calls `forge ship spec "add rate limiting"` in a non-TTY subprocess, When the command completes, Then stdout is valid JSON containing `ok`, `status`, `path`, `context_summary`, `next_actions`, `llm_tokens_used`, and `cost_usd` — with zero ANSI escape codes.
- [ ] Given a `forge ship` checkpoint fails, When the JSON error response is received, Then it contains a `remedy` field with a concrete shell command or file edit that resolves the error, usable by the LLM without additional lookup.
- [ ] Given `FORGE_LLM_MODE=1` is set, When any forge command runs, Then all interactive prompts are suppressed and all output is valid JSON (no hanging on `y/n` gates).
- [ ] Given an LLM issues `forge mcp list-tools`, When the response is received, Then all `forge ship` subcommands appear as MCP tool definitions with typed input schemas the LLM can use for tool-use calls.
- [ ] Given an LLM calls any `forge ship` MCP tool, When the tool returns, Then the response schema is identical to the subprocess JSON envelope (same fields, same error codes).
- [ ] Given a checkpoint was already completed, When the LLM re-runs the same checkpoint (retry), Then the response has `status: "skipped"` and `ok: true` — no duplicate side effects.
- [ ] Given an LLM calls `forge ship verify --json`, When the response is received, Then `context_summary` contains enough information for a fresh LLM session to understand what was built without reading any files.
- [ ] Given `FORGE_BUDGET_USD=2.00` is set, When total spend reaches the cap mid-pipeline, Then the current command returns `ok: false` with `code: "FORGE-2001"` and `remedy: "raise FORGE_BUDGET_USD or run with --budget-usd=<n>"`.
- [ ] Given a developer uses a TTY and does not set `FORGE_LLM_MODE`, When forge runs, Then output is the existing human-friendly format (full backwards compatibility).

## Non-Functional Requirements

- [ ] JSON response parsing must succeed with `json.Unmarshal` in any language without pre-processing (no trailing commas, no comments in production output).
- [ ] `context_summary` must be ≤ 500 tokens (measured by `cl100k_base` tokeniser) for any single checkpoint response.
- [ ] All `forge ship` subcommands must respond within 200 ms (excluding LLM call latency) when called with `--dry-run`.
- [ ] MCP tool schemas must be published as static JSON under `docs/mcp/` and validated by a contract test on every PR.
- [ ] `remedy` field must be present on every non-zero exit code response; CI gate enforces this via a linter on the `agentapi` package.
- [ ] No stdout ANSI codes when `NO_COLOR=1`, `FORGE_LLM_MODE=1`, or stdout is not a TTY.

## Out of Scope

- Building an orchestration runtime or agent framework — the LLM is the orchestrator.
- Agent-to-agent communication protocols — out of scope for this release.
- Replacing the underlying LLM provider routing (`tierrouter`) — unchanged.
- Removing the human CLI surface — `--human` / TTY autodetect keeps it fully functional.
- Streaming/SSE output for LLMs — future work; this spec targets request/response.
- Authentication server / identity management — LLMs inherit the user's existing credentials.

## Open Questions

1. Should `context_summary` be generated by an LLM call or computed deterministically from checkpoint state? (Deterministic preferred to avoid cost recursion.)
2. For MCP tool-use, should `forge` expose one MCP tool per subcommand or one tool with a `checkpoint` parameter?
3. Should `next_actions` be ranked by likelihood or unordered?
