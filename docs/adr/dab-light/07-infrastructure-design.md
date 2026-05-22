# Section 07 — Infrastructure Design (Light)

> **Template**: DAB Light

---

## Deployment

| Component | Platform | Replicas | Autoscaling |
|-----------|---------|---------|-------------|
| Service | Kubernetes | Min 2 | CPU > 70% |
| DB | RDS PostgreSQL 15 | 1 writer + 1 reader | Manual |

---

## IaC

- [ ] Terraform/Pulumi module created or updated
- [ ] No manual console changes

---

## Observability

| Signal | Name | Alert |
|--------|------|-------|
| Histogram | `request_duration_seconds` | p99 > 500 ms |
| Counter | `requests_total` | — |

---

## DR Summary

| Scenario | RTO | RPO |
|----------|-----|-----|
| AZ failure | 5 min | 0 min |
| Data corruption | 4 h | 1 h |

---

*Next: [08-security-design.md](08-security-design.md)*
