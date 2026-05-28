# Forge Release Guide

End-to-end process for cutting a release: Go binaries via goreleaser, GitHub Release, and npm distribution of `@forge/cli`.

---

## Prerequisites

### One-time setup (per developer machine)

```sh
make tools          # installs goreleaser, golangci-lint, govulncheck, goimports, gotestsum
make hooks          # wires .githooks/pre-push quality gate
```

### One-time setup (GitHub repository secrets)

Go to **Settings → Secrets and variables → Actions** and add:

| Secret | Value |
|---|---|
| `GORELEASER_GITHUB_TOKEN` | Fine-grained PAT — repo `teragrid/forge`, permission `contents: write` |
| `NPM_TOKEN` | npm automation token with publish access to the `@forge` org |

### One-time setup (npm org)

```sh
npm login
# Create the @forge org on npmjs.com if it doesn't exist yet:
# https://www.npmjs.com/org/create
```

---

## Release flow

### 1 — Prepare the release

```sh
# Make sure main is clean and all tests pass
git checkout main
git pull
go test ./...

# Update CHANGELOG.md — add a ## [x.y.z] section with the changes
# Update version references in README if any
git add CHANGELOG.md
git commit -m "chore: prepare release vX.Y.Z"
git push
```

### 1.1 — Version scope gate (required before tagging)

Do not bump to the next minor/major by default. Pick the **smallest valid** semver scope:

| If the release contains | Bump |
|---|---|
| Bug fixes, hardening, docs, internal refactors, non-breaking behavior corrections | `PATCH` |
| Backward-compatible additive capability (new verb/flag/output field that does not break existing usage) | `MINOR` |
| Intentional breaking contract change | `MAJOR` |

Required evidence before tagging:

1. Write a short "Version Scope Decision" in the release PR/body:
  - `Chosen bump`: PATCH/MINOR/MAJOR
  - `Why not smaller`: one line
  - `Breaking impact`: none / described
2. Confirm the relevant feature spec(s) have a top `Status Summary` block with:
  - `Lifecycle`
  - `Version Scope` (+ rationale)
  - `Last Updated`
  - `Checkpoint Progress`
3. If uncertain between `PATCH` and `MINOR`, ship `PATCH` first (or cut `-rc.N`).

### 2 — Tag and push

```sh
make tag VERSION=1.0.0
# Inspect the tag before pushing:
git show v1.0.0

git push origin v1.0.0
```

Pushing the tag triggers `.github/workflows/release.yml` automatically.

### 3 — CI does the rest

The release workflow runs in this order:

```
goreleaser job
  ├─ go build  × 5 platforms  (linux-x64, linux-arm64, darwin-x64, darwin-arm64, win32-x64)
  ├─ creates GitHub Release with:
  │    ├─ forge_X.Y.Z_linux_amd64.tar.gz
  │    ├─ forge_X.Y.Z_darwin_arm64.tar.gz
  │    ├─ forge_X.Y.Z_windows_amd64.zip
  │    ├─ ... (all 5 archives)
  │    ├─ forge_X.Y.Z_checksums.txt   (SHA-256)
  │    └─ SBOM (CycloneDX JSON)
  └─ uploads dist/ as workflow artifact

npm-publish job  (runs after goreleaser)
  ├─ downloads dist/ artifact
  ├─ scripts/npm-publish.sh  (stamps versions, copies binaries into packages/)
  ├─ npm publish @forge/cli-linux-x64
  ├─ npm publish @forge/cli-linux-arm64
  ├─ npm publish @forge/cli-darwin-x64
  ├─ npm publish @forge/cli-darwin-arm64
  ├─ npm publish @forge/cli-win32-x64
  └─ npm publish @forge/cli          ← wrapper, published last
```

Monitor at: `https://github.com/teragrid/forge/actions`

### 4 — Verify the release

```sh
# Check GitHub Release page
open https://github.com/teragrid/forge/releases/latest

# Verify npm packages (allow ~2 min for registry propagation)
npm info @forge/cli version
npm info @forge/cli-linux-x64 version

# Smoke test via npx (no install needed)
npx @forge/cli@1.0.0 --version
npx @forge/cli@1.0.0 new smoke-test --template ts-service
```

---

## Pre-release (rc / beta)

Tag with a pre-release suffix — goreleaser marks it as a GitHub pre-release automatically:

```sh
make tag VERSION=1.0.0-rc.1
git push origin v1.0.0-rc.1

# npm tag it as 'next' so it doesn't become the default install:
# (edit release.yml npm publish steps to add --tag next for pre-release tags)
npx @forge/cli@next --version
```

---

## Local snapshot build (no GitHub token needed)

Test the full goreleaser pipeline locally without publishing anything:

```sh
make release-snapshot
# Produces dist/forge_linux_amd64/forge, dist/forge_darwin_arm64/forge, etc.

# Optionally stage npm packages to inspect them:
make npm-stage VERSION=0.0.0-snapshot
# Copies binaries into packages/ and stamps version — does NOT publish
```

---

## Manual npm publish (emergency / hotfix)

If CI fails after goreleaser already created the GitHub Release:

```sh
# 1. Download the release archives from GitHub
VERSION=1.0.0
for platform in linux_amd64 linux_arm64 darwin_amd64 darwin_arm64 windows_amd64; do
  gh release download "v${VERSION}" --pattern "forge_${VERSION}_${platform}*" --dir dist/
done

# 2. Extract binaries into dist/ subdirs that npm-publish.sh expects
# (goreleaser already does this layout; if downloading raw archives, extract manually)

# 3. Stage + publish
bash scripts/npm-publish.sh --version "$VERSION" --dry-run false

npm publish packages/cli-linux-x64  --access public
npm publish packages/cli-linux-arm64 --access public
npm publish packages/cli-darwin-x64  --access public
npm publish packages/cli-darwin-arm64 --access public
npm publish packages/cli-win32-x64   --access public
npm publish packages/cli             --access public
```

---

## Versioning policy

Forge follows **Semantic Versioning** (`MAJOR.MINOR.PATCH`):

| Change | Version bump |
|---|---|
| Breaking CLI flag / output change | MAJOR |
| New command or template | MINOR |
| Bug fix, security patch | PATCH |
| Pre-release candidate | `x.y.z-rc.N` |

Default release behavior: **prefer PATCH unless a larger scope is clearly justified**.

All six npm packages (`@forge/cli` + 5 platform packages) are always published at the same version and kept in lockstep.

---

## Rollback

If a bad release reaches npm:

```sh
# Deprecate the bad version (preferred over unpublish — keeps lock files working)
npm deprecate @forge/cli@1.0.1 "Broken release — use 1.0.0 or 1.0.2"
npm deprecate @forge/cli-linux-x64@1.0.1 "Broken release — use 1.0.0 or 1.0.2"
# ... repeat for all 5 platform packages

# Then cut a patch release immediately:
make tag VERSION=1.0.2
git push origin v1.0.2
```

---

## Troubleshooting

| Symptom | Likely cause | Fix |
|---|---|---|
| `goreleaser: tag already exists` | Re-running after partial failure | Delete tag locally + remotely, re-tag |
| `npm 403 Forbidden` | NPM_TOKEN expired or wrong scope | Rotate token in npm → update GitHub secret |
| `Could not find the @forge/cli-linux-x64 package` | Optional dep skipped | `npm install --include=optional` |
| goreleaser `git is dirty` | Uncommitted changes | `git stash` or commit before tagging |
| npm package already exists at version | Tried to republish same version | npm does not allow overwriting — bump version |

---

## Key files reference

| File | Purpose |
|---|---|
| [`.goreleaser.yml`](../.goreleaser.yml) | Build matrix, archive format, changelog, GitHub Release config |
| [`.github/workflows/release.yml`](../.github/workflows/release.yml) | CI: goreleaser → npm publish |
| [`scripts/npm-publish.sh`](../scripts/npm-publish.sh) | Stamps versions + copies binaries into `packages/` |
| [`packages/cli/bin/forge.js`](../packages/cli/bin/forge.js) | Platform detection wrapper (the only JS that ships) |
| `packages/cli-*/package.json` | One per platform; `os`+`cpu` fields restrict npm install to matching platforms |
