# Forge v1.0.0 Release Notes

> Release date: TBD (targeting Q2 2025)  
> See [CHANGELOG.md](../CHANGELOG.md) for the full history.

---

## Highlights

Forge v1.0.0 is the first stable, production-supported release of the
LLM-native ship workflow CLI. This release locks the public CLI contract,
plugin interface, audit ledger schema, and telemetry payload schema for
semantic versioning compatibility.

---

## What's new since v0.x

### M0 — Core foundation
- `forge init`, `forge new`, `forge add`, `forge doctor`, `forge scan`
- Append-only hash-chained audit ledger
- Token ledger + spend tracking
- `forge review`, `forge fix`, `forge explain`, `forge ask`
- `forge ship` (5-checkpoint workflow)
- `forge check`, `forge lint`, `forge test`, `forge generate`
- `forge adopt` — onboard existing projects
- WASM plugin runtime (wazero-backed sandbox)
- LLM gateway: Anthropic, OpenAI, Gemini

### M1 — Hardening & resilience
- `forge context`, `forge telemetry`, `forge version`
- `forge migrate` — database migration runner with `forge scan rls`
- Resilience library: `Retry[T]`, `WithTimeout[T]`, circuit-breaker
- Chaos drill harness (`forge chaos drill`)
- i18n scaffolding (en, es, fr, de, ja, zh locales)
- Plugin compliance test runner
- 14-family scanner suite (secrets, supply-chain, rls, prompt-injection,
  auth, perf, accessibility, cost, …)
- Structured error codes (FORGE-XXXX) with `docs/ERROR_CODES.md`
- AI tool instructions: AGENTS.md, CLAUDE.md, .cursorrules, .windsurfrules

### M2 — Ecosystem
- Plugin registry with install/upgrade/remove lifecycle
- Deploy adapters: Fly.io, Railway
- `forge eject` — remove Forge from a project
- `forge docs sync/heal` — documentation drift detection
- `forge hygiene report/manifest`
- `forge learn` — learning loop with opt-in community sharing
- `forge agents start/stop/list` — multi-agent runtime
- `forge deploy` / `forge rollback`
- Eval harness with 7 reference scenarios
- Post-mortem CI gate
- Perf benchmark regression gate

### M3 — Quality & launch
- `forge optimize` — AI-powered code optimisation
- `forge report` — cross-cutting aggregated report
- `forge add <primitive>` — scaffold code primitives
- `forge undo` — reversibility contract
- Per-verb prompt templates (`internal/prompttemplates`)
- Two-key enforcement for high-impact operations (ADR-022)
- Regulated-industry scaffold templates: SOC2, HIPAA, FinServ
- Eval flake quarantine policy (ADR-023)
- Status-page webhook integration
- RFC process (`docs/rfcs/`) with 3 accepted RFCs
- Deprecation policy (`docs/DEPRECATION_POLICY.md`)
- NFR budgets locked in CI (startup <200ms, help <500ms)

---

## Breaking changes from v0.x

See [BREAKING.md](../BREAKING.md) for the full list.

Key changes:
- Plugin interface `Scanner.Scan()` signature changed — `context.Context` is
  now the first argument. Update all plugins.
- Config key `forge.provider` renamed to `forge.llm.provider`.
- `forge review --format` flag default changed from `text` to `json`.

---

## Artifact signing

All v1.0.0 release artifacts are signed with `cosign`. Verify with:

```bash
cosign verify-blob \
  --certificate forge_1.0.0_checksums.txt.pem \
  --signature forge_1.0.0_checksums.txt.sig \
  forge_1.0.0_checksums.txt
```

The signing key is published at `https://keys.forge.dev/cosign.pub`.

---

## Upgrade path

```bash
# macOS
brew upgrade forge

# Windows
scoop update forge

# Linux
curl -fsSL https://get.forge.dev | sh
```

After upgrading, run `forge doctor` to verify the installation and check for
any deprecated config keys.

---

## Known issues

- `forge deploy --adapter railway` requires Railway CLI v3.x. v2.x is not supported.
- Windows: `forge chaos drill` requires PowerShell 5.1 or later.

---

*Full changelog: [CHANGELOG.md](../CHANGELOG.md)*  
*Report issues: https://github.com/teragrid/forge/issues*
