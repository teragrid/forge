# Section 07 — Infrastructure Design

> **Template**: DAB Full
> Define compute, storage, networking, IaC, scaling, and disaster recovery.

---

## 7.1 Deployment Topology

| Component | Platform | Region(s) | Replicas | Autoscaling |
|-----------|---------|-----------|---------|-------------|
| API Service | Kubernetes | us-east-1, eu-west-1 | Min 2, Max 10 | CPU > 70% |
| Worker | Kubernetes | us-east-1 | Min 1, Max 5 | Queue depth |
| Database | RDS PostgreSQL 15 | us-east-1 (primary) | 1 writer + 2 readers | Manual |
| Cache | ElastiCache Redis 7 | us-east-1 | Cluster mode | Manual |

---

## 7.2 Container Specification

```yaml
# Example — replace with actual values
resources:
  requests:
    cpu: "100m"
    memory: "128Mi"
  limits:
    cpu: "500m"
    memory: "512Mi"
readinessProbe:
  httpGet:
    path: /healthz/ready
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 10
livenessProbe:
  httpGet:
    path: /healthz/live
    port: 8080
  initialDelaySeconds: 15
  periodSeconds: 20
```

---

## 7.3 Networking & Security Groups

| Rule | Protocol | Port | Source | Destination | Justification |
|------|---------|------|--------|------------|---------------|
| Ingress | HTTPS | 443 | Internet | API Gateway | Public API |
| Internal | gRPC | 9090 | VPC | Services | Service mesh |
| DB | TCP | 5432 | Services subnet | RDS | DB access |

---

## 7.4 Infrastructure as Code

> All infrastructure must be declared in IaC — no manual console changes.

| Resource | IaC Tool | Module / File | Owner |
|----------|---------|--------------|-------|
| TODO | Terraform | `infra/modules/todo/` | Platform |

---

## 7.5 CI/CD Pipeline

| Stage | Tool | Trigger | Gate |
|-------|------|---------|------|
| Test | GitHub Actions | PR | All tests green |
| Security scan | `forge scan security` | PR | No High/Critical findings |
| Container build | Docker | Merge to main | Linting pass |
| Staging deploy | ArgoCD | Merge to main | Smoke tests pass |
| Production deploy | ArgoCD | Release tag | Manual approval |

---

## 7.6 Observability

| Signal | Name | Description | Alert Threshold |
|--------|------|-------------|----------------|
| Counter | `requests_total{service,path,status}` | Total HTTP requests | — |
| Histogram | `request_duration_seconds` | p99 latency per path | p99 > 500 ms |
| Gauge | `queue_depth` | Worker queue depth | > 1000 |
| Log | `audit.access` | Auth events | Any 4xx/5xx burst |
| Trace | `http.server.request` | Distributed trace span | — |

---

## 7.7 Disaster Recovery

| Scenario | RTO | RPO | Trigger | Owner |
|----------|-----|-----|---------|-------|
| AZ failure | 5 min | 0 min | Automatic (k8s reschedule) | Platform |
| Region failure | 30 min | 5 min | Manual DNS failover | On-call |
| Data corruption | 4 h | 1 h | Restore from latest snapshot | DBA |

### Failover runbook

1. TODO — detection step
2. TODO — escalation step
3. TODO — failover action
4. TODO — validation step
5. TODO — post-mortem

---

## 7.8 Cost Estimate

| Resource | Size | Unit Cost | Monthly |
|----------|------|-----------|---------|
| TODO | TODO | TODO | $TODO |
| **Total** | | | **$TODO** |

---

*Next section: [08-security-design.md](08-security-design.md)*
