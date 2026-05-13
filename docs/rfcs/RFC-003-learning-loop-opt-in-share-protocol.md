# RFC-003 — Learning Loop Opt-In Share Protocol

| Field | Value |
|-------|-------|
| RFC | 003 |
| Title | Learning Loop Opt-In Share Protocol |
| Author | Forge Maintainers |
| Status | **Accepted** |
| Created | 2025-03-05 |
| Implemented in | `internal/learningloop` |

---

## Summary

Define a privacy-first protocol for teams to voluntarily share anonymised
prompt/outcome pairs with the Forge community. The aggregated data improves
default instruction packs and surfaces common failure patterns.

---

## Motivation

Forge accumulates valuable signal about which prompts work, which fail, and
what error patterns repeat across projects. Teams that want to contribute this
signal back to the community have no mechanism to do so safely today.

Requirements:
1. **Opt-in only** — zero data leaves the machine without explicit user action.
2. **No PII** — file paths, project names, and author info are stripped before any share.
3. **Auditable payload** — the full payload schema is public and versioned.
4. **Local-first** — teams can run their own aggregator; no mandatory cloud endpoint.

---

## Design

### Event schema (v1)

```json
{
  "schema_version": "1",
  "events": [
    {
      "id": "<random-uuid>",
      "verb": "review",
      "model": "claude-3-5-sonnet-20241022",
      "input_tokens": 1234,
      "output_tokens": 456,
      "outcome": "success",
      "tags": {"language": "go"},
      "recorded_at": "2025-03-05T12:00:00Z"
    }
  ]
}
```

Fields **never** included: file paths, project names, prompt text, git history,
user identifiers.

### Enablement

```bash
forge learn share --enable --aggregator https://community.forge.dev/api/v1/share
```

This writes to `.forge/learn-config.json`. The config file must be git-ignored.

### Local aggregator

Teams can run `forge learn aggregate` to produce local statistics without
sharing externally. This is the default experience.

---

## Privacy guarantees

- The client (`internal/learningloop.Client`) performs stripping before queuing.
- Events are queued in `.forge/learn-queue.json` (mode 0600) and never readable by other users.
- The aggregator endpoint is configurable and defaults to `localhost:7420` (no-op).
- Sharing can be disabled at any time with `forge learn share --disable`.

---

## Alternatives considered

- **Telemetry framework reuse** — rejected; telemetry captures operational metrics, not prompt quality signals. Keeping them separate avoids scope creep.
- **GitHub Discussions integration** — rejected; requires GitHub auth and doesn't work air-gapped.

---

## Decision

Accepted. Implemented in `internal/learningloop/client.go` and `aggregator.go`.
The community aggregator endpoint is planned for post-v1.0.
