# Getting Started with Forge

Get from zero to a production-ready AI-augmented project.

---

## Prerequisites

| Requirement | Minimum version | Notes |
|-------------|-----------------|-------|
| **Go** | 1.24+ | `go version` to check |
| **Git** | 2.34+ | Required for audit ledger and commit signing |
| **LLM provider** | — | At least one API key (see below) |

### LLM provider setup

Forge detects your provider from environment variables. Set **one** of the
following before running any LLM-powered verb:

```bash
# Anthropic Claude (recommended)
export ANTHROPIC_API_KEY=sk-ant-...

# OpenAI
export OPENAI_API_KEY=sk-...

# Google Gemini
export GEMINI_API_KEY=...

# Azure OpenAI
export AZURE_OPENAI_API_KEY=...
export AZURE_OPENAI_ENDPOINT=https://<resource>.openai.azure.com
export AZURE_OPENAI_DEPLOYMENT=gpt-4o   # optional, default: gpt-4o

# AWS Bedrock
export AWS_BEDROCK_REGION=us-east-1
export AWS_BEDROCK_MODEL=anthropic.claude-3-5-sonnet-20241022-v2:0  # optional

# Ollama (local / air-gapped)
export OLLAMA_HOST=http://localhost:11434
export OLLAMA_MODEL=llama3.2   # optional
```

> **Tip:** Run `forge doctor` at any time to verify your environment.

---

## Step 1 — Install forge

### macOS (Homebrew)

> **Coming soon.** Use npm or Go install below for now.

### Linux (curl installer)

```bash
curl -fsSL https://install.forge.dev | sh
# Installs to /usr/local/bin/forge by default
```

### Windows (Scoop)

> **Coming soon.** Use npm or the direct binary download below for now.

### Go install (any platform)

```bash
go install github.com/teragrid/forge/cmd/forge@latest
```

### Air-gapped / offline install

```bash
# On a connected machine, create a bundle:
forge bundle create --out forge-bundle.tar.gz

# Transfer forge-bundle.tar.gz to the air-gapped host, then:
forge bundle extract --from forge-bundle.tar.gz --to /opt/forge-bundle
export FORGE_AIRGAP=1
export FORGE_BUNDLE_DIR=/opt/forge-bundle
```

See [airgap.md](airgap.md) for full instructions.

### Verify the installation

```bash
forge --version
# forge 1.0.1

forge doctor
# ✓ go: 1.24.1
# ✓ git: 2.45.0
# ✓ LLM provider: anthropic
# ✓ network: online
```

> **Windows only:** if `forge version` shows `0.0.0-dev` after
> `npm install -g @forgeone/cli`, npm kept an older platform package. Fix:
> ```powershell
> npm install -g @forgeone/cli-win32-x64@latest
> ```

---

## Step 2 — Scaffold a new project

```bash
forge new my-service --template go-api
cd my-service
```

### Available templates

| Template | Description |
|----------|-------------|
| `ts-service` | TypeScript service with Vitest and Forge CI gates |
| `next-app` | Next.js 14, TypeScript, Tailwind CSS, App Router, Vitest + Playwright |
| `go-api` | Go HTTP API with forge CI gates |
| `go-cli` | Go CLI application with Cobra |
| `python-service` | Python FastAPI service |
| `node-service` | Node.js Express service |
| `regulated/soc2` | SOC 2-ready scaffold with audit hooks |
| `regulated/hipaa` | HIPAA-ready scaffold with PHI guards |
| `regulated/finserv` | Financial-services scaffold |

### What gets created

```
my-service/
├── main.go                     # entry point
├── go.mod                      # module: github.com/you/my-service
├── go.sum
├── .gitignore                  # forge-managed block included
├── .gitleaks.toml              # secret-scanning rules
├── .forge/
│   ├── config.json             # forge project config
│   ├── manifest.json           # verb + plugin registry
│   └── waivers/                # scan waiver store
└── .github/
    └── workflows/
        └── ci.yml              # forge CI gates (scan, lint, test, ship)
```

### Customise the project config

`.forge/config.json` controls forge behaviour for this project:

```json
{
  "project": "my-service",
  "budget": {
    "per_command_usd": 0.50,
    "per_day_usd": 10.00
  },
  "scan": {
    "severity_threshold": "medium"
  },
  "deploy": {
    "adapter": "fly",
    "target": "my-service-app"
  }
}
```

Validate config at any time:

```bash
forge config show
forge config get budget.per_day_usd
```

---

## Step 3 — Run your first scan

```bash
forge scan
```

Forge runs four scanner families:

| Scanner | What it checks |
|---------|----------------|
| `security` | OWASP Top 10, path traversal, prompt injection, supply chain |
| `secrets` | API keys, tokens, and credentials in staged files |
| `lint` | Convention drift, hygiene markers |
| `hygiene` | Manifest completeness, `.gitignore` coverage |

Sample output:

```
✓ security: no issues found
✓ secrets: 0 leaked credentials
⚠ lint: 2 findings
  [MEDIUM] missing verbmeta.Register call in internal/cli/cmdfooo/fooo.go
  [LOW]    go.sum not committed
✓ hygiene: 6 files checked
```

### Filter by scanner type

```bash
forge scan --only security
forge scan --only secrets
forge scan --json | jq '.findings[] | select(.severity == "HIGH")'
```

### Understanding a finding

Each finding includes:
- **Code** — e.g. `FORGE-SEC-001` (linkable to the error catalogue)
- **Severity** — `CRITICAL | HIGH | MEDIUM | LOW | INFO`
- **File + line** — exact location
- **Message** — human-readable description
- **Fix hint** — suggestion for remediation

---

## Step 4 — Fix issues automatically

```bash
forge fix
```

Forge presents each finding and asks whether to apply the LLM-generated fix:

```
Finding: missing verbmeta.Register call
File:    internal/cli/cmdfooo/fooo.go:12
Fix:     Add verbmeta.Register(verbmeta.Manifest{...}) in init()
Apply? [y/N/skip/quit]:
```

To apply all fixes without prompting:

```bash
forge fix --auto
```

Always review before committing:

```bash
git diff
git add -p   # stage selectively
```

### Waiving a finding

If a finding is a false positive, waive it rather than suppressing the scanner:

```bash
forge scan --json | jq '.findings[0].code'  # e.g. "FORGE-LNT-007"
# Add a waiver:
cat >> .forge/waivers/my-waiver.json << 'EOF'
{
  "code": "FORGE-LNT-007",
  "reason": "Intentional — this file is generated, not hand-edited.",
  "expires": "2025-12-31",
  "approved_by": "alice"
}
EOF
```

---

## Step 5 — Write and run tests

Forge enforces a **tests-before-code** policy. Run all test families:

```bash
forge test unit
forge test integration
forge test e2e
```

Or run them all:

```bash
forge test all
```

The `forge ship` gate will fail if tests are absent for new code paths.

---

## Step 6 — Add a plugin

Extend forge with community or in-house WASM plugins:

```bash
# Install from the registry
forge plugin install gosec

# List installed plugins
forge plugin list

# Verify it works
forge scan --only security   # gosec scanner now active
```

For offline environments, install from a bundle:

```bash
forge plugin install --from-bundle /opt/forge-bundle gosec
```

---

## Step 7 — Deploy

Configure your deploy target in `.forge/config.json`, then:

```bash
# Dry run first
forge deploy --dry-run

# Deploy for real
forge deploy
```

Supported adapters: `fly`, `railway`, `render`, `heroku`, `aws-ecs`.

```bash
# Deploy to Fly.io
forge deploy --adapter fly --target my-service-app

# Deploy to AWS ECS
forge deploy --adapter aws-ecs --target my-cluster/my-service
```

If something goes wrong, roll back:

```bash
forge rollback --to v0.9.0 --allow-irreversible
```

---

## Step 8 — Ship a release

```bash
forge ship --tag v1.0.0
```

`forge ship` runs five checkpoints in order:

| Checkpoint | What happens |
|------------|-------------|
| **Spec** | Spec file present and linked to tasks |
| **Tests** | All test families pass |
| **Scan** | Zero HIGH/CRITICAL findings |
| **Budget** | LLM spend within daily limit |
| **Ship** | Git tag created and pushed |

```
✓ spec:   FORGE_FRAMEWORK_SPEC.md present
✓ tests:  42 passed, 0 failed
✓ scan:   clean
✓ budget: $0.23 / $10.00
→ Tagged: v1.0.0
→ Pushed: origin v1.0.0
```

Dry-run to see what would happen:

```bash
forge ship --dry-run
```

---

## Step 9 — Monitor and audit

### View usage analytics

```bash
forge insights
forge insights --since 2024-01-01 --json
```

### Check LLM spend

```bash
forge spend show
forge spend show --since 2024-01-01
```

### Query the audit ledger

Every forge operation is recorded in a tamper-evident ledger:

```bash
forge audit log
forge audit query --op deploy
forge audit verify   # confirm no tampering
```

### Check environment health

```bash
forge doctor
forge doctor --json
```

---

## Common workflows

### Daily development loop

```bash
forge scan              # check before you code
# ... make changes ...
forge fix               # fix any new findings
forge test unit         # run unit tests
git add -p && git commit -S -m "feat: ..."
```

### Pre-PR checklist

```bash
forge scan --only security,secrets
forge test all
forge lint
forge doctor
```

### Release workflow

```bash
forge ship --tag vX.Y.Z --dry-run   # verify
forge ship --tag vX.Y.Z             # tag and push
forge deploy                         # ship to production
forge insights                       # review post-release metrics
```

---

## Environment variables reference

| Variable | Description |
|----------|-------------|
| `ANTHROPIC_API_KEY` | Anthropic Claude API key |
| `OPENAI_API_KEY` | OpenAI API key |
| `GEMINI_API_KEY` | Google Gemini API key |
| `AZURE_OPENAI_API_KEY` | Azure OpenAI API key |
| `AZURE_OPENAI_ENDPOINT` | Azure OpenAI resource endpoint |
| `OLLAMA_HOST` | Ollama base URL (air-gapped) |
| `FORGE_AIRGAP=1` | Force air-gap mode |
| `FORGE_BUNDLE_DIR` | Path to offline bundle directory |
| `FORGE_LEARN_OPT_IN=1` | Opt in to anonymised pattern sharing |
| `FORGE_LOG_LEVEL` | `debug \| info \| warn \| error` |
| `FORGE_LOG_FORMAT` | `text \| json` |

---

## Next steps

| Goal | Command / Resource |
|------|--------------------|
| Full CLI reference | `forge help` or [VERBS.md](VERBS.md) |
| Evaluate LLM quality | `forge eval` |
| Understand the architecture | [ARCHITECTURE.md](ARCHITECTURE.md) |
| Add a community plugin | `forge plugin install <name>` |
| Air-gapped deployment | [airgap.md](airgap.md) |
| Write a plugin | [PLUGIN_AUTHORING.md](PLUGIN_AUTHORING.md) |
| Security policy | [SECURITY.md](SECURITY.md) |
| Error code catalogue | [ERROR_CODES.md](ERROR_CODES.md) |
| Undo the last operation | `forge undo` |
