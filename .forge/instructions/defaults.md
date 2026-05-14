# Forge Default Instructions
# See: https://github.com/teragrid/forge/blob/main/docs/FORGE_FRAMEWORK_SPEC.md#instructions-pack
#
# This file provides default instructions for AI coding assistants working in
# this repository. It is read automatically by GitHub Copilot, Claude, Cursor,
# Windsurf, and other tools that support .forge/instructions.
#
# See also: AGENTS.md (canonical AI tool instructions)

## Code style
- Go 1.24+, CGO_ENABLED=0 for production builds
- All errors use `errcode.Register(errcode.Code(NNNN), "message")`
- Never use `os.Exit` except in `cmd/forge/main.go`
- All LLM calls go through `internal/llmprovider`
- All subprocess execution goes through `internal/procspawn`

## Test conventions
- Tests live alongside production code in `<package>/<file>_test.go`
- Table-driven tests preferred; no third-party test frameworks
- Fuzz tests use `testing.F` in `*_fuzz_test.go` files

## Security
- Never store secrets in code — use `internal/secretrewriter`
- Validate all user-supplied paths via `internal/fssandbox`
- OWASP Top 10 compliance enforced by `forge scan security`
