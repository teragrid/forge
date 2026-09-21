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

## Placeholders in `generic-bearer`

The built-in `generic-bearer` rule flags a quoted literal of 16+ characters assigned to a name
containing `token`, `secret`, `password` or `api-key`. That is a heuristic, so it also matches test
fixtures (`test_access_token`, `whsec_placeholder`) and documentation (`sbp_your_token_here`).

Since the rule learned to recognise these, a value is **not** reported when it is a *phrase* — two or
more `-`/`_` separated segments, each a plain word (all lower case, all UPPER case or Capitalised,
with at most six trailing digits) — **and** either

- it contains a marker word (`test`, `mock`, `fake`, `dummy`, `example`, `sample`, `placeholder`,
  `changeme`, `invalid`, `your`, `redacted`, `xxx`, `demo`, `fixture`, `stub`), anywhere in the tree, or
- the file is test code (a `test`/`tests`/`__tests__`/`__mocks__`/`mocks`/`e2e` directory, or a
  `*.test.*`, `*.spec.*`, `*_test.*`, `test_*` or `*.mock.*` file).

Opaque values are never excused: anything that mixes upper and lower case or letters and digits inside
one segment, or is a single long segment (live and test-mode Stripe keys, hex, UUIDs, base62, JWT
headers), is still reported in production code **and** in test files. A phrase-shaped value with no
marker (`correct-horse-battery-staple`) is still reported outside test code. Only `generic-bearer` is
affected: the AWS, `sk-`, GitHub-token and private-key-block rules fire everywhere, including tests.

## Waivers

Accept a specific finding with a waiver instead of leaving the gate red. Put YAML files in
`.forge/waivers/` (commit them):

```yaml
- id: W-001
  rule_id: generic-bearer                       # the rule name printed next to the finding
  file_path: src/lib/webhookConfig.ts           # optional; omit to cover the rule in every file
  rationale: >-
    Public webhook verify token that customers type into their Meta app; documented as not a secret.
  approved_by: alice
  expires_at: "2027-03-31"                      # YYYY-MM-DD
```

- `rationale`, `approved_by` and `expires_at` are required. A waiver missing any of them makes the scan
  **fail** rather than silently exempt findings.
- An expired waiver is never honoured. The finding comes back and the `note` says which waiver lapsed.
  To renew, add a new entry; a valid waiver wins over an expired one that also matches.
- `file_path` is matched against the scan-relative, slash-separated path (as printed in the finding).
- Waived findings are removed from `findings`, do not affect `count`, `status` or the exit code, and are
  counted in `waived` (JSON) / `waived:` (text). `forge ship`'s security checkpoint honours the same files.
