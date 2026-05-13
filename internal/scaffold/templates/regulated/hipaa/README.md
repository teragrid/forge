# HIPAA — Forge Service Template

> **Regulated-industry scaffold** — use with `forge new --template regulated/hipaa`

This template provides the baseline technical safeguards (§164.312) and
administrative controls required for HIPAA-compliant services built with Forge.

---

## Safeguards included

| Safeguard | Regulation ref | Mechanism |
|-----------|---------------|-----------|
| Access control | §164.312(a)(1) | RBAC in `config/rbac.yaml`; unique user ID enforcement |
| Audit controls | §164.312(b) | Forge audit ledger (append-only, hash-chained) |
| Integrity | §164.312(c)(1) | Checksums on PHI writes; `forge scan integrity` |
| Transmission security | §164.312(e)(1) | TLS 1.3 minimum; FIPS 140-2 cipher list |
| Encryption at rest | §164.312(a)(2)(iv) | AES-256 requirement documented in `config/encryption.yaml` |

---

## Files generated

```
.
├── config/
│   ├── rbac.yaml                # Unique-user-ID + minimum-necessary access
│   ├── encryption.yaml          # Encryption-at-rest requirements
│   └── tls-policy.yaml          # TLS 1.3 minimum; FIPS cipher list
├── docs/
│   ├── BAA_TEMPLATE.md          # Business Associate Agreement template
│   ├── PHI_INVENTORY.md         # Protected Health Information data map
│   ├── INCIDENT_RESPONSE.md     # HIPAA breach notification procedure
│   └── RISK_ASSESSMENT.md       # Annual risk assessment template
├── .forge/
│   └── scan-rules.toml          # Enables phi-detector scanner (M3 plugin)
└── README.md                    # This file
```

---

## Mandatory actions before go-live

1. Complete `docs/PHI_INVENTORY.md` — identify every field that may contain PHI.
2. Execute the BAA with each sub-processor (see `docs/BAA_TEMPLATE.md`).
3. Complete `docs/RISK_ASSESSMENT.md` annually.
4. Run `forge scan all` and ensure zero critical findings.
5. Verify audit ledger with `forge audit verify`.

---

## CI gates added

- `forge scan secrets` — no credentials or PHI in code
- `forge scan security` — OWASP + dependency audit
- `forge audit verify` — ledger integrity check

---

## Usage

```bash
forge new --template regulated/hipaa my-phi-service
cd my-phi-service
forge doctor
```
