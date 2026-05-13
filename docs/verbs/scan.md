# forge scan

Detect security issues, lint drift, and hygiene violations.

## Synopsis

```
forge scan [--root <path>] [--fix] [--json] [--only security|lint|hygiene]
```

## Scan types

| Type | Description |
|------|-------------|
| `security` | OWASP Top 10 + custom rules via `forge scan security` |
| `lint` | golint, go vet drift |
| `hygiene` | manifest, secret leaks, `.gitignore` completeness |
| `secrets` | API keys, tokens, credentials in staged changes |

## Examples

```bash
forge scan
forge scan --only security
forge scan --json | jq '.findings'
```
