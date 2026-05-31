# Architecture: llm-driven automation testing mcp integration

> **Status**: Draft — fill in each section before running forge ship test.

---

## 1. Component Topology

> TODO: Describe components, their boundaries, and relationships.
>
> Example:
> - **API Gateway** — routes requests; enforces auth
> - **Feature Service** — owns business logic; exposes gRPC + REST
> - **Database** — PostgreSQL 15; single writer, read replicas

## 2. API Contracts

> See openapi.yaml in this directory for the full API contract.
> Edit openapi.yaml and re-run "forge ship arch" to validate.
>
> **API style** (choose one and delete the other):
> - **Standard REST** — resource-oriented paths, e.g. POST /api/v1/{resource}
> - **Supabase RPC** — PostgREST function exposure, e.g. POST /rest/v1/rpc/{function_name}
>
> Summary of primary endpoints (auto-populated from openapi.yaml when available):
>
> | Method | Path | Description | Schema |
> |--------|------|-------------|--------|
> | POST | /api/v1/TODO | TODO | TODORequest -> TODOResponse |
> | POST | /rest/v1/rpc/TODO | (Supabase RPC alternative — delete if not using Supabase) | params -> result |

## 3. Data Model & Consistency

> TODO: Describe data entities, migration strategy, and consistency model.

### Entities

- **Entity**: fields, indexes, constraints

### Consistency Model

> TODO: Choose one: strong consistency | eventual consistency (saga) | two-phase commit

### Migration Plan

> TODO: Describe migration steps, rollback script, and CI test coverage.

## 4. Non-Functional Requirements

| NFR          | Target         | Measurement              |
|--------------|----------------|--------------------------|
| p99 latency  | < 200 ms       | Prometheus histogram     |
| Throughput   | > 100 RPS      | Load test (k6)           |
| Availability | ≥ 99.9%       | SLO burn-rate alert      |

## 5. Security Threat Model

> TODO: Identify STRIDE threats and mitigations.

| Threat | Category | Mitigation |
|--------|----------|------------|
| ... | Spoofing | ... |

### Auth/Authz Boundaries

> TODO: Specify who may call each endpoint, required scopes/roles, and token validation.

## 6. Deployment & Observability

### Deployment Topology

> TODO: Describe replicas, regions, autoscaling policy, and IaC module.

### Observability

| Signal  | Name                   | Description |
|---------|------------------------|-------------|
| Counter | feature_requests_total | ... |
| Histogram | feature_latency_seconds | ... |

### Disaster Recovery

> TODO: Document failover trigger, DNS TTL, and data-replication lag tolerance.

---

## ADR Summary

**Status**: Proposed

**Context**: llm-driven automation testing mcp integration

**Decision**: TODO — record the key architectural decision here.

**Consequences**:
- ✓ TODO: positive consequence
- ✗ TODO: trade-off or risk to mitigate
