# ADR-013 — Secret-scanning engine

- **Status:** Proposed
- **Tracker:** ARCH-DEC-13
- **Spec/Arch anchor:** Spec §4 Repo Hygiene Layer (`.gitleaks.toml`), Spec §16.5.4 #12, Arch §15
- **Decision date:** TBD
- **Deciders:** Security engineer
- **Consulted:** DevSecOps, core engineering

## Context

Forge must prevent secrets from being committed and detect them in pre-existing history. The engine must:

- Be fast (a `forge ship` should add ≤ 2 s for incremental scans on a typical repo).
- Have a maintained rule corpus.
- Allow Forge-specific rules (e.g. `FORGE_*` env-var redaction patterns, telemetry-payload regex).
- Permit allowlisting with **mandatory `review-by` metadata + expiry**.
- Be packageable as a single binary (no Python/runtime dependency).

## Decision

Forge will use **gitleaks** as the secret-scanning engine, embedded as a vendored Go call-out (gitleaks is a Go binary/library — imported as a module where the API surface allows, otherwise shelled out to a vendored static binary). On top of gitleaks's stock rules, Forge ships a **Forge rule pack** (`.gitleaks.toml` fragment) and an **allowlist schema** that enforces `review-by` ownership + expiry.

### Rule-pack schema

```toml
# rules/forge.toml — distributed as a hygiene fragment (per ADR-011)
[[rules]]
id = "forge-openai-key"
description = "OpenAI API key (Forge-shipped)"
regex = '''sk-[A-Za-z0-9]{32,}'''
tags = ["openai", "key", "forge"]
[rules.allowlist]
# fragment-level allowlist may not waive the rule — only repo-level allowlists may.

[[rules]]
id = "forge-stripe-live-key"
description = "Stripe live secret key"
regex = '''sk_live_[A-Za-z0-9]{16,}'''
tags = ["stripe", "key", "forge", "high"]
```

### Repo-level allowlist (`.gitleaks-allowlist.yml`)

```yaml
api_version: forge.sh/v1
kind: SecretAllowlist
spec:
  entries:
    - id: legacy-test-key
      rule: forge-openai-key
      file: tests/fixtures/old.json
      line: 42
      reason: "Test fixture string, not a live key."
      review_by: alice@example.com
      expires_at: "2026-12-31"
      ticket: SEC-128
```

### Enforcement

- `forge scan secrets` runs gitleaks + Forge rule pack on the working tree and the diff.
- `forge ship` blocks on any unresolved finding; allowlist entries past `expires_at` count as findings.
- `forge audit allowlist verify` lints the allowlist file: every entry has `review_by`, `reason`, `expires_at` (≤ 12 months from `created_at`), and a tracking `ticket`.

## Alternatives considered

### Option A — trufflehog (rejected)

Pros: deeper entropy + verifier checks (live API validation).
Cons: live-API verification is a privacy hazard for a CLI; slower; rule format less stable.

### Option B — In-house engine from scratch (rejected)

Pros: zero external dep.
Cons: rule corpus maintenance is a full-time job; reinvents what gitleaks already maintains.

### Option C — GitHub Secret Scanning (vendor-side) (rejected)

Pros: zero local cost.
Cons: GitHub-specific; doesn't help local pre-commit; doesn't cover non-GitHub forges (GitLab self-hosted etc.).

## Consequences

### Positive

- Fast, mature engine with active rule maintenance.
- Forge rule pack ships as a hygiene fragment → composes cleanly with user customisations (per ADR-012).
- Allowlist `review-by` + expiry prevents permanent-allow drift.

### Negative / accepted trade-offs

- gitleaks license (MIT) compatible with Apache-2.0 (per ADR-008).
- Vendored engine adds binary size; mitigated by feature-flagging out for the smallest install profile.
- Rule false-positives are inevitable; allowlist + telemetry-driven rule tuning is the recurring OPS burden.

### Follow-ups created

- DEV-M0-27 — vendor gitleaks + Go wrapper.
- DEV-M0-28 — Forge rule pack v1.
- DEV-M1-39-related — `forge audit allowlist verify` verb.
- TEST-21 — secrets fixture corpus.
- TEST-24 — allowlist-expiry regression test.

## Compliance hooks

- CI gate: every PR runs `forge scan secrets`.
- CI gate: `forge audit allowlist verify` runs nightly + on every PR touching the allowlist.
- Test: an expired allowlist entry causes a finding to fire (TEST-24).
- Test: a fresh fixture leak is detected end-to-end (TEST-21).

## References

- Spec §4 Repo Hygiene Layer, §16.5.4 #12.
- gitleaks: <https://github.com/gitleaks/gitleaks>.
