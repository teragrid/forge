# SOC 2 Type II — Forge Service Template

> **Regulated-industry scaffold** — use with `forge new --template regulated/soc2`

This template includes the baseline controls, annotations, and CI gates required
to support a SOC 2 Type II audit for a Forge-managed service.

---

## Controls included

| Control | Mechanism |
|---------|-----------|
| CC6.1 — Logical access | RBAC annotations in `config/rbac.yaml` |
| CC6.6 — Transmission security | TLS-only networking enforced in `config/network-policy.yaml` |
| CC7.1 — Monitoring | `forge scan security` + SIEM forwarding in `config/log-forwarder.yaml` |
| CC8.1 — Change management | Mandatory PR review gate; `forge ship` requires passing CI |
| A1.1 — Availability | Health checks in `config/healthcheck.yaml`; SLA in `docs/SLA.md` |
| P6.1 — Personal data | `forge scan secrets` blocks PII in code; `docs/DATA_MAP.md` required |

---

## Files generated

```
.
├── config/
│   ├── rbac.yaml                # Role-based access control annotations
│   ├── network-policy.yaml      # Deny-all egress except allow-listed endpoints
│   ├── log-forwarder.yaml       # SIEM log forwarding config
│   └── healthcheck.yaml         # Liveness + readiness probes
├── docs/
│   ├── SLA.md                   # Service Level Agreement
│   ├── DATA_MAP.md              # Personal data inventory
│   └── VENDOR_ASSESSMENT.md     # Third-party vendor risk register
├── .forge/
│   └── scan-rules.toml          # Force-enables secrets + supply-chain scanners
└── README.md                    # This file
```

---

## CI gates added

The following gates are added to `.github/workflows/ci.yml`:

- `forge scan security` — OWASP Top 10 + supply-chain
- `forge scan secrets` — blocks PII and credential leakage
- `forge audit verify` — verifies audit ledger integrity
- `go test -race ./...` — data-race detection required for CC6

---

## Usage

```bash
forge new --template regulated/soc2 my-service
cd my-service
forge doctor
```

Review the generated `docs/DATA_MAP.md` and fill in your data processing inventory
before your first audit evidence collection period.
