# forge eval

Evaluate LLM quality against test scenarios.

## Synopsis

```
forge eval [--root <path>] [--scenario <file>] [--json] [--quarantine]
```

## Description

`forge eval` runs the configured LLM against a set of scenario files and
reports pass/fail with quality metrics. Failed scenarios can be quarantined
(max 14 days) while fixes are developed.

## Examples

```bash
forge eval
forge eval --scenario tests/fixtures/scenarios/
forge eval --json | jq '.results'
```
