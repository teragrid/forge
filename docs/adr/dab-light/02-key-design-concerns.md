# Section 02 — Key Design Concerns (Light)

> **Template**: DAB Light
> List the top risks and decisions. One row per concern.

---

| # | Area | Concern | Risk | Mitigation |
|---|------|---------|------|-----------|
| 1 | Architecture | TODO | Medium | TODO |
| 2 | Security | Auth/authz boundary | High | Define scopes in openapi.yaml |
| 3 | Data | Schema migration safety | High | Backwards-compatible only |
| 4 | API | openapi.yaml defined | High | Edit .forge/specs/<slug>/openapi.yaml |
| 5 | API | **API style declared** | High | Choose REST (`/api/v1/`) **or** Supabase RPC (`/rest/v1/rpc/`); `forge ship` detects from path prefix |
| 6 | API | KB injection coverage | Low | `forge ship arch` injects top-5 KB entries; add `.forge/knowledge/` entries if project-specific guidance is missing |
| 7 | Performance | p99 latency target | Medium | Define SLO; instrument histogram |

---

## Open Questions

| Question | Owner | Due |
|----------|-------|-----|
| TODO | TODO | TODO |

---

*Next: [03-high-level-architecture.md](03-high-level-architecture.md)*
