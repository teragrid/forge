# ADR-022 — Two-key enforcement

- **Status:** Proposed
- **Tracker:** ARCH-DEC-22
- **Spec/Arch anchor:** Arch §18.4
- **Decision date:** TBD
- **Deciders:** Security engineer + DevSecOps
- **Consulted:** Founder, core engineering

## Context

Irreversible incident-time operations (force-push to `main`, release-key rotation, gate-bypass merges, registry trust-root rotation) historically cause silent harm when authorised by a single actor under stress. The "two-key" rule mitigates by requiring an independent second human approval, recorded for audit.

## Decision

Forge enforces a two-key rule across **four** distinct surfaces, each with its own mechanism:

### 1. Branch protection (`main` + release branches)

- Force-push: **disabled** for everyone, including admins (admin overrides logged separately to a dedicated audit channel).
- Direct push: **disabled**; merges via PR only.
- Merge requires **≥ 2 approving reviews** from CODEOWNERS, including ≥ 1 Maintainer-tier per Spec §16.5.8.

### 2. Sigstore release signing

- Release artefacts (per ADR-003) signed by **two custodian identities** (`releases-a@forge.sh` + `releases-b@forge.sh`).
- The release workflow assembles a sigstore bundle requiring both signatures; `cosign verify --certificate-identity-list` checks both at install time.
- Custodian roles are held by **distinct human Maintainers** (no shared service account).

### 3. `gate-bypass` PR check (per ADR-017)

- A PR labelled `gate-bypass` requires **≥ 2 Maintainer approvals** AND a `Bypass-rationale:` line in the PR description AND a linked tracking incident issue.
- The bot (`gate-bypass-approval.yml`) fails the merge until all three are satisfied.
- The bypass is automatically logged in the next post-mortem (per ADR-020 §8) — OPS-18 enforces.

### 4. Registry trust-root rotation (per ADR-004)

- The `forge-sh/registry` trust-root key rotation is a documented ceremony in `docs/security/trust-root-rotation.md`:
  1. New keypair generated on an air-gapped device.
  2. Two custodians attend (one from Security WG, one from Core).
  3. Both sign the rotation manifest with their personal sigstore identities.
  4. New root published to a notary log (sigstore Rekor) before old root revoked.
  5. Recorded in `docs/security/key-history.md`.
- Annual rotation cadence; emergency rotation triggered by compromise indicators.

## Acceptance scope

- Pre-1.0: surfaces 1, 3 hard-enforced; surfaces 2, 4 documented but operational ramp-up by M2/M3.
- Post-1.0: all four surfaces hard-enforced.

## Alternatives considered

### Option A — Single-key with audit log (rejected)

Pros: faster.
Cons: no real-time prevention; prior-art incidents (most "stress force-push" disasters) all had post-hoc logs that didn't help.

### Option B — Hardware security module (HSM) co-sign (rejected pre-1.0)

Pros: very strong.
Cons: cost + ceremony complexity premature; sigstore custodianship is sufficient pre-1.0.

### Option C — N-of-M threshold signatures (e.g. FROST / Shamir) (rejected pre-1.0)

Pros: cryptographically elegant.
Cons: tooling immature; revisit for 1.0+.

## Consequences

### Positive

- Stress-time mistakes by a single actor cannot ship.
- Audit trail is intrinsic to each surface (PR review history, sigstore bundle, post-mortem bypass log).
- Aligns with industry norms (e.g. `cosign` two-custodian releases).

### Negative / accepted trade-offs

- Slower incident-time merges by minutes; accepted as the price of avoiding catastrophic single-actor errors.
- Requires ≥ 2 active Maintainers at any moment; covered by ADR-019 escalation chain.

### Follow-ups created

- DEV-M3-20 — two-key enforcement implementation.
- OPS-related — quarterly key-rotation drill.
- ADR-017 dependency: `gate-bypass-approval.yml` bot.
- ADR-004 dependency: trust-root ceremony doc.

## Compliance hooks

- CI gate: `gate-bypass-approval.yml` (TEST-DEV-M3-20 TC-20-01).
- Test: single-signer release blocked (TEST-DEV-M3-20 TC-20-03).
- Test: force-push to `main` rejected (TEST-DEV-M3-20 TC-20-02).
- Audit: quarterly review of GitHub branch-protection settings against this ADR.

## References

- Arch §18.4.
- sigstore co-sign: <https://docs.sigstore.dev/cosign/multiple_signatures/>.
- "The Two-Person Rule" (NIST SP 800-53 AC-3(2)).
