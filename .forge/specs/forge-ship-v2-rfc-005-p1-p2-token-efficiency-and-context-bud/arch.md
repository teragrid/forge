```markdown
# Architecture Decision Record: Forge Ship V2 Token-Efficiency and Context-Budget Improvements

## 1. Component Topology
The Forge Ship V2 pipeline has the following components:
- **Input Processor:** Pre-tokenizes and preprocesses user input.
- **Context Manager:** Manages token pruning, prioritization of high-value context, and boundary handling.
- **Model Runner:** Handles encoding, attention mechanisms, and decoding via Transformer-based architecture.
- **Output Processor:** Post-processes responses, including token-optimization adaptations.

### Boundaries:
- **Input Processor ↔ Context Manager:** Passes tokenized input with metadata for contextual prioritization.
- **Context Manager ↔ Model Runner:** Exchanges context-prioritized input token lists.
- **Model Runner ↔ Output Processor:** Transmits raw model-generated output for semantic validation and token efficiency reconciliation.

### Relationships:
- Input Processor → Context Manager → Model Runner → Output Processor → End User

## 2. API Contracts
All enhancements will interact with client systems through a standard RESTful interface.

- API Style: **Resource-oriented paths (REST)**
- Referenced Contract: See OpenAPI spec appended below.

Primary APIs:
- `POST /api/v1/token-efficiency`: Optimizes token usage for provided input contexts.
- `POST /api/v1/context-budget`: Prioritizes and trims context to maximize contextual relevance and utilization within token limits.

## 3. Data Model & Consistency
### Data Entities:
- **TokenMetadata:** Encodes token properties such as importance, source, and sequence context.
- **ContextState:** Tracks current context composition and trims prioritized portions.
- **ProcessingMetrics:** Logs efficiency statistics, e.g., reductions achieved and latency impacts.

### Migration Strategy:
No structural database migrations are needed. Any new metadata (e.g., context prioritization logs) will be stored in existing operational event pipelines.

### Consistency Model:
Eventual consistency is sufficient for metrics aggregation. Critical path (context and token reduction) operates in strongly consistent mode to ensure processing integrity.

## 4. Non-Functional Requirements
- **p99 Latency:** ≤ 5% increase over baseline latencies for high-complexity inputs.
- **Throughput:** Sustained processing at scale for 50,000 context requests/hour with linear scaling.
- **Availability:** ≥ 99.95%.

## 5. Security Threat Model
### STRIDE Threats:
1. **Spoofing:** Secure token handling through strict validation of input integrity.
2. **Tampering:** Context-manager operations restricted by role-permission boundaries.
3. **Repudiation:** Log key operations (context prioritization, token reductions) with traceable request IDs.
4. **Information Disclosure:** Ensure context trimming doesn’t inadvertently retain sensitive user data.
5. **Denial of Service:** Rate limits and backpressure mechanisms on API endpoints.
6. **Elevation of Privileges:** Authenticate all traffic using JWTs scoped to user roles.

### Mitigations:
- **Authentication:** Supabase anon/service-role JWT tokens.
- **Authorization:** Enforce resource-scoped access control at API and context-management layers.
- **Transport Security:** Enforce HTTPS.

## 6. Deployment & Observability
### Deployment Topology:
- Horizontal scaling for all pipeline components, with autoscaling policies based on token complexity metrics.

### Observability:
- **Health Checks:** Liveness and readiness endpoints for each service.
- **Metrics:** Include token inefficiency rates, context prioritization accuracy, and processing latency.
- **Tracing:** Use OpenTelemetry to trace requests through each pipeline stage.
- **Disaster Recovery Plan:** RPO < 5 minutes, RTO < 30 minutes, cross-region deployments.

---

## ADR Summary
- **Status:** Accepted.
- **Context:** Improvements to token-efficiency and context-budget management for scaling Forge Ship V2.
- **Decision:** Introduce token-reduction heuristics and dynamic context prioritization logic. Implement interfaces for runtime operation with backward compatibility.
- **Consequences:** Achieves measurable cost and performance benefits but requires API consumers to validate against new contracts and metrics.

> See [openapi.yaml](openapi.yaml) for the full API contract.
