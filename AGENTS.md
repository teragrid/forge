# AGENTS.md — Forge AI Tool Instructions

> This file provides context and instructions for AI coding assistants working
> in the Forge repository. It is read automatically by GitHub Copilot, Claude,
> Cursor, Windsurf, and other tools that support AGENTS.md / CLAUDE.md conventions.

---

## Project overview

**Forge** is a single-binary Go CLI that bundles the scan-fix-learn loop, LLM
gateway, plugin runtime, and ship workflow for AI-generated code. Module path:
`github.com/teragrid/forge`.

---

## Repository layout

```
cmd/forge/          — main entry point (thin wrapper around internal/cli)
internal/cli/       — one package per verb: cmd<verb>/
internal/           — shared packages (errcode, plugin, resilience, etc.)
docs/               — specifications, ADRs, task lists
tests/fixtures/     — test fixture data
```

---

## Code conventions

### Adding a new verb

1. Create `internal/cli/cmd<verb>/<verb>.go` with `package cmd<verb>`.
2. Export a `New() *cobra.Command` function.
3. Register `verbmeta.Manifest{...}` in an `init()` func (required — CI gate M3-07 enforces this).
4. Register error codes in the verb's range (see `docs/ERROR_CODES.md`).
5. Add `cmd<verb>.New()` to the `root.AddCommand(...)` list in `internal/cli/root.go`.
6. Add an import for the new package in `root.go`.

### Error codes

All errors use `errcode.Register(errcode.Code(NNNN), "message")`. Ranges are
documented in `docs/ERROR_CODES.md`. Never reuse an existing code.

### LLM calls

Use `internal/llmprovider` for all LLM calls:
```go
p, err := llmprovider.Detect()
resp, err := p.Complete(ctx, &llmprovider.Request{...})
```

### Tests

- Tests live in `<package>/<file>_test.go` alongside production code.
- Use `testing.T` directly; no third-party test framework.
- Fuzz tests use `testing.F` and go in `*_fuzz_test.go` files.
- Table-driven tests are preferred.

### Security

- Never store secrets in code. Use `internal/secretrewriter` for redaction.
- Validate all user-supplied paths via `internal/fssandbox`.
- All subprocess execution goes through `internal/procspawn` (allow-list).
- OWASP Top 10 compliance is enforced by `forge scan security` in CI.

---

## Build & test

```bash
# Build
go build ./...

# Test
go test ./...

# Fuzz (example)
go test -fuzz=FuzzManifestValidate -fuzztime=30s ./internal/plugin/

# Vet
go vet ./...

# Full CI gates
make ci
```

---

## Key design decisions (ADRs)

| ADR | Decision |
|-----|---------|
| ADR-001 | Go 1.24+, CGO_ENABLED=0 |
| ADR-002 | WASM plugin runtime (wazero) |
| ADR-003 | Brew + Scoop + winget distribution |
| ADR-009 | Error code namespacing (FORGE-XXXX) |
| ADR-014 | Resilience-pattern library |
| ADR-024 | Reversibility contract (forge undo) |

Full list in `docs/adr/`.

---

## Prompt engineering notes

- Prefer small, focused prompts. Use `internal/prompttemplates` for per-verb system prompts.
- Always include project context from `forge context generate` when making LLM calls.
- Token budgets are enforced by `internal/llmbudget`. Respect them.
- Cache LLM responses via `internal/llmcache` when the input is deterministic.

---

## Do not

- Do not import C libraries (CGO must remain disabled).
- Do not use `os.Exit` anywhere except `cmd/forge/main.go`.
- Do not add third-party dependencies without an ADR.
- Do not bypass the plugin sandbox (`internal/fssandbox`, `internal/procspawn`).
- Do not write PII to logs or telemetry.

---

*See also: CONTRIBUTING.md · CODE_OF_CONDUCT.md · SECURITY.md · docs/ARCHITECTURE.md*
