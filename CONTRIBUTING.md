# Contributing to forge

Thanks for considering a contribution. forge is **spec-first**: nothing lands without a written spec, an ADR (for non-trivial decisions), and a test that would have failed before your change.

## Ground rules

1. **DCO sign-off required.** All commits must use `git commit -s`. The DCO bot blocks unsigned commits.
2. **Two-Maintainer review on every PR** (per [ADR-022](adr/ADR-022-governance.md)). Use [CODEOWNERS](CODEOWNERS) to find the right reviewers.
3. **Follow the spec hierarchy:** `FORGE_FRAMEWORK_SPEC.md` → `ARCHITECTURE.md` → ADRs → code. Diverging from any layer requires updating that layer first.
4. **No code without a failing test.** See `tests/` and the per-task `TC-NN-NN` matrix in `tasks/`.

## Local dev loop

```bash
make tools     # one-time install of golangci-lint, govulncheck, goimports, gotestsum
make fmt       # gofmt + goimports
make lint      # golangci-lint
make test      # go test -race ./...
make build     # produces ./dist/forge
make all       # lint + test + build (the same gates CI runs)
```

The full pre-merge gate set is encoded in [`.github/workflows/ci.yml`](.github/workflows/ci.yml). If `make all && make vuln` is green locally on a fresh clone, CI will be green too.

## Adding a new CLI verb

1. Open or update the relevant section of `FORGE_FRAMEWORK_SPEC.md`.
2. Open an ADR if your verb introduces a new external dependency, contract, or breaking behaviour.
3. Add the package under `internal/cli/<verb>/`.
4. Register it in `internal/cli/root.go` (one-line `root.AddCommand(...)`).
5. Add unit tests + an integration test fixture (per [tasks/TEST_TASKS.md](tasks/TEST_TASKS.md) TEST-02).
6. Update [tasks/DEVELOPMENT_TASKS.md](tasks/DEVELOPMENT_TASKS.md) acceptance + test cases.

## Plugin contributions

Plugins are WASM components hosted on `wazero` (per [ADR-002](adr/ADR-002-plugin-runtime.md)). They may be authored in any language that compiles to the WASM Component Model (Go via TinyGo, Rust, JS, Python, C/C++).

## Reporting security issues

See [docs/SECURITY.md](docs/SECURITY.md) for the private disclosure process. Do **not** open a public issue for vulnerabilities.

## License

By contributing you agree that your contributions are licensed under the [Apache License 2.0](LICENSE).
