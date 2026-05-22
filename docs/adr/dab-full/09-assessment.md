# Section 09 — Assessment & Sign-off

> **Template**: DAB Full
> Architecture readiness gate — must be completed before `forge ship test` can proceed.

---

## 9.1 Architecture Readiness Checklist

### Business context (Section 01)

- [ ] Problem statement is clear and measurable
- [ ] Stakeholders identified and notified
- [ ] Success criteria are SMART

### Design concerns (Section 02)

- [ ] All 6 concern categories reviewed (sys-arch, sec-arch, dat-arch, api-design, perf-eng, plat-arch)
- [ ] Open questions resolved or tracked

### High-level architecture (Section 03)

- [ ] Component topology diagram is up to date
- [ ] External integrations listed with auth and ownership

### API contract (openapi.yaml)

- [ ] `openapi.yaml` exists at `.forge/specs/<slug>/openapi.yaml`
- [ ] All paths, schemas, and error shapes are defined
- [ ] **API style declared**: REST (`/api/v1/…`) **or** Supabase RPC (`/rest/v1/rpc/…`) — not mixed without justification
- [ ] Security schemes and required scopes are declared (Supabase: `anon` + `service_role`)
- [ ] Breaking-change policy documented (Section 06)
- [ ] `forge ship arch` KB injection verified: top-5 relevant KB entries found (see ADR-026)

### Data design (Section 04)

- [ ] All entities catalogued with classification
- [ ] Migration steps are backwards-compatible and tested
- [ ] Retention / deletion policy defined

### Detailed design (Section 05)

- [ ] Critical path sequence diagram present
- [ ] Key function signatures defined
- [ ] Error handling covers all error classes

### Infrastructure (Section 07)

- [ ] Deployment topology defined
- [ ] IaC module or ticket created
- [ ] DR runbook drafted

### Security (Section 08)

- [ ] STRIDE threat model complete
- [ ] Auth/authz boundaries defined in openapi.yaml
- [ ] PII data classification done

---

## 9.2 Review Scores

> Score each section: 🟢 Ready | 🟡 Minor gaps — acceptable to proceed | 🔴 Blocking gaps

| Section | Reviewer | Score | Notes |
|---------|---------|-------|-------|
| 01 Business Context | TODO | 🟡 | |
| 02 Key Design Concerns | TODO | 🟡 | |
| 03 HLA | TODO | 🟡 | |
| 04 Data Design | TODO | 🟡 | |
| 05 Detailed Design | TODO | 🟡 | |
| 06 Integration + openapi.yaml | TODO | 🔴 | openapi.yaml must be edited |
| 07 Infrastructure | TODO | 🟡 | |
| 08 Security | TODO | 🟡 | |

---

## 9.3 Formal Sign-off

| Role | Name | Date | Signature |
|------|------|------|-----------|
| Engineering Lead | TODO | TODO | TODO |
| Security Reviewer | TODO | TODO | TODO |
| Product Owner | TODO | TODO | TODO |

> All 🔴 items must be resolved before sign-off.
> Once signed off, update `arch.md` status from `Draft` to `Approved`.

---

## 9.4 Post-Architecture Actions

| Action | Owner | Due | Linked Issue |
|--------|-------|-----|-------------|
| Edit openapi.yaml — replace TODO paths with real endpoints | TODO | TODO | |
| Create IaC ticket | Platform team | TODO | |
| Schedule security review | Security team | TODO | |
| Update ADR status to Approved | Tech Lead | TODO | |

---

*Architecture review complete. Proceed to `forge ship test`.*
