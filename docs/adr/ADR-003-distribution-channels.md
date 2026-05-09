# ADR-003 — Distribution channels

- **Status:** Proposed
- **Tracker:** ARCH-DEC-03
- **Spec/Arch anchor:** Arch §13 ADR-003, Spec §3 (install UX), §16.5.4 (universal gates)
- **Decision date:** TBD
- **Deciders:** DevSecOps
- **Consulted:** Founder, community WG

## Context

Forge must be installable in ≤ 60 s on a clean developer laptop with one command, on Linux, macOS, and Windows. CI runners must also install it deterministically. The team has zero infra budget pre-1.0 and wants no central package registry of its own.

## Decision

Forge will be distributed via **GitHub Releases** as the single source of truth, with thin per-platform wrappers:

| Platform | Channel | Delivery |
|----------|---------|----------|
| macOS | Homebrew tap `forge-sh/tap` | `brew install forge-sh/tap/forge` |
| Windows | Scoop bucket `forge-sh/scoop-bucket` + winget manifest | `scoop install forge` / `winget install ForgeSH.Forge` |
| Linux | curl-bash installer + `.deb`/`.rpm` artefacts in the GH release | `curl -fsSL https://get.forge.sh \| sh` |
| Cross-platform CI | Pre-built tarball download by version pin | direct GH Releases URL |
| Devcontainer / Nix | Community-maintained derivations (NOT first-party) | best-effort |

Every artefact is **sigstore-signed** (per ADR-022) and SHA-256-pinned in the tap/bucket manifests.

### Install matrix (acceptance artefact for ARCH-DEC-03)

```
                 brew  scoop  winget  curl-sh  deb  rpm  msi  tarball
macos arm64       ✓                                                ✓
macos x64         ✓                                                ✓
windows x64              ✓     ✓               ─    ─    ✓        ✓
windows arm64            ✓     ✓               ─    ─    ✓        ✓
linux x64                              ✓        ✓    ✓             ✓
linux arm64                            ✓        ✓    ✓             ✓
```

## Alternatives considered

### Option A — Self-hosted package server + private CDN (rejected)

Pros: full control over upgrade semantics.
Cons: infra cost + uptime burden pre-1.0; trust establishment harder than piggy-backing on GH + sigstore.

### Option B — `go install` only (rejected)

Pros: trivial for Go devs.
Cons: requires Go toolchain on user machine; violates "single static binary" UX; no per-OS package-manager surface.

### Option C — Container image only (rejected)

Pros: hermetic.
Cons: developer ergonomics poor for a CLI run dozens of times per day; cold-start budget violated.

## Consequences

### Positive

- Zero infra to operate; GH SLAs cover availability.
- Each channel has community-standard upgrade paths (`brew upgrade`, `scoop update`, `winget upgrade`).
- `forge upgrade` self-update reads the GH Releases API → single source of truth.

### Negative / accepted trade-offs

- The curl-pipe-shell installer is a known supply-chain wart; mitigated by SHA-256 pin + sigstore verification embedded in the script + a documented `--no-pipe` flag for paranoid users.
- Winget acceptance can take days; M0 ships the manifest but does not block on Microsoft's review.
- Linux package signing keys must be rotated — covered by ADR-022.

### Follow-ups created

- DEV-M0-29 — Homebrew tap repo + auto-bump GH Action.
- DEV-M0-30 — Scoop bucket repo + auto-bump.
- DEV-M0-31 — `get.forge.sh` installer script + sigstore verify.
- OPS-03 — quarterly install-matrix smoke test on every supported OS.

## Compliance hooks

- CI gate: install-matrix smoke test on tagged releases (OPS-03).
- CI gate: every release artefact carries a sigstore signature; absence fails the release workflow.
- Test: `forge --version` on each channel returns the released semver byte-identically (TEST-15).

## References

- Arch §13 ADR-003.
- sigstore: <https://www.sigstore.dev/>.
