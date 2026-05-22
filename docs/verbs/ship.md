# forge ship

Run the 6-checkpoint pre-ship pipeline for a feature or release.

## Synopsis

```
forge ship [<feature>] [spec|arch|test|breakdown|code|ship] [flags]
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

> **KB injection**: checkpoints 2–5 (`arch`, `test`, `breakdown`, `code`) all use `InvokeWithKnowledge`
> — the top-5 relevant knowledge-base entries are prepended to the LLM system prompt automatically.
> Add project-specific guidance to `.forge/knowledge/` to influence generated artifacts.

> **Supabase RPC auto-detection**: `forge ship arch` calls `detectAPIStyle(openapi.yaml)`.
> If `/rest/v1/rpc/` paths are present, downstream checkpoints inject targeted Supabase guidance
> (PostgreSQL function creation, `GRANT EXECUTE`, RLS policies, `.rpc()` client calls).
> Use standard REST paths (`/api/v1/…`) for non-Supabase projects — detection is automatic.

## Examples

```bash
# Run the full 6-checkpoint pipeline for a feature
forge ship auth/email

# Dry-run to preview without side effects
forge ship auth/email --dry-run

# Run only the Spec checkpoint
forge ship auth/email spec

# Run only the Arch checkpoint (generates arch.md + openapi.yaml)
forge ship auth/email arch

# Resume from the first missing checkpoint
forge ship auth/email --resume

# NDJSON event stream (one line per checkpoint) for agent orchestration
forge ship auth/email --yes --json

# Tag and push a release
forge ship --tag v1.2.3
```

## Deprecated aliases

| Old form | New form | Removed in |
|----------|----------|------------|
| `forge ship --description "auth/email"` | `forge ship auth/email` | v1.1 |
| `forge ship verify` | `forge ship ship` | v1.1 |
| `forge ship resume <feature>` | `forge ship <feature> --resume` | v1.1 |
