# ADR-023 — Eval-harness flake-quarantine policy

- **Status:** Proposed
- **Tracker:** ARCH-DEC-23
- **Spec/Arch anchor:** Arch §17.2 eval-harness row, Arch §17.3 #6
- **Decision date:** TBD
- **Deciders:** Quality WG
- **Consulted:** Core engineering, DevSecOps

## Context

LLM-touching evals are sensitive to model drift, cassette corruption, and provider non-determinism. Flaky scenarios that "just retry green" silently mask real regressions. A declared policy is required to balance noise vs masked-regression risk.

## Decision

Each scenario (per ADR-005) runs under a **3-run quorum** with these rules:

### Quorum rules

- A scenario is **green** iff ≥ 2 of 3 runs pass with byte-identical artefact assertions.
- A scenario is **flaky** if 1 of 3 disagrees → auto-quarantine (see below).
- A scenario is **broken** if all 3 disagree on the same assertion → CI fails immediately (no quarantine).
- A scenario is **suspicious** if all 3 disagree across different assertions → fails with a "non-reproducible" classification → escalate to on-call.

### Determinism guards (per scenario)

- **Seed pin:** `spec.determinism.seed` mandatory.
- **Cassette pin:** `spec.determinism.cassette_sha256` mandatory; mismatch on load = `FORGE-1701` and is **not** subject to the quorum (it fails immediately).
- **Tolerance:** numeric oracles may declare a `tolerance` (default 0); LLM-text oracles must use semantic-equivalence + a published rubric.

### Auto-quarantine

When a scenario is classed `flaky`:

1. CI exits 0 for the affected job (does NOT block merge).
2. The harness writes an entry to `eval/quarantine/<scenario-id>.yml`:
   ```yaml
   id: ship-reference-stripe
   first_quarantined_at: 2026-05-09
   quarantined_by: ci
   reason: 1-of-3 disagreement on token-budget oracle
   owner: null    # set by triage
   max_quarantine_until: 2026-06-08   # 30 days
   ```
3. An issue is auto-opened (template `flake.yml` per ADR-017) with `area:eval` + `severity:S3`.
4. The scenario remains quarantined (skipped from CI) for ≤ 30 days.

### Escape from quarantine

- An owner is assigned within 7 days (else OPS-related warning).
- The scenario must demonstrate **5 consecutive green runs** in a dedicated workflow before un-quarantine; recorded in the YAML as `unquarantined_at` + `evidence_run_ids`.
- Force-unquarantine requires two Maintainer approvals (per ADR-022 surface 3).

### Hard timeout

- A scenario quarantined > 30 days **without an owner** fails CI on the next nightly run (`forge eval --quarantine-report --fail-on-stale`).
- A scenario quarantined > 90 days regardless of owner triggers an automatic post-mortem (per ADR-020).

## Alternatives considered

### Option A — Always retry on flake, no quarantine (rejected)

Pros: simplest.
Cons: masks real regressions; the §17.3 #6 cassette-drift scenario becomes invisible.

### Option B — Single-run, fail loud (rejected)

Pros: zero hidden flakes.
Cons: provider-side non-determinism would block every CI run.

### Option C — N-of-M with N=5, M=7 (rejected)

Pros: more statistical confidence.
Cons: cost + latency 2.3× higher; 3-run quorum is the proven sweet spot.

## Consequences

### Positive

- Flaky scenarios are visible, owned, and time-bounded.
- Real regressions are not absorbed by retries.
- Cassette drift is detected instantly (separate code path from the quorum).

### Negative / accepted trade-offs

- 3× cost per scenario; mitigated by parallel execution and cassette-cache reuse.
- Quarantine YAML is yet another file to maintain; ADR-016 verifier cross-checks it against `failure-register.yml`.

### Follow-ups created

- DEV-M3-21 — flake-quarantine policy implementation.
- TEST-related — quarantine regression suite (covered by DEV-M3-21 TCs).
- ADR-005 — scenario format includes `determinism` block.

## Compliance hooks

- CI gate: `forge eval --quarantine-report --fail-on-stale` nightly (DEV-M3-21 TC-21-03).
- Test: cassette SHA mismatch fails immediately (DEV-M3-21 TC-21-05).
- Audit: monthly review of open quarantine YAMLs by Quality WG.

## References

- Arch §17.2 eval-harness row, §17.3 #6.
- Google "Avoiding Flaky Tests" practices.
