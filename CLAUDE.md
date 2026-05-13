# CLAUDE.md — Forge Project Instructions for Claude

> Claude reads this file automatically when working in this repository.
> See also: AGENTS.md (canonical AI tool instructions)

---

## Quick start

This is a Go 1.24+ project. Module: `github.com/teragrid/forge`. CGO is
disabled — do not import C libraries.

```bash
go build ./...    # must produce zero output
go vet ./...      # must produce zero output
go test ./...     # all tests must pass
```

---

## Critical rules

1. **Never use `os.Exit`** except in `cmd/forge/main.go`.
2. **Never add third-party dependencies** without an ADR in `docs/adr/`.
3. **Every new verb** needs a `verbmeta.Register(...)` call in its `init()`.
4. **Error codes** must be unique. Check `docs/ERROR_CODES.md` before choosing a range.
5. **All LLM calls** go through `internal/llmprovider`. No direct HTTP to OpenAI/Anthropic/Gemini.
6. **Path traversal**: validate all user paths via `internal/fssandbox`.
7. **Subprocess execution**: use `internal/procspawn` allow-list only.

---

## Package structure

```
internal/cli/cmd<verb>/   — one package per CLI verb
internal/errcode/         — error code registry
internal/llmprovider/     — LLM gateway (Anthropic, OpenAI, Gemini)
internal/plugin/          — plugin interface + registry
internal/resilience/      — Retry[T], WithTimeout[T]
internal/fssandbox/       — path allow/deny list
internal/procspawn/       — subprocess allow-list
internal/audit/           — append-only hash-chained audit ledger
```

---

## When you add a verb

1. `internal/cli/cmd<verb>/<verb>.go` — `package cmd<verb>`; export `New() *cobra.Command`
2. Register verbmeta in `init()`
3. Register error codes for the verb
4. Add to `root.AddCommand(...)` in `internal/cli/root.go`
5. Add import in `root.go`

---

*Full instructions: AGENTS.md*
