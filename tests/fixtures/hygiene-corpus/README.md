# tests/fixtures/hygiene-corpus — Hygiene pattern test fixtures (DEV-M0-32)

This directory contains ≥30 fixture files that represent every hygiene pattern
recognised by `forge clean`. Tests in `internal/cli/cmdclean/` use this corpus
to verify that `forge clean --dry-run` correctly identifies candidates across
all pattern families.

## File categories

| Directory | Pattern family | Forge scratch match |
|---|---|---|
| `scratch-files/` | `_scratch_*` files | Yes |
| `tmp-files/` | `*.tmp.*` files | Yes |
| `forge-scratch/` | `.forge/scratch/**` | Yes |
| `secret-files/` | `.env`, `secrets.json`, etc. | Secret guard |
| `key-files/` | `*.pem`, `*.key`, `id_rsa`, etc. | Secret guard |
| `llm-output/` | LLM-generated draft files (named `gpt-*`, `claude-*`) | Yes |

## Usage in tests

```go
fixtureDir := filepath.Join("testdata", "hygiene-corpus")
// or use the shared corpus:
fixtureDir := filepath.Join("..", "..", "..", "tests", "fixtures", "hygiene-corpus")
```

## Adding new patterns

1. Add the new file to the appropriate subdirectory.
2. Update this README's table.
3. Ensure the file content is clearly fake/test data (no real credentials).
