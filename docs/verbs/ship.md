# forge ship

Run all CI gates and publish a release.

## Synopsis

```
forge ship [--root <path>] [--dry-run] [--tag <version>] [--skip-gates <list>]
```

## Gates

1. `forge scan` — must be clean
2. `go vet ./...` — must produce no output
3. `go test ./...` — all tests must pass
4. Budget check — spend must be under limit
5. Manifest validation — verbmeta registered for all verbs
6. Tag and push — `git tag vX.Y.Z && git push origin vX.Y.Z`

## Examples

```bash
forge ship --tag v1.2.3
forge ship --dry-run
```
