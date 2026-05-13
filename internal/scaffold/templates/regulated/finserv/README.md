# Financial Services — Forge Service Template

> **Regulated-industry scaffold** — use with `forge new --template regulated/finserv`

This template provides controls for services operating under common financial
services regulations (PCI-DSS, FFIEC, SOX IT controls, FCA/MiFID II audit
trail requirements).

---

## Controls included

| Control | Regulation | Mechanism |
|---------|-----------|-----------|
| Cardholder data environment | PCI-DSS §3 | Scope annotation in `config/pci-scope.yaml` |
| Audit trail | SOX IT / MiFID II | Forge audit ledger + immutable log forwarding |
| Separation of duties | PCI-DSS §6.4 | Two-key enforcement for production changes (ADR-022) |
| Vulnerability management | PCI-DSS §6.3 | `forge scan security` in CI; 30-day patch SLA |
| Cryptography | PCI-DSS §4 | TLS 1.2+ minimum; no RC4/DES/SHA1 |
| Change management | SOX ITGC | Mandatory PR + change-ticket link; `forge ship` CI gate |
| Incident response | FFIEC | `forge incident` + `docs/INCIDENT_RESPONSE.md` |

---

## Files generated

```
.
├── config/
│   ├── pci-scope.yaml           # Cardholder data environment boundary
│   ├── tls-policy.yaml          # PCI-DSS compliant TLS/cipher policy
│   └── network-segmentation.yaml # CDE network isolation
├── docs/
│   ├── INCIDENT_RESPONSE.md     # FFIEC incident response plan
│   ├── CHANGE_MANAGEMENT.md     # SOX change-management policy
│   ├── VENDOR_ASSESSMENT.md     # Third-party risk register
│   └── AUDIT_EVIDENCE.md        # Evidence collection guide for auditors
├── .forge/
│   └── scan-rules.toml          # PCI-specific scanner rules
└── README.md                    # This file
```

---

## Two-key enforcement

This template enables ADR-022 two-key enforcement for production deployments.
Any `forge deploy --env prod` requires approval from a second authorised key:

```bash
# Operator 1 initiates:
forge deploy --env prod --request-approval

# Operator 2 approves:
forge twokey approve <request-id>

# Operator 1 executes (now approved):
forge deploy --env prod --approve <request-id>
```

---

## CI gates added

- `forge scan security` — OWASP Top 10 + PCI-DSS specific rules
- `forge scan secrets` — no PANs or credentials in code
- `forge audit verify` — ledger integrity (required for SOX evidence)
- Change ticket link validation (blocks merges without `JIRA-XXXX` or `CHG-XXXX` in PR body)

---

## Usage

```bash
forge new --template regulated/finserv my-payment-service
cd my-payment-service
forge doctor
```
