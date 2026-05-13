# Forge Distribution Guide

How to publish Forge CLI to the three supported distribution channels:
**npm registry**, **Homebrew tap**, and **Scoop bucket**.

All three channels are automated by the `.github/workflows/release.yml` pipeline that
fires when a `v*` tag is pushed. This document covers the one-time setup you must
complete before the first release, the automated flow, and how to perform manual or
emergency publishes.

---

## Table of contents

1. [npm registry](#1-npm-registry)
2. [Homebrew tap](#2-homebrew-tap)
3. [Scoop bucket](#3-scoop-bucket)
4. [Verification checklist](#4-verification-checklist)
5. [Troubleshooting](#5-troubleshooting)

---

## 1. npm registry

Forge ships six npm packages: one platform-specific binary package per supported
platform and one wrapper (`@forge/cli`) that selects the right binary at install time.

| Package | Platform |
|---|---|
| `@forge/cli-linux-x64` | Linux x86-64 |
| `@forge/cli-linux-arm64` | Linux ARM64 (Graviton, Pi) |
| `@forge/cli-darwin-x64` | macOS Intel |
| `@forge/cli-darwin-arm64` | macOS Apple Silicon |
| `@forge/cli-win32-x64` | Windows x86-64 |
| `@forge/cli` | Wrapper — auto-selects the right binary above |

### 1.1 One-time setup

#### Create the npm org

```sh
# Sign in to your personal npm account first
npm login

# Create the @forge org at https://npmjs.com/org/create
# org name: forge
# visibility: public (required for free tier)
```

#### Add the `NPM_TOKEN` secret to GitHub

1. Generate an npm **Automation** token (Settings → Access Tokens → Generate New Token → Automation).
   Automation tokens bypass 2FA requirements in CI.
2. Go to your GitHub repo → **Settings → Secrets and variables → Actions → New repository secret**.
3. Name: `NPM_TOKEN`, value: the token above.

The release workflow already reads this secret:

```yaml
# .github/workflows/release.yml (excerpt)
- name: Publish npm packages
  env:
    NODE_AUTH_TOKEN: ${{ secrets.NPM_TOKEN }}
  run: bash scripts/npm-publish.sh
```

#### Verify your setup locally

```sh
npm whoami          # should print your npm username
npm org ls forge    # should list you as owner
```

### 1.2 Automated publish (normal release)

Pushing a `v*` tag triggers the full pipeline automatically:

```sh
# Prepare and tag
git checkout main && git pull
make tag VERSION=1.2.0
git push origin v1.2.0
```

The CI pipeline:
1. `goreleaser` cross-compiles for all 5 platforms, creates a GitHub Release with
   archives and `forge_X.Y.Z_checksums.txt`.
2. `scripts/npm-publish.sh` stamps the version into each `packages/*/package.json`,
   copies the matching binary, and publishes all six packages in dependency order
   (platform packages first, then the `@forge/cli` wrapper).

Monitor progress at: `https://github.com/teragrid/forge/actions`

### 1.3 Manual / emergency publish

If CI failed partway through and you need to re-publish manually:

```sh
# Authenticate
npm login   # or export NODE_AUTH_TOKEN=<token>

# Stamp the version (replaces "0.1.0" placeholders)
VERSION=1.2.0 bash scripts/npm-publish.sh --dry-run   # preview
VERSION=1.2.0 bash scripts/npm-publish.sh              # actual publish

# Or publish a single package manually
cd packages/cli-darwin-arm64
npm version 1.2.0 --no-git-tag-version
npm publish --provenance --access public
```

> **Provenance attestation** (`--provenance`) links each published package to the
> GitHub Actions run that produced it. This is required for SLSA level 2+.
> It only works inside GitHub Actions — skip `--provenance` for local emergency
> publishes, but note the provenance gap in your release notes.

### 1.4 Pre-release tags

```sh
make tag VERSION=1.2.0-rc.1
git push origin v1.2.0-rc.1
```

goreleaser marks the GitHub Release as a **pre-release** automatically. Publish to
npm with the `next` dist-tag so it doesn't become the default install:

```sh
# In scripts/npm-publish.sh, add --tag next when IS_PRERELEASE=true
npm publish --provenance --access public --tag next
```

Users opt in with `npm install -g @forge/cli@next`.

### 1.5 Two-factor authentication

Always enable 2FA on the npm `@forge` org. Automation tokens bypass 2FA for
CI/CD (by design), but human logins require the second factor. Store the TOTP
seed in 1Password or Bitwarden in addition to the authenticator app.

---

## 2. Homebrew tap

Users on macOS and Linux install via:

```sh
brew tap teragrid/tap
brew install forge
```

### 2.1 One-time setup — create the tap repository

1. Create a new **public** GitHub repository named `teragrid/homebrew-tap`.
2. Create the `Formula/` directory and commit an initial `forge.rb` formula stub:

```ruby
# Formula/forge.rb
class Forge < Formula
  desc "AI-native developer framework — scaffold, lint, scan, ship"
  homepage "https://github.com/teragrid/forge"
  version "0.1.0"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/teragrid/forge/releases/download/v#{version}/forge_#{version}_darwin_arm64.tar.gz"
      sha256 "PLACEHOLDER_DARWIN_ARM64"
    else
      url "https://github.com/teragrid/forge/releases/download/v#{version}/forge_#{version}_darwin_amd64.tar.gz"
      sha256 "PLACEHOLDER_DARWIN_AMD64"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/teragrid/forge/releases/download/v#{version}/forge_#{version}_linux_arm64.tar.gz"
      sha256 "PLACEHOLDER_LINUX_ARM64"
    else
      url "https://github.com/teragrid/forge/releases/download/v#{version}/forge_#{version}_linux_amd64.tar.gz"
      sha256 "PLACEHOLDER_LINUX_AMD64"
    end
  end

  def install
    bin.install "forge"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/forge version")
  end
end
```

3. Add a `HOMEBREW_TAP_TOKEN` GitHub secret to `teragrid/forge` (fine-grained PAT
   with `contents: write` on `teragrid/homebrew-tap`).

### 2.2 Automated formula update via goreleaser

Add a `brews:` block to `.goreleaser.yml` so goreleaser auto-updates the formula
SHA256 values and version after each release:

```yaml
# .goreleaser.yml — add after the `release:` block
brews:
  - name: forge
    repository:
      owner: teragrid
      name: homebrew-tap
      token: "{{ .Env.HOMEBREW_TAP_TOKEN }}"
    directory: Formula
    homepage: "https://github.com/teragrid/forge"
    description: "AI-native developer framework — scaffold, lint, scan, ship"
    license: "Apache-2.0"
    test: |
      system "#{bin}/forge", "version"
    install: |
      bin.install "forge"
```

goreleaser computes the SHA256 from the release archives and pushes a commit to
`teragrid/homebrew-tap` automatically.

### 2.3 Manual SHA256 update

If you need to update the formula manually after a release:

```sh
# Download the checksums file
curl -sL https://github.com/teragrid/forge/releases/download/v1.2.0/forge_1.2.0_checksums.txt

# Example output:
# abc123...  forge_1.2.0_darwin_arm64.tar.gz
# def456...  forge_1.2.0_darwin_amd64.tar.gz
# ...

# Edit Formula/forge.rb in teragrid/homebrew-tap:
# Replace PLACEHOLDER_* with the actual SHA256 values
# Update version "0.1.0" → "1.2.0"
git -C homebrew-tap add Formula/forge.rb
git -C homebrew-tap commit -m "forge 1.2.0"
git -C homebrew-tap push
```

### 2.4 User install commands

```sh
# Install
brew tap teragrid/tap
brew install forge

# Upgrade to latest
brew update && brew upgrade forge

# Check installed version
forge version
```

---

## 3. Scoop bucket

Users on Windows install via:

```sh
scoop bucket add teragrid https://github.com/teragrid/scoop-bucket
scoop install forge
```

### 3.1 One-time setup — create the bucket repository

1. Create a new **public** GitHub repository named `teragrid/scoop-bucket`.
2. Create the `bucket/` directory and add the initial `forge.json` manifest:

```json
{
  "version": "0.1.0",
  "description": "AI-native developer framework — scaffold, lint, scan, ship",
  "homepage": "https://github.com/teragrid/forge",
  "license": "Apache-2.0",
  "architecture": {
    "64bit": {
      "url": "https://github.com/teragrid/forge/releases/download/v0.1.0/forge_0.1.0_windows_amd64.zip",
      "hash": "PLACEHOLDER_WINDOWS_AMD64"
    }
  },
  "bin": "forge.exe",
  "checkver": {
    "github": "https://github.com/teragrid/forge"
  },
  "autoupdate": {
    "architecture": {
      "64bit": {
        "url": "https://github.com/teragrid/forge/releases/download/v$version/forge_$version_windows_amd64.zip",
        "hash": {
          "url": "https://github.com/teragrid/forge/releases/download/v$version/forge_$version_checksums.txt",
          "find": "([a-f0-9]{64})\\s+forge_$version_windows_amd64\\.zip"
        }
      }
    }
  }
}
```

3. Add a `SCOOP_BUCKET_TOKEN` GitHub secret to `teragrid/forge` (fine-grained PAT
   with `contents: write` on `teragrid/scoop-bucket`).

### 3.2 Automated manifest update via goreleaser

Add a `scoops:` block to `.goreleaser.yml`:

```yaml
# .goreleaser.yml — add after the `brews:` block
scoops:
  - name: forge
    repository:
      owner: teragrid
      name: scoop-bucket
      token: "{{ .Env.SCOOP_BUCKET_TOKEN }}"
    directory: bucket
    homepage: "https://github.com/teragrid/forge"
    description: "AI-native developer framework — scaffold, lint, scan, ship"
    license: "Apache-2.0"
```

goreleaser updates `bucket/forge.json` with the new version and SHA256 automatically.

### 3.3 Manual manifest update

```sh
# Get the Windows SHA256 from the checksums file
curl -sL https://github.com/teragrid/forge/releases/download/v1.2.0/forge_1.2.0_checksums.txt \
  | grep windows_amd64

# Edit bucket/forge.json in teragrid/scoop-bucket:
# - Update "version": "1.2.0"
# - Update "url" to .../v1.2.0/forge_1.2.0_windows_amd64.zip
# - Update "hash" to the SHA256 from checksums

git -C scoop-bucket add bucket/forge.json
git -C scoop-bucket commit -m "forge 1.2.0"
git -C scoop-bucket push
```

### 3.4 Scoop autoupdate

The `autoupdate` block in the manifest lets Scoop auto-detect new releases by
checking the GitHub releases page and recomputing hashes. Run `scoop checkver forge`
inside the bucket repo to validate that detection works:

```sh
git clone https://github.com/teragrid/scoop-bucket
cd scoop-bucket
# requires Scoop installed on Windows
scoop checkver forge -u   # -u updates the manifest in place
```

### 3.5 User install commands

```sh
# Install (run in PowerShell)
scoop bucket add teragrid https://github.com/teragrid/scoop-bucket
scoop install forge

# Upgrade
scoop update forge

# Check version
forge version
```

---

## 4. Verification checklist

After every release, verify all three channels within **15 minutes** of the
GitHub Release appearing:

### npm

```sh
# Allow 2–3 minutes for registry propagation
npm info @forge/cli version
npm info @forge/cli-darwin-arm64 version

# Smoke test without installing globally
npx @forge/cli@1.2.0 version
npx @forge/cli@1.2.0 new smoke-test --template ts-service --dry-run
rm -rf smoke-test
```

### Homebrew (macOS/Linux)

```sh
brew update
brew info teragrid/tap/forge  # should show new version

# Fresh install test in a throw-away prefix
brew install --build-from-source teragrid/tap/forge 2>&1 | tail -5
forge version
brew uninstall forge
```

### Scoop (Windows PowerShell)

```powershell
scoop update
scoop info forge           # should show new version

scoop install forge
forge version
scoop uninstall forge
```

### Combined smoke test (all platforms, automated)

The release workflow includes a post-publish smoke-test job matrix. Review its
status on the Actions page; look for the `smoke-test` job run after `npm-publish`.

---

## 5. Troubleshooting

### npm: `403 Forbidden` on publish

- Check that `NPM_TOKEN` is an **Automation** token (not a read-only token).
- Verify the `@forge` org exists and your account has `owner` role:
  `npm org ls forge`
- Make sure the package name matches the org exactly — `@forge/cli`, not `@Forge/cli`.

### npm: `E409 Conflict — cannot publish over existing version`

You cannot re-publish the same version. Bump to a patch version (`1.2.1`) and
re-release, or use `npm deprecate @forge/cli@1.2.0 "bad release"` to warn users.

### Homebrew: `SHA256 mismatch`

Re-download the archive and recompute:

```sh
curl -sLO https://github.com/teragrid/forge/releases/download/v1.2.0/forge_1.2.0_darwin_arm64.tar.gz
sha256sum forge_1.2.0_darwin_arm64.tar.gz
```

Update `forge.rb` with the correct value and push to `homebrew-tap`.

### Homebrew: formula not visible after push

Homebrew caches tap metadata. Users run `brew update` to refresh. For CI/CD
testing use `HOMEBREW_NO_AUTO_UPDATE=1 brew install teragrid/tap/forge`.

### Scoop: `Hash mismatch`

Re-run `scoop checkver forge -u` inside the `scoop-bucket` repo — it will
recompute the hash from the actual release asset and update the manifest.

### goreleaser: `brews/scoops block token not set`

Both `HOMEBREW_TAP_TOKEN` and `SCOOP_BUCKET_TOKEN` must be added to the
`teragrid/forge` repository secrets (not the tap/bucket repos). Verify with:

```sh
gh secret list --repo teragrid/forge
```

### Release workflow failed halfway through

If goreleaser succeeded but npm-publish failed, the GitHub Release already exists.
You cannot re-run the goreleaser job without deleting the release tag first.
Instead:
1. Fix the npm-publish script or secrets.
2. Re-run only the `npm-publish` job from the Actions UI (GitHub supports
   re-running individual failed jobs).
3. If the entire workflow needs a re-run, delete the tag and release:
   ```sh
   git tag -d v1.2.0
   git push origin :refs/tags/v1.2.0
   gh release delete v1.2.0 --yes
   ```
   Then re-tag and push.

---

## Appendix A — Required GitHub secrets summary

| Secret | Used by | Description |
|---|---|---|
| `GORELEASER_GITHUB_TOKEN` | goreleaser | Fine-grained PAT, `teragrid/forge`, `contents: write` |
| `NPM_TOKEN` | npm-publish job | npm Automation token, `@forge` org publish access |
| `HOMEBREW_TAP_TOKEN` | goreleaser brews block | Fine-grained PAT, `teragrid/homebrew-tap`, `contents: write` |
| `SCOOP_BUCKET_TOKEN` | goreleaser scoops block | Fine-grained PAT, `teragrid/scoop-bucket`, `contents: write` |

## Appendix B — Repositories to create

| Repository | Visibility | Purpose |
|---|---|---|
| `teragrid/homebrew-tap` | Public | Homebrew formula for `forge` |
| `teragrid/scoop-bucket` | Public | Scoop manifest for `forge` |

## Appendix C — Install command quick reference

```sh
# npm (all platforms)
npm install -g @forge/cli
npx @forge/cli@latest version

# Homebrew (macOS / Linux)
brew tap teragrid/tap && brew install forge

# Scoop (Windows)
scoop bucket add teragrid https://github.com/teragrid/scoop-bucket
scoop install forge

# Direct download
# https://github.com/teragrid/forge/releases/latest
```
