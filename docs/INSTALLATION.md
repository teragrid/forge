# Installing Forge

Forge is one small program. You install it once, then it works everywhere — Mac, Windows, Linux. No background services, no daemons, no signup.

Once it's installed, Forge does the rest of the heavy lifting: it scaffolds production-grade projects, runs quality gates before every push, enforces AI spending limits, and keeps a tamper-proof audit trail — automatically. Getting Forge installed is the only manual step.

> **New to Forge?** Finish this page, then go to [GETTING_STARTED.md](../GETTING_STARTED.md) for the full walkthrough.

---

## Which install option is for me?

| If you... | Pick |
|---|---|
| Already have Node.js (most vibe-coders) | **Option A — npm** |
| Want to try Forge without installing it permanently | **Option B — npx** |
| Are a Go developer | **Option C — go install** |
| Don't have Node.js or Go, and don't want to install them | **Option D — download a binary** |
| Use a Mac with Homebrew | **Option E — Homebrew** (coming soon) |
| Need to install on a machine with no internet | See [airgap.md](airgap.md) |

---

## Option A — npm (recommended)

This is the easiest path for almost everyone.

```bash
npm install -g @forgeone/cli
forge version
```

**Requires:** Node.js 18 or newer ([download here](https://nodejs.org)).

The `-g` flag installs Forge globally so you can run `forge` from any folder. npm automatically downloads the correct binary for your operating system — no compilation needed.

> **Windows quirk:** if `forge version` shows `0.0.0-dev` after install, npm cached an old version. Run this once to fix:
> ```powershell
> npm install -g @forgeone/cli-win32-x64@latest
> ```

---

## Option B — npx (try without installing)

```bash
npx @forgeone/cli version
```

This runs Forge once and discards it. Great for kicking the tyres before committing.

---

## Option C — `go install`

For developers who already have **Go 1.24+** installed.

```bash
go install github.com/teragrid/forge/cmd/forge@latest
```

Make sure `$(go env GOPATH)/bin` is on your `PATH`. Confirm with:

```bash
forge version
# forge 1.0.1 (go1.26.3 ...)
```

---

## Option D — Download a pre-built binary

If you don't have Node.js or Go, grab a binary directly from the [Releases page](https://github.com/teragrid/forge/releases).

Pick the file that matches your computer:

| Your computer | Download this file |
|---|---|
| Windows (any modern PC) | `forge-windows-amd64.exe` |
| Mac with M1/M2/M3/M4 chip | `forge-darwin-arm64` |
| Mac with Intel chip | `forge-darwin-amd64` |
| Linux (most desktops/servers) | `forge-linux-amd64` |
| Linux on Raspberry Pi or ARM server | `forge-linux-arm64` |

**Mac / Linux:**

```bash
# Example for Linux amd64 — substitute your file
curl -Lo forge https://github.com/teragrid/forge/releases/latest/download/forge-linux-amd64
chmod +x forge
sudo mv forge /usr/local/bin/
forge version
```

**Windows (PowerShell):**

```powershell
Invoke-WebRequest -Uri https://github.com/teragrid/forge/releases/latest/download/forge-windows-amd64.exe `
  -OutFile forge.exe
Move-Item forge.exe "$env:USERPROFILE\bin\forge.exe"
# Make sure "$env:USERPROFILE\bin" is in your PATH
forge version
```

---

## Option E — Homebrew (Mac/Linux)

> **Coming soon.** The Homebrew tap is not yet published. Use Option A (npm) until then.

## Option F — Scoop / Winget (Windows)

> **Coming soon.** Use Option A (npm) or Option D (binary) until then.

---

## Connect Forge to your AI

Forge does NOT store your AI API keys. Instead it reads the key your IDE or coding tool already uses.

| If you use... | Forge reads |
|---|---|
| **VS Code + GitHub Copilot** | Your Copilot login (stored by VS Code) |
| **Claude Code** | The `ANTHROPIC_API_KEY` environment variable |
| **Cursor or Windsurf** | The `OPENAI_API_KEY` environment variable |
| **GitHub Actions / CI** | Repository secrets exposed as env vars (`OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, etc.) |
| **Nothing yet** | Set `ANTHROPIC_API_KEY=sk-ant-...` — get a key at [console.anthropic.com](https://console.anthropic.com) |

After configuring, check everything is wired up:

```bash
forge doctor
```

Expected output (all green checks):

```
v git            found (git version 2.44.0)
v .gitignore     managed block present
v LLM provider   copilot (VS Code credential store)
```

If anything is missing, `forge doctor` prints the exact fix and an error code (like `FORGE-4001`) you can look up in [ERROR_CODES.md](ERROR_CODES.md).

---

## Verify the download (optional)

For paranoid setups and CI pipelines, every release ships with SHA-256 checksums:

```bash
sha256sum -c forge-linux-amd64.sha256
```

Sigstore signature verification is on the roadmap.

---

## Uninstall

| Installed via | Uninstall command |
|---|---|
| **npm** | `npm uninstall -g @forgeone/cli` |
| **go install** (Mac/Linux) | `rm -f "$(go env GOPATH)/bin/forge"` |
| **go install** (Windows) | `Remove-Item "$(go env GOPATH)\bin\forge.exe"` |
| **Homebrew** | `brew uninstall forge` |
| **Scoop** | `scoop uninstall forge` |
| **Binary download** | Delete the `forge` file you copied to your PATH |

---

## Switching from go install to npm

If your `forge version` shows `0.0.0-dev`, you have an old go-installed copy on your PATH and need to remove it first.

**Windows:**

```powershell
Remove-Item "$(go env GOPATH)\bin\forge.exe" -ErrorAction SilentlyContinue
Get-Command forge -ErrorAction SilentlyContinue   # should print nothing
npm install -g @forgeone/cli
npm install -g @forgeone/cli-win32-x64@latest      # if still 0.0.0-dev
forge version
```

**Mac / Linux:**

```bash
rm -f "$(go env GOPATH)/bin/forge"
which forge        # should print nothing
npm install -g @forgeone/cli
forge version
```

---

## Stuck?

- Run `forge doctor` — it diagnoses most environment issues and tells you the fix.
- Look up your error code in [ERROR_CODES.md](ERROR_CODES.md).
- Ask in [GitHub Discussions](https://github.com/teragrid/forge/discussions).