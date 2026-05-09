# ADR-009 — Error-code namespacing

- **Status:** Proposed
- **Tracker:** ARCH-DEC-09
- **Spec/Arch anchor:** Arch §11 (error model), Spec §16.5.4 (universal gates)
- **Decision date:** TBD
- **Deciders:** Core engineer
- **Consulted:** Quality WG, DevSecOps

## Context

`FORGE-NNNN` codes already appear in spec, drill reports, and post-mortems. They must be:

- Globally unique and **never reused** (even after a feature is removed).
- Greppable in source.
- Range-allocated per subsystem so on-call can route a code to a team in seconds.
- Documentable centrally (`docs/errors/FORGE-NNNN.md`).

## Decision

Forge error codes follow the format **`FORGE-<RANGE><NN>`** where `<RANGE>` is a two-digit subsystem prefix and `<NN>` is the per-subsystem sequence (00–99). Codes are 4 digits total → up to 99 subsystems × 100 codes = 9 900 unique codes (sufficient through 1.0; expansion plan defined below).

### Reserved ranges (initial allocation)

| Range | Subsystem | Owner |
|-------|-----------|-------|
| 00 | Reserved (success / no-error sentinels) | core |
| 01 | CLI / argument parsing | core |
| 02 | Config loader | core |
| 03 | Workspace / FS | core |
| 04 | Spec parser | core |
| 05 | Plugin loader / WASM host | plugin WG |
| 06 | LLM gateway | core |
| 07 | LLM cache | core |
| 08 | Scan engine | quality WG |
| 09 | Hygiene engine | core |
| 10 | Secret scanner | security WG |
| 11 | Audit ledger | security WG |
| 12 | Migration runner | core |
| 13 | Deploy adapter | core |
| 14 | Storage adapter | core |
| 15 | Telemetry | DevSecOps |
| 16 | Learning loop | quality WG |
| 17 | Eval harness | quality WG |
| 18 | Registry resolver | community WG |
| 19 | Upgrade / self-update | DevSecOps |
| 20 | Ship orchestrator | core |
| 21 | Plan engine | core |
| 22 | Capability runtime | core |
| 23 | Resilience runtime (timeouts, breakers) | core |
| 24 | Chaos-drill harness | quality WG |
| 25 | Failure-register verifier | core |
| 26 | Reversibility / `forge undo` | core |
| 90–98 | Reserved for future subsystems | unallocated |
| 99 | Reserved for tests / examples (never shipped to users) | core |

### Lifecycle rules

- Allocating a new code: PR adds a row to `docs/errors/index.md` AND a one-page `docs/errors/FORGE-NNNN.md` describing cause, user action, and `Fixes:` linkage. Lint enforces both.
- **Codes are never reused.** A removed feature's codes go to `status: retired` in the index. New uses get fresh numbers.
- A code's user-facing message can change (locale, clarity); its identity cannot.
- **Expansion plan:** when a subsystem exhausts its 100 codes, allocate a sibling range (e.g. scan engine 08 → 28). The lint allows multi-range subsystems via the index file.

## Alternatives considered

### Option A — Free-form strings (`E_BAD_CONFIG`) (rejected)

Pros: self-documenting.
Cons: collisions; renames lose grep history; harder for non-English speakers to search docs.

### Option B — UUIDs (rejected)

Pros: trivially unique.
Cons: ungreppable in incident threads; no human-route signal.

### Option C — `FORGE-<subsystem>-<seq>` text form (`FORGE-SCAN-007`) (rejected)

Pros: more readable.
Cons: subsystem rename → user-facing breakage; numeric form is friendlier to log aggregators and i18n strings.

## Consequences

### Positive

- One regex (`FORGE-\d{4}`) finds every code in source and logs.
- Range-based routing is automatic for on-call (ADR-019).
- Doc-per-code policy gives a stable URL surface for incident response.

### Negative / accepted trade-offs

- 4 digits will eventually exhaust; expansion-range plan handles this.
- Range allocation is a small bureaucratic surface; mitigated by lint + an index file PR review.

### Follow-ups created

- DEV-M0-21 — error-code lint rule (`forge audit error-codes verify`).
- DEV-M0-22 — `docs/errors/` scaffold + per-code template.
- TEST-08 — error-code reuse regression test (greps history).

## Compliance hooks

- CI gate: lint rejects a new `FORGE-NNNN` literal in source unless `docs/errors/FORGE-NNNN.md` exists in the same PR.
- CI gate: lint rejects re-use of a `retired` code.
- Test: every code in source has an entry in `docs/errors/index.md` (TEST-08).

## References

- Arch §11.
- PostgreSQL error-code allocation (prior art): <https://www.postgresql.org/docs/current/errcodes-appendix.html>.
