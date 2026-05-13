# forge fix

Auto-apply LLM-suggested fixes for scan findings.

## Synopsis

```
forge fix [--root <path>] [--auto] [--dry-run] [--finding <id>]
```

## Description

`forge fix` reads findings from `forge scan` and uses the configured LLM
to generate and apply fixes. With `--auto` it applies all fixes without
prompting. Without `--auto` it presents each fix for review.

## Examples

```bash
forge fix
forge fix --auto
forge fix --finding FORGE-SEC-001
forge fix --dry-run
```
