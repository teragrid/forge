# Forge Documentation

**Forge** is a single-binary Go CLI that bundles the scan-fix-learn loop, LLM
gateway, plugin runtime, and ship workflow for AI-generated code.

---

## Quick links

| Resource | Description |
|----------|-------------|
| [Installation](INSTALLATION.md) | Install forge on Linux, macOS, or Windows |
| [Quick Start](getting-started.md) | Your first forge project in 5 minutes |
| [CLI Reference](VERBS.md) | All forge verbs and flags |
| [Plugin Authoring](PLUGIN_AUTHORING.md) | Build and publish forge plugins |
| [Security Policy](SECURITY.md) | Vulnerability reporting and disclosure |
| [Architecture](ARCHITECTURE.md) | System design and internal packages |

---

## What forge does

```
forge new         — scaffold a new AI-augmented project
forge scan        — detect security issues, lint drift, and hygiene violations
forge fix         — auto-apply LLM-suggested fixes
forge learn       — submit anonymised patterns to the learning loop
forge ship        — run all gates and publish a release
forge deploy      — deploy to Fly, Railway, Render, Heroku, or AWS ECS
forge eval        — evaluate LLM quality against test scenarios
forge plugin      — install, list, and manage WASM plugins
forge add         — add a verified dependency with security scan
forge optimize    — AI-powered performance analysis and recommendations
forge undo        — reverse the last forge operation (ADR-024)
forge doctor      — check environment health, API keys, and network mode
forge audit       — append-only hash-chained audit ledger
forge insights    — local telemetry rollup and usage analytics
forge report      — generate HTML/Markdown compliance reports
forge incident    — manage on-call incidents and status-page webhooks
forge bundle      — create and extract offline (air-gapped) install bundles
```

---

## Key design decisions

| ADR | Decision |
|-----|----------|
| [ADR-001](adr/ADR-001-implementation-language.md) | Go 1.24+, CGO_ENABLED=0 |
| [ADR-002](adr/ADR-002-plugin-runtime.md) | WASM plugin runtime (wazero) |
| [ADR-009](adr/ADR-009-error-code-namespacing.md) | Error code namespacing (FORGE-XXXX) |
| [ADR-014](adr/ADR-014-resilience-pattern-library.md) | Resilience-pattern library |
| [ADR-022](adr/ADR-022-two-key-enforcement.md) | Two-key enforcement for high-impact ops |
| [ADR-024](adr/ADR-024-reversibility-contract.md) | Reversibility contract (forge undo) |

---

## Community

- [GitHub Discussions](https://github.com/teragrid/forge/discussions)
- [Community Plugins](COMMUNITY_PLUGINS.md)
- [Contributing](../CONTRIBUTING.md)
- [Code of Conduct](../CODE_OF_CONDUCT.md)
- [RFCs](rfcs/README.md)
