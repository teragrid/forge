# Changelog

All notable changes to forge will be documented in this file. Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/); the project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0-mvp] — community-launch preview

The first runnable slice of forge. Goal: contributors can clone, build, and scaffold a working Go service in under a minute. The full ship/scan loop (M1) is intentionally **not** in this drop.

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
- **`internal/errcode`** — `FORGE-XXXX` registry with reserved code ranges (1000s = CLI, 2000s = config/fs/scaffold/manifest, 3000s = scan, 9000s = test). Panics on duplicate or out-of-range registration.
- **`internal/logobs`** — slog wrapper. Auto / JSON / text formatter, secret-key redaction (`secret_*`, `token_*`, `api_key*`, `password`, `token`, `secret`), `Explain=true` opt-in to bypass redaction (for `forge explain`-class verbs).
- **`internal/verbmeta`** — verb manifest registry powering `forge explain`.
- **`internal/manifest`** — `.forge/manifest` text-format reader. Sections: `[scratch]`, `[managed]`. Glob matcher supports `**`, `*`, `?`. **Managed wins over scratch** to prevent false-positive deletions.
- **`internal/scaffold`** — `embed.FS`-backed template renderer (`all:` glob to include dotfiles), `text/template` substitution with `missingkey=error`, `__name__` path interpolation, force-overwrite gate.

### Test coverage

Every package ships unit tests covering the [9-point design checklist](https://github.com/teragrid/forge/blob/main/CONTRIBUTING.md): happy / boundary / negative / idempotency / cross-tenant (where applicable) / regression / data-accuracy / false-positive guard.

```text
ok  internal/cli            ok  internal/cli/cmdclean
ok  internal/cli/cmddoctor  ok  internal/cli/cmdexplain
ok  internal/cli/cmdnew     ok  internal/cli/cmdversion
ok  internal/errcode        ok  internal/logobs
ok  internal/manifest       ok  internal/scaffold
ok  internal/verbmeta
```

### Deferred to next release

`forge ship`, `forge scan`, plugin runtime (wazero ABI), audit ledger, LLM gateway, Spec-Lock, governance, telemetry. Tracked in [tasks/DEVELOPMENT_TASKS.md](tasks/DEVELOPMENT_TASKS.md).

[Unreleased]: https://github.com/teragrid/forge/compare/v0.1.0-mvp...HEAD
[0.1.0-mvp]: https://github.com/teragrid/forge/releases/tag/v0.1.0-mvp
