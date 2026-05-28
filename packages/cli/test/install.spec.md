# Spec — `@forgeone/cli` install regression suite

## Status Summary

- Lifecycle: In Progress
- Version Scope: PATCH (install regression checks only; do not bump faster unless the package contract changes)
- Owner: release
- Last Updated: 2026-05-28
- Checkpoint Progress: 0/5

### Completed Tasks

- [ ] S1 — Global install
- [ ] S2 — Local install
- [ ] S3 — npx (no prior install)
- [ ] S4 — Optional-dependency skipped
- [ ] S5 — postinstall hook

> Version: 1.0.0 · Owner: release · Lives in CI: `.github/workflows/install-test.yml`

This spec defines the contract the npm package must satisfy on every
supported environment. A failure of any scenario below is a release-blocker.

## Scope

The published `@forgeone/cli` package, plus its five platform-specific
optional dependencies:

- `@forgeone/cli-linux-x64`
- `@forgeone/cli-linux-arm64`
- `@forgeone/cli-darwin-x64`
- `@forgeone/cli-darwin-arm64`
- `@forgeone/cli-win32-x64`

## Environments

| OS              | Arch  | Node versions covered |
|-----------------|-------|------------------------|
| Ubuntu (latest) | x64   | 18, 20, 22             |
| macOS (latest)  | arm64 | 18, 20, 22             |
| Windows (latest)| x64   | 18, 20, 22             |

(Linux arm64 and darwin x64 are covered by `optionalDependencies`
resolution; runtime smoke-test is currently OS-native only.)

## Scenarios (must all pass)

### S1 — Global install
```sh
npm install -g @forgeone/cli@<version>
forge version
forge --help
```
**Acceptance:** every command exits 0; `forge version` prints `v<version>`.
**Forbidden:** any output containing `'true' is not recognized` (Windows
`cmd.exe` shell-syntax leak), `EACCES`, `permission denied`, or
`Could not find the @forgeone/cli-* package`.

### S2 — Local install
```sh
mkdir t && cd t && npm init -y
npm install @forgeone/cli@<version>
npx forge version
```
**Acceptance:** install completes without `npm error`; `npx forge` resolves
to the project-local binary (not a globally cached one).

### S3 — npx (no prior install)
```sh
npx --yes @forgeone/cli@<version> version
```
**Acceptance:** prints version and exits 0 in a clean environment.

### S4 — Optional-dependency skipped
Simulated with `npm install --omit=optional @forgeone/cli`.
**Acceptance:** `forge version` exits 1 with the message:
> Could not find the @forgeone/cli-* package.
> This usually means the optional dependency was skipped during install.
> Try: npm install --include=optional

(Friendly, actionable error — never an unhandled `ENOENT`.)

### S5 — postinstall hook
**Requirement:** the package MUST NOT ship a `postinstall` script that
relies on POSIX shell syntax (`2>/dev/null`, `||`, `true`). Such scripts
break Windows `cmd.exe` and trigger the `'true' is not recognized` error
seen in v1.0.0.
**Verification:** `jq -e '.scripts.postinstall // empty | length == 0'
packages/cli/package.json` (CI gate).

## Non-goals

- Does not test against Node ≤ 16 (declared unsupported in `engines`).
- Does not test offline / air-gapped install (covered by `airgap.md`).
- Does not test arm64 Linux runtime (no public arm64 GitHub runners yet).

## Cadence

- Daily at 04:00 UTC against the `latest` dist-tag.
- On-demand via `workflow_dispatch` for any specific version (release
  rehearsal).
- Should be moved to per-PR if the package contents change (i.e. anything
  inside `packages/cli/` is touched).
