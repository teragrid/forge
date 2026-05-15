# forge ship

Run the 5-checkpoint pre-ship pipeline for a feature or release.

## Synopsis

```
forge ship [<feature>] [spec|test|breakdown|code|ship] [flags]
```

The positional `<feature>` argument (e.g. `auth/email`) is slugified to a
directory name (`auth-email`) and used as the spec/artifact root under
`.forge/specs/`. `--description` is a deprecated alias for the positional arg.

## Checkpoints

1. **Spec** — spec file present and linked to tasks
2. **Test** — all test families pass
3. **Breakdown** — task breakdown present
4. **Code** — code changes detected
5. **Ship** — security scan clean; hygiene OK; manifest OK

## Examples

```bash
# Run the full 5-checkpoint pipeline for a feature
forge ship auth/email

# Dry-run to preview without side effects
forge ship auth/email --dry-run

# Run only the Spec checkpoint
forge ship auth/email spec

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
