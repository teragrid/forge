# Section 05 — Detailed Design (Light)

> **Template**: DAB Light

---

## Critical Path (happy path)

```
Client -> API Gateway -> Service -> DB -> return 201
```

> TODO: replace with actual sequence.

---

## Key Functions

```go
// TODO: list key function signatures
// func CreateTODO(ctx context.Context, req *CreateTODORequest) (*CreateTODOResponse, error)
```

---

## Error Handling

| Error | HTTP Status | Retryable |
|-------|------------|-----------|
| Validation | 400 | No |
| Not found | 404 | No |
| Conflict | 409 | With idempotency key |
| Server error | 500/503 | Yes (with backoff) |

---

## Idempotency

All POST/PUT/PATCH operations support `Idempotency-Key` header.
See `openapi.yaml` for header parameter definitions.

---

*Next: [06-integration-design.md](06-integration-design.md)*
