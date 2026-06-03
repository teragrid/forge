# Spec: Rearchitect Forge Framework for AI-Agent-First Workflows

## Status Summary
- Lifecycle: Draft
- Version Scope: MINOR (v1.7.0)
- Owner: forge-core
- Last Updated: 2026-06-02
- Checkpoint Progress: 1/7

## What

Rearchitect the Forge CLI framework so that AI agents (LLM-driven, autonomous, non-interactive) are the primary consumers — replacing the current human-interactive CLI as the first-class interface. Humans retain a thin review/approval surface but the default execution path is fully agent-operated.

Key pillars:
1. **Machine-readable I/O by default** — all commands emit structured JSON unless `--human` is explicitly passed; no spinner/ANSI decoration on stdout.
2. **Stateless, resumable pipeline contracts** — every checkpoint exposes a deterministic input/output schema so an agent can call, inspect, resume, or roll back without side-channel knowledge.
3. **Agent identity & authorisation** — `FORGE_AGENT_ID` env var; agent tokens separate from human PATs; per-agent spend caps, audit entries, and rate limits.
4. **Headless approval gates** — async approval webhooks and policy-file-driven auto-approve replace interactive `y/n` prompts; human review is opt-in via `--require-human-review`.
5. **Declarative task graph** — replace imperative checkpoint chaining with a YAML task graph (`agent-tasks.yml`) that an orchestrator (LangGraph, AutoGen, Forge-native) can schedule, parallelise, and retry.
6. **Observability hooks** — structured event stream (OpenTelemetry-compatible) for every agent action; agents can subscribe to events from sibling agents.
7. **Sandboxed execution** — each agent sub-task runs in an isolated `fssandbox` scope; resource quotas enforced by `llmbudget` + `tokenledger`.

## Why

The current Forge CLI was designed around a human sitting at a terminal: interactive prompts, ANSI colours, single-threaded checkpoints, and `y/n` gates. As AI agents become the primary consumers (coding agents, review agents, QA agents), this design creates friction:

- Agents cannot parse ANSI-decorated output reliably.
- Interactive prompts block unattended pipelines.
- No agent identity means no per-agent audit trail or spend control.
- Checkpoint state is opaque — agents cannot introspect or branch the pipeline.
- No event bus means agents cannot coordinate or react to each other's progress.

This rearchitecture makes Forge the execution layer for multi-agent AI development workflows, while keeping it fully backwards-compatible for human users via the `--human` flag.

## Acceptance Criteria

- [ ] Given an agent calls `forge ship spec --json`, When the command completes, Then stdout contains valid JSON with `status`, `path`, and `checkpoint` keys and no ANSI escape codes.
- [ ] Given `FORGE_AGENT_ID=agent-1` is set and `FORGE_AGENT_TOKEN` is valid, When any forge command runs, Then the audit log entry includes `agent_id: "agent-1"` and the human audit trail is separate.
- [ ] Given an `agent-tasks.yml` task graph exists in `.forge/`, When `forge agent run --graph agent-tasks.yml` is invoked, Then tasks execute in dependency order with parallelism where the graph allows.
- [ ] Given a checkpoint fails, When an agent calls `forge ship --resume --json`, Then the pipeline resumes from the last successful checkpoint without re-running prior steps and returns a JSON progress report.
- [ ] Given `approval_policy: auto` is set in `.forge/config.yml`, When a checkpoint gate is reached, Then the gate passes automatically without any interactive prompt.
- [ ] Given `approval_policy: webhook` is set with a URL, When a checkpoint gate is reached, Then Forge POSTs a JSON payload to the webhook and waits for a `{"approved": true}` response before continuing.
- [ ] Given an agent sub-task exceeds its `budget_usd` cap, When the LLM call is attempted, Then it is rejected with `FORGE-2001` and the pipeline halts cleanly with a JSON error response.
- [ ] Given `--human` is NOT passed, When `forge` runs in a TTY, Then it falls back to human-friendly output automatically (backwards-compat).
- [ ] Given two agents run the same spec concurrently, When both attempt to write the same checkpoint, Then one succeeds and the other receives a `FORGE-XXXX` conflict error (optimistic locking).

## Non-Functional Requirements

- [ ] All `forge ship` subcommands must respond within 200 ms (excluding LLM latency) when called with `--dry-run --json`.
- [ ] JSON schema for every command's output must be published under `docs/agent-api/` and validated by a contract test on every PR.
- [ ] Agent token rotation must not require a process restart; tokens are re-read from env/vault on each invocation.
- [ ] No stdout ANSI codes when `NO_COLOR=1` or `FORGE_AGENT_MODE=1` is set.
- [ ] `forge agent run` must support at least 10 concurrent sub-task goroutines without data races (verified by `-race` tests).

## Out of Scope

- Replacing the underlying LLM provider routing (`tierrouter`) — that is unchanged.
- Removing the human CLI surface — `--human` / TTY autodetect keeps it fully functional.
- Building a graphical UI or web dashboard.
- Agent-to-agent networking across machines (single-host only in this phase).
- Authentication server / identity provider — agents bring their own tokens.

## Open Questions

1. Should `agent-tasks.yml` be a new top-level format or extend the existing spec/checkpoint schema?
2. Webhook approval: synchronous poll vs. long-poll vs. SSE callback — which fits best with existing `outbox` package?
3. Should agent identity integrate with the existing `audit` package or require a new `agentaudit` sub-package?
