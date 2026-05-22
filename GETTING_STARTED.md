# Getting Started with Forge

<p align="center">
  <img src="forge-logo.png" alt="Forge logo" width="600" />
</p>

> **Goal:** zero to your first `forge ship` in under 10 minutes — no prior coding experience required.

You vibe-coded something with ChatGPT, Claude, or Cursor. Forge is the AI-native framework that takes it from there — turning your AI-generated code into a production-grade, shippable product with tests, CI, security guardrails, spend controls, and an audit trail, all set up automatically.

---

## What you need before starting

| Thing | What it is | How to check you have it |
|---|---|---|
| **Node.js 18+** | Runs JavaScript on your computer | Open a terminal, type `node --version`. If you get a number ≥ 18, you're good. If "command not found", [download Node.js](https://nodejs.org) (free, 2 minutes). |
| **npm 10+** | Comes with Node.js, installs packages | `npm --version` — should show 10 or higher |
| **Git** | Tracks changes to your code | `git --version` — any version is fine. Install from [git-scm.com](https://git-scm.com) if missing. |
| **An AI coding tool** | VS Code + Copilot, Cursor, Claude Code, or similar | You probably already have this if you're vibe-coding |

> **Building a Go service?** You also need Go 1.24+. Check with `go version`. For TypeScript, React, or Next.js projects, Node.js is all you need.

> **How to open a terminal:**
> - **Windows:** press `Win + R`, type `powershell`, press Enter
> - **Mac:** press `Cmd + Space`, type `Terminal`, press Enter
> - **VS Code (any OS):** press `` Ctrl + ` `` (the backtick key, top-left of keyboard)

---

## Step 1 — Install Forge

### The recommended way (npm)

```bash
npm install -g @forgeone/cli
```

The `-g` means "install globally" — so you can type `forge` from any folder, not just one project.

Confirm it worked:

```bash
forge version
# Should print something like: forge 1.0.1
```

> **Windows only:** if `forge version` shows `0.0.0-dev` instead of a real number, npm cached an old version. Fix it with:
> ```powershell
> npm install -g @forgeone/cli-win32-x64@latest
> ```
> Then run `forge version` again — it should now show the real version.

### Try before installing (zero commitment)

```bash
npx @forgeone/cli version
```

This runs Forge once without permanently installing it. Good for a quick look.

### Other ways

| Method | Command | When to use |
|---|---|---|
| **Go install** | `go install github.com/teragrid/forge/cmd/forge@latest` | If you're already a Go developer |
| **Download binary** | [Releases page](https://github.com/teragrid/forge/releases) | No Node.js or Go installed |



---

## Step 2 — Check your environment

Run this after installing:

```bash
forge doctor
```

It checks that everything is set up correctly and tells you exactly what to fix if something is missing.

Example output when all is good:

```
✓ git            found (git version 2.44.0)
✓ .gitignore     managed block present
✓ go             1.24.x
✓ LLM provider   detected (Copilot)
```

If any line shows a red ✗ instead of a green ✓, `forge doctor` prints the exact command to fix it. Follow the instructions and run `forge doctor` again.

---

## Step 3 — Connect your AI tool

Forge uses the same AI connection your coding tool already has — you don't create a separate account or API key for Forge itself.

| If you use… | What Forge does |
|---|---|
| **VS Code + GitHub Copilot** | Uses your existing Copilot login automatically |
| **Claude Code** | Reads the `ANTHROPIC_API_KEY` environment variable you already set |
| **Cursor or Windsurf** | Reads the `OPENAI_API_KEY` you configured in Cursor |
| **No AI tool yet** | Set one env var: `ANTHROPIC_API_KEY=sk-ant-...` (get a key at [console.anthropic.com](https://console.anthropic.com)) |

If none is configured, `forge doctor` will show:


```
✗ LLM provider   none detected (FORGE-4001) — configure your IDE first
```

---

## Step 4 — Start or adopt a project

### Option A: Start fresh — TypeScript (most common for AI projects)

Most vibe-coded web apps, APIs, and chatbots land here.

```bash
forge new ts-service my-app
cd my-app
npm install
npm run dev       # starts the app — open http://localhost:3000
npm test          # runs the tests (all pass out of the box)
```

What Forge creates for you:
- A working TypeScript service with auth, database migrations, and tests already passing
- A `forge.yaml` config file
- A `.forge/` folder with instructions your AI tool reads automatically
- GitHub Actions workflows for CI and deployment
- Secret-scanning rules so API keys can't accidentally slip into commits
- AI context files (`AGENTS.md`, `.cursorrules`) so Cursor/Copilot/Claude know your project's rules

### Option B: Start fresh — Next.js app (React + Tailwind)

For web apps and dashboards.

```bash
forge new next-app my-app
cd my-app
npm install
npm run dev       # Next.js dev server at http://localhost:3000
npm test          # Vitest unit tests
```

### Option C: Start fresh — Go service

For high-performance APIs.

```bash
forge new go-service my-app
cd my-app
go run ./...      # HTTP server on :8080
go test ./...     # passes immediately
```

### Option D: Enterprise or complex project — use a TSD blueprint

For larger projects where you know your full stack upfront, use TSD (Tech Stack Decision) mode. You describe every architectural choice once, and Forge composes all the matching modules into a production-grade scaffold.

```bash
forge tsd init                           # interactive wizard → writes .forge/tsd.yml
forge new "campaign analytics service"   # reads .forge/tsd.yml automatically
```

What `forge tsd init` asks you (takes ~2 minutes):

| Question | Example answers |
|---|---|
| Project type | `saas`, `api`, `data-platform`, `marketplace` |
| Frontend framework | `nextjs-15-supabase`, `react-vite`, `none` |
| Backend language | `go`, `typescript`, `python` |
| Database | `postgresql`, `neon`, `sqlite` |
| Auth provider | `supabase`, `auth0`, `clerk`, `none` |
| Payments | `stripe`, `adyen`, `none` |
| AI layer | `openai`, `anthropic`, `none` |
| Infra | `gcp-cloud-run`, `vercel`, `fly-io`, `none` |
| Observability | `datadog`, `opentelemetry`, `none` |

The answers go into `.forge/tsd.yml`. You can edit it by hand any time. Run `forge tsd validate` to check it.

```bash
forge tsd validate       # lint the TSD file — catches unknown keys before scaffold runs
forge templates list     # browse available community blueprints and enterprise modules
```

> **Why this matters:** when `forge new` runs in TSD mode it draws from a built-in knowledge base of 172 curated entries — reference architectures, compliance patterns, and best practices from real production systems — to decide which modules to compose. You get the same architectural judgement a senior engineer brings on day one, regardless of your own experience level.

Browse available templates before scaffolding:

```bash
$ forge templates list
ID                        MODE   DESCRIPTION
enterprise-cloud-native   tsd    TSD-driven enterprise SaaS scaffold (multi-tenant, RBAC, audit-log, feature-flags)
go-cloud-native           tsd    Go + Chi + Neon + GCP cloud-native service
marketplace-platform      tsd    Next.js + Go + Adyen marketplace with multi-tenant payments
data-platform             tsd    Python + FastAPI + dbt + Metabase data platform
ts-service                classic  TypeScript + Vitest + Forge CI gates
next-app                  classic  Next.js 14, TypeScript, Tailwind CSS, App Router
go-service                classic  Go HTTP service with graceful shutdown, /healthz, /readyz
```

### Option E: You already have a project — add Forge to it

```bash
cd my-existing-project
forge init
```

Forge detects what kind of project it is and sets up accordingly. It doesn't change your existing code.
---

## Step 5 — Ship your first change safely

This is the big moment. `forge ship` is Forge's full pre-flight check — it runs six stages automatically before your code goes anywhere.

```bash
forge ship --dry-run    # preview what would happen — nothing changes
forge ship              # do the real thing
```

What happens under the hood (you don't need to do any of this manually):

| Stage | What Forge does | Why it matters |
|---|---|---|
| **1. Spec** | Checks that the code matches what you said you wanted to build | Catches "the AI went off-script" problems |
| **2. Arch** | Generates `arch.md` + `openapi.yaml`; declares API style (REST or Supabase RPC); KB-enriched LLM call | Produces the authoritative API contract before any tests or code are written |
| **3. Test** | Runs your test suite | Catches broken logic before it reaches users |
| **4. Breakdown** | Scans for obvious logic problems or missing error handling | Catches edge cases the AI glossed over |
| **5. Code** | Checks code quality and security patterns | Catches vulnerabilities |
| **6. Ship** | Runs all scanners, confirms everything is clean | Final safety check |

If any stage fails, the whole pipeline stops and Forge tells you exactly what failed and why. Fix it, run `forge ship` again.

---

## Step 6 ? Other commands you'll use regularly

You don't need to learn all of these now. Come back to this list as you need them.

```bash
forge --help                    # see all available commands
forge explain <command>         # plain-English explanation of any command

# Safety scans
forge scan all                  # run all safety checks at once
forge scan secrets              # look for API keys hardcoded in files
forge scan prompt-injection     # check if your AI app is vulnerable to manipulation

# Spend limits
forge spend set --daily 2.00 --monthly 30.00
forge spend status              # see how much you've used today and this month

# Change history
forge audit show                # see what Forge has done in this project
forge audit verify              # confirm nothing was tampered with

# When something goes wrong in production
forge incident new --id INC-001 --title "API down" --severity S1
forge incident triage INC-001   # Forge suggests what to do
forge rollback --advise         # get a recommendation on what to roll back to

# Hygiene
forge clean                     # remove AI-generated placeholder comments and dead TODOs
forge lint                      # check for missing .gitignore rules and other hygiene
```
---

## Uninstalling or switching versions

If you installed an earlier version via `go install` (it showed `0.0.0-dev`), remove it first to avoid conflicts:

**Windows:**

```powershell
Remove-Item "$(go env GOPATH)\bin\forge.exe" -ErrorAction SilentlyContinue
npm install -g @forgeone/cli
npm install -g @forgeone/cli-win32-x64@latest   # if version still shows 0.0.0-dev
forge version   # should now show the real version
```

**Mac / Linux:**

```bash
rm -f "$(go env GOPATH)/bin/forge"
npm install -g @forgeone/cli
forge version   # should now show the real version
```

To uninstall Forge completely:

```bash
npm uninstall -g @forgeone/cli
```

---

## What to do next

| Goal | Where to go |
|---|---|
| I want to see real products built with Forge | [docs/SHOWCASE.md](docs/SHOWCASE.md) |
| I want to understand every flag for every command | [docs/VERBS.md](docs/VERBS.md) |
| I want to add a custom scanner or tool | [docs/PLUGIN_AUTHORING.md](docs/PLUGIN_AUTHORING.md) |
| I want to use Forge offline / without internet | [docs/airgap.md](docs/airgap.md) |
| I want to contribute to Forge itself | [CONTRIBUTING.md](CONTRIBUTING.md) |
| I want to understand how Forge works under the hood | [docs/ARCHITECTURE_OVERVIEW.md](docs/ARCHITECTURE_OVERVIEW.md) |
| I want to see community-built plugins | [docs/COMMUNITY_PLUGINS.md](docs/COMMUNITY_PLUGINS.md) |
| I got an error code like FORGE-1234 | [docs/ERROR_CODES.md](docs/ERROR_CODES.md) |

---

*Questions? Open a [GitHub Discussion](https://github.com/teragrid/forge/discussions) or run `forge explain <command>` for help with any specific command.*
