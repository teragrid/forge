# Section 05 — Detailed Design

> **Template**: DAB Full
> Component-level design: modules, key functions, sequence diagrams, and error handling.

---

## 5.1 Module Breakdown

| Module | Package / Path | Responsibility |
|--------|---------------|----------------|
| API Handler | `internal/api/` | HTTP/gRPC handler, request parsing, response serialisation |
| Domain Service | `internal/service/` | Business logic, orchestration |
| Repository | `internal/repository/` | Data access layer, query builders |
| Event Publisher | `internal/events/` | Outbox pattern, event dispatch |

---

## 5.2 Critical Path Sequence Diagram

```
Client           API Gateway          Service             Repository        Event Bus
  │                   │                  │                    │                │
  │ POST /api/v1/TODO │                  │                    │                │
  │──────────────────>│                  │                    │                │
  │                   │ Validate JWT     │                    │                │
  │                   │ Check rate-limit │                    │                │
  │                   │────────────────>│                    │                │
  │                   │                  │ Begin Tx           │                │
  │                   │                  │───────────────────>│                │
  │                   │                  │ Insert entity      │                │
  │                   │                  │───────────────────>│                │
  │                   │                  │ Insert outbox row  │                │
  │                   │                  │───────────────────>│                │
  │                   │                  │ Commit Tx          │                │
  │                   │                  │<───────────────────│                │
  │                   │                  │                    │  Publish event │
  │                   │                  │────────────────────────────────────>│
  │                   │<────────────────│ 201 Created        │                │
  │<──────────────────│ 201 Created      │                    │                │
```

> TODO: Replace with actual sequence for the primary happy path.

---

## 5.3 Key Function Signatures

```go
// TODO: List key function signatures with types.
// Example:
//
// func CreateTODO(ctx context.Context, req *CreateTODORequest) (*CreateTODOResponse, error)
// func GetTODO(ctx context.Context, id uuid.UUID) (*TODO, error)
```

---

## 5.4 Error Handling Strategy

| Error Class | HTTP Status | Error Code | Behaviour |
|-------------|------------|------------|-----------|
| Validation failure | 400 | FORGE-XXXX | Return structured error body; no retry |
| Authentication | 401 | FORGE-XXXX | Return 401; client must re-authenticate |
| Not found | 404 | FORGE-XXXX | Return 404; safe to retry with different ID |
| Conflict | 409 | FORGE-XXXX | Return 409 + idempotency key guidance |
| Transient / server | 500/503 | FORGE-XXXX | Return 503 with `Retry-After` header |

> Error response envelope (matches OpenAPI `ErrorBody` schema in `openapi.yaml`):
>
> ```json
> { "code": "FORGE-XXXX", "message": "human-readable description" }
> ```

---

## 5.5 Idempotency Design

> All mutating operations (POST/PUT/PATCH) must support the `Idempotency-Key` header.
> See `openapi.yaml` for the header parameter definition on each operation.

| Operation | Key Scope | Storage | TTL |
|-----------|-----------|---------|-----|
| POST /api/v1/TODO | User + Idempotency-Key | Redis | 24 h |

---

## 5.6 Pagination Design

| Endpoint | Strategy | Page Size | Cursor |
|----------|----------|-----------|--------|
| GET /api/v1/TODO | Cursor-based | Default: 20, Max: 100 | Opaque base64 cursor |

---

## 5.7 Concurrency & Race Conditions

| Scenario | Risk | Mitigation |
|----------|------|------------|
| Concurrent writes | Data loss / duplicate | Optimistic locking (`version` field) |
| Double-delivery (webhook) | Duplicate side-effects | Idempotency key + deduplication table |
| Read-modify-write | Lost update | `SELECT FOR UPDATE` or compare-and-swap |

---

*Next section: [06-integration-design.md](06-integration-design.md)*
