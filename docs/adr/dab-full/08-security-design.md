# Section 08 — Security Design

> **Template**: DAB Full
> STRIDE threat model, auth/authz design, compliance requirements, and security testing plan.
> All API security controls must be reflected in `openapi.yaml` (security schemes and scopes).

---

## 8.1 STRIDE Threat Model

| Threat | Category | Component | Likelihood | Impact | Mitigation | Status |
|--------|---------|-----------|-----------|--------|-----------|--------|
| Token forgery | Spoofing | API Gateway | Medium | High | Validate JWT signature at gateway; short TTL | TODO |
| SQL injection | Tampering | Repository | Low | High | Parameterised queries only; ORM enforced | TODO |
| Data exfiltration | Information Disclosure | API | Medium | High | Field-level access control; audit logging | TODO |
| Request flooding | Denial of Service | API Gateway | High | Medium | Rate limiting (per-user and per-IP) | TODO |
| Privilege escalation | Elevation of Privilege | Service | Low | High | Least-privilege RBAC; deny-by-default | TODO |
| TODO | Repudiation | TODO | TODO | TODO | Immutable audit log | TODO |

> Reference: https://owasp.org/www-community/Threat_Modeling_Process

---

## 8.2 Authentication & Authorisation Design

### Authentication

| Method | Scope | Token Store | Rotation |
|--------|-------|------------|---------|
| Bearer JWT (OAuth2) | External clients | — | 1-hour TTL |
| mTLS | Service-to-service | Cert manager | 30-day rotation |
| API Key | Third-party | Hashed in DB | Manual revoke |

### Authorisation (RBAC)

| Role | Permissions | Notes |
|------|------------|-------|
| `admin` | Full CRUD | Internal only |
| `operator` | Read + limited write | Service accounts |
| `reader` | Read only | Audit tools |

> OpenAPI security scheme:
> All endpoints in `openapi.yaml` must declare the required security scheme and OAuth2 scopes.
> Example:
> ```yaml
> security:
>   - bearerAuth: [read:todo, write:todo]
> ```

---

## 8.3 Data Protection

| Data Class | At Rest | In Transit | Masking / Redaction |
|-----------|---------|-----------|---------------------|
| PII | AES-256 (envelope) | TLS 1.3 | Redacted from logs via `secretrewriter` |
| Credentials | Never stored | TLS 1.3 | Never logged; `secretrewriter` enforced |
| Business data | AES-256 | TLS 1.3 | — |

---

## 8.4 Supply-Chain Security

| Control | Tool | Enforcement |
|---------|------|------------|
| Dependency CVE scan | `forge scan security` | CI gate — blocks on High/Critical |
| Secret scanning | `forge scan secrets` | Pre-commit hook + CI |
| SBOM generation | TODO | Release pipeline |
| Image signing | Cosign | Release pipeline |

---

## 8.5 Compliance Requirements

| Standard | Requirement | Controls | Evidence |
|---------|------------|---------|---------|
| GDPR | Right to erasure | Soft-delete + data purge job | Deletion audit log |
| SOC 2 Type II | Access logging | Immutable audit trail | Log export |
| OWASP Top 10 | All controls | `forge scan security` in CI | CI artifact |

---

## 8.6 Security Testing Plan

| Test Type | Tool | When | Owner |
|-----------|------|------|-------|
| SAST | `forge scan security` | Every PR | CI |
| Secret scan | `forge scan secrets` | Pre-commit + CI | CI |
| Dependency scan | `go mod audit` | Every PR | CI |
| Penetration test | External | Quarterly | Security team |
| Auth boundary test | Go unit tests | Every PR | Feature team |

---

*Next section: [09-assessment.md](09-assessment.md)*
