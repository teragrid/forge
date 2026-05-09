# Changelog

All notable changes to forge will be documented in this file. Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/); the project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0-mvp] — community-launch preview

The first runnable slice of forge with **8 working verbs** (5 core M0 + 3 M1 security/hygiene/preview). Goal: contributors can clone, build, and scaffold a working Go service in under a minute, plus scan secrets, verify hygiene, and preview the ship workflow.

### Added

- **`forge version` [`--json`]** — prints version + Go/OS/arch.
- **`forge new <template> <path>`** — embedded template renderer. Ships one template (`go-service`) with:
  - HTTP server with `/healthz`, `/readyz`, graceful shutdown via `signal.NotifyContext`.
  - Healthz `httptest` regression test.
  - Managed `.gitignore` with `forge:gitignore:start/end` marker block.
  - Baseline `.gitleaks.toml` (generic-api-key, private-key-block, OpenAI sk-*, AWS AKIA*).
  - `.forge/manifest` baseline (scratch + managed sections).
  - Flags: `--name`, `--module`, `--force`, `--json`.
- **`forge doctor` [`--json`]** — env health checks (git, go, temp-dir writable). Non-zero exit on required-check failure.
- **`forge clean` [`--check`|`--apply`] [`--root`] [`--json`]** — manifest-driven LLM-scratch sweeper. `--check` is the default and exits non-zero when candidates exist (CI-gateable).
- **`forge explain [verb]` [`--json`]** — verb-manifest browser. With no arg lists all registered verbs; with one arg prints inputs / outputs / side-effects / gates touched / error codes.
- **`forge scan secrets` [`--root`] [`--json`]** ⭐ **(M1 headline)** — secret scanner (attempts gitleaks; fallback to built-in patterns). Outputs findings with file/line/rule/match. Exit non-zero on findings.
- **`forge lint` [`--root`] [`--json`]** ⭐ **(M1 hygiene)** — hygiene checker (manifest presence, .gitignore markers, .gitleaks.toml baseline). Outputs structured {file, level, code, message}. Error exit if any check fails.
- **`forge ship [--dry-run]` [`--json`]** ⭐ **(M1 preview)** — validates 5-checkpoint pipeline without executing (spec, test, breakdown, code, hygiene). MVP: hygiene checkpoint only. Exit non-zero if any checkpoint fails.
- **`internal/errcode`** — `FORGE-XXXX` registry with reserved code ranges (1000s = CLI verbs, 2000s = config/fs/scaffold/manifest, 3000s = scan/lint/ship, 9000s = test). Panics on duplicate or out-of-range registration.
- **`internal/logobs`** — slog wrapper. Auto / JSON / text formatter, secret-key redaction (`secret_*`, `token_*`, `api_key*`, `password`, `token`, `secret`), `Explain=true` opt-in to bypass redaction (for `forge explain`-class verbs).
- **`internal/verbmeta`** — verb manifest registry powering `forge explain`.
- **`internal/manifest`** — `.forge/manifest` text-format reader. Sections: `[scratch]`, `[managed]`. Glob matcher supports `**`, `*`, `?`. **Managed wins over scratch** to prevent false-positive deletions.
- **`internal/scaffold`** — `embed.FS`-backed template renderer (`all:` glob to include dotfiles), `text/template` substitution with `missingkey=error`, `__name__` path interpolation, force-overwrite gate.

### Test coverage

All 14 packages with unit tests covering the [9-point design checklist](https://github.com/teragrid/forge/blob/main/CONTRIBUTING.md): happy / boundary / negative / idempotency / cross-tenant (where applicable) / regression / data-accuracy / false-positive guard.

| Package | Status | Package | Status |
|---------|--------|---------|--------|
| `internal/cli` | ✅ | `internal/cli/cmdscan` | ✅ |
| `internal/cli/cmdclean` | ✅ | `internal/cli/cmdlint` | ✅ |
| `internal/cli/cmddoctor` | ✅ | `internal/cli/cmdship` | ✅ |
| `internal/cli/cmdexplain` | ✅ | `internal/cli/cmdversion` | ✅ |
| `internal/cli/cmdnew` | ✅ | `internal/errcode` | ✅ |
| `internal/logobs` | ✅ | `internal/manifest` | ✅ |
| `internal/scaffold` | ✅ | `internal/verbmeta` | ✅ |

### Pre-push quality gates

All commits pass a 7-stage gate (installed via `git config core.hooksPath .githooks`):

1. `goimports` — import formatting
2. `gofmt -s` — code formatting
3. `go vet` — correctness checks
4. `golangci-lint` — static analysis (staticcheck, gocritic, gosec, errcheck, ineffassign, govet)
5. `go build` — compilation (CGO_ENABLED=0 for cross-platform)
6. `go test -count=1` — all unit tests
7. `govulncheck` — no known CVEs
8. `go mod verify` — module integrity

### Deferred to M1+ releases

Plugin runtime (wazero ABI), audit ledger, LLM gateway, Spec-Lock, governance, telemetry, full ship workflow (spec validation, test orchestration, breakdown composition, code generation). See [DEVELOPMENT_PLAN.md](docs/DEVELOPMENT_PLAN.md) for roadmap.

[Unreleased]: https://github.com/teragrid/forge/compare/v0.1.0-mvp...HEAD
[0.1.0-mvp]: https://github.com/teragrid/forge/releases/tag/v0.1.0-mvp
