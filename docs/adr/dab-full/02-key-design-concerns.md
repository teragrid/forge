# Section 02 — Key Design Concerns

> **Template**: DAB Full
> Surface the most important architectural decisions and risks BEFORE design work begins.
> Each concern maps to a role in the `forge ship arch` self-debate engine.

---

## 2.1 System Architecture Concerns (`sys-arch`)

| # | Concern | Risk | Proposed Mitigation |
|---|---------|------|---------------------|
| 1 | Component boundary ambiguity | High | Define explicit service contracts at design time |
| 2 | Coupling between services | Medium | Use event-driven integration; minimize synchronous calls |
| 3 | Technology selection | Medium | ADR required per technology choice |

---

## 2.2 Security Architecture Concerns (`sec-arch`)

| # | Concern | Risk | Proposed Mitigation |
|---|---------|------|---------------------|
| 1 | Authentication/Authorization boundary | High | Define auth surface; document required scopes per endpoint |
| 2 | Data classification | High | Classify all data entities as PII / sensitive / public |
| 3 | Dependency supply-chain | Medium | Pin all dependency versions; run `forge scan security` in CI |

---

## 2.3 Data Architecture Concerns (`dat-arch`)

| # | Concern | Risk | Proposed Mitigation |
|---|---------|------|---------------------|
| 1 | Schema migration safety | High | All migrations must be backward-compatible; Blue/Green deploy |
| 2 | Data consistency model | High | Define strong vs. eventual consistency per aggregate |
| 3 | Retention & deletion | Medium | Document data-lifecycle; implement right-to-erasure |

---

## 2.4 API Design Concerns (`api-design`)

> These concerns are validated against `openapi.yaml` in each subsequent step.
> `forge ship arch` injects relevant knowledge-base entries (ADR-026) into the LLM prompt,
> so architecture guidance from the KB automatically influences the generated contract.

| # | Concern | Risk | Proposed Mitigation |
|---|---------|------|---------------------|
| 1 | OpenAPI 3.1.0 contract | High | Define `openapi.yaml`; all paths, schemas, and error shapes must be present |
| 2 | Backwards compatibility | High | No breaking changes without 90-day deprecation window; version the spec |
| 3 | Idempotency | Medium | All POST/PUT/PATCH operations must support `Idempotency-Key` header |
| 4 | **API style declared** | High | Choose REST (`/api/v1/{resource}`) **or** Supabase RPC (`/rest/v1/rpc/{fn}`) — document in `openapi.yaml` paths and in Section 03; do not mix styles without justification |
| 5 | KB injection coverage | Low | `forge ship arch` selects top-5 KB entries (tags: openapi, architecture, supabase); verify relevant entries exist in `.forge/knowledge/` |

---

## 2.5 Performance Engineering Concerns (`perf-eng`)

| # | Concern | SLO Target | Measurement |
|---|---------|-----------|-------------|
| 1 | p99 latency | < 200 ms | Prometheus histogram |
| 2 | Throughput ceiling | > 100 RPS | k6 load test |
| 3 | Cache strategy | hit rate > 90% | Cache metrics |

---

## 2.6 Platform / Infrastructure Concerns (`plat-arch`)

| # | Concern | Risk | Proposed Mitigation |
|---|---------|------|---------------------|
| 1 | IaC completeness | High | All infra must be declared in Terraform/Pulumi — no manual changes |
| 2 | Container resource limits | Medium | Define CPU/memory requests and limits per container |
| 3 | DR & failover | Medium | Define RTO/RPO; document failover runbook |

---

## 2.7 Open Questions

> List unresolved questions that must be answered before architecture is finalised.

| # | Question | Owner | Due |
|---|----------|-------|-----|
| 1 | TODO | TODO | TODO |

---

*Next section: [03-high-level-architecture.md](03-high-level-architecture.md)*
