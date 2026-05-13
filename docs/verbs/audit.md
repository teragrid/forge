# forge audit

Append-only hash-chained audit ledger.

## Synopsis

```
forge audit log [--root <path>] [--json]
forge audit verify [--root <path>]
forge audit query [--root <path>] [--op <verb>] [--since <date>]
```

## Description

`forge audit` maintains a tamper-evident append-only log of all forge
operations that modify project state. The ledger uses SHA-256 hash chaining.

## Examples

```bash
forge audit log
forge audit verify
forge audit query --op deploy --since 2024-01-01
```

## Error codes

| Code | Meaning |
|------|---------|
| `FORGE-3400` | Audit operation failed |
| `FORGE-3401` | Audit ledger tampered or corrupted |
| `FORGE-3402` | Audit query failed |
