# ADR-004 — Registry storage

- **Status:** Proposed
- **Tracker:** ARCH-DEC-04
- **Spec/Arch anchor:** Arch §3 C4, §13 ADR-004, Spec §6 (plugin registry)
- **Decision date:** TBD
- **Deciders:** Community WG
- **Consulted:** Security WG, DevSecOps

## Context

The plugin registry must:

- Be mirrorable by anyone (no single point of failure / vendor lock-in).
- Support cryptographic verification of every plugin manifest.
- Avoid running a database pre-1.0.
- Allow community to PR new plugins via a familiar workflow (Git PR).
- Resolve a plugin name → signed artefact URL in ≤ 200 ms p95.

## Decision

The Forge plugin registry will be a **signed JSON index stored in a public Git repository (`forge-sh/registry`)**, served via a CDN-fronted static origin (`registry.forge.sh`). The index is regenerated from per-plugin manifest files on every merge to `main` by a CI job and re-signed with the registry trust-root key (sigstore, two-custodian rotation per ADR-022).

### Manifest schema (acceptance artefact)

```yaml
# plugins/<name>/manifest.yml
api_version: forge.sh/v1
kind: Plugin
metadata:
  name: rls-scan
  version: 1.4.0
  authors: ["alice@example.com"]
  license: Apache-2.0
  homepage: https://github.com/...
  description: Postgres RLS rule scanner.
spec:
  wit_world: forge:plugin@1
  capabilities:
    - fs.read:workspace
    - fs.write:workspace/.forge/cache
  artifact:
    url: https://github.com/.../releases/download/v1.4.0/rls-scan.wasm
    sha256: <hex>
    sigstore_bundle_url: https://github.com/.../releases/download/v1.4.0/rls-scan.wasm.sigstore
  forge_compat: ">=0.10.0,<2.0.0"
  tier: T3
```

The aggregated index `index.json` is `{plugins: [{name, latest, manifest_url, sha256, signature}]}` — also signed.

## Alternatives considered

### Option A — Hosted database (Postgres + REST API) (rejected)

Pros: rich querying, fast updates.
Cons: infra to operate + secure; lock-in; mirror story poor.

### Option B — OCI registry (e.g. ghcr.io) (rejected)

Pros: existing signed-artefact infra (cosign + OCI).
Cons: discovery (search/list) requires custom indexing layer anyway; mirror story is per-vendor.

### Option C — Git-only index repo with no CDN (rejected)

Pros: simplest; analogous to Cargo's index design.
Cons: 200 ms p95 SLO at scale requires CDN.

## Consequences

### Positive

- Anyone can `git clone forge-sh/registry` and run a private mirror with one DNS change.
- All updates are PRs — review trail is intrinsic.
- sigstore signatures bind manifest → artefact → identity.

### Negative / accepted trade-offs

- Search queries beyond exact-name require client-side index download (a few hundred KB by 1.0).
- Mass updates (e.g. CVE pull) are bottlenecked on Git merges → mitigated by automation script.
- Trust-root rotation is a high-stakes ceremony (handled by ADR-022).

### Follow-ups created

- DEV-M2-01 — registry repo + index generator.
- DEV-M2-02 — `forge plugin install <name>` resolver.
- TEST-12 — manifest schema validation.

## Compliance hooks

- CI gate: every PR to `forge-sh/registry` runs schema lint + sigstore verification of the referenced artefact.
- Test: corrupt or unsigned artefact rejected by `forge plugin install` (TEST-12).
- Periodic: nightly job verifies all `index.json` entries still resolve (OPS-09).

## References

- Arch §3 C4, §13 ADR-004.
- Cargo index design: <https://doc.rust-lang.org/cargo/reference/registries.html>.
- sigstore bundle spec: <https://github.com/sigstore/protobuf-specs>.
