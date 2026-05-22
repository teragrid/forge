# Section 06 — Integration Design

> **Template**: DAB Full
> Define all external integrations, event contracts, and the OpenAPI specification.
> The `openapi.yaml` file in `.forge/specs/<slug>/openapi.yaml` is the **authoritative**
> API contract — this section provides context and rationale around it.

---

## 6.1 Integration Inventory

| Integration | Direction | Protocol | Auth | Sync/Async | Owner |
|-------------|-----------|----------|------|-----------|-------|
| TODO | Inbound | HTTPS REST | Bearer JWT | Sync | Team A |
| TODO | Outbound | gRPC | mTLS | Sync | Team B |
| TODO | Outbound | AMQP / Kafka | SASL | Async | Platform |

---

## 6.2 OpenAPI Contract

> The OpenAPI 3.1.0 specification for this feature is generated and maintained at:
>
> ```
> .forge/specs/<slug>/openapi.yaml
> ```
>
> Run `forge ship arch` to (re)generate the stub. The arch checkpoint uses **KB injection**
> (ADR-026): top-5 matching knowledge-base entries are appended to the LLM system prompt
> (tags searched: `openapi`, `architecture`, `api-contract`, `supabase`).
> Edit `openapi.yaml` to match the actual API before running `forge ship test` and `forge ship breakdown`.

### API style declaration

> **Choose one** — `forge ship` detects the style from `openapi.yaml` path prefixes and injects
> targeted guidance into test, breakdown, and code checkpoints:

- [ ] **Standard REST** — paths follow `/api/v1/{resource}` convention
- [ ] **Supabase RPC** — paths use `/rest/v1/rpc/{function_name}`; include Supabase `anon` / `service_role` security schemes
- [ ] **Mixed** (justify below): _TODO — explain why both styles are needed_

**Declared style**: _TODO_

> If Supabase RPC: downstream checkpoints will generate tasks for PostgreSQL function creation,
> `GRANT EXECUTE`, RLS policies, and `.rpc()` client integration tests automatically.

### Contract governance rules

1. **Additive-only changes** are non-breaking: adding new optional fields, new paths, new response codes.
2. **Breaking changes** (removing fields, changing types, renaming paths) require:
   - A new API version (URI versioning: `/api/v2/…`)
   - A 90-day deprecation notice on the old version
   - A migration guide linked in the changelog
3. All breaking changes must be reviewed by the API Design role owner before merge.
4. **Supabase RPC functions** — function signature changes are treated as breaking changes;
   create a new versioned function (`get_profile_v2`) rather than altering the existing one.

### Versioning strategy

> Choose one:
> - [ ] URI versioning (`/api/v1/`, `/api/v2/`)
> - [ ] Header versioning (`Accept: application/vnd.forge.v2+json`)
> - [ ] Supabase RPC function versioning (`rpc/{fn}_v2`)

**Chosen strategy**: _TODO_

### Current API version

| Version | Status | Sunset Date |
|---------|--------|------------|
| v1 | Active | — |

---

## 6.3 Event Contracts

> List all events published and consumed by this feature.
> Use CloudEvents v1.0 envelope format.

### Published events

```yaml
# Example CloudEvent envelope
type: com.example.todo.created      # reverse-domain, past tense
specversion: "1.0"
source: /services/todo-service
datacontenttype: application/json
data:
  id: <uuid>
  # TODO: add payload fields
```

| Event Type | Producer | Consumer(s) | Schema | Idempotent |
|-----------|---------|------------|--------|-----------|
| com.example.todo.created | TODO Service | TODO | TODO | Yes |

### Consumed events

| Event Type | Producer | Handler | At-least-once? |
|-----------|---------|---------|---------------|
| TODO | TODO | TODO | Yes |

---

## 6.4 Webhook / Callback Contracts

| Trigger | URL Pattern | Payload Schema | Retry Policy | HMAC Signature |
|---------|------------|----------------|-------------|----------------|
| TODO | `https://customer.example.com/hooks/todo` | See openapi.yaml | 3× exp backoff | Yes — `X-Forge-Signature` |

---

## 6.5 Third-Party SDK / Library Dependencies

> List any new third-party libraries introduced by this integration.
> Each must have a corresponding ADR (per `AGENTS.md` — no third-party deps without an ADR).

| Library | Version | Purpose | ADR |
|---------|---------|---------|-----|
| TODO | TODO | TODO | ADR-0XX |

---

## 6.6 Contract Testing Strategy

> Integration contracts are enforced by consumer-driven contract tests.

| Contract | Framework | Location | Runs in CI |
|----------|-----------|---------|-----------|
| openapi.yaml conformance | `go-swagger` / `kin-openapi` | `tests/<slug>.integration.test.ts` | Yes |
| Consumer contract | Pact | `tests/<slug>.contract.test.*` | Yes |

---

*Next section: [07-infrastructure-design.md](07-infrastructure-design.md)*
