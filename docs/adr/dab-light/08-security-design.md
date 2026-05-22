# Section 08 — Security Design (Light)

> **Template**: DAB Light
> All API security controls must be declared in `openapi.yaml` (security schemes + scopes).

---

## Top Threats (STRIDE)

| Threat | Mitigation |
|--------|-----------|
| Token forgery | Validate JWT at gateway; short TTL |
| Injection | Parameterised queries; ORM enforced |
| DoS | Rate limiting at API gateway |
| Privilege escalation | Least-privilege RBAC; deny-by-default |

---

## Auth/Authz

- Authentication: Bearer JWT (OAuth2)
- Authorisation: Role-based (`admin`, `operator`, `reader`)
- Security scheme declared in `openapi.yaml`

---

## Data Protection

- PII fields: AES-256 at rest, TLS 1.3 in transit, redacted from logs via `secretrewriter`
- No secrets in code (enforced by `forge scan secrets`)

---

## Security Gates (CI)

- [ ] `forge scan security` — blocks on High/Critical CVEs
- [ ] `forge scan secrets` — blocks on any secret leak
- [ ] Auth boundary unit tests

---

*Next: [09-assessment.md](09-assessment.md)*
