> **Pipeline status** — 7 checkpoints: `spec` → `arch` → `test` → `breakdown` → `code` → `ship` → `qa-verify`
>
> | # | Checkpoint | Gate | Agent | Primary output |
> |---|-----------|------|-------|---------------|
> | 1 | `spec` | LLM / stub | — | `.forge/specs/<feature>/spec.md` |
> | 2 | `arch` | LLM + KB | — | `arch.md`, `openapi.yaml` |
> | 3 | `test` | LLM + KB | — | test files (TDD gate) |
> | 4 | `breakdown` | LLM + KB | — | task list |
> | 5 | `code` | LLM + KB | — | implementation |
> | 6 | `ship` | scan + hygiene | — | secrets clean, manifest OK |
> | 7 | `qa-verify` | **QA / QE agent** | MCP probe or native tests | test suite pass + AI-agent tools confirmed operational |

# forge ship

Run the 7-checkpoint pre-ship pipeline for a feature or release.

## Synopsis

```
forge ship [<feature>] [spec|arch|test|breakdown|code|ship|qa-verify] [flags]
```

The positional `<feature>` argument (e.g. `auth/email`) is slugified to a
directory name (`auth-email`) and used as the spec/artifact root under
`.forge/specs/`. `--description` is a deprecated alias for the positional arg.

## Checkpoints

1. **Spec** — spec file present and linked to tasks
2. **Arch** — architecture ADR (`arch.md`) and OpenAPI contract (`openapi.yaml`) generated via KB-enriched LLM call
3. **Test** — all test families pass (Supabase RPC integration tests auto-included when `openapi.yaml` contains `/rest/v1/rpc/` paths)
4. **Breakdown** — task breakdown present (Supabase RPC tasks — PostgreSQL function, `GRANT EXECUTE`, RLS policy — auto-included for RPC features)
5. **Code** — code changes detected (SQL function + TypeScript `.rpc()` client auto-generated for RPC features)
6. **Ship** — security scan clean; hygiene OK; manifest OK
7. **QA-Verify** — QA/QE agent: probes the project's MCP server unit tests (Go: `go test ./internal/mcpserver/...`; Python: `pytest tests/test_mcp_server.py`) to confirm every AI-agent tool is operational. Falls back to the native test suite (`go test ./...` / `pytest`) when no MCP server is configured. Passes with a warning if no test runner is found — override with `--skip-checkpoint qa-verify`.

> **KB injection**: checkpoints 2–5 (`arch`, `test`, `breakdown`, `code`) all use `InvokeWithKnowledge`
> — the top-5 relevant knowledge-base entries are prepended to the LLM system prompt automatically.
> Add project-specific guidance to `.forge/knowledge/` to influence generated artifacts.

> **Supabase RPC auto-detection**: `forge ship arch` calls `detectAPIStyle(openapi.yaml)`.
> If `/rest/v1/rpc/` paths are present, downstream checkpoints inject targeted Supabase guidance
> (PostgreSQL function creation, `GRANT EXECUTE`, RLS policies, `.rpc()` client calls).
> Use standard REST paths (`/api/v1/…`) for non-Supabase projects — detection is automatic.

## Feature-branch workflow

When you run `forge ship <feature>` from a **protected branch** (`main`, `master`, `develop`, `dev`, `trunk`, `production`, `prod`), Forge automatically creates and checks out `feature/<slug>` before starting the pipeline. This keeps your work isolated from the main branch at all times.

```bash
# On main — Forge creates feature/auth-email, then runs the pipeline
forge ship auth/email

# On a feature branch already — Forge runs on the current branch (no new branch)
git checkout -b feature/auth-email
forge ship auth/email

# Skip branch creation entirely — run on current branch regardless
forge ship auth/email --no-branch
```

After all seven checkpoints pass, Forge prints the next steps:

```
  Branch:   feature/auth-email
  Push:     git push -u origin feature/auth-email
  PR:       gh pr create --base main --head feature/auth-email --title "feat: auth/email"
```

## Examples

```bash
# Run the full 7-checkpoint pipeline for a feature (auto-creates feature branch from main)
forge ship auth/email

# Dry-run to preview without side effects
forge ship auth/email --dry-run

# Run only the Spec checkpoint
forge ship auth/email spec

# Run only the Arch checkpoint (generates arch.md + openapi.yaml)
forge ship auth/email arch

# Run only the QA agent checkpoint
forge ship auth/email qa-verify

# Skip the QA agent (no test runner configured)
forge ship auth/email --skip-checkpoint qa-verify

# Resume from the first missing checkpoint
forge ship auth/email --resume

# NDJSON event stream (one line per checkpoint) for agent orchestration
forge ship auth/email --yes --json

# Skip auto-branch creation (stay on current branch)
forge ship auth/email --no-branch

# Tag and push a release
forge ship --tag v1.2.3
```

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--dry-run` | false | Preview what would happen without making any changes |
| `--json` | false | Emit NDJSON event stream (one line per checkpoint) |
| `--yes` | false | Non-interactive mode; auto-accept prompts |
| `--resume` | false | Skip checkpoints that already have passing artifacts |
| `--no-branch` | false | Do not create or switch to a feature branch; run on current branch |
| `--tag <version>` | — | After a clean pipeline, tag and push a release |
| `--skip-checkpoint <name>` | — | Skip a named checkpoint (e.g. `qa-verify` when no test runner is configured) |

## Deprecated aliases

| Old form | New form | Removed in |
|----------|----------|------------|
| `forge ship --description "auth/email"` | `forge ship auth/email` | v1.1 |
| `forge ship verify` | `forge ship ship` | v1.1 |
| `forge ship resume <feature>` | `forge ship <feature> --resume` | v1.1 |
