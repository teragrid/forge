# Security Policy

## Supported versions

forge is pre-1.0 (M0 Bootstrap). No version is GA. Once 1.0 ships, the latest minor will receive security patches per the policy in [adr/ADR-022-governance.md](../adr/ADR-022-governance.md).

## Reporting a vulnerability

**Please do not open a public GitHub issue for security reports.**

Use GitHub's private vulnerability reporting:

1. Go to the [Security tab](https://github.com/teragrid/forge/security) of this repo.
2. Click **Report a vulnerability**.
3. Provide a clear reproducer and impact assessment.

We aim to acknowledge within **3 business days** and to ship a fix or mitigation within **30 days** for High/Critical findings (CVSS ≥ 7.0).

Coordinated disclosure: we will credit reporters in the release notes unless anonymity is requested.

## Scope

In-scope:

- The `forge` CLI binary and any first-party packages under `cmd/`, `internal/`, `pkg/`.
- The plugin runtime (per [ADR-002](../adr/ADR-002-plugin-runtime.md)).
- CI/CD configuration in `.github/workflows/`.

Out of scope:

- Third-party plugins (report to the plugin author).
- Test fixtures / example apps explicitly marked "vulnerable on purpose" in their README.
