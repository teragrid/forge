# forge rollback

Re-deploy a previous artifact version.

## Synopsis

```
forge rollback [--root <path>] [--to <tag>] [--dry-run] [--allow-irreversible]
```

## Description

`forge rollback` re-deploys to a previous release tag using the same adapter
as `forge deploy`. Requires `--allow-irreversible` (ADR-024 reversibility contract).

## Examples

```bash
forge rollback --to v1.1.0 --allow-irreversible
forge rollback --dry-run
```

## See also

- `forge deploy` — forward deployment
- [ADR-024](../adr/ADR-024-reversibility-contract.md) — reversibility contract
