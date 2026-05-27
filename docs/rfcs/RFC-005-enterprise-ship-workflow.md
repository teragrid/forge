# RFC-005 — Enterprise Ship Workflow v2

**Status**: Draft — For Discussion  
**Author**: Forge Core Team  
**Date**: 2026-05-27  
**Relates to**: ADR-001, ADR-002, ADR-009, ADR-014, ADR-024  

---

## 1. Executive Summary

`forge ship` hiện tại là một pipeline 7-checkpoint tuần tự, chạy tốt cho dự án vừa và nhỏ, nhưng còn khoảng cách đáng kể so với yêu cầu của sản phẩm enterprise-grade. RFC này audit trạng thái hiện tại, định nghĩa các yêu cầu enterprise còn thiếu, và đề xuất kiến trúc cải tiến cho **Ship v2** — một workflow có khả năng:

- **Chạy song song** các checkpoint độc lập để giảm latency
- **Tối ưu token-turn** qua dynamic context budgeting và KB tiering
- **Tự học liên tục** từ dữ liệu thực tế của mỗi team/dự án
- **Customize sâu** theo từng hệ thống, nghiệp vụ, hoặc compliance domain
- **Audit trail đầy đủ** đáp ứng SOC-2, ISO-27001, FedRAMP

---

## 2. Audit: Trạng Thái Hiện Tại

### 2.1 Điểm Mạnh

| Thành phần | Đánh giá |
|---|---|
| 7-checkpoint sequential pipeline | Rõ ràng, dễ debug |
| Multi-role debate system (8 + 6 roles) | Giảm blind-spot tốt |
| TDD enforcement (M1-03 guard) | Chống code-before-test |
| Spec-vs-code gap audit (TG-39) | Phát hiện drift sớm |
| Learning loop (G-011/G-015) | Nền tảng self-improvement đã có |
| KB scoring với tag overlap | Knowledge retrieval contextual |
| Steering system | Customizable behavior |
| Dry-run mode | Safe exploration |

### 2.2 Gaps Nghiêm Trọng — Enterprise Perspective

#### G1: Token Economics Chưa Tối Ưu

```
Hiện tại:
  spec=1,500 + arch=4,000 + test=2,500 + breakdown=1,500 + code=2,000 + ship=2,000 + qa=5,000
  → 19,000 tokens mỗi feature (baseline, không tính debate rounds)
  → Với multi-role debate (3 rounds × 6 roles × arch): +18,000 tokens
  → Tổng thực tế: ~37,000 tokens/feature

Vấn đề:
  - Budget cứng per-checkpoint, không adaptive theo độ phức tạp
  - Không có context compression giữa checkpoints
  - KB top-5 entries cố định, không weighted theo model context window
  - No caching của unchanged artefacts
  - Re-debate full từ đầu khi resume
```

#### G2: Sequential Pipeline = Latency Bottleneck

```
Hiện tại: Spec → Arch → Test → Breakdown → Code → Ship → QA-Verify
  Mỗi checkpoint chờ checkpoint trước hoàn thành 100%

Enterprise use case:
  - "arch" và "test design" có thể chạy song song sau khi spec hoàn thành
  - "breakdown" phụ thuộc arch nhưng không phụ thuộc test
  - "code review hooks" có thể chạy async ngay khi code thay đổi

Ước tính latency giảm nếu parallel: -35~40% trên full pipeline
```

#### G3: Learning Loop Còn Shallow

```
Hiện tại:
  - Failure records: chỉ lưu free-text error messages
  - Pattern extraction: chạy 1 lần post-success, không incremental
  - Scope: per-project only (.forge/learned/)
  - No aggregation across features hay teams
  - KB updates require manual re-index (forge scan)
  - Learned patterns không có confidence score hay decay

Enterprise needs:
  - Cross-project pattern library (org-level KB)
  - Failure clustering (nhóm lỗi tương tự → root cause)
  - A/B testing: so sánh 2 steering variants trên cùng loại feature
  - Temporal relevance: patterns cũ giảm trọng số
  - Team velocity metrics: checkpoint success rate, avg retry count
```

#### G4: Customization Còn Hạn Chế

```
Hiện tại:
  - Steerings: inject vào system prompt, max 300 tokens, project-local
  - Hooks: enable/disable by name, no conditional logic
  - Roles: hardcoded 8 general + 6 arch-specific
  - Checkpoints: không thể add, remove, hay reorder
  - Không có domain-specific checkpoint (e.g., "compliance", "migration")

Enterprise needs:
  - Banking/Fintech: thêm checkpoint "RegTech-Review" (PCI-DSS, Basel III)
  - Healthcare: thêm checkpoint "HIPAA-Gate" (PHI handling, audit trail)
  - SaaS: thêm checkpoint "Tenant-Isolation-Gate" (multi-tenancy check)
  - Custom roles: thêm "InfoSec-CISO", "Data-Privacy-Officer", "SRE-Lead"
  - Conditional pipeline: nếu spec có `data_migration: true` → thêm "DB-Migration-Gate"
```

#### G5: Approval Gates Chưa Production-Ready

```
Hiện tại:
  - Gate: chỉ y/N interactive prompt hoặc --yolo (bỏ qua tất cả)
  - Không có multi-approver workflow
  - Không có time-limited approval (expire sau X giờ)
  - Không có async approval (Slack/webhook notification)
  - Không audit trail của approvals

Enterprise needs:
  - 4-eyes principle: code + security phải được 2 người approve
  - CISO approval gate khi có high-risk changes
  - Slack/Teams notification → approve directly from chat
  - Approval expiry + re-validation nếu code thay đổi sau approval
  - Full audit trail (who approved, when, with what context)
```

#### G6: No Rollback Integration

```
Hiện tại:
  - forge undo tồn tại (ADR-024) nhưng KHÔNG được wired vào ship pipeline
  - Nếu checkpoint thất bại giữa chừng: filesystem ở trạng thái partial
  - Không có transactional checkpoint (all-or-nothing)
  - Không có snapshot trước khi pipeline bắt đầu

Enterprise needs:
  - Snapshot state trước mỗi checkpoint
  - Auto-rollback khi checkpoint thất bại (configurable)
  - Checkpoint khả năng idempotent replay
```

#### G7: Observability Còn Yếu

```
Hiện tại:
  - token-ledger.jsonl: lưu per-call token usage
  - audit.jsonl: lưu verb action history
  - ShipEvent NDJSON: chỉ có khi --json --yolo
  - Không có structured metrics (latency, success rate, cost per feature)
  - Không có distributed tracing (trace_id xuyên suốt pipeline)
  - Không có alerting khi pipeline thất bại

Enterprise needs:
  - OpenTelemetry traces (span per checkpoint, per LLM call)
  - Prometheus metrics: forge_ship_duration_seconds{checkpoint="arch"}
  - Structured logs với trace_id, span_id, feature_slug
  - Cost per feature tracking (model + provider)
  - SLA dashboard: % features shipped < 30 min
```

#### G8: Security & Compliance Gaps

```
Hiện tại:
  - SecretRewriter scrubs creds trước LLM call
  - forge scan security (gitleaks-based)
  - Không có data residency enforcement (prompt có thể contain PII)
  - Không có LLM provider failover với compliance constraints
  - Không audit trail immutability (audit.jsonl có thể bị tamper)

Enterprise needs:
  - PII detection trước khi prompt gửi đến LLM
  - Data residency: "EU data → EU-hosted LLM only"
  - Immutable audit trail (append-only, signed, offloaded)
  - LLM provider policy: "no training on our data" contracts verified
  - SOC-2 Type II: continuous compliance monitoring
```

#### G9: Spec Không Có Workspace Context

```
Vấn đề cốt lõi:
  Khi `forge ship spec "add invoice export"` chạy, LLM chỉ nhận:
    - Feature description (user input)
    - Recent spec failures (loadRecentFailures)
    - KB entries (InvokeWithKnowledge, generic)

  LLM KHÔNG biết:
    - Tech stack của project (Go? Node? Python? Monorepo?)
    - Conventions của team (AGENTS.md, copilot-instructions.md)
    - Existing features đã được ship (tránh duplicate logic)
    - Recent changes (git log) — spec có thể conflict với work-in-progress
    - Existing interfaces/APIs — spec references interface chưa tồn tại
    - Existing test patterns — AC không match framework hiện tại

  Hệ quả:
    - Spec tạo ra "in a vacuum": không reference existing modules
    - Acceptance Criteria không match test framework thực tế (e.g., dùng pytest
      trong khi project dùng go test)
    - Duplicate feature specs (tạo "auth" spec khi đã có auth trong .forge/specs/)
    - Arch checkpoint phải "discover" lại context mà spec checkpoint đã bỏ qua
    - LLM hallucinate external dependencies không có trong go.mod / package.json

Context hiện tại chỉ đến từ KB entries (generic best practices),
không phải từ workspace cụ thể → spec phù hợp với "một project nào đó"
nhưng không chắc phù hợp với "project này".
```

---

## 3. Đề Xuất Kiến Trúc: Ship v2

### 3.1 Dependency-Aware Parallel Pipeline

```
Thay vì sequential, Ship v2 dùng DAG (Directed Acyclic Graph):

                    ┌──────────┐
                    │   Spec   │
                    └────┬─────┘
                         │ (spec.md + spec.yml ready)
              ┌──────────┴──────────┐
              ▼                     ▼
         ┌─────────┐          ┌──────────┐
         │  Arch   │          │   Test   │  ← parallel
         └────┬────┘          └─────┬────┘
              │                     │
              └──────────┬──────────┘
                         ▼ (arch.md + tests.md ready)
                   ┌───────────┐
                   │ Breakdown │
                   └─────┬─────┘
                         │
              ┌──────────┴──────────┐
              ▼                     ▼
          ┌──────┐           ┌───────────┐
          │ Code │           │  Custom   │  ← domain gates chạy song song
          └──┬───┘           │  Gates    │
             │               └─────┬─────┘
             └──────────┬──────────┘
                        ▼
                 ┌────────────┐
                 │    Ship    │
                 └─────┬──────┘
                       │
                 ┌─────▼──────┐
                 │ QA-Verify  │
                 └────────────┘

Execution model:
  - Mỗi checkpoint là một "task" với declared inputs/outputs
  - Scheduler chạy tasks theo topological order
  - Max parallelism configurable (default: 2, max: nCPU/2)
  - Timeout per-checkpoint (configurable, default: 10 min)
```

**Vấn đề cần thảo luận:**
- Conflict resolution khi 2 checkpoint song song cùng write `.forge/specs/<slug>/`?
- User approval gate trong parallel context: block toàn bộ pipeline hay chỉ downstream tasks?
- Resume semantics: khi resume sau failure trong parallel run, replay như thế nào?

---

### 3.2 Adaptive Token Budgeting

```
Thay vì budget cứng, Ship v2 dùng 3-tier dynamic budget:

Tier 1 — Complexity Score (0-100, tính từ spec):
  factors: LOC estimate, number of API endpoints, number of roles,
           has_data_migration, has_external_dependencies, risk_level
  
Tier 2 — Budget Multiplier:
  score 0-30  → 0.7× (simple feature: config change, UI tweak)
  score 31-60 → 1.0× (standard feature: new endpoint + CRUD)
  score 61-80 → 1.5× (complex: new service, schema migration)
  score 81-100 → 2.0× (epic: cross-service, compliance-sensitive)

Tier 3 — Per-Checkpoint Allocation:
  Base budgets × multiplier, capped at model's context window
  
  Example (score=75, multiplier=1.5×):
    spec: 1500 × 1.5 = 2,250
    arch: 4000 × 1.5 = 6,000
    test: 2500 × 1.5 = 3,750
    ...
    Total: ~28,500 (vs current flat 19,000)

Context Compression (mới):
  - Giữa checkpoints: compress prior artefacts → structured summary
  - Summary budget: max 500 tokens per prior checkpoint
  - Only include diffs từ lần review cuối (not full re-send)
  - KB entries: re-score mỗi checkpoint, không reuse prior selection

Caching (mới):
  - Cache LLM response nếu inputs identical (content hash)
  - Cache scope: per-project, 24h TTL
  - Invalidation: khi spec.yml thay đổi → invalidate all downstream
```

**Vấn đề cần thảo luận:**
- Complexity scorer có cần LLM call để tính không? (chicken-and-egg với token budget)
- Context compression trade-off: mất detail hay tốn thêm 1 LLM call để summarize?
- Cache invalidation granularity: per-checkpoint hay per-artefact?

---

### 3.3 Tiered Learning Architecture

```
3 tầng knowledge, mỗi tầng có scope và update frequency khác nhau:

Layer 3 (Forge Global KB)
  ─────────────────────────────────────────────────────
  - Curated by Forge team, released với binary
  - Encrypted, read-only trong production
  - Families: security, reliability, compliance, api-design, testing
  - Update cycle: per Forge release (~monthly)
  
Layer 2 (Org/Team KB)
  ─────────────────────────────────────────────────────
  - Stored tại org-level: git submodule hoặc internal registry
  - Entries contributed bởi senior engineers, auto-extracted từ patterns
  - Confidence score: 0-1 (dựa trên số feature dùng pattern này thành công)
  - Decay: confidence × 0.95 mỗi 30 ngày nếu không được dùng
  - Auto-promoted từ Layer 1 khi pattern xuất hiện ≥5 lần trong 90 ngày
  
Layer 1 (Project KB — current)
  ─────────────────────────────────────────────────────
  - .forge/learned/ (hiện tại)
  - Feature-specific, chạy post-success
  - NEW: incremental update, không cần full re-index
  - NEW: failure clustering (group similar failures → named anti-pattern)
  - NEW: confidence score per pattern (starts 0.5, grows với usage)

Selection Algorithm v2:
  score(entry) = base_tag_overlap
               + checkpoint_match × 3
               + confidence × 2      ← new: confidence-weighted
               - age_days × 0.01     ← new: temporal decay
               + layer_priority × 1  ← new: Layer 3 > 2 > 1 for cold start

A/B Testing Framework (mới):
  - Nếu có 2+ steering variants cho cùng checkpoint: 50/50 random split
  - Track: success rate, retry count, user approval rate per variant
  - Auto-promote winner sau 20 samples (p-value < 0.05)
  - Report: `forge insights ab-results`
```

**Vấn đề cần thảo luận:**
- Org KB storage: git submodule vs internal HTTP registry vs embedded trong forge config?
- Privacy: learned patterns có thể leak implementation details — cần anonymization?
- Confidence scoring: success = "user approved checkpoint" hay "full pipeline succeeded"?
- Auto-promotion từ Layer 1 → 2: cần human review trước khi promote?

---

### 3.4 Enterprise Customization Model

```
Ship v2 giới thiệu khái niệm "Domain Profile" — 1 file YAML định nghĩa
toàn bộ pipeline behavior cho 1 loại hệ thống cụ thể:

.forge/domains/banking.yml:
─────────────────────────────────────────────────────────────────
profile: banking
extends: default                    # kế thừa default pipeline
description: "Core banking system"

checkpoints:                        # override checkpoint config
  arch:
    budget_multiplier: 2.0          # arch luôn full budget
    extra_roles:
      - name: InfoSecCISO
        persona: "I am the CISO. Every change is a potential breach..."
        phase: [review, sign-off]
    steerings: [review-dab, pci-dss-controls, data-residency-eu]

extra_checkpoints:                  # add custom checkpoints
  - name: RegTech-Review
    after: arch                     # vị trí trong pipeline
    parallel_with: []               # không parallel
    prompt_template: .forge/prompts/regtech-review.md
    roles: [ComplianceOfficer, RegulatoryLead]
    blocking: true                  # fail = pipeline stops
    condition: "spec.yml has field 'regulatory_scope'"  # conditional
    
  - name: DB-Migration-Gate
    after: code
    condition: "tasks.md contains 'migration'"
    prompt_template: .forge/prompts/db-migration.md
    blocking: true

gates:
  require_approvers: 2              # 4-eyes principle
  approvers:                        # specific roles must approve
    - role: SecurityLead
      checkpoints: [arch, ship]
    - role: CISO  
      checkpoints: [ship]
  approval_expiry_hours: 8          # re-approval sau 8h
  notification:
    slack_webhook: ${SLACK_WEBHOOK_SECURITY}
    message_template: .forge/notifications/approval-request.md

quality_gates:
  strict: true
  extra_hooks:
    - name: pci-dss-check
      checkpoint: [arch, ship]
      script: .forge/hooks/pci-dss-check.sh
      on_failure: block

llm:
  provider_policy:
    allowed_providers: [azure-openai]   # chỉ dùng Azure (data residency)
    region: eastus                       # EU data → EU region
    no_training_guarantee: required      # require contract
  pii_detection:
    enabled: true
    action: redact                       # redact trước khi gửi LLM
    patterns: [vn-phone, vn-id, ccn]

audit:
  immutable: true
  backend: s3                            # offload to S3
  sign: true                             # HMAC-signed entries
  retention_days: 365                    # SOC-2 requirement
─────────────────────────────────────────────────────────────────

Các domain profile built-in đề xuất:
  - default     : hiện tại (baseline)
  - banking     : PCI-DSS, 4-eyes, data residency, RegTech checkpoint
  - healthcare  : HIPAA, PHI detection, audit immutability
  - saas-b2b    : multi-tenancy gate, tenant isolation check
  - data-heavy  : DB migration gate, schema evolution check
  - microservice: API contract gate, backward-compat check
```

**Vấn đề cần thảo luận:**
- `extends` inheritance model: deep merge hay shallow override? Khi conflict thì sao?
- Custom checkpoint scripts: có nên support `.sh` hay chỉ WASM plugins (theo ADR-002)?
- `condition` expression language: Golang template? CEL (Common Expression Language)?
- Built-in domain profiles: ship với binary hay community-contributed?

---

### 3.5 Enterprise Approval Workflow

```
Ship v2 thay thế y/N prompt bằng Approval Protocol:

┌──────────────────────────────────────────────────────┐
│                  Approval Protocol                    │
│                                                        │
│  1. Checkpoint completes, creates ApprovalRequest:    │
│     {checkpoint, artefacts[], required_approvers,     │
│      expires_at, context_summary, risk_level}         │
│                                                        │
│  2. Notify channels (configurable):                   │
│     - Terminal (default): rich diff + prompt          │
│     - Slack/Teams: card với Approve/Reject buttons    │
│     - GitHub PR comment: inline review               │
│     - Email: digest khi > N checkpoints pending       │
│                                                        │
│  3. Approval modes:                                    │
│     - sync  : pipeline blocks, chờ response           │
│     - async : pipeline suspends, resume khi approved  │
│     - auto  : --yolo flag (bypass, log only)          │
│                                                        │
│  4. Multi-approver:                                    │
│     - Collect approvals until quorum met               │
│     - Any rejection → auto-rollback to snapshot       │
│     - Tie-break: require explicit override by owner   │
│                                                        │
│  5. Audit record: {approver, timestamp, gate_version, │
│                    checkpoint_hash, decision, comment} │
└──────────────────────────────────────────────────────┘

API (for external integrations):
  forge ship approve <feature> --checkpoint arch --comment "LGTM"
  forge ship reject <feature> --checkpoint arch --reason "missing threat model"
  forge ship pending                    # list all awaiting approval
  forge ship approval-status <feature>  # full approval history
```

**Vấn đề cần thảo luận:**
- Async approval: state persistence ở đâu? File-based (.forge/) có scalable không khi team lớn?
- Slack integration: cần forge server process thường trực hay webhook-on-demand?
- Approval token: khi approve qua Slack, verify identity như thế nào?

---

### 3.6 Observability Stack

```
Ship v2 emit OpenTelemetry signals xuyên suốt pipeline:

Traces:
  forge.ship (root span)
  ├── forge.ship.checkpoint{name="spec"}
  │   ├── forge.ship.kb_lookup{checkpoint="spec"}
  │   ├── forge.llm.invoke{model="claude-3-5", tokens_in=1200}
  │   └── forge.ship.hook{name="spec-completeness-gate"}
  ├── forge.ship.checkpoint{name="arch"}  [parallel]
  ├── forge.ship.checkpoint{name="test"}  [parallel]
  └── ...

Metrics (Prometheus-compatible):
  forge_ship_duration_seconds{checkpoint, status, profile}
  forge_ship_llm_tokens_total{checkpoint, model, direction}
  forge_ship_llm_cost_usd{checkpoint, model, provider}
  forge_ship_kb_entries_used{checkpoint, layer}
  forge_ship_approval_wait_seconds{checkpoint, mode}
  forge_ship_remediation_rounds{checkpoint}
  forge_ship_features_total{status, profile, domain}

Events (structured log):
  {
    "trace_id": "abc123",
    "span_id": "def456",
    "event": "checkpoint.completed",
    "checkpoint": "arch",
    "status": "ok",
    "duration_ms": 8432,
    "tokens": {"in": 3800, "out": 1200},
    "cost_usd": 0.0124,
    "kb_entries": 3,
    "remediation_rounds": 0,
    "feature": "auth-mfa",
    "profile": "banking",
    "ts": "2026-05-27T10:32:00Z"
  }

Export targets (configurable):
  - stdout (default, --verbose)
  - OTLP gRPC endpoint (Jaeger, Grafana Tempo)
  - Prometheus push gateway
  - Datadog / New Relic / Honeycomb (via OTLP)
```

---

### 3.7 Rollback & Recovery Protocol

```
Ship v2 implements Transactional Checkpoints:

Before each checkpoint:
  1. Snapshot .forge/specs/<slug>/ → .forge/.snapshots/<slug>/<checkpoint>/
  2. Record git HEAD + working tree hash
  3. Create checkpoint_lock file (prevent concurrent runs)

On checkpoint failure:
  Option A (default): suspend, show diff, ask user
  Option B (auto-rollback): restore from snapshot, log failure
  Option C (partial-keep): keep generated artefacts, mark checkpoint as "needs-review"

Idempotency design:
  - Checkpoint output = deterministic function of inputs + spec hash
  - Re-running checkpoint with same inputs → same output (no new files)
  - Re-running after input change → re-generate only changed artefacts

forge undo integration:
  forge ship undo arch          # rollback arch checkpoint
  forge ship undo --all         # full pipeline rollback
  forge ship undo --to spec     # rollback to post-spec state
```

---

### 3.8 Workspace Context Collection — Pre-Spec Phase

#### Vấn đề

Spec checkpoint hiện tại gửi LLM một feature description và KB entries generic.
LLM không biết tech stack, team conventions, existing features, hay recent changes
của project cụ thể. Kết quả: spec "hợp lệ về mặt lý thuyết" nhưng không phù hợp
với hệ thống đang tồn tại.

#### Giải pháp

Thêm phase **`spec/workspace-context`** chạy TRƯỚC `spec/intake` — hoàn toàn
deterministic, zero LLM tokens, zero latency overhead đáng kể.

```
Phase order trong Spec checkpoint (Ship v2):

  TRƯỚC:
    spec/intake            (LLM: BA + PO)
    spec/impact-analysis   (LLM: SA + CPO)
    spec/completeness-gate (deterministic)

  SAU:
    spec/workspace-context (deterministic ← NEW, chạy TRƯỚC intake)
    spec/intake            (LLM: BA + PO — giờ có workspace context)
    spec/impact-analysis   (LLM: SA + CPO)
    spec/completeness-gate (deterministic)
```

#### Workspace Context Collection Logic

```go
// workspace_context.go
// collectWorkspaceContext scans root deterministically and writes
// .forge/specs/<slug>/workspace-context.md (zero LLM calls).

type WorkspaceContextResult struct {
    SnapshotPath string  // .forge/specs/<slug>/workspace-context.md
    Content      string  // markdown content (capped ~600 tokens)
    TechStack    []string
    HasGit       bool
}

func collectWorkspaceContext(root, slug string) WorkspaceContextResult {
    // 1. Tech stack: detect go.mod / package.json / requirements.txt / pom.xml...
    // 2. Top-level dirs (excluding hidden): cmd/, internal/, docs/, tests/...
    // 3. Recent git log (--oneline -n 10): recent changes context
    // 4. Existing specs: ls .forge/specs/ → avoid duplicate features
    // 5. Conventions: read AGENTS.md (first 500 chars) or copilot-instructions.md
    // Output cap: ~600 tokens max — đủ để orient LLM, không đủ để blow context
}
```

**Tech stack indicators:**

| File detected | Stack label |
|---|---|
| `go.mod` | Go module (includes module path + Go version) |
| `package.json` | Node.js (includes name + scripts.test) |
| `requirements.txt` / `pyproject.toml` | Python |
| `Cargo.toml` | Rust |
| `pom.xml` / `build.gradle` | Java (Maven / Gradle) |
| `Dockerfile` | Docker |
| `.github/` | GitHub Actions CI |
| `go.work` | Go workspace (monorepo) |

**Output: `.forge/specs/<slug>/workspace-context.md`**

```markdown
# Workspace Context Snapshot
Generated: 2026-05-27T10:00:00Z

## Tech Stack
- Go module (go 1.26.3, module: github.com/teragrid/forge)
- Makefile
- Docker
- GitHub Actions CI

## Project Structure
- cmd/, docs/, forge-knowledge/, internal/, packages/, scripts/, tests/

## Recent Changes (last 10 commits)
```
6854f93 test: verify pre-push hook works end-to-end
0ae9054 fix: unset GIT_DIR in forge-qa-real.sh
6d3f4d6 fix: pre-push hook CRLF and minimal init
```

## Existing Feature Specs (avoid duplicates)
- auth-mfa, invoice-service, langchain-agents-template

## Project Conventions (AGENTS.md)
Module path: github.com/teragrid/forge. CGO disabled. No os.Exit except main.
Tests in <package>/<file>_test.go. No third-party test frameworks.
All subprocess via internal/procspawn. Secrets via internal/secretrewriter.
[truncated at 500 chars]
```

#### Injection vào Spec Generation Prompts

```
Trước (user prompt cho spec generation):
  "Generate a complete feature specification for: add invoice PDF export"

Sau:
  "Generate a complete feature specification for: add invoice PDF export

  ## Workspace Context
  [workspace-context.md content]"
```

LLM bây giờ biết: đây là Go project, có existing invoice-service spec,
conventions yêu cầu go test (không pytest), không có CGO, recent changes
liên quan đến pre-push hooks.

#### Token Cost

```
Phase overhead:
  spec/workspace-context execution: ~0 tokens (deterministic)
  Injected into spec/intake user prompt: +600 tokens input
  Cost at claude-3-5-haiku:  600 × $0.80/1M = $0.00048 extra per feature
  Cost at claude-3-5-sonnet: 600 × $3.00/1M = $0.0018 extra per feature

ROI:
  Marginal cost: <$0.002 per feature
  Benefit: spec tham chiếu đúng tech stack, framework, và conventions →
           giảm arch checkpoint "discovery overhead" (~500-800 tokens) →
           net neutral or net positive on token burn
```

#### Implementation Files

- `internal/cli/cmdship/workspace_context.go` — `collectWorkspaceContext(root, slug)`
- `internal/cli/cmdship/subworkflow.go` — thêm `spec/workspace-context` phase
- `internal/cli/cmdship/ship.go` — gọi `collectWorkspaceContext` trong `checkSpec`,
  inject vào tất cả user prompts của spec generation và review calls

---

## 4. Token Burn Optimization

> Token burn = tổng tokens input + output tiêu thụ thực tế, tính bằng đô la.  
> Đây là chi phí trực tiếp và latency gián tiếp của mỗi feature được ship.

### 4.1 Anatomy: Token Burn Hiện Tại

Mỗi LLM call trong pipeline được build bởi hai hàm: `BuildSystemWithSteerings`
+ `InvokeWithKnowledge`. Bổ sung qua phân tích code thực tế:

```
┌─────────────────────────────────────────────────────────────────┐
│  SYSTEM PROMPT = applySteerings(baseSystem, steerings[≤3])      │
│  ├─ 3 steerings × ≤300 tokens                  ≈   750 tokens   │
│  ├─ base system (role + core instruction)       ≈   400 tokens   │
│  └─ KB entries (AppendDocsBudgeted, top-5)      ≈   800 tokens   │
│                                           FIXED ≈ 1,950 tokens   │
├─────────────────────────────────────────────────────────────────┤
│  USER PROMPT = description + prior artefacts (dynamic)          │
│  ├─ spec      :  800 tokens avg (spec.md)                       │
│  ├─ arch      : 1,500 tokens avg (arch.md + openapi.yaml)       │
│  ├─ tests     : 1,000 tokens avg (tests.md)                     │
│  ├─ breakdown :   800 tokens avg (breakdown.md + tasks.md)      │
│  └─ code-plan :   700 tokens avg (code-plan.md)                 │
│                                                                  │
│  Checkpoint nhận context CỘNG DỒN: mỗi checkpoint sau nhận     │
│  nhiều prior artefacts hơn checkpoint trước.                    │
└─────────────────────────────────────────────────────────────────┘
```

**Model pricing từ `estimateCost()` (USD / 1M tokens):**

| Model | Input | Output | Ratio out/in |
|---|---|---|---|
| `claude-3-5-sonnet-20241022` | $3.00 | $15.00 | 5× |
| `claude-3-5-haiku-20241022` | $0.80 | $4.00  | 5× |
| `gpt-4o`                    | $2.50 | $10.00 | 4× |
| `gpt-4o-mini`               | $0.15 | $0.60  | 4× |

Output tokens đắt hơn input 4-5×. Mọi token output lãng phí (preamble, hedge,
kết luận) tốn nhiều hơn token input lãng phí.

**Breakdown token burn per checkpoint (--yolo mode, claude-3-5-sonnet, happy path):**

```
Checkpoint   Calls   Input/call   Output/call   Total tokens   Cost
──────────────────────────────────────────────────────────────────
Spec             1    2,250         1,500          3,750        $0.03
Arch — primary   1    3,250         4,000          7,250        $0.08
Arch — debate   18    4,350*          600         88,200*       $1.58*
Test             2    4,750         2,500         14,500        $0.17
Breakdown        1    5,050         1,500          6,550        $0.07
Code             1    6,250         2,000          8,250        $0.11
Ship             1    8,100         2,000         10,100        $0.12
  + remediation ×3   9,650         1,500         33,450        $0.36
QA-Verify        1   10,100         5,000         15,100        $0.20
  + remediation ×2  11,650         2,000         27,300        $0.29
──────────────────────────────────────────────────────────────────
TOTAL (worst)        —             —            214,450        $3.41
TOTAL (best†)        —             —             49,700        $0.86
```

`*` Mỗi trong 18 debate call gửi lại: 1,950 (fixed overhead) + artefacts (~1,600) +
debate history (~800 growing) = 4,350 avg. Arch debate một mình chiếm **41%** tổng cost.

`†` Best: không debate, không remediation, chỉ 10 primary calls.

**Kết luận:** 3 thành phần tốn tokens nhất theo thứ tự:
1. **Arch debate sub-calls** — 18 calls, mỗi call mang full system overhead (steerings + KB) không cần thiết
2. **Prior artefact context cộng dồn** — mỗi checkpoint sau gửi ngày càng nhiều artefact cũ
3. **Remediation loops** — mỗi round gửi lại toàn bộ context để sửa 1 gap nhỏ

---

### 4.2 Lever 1: Stripped Overhead Cho Debate Sub-calls

**Vấn đề cốt lõi:** `BuildSystemWithSteerings` + `InvokeWithKnowledge` được gọi
như nhau cho cả primary call lẫn mọi debate round call. Nhưng:

- **Primary call** (generate artefact): cần steerings (quality standards) + KB (domain knowledge)
- **Debate round call** (role reviews artefact): chỉ cần persona + artefact content

Steerings + KB đang bị inject vào debate calls mà không có giá trị gì cho
việc "một role đọc ADR và đưa ra concern".

```
Before (mỗi debate call):
  system = 3 steerings (750) + base system (400) + KB entries (800) + persona (150)
         = 2,100 tokens cố định
  user   = spec (800) + arch draft (1,500) + prior concerns (800 growing)
         = 3,100 tokens
  Total input/call = 5,200 tokens

After (debate call stripped):
  system = persona (150) + debate-instruction (100)
         = 250 tokens
  user   = artefact to review (arch draft 1,500) + prior concerns (800)
         = 2,300 tokens  
  Total input/call = 2,550 tokens

Savings per debate call: 5,200 → 2,550 = -2,650 tokens (51% reduction)
Savings across 18 arch debate calls: 47,700 tokens ≈ $0.14
```

**Implementation sketch:**

```go
// LLMPipe cần thêm method chuyên dụng cho debate rounds
func (p *LLMPipe) InvokeDebateRound(
    operation, model string,
    persona string,        // chỉ persona, KHÔNG có steerings/KB
    artefact string,       // artefact cần review
    priorConcerns string,  // concerns từ round trước
    maxTokens int,
) (string, error) {
    system := fmt.Sprintf(debateRoundSystemTemplate, persona)
    user   := buildDebateUserPrompt(artefact, priorConcerns)
    return p.Invoke(operation, model, system, user, maxTokens)
    // NOTE: không gọi InvokeWithKnowledge → không load KB
    // NOTE: không gọi BuildSystemWithSteerings → không inject steerings
}
```

**Risk:** Debate quality có thể giảm nhẹ nếu role không có KB context? Cần A/B test.
Counter-argument: một human reviewer đọc ADR cũng không cần toàn bộ KB — họ chỉ cần
artefact và persona của mình.

---

### 4.3 Lever 2: Progressive Context Digest

**Vấn đề:** Mỗi downstream checkpoint gửi toàn bộ prior artefacts dưới dạng raw
content. Đến QA-Verify, user prompt có thể chứa 5+ artefacts với ~5,000-7,000 input tokens.
Phần lớn là context "để hiểu quyết định đã được đưa ra" — không phải content cần
xử lý trực tiếp.

```
Hiện tại — input tokens per checkpoint (prior artefacts only):
  Spec         :     0 tokens (first checkpoint)
  Arch         :   800 tokens (spec.md)
  Test         : 2,300 tokens (spec + arch)
  Breakdown    : 3,300 tokens (spec + arch + tests)
  Code         : 5,050 tokens (spec + arch + tests + breakdown + tasks)
  Ship         : 6,100 tokens (tất cả trên + code-plan)
  QA-Verify    : 6,900 tokens (tất cả trên)
  ─────────────────────────────────────────────────────────
  Total context overhead: 24,450 tokens

Proposed — mỗi checkpoint chỉ nhận:
  a) Full artefact của checkpoint TRỰC TIẾP TRƯỚC (nó cần review/build từ đó)
  b) Digest (~300 tokens) của tất cả artefact cũ hơn

  Spec         :     0 tokens
  Arch         :   800 tokens (spec.md — full, vì arch đang viết dựa trên spec)
  Test         :   800 (arch — full) + 300 (spec digest)     = 1,100 tokens
  Breakdown    :   800 (arch — full) + 600 (spec+test digest) = 1,400 tokens
  Code         :   900 (breakdown+tasks — full) + 900 (digest 3 prior) = 1,800 tokens
  Ship         :   600 (code-plan — full) + 1,200 (digest 5 prior) = 1,800 tokens
  QA-Verify    :   800 (ship-report — full) + 1,500 (digest 6 prior) = 2,300 tokens
  ─────────────────────────────────────────────────────────
  Total context overhead: 9,400 tokens (-61% reduction)
```

**Digest format (per prior checkpoint, capped 300 tokens):**

```yaml
# .forge/specs/<slug>/digests/arch.digest.yaml  
checkpoint: arch
generated: 2026-05-27T10:30:00Z
source_hash: "sha256:abc123..."
decisions:
  - "REST API, 3 endpoints: POST /invoice, GET /invoice/{id}, PATCH /invoice/{id}/status"
  - "PostgreSQL, RLS enabled, tenant isolation via workspace_id column"
  - "Rate limiting: 100 req/min per tenant via Redis sliding window"
constraints:
  - "No breaking changes to existing /v1/payment endpoint"
  - "PHI fields must be encrypted at rest (AES-256)"
risks_accepted:
  - "Eventual consistency in invoice status updates (async via outbox)"
```

Digest được generate 1 lần sau khi checkpoint hoàn thành, dùng lại cho tất cả
downstream checkpoints. Chi phí: 1 LLM call (~2,000 tokens) per checkpoint để tạo
digest → amortized qua nhiều downstream consumers.

**Câu hỏi mở:** Digest generated by LLM hay bằng deterministic extraction từ
structured artefacts (spec.yml có sẵn)? Hybrid: spec.yml fields → structural digest
miễn phí; arch.md/tests.md → LLM-generated digest.

---

### 4.4 Lever 3: Model Cascade Per Checkpoint

**Vấn đề:** Hiện tại không có model routing — provider là 1 model duy nhất cho
toàn pipeline. Nhưng không phải checkpoint nào cũng cần reasoning nặng.

```
Phân loại theo task complexity:

  GENERATION (cần reasoning sâu):
    arch (multi-role debate, threat modeling)  → tier-1
    code (implementation strategy, OWASP)      → tier-1  
    ship security gate (HIGH-conf findings)    → tier-1

  STRUCTURED EXTRACTION (pattern matching):
    spec (extract ACs, NFRs từ description)   → tier-2
    test (fill TDD template, 4 artifact types) → tier-2
    breakdown (atomic tasks từ arch template)  → tier-2

  SYNTHESIS (summarize, classify):
    debate synthesis call                      → tier-2
    context digest generation                  → tier-2
    qa-verify manual test plan                → tier-1 (6-role cross-check)

Cost comparison (1,000-token output call):
  claude-3-5-sonnet: $0.015
  claude-3-5-haiku:  $0.004
  gpt-4o-mini:       $0.0006
  Ratio sonnet/haiku: 3.75×
  
Projected fleet savings (mix: 60% tier-2, 40% tier-1):
  Before: 100% sonnet  → $3.41/feature (worst case)
  After:  60/40 mix    → ~$1.85/feature (-46%)
```

**Implementation:** thêm `model` field vào `Phase` struct trong `subworkflow.go`:

```go
type Phase struct {
    // existing fields...
    ModelTier   string // "tier-1" | "tier-2" | "" (use provider default)
}
```

`LLMPipe.Invoke` resolves tier → model name theo provider config. Cho phép
override qua `forge config set ship.model_policy.spec tier-1`.

---

### 4.5 Lever 4: Remediation Loop — Shrinking Context

**Vấn đề:** Auto-remediation trong Ship (5 rounds) và QA-Verify (5 rounds) gửi
lại FULL context mỗi round. Nhưng sau round 1, vấn đề đã được thu hẹp: chỉ còn
1-2 gap items cụ thể. Full context lúc này là nhiễu.

```
Round structure hiện tại:
  Round 1: full_context (8,100 tokens) + gap_description → fix
  Round 2: full_context (8,100 tokens) + updated gap → fix
  Round 3: full_context (8,100 tokens) + updated gap → fix
  ...
  Cost: 5 rounds × 8,100 input + 5 × 1,500 output = 48,000 tokens ($0.86)

Round structure proposed:
  Round 1: full_context (8,100) + gap_list → targeted_fix
           → extract: specific_gap_item + attempted_fix + reason_for_failure
  Round 2: gap_item (100) + attempted_fix (300) + "why it failed" (150) → correction
           cost: ~550 input + 500 output = 1,050 tokens
  Round 3: gap_item (100) + round2_attempt (300) → final attempt
           cost: ~400 input + 500 output = 900 tokens
  Cost: 1 round full (9,600) + 2 rounds minimal (1,950) = 11,550 tokens ($0.21)
  Savings vs current 3 rounds: 48,000 → 11,550 = -76%
```

**Implementation sketch:**

```go
type RemediationState struct {
    GapItem      string // specific gap being remediated
    PriorAttempt string // last generated fix (truncated to 300 tokens)
    FailureNote  string // why prior attempt didn't resolve the gap
    Round        int
}

func remediateWithShrinkingContext(pipe *LLMPipe, state RemediationState) (string, error) {
    if state.Round == 1 {
        // Round 1: full context (existing behavior)
        return remediateFullContext(pipe, state)
    }
    // Round 2+: minimal context — only the gap and prior attempt
    system := fmt.Sprintf(minimalRemediationSystem, state.GapItem)
    user := fmt.Sprintf("Previous attempt:\n%s\n\nWhy it failed: %s\n\nFix it.",
        state.PriorAttempt, state.FailureNote)
    return pipe.Invoke("ship:remediate:round"+strconv.Itoa(state.Round),
        "", system, user, 600)
}
```

---

### 4.6 Lever 5: Complexity Tiering — Pipeline Routing

Đây là lever có **tổng impact lớn nhất** nếu được calibrate đúng, vì nó không
chỉ giảm token burn mà còn giảm số checkpoints chạy.

**Hypothesis:** Phân phối complexity trong thực tế:

```
Nano (trivial)   — config change, copy edit, minor UI: ~25% features
Micro (simple)   — 1-2 endpoints, no schema change:    ~35% features  
Standard         — new service/module, schema changes:  ~30% features
Complex          — cross-service, compliance domain:    ~10% features

Token burn per tier:
  Nano     :  2 checkpoints (spec-lite + verify)           =  ~5,000 tokens   $0.02
  Micro    :  4 checkpoints, no debate                     = ~22,000 tokens   $0.19
  Standard :  7 checkpoints, light debate (1 round)        = ~50,000 tokens   $0.87
  Complex  :  7 checkpoints + domain gates + full debate   = ~90,000 tokens   $1.65

Fleet average (với tiering vs không):
  Without tiering: 100% standard path = $0.87/feature avg
  With tiering:    0.25×$0.02 + 0.35×$0.19 + 0.30×$0.87 + 0.10×$1.65
                 = $0.005 + $0.067 + $0.261 + $0.165 = $0.50/feature avg
  Savings: -43% fleet cost
```

**Classifier (không dùng LLM — heuristic thuần):**

```go
type ComplexitySignal struct {
    DescriptionWords    int
    MentionsNewService  bool   // "new service", "microservice", "worker"
    MentionsMigration   bool   // "migrate", "schema", "ALTER TABLE"
    MentionsCompliance  bool   // "PCI", "HIPAA", "GDPR", "SOX", "audit"
    MentionsExternal    bool   // "third-party", "webhook", "integration"
    HasSpecYML          bool   // spec.yml already exists from prior run
    ExistingArtefacts   int    // how many .forge/specs/<slug>/ files exist
}

func classifyComplexity(desc string, root string) ComplexityTier {
    s := analyzeSignals(desc, root)  // pure string matching, no LLM
    score := 0
    if s.MentionsMigration  { score += 40 }
    if s.MentionsCompliance { score += 40 }
    if s.MentionsNewService { score += 25 }
    if s.MentionsExternal   { score += 20 }
    if s.DescriptionWords > 100 { score += 15 }
    switch {
    case score >= 60: return ComplexityComplex
    case score >= 30: return ComplexityStandard
    case score >= 10: return ComplexityMicro
    default:          return ComplexityNano
    }
}
```

Zero LLM cost để route. Được chạy ở đầu pipeline trước bất kỳ call nào.

**Câu hỏi mở:** False positive (classify Nano nhưng thực ra là Standard) đắt hơn
false negative? Cần calibration data từ real features. Chiến lược an toàn: bắt đầu
với thresholds cao (khó vào Nano), nới lỏng sau khi có data.

---

### 4.7 Lever 6: JSON-Structured Output

**Vấn đề:** Free-form markdown output của LLM có 15-25% tokens lãng phí vào:
- Preamble: _"I'll now analyze the architecture and provide..."_ (~30-50 tokens)
- Postamble: _"In summary, the above design addresses..."_ (~40-60 tokens)
- Hedging: _"This might", "could potentially", "it's possible that"_ (~20-40 tokens)
- Verbose section headers lặp lại instruction (~30-50 tokens)

Tổng: ~120-200 tokens/call × nhiều calls = đáng kể về tổng.

Quan trọng hơn: JSON-structured output cho phép **streaming interrupt** — dừng
generation khi tất cả required fields đã được fill.

```go
// Ví dụ: spec checkpoint với structured output
type SpecOutput struct {
    Summary          string   `json:"summary"`           // max 2 sentences
    AcceptanceCriteria []string `json:"acceptance_criteria"` // Given/When/Then
    NFRs             []NFR    `json:"nfrs"`
    ImpactedSystems  []string `json:"impacted_systems"`
    RiskLevel        string   `json:"risk_level"`        // low|medium|high|critical
    SpecYMLFields    SpecYAML `json:"spec_yml"`
}

// Output budget giảm vì không có preamble/postamble
// Structured output giúp gate hooks parse dễ hơn, không cần regex
// Streaming interrupt: dừng khi tất cả fields present = tiết kiệm tail tokens
```

Cần đánh giá: không phải mọi provider đều support JSON mode với schema enforcement
(Anthropic hỗ trợ qua tool_use pattern, OpenAI qua response_format).

---

### 4.8 Lever 7: Prompt Prefix Caching

Anthropic (cache_control) và OpenAI (prompt caching tự động) cache prefix của
prompt khi prefix > 1,024 tokens. Subsequent calls với cùng prefix được discount
50-90% input token cost.

**Điều kiện:** Phần static của system prompt phải đứng TRƯỚC phần dynamic.

```
Hiện tại (order trong applySteerings):
  [steerings] + [base system]                 = static  ← OK
  sau đó AppendDocsBudgeted thêm [KB entries] = static  ← OK
  nhưng KB entries re-selected mỗi call       = MAY DIFFER
  
Vấn đề: KB entries thay đổi giữa các calls cùng checkpoint
(tags scoring khác nhau) → cache miss.

Fix: freeze KB selection tại đầu checkpoint (không re-select mỗi sub-call)
     → static prefix = steerings + base system + frozen KB entries
     → dynamic suffix = artefact content + description
     → Prefix cache hit trên tất cả debate sub-calls trong cùng checkpoint
     
Savings: subsequent debate calls (calls 2-18) nhận cache discount
  Input discount: ~50-90% trên ~1,950 token static prefix
  Worst case (50%): 17 calls × 975 tokens discount = 16,575 tokens ≈ $0.05
  Best case (90%): 17 calls × 1,755 tokens = 29,835 tokens ≈ $0.09
```

**Bug hiện tại:** `InvokeWithKnowledge` re-calls `knowledge.Select()` và
`knowledge.AppendDocsBudgeted()` trên mỗi invocation. KB entries không stable
giữa các calls trong cùng checkpoint → không thể exploit prefix cache. Fix đơn
giản: cache KB selection per checkpoint run trong `RunOptions`.

---

### 4.9 Design Bug: KB Budget Sử Dụng Sai `maxTokens`

Phát hiện từ code review của `InvokeWithKnowledge`:

```go
// llmpipe.go
func (p *LLMPipe) InvokeWithKnowledge(..., maxTokens int, ...) (string, error) {
    entries := knowledge.Select(idx, checkpoint, family, tmpl, tags)
    enriched := knowledge.AppendDocsBudgeted(system, entries, maxTokens)  // ← BUG
    return p.Invoke(operation, model, enriched, user, maxTokens)
}
```

`AppendDocsBudgeted(system, entries, maxTokens)` dùng `maxTokens` (output budget)
làm budget cho KB entries. Điều này sai hoàn toàn:
- Khi output budget nhỏ (spec=1,500): KB bị cắt ngắn quá mức
- Khi output budget lớn (qa-verify=5,000): KB được inject quá nhiều

Đúng ra phải dùng `availableInputTokens = contextWindowSize - estimatedSystemSize - estimatedUserSize`.

Fix:

```go
func (p *LLMPipe) InvokeWithKnowledge(..., maxTokens int, ...) (string, error) {
    entries := knowledge.Select(idx, checkpoint, family, tmpl, tags)
    // Correct budget: remaining input capacity, not output budget
    kbBudget := p.provider.Capabilities().MaxTokens - maxTokens - estimateTokens(system) - estimateTokens(user)
    if kbBudget < 200 { kbBudget = 0 } // not worth injecting
    enriched := knowledge.AppendDocsBudgeted(system, entries, kbBudget)
    return p.Invoke(operation, model, enriched, user, maxTokens)
}
```

Đây là bug ảnh hưởng trực tiếp đến token burn: QA-Verify với maxTokens=5,000
hiện đang inject KB entries lên đến 5,000 token budget, trong khi spec với
maxTokens=1,500 bị giới hạn ở 1,500 — hoàn toàn ngược với nhu cầu thực tế.

---

### 4.10 Tổng Hợp: Impact Matrix

```
Lever                           Impl Effort   Token Savings   Cost Savings   Priority
────────────────────────────────────────────────────────────────────────────────────────
L1: Stripped debate overhead    Low           ~47,700/feature  ~$0.14        P1 ★★★
L2: Progressive context digest  Medium        ~15,000/feature  ~$0.22        P1 ★★★
L3: Model cascade               Low           (same tokens)    ~$0.37        P1 ★★★
L4: Remediation shrinking ctx   Medium        ~36,000/feature  ~$0.51        P1 ★★★
L5: Complexity tiering          Medium        -43% fleet avg   ~$0.37/feat   P1 ★★★
L6: JSON structured output      Low           ~10%/call        ~$0.03/feat   P2 ★★
L7: Prefix cache exploit        Low           ~17,000/feature  ~$0.05        P2 ★★
Bug fix: KB budget              Very Low      quality fix      N/A           P0 🔴

Combined P1 savings (L1+L2+L3+L4+L5):
  Worst-case feature:   $3.41 → ~$0.90  (-74%)
  Average fleet:        $0.87 → ~$0.32  (-63%)
  
Target state:
  tokens_per_feature  ≤ 30,000 (từ ~102,000 worst / ~49,700 best hiện tại)
  cost_per_feature    ≤ $0.35  (từ $3.41 worst / $0.86 best hiện tại)
  fleet_avg_cost      ≤ $0.30/feature với complexity tiering
```

### 4.11 Đo Lường

```go
// Thêm vào ShipResult.TokenUsage (đã có field này)
type TokenBreakdown struct {
    PrimaryCallsInput    int     `json:"primary_input"`
    PrimaryCallsOutput   int     `json:"primary_output"`
    DebateCallsInput     int     `json:"debate_input"`
    DebateCallsOutput    int     `json:"debate_output"`
    RemediationInput     int     `json:"remediation_input"`
    RemediationOutput    int     `json:"remediation_output"`
    ContextOverheadInput int     `json:"context_overhead_input"` // prior artefacts
    SteeringTokens       int     `json:"steering_tokens"`        // fixed overhead
    KBEntryTokens        int     `json:"kb_tokens"`
    CostUSD              float64 `json:"cost_usd"`
    ComplexityTier       string  `json:"complexity_tier"`
}
```

`forge insights token-report <feature>` — hiển thị breakdown này per feature.  
`forge insights token-trend` — trend theo thời gian (cần data từ nhiều runs).

---

## 5. Continuous Improvement Engine

### 5.1 Feedback Loops

```
4 feedback loops theo time horizon khác nhau:

Loop 1 (Immediate, per-run):
  - Failure records → pre-prompt context (đã có)
  - NEW: auto-tune steering nếu same checkpoint fails 3 lần liên tiếp
  - NEW: model fallback nếu primary model timeout/rate-limit

Loop 2 (Short-term, per-week):
  - Pattern extraction từ successful runs (đã có, nhưng manual)
  - NEW: automatic nếu run_count > 5 và avg_success_rate > 85%
  - NEW: failure clustering → named anti-patterns → thêm vào KB
  - NEW: `forge insights weekly` report: top failures, emerging patterns

Loop 3 (Medium-term, per-sprint):
  - A/B testing kết quả → promote winning variants (đề xuất ở 3.3)
  - Steering library review: cull entries với confidence < 0.3
  - `forge insights sprint-review`: velocity, cost trend, quality trend

Loop 4 (Long-term, per-quarter):
  - Org KB audit: gì được dùng, gì bị bỏ qua
  - Model performance comparison: quality/cost trade-off
  - Domain profile evolution: industry regulation changes
  - Forge team: update global KB với learnings từ all customers
```

### 5.2 Phát Hiện Drift Sớm

```
Một enterprise pain point: codebase evolve nhưng specs/patterns lỗi thời

Ship v2 giới thiệu Continuous Alignment Monitor:
  - Chạy background (scheduled hoặc triggered bởi git push)
  - So sánh code structure với specs trong .forge/specs/
  - Detect:
    - New endpoints không có spec
    - Schema changes không có migration spec  
    - Auth changes không có security spec update
  - Report: `forge drift report` (không block, chỉ warn)
  - Tích hợp vào pre-push hook (optional, default: off)
```

---

## 6. Migration Path

```
Ship v1 → Ship v2: backward-compatible migration

Phase 1 (non-breaking):
  - Thêm observability (OTLP emission) — không thay đổi behavior
  - Thêm snapshot trước checkpoint (invisible to user)
  - Rollout model-tier routing (transparent, controlled bởi config)

Phase 2 (opt-in):
  - Parallel pipeline: `forge config set ship.parallel true`
  - Domain profiles: chỉ active khi .forge/domains/ có file
  - Adaptive token budget: `forge config set ship.adaptive_budget true`

Phase 3 (new default, old opt-out):
  - Approval workflow v2: --legacy-gate flag để dùng y/N cũ
  - Learning loop v2 (Layer 2 KB): --no-org-kb để disable
  - Compound checkpoints: --no-compound để disable

Phase 4 (deprecation):
  - Remove --legacy-gate
  - Ship v1 code path removed
  - All pipeline runs through v2 DAG scheduler
```

---

## 7. Open Questions — Cần Thảo Luận

### OQ-1: Complexity Scorer Implementation
> Cần 1 LLM call để tính complexity_score từ spec không? Nếu có, đây là LLM call ngoài budget. Hay dùng heuristic đơn giản (line count, regex pattern count)? Trade-off: accuracy vs cost.

### OQ-2: Parallel Pipeline & Team Coordination
> Khi 2 engineer cùng `forge ship` trên cùng 1 feature branch, checkpoints conflict như thế nào? Cần distributed lock (Redis?) hay chỉ file-based lock?

### OQ-3: Org KB Distribution Model
> - Option A: Git submodule (version-controlled, requires write access)
> - Option B: Internal HTTPS registry (like npm private registry)
> - Option C: Embedded trong forge.config.yml (simple but limited)
> - Option D: Forge cloud service (SaaS, subscription-based, data privacy concerns)

### OQ-4: Custom Domain Profiles — Security Model
> Domain profiles có thể chứa arbitrary scripts (`.forge/hooks/*.sh`). Cần sandbox không? Hiện tại `internal/procspawn` có allow-list. Nên require WASM plugin (per ADR-002) cho security, hay cho phép native scripts với explicit user consent?

### OQ-5: Async Approval — State Backend
> Pipeline suspend state cần lưu đủ thông tin để resume sau N giờ/ngày. File-based ok cho single-machine. Khi distributed (team development, CI): cần external state store. Options: git-backed state, database, Forge cloud. Threshold nào để upgrade?

### OQ-6: PII Detection Before LLM
> PII detector cần chạy mỗi LLM call. Latency overhead ~50-100ms per call (nếu regex-based) hay ~500ms (nếu ML-based). Acceptable? Nên ship built-in detector hay plugin interface?

### OQ-7: Debate Round Parallelism
> LLM providers có rate limits (requests/min). 6 parallel calls trong arch debate có thể hit rate limit với small-tier plans. Fallback strategy: serial (safe, slow) hay jitter + retry (complex)? Nên expose `ship.debate_concurrency` config?

### OQ-8: Learning Data Privacy
> Learned patterns extracted từ feature descriptions có thể contain sensitive business logic. Trước khi contribute lên Org KB Layer 2, cần anonymization hay explicit opt-in per pattern?

### OQ-9: Backward Compatibility của Domain Profiles
> Khi forge version mới thay đổi checkpoint behavior, domain profiles viết cho version cũ có bị break không? Cần versioning schema cho domain profiles (`profile_schema_version: "2"`) và migration tooling?

### OQ-10: Compound Checkpoints & Resumability
> Nếu "Spec + Test Design" compound thành 1 call và call đó fail giữa chừng: resume tiếp tục từ đâu? Cần phân tách partial output (structured streaming parse) hay chạy lại toàn bộ compound call?

---

## 8. Summary: Enterprise Readiness Checklist

| Category | Current | Ship v2 | Priority |
|---|---|---|---|
| Pipeline parallelism | ❌ Sequential | ✅ DAG-based | P1 |
| Adaptive token budget | ❌ Fixed | ✅ Complexity-driven | P1 |
| Model tier routing | ❌ Single model | ✅ Tier-1/2 routing | P1 |
| Parallel debate | ❌ Sequential | ✅ Concurrent roles | P1 |
| 3-layer KB | ❌ Project-only | ✅ Project+Org+Global | P1 |
| Domain profiles | ❌ None | ✅ YAML-configurable | P1 |
| Multi-approver gates | ❌ y/N only | ✅ Quorum + async | P2 |
| Approval audit trail | ❌ None | ✅ Immutable log | P2 |
| Slack/Teams approval | ❌ None | ✅ Webhook integration | P2 |
| OpenTelemetry traces | ❌ None | ✅ Full OTEL | P2 |
| Prometheus metrics | ❌ token-ledger only | ✅ Full metrics | P2 |
| Transactional snapshots | ❌ None | ✅ Per-checkpoint | P2 |
| forge undo integration | ❌ Disconnected | ✅ Wired into pipeline | P2 |
| PII detection | ❌ None | ✅ Pre-LLM filter | P3 |
| Async approval | ❌ None | ✅ Suspend + resume | P3 |
| A/B steering tests | ❌ None | ✅ Auto-experiment | P3 |
| Continuous drift detect | ❌ None | ✅ Background monitor | P3 |
| Compound checkpoints | ❌ None | ✅ Opt-in | P3 |
| Immutable audit trail | ❌ Mutable JSON | ✅ Signed + offloaded | P3 |
| Incremental re-run | ❌ Full re-run | ✅ Diff-aware | P3 |

**P1 = Cần cho enterprise pilot** (có thể ship trong 2-3 sprint)  
**P2 = Cần cho enterprise GA** (4-6 sprint)  
**P3 = Nice-to-have, competitive advantage** (roadmap 6-12 tháng)

---

*Spec này dùng cho thảo luận. Chưa commit bất kỳ implementation nào. Mọi số liệu (token budget, latency targets, cost estimates) là ước tính sơ bộ cần validate với benchmark thực tế.*
