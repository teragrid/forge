# Architecture: Forge Framework — AI-Agent-First Rearchitecture

> **Status**: Draft — awaiting review before `forge ship code`
> **Spec**: `.forge/specs/agent-first-rearchitect/spec.md`
> **Target release**: v1.7.0

---

## 1. Component Topology

```
┌─────────────────────────────────────────────────────────────────┐
│  Agent Consumers (LangGraph / AutoGen / Forge-native / curl)    │
│  FORGE_AGENT_ID + FORGE_AGENT_TOKEN in env                      │
└────────────────────────┬────────────────────────────────────────┘
                         │  JSON over stdin/stdout or HTTP/SSE
┌────────────────────────▼────────────────────────────────────────┐
│  forge CLI entrypoint  (cmd/forge/main.go)                      │
│  • Detects FORGE_AGENT_MODE=1 or NO_COLOR=1 → JSON mode        │
│  • Detects TTY + no flag → human mode (backwards compat)        │
└──┬──────────────┬──────────────┬──────────────┬─────────────────┘
   │              │              │              │
   ▼              ▼              ▼              ▼
[cmdship]    [cmdagent]    [cmdaudit]    [cmdpolicy]
Ship pipeline  Task graph    Audit trail   Approval gates
checkpoints    orchestrator  & agent IDs   & webhooks
   │              │
   ▼              ▼
[internal/agentidentity]   new package — agent token validation,
                           per-agent spend caps, audit tagging
   │
   ▼
[internal/taskgraph]       new package — agent-tasks.yml parser,
                           DAG scheduler, parallelism + retry
   │
   ▼
[internal/approvalgate]    new package — policy-file auto-approve,
                           webhook async gate, SSE callback
   │
   ├──► [internal/llmbudget]    (existing) — per-agent budget caps
   ├──► [internal/tokenledger]  (existing) — per-agent token accounting
   ├──► [internal/audit]        (existing) — extended with agent_id field
   ├──► [internal/fssandbox]    (existing) — per-task fs isolation
   ├──► [internal/outbox]       (existing) — webhook delivery transport
   └──► [internal/telemetry]    (existing) — OTel events + agent spans
```

**New packages** (all under `internal/`):

| Package | Responsibility |
|---|---|
| `agentidentity` | Validate `FORGE_AGENT_TOKEN`, resolve `FORGE_AGENT_ID`, attach to context |
| `taskgraph` | Parse `agent-tasks.yml`, build DAG, schedule with goroutine pool (max 10) |
| `approvalgate` | Policy-file auto-approve, webhook gate with configurable timeout |
| `agentapi` | JSON schema types for all machine-readable command responses |

**Modified packages**:

| Package | Change |
|---|---|
| `internal/audit` | Add `agent_id`, `agent_mode` fields to every log entry |
| `internal/llmbudget` | Accept per-agent spend cap from `agentidentity.Context` |
| `internal/cli/cmdship` | Wrap all output in `agentapi.Response` when in agent mode |
| `cmd/forge/main.go` | Inject agent identity into context before dispatch |

---

## 2. Agent-Mode I/O Contract

All commands emit the following JSON envelope when `FORGE_AGENT_MODE=1`, `--json` flag, or `NO_COLOR=1`:

```jsonc
{
  "ok": true,                  // false on any error
  "checkpoint": "spec",        // current pipeline checkpoint name
  "status": "completed",       // pending | running | completed | failed
  "path": ".forge/specs/...",  // primary artifact path (if any)
  "agent_id": "agent-1",       // from FORGE_AGENT_ID env (empty if human)
  "duration_ms": 142,
  "error": null,               // FORGE-XXXX error code + message on failure
  "next": ["forge ship arch …"] // suggested next actions
}
```

Error envelope (`ok: false`):
```jsonc
{
  "ok": false,
  "error": { "code": "FORGE-2001", "message": "budget cap exceeded", "details": {} },
  "checkpoint": "code",
  "agent_id": "agent-1"
}
```

Schema source of truth: `internal/agentapi/response.go` (generated; do not hand-edit).

---

## 3. Task Graph (`agent-tasks.yml`)

```yaml
# .forge/agent-tasks.yml
version: "1"
tasks:
  spec:
    command: forge ship spec "{{.description}}"
    outputs: [".forge/specs/{{.name}}/spec.md"]

  arch:
    depends_on: [spec]
    command: forge ship arch "{{.description}}"
    outputs: [".forge/specs/{{.name}}/arch.md"]

  code:
    depends_on: [arch]
    command: forge ship code "{{.description}}"
    approval: auto          # or: webhook, human

  test:
    depends_on: [code]
    command: forge ship test "{{.description}}"

  breakdown:
    depends_on: [test]
    command: forge ship breakdown "{{.description}}"

  verify:
    depends_on: [breakdown]
    command: forge ship verify "{{.description}}"
    approval: human         # always require human sign-off before ship
```

`forge agent run --graph .forge/agent-tasks.yml --var description="..." --var name="..."` drives the full pipeline. Independent tasks (e.g. parallel test shards) execute concurrently up to `--concurrency 10`.

---

## 4. Agent Identity & Authorisation

```
FORGE_AGENT_ID    = "my-coding-agent"      # logical name, free-form
FORGE_AGENT_TOKEN = "fgt_..."              # HMAC-signed opaque token
FORGE_BUDGET_USD  = "2.00"                 # per-invocation cap (optional)
```

- Tokens are validated by `internal/agentidentity.Validate()` on every invocation — no caching, no restart needed.
- Token format: `fgt_<base64url(claims)>.<base64url(hmac-sha256)>` — self-contained, no network call.
- Claims include: `agent_id`, `allowed_commands` (glob list), `budget_usd`, `expires_at`.
- Missing/invalid token: `FORGE-4001` — operation proceeds in **anonymous agent mode** (no spend cap, audit entry has `agent_id: ""`).
- Human users are unaffected; token env vars are ignored when TTY is detected and `--json` is absent.

---

## 5. Approval Gate Architecture

```
checkpoint gate reached
        │
        ▼
read .forge/config.yml → approval_policy
        │
   ┌────┴────────────────┐
   │ auto                │ webhook              │ human (default)
   ▼                     ▼                      ▼
pass immediately    POST JSON to URL        interactive y/n
                    wait for {"approved":true}  (TTY only)
                    timeout → FORGE-4010
```

Config example (`.forge/config.yml`):
```yaml
approval:
  policy: webhook
  webhook_url: https://my-approver.example.com/forge/approve
  timeout_seconds: 300
  # policy: auto   — no gate
  # policy: human  — always interactive
```

Webhook payload (POST):
```jsonc
{
  "event": "approval_requested",
  "checkpoint": "code",
  "spec": "agent-first-rearchitect",
  "agent_id": "my-coding-agent",
  "artifacts": [".forge/specs/agent-first-rearchitect/arch.md"],
  "callback_token": "…"   // HMAC nonce; must be echoed back to confirm
}
```

---

## 6. Observability

Every agent action emits an OpenTelemetry span on the `forge.agent` tracer:

| Signal | Name | Labels |
|---|---|---|
| Span | `forge.ship.checkpoint` | `checkpoint`, `agent_id`, `status` |
| Counter | `forge_agent_runs_total` | `agent_id`, `checkpoint`, `result` |
| Histogram | `forge_checkpoint_duration_ms` | `checkpoint` |
| Counter | `forge_budget_exceeded_total` | `agent_id` |
| Counter | `forge_approval_gate_total` | `policy`, `outcome` |

Export via `OTEL_EXPORTER_OTLP_ENDPOINT` (existing `internal/telemetry` wiring) — no new config needed.

---

## 7. Non-Functional Requirements

| NFR | Target | Measurement |
|---|---|---|
| CLI cold start (no LLM) | < 50 ms | `time forge --version` |
| Checkpoint dispatch overhead | < 200 ms (excl. LLM) | `--dry-run --json` benchmark |
| Task graph scheduling | < 10 ms for 100-node DAG | unit benchmark |
| Concurrent sub-tasks | 10 goroutines, zero races | `go test -race ./internal/taskgraph/...` |
| Agent token validation | < 1 ms | unit benchmark |
| Webhook gate timeout | configurable 1–3600 s | integration test |

---

## 8. Security Threat Model

| Threat | STRIDE | Mitigation |
|---|---|---|
| Rogue agent escalates privileges | Elevation | Token `allowed_commands` claim; explicit glob allowlist |
| Token replay after expiry | Spoofing | `expires_at` checked on every invocation; short TTLs recommended (1 h) |
| Agent exceeds budget | DoS (cost) | `FORGE_BUDGET_USD` hard cap via `llmbudget`; `FORGE-2001` on breach |
| Webhook URL SSRF | Tampering | Allowlist in `.forge/config.yml`; reject private IP ranges |
| Task graph cycle hangs process | DoS | Cycle detection in `taskgraph.Build()` → `FORGE-5001` at load time |
| Prompt injection via spec content | Tampering | PIIFilter + existing guardrails applied before LLM calls |
| Concurrent checkpoint write conflict | Tampering | Optimistic lock (file mtime check) → `FORGE-5002` |

---

## 9. Migration & Backwards Compatibility

| Scenario | Behaviour |
|---|---|
| No `FORGE_AGENT_MODE`, has TTY | Existing human UX unchanged |
| No `FORGE_AGENT_MODE`, no TTY | Automatically switches to JSON mode (safe for scripts) |
| `FORGE_AGENT_MODE=1` | Full agent mode, JSON output, no prompts |
| `--human` flag | Forces human mode even in CI/agent contexts |
| Old `forge ship spec …` invocation | Works identically; new JSON envelope only added when agent mode active |
| Existing `.forge/specs/` directories | Not touched; task graph is opt-in |

No breaking changes. All new flags are additive. `--human` provides full rollback to current behaviour.

---

## 10. Phased Delivery

| Phase | Scope | Target |
|---|---|---|
| P1 | `agentapi` JSON envelope, `NO_COLOR`/`FORGE_AGENT_MODE` detection | v1.7.0-alpha |
| P2 | `agentidentity` token validation, per-agent audit entries | v1.7.0-beta |
| P3 | `taskgraph` DAG scheduler, `forge agent run` command | v1.7.0-rc |
| P4 | `approvalgate` (auto + webhook), `.forge/config.yml` policy | v1.7.0 |
| P5 | OTel spans, per-agent budget caps wired into `llmbudget` | v1.7.1 |

### Disaster Recovery

> TODO: Document failover trigger, DNS TTL, and data-replication lag tolerance.

---

## ADR Summary

**Status**: Proposed

**Context**: rearchitect forge framework for ai-agent-first workflows

**Decision**: TODO — record the key architectural decision here.

**Consequences**:
- ✓ TODO: positive consequence
- ✗ TODO: trade-off or risk to mitigate
