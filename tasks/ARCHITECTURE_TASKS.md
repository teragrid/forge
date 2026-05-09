# Forge — Architecture Decision Tasks

> Companion to `../FORGE_FRAMEWORK_SPEC.md` and `../ARCHITECTURE.md`.
> Tracker for the Section-A tasks from `../TASK_BREAKDOWN.md`.

These are the architectural questions that must resolve before downstream tasks unblock. **Close before/during M0.**

Conventions used across all task files:
- **ID format:** `<PLAN>-<MILESTONE>-<NN>` (e.g. `ARCH-DEC-01`).
- **Tier:** maps to spec §16.5.1 trust tiers (T1 core / T2 adapter / T3 plugin / DOC docs / OPS ops).
- **Acceptance:** every task has objective, CI-checkable acceptance criteria.
- **Spec anchors:** every task references the spec section that justifies it.
- **Definition of Done (universal):** task ships through the workflow it describes — pre-M1 via the manual checklist, post-M1 via `forge ship`. All §16.5.4 universal gates pass.

---

| ID | Task | Owner role | Tier | Spec/Arch anchor | Acceptance |
|----|------|-----------|------|------------------|------------|
| ARCH-DEC-01 | Decide implementation language (Rust vs Go) | Founder + 1 reviewer | T1 | Arch §13 ADR-001 | ADR-001 merged with rationale + rejected alternatives → Proposed: **Go** — [ADR-001](../adr/ADR-001-implementation-language.md) |
| ARCH-DEC-02 | Decide plugin runtime (WASM component model vs in-proc) | Plugin WG lead | T1 | Arch §13 ADR-002 | ADR-002 merged + spike repo with `hello-plugin` working → Proposed: **WASM component model on `wazero`** — [ADR-002](../adr/ADR-002-plugin-runtime.md) |
| ARCH-DEC-03 | Decide distribution channels (brew/scoop/winget/curl) | DevSecOps | OPS | Arch §13 ADR-003 | ADR-003 merged; install matrix documented → Proposed: [ADR-003](../adr/ADR-003-distribution-channels.md) |
| ARCH-DEC-04 | Decide Registry storage (Git+CDN vs hosted DB) | Community WG | T1 | Arch §13 ADR-004 | ADR-004 merged; manifest schema published → Proposed: [ADR-004](../adr/ADR-004-registry-storage.md) |
| ARCH-DEC-05 | Decide eval scenario format (`scenario.yml` schema) | Quality WG | T1 | Arch §13 ADR-005 | ADR-005 merged + JSON schema published → Proposed: [ADR-005](../adr/ADR-005-eval-scenario-format.md) |
| ARCH-DEC-06 | Decide telemetry transport (OTLP) and opt-in mechanics | DevSecOps | T1 | Arch §13 ADR-006 | ADR-006 merged; payload schema documented → Proposed: [ADR-006](../adr/ADR-006-telemetry-transport.md) |
| ARCH-DEC-07 | Decide spec format (Markdown + YAML frontmatter) | Founder | T1 | Arch §13 ADR-007 | ADR-007 merged; `spec.md` linter scaffolded → Proposed: [ADR-007](../adr/ADR-007-spec-format.md) |
| ARCH-DEC-08 | Decide license (Apache-2.0 vs MIT vs BSL-then-Apache) | Legal advisor | OPS | Spec §8 Q22 | LICENSE file in repo before first public push → Proposed: [ADR-008](../adr/ADR-008-license.md) (LICENSE deferred pending legal sign-off) |
| ARCH-DEC-09 | Decide error-code namespacing scheme final form | Core engineer | T1 | Arch §11 | ADR + reserved-ranges doc; lint rule prevents reuse → Proposed: [ADR-009](../adr/ADR-009-error-code-namespacing.md) |
| ARCH-DEC-10 | Decide threat-model framework + tooling | Security engineer | T1 | Arch §15 | STRIDE template + `THREAT_MODEL.md` skeleton in repo → Proposed: [ADR-010](../adr/ADR-010-threat-model-framework.md); skeleton at [`forge/THREAT_MODEL.md`](../THREAT_MODEL.md) |
| ARCH-DEC-11 | Decide hygiene-manifest schema and ownership semantics | Core engineer | T1 | Spec §4 hygiene + §16.5.4 #11 | ADR-011 merged; manifest JSON schema + ownership rules published → Proposed: [ADR-011](../adr/ADR-011-hygiene-manifest-schema.md) |
| ARCH-DEC-12 | Decide `.gitignore` template composition model (per-stack fragments + managed-block markers) | Core engineer | T1 | Spec §4 Repo Hygiene Layer (`.gitignore` standards) | ADR-012 merged; fragment registry + version-stamp marker spec published → Proposed: [ADR-012](../adr/ADR-012-gitignore-template-composition.md) |
| ARCH-DEC-13 | Decide secret-scanning engine (gitleaks vs trufflehog vs in-house) and Forge-aware rule pack format | Security engineer | T1 | Spec §4 Repo Hygiene Layer (`.gitleaks.toml` standards) + §16.5.4 #12 | ADR-013 merged; rule-pack schema + allowlist `review-by` enforcement spec published → Proposed: [ADR-013](../adr/ADR-013-secret-scanning-engine.md) |
| ARCH-DEC-14 | Decide resilience-pattern library (circuit-breaker, retry-with-jitter, bulkhead, timeout-budget) — build vs adopt (e.g. `failsafe`/`resilience4j`-style) and the lint rules that enforce §17.4 invariants | Core engineer | T1 | Arch §17.1 + §17.4 | ADR-014 merged; reference implementation in foundation layer; lint rule rejects unbounded `await` in capability namespaces → Proposed: [ADR-014](../adr/ADR-014-resilience-pattern-library.md) |
| ARCH-DEC-15 | Decide chaos-drill harness (in-tree synthetic faults vs external tool) and the catalog format for §17.3 cross-cutting scenarios | Quality WG + DevSecOps | T1 | Arch §17.3 + OPS-17 | ADR-015 merged; harness can inject the 8 catalogued scenarios; per-scenario "drill report" schema published → Proposed: [ADR-015](../adr/ADR-015-chaos-drill-harness.md) |
| ARCH-DEC-16 | Decide failure-register data model (machine-readable counterpart to §17.2 — JSON/YAML schema, file location, sync contract with the doc table) so dashboards + post-mortem PRs can update it programmatically | Core engineer | T1 | Arch §17.2 + §18.6 | ADR-016 merged; schema published; `forge audit failure-register verify` lints the doc table against the source of truth → Proposed: [ADR-016](../adr/ADR-016-failure-register-data-model.md) |
| ARCH-DEC-17 | Decide bug/issue tracker tooling stack (issue templates, auto-triage bot, severity-label taxonomy, `Fixes:` trailer enforcement, `gate-bypass` workflow) | DevSecOps + Community WG | T1 | Arch §18.1 + §18.3 + §18.4 | ADR-017 merged; templates + bot live in `.github/`; CI rejects PRs lacking a `Fixes: #NNN` trailer when label is `bug` → Proposed: [ADR-017](../adr/ADR-017-bug-issue-tracker-stack.md) |
| ARCH-DEC-18 | Decide private-vulnerability intake + disclosure workflow (GitHub Security Advisory vs HackerOne vs huntr.dev primary; CNA status; coordinated-disclosure window default) | Security engineer | T1 | Arch §18.1 vulnerability row + Spec §15 | ADR-018 merged; `SECURITY.md` published with disclosure window + PGP key + safe-harbor language → Proposed: [ADR-018](../adr/ADR-018-vulnerability-intake-disclosure.md); skeleton at [`SECURITY.md`](../../SECURITY.md) |
| ARCH-DEC-19 | Decide on-call rota model (rotation cadence, escalation chain, paging tooling, eligibility tied to spec §16.5.8 ladder) and the public publication contract (`docs/oncall/`) | Delivery lead | T1 | Arch §18.2 + OPS-16 | ADR-019 merged; first 4-week rota published; rotation script in `scripts/` → Proposed: [ADR-019](../adr/ADR-019-oncall-rota-model.md) |
| ARCH-DEC-20 | Decide post-mortem template + storage location + linkage to failure-register entries; define the "durable-action-item" enforcement (CI rejects PMs whose action items have no tracking issue) | Quality WG | T1 | Arch §18.6 + OPS-18 | ADR-020 merged; template at `docs/postmortems/_TEMPLATE.md`; CI gate enforces `## 6. Action items` contains ≥1 linked issue + ≥1 §17.2-touching commit reference → Proposed: [ADR-020](../adr/ADR-020-postmortem-template-ci-gate.md) |
| ARCH-DEC-21 | Decide status-page tooling (self-hosted vs SaaS — Statuspage / Instatus / cstate) and the incident-state-machine wiring to §18.5 (identified → mitigated → fixed → post-mortem published) | DevSecOps | T1 | Arch §18.5 #6 + §18.8 | ADR-021 merged; status-page reachable; webhook publishes state transitions to the Quality Dashboard (TEST-19) → Proposed: [ADR-021](../adr/ADR-021-status-page-tooling.md) |
| ARCH-DEC-22 | Decide two-key enforcement mechanism for irreversible incident-time operations (sigstore key custody, branch-protection rule for force-push, registry trust-root rotation ceremony, gate-bypass PR check) | Security engineer + DevSecOps | T1 | Arch §18.4 | ADR-022 merged; branch protections live; sigstore signing requires two-custodian step; bot enforces second-maintainer approval on `gate-bypass`-labelled PRs → Proposed: [ADR-022](../adr/ADR-022-two-key-enforcement.md) |
| ARCH-DEC-23 | Decide eval-harness flake-quarantine policy (3-run quorum default, auto-quarantine threshold, auto-issue creation, max time in quarantine before forced revisit) | Quality WG | T1 | Arch §17.2 eval-harness row + §17.3 #6 | ADR-023 merged; `forge eval --quarantine-report` exists; CI gate fails when a scenario has been quarantined > 30 days without an owner → Proposed: [ADR-023](../adr/ADR-023-eval-flake-quarantine-policy.md) |
| ARCH-DEC-24 | Decide reversibility contract details — `.forge/trash/` retention window, `forge undo` granularity, FS-vs-DB inverse semantics, cross-platform safe-delete strategy | Core engineer | T1 | Arch §17.1 #5 | ADR-024 merged; retention default + override config; `forge undo` covers FS, DB migration, and `--apply` scan fixes → Proposed: [ADR-024](../adr/ADR-024-reversibility-contract.md) |

---

*Task file version: 0.4 — companion to spec v0.10.9. ADR-001..024 drafted as Proposed; acceptance pending two-Maintainer review per ARCH-DEC-22.*
