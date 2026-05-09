# ADR-016 — Failure-register data model

- **Status:** Proposed
- **Tracker:** ARCH-DEC-16
- **Spec/Arch anchor:** Arch §17.2, Arch §18.6
- **Decision date:** TBD
- **Deciders:** Core engineer
- **Consulted:** Quality WG

## Context

Arch §17.2 documents per-component failure modes in a Markdown table. Without a machine-readable counterpart:

- Dashboards cannot count failures by row.
- Post-mortem PRs cannot mechanically prove they updated the right row.
- Drift between code (which raises `FORGE-NNNN`) and the doc table is invisible.

## Decision

The failure register will live at **`.forge/failure-register.yml`** (in the Forge source repo, NOT the user workspace) as the source of truth. The §17.2 doc table is generated/verified from this file by `forge audit failure-register verify`.

### Schema (`forge/schemas/failure-register.schema.json`)

```yaml
api_version: forge.sh/v1
kind: FailureRegister
metadata:
  generated_at: "2026-05-09T00:00:00Z"
spec:
  entries:
    - id: FR-005
      component: audit-ledger
      tier: T1
      failure_mode: "Append-only ledger corrupted or rewritten by a malicious actor."
      detection: "Per-host signing key + verifier; mismatch surfaces in `forge audit ledger`."
      recovery: "Quarantine the host; replay from last-known-good snapshot; rotate key."
      severity_default: S1
      error_codes: ["FORGE-1101", "FORGE-1102"]
      test_anchor: "TEST-23-04"
      drill_anchor: "drill-ledger-tamper"
      first_seen_in_doc_table: "2026-05-09"
      status: tracked   # tracked | retired
```

### Sync contract

`forge audit failure-register verify` enforces:

1. Every row in `Arch §17.2` table has a matching `FR-NNN` entry (cross-checked by `component` + `failure_mode`).
2. Every `FR-NNN` entry resolves to a real test (`test_anchor`) and a real drill (`drill_anchor` if non-null).
3. Every `error_codes` value matches an entry in `docs/errors/index.md` (per ADR-009).
4. PRs touching `.forge/failure-register.yml` must also touch `forge/ARCHITECTURE.md` §17.2 (or vice versa), unless the PR carries a `register-only` label.
5. A `status: retired` entry remains in the file (never deleted) and is omitted from the rendered table.

### CLI behaviour

- `forge audit failure-register verify` — exit 0 on parity; non-zero with diff on drift.
- `forge audit failure-register list --json` — machine-readable dump for dashboards.
- `forge audit failure-register lint` — schema only (faster pre-commit hook).

## Alternatives considered

### Option A — Parse the Markdown table directly (rejected)

Pros: single source.
Cons: brittle; ambiguous columns; hostile to dashboards; lint authoring against Markdown is fragile.

### Option B — TOML format (rejected)

Pros: stricter syntax than YAML.
Cons: nested arrays of objects are awkward; team has more YAML tooling already.

### Option C — Database (SQLite) (rejected)

Pros: query power.
Cons: not diff-friendly; PR review impossible.

## Consequences

### Positive

- Dashboards (TEST-19) consume `--json` directly.
- Post-mortem PRs (per ADR-020) can be linted to require a register update.
- Drift between docs and code is impossible to merge silently.

### Negative / accepted trade-offs

- Two files to keep in sync — but the verifier exists exactly to make that mechanical.
- YAML schema evolution requires `api_version` bumps with backward-compat — handled by the standard Forge schema-versioning policy.

### Follow-ups created

- DEV-M1-40 — failure-register verifier + schema.
- TEST-30 — failure-register sync linter.
- ADR-020 — post-mortem template (references this register).

## Compliance hooks

- CI gate: `forge audit failure-register verify` on every PR.
- Test: parity test (TEST-30 TC-30-01).
- Test: missing/extra row fails verifier (TEST-30 TC-30-02 / TC-30-05).

## References

- Arch §17.2, §18.6.
- ADR-009 (error codes), ADR-015 (drills).
