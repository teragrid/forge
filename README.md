# forge

<p align="center">
  <img src="forge-logo.png" alt="Forge logo" width="600" />
</p>

<p align="center">
  <strong>The LLM-native framework that makes AI-generated code survive production.</strong><br/>
  Security, multi-tenancy, audit, and observability — built in, not bolted on.
</p>

<p align="center">
  <a href="https://github.com/teragrid/forge/releases"><img src="https://img.shields.io/github/v/tag/teragrid/forge?sort=semver&color=orange&label=latest" alt="Latest release"/></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-Apache--2.0-blue" alt="Apache 2.0"/></a>
  <a href="https://github.com/teragrid/forge/actions"><img src="https://img.shields.io/github/actions/workflow/status/teragrid/forge/ci.yml?label=CI" alt="CI"/></a>
</p>

---

## What is Forge?

Forge is a single-binary CLI that closes the gap between "vibe-coded" AI output and production-grade software. It wraps your existing project with the checks, fixes, and workflows that LLMs forget to include:

- **Scan** for secrets, prompt-injection, and supply-chain risks before they ship
- **Clean** LLM cruft from your codebase automatically
- **Ship** through a validated 5-checkpoint pipeline
- **Audit** every change in a tamper-evident ledger
- **Evaluate** LLM regression scenarios in CI
- **Monitor** spend, telemetry, and incidents — all from one tool

> *Vibe it and ship it. Built to last.*

---

## Install

### npm (recommended — no Go required)

```sh
npm install -g @forge/cli
```

### npx (try without installing)

```sh
npx @forge/cli new my-app --template ts-service
```

### Homebrew

```sh
brew install teragrid/tap/forge
```

### Download a binary

Pre-built binaries for Linux, macOS, and Windows are on the [Releases page](https://github.com/teragrid/forge/releases). Each release includes checksums and an SBOM.

### Go install

```sh
go install github.com/teragrid/forge/cmd/forge@latest
```

---

## Quick start

```sh
# Scaffold a new TypeScript service
forge new ts-service my-app
cd my-app && npm install && npm run dev

# Or scaffold a Go service
forge new go-service my-app
cd my-app && go run ./...

# Adopt an existing project (like git init)
cd my-existing-project
forge init
```

Then run your first scan:

```sh
forge scan all          # secrets, prompt-injection, supply-chain
forge doctor            # check your environment
forge ship --dry-run    # preview the full ship pipeline
```

---

## Commands

| Command | What it does |
|---------|-------------|
| `forge new <template> <path>` | Scaffold a new project (`go-service`, `ts-service`) |
| `forge init` | Adopt an existing directory as a Forge project |
| `forge doctor` | Check your environment (git, Go, OS, permissions) |
| `forge scan <family>` | Scan for `secrets`, `rls`, `prompt-injection`, `supply-chain`, or `all` |
| `forge clean` | Remove LLM cruft based on your project manifest |
| `forge lint` | Check hygiene (manifest, `.gitignore` markers, gitleaks config) |
| `forge ship [--dry-run]` | Run the 5-checkpoint ship pipeline |
| `forge upgrade <codemod>` | Apply a codemod — `gitignore-marker`, `gitleaks-baseline`, `list` |
| `forge audit <show\|verify>` | Inspect the tamper-evident audit ledger |
| `forge eval [path]` | Run LLM scenario regression suites |
| `forge explain <verb>` | Print what a command does, its inputs and side-effects |
| `forge spend <status\|set>` | Track and enforce LLM API spend limits |
| `forge incident <new\|list>` | Manage incident lifecycle (identified → fixed) |
| `forge insights` | Summarise activity from the local audit log |
| `forge telemetry <enable\|disable>` | Control opt-in local telemetry (no PII ever leaves your machine) |
| `forge version` | Print version and build metadata |

Run `forge --help` or `forge <command> --help` for full flag documentation.

---

## Why Forge?

| Without Forge | With Forge |
|---------------|------------|
| Secrets committed by LLM hallucinations | `forge scan secrets` catches them before push |
| No audit trail of AI-generated changes | `forge audit` logs every change in a hash-chained ledger |
| LLM output breaks in CI | `forge ship` validates the full pipeline before you push |
| Runaway API costs | `forge spend` enforces daily and monthly LLM budget limits |
| Incidents lost in chat threads | `forge incident` tracks full lifecycle with structured state |

---

## Community

- **Discussions** — [GitHub Discussions](https://github.com/teragrid/forge/discussions) for questions and ideas
- **Issues** — [Bug reports and feature requests](https://github.com/teragrid/forge/issues)
- **Security** — Please read [docs/SECURITY.md](docs/SECURITY.md) before reporting vulnerabilities
- **Contributing** — See [CONTRIBUTING.md](CONTRIBUTING.md). All commits must be DCO-signed (`git commit -s`)

We follow the [Contributor Covenant](CODE_OF_CONDUCT.md). Everyone is welcome.

---

## License

Apache-2.0 — see [LICENSE](LICENSE) and [NOTICE](NOTICE).