# Installing Forge

Forge ships as a single, statically-linked binary with no runtime dependencies.
Pick whichever path suits your setup.

---

## Option A — npm (recommended)

```bash
npm install -g @forgeone/cli
forge version
```

Requires Node.js 18+. The correct pre-built binary for your platform is pulled
automatically via optional dependencies.

---

## Option B — `go install`

If you have **Go 1.24+** installed, compile and install directly from source:

```bash
go install github.com/teragrid/forge/cmd/forge@latest
```

Make sure `$(go env GOPATH)/bin` is on your `$PATH`. Confirm:

```bash
forge version
# forge 1.0.1 (go1.26.3 ...)
```

---

## Option C — Download a Pre-built Binary

Pre-built binaries for every supported platform are attached to each
[GitHub Release](https://github.com/teragrid/forge/releases).

**Linux / macOS:**

```bash
# Replace linux-amd64 with darwin-arm64, linux-arm64, etc. as needed
curl -Lo forge https://github.com/teragrid/forge/releases/latest/download/forge-linux-amd64
chmod +x forge
sudo mv forge /usr/local/bin/
forge version
```

**Windows (PowerShell):**

```powershell
Invoke-WebRequest -Uri https://github.com/teragrid/forge/releases/latest/download/forge-windows-amd64.exe `
  -OutFile forge.exe
# Move forge.exe to a directory on your PATH, e.g.:
Move-Item forge.exe "$env:USERPROFILE\bin\forge.exe"
forge version
```

---

## Option C — Homebrew (macOS / Linux)

> **Coming soon.** The `teragrid/homebrew-tap` repository is not yet published.
> Use Option A (npm) or Option B (binary) in the meantime.

---

## Option D — Scoop (Windows)

> **Coming soon.** The `teragrid/scoop-forge` bucket is not yet published.
> Use Option A (npm) or Option B (binary) in the meantime.

> **Note:** Winget support is on the roadmap for the 1.x release.

---

## Connecting Forge to your LLM

Forge **never stores your API keys**.  Instead it reads the credential that your
IDE already manages:

| IDE / tool | What Forge reads |
|------------|-----------------|
| **VS Code + GitHub Copilot** | Copilot token from the VS Code credential store |
| **Claude Code** | `ANTHROPIC_API_KEY` env var or the Claude Code session token |
| **Cursor / Windsurf** | `OPENAI_API_KEY` env var or the Cursor credential store |
| **CI / GitHub Actions** | `OPENAI_API_KEY` / `ANTHROPIC_API_KEY` as repository secrets |

After configuring your IDE, verify everything is wired up:

```bash
forge doctor
```

Expected output (all green):

```
✓ git            found (git version 2.44.0)
✓ go             1.24.x
✓ .gitignore     managed block present
✓ LLM provider   copilot (VS Code credential store)
```

If any item is amber or red, `forge doctor` prints the exact remediation step
and the `FORGE-XXXX` error code to look up in
[`docs/ERROR_CODES.md`](docs/ERROR_CODES.md).

---

## Verifying the Binary Signature (optional, recommended for CI)

Release binaries are signed via sigstore (planned for v0.3.0+).  Until then,
verify the SHA-256 checksum published alongside each release:

```bash
# Example (Linux amd64)
sha256sum -c forge-linux-amd64.sha256
```

---

## Uninstalling

### npm global install

```bash
npm uninstall -g @forgeone/cli
```

### `go install` binary

**Windows (PowerShell):**
```powershell
Remove-Item "$(go env GOPATH)\bin\forge.exe" -ErrorAction SilentlyContinue
```

**macOS / Linux:**
```bash
rm -f "$(go env GOPATH)/bin/forge"
```

### Homebrew

```bash
brew uninstall forge
```

### Scoop

```powershell
scoop uninstall forge
```

---

## Switching from `go install` to npm

If you installed an earlier version via `go install` and want to switch to the
npm-managed release, remove the old binary first to avoid PATH conflicts:

**Windows:**
```powershell
# 1. Remove old binary
Remove-Item "$(go env GOPATH)\bin\forge.exe" -ErrorAction SilentlyContinue

# 2. Confirm it's gone (should return nothing)
Get-Command forge -ErrorAction SilentlyContinue

# 3. Install the npm release
npm install -g @forgeone/cli

# 4. If forge still shows 0.0.0-dev, npm kept the old platform package.
#    Force-install it explicitly:
npm install -g @forgeone/cli-win32-x64@latest

# 5. Verify
forge version   # forge 1.0.1 (...)
```

**macOS / Linux:**
```bash
# 1. Remove old binary
rm -f "$(go env GOPATH)/bin/forge"

# 2. Confirm it's gone
which forge   # should return nothing

# 3. Install the npm release
npm install -g @forgeone/cli

# 4. Verify
forge version   # forge 1.0.1 (...)
```
