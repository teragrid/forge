# Getting Started with Forge

<p align="center">
  <img src="forge-logo.png" alt="Forge logo" width="600" />
</p>

> **Goal:** zero to your first `forge ship` in under 10 minutes.

---

## Prerequisites

| Requirement | Version | Check |
|-------------|---------|-------|
| Node.js | ≥18 | `node --version` |
| npm | ≥10 | `npm --version` |
| Git | any recent | `git --version` |
| An IDE with an LLM configured | VS Code + GitHub Copilot, Claude Code, or Cursor | see §3 below |

> **Building a Go service?** Also install Go ≥01.24. TypeScript/React projects need only Node.js.

Forge **reads your IDE's LLM configuration** — it never stores credentials itself.
If no IDE is detected, `forge ship` exits with `FORGE-4001` and tells you exactly
what to configure first.

---

## Step 1 — Install Forge

### Option A: npm (recommended — no Go required)

```bash
npm install -g @forge/cli
```

Verify:

```bash
forge version
# forge 0.1.0  (or the release you installed)
```

No Go installation needed. The right pre-compiled binary for your platform is
pulled automatically via npm’s optional dependencies.

### Option B: `npx` (zero-install, try before committing)

```bash
npx @forge/cli@latest new my-app --template ts-service
```

### Option C: Download a release binary

Pre-built binaries for Linux, macOS, and Windows (amd64 + arm64) are attached to
every [GitHub release](https://github.com/teragrid/forge/releases).

```bash
# Linux / macOS example
curl -Lo forge https://github.com/teragrid/forge/releases/latest/download/forge-linux-amd64
chmod +x forge
sudo mv forge /usr/local/bin/
```

---

## Step 2 — Confirm your environment

```bash
forge doctor
```

Expected output (all green):

```
✓ git            found (git version 2.44.0)
✓ .gitignore     managed block present
✓ gitleaks       found (v8.x)
✓ go             1.24.x
```

If any check is amber or red, `forge doctor` prints the exact remediation step.

---

## Step 3 — Configure your LLM (if not already done)

Forge bridges to your IDE's LLM connection. You do not set credentials in Forge.

| IDE / tool | What Forge reads |
|------------|-----------------|
| **VS Code + GitHub Copilot** | Copilot token from the VS Code credential store |
| **Claude Code** | `ANTHROPIC_API_KEY` or the Claude Code session token |
| **Cursor / Windsurf** | `OPENAI_API_KEY` env var or the Cursor credential store |
| **CI / GitHub Actions** | Secrets vault via env vars (`OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, etc.) |

If none of the above is configured, `forge doctor` will show:

```
✗ LLM provider   none detected (FORGE-4001) — configure your IDE first
```

---

## Step 4 — Scaffold your first project

### TypeScript service (recommended for most projects)

```bash
forge new ts-service my-app
cd my-app
npm install
npm run dev       # starts the service with tsx watch
npm test          # runs vitest (already passing out of the box)
```

What you get out of the box:
- `src/modules/auth/` — auth service, controller, types, and tests (vitest)
- `migrations/20260101000000_init.sql` — workspaces + users + audit_log + RLS
- `forge.config.ts` — typed project config (tenancy, auth, DB, observability)
- `.github/workflows/ci.yml` — typecheck + lint + test + security scan
- `.github/workflows/deploy-staging.yml` + `deploy-production.yml`
- `.forge/conventions.json` and `.forge/hygiene.yml` — LLM instruction files
- `.gitleaks.toml` — secret scanning baseline
- `AGENTS.md`, `CLAUDE.md`, `.cursorrules`, `.windsurfrules` — AI agent context

### Go HTTP service

```bash
forge new go-service my-app
cd my-app
go run ./...      # HTTP server on :8080
go test ./...     # passes immediately
```

What you get:
- `main.go` — production HTTP server with graceful shutdown, `/healthz`, `/readyz`
- `main_test.go` — passing tests against `httptest.Server`
- `docker-compose.yml` — local Postgres
- Same `.forge/`, `.gitignore`, `.gitleaks.toml`, CI workflow structure

### Initialise an existing directory

```bash
cd my-existing-project
forge init                       # auto-detects template from package.json / go.mod
forge init --template ts-service # or specify explicitly
```

---

## Step 5 — Make a change with `forge ship`

```bash
# Dry-run (safe; default)
forge ship --description "add hello-world handler"

# When you're ready to apply
forge ship --description "add hello-world handler" --dry-run=false
```

`forge ship` runs through five stages automatically:

1. **Spec** — writes `.forge/specs/<slug>/spec.md` with intent + acceptance criteria.
2. **Test** — generates failing tests from the spec (committed before code).
3. **Breakdown** — produces `.forge/specs/<slug>/tasks.md`.
4. **Code** — iterates until tests are green.
5. **Ship** — runs `forge scan all`, `forge lint`, `forge eval`, creates a PR.

---

## Step 6 — Explore other verbs

```bash
forge --help                            # full verb list
forge explain ship                      # describe any verb’s inputs, outputs, side-effects
forge scan all                          # run all 9 scanner families
forge lint                              # convention + hygiene checks
forge upgrade --check                   # show available codemods (dry-run)
forge doctor                            # environment health check

# Learning loop
forge learn teach                       # record a project convention
forge learn share                       # opt-in/out of sharing anonymized counts
forge learn promote                     # promote a spec to production

# Incident management
forge incident new --id INC-001 --title "API down" --severity S1
forge incident triage INC-001           # AI auto-triage
forge generate test --from-bug INC-001  # generate regression tests from incident

# Semantic cache & streaming
# (automatic — Forge caches LLM responses by token similarity and streams output)

# Deploy & rollback
forge deploy --dry-run                  # preview deploy
forge rollback --advise <deploy-id>     # get AI-recommended rollback target

# Privacy
forge audit erase --subject <user-id>   # GDPR right-to-erasure
forge context --redact                  # redact PII from context snapshots

# Insights
forge insights cli                      # unused verbs, schema drift
forge insights hygiene                  # weekly hygiene digest
```

---

## Next steps

| Goal | Where to look |
|------|---------------|
| Add a third-party scanner plugin | [docs/PLUGIN_AUTHORING.md](docs/PLUGIN_AUTHORING.md) |
| Understand every verb and flag | [docs/VERBS.md](docs/VERBS.md) |
| Install on another platform | [docs/INSTALLATION.md](docs/INSTALLATION.md) |
| Contribute a change | [CONTRIBUTING.md](CONTRIBUTING.md) |
| Architecture deep-dive | [docs/ARCHITECTURE_OVERVIEW.md](docs/ARCHITECTURE_OVERVIEW.md) |
| Community plugin index | [docs/COMMUNITY_PLUGINS.md](docs/COMMUNITY_PLUGINS.md) |
| Error code reference | [docs/ERROR_CODES.md](docs/ERROR_CODES.md) |
| Air-gap / offline use | [docs/airgap.md](docs/airgap.md) |

---

*Last updated: forge v0.M0 (pre-release). For issues, open a bug report using
the template at `.github/ISSUE_TEMPLATE/bug.yml`.*
