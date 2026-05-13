# forge undo

Reverse the last forge operation (ADR-024 reversibility contract).

## Synopsis

```
forge undo [--root <path>] [--op <id>] [--dry-run] [--list]
```

## Description

`forge undo` reverses the most recent forge operation that supports reversal.
Operations that cannot be undone require `--allow-irreversible` at the time
they are run. See ADR-024.

## Examples

```bash
forge undo
forge undo --list
forge undo --op fix-2024-01-15T10:30:00Z
forge undo --dry-run
```
