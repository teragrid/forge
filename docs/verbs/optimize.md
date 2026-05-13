# forge optimize

AI-powered performance analysis and recommendations.

## Synopsis

```
forge optimize [--root <path>] [--target <cpu|memory|latency>] [--dry-run] [--apply]
```

## Description

`forge optimize` profiles the project and uses the configured LLM to suggest
and optionally apply performance improvements.

## Examples

```bash
forge optimize
forge optimize --target memory
forge optimize --apply --dry-run
```
