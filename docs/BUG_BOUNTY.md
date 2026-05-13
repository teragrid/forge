# Bug Bounty Program

> Forge Community Bug Bounty — effective 2024-Q4
> See also: [SECURITY.md](SECURITY.md) · [PENTEST.md](PENTEST.md)

---

## Overview

The Forge project runs a community bug-bounty program to reward security
researchers who responsibly disclose vulnerabilities in the `forge` CLI and its
core libraries. We believe coordinated disclosure makes the ecosystem safer for
everyone.

**This is a community (non-monetary) program.** Rewards are recognition-based:
Hall-of-Fame listing, swag, and (for Critical findings) a public commendation
in the release notes.

If you are looking for paid bug-bounty programs, please check whether any
enterprise Forge distributors operate their own programmes.

---

## Submission

Submit via GitHub's private vulnerability reporting:

1. Go to <https://github.com/teragrid/forge/security>.
2. Click **Report a vulnerability**.
3. Use the template in [SECURITY.md](SECURITY.md#what-to-include).

Or email **security@forge.dev** (PGP key in `KEYS.asc`).

---

## Scope

### In scope

| Target | Example findings |
|--------|------------------|
| `forge` CLI binary (all platforms) | Arbitrary code execution, privilege escalation |
| Plugin loader + WASM sandbox | Capability escape, host filesystem access |
| `internal/fssandbox` | Path traversal, symlink attacks |
| `internal/procspawn` | Command injection, allow-list bypass |
| `internal/secretrewriter` | Secret exfiltration through logs or LLM prompts |
| `internal/audit/twokey.go` | Signature verification bypass |
| `forge scan security` rules | Rule bypass, false-negative injection |
| LLM provider integration | Prompt injection, API key leakage |
| CI/CD configuration (`.github/`) | Supply-chain attack, secret exposure |

### Out of scope

The following are **not eligible** for bounty rewards:

- Third-party plugins (report to the plugin author).
- Vulnerabilities requiring physical access to the developer's machine.
- Denial-of-service (volumetric, amplification, or resource exhaustion via
  intentionally malformed local input unless exploitable remotely).
- Social engineering / phishing of maintainers.
- Self-XSS, missing security headers on non-existent web properties.
- Findings in dependencies not yet disclosed upstream.
- Theoretical vulnerabilities without a working proof of concept.
- Issues in example apps / fixtures explicitly marked "vulnerable on purpose".

---

## Reward tiers

| Severity (CVSS v3.1) | Reward |
|----------------------|--------|
| Critical (9.0–10.0)  | Hall-of-Fame + swag + commendation in release notes + LinkedIn recommendation |
| High (7.0–8.9)       | Hall-of-Fame + swag |
| Medium (4.0–6.9)     | Hall-of-Fame |
| Low (0.1–3.9)        | Acknowledgement in CHANGELOG |
| Informational        | Thanks (no listing unless requested) |

"Swag" means a Forge-branded item (sticker pack, t-shirt) sent to the address
provided by the researcher. We reserve the right to verify the finding before
shipping.

---

## Rules

1. **Do not publicly disclose** the vulnerability before we ship a fix (or until
   90 days after initial report, whichever comes first — see [SECURITY.md](SECURITY.md)).
2. **Do not access, modify, or exfiltrate** data beyond what is strictly
   necessary to demonstrate the vulnerability.
3. **Do not disrupt** the public repository, CI pipelines, or any third-party
   services.
4. **Do not automate** large-scale scanning against forge.dev or GitHub APIs
   without prior written authorisation.
5. Act in good faith. Researchers who violate these rules are disqualified and
   may be reported to relevant authorities.

---

## Process

```
1. Submit finding via private channel
2. Maintainers acknowledge within 3 business days
3. Triage, reproduce, assign FORGE-VULN-YYYY-NNN
4. Assess severity (CVSS v3.1)
5. Develop and review fix
6. Release fix + publish advisory
7. Update Hall-of-Fame + send reward
```

---

## Hall of Fame

Security researchers who have responsibly disclosed vulnerabilities in Forge:

| Researcher | Finding | Severity | Date |
|------------|---------|----------|------|
| *(be the first!)* | — | — | — |

---

## FAQ

**Q: Can I test against the live repository or forge.dev?**  
A: Only with prior written permission from two core maintainers. Without
permission, use a local build.

**Q: What if I find a vulnerability in a third-party dependency?**  
A: Report it upstream first. If it is exploitable through forge specifically,
submit it to us as well (with the upstream report reference).

**Q: What if 90 days pass without a fix?**  
A: Contact us at security@forge.dev. We will negotiate an extension or help
coordinate an emergency release. We are committed to transparency.

**Q: Can teams submit?**  
A: Yes. One Hall-of-Fame entry per finding; credit all team members.

---

*This policy may be updated at any time. The version in the `main` branch is
authoritative.*
