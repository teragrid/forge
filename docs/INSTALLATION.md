# Installing Forge

Forge ships as a single, statically-linked binary with no runtime dependencies.
Pick whichever path suits your setup.

---

## Option A — Download a Pre-built Binary

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

```bash
# If installed via go install
rm "$(go env GOPATH)/bin/forge"

# If installed via Homebrew
brew uninstall forge

# If installed via Scoop
scoop uninstall forge
```
