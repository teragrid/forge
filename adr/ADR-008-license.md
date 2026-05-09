# ADR-008 — License

- **Status:** Proposed (pending legal-advisor review before LICENSE file is committed)
- **Tracker:** ARCH-DEC-08
- **Spec/Arch anchor:** Spec §8 Q22, Arch §15 (legal threats)
- **Decision date:** TBD
- **Deciders:** Founder + Legal advisor
- **Consulted:** Core engineering, community WG

## Context

Forge will be an open-source CLI used inside enterprises and by individual developers. The license must:

- Be OSI-approved (table-stakes for enterprise adoption).
- Be compatible with the dependencies already chosen (`wazero` — Apache-2.0; `cobra`/`viper` — Apache-2.0; OpenTelemetry-Go — Apache-2.0).
- Provide an explicit patent grant (defensive against patent trolls).
- Permit commercial use, including SaaS resale, without surprise obligations.
- Allow the founders to optionally relicense future versions if a sustainable business model requires it.

## Decision

Forge core (the CLI binary, T1 + T2 code in this repo) will be released under **Apache License 2.0**. The plugin registry repo (`forge-sh/registry`) and individual plugins set their own licenses; the registry CI rejects manifests whose license is not OSI-approved.

A `NOTICE` file accompanies `LICENSE` and tracks third-party attribution per Apache-2.0 §4(d).

A `CONTRIBUTING.md` file requires either:

- The Linux Foundation **DCO** (Developer Certificate of Origin) sign-off, OR
- A signed **CLA** for contributions ≥ 100 lines from organisations.

The DCO route is the default to lower contributor friction; the CLA route is reserved for cases where downstream relicensing flexibility matters.

### Files to create (post-acceptance)

- `LICENSE` — full Apache-2.0 text.
- `NOTICE` — initial attribution lines for `wazero`, `cobra`, `viper`, OpenTelemetry-Go, etc.
- `CONTRIBUTING.md` — DCO + CLA workflow.
- `.github/workflows/dco.yml` — DCO sign-off check.

## Alternatives considered

### Option A — MIT (rejected)

Pros: shortest, most familiar.
Cons: no explicit patent grant; weaker for enterprise legal review; ASF-style attribution mismatch with Apache-licensed deps.

### Option B — BSL 1.1 → Apache-2.0 (4-year change date) (rejected)

Pros: closes the SaaS-resale loophole during the funding-sensitive years.
Cons: not OSI-approved; chills early enterprise adoption; community signal-cost too high pre-1.0.

### Option C — AGPL-3.0 (rejected)

Pros: copyleft over the network → forces SaaS forks to publish.
Cons: most enterprise legal teams blanket-ban AGPL; catastrophic for adoption velocity.

### Option D — Elastic License v2 (rejected)

Pros: explicitly blocks competing managed services.
Cons: not OSI-approved; same enterprise-block problem as AGPL.

## Consequences

### Positive

- Maximum compatibility with existing OSS ecosystem.
- Patent grant covers Forge's novel scan + ship algorithms.
- DCO keeps contribution friction low; CLA available when needed.

### Negative / accepted trade-offs

- A competitor could fork Forge into a hosted service. Mitigation: brand + community + plugin registry network effects, not legal moats.
- Future relicensing of unilateral copyrights is possible only on contributor-by-contributor basis without CLA — accepted as a deliberate trade-off for early-community goodwill.

### Follow-ups created

- DEV-M0-03 — `LICENSE` + `NOTICE` files committed (gated on legal-advisor sign-off).
- DEV-M0-05 — `CONTRIBUTING.md` with DCO instructions.
- DEV-M0-06 — DCO check workflow.
- OPS-04 — quarterly `NOTICE` regen via `go-licenses report ./...`.

## Compliance hooks

- CI gate: every PR commit must carry `Signed-off-by:` (DCO).
- CI gate: `go-licenses check ./...` rejects dependencies with non-Apache-compatible licenses.
- Test: LICENSE file SHA-256 matches the canonical Apache-2.0 (drift detection).

## References

- Apache-2.0 text: <https://www.apache.org/licenses/LICENSE-2.0>.
- DCO: <https://developercertificate.org/>.
- OSI license list: <https://opensource.org/licenses>.
