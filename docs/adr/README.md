# Forge — Architecture Decision Records (ADRs)

This directory holds the binding architectural decisions for the Forge framework. Every ADR resolves one tracked architecture task in [`../tasks/ARCHITECTURE_TASKS.md`](../tasks/ARCHITECTURE_TASKS.md).

## Workflow

1. A new ADR starts as a PR copying [`_TEMPLATE.md`](./_TEMPLATE.md) into `ADR-NNN-<slug>.md` (next free three-digit number).
2. **Status** transitions: `Proposed → Accepted` (or `Rejected`, `Superseded by ADR-NNN`, `Deprecated`). Only an Accepted ADR satisfies a tracker task's "ADR-NNN merged" acceptance.
3. Acceptance requires:
   - Two-maintainer approval per `forge/ARCHITECTURE.md` §18.4 + ARCH-DEC-22.
   - The matching tracker row is updated in the same PR (cross-ref check enforced by CI).
   - Rejected alternatives are listed with at least one sentence of why-not each.
4. Superseding an ADR uses a **new** ADR (do not rewrite history); the old one is marked `Superseded by ADR-NNN`.
5. ADR numbers are immutable and never reused, even if the ADR is rejected.

## Index (Status snapshot)

| # | Title | Status | Tracker |
|---|-------|--------|---------|
| 001 | Implementation language | Proposed (Go) | ARCH-DEC-01 |
| 002 | Plugin runtime | Proposed (WASM component model on `wazero`) | ARCH-DEC-02 |
| 003 | Distribution channels | Proposed (GH Releases + brew/scoop/winget + curl) | ARCH-DEC-03 |
| 004 | Registry storage | Proposed (signed JSON in Git + CDN) | ARCH-DEC-04 |
| 005 | Eval scenario format | Proposed (`scenario.yml` + JSON schema) | ARCH-DEC-05 |
| 006 | Telemetry transport | Proposed (OTLP/HTTPS, opt-in) | ARCH-DEC-06 |
| 007 | Spec format | Proposed (Markdown + YAML frontmatter) | ARCH-DEC-07 |
| 008 | License | Proposed (Apache-2.0) | ARCH-DEC-08 |
| 009 | Error-code namespacing | Proposed (`FORGE-<RANGE><NN>`) | ARCH-DEC-09 |
| 010 | Threat-model framework | Proposed (STRIDE + LINDDUN-lite) | ARCH-DEC-10 |
| 011 | Hygiene-manifest schema | Proposed (`hygiene.manifest.yml` v1) | ARCH-DEC-11 |
| 012 | `.gitignore` template composition | Proposed (per-stack fragments + managed-block markers) | ARCH-DEC-12 |
| 013 | Secret-scanning engine | Proposed (gitleaks + Forge rule-pack) | ARCH-DEC-13 |
| 014 | Resilience-pattern library | Proposed (in-tree, `failsafe`-style) | ARCH-DEC-14 |
| 015 | Chaos-drill harness | Proposed (in-tree synthetic faults + drill report schema) | ARCH-DEC-15 |
| 016 | Failure-register data model | Proposed (`.forge/failure-register.yml`) | ARCH-DEC-16 |
| 017 | Bug/issue tracker stack | Proposed (GH Issues + auto-triage bot + `Fixes:` trailer) | ARCH-DEC-17 |
| 018 | Vulnerability intake & disclosure | Proposed (GH Security Advisory + 90d window) | ARCH-DEC-18 |
| 019 | On-call rota model | Proposed (weekly rotation, paging via PagerDuty) | ARCH-DEC-19 |
| 020 | Post-mortem template + CI gate | Proposed (`docs/postmortems/_TEMPLATE.md`) | ARCH-DEC-20 |
| 021 | Status-page tooling | Proposed (cstate self-hosted) | ARCH-DEC-21 |
| 022 | Two-key enforcement | Proposed (sigstore co-sign + branch protections + bot) | ARCH-DEC-22 |
| 023 | Eval-harness flake-quarantine policy | Proposed (3-run quorum, 30d max quarantine) | ARCH-DEC-23 |
| 024 | Reversibility contract | Proposed (`.forge/trash/` + `forge undo`) | ARCH-DEC-24 |

## Conventions

- One decision per ADR. If an ADR grows two decisions, split it.
- Decision text MUST be a short imperative paragraph ("Forge will use …"). No conditional language ("we might", "perhaps") in the **Decision** section — those belong in **Context** or **Consequences**.
- Every ADR ends with a **Compliance hooks** section listing the test IDs / lint rules that prove the decision is in force.
