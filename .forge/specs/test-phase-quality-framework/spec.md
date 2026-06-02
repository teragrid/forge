# Spec: Test Phase Quality Framework (RFC-005 §6)

**Slug**: test-phase-quality-framework  
**Phase scope**: P1 + P2  
**RFC reference**: RFC-005 §6.1–6.8  
**Author**: Forge Core Team  
**Date**: 2026-06-02  
**Status**: Spec ✅ · Arch ⬜ · Test ⬜ · Breakdown ⬜ · Code ⬜ · Ship ⬜ · QA-Verify ⬜  

---

## Problem Statement

The current `forge ship` test checkpoint fills a single TDD template via one LLM call
to produce 4 artefact types — all hardcoded to TypeScript/Jest regardless of the actual
project tech stack. There is no quality scoring, no AC-to-test traceability, no role-based
review, and no CI alignment validation. As a result:

- Spec/test mismatches occur (pytest patterns injected into Go projects) — Gap G10
- Acceptance Criteria can pass without actual test coverage — no traceability
- Test depth varies wildly — happy-path-only tests are common
- CI disconnect — generated `tests.md` passes even if actual CI fails

---

## Goals

1. **Framework-awareness (P1)**: Detect the actual project test framework deterministically
   (zero LLM calls) and generate language-appropriate test stubs.
2. **Regression guard (P1)**: When bug-fix signals are present, D7 is mandatory at ≥8.
3. **Dimension warnings (P1)**: Emit `gate_status: WARNING` when any test dimension is missing;
   do not block the pipeline in P1.
4. **9-dimension scoring (P2)**: Compute a weighted composite score (≥6.5 to pass) and block
   the pipeline when thresholds are not met.
5. **Traceability matrix (P2)**: Write `traceability.yaml` with a full AC→test mapping and
   `coverage_summary` including per-dimension coverage and composite score.
6. **Multi-role test review (P2)**: Run 3 roles (QA Architect, Security Tester, Reliability
   Tester) in one parallel round with synthesis.
7. **CI gate validation (P2)**: Verify test files exist on disk; optionally execute tests.

---

## Out of Scope

- Fuzz test stub auto-generation (§6.7) — deferred to P3
- Approval workflow changes — deferred to P4
- Distributed lock for concurrent `forge ship` runs (OQ-2)

---

## Acceptance Criteria

### P1

| AC | Summary | Test dimension |
|---|---|---|
| AC-001 | `TestFrameworkContext` struct declared with fields: Language, TestRunner, AssertionStyle, MockLibrary, CoverageCmd, FuzzSupport, IntegTestDir, FixtureDir, ExistingTests | D8 (data accuracy) |
| AC-002 | `detectTestFramework(root string) TestFrameworkContext` performs deterministic detection for Go, Python, TypeScript, Java — zero LLM calls | D1 (happy path), D2 (boundary), D3 (negative) |
| AC-003 | Test checkpoint system prompt enriched with `TestFrameworkContext`; framework-specific stubs generated (no pytest for Go, no Jest for Python) | D7 (regression guard — G10 fix) |
| AC-004 | Bug-fix signals in feature description trigger D7 mandatory at ≥8; two labeled `// Regression:` test stubs generated (one pre-fix fail, one post-fix pass) | D6 (authz/security), D7 |
| AC-005 | `gate_status: WARNING` emitted when any of D1–D9 dimensions are uncovered; pipeline does NOT block in P1 | D9 (false-positive guard) |
| AC-006 | Go projects produce `*_test.go` stubs only (no `.test.ts` generated); TypeScript projects produce `.test.ts` only | D7 (regression guard) |

### P2

| AC | Summary | Test dimension |
|---|---|---|
| AC-007 | 9-dimension scoring rubric implemented: weights and thresholds per §6.2 table; composite = Σ(score_i × weight_i) / Σ(weight_i) | D1, D8 |
| AC-008 | Pipeline emits `gate_status: BLOCK` and halts when composite < 6.5 OR any dimension with weight ≥ 1.5 scores below its threshold | D2 (boundary), D3 (negative) |
| AC-009 | T0-tier features may waive D5/D6/D8 with justification recorded in traceability.yaml; T2 raises D6 threshold to ≥9 | D4 (idempotency), D6 |
| AC-010 | `traceability.yaml` written to `.forge/specs/<slug>/` with full RFC §6.5 format: `feature`, `generated`, `spec_version`, `matrix[]`, `coverage_summary` | D8 (data accuracy) |
| AC-011 | 3-role test review runs 1 parallel round (QA Architect, Security Tester, Reliability Tester) with QA Architect synthesis; tier-2 roles, tier-1 synthesis | D5 (concurrency) |
| AC-012 | CI Alignment Check: all file paths in `traceability.yaml` matrix entries verified to exist on disk; coverage shell snippet emitted to stdout | D1, D3 |
| AC-013 | When `ship.run_tests_on_verify = true`, execute the coverage command; non-zero exit → add failure to gap list → remediation loop fires | D4 (idempotency — same result on re-run) |
| AC-014 | `ship.test_debate_threshold` config key: when set to `T0`, 3-role debate is skipped for T0-tier features; single QA Architect pass runs instead | D9 (false-positive guard) |

---

## NFR

| Metric | Target |
|---|---|
| `detectTestFramework` latency | < 50 ms (pure filesystem scan) |
| `WriteTraceability` latency | < 200 ms |
| 3-role parallel debate overhead | ~6,500 tokens per feature (§6.4) |
| Composite score computation | < 5 ms (pure arithmetic) |

---

## Open Questions (Resolved)

| OQ | Question | Resolution |
|---|---|---|
| OQ-1 | Complexity scoring: LLM or heuristic? | Heuristic (line count + pattern count) — AC-009 |
| OQ-2 | T0 test debate: skip entirely or single-pass? | Single QA Architect pass via `ship.test_debate_threshold` config — AC-014 |
| OQ-3 | `run_tests_on_verify`: fail gracefully if env incomplete? | Graceful: skip with warning if coverage command fails to start — AC-013 |
