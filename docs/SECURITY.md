# Security Policy

> ADR-018 governs the full vulnerability-intake and disclosure workflow.
> ADR-022 governs the two-key enforcement requirement for high-impact operations.

## Supported versions

| Version | Status        | Security patches |
|---------|---------------|------------------|
| 1.x     | ✅ GA         | Latest minor only |
| 0.x     | ⛔ Deprecated | None (upgrade required) |

## Private vulnerability reporting (intake)

**Do not open a public GitHub issue for security reports.**

### Option A — GitHub private reporting (preferred)

1. Go to the [Security tab](https://github.com/teragrid/forge/security).
2. Click **Report a vulnerability**.
3. Fill in the template (see below).
4. GitHub creates a private advisory — only maintainers and the reporter see it.

### Option B — Encrypted e-mail

Send to **security@forge.dev** (PGP key ID `0xDEADBEEF`, fingerprint in `KEYS.asc`).

### What to include

```
Title:       Short description (e.g. "Path traversal in forge scaffold")
CVSS score:  (your estimate, or leave blank)
Affected:    forge version(s), OS, Go version
Reproducer:  Minimal steps or PoC (attach, do not paste keys/tokens)
Impact:      What an attacker can achieve
Mitigations: Anything that reduces exploitability
```

## Response timeline (SLA)

| Milestone                        | Target    |
|----------------------------------|-----------|
| Acknowledgement                  | 3 business days |
| Initial triage + severity rating | 7 calendar days |
| Fix or mitigation shipped        | 30 days for CVSS ≥ 7.0 (High/Critical) |
| Fix or mitigation shipped        | 90 days for CVSS < 7.0 (Medium/Low) |
| Public disclosure                | After fix ships, coordinated with reporter |

We follow a **90-day coordinated disclosure** model (aligned with Google Project Zero).
If the reporter requires disclosure earlier, we will negotiate in good faith.

## Bug-bounty program

Forge operates a community bug-bounty program. See [BUG_BOUNTY.md](BUG_BOUNTY.md) for scope,
reward tiers, and submission instructions.

## Severity rating

We use CVSS v3.1 base scores:

| Score range | Severity | Response target |
|-------------|----------|-----------------|
| 9.0–10.0    | Critical | 30 days         |
| 7.0–8.9     | High     | 30 days         |
| 4.0–6.9     | Medium   | 90 days         |
| 0.1–3.9     | Low      | 90 days         |

## CVE assignment

For confirmed vulnerabilities, Forge will request a CVE from MITRE via the
GitHub Security Advisory workflow. Tracking IDs use the format `FORGE-VULN-YYYY-NNN`.

## Scope

### In scope

- The `forge` CLI binary and any first-party packages under `cmd/`, `internal/`.
- The plugin runtime and WASM sandbox (per [ADR-002](../adr/ADR-002-plugin-runtime.md)).
- CI/CD configuration in `.github/workflows/`.
- The `forge scan security` engine and rule set.
- The two-key enforcement library (`internal/audit/twokey.go`, per ADR-022).
- LLM provider integration (`internal/llmprovider/`) — prompt injection, key exposure.

### Out of scope

- Third-party plugins (report to the plugin author directly).
- Test fixtures / example apps explicitly marked "vulnerable on purpose" in their README.
- Denial-of-service attacks requiring >10 Gbps sustained traffic.
- Social engineering of maintainers.
- Vulnerabilities in dependencies not yet reported upstream.

## Security mitigations in place

| Control | Implementation |
|---------|----------------|
| Path traversal | `internal/fssandbox` allow/deny list on all user-supplied paths |
| Subprocess injection | `internal/procspawn` allow-list; no shell=true |
| Secret redaction | `internal/secretrewriter` strips keys from logs and LLM prompts |
| Two-key enforcement | `internal/audit/twokey.go` for high-impact ops (ADR-022) |
| Plugin sandbox | WASM/wazero with capability-gated imports (ADR-002) |
| OWASP Top 10 | `forge scan security` CI gate enforced on every PR |
| Token budget | `internal/llmbudget` prevents runaway LLM spend |

## Pentest

Forge undergoes periodic external penetration testing. See [PENTEST.md](PENTEST.md) for scope,
methodology, and findings process. The last published pentest summary is in
`docs/pentest/` (redacted version).

## Disclosure policy

1. Reporter submits via private channel.
2. Maintainers acknowledge within 3 business days.
3. Maintainers triage, assign `FORGE-VULN-YYYY-NNN`, rate severity.
4. Fix developed in a private fork or branch.
5. Advisory draft shared with reporter for review ≥ 7 days before publication.
6. Fix released; public advisory published; CVE requested.
7. Reporter credited in the advisory (unless anonymity requested).

## Credits

Security researchers who have responsibly disclosed vulnerabilities are credited
in `CHANGELOG.md` and the GitHub security advisory. Thank you for keeping forge safe.
