# forge report

Generate HTML/Markdown compliance reports.

## Synopsis

```
forge report [--root <path>] [--format html|md] [--out <file>]
```

## Description

`forge report` generates a compliance report covering scan findings, audit
history, LLM spend, and gate results.

## Examples

```bash
forge report
forge report --format html --out report.html
forge report --format md --out REPORT.md
```
