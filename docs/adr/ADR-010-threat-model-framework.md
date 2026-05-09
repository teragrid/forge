# ADR-010 — Threat-model framework

- **Status:** Proposed
- **Tracker:** ARCH-DEC-10
- **Spec/Arch anchor:** Arch §15, Spec §15
- **Decision date:** TBD
- **Deciders:** Security engineer
- **Consulted:** DevSecOps, core engineering

## Context

Forge runs untrusted plugin code, talks to external LLM providers, holds developer secrets in transit, and writes to user filesystems. A threat model is required:

- Pre-1.0 to drive design (per Arch §15).
- Per-feature for material additions (per `forge ship` workflow gates).
- Reviewable by external security researchers without onboarding cost.

## Decision

Forge will use **STRIDE** as the primary threat-modelling framework, augmented with **LINDDUN-lite** for privacy threats (since telemetry + LLM prompts surface PII risks). Threat models live in `THREAT_MODEL.md` (project-wide) and `docs/threats/<feature>.md` (per-feature). Tooling: **markdown-driven** (no proprietary tool) + an embedded YAML block per threat for machine indexing.

### Threat entry shape (machine-readable)

```yaml
- id: THREAT-005
  category: STRIDE.Tampering
  asset: audit-ledger
  attacker: malicious-plugin (T3)
  description: A plugin appends a forged ledger entry to mask its own actions.
  likelihood: medium
  impact: high
  mitigations:
    - id: MIT-005a
      summary: Ledger entries signed with per-host key (Arch §17.2 row 5).
      test: TEST-23
  status: mitigated
```

`forge audit threat-model verify` lints the file: every threat has ≥1 mitigation, every mitigation has ≥1 test anchor, no `status: open` item is older than 90 days without an exception.

### Per-feature threat-model trigger

A feature spec (per ADR-007) whose frontmatter sets `security_review: required: true` cannot be merged without a corresponding `docs/threats/<feature>.md`.

## Alternatives considered

### Option A — PASTA (rejected)

Pros: business-context-driven; thorough.
Cons: heavyweight for an OSS CLI; onboarding cost too high for community contributors.

### Option B — OCTAVE (rejected)

Pros: enterprise-proven.
Cons: organisational-risk focus; mismatch with engineering-feature granularity.

### Option C — Microsoft Threat Modeling Tool (rejected)

Pros: mature UX.
Cons: Windows-only; binary file format; not diffable in PRs.

### Option D — STRIDE only (no LINDDUN) (rejected)

Pros: simpler.
Cons: privacy risks (telemetry, prompt logging) underweighted; ADR-006 obligations would lack a structured frame.

## Consequences

### Positive

- Markdown + YAML → diffable in PRs, no special tooling.
- STRIDE is widely understood; LINDDUN-lite covers the privacy gap.
- Verifier ties threats to tests → mitigations are provably exercised.

### Negative / accepted trade-offs

- Manual data-flow diagrams (Mermaid) need to be kept in sync; verifier checks "diagram exists" but not semantics.
- LINDDUN-lite is a curated subset, not the full method — accepted to control complexity.

### Follow-ups created

- DEV-M0-15 — `THREAT_MODEL.md` skeleton + per-feature template.
- DEV-M0-16 — `forge audit threat-model verify` linter.
- TEST-22 — threat-model regression suite.

## Compliance hooks

- CI gate: `forge audit threat-model verify` runs on every PR.
- CI gate: PR labelled `security` cannot merge without an updated `THREAT_MODEL.md` entry.
- Test: every `MIT-NNNa` resolves to a real test ID (TEST-22).

## References

- STRIDE: <https://learn.microsoft.com/en-us/azure/security/develop/threat-modeling-tool-threats>.
- LINDDUN: <https://linddun.org/>.
- Arch §15.
