# Task Breakdown: Forge LLM-First Rearchitecture

> Spec: `.forge/specs/agent-first-rearchitect/spec.md`
> Arch: `.forge/specs/agent-first-rearchitect/arch.md`
> Target: v1.7.0 (5 phases)
> Updated: 2026-06-08

---

## Phase P1 — LLM Response Envelope + Mode Detection

### T-001 — Define llmresponse envelope types
File: `internal/llmresponse/envelope.go`
Change: Define `Response`, `ErrorDetail`, `Status` types with all required fields (`ok`, `status`, `checkpoint`, `path`, `context_summary`, `next_actions`, `remedy`, `cost_usd`, `llm_tokens_used`, `duration_ms`, `error`). Export `Wrap()` constructor.
Effort: S
Done when: `go build ./internal/llmresponse/...` passes; all fields match the JSON schema in arch.md §3.
Depends on: none

### T-002 — Implement context_summary generator (deterministic)
File: `internal/llmresponse/summary.go`
Change: `GenerateSummary(checkpoint, slug, status, acCount, acTotal, nextCheckpoint string) string`. No LLM call. Output truncated to 500 cl100k_base tokens (≈2000 chars).
Effort: S
Done when: Unit test `TestGenerateSummary_TokenBound` passes; output ≤2000 chars for all inputs.
Depends on: T-001

### T-003 — Implement next_actions generator
File: `internal/llmresponse/nextactions.go`
Change: `NextActions(checkpoint, slug string, failed bool) []string` — returns ordered list of suggested forge commands based on current checkpoint and status.
Effort: S
Done when: Each checkpoint maps to ≥1 correct next action; verified by table-driven unit test.
Depends on: T-001

### T-004 — LLM mode detection in cmd/forge/main.go
File: `cmd/forge/main.go`
Change: Before dispatch, set `ctx = llmresponse.WithLLMMode(ctx, isLLMMode())` where `isLLMMode()` checks priority: `--json` flag > `FORGE_LLM_MODE=1` > `NO_COLOR=1` > `!term.IsTerminal(os.Stdout.Fd())`. `--human` flag forces human mode regardless.
Effort: S
Done when: `TestLLMModeDetection_Priority` passes all 6 priority cases (see test design).
Depends on: T-001

### T-005 — Wire llmresponse.Wrap() into cmdship checkpoints
File: `internal/cli/cmdship/runner.go`
Change: After each checkpoint completes (or fails), if `llmresponse.IsLLMMode(ctx)` then marshal `llmresponse.Wrap(...)` to stdout instead of the current human-formatted output.
Effort: M
Done when: `forge ship spec "x" --dry-run` with `FORGE_LLM_MODE=1` emits valid JSON matching the envelope schema; zero ANSI codes.
Depends on: T-001, T-002, T-003, T-004

### T-006 — Suppress interactive prompts in LLM mode
File: `internal/cli/cmdship/gates.go`
Change: All `y/N` gates check `llmresponse.IsLLMMode(ctx)`; in LLM mode auto-approve (equivalent to `--yolo`) and include `"gate_auto_approved": true` in the response.
Effort: S
Done when: `TestGate_LLMMode_NoPrompt` verifies no stdin read occurs and exit is clean.
Depends on: T-004

### T-007 — Tests for P1
File: `internal/llmresponse/llmresponse_test.go`
Change: Unit tests for T-001 through T-006. Cover: envelope shape, context_summary token bound, next_actions per checkpoint, LLM mode detection priority (6 cases), no-ANSI output, prompt suppression.
Effort: M
Done when: `go test ./internal/llmresponse/...` passes; `-race` clean.
Depends on: T-001–T-006

---

## Phase P2 — remedy Field + Error Code Registry

### T-008 — Add remedy template to every error code registration
File: `internal/errcode/registry.go`
Change: Add `Remedy string` field to `errcode.Definition`. Populate for all existing error codes in range FORGE-1600–FORGE-9999. Remedy must be a complete shell command or edit instruction ≤120 chars.
Effort: M
Done when: `TestAllErrorsHaveRemedy` passes (iterates registry, asserts `Remedy != ""`).
Depends on: T-001

### T-009 — Include remedy in llmresponse error envelope
File: `internal/llmresponse/envelope.go`
Change: In `Wrap()`, if `err != nil`, look up `errcode.Remedy(code)` and populate `ErrorDetail.Remedy`.
Effort: S
Done when: `TestWrap_ErrorIncludesRemedy` verifies remedy is populated from registry; remedy contains "forge" for all FORGE-16xx codes.
Depends on: T-008, T-001

### T-010 — CI gate: TestAllErrorsHaveRemedy
File: `internal/errcode/remedy_test.go`
Change: Table-driven test that iterates every registered error code and asserts `Remedy != ""`. Runs in CI pre-merge gate.
Effort: S
Done when: Test added to `make ci` target; fails if any error code missing remedy.
Depends on: T-008

---

## Phase P3 — MCP Tool Surface

### T-011 — Add forge_ship_* MCP tools to cmdmcp
File: `internal/cli/cmdmcp/tools.go`
Change: Register 10 MCP tools (see arch.md §4 table). Each tool wraps the equivalent `forge ship <checkpoint>` command. Input schemas validated against static JSON schema files.
Effort: M
Done when: `forge mcp info --json` lists all 10 tools; `TestMCPToolInventory` verifies tool names.
Depends on: T-005

### T-012 — Static MCP schema files
File: `docs/mcp/tools.json`
Change: Publish static JSON schema for all 10 MCP tools. Each tool entry: `name`, `description`, `inputSchema` (JSON Schema draft-07).
Effort: S
Done when: `tools.json` validates against JSON Schema meta-schema; contract test `TestMCPSchemaContract` passes on every PR.
Depends on: T-011

### T-013 — MCP response schema == subprocess envelope contract test
File: `internal/cli/cmdmcp/schema_test.go`
Change: `TestMCPResponseMatchesEnvelope` — call each MCP tool in dry-run mode and assert the returned JSON has the same top-level fields as `llmresponse.Response`.
Effort: S
Done when: Test passes for all 10 tools.
Depends on: T-011, T-012

---

## Phase P4 — Spend Transparency

### T-014 — Populate cost_usd + llm_tokens_used per checkpoint
File: `internal/llmresponse/cost.go`
Change: `CostFromLedger(ctx context.Context) (costUSD float64, tokensUsed int)` — reads from `tokenledger.FromContext(ctx)` for the current invocation scope.
Effort: S
Done when: `TestCostFromLedger_Populated` verifies non-zero values after a real (non-dry-run) LLM call.
Depends on: T-001

### T-015 — Wire cost into Wrap()
File: `internal/llmresponse/envelope.go`
Change: `Wrap()` calls `CostFromLedger(ctx)` and sets `Response.CostUSD` and `Response.LLMTokensUsed`.
Effort: S
Done when: `forge ship spec "x" --dry-run` in LLM mode returns `cost_usd: 0` (dry-run) and `llm_tokens_used: 0`; real run returns non-zero.
Depends on: T-014, T-001

### T-016 — Budget cap returns FORGE-2001 with remedy via envelope (AC-8)
File: `internal/llmbudget/budget.go`
Change: When `FORGE_BUDGET_USD` cap is hit, return `errcode.FORGE2001` with `Remedy: "export FORGE_BUDGET_USD=<n> && " + originalCommand`.
Effort: S
Done when: `TestBudgetCap_ReturnsForge2001WithRemedy` passes; remedy string contains `FORGE_BUDGET_USD`.
Depends on: T-009, T-014

---

## Phase P5 — Migration Note + forge doctor Integration

### T-017 — BREAKING.md migration note for piped-output scripts
File: `BREAKING.md`
Change: Add entry under v1.7.0: "Piped subprocess output (`forge ... | grep`) now emits JSON by default. Add `--human` to restore text output."
Effort: S
Done when: Entry present in BREAKING.md; `forge doctor` prints advisory if legacy pipe pattern detected.
Depends on: T-005

### T-018 — forge doctor advisory for affected pipe scripts
File: `internal/cli/cmdhygiene/doctor.go`
Change: Add check: scan shell scripts in project root for `forge ... |` patterns without `--human`; emit advisory `FORGE-ADV-001: piped forge output will switch to JSON in v1.7.0`.
Effort: S
Done when: `TestDoctor_PipedForgeAdvisory` passes; advisory printed at correct severity (warn, not block).
Depends on: T-017

### T-019 — CHANGELOG.md v1.7.0 entry
File: `CHANGELOG.md`
Change: Add v1.7.0 section: LLM-first rearchitecture, new `FORGE_LLM_MODE`, `--json` default for non-TTY, `internal/llmresponse` package, 10 MCP tools, spend transparency.
Effort: S
Done when: Entry present and follows existing CHANGELOG format; reviewed in `forge ship verify`.
Depends on: T-015, T-016

---

## Summary

| Phase | Tasks | Effort |
|---|---|---|
| P1 — Envelope + Mode Detection | T-001–T-007 | ~2d |
| P2 — remedy + Error Registry | T-008–T-010 | ~1d |
| P3 — MCP Tool Surface | T-011–T-013 | ~1d |
| P4 — Spend Transparency | T-014–T-016 | ~0.5d |
| P5 — Migration + Doctor | T-017–T-019 | ~0.5d |
| **Total** | **19 tasks** | **~5d** |

Execution order: T-001 → T-002,T-003 → T-004 → T-005 → T-006 → T-007 → T-008 → T-009,T-010 → T-011 → T-012,T-013 → T-014 → T-015,T-016 → T-017 → T-018,T-019

