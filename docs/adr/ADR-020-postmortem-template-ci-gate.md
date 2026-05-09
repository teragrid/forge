# ADR-020 — Post-mortem template + CI gate

- **Status:** Proposed
- **Tracker:** ARCH-DEC-20
- **Spec/Arch anchor:** Arch §18.6, OPS-18
- **Decision date:** TBD
- **Deciders:** Quality WG
- **Consulted:** Delivery lead, core engineering

## Context

Without a forced post-mortem template, action items become "be more careful next time" platitudes. Without a CI gate, post-mortems silently get skipped under deadline pressure.

## Decision

Post-mortems live at **`docs/postmortems/INC-<n>-<slug>.md`**, follow the template at **`docs/postmortems/_TEMPLATE.md`**, and are enforced by **`forge audit postmortem verify`** running in CI.

### Template structure (8 mandatory sections)

```markdown
# INC-<n> — <title>

- **Status:** draft | published
- **Severity:** S0 | S1 | S2
- **Incident date:** YYYY-MM-DD
- **Author:** @handle
- **Reviewers:** @handle, @handle (≥ 2 required for `published`)
- **Tracking issue:** #NNN
- **Failure-register entries touched:** FR-NNN, FR-NNN

## 1. Summary
(One paragraph; user-visible impact + root cause.)

## 2. Impact
(Quantified: users affected, duration, data loss, financial cost.)

## 3. Timeline
(Bulleted, UTC timestamps. Include detection, paging, mitigation, resolution.)

## 4. Root cause
(The five-whys or causal chain; include code refs.)

## 5. What went well
(Detection time, rollback worked, etc.)

## 6. Action items
(Each MUST be a tracked issue link AND tagged owner + due date. AT LEAST ONE
 must reference a §17.2 register entry update OR a new test commit.)

- [ ] AI-01 — <description> — owner: @handle — due: YYYY-MM-DD — issue: #NNN — register: FR-NNN

## 7. Lessons / non-actions
(Things noted but explicitly not actioned, with rationale.)

## 8. Bypass log
(Required if `gate-bypass` was used during the incident; per ADR-017 / ADR-022.)
```

### CI gate (`forge audit postmortem verify`)

Validates each `docs/postmortems/INC-*.md`:

1. All 8 sections present (template-shape lint).
2. `## 6. Action items` contains ≥ 1 line matching the action-item shape (`- [ ] AI-NN — … — owner: … — due: … — issue: #NNN`).
3. ≥ 1 action item must include a `register: FR-NNN` reference OR a `commit: <sha>` reference to a test addition.
4. For `Status: published`, ≥ 2 distinct reviewers in frontmatter.
5. The frontmatter `tracking issue` exists and is closed (or marked `keep-open` with rationale).

### Triggering rules

- A closed `severity:S0` or `severity:S1` issue without a corresponding `INC-<n>-*.md` file in the same PR (or within 14 days for OPS-18 nightly check) **fails CI**.
- Any PR labelled `gate-bypass` (per ADR-017) requires a follow-up post-mortem within 7 days; OPS-18 enforces.

## Alternatives considered

### Option A — Free-form post-mortems (rejected)

Pros: low authoring friction.
Cons: drift; "be more careful" outcomes; no machine-checkable accountability.

### Option B — Issue-only post-mortems (no markdown file) (rejected)

Pros: native tracking.
Cons: poor long-term archive; harder to cross-link to architecture sections.

### Option C — Confluence / Notion docs (rejected)

Cons: not in the OSS repo; not reviewable by community.

## Consequences

### Positive

- Action items are machine-traceable to register entries + tests.
- Public archive of incidents builds operational credibility.
- Two-reviewer rule prevents drive-by post-mortems.

### Negative / accepted trade-offs

- Authoring time is non-trivial; mitigated by template + CLI scaffolding (`forge new postmortem`).
- Strict gate may delay incident closure; accepted because closure without a learning is the bug we are fixing.

### Follow-ups created

- DEV-M2-24 — post-mortem template + CI gate.
- OPS-18 — monthly post-mortem SLA monitor.
- TEST-28 — post-mortem CI gate enforcement tests.

## Compliance hooks

- CI gate: `forge audit postmortem verify` on every PR touching `docs/postmortems/`.
- Test: missing PM file for closed S0/S1 fails OPS-18 (TEST-28 TC-28-06).
- Test: action-item-light PM rejected (TEST-28 TC-28-03).

## References

- Arch §18.6, OPS-18.
- Google SRE book ch. 15 ("Postmortem Culture").
