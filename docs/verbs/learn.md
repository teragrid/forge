# forge learn

Submit anonymised patterns to the learning loop.

## Synopsis

```
forge learn [--root <path>] [--dry-run]
```

## Description

`forge learn` collects anonymised fix patterns (stripped of project-specific
identifiers and secrets) and queues them for the opt-in telemetry aggregator.
Requires `FORGE_LEARN_OPT_IN=1`.

## Examples

```bash
export FORGE_LEARN_OPT_IN=1
forge learn
```
