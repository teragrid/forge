# ADR-025: Six-Role Self-Debate for High-Stakes LLM Decisions

**Status**: Accepted  
**Date**: 2024-01-01  
**Authors**: Forge Core Team  
**Relates to**: G-015, ADR-001, ADR-009

---

## Context

When Forge makes high-stakes LLM-assisted decisions (e.g., auto-applying a security
fix, generating an ADR, proposing a refactor), a single-shot prompt risks:

- Confirmation bias (the model agrees with the framing in the prompt)
- Missing adversarial edge cases
- Insufficient exploration of alternative designs

The "six-role self-debate" pattern forces the model to argue from six distinct
perspectives before converging on a recommendation.

---

## Decision

For decisions flagged as `high_stakes: true` in the verb manifest or explicitly
requested via `--debate`, Forge will invoke the six-role self-debate protocol:

| Role | Perspective |
|------|-------------|
| **Proposer** | Argues *for* the proposed change (benefits, rationale) |
| **Critic** | Argues *against* the change (risks, downsides) |
| **Security** | Evaluates OWASP Top-10 and supply-chain implications |
| **Reversibility** | Checks whether the change can be undone (ADR-024 contract) |
| **User Impact** | Considers end-user and operator experience |
| **Simplicity** | Challenges complexity; prefers the simplest correct solution |

The debate is structured as a multi-turn prompt sequence:

1. **Round 1**: Each role independently produces a 1-paragraph argument.
2. **Round 2**: Each role responds to the strongest opposing argument.
3. **Synthesis**: A neutral synthesiser prompt produces a final recommendation
   with explicit tradeoffs noted.

The synthesised output is included in the command's JSON output under
`"debate_summary"` and stored in `.forge/debates/<timestamp>-<verb>.json`.

---

## Implementation Notes

- Implemented in `internal/prompttemplates/` as `SixRoleDebateTemplate`.
- Triggered when `llmprovider.Request.HighStakes == true`.
- Each role maps to a named system-prompt section; all six completions are
  batched via `llmprovider.Batch()` (ADR-045 batch API).
- Token cost for a full debate is approximately 6× a standard prompt; the
  token ledger (`internal/tokenledger`) records the cost under
  `"debate"` category.
- The CI cost gate (G-049) applies to debate runs.

---

## Consequences

**Positive**:
- Higher-quality decisions for irreversible or security-sensitive changes.
- Explicit audit trail of considered alternatives.
- Reduces blind spots in LLM-generated code.

**Negative**:
- ~6× token cost for high-stakes decisions.
- Adds latency (mitigated by batching).
- Requires disciplined flagging of `high_stakes` in verb manifests.

---

## Alternatives Considered

| Alternative | Why rejected |
|-------------|-------------|
| Single adversarial prompt | Less thorough than six perspectives |
| Human review gate | Blocks automation; defeats purpose of AI-assisted workflow |
| Constitutional AI self-critique | Requires provider support; less portable |

---

*See also: ADR-014 (resilience), ADR-024 (reversibility), G-045 (batch API)*
