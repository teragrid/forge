# forge

> The LLM-native framework that makes AI-generated code survive contact with real users — security, multi-tenancy, audit, and observability built in, not bolted on.

**Status:** Pre-RFC / M0 Bootstrap. Not yet usable. See [DEVELOPMENT_PLAN.md](DEVELOPMENT_PLAN.md) for the milestone roadmap.

## Spec-driven repo

This repo is **spec-first**. Every feature lands as: `spec` → ADR → red test → green code → docs.

| Doc | Purpose |
|-----|---------|
| [FORGE_FRAMEWORK_SPEC.md](FORGE_FRAMEWORK_SPEC.md) | Product specification (v0.10.6). |
| [ARCHITECTURE.md](ARCHITECTURE.md) | System architecture (tier model, NFRs, ADR index). |
| [THREAT_MODEL.md](THREAT_MODEL.md) | STRIDE threat model + mitigations. |
| [DEVELOPMENT_PLAN.md](DEVELOPMENT_PLAN.md) | Per-milestone delivery plan (M0–M3). |
| [TEST_PLAN.md](TEST_PLAN.md) | Test strategy + coverage gates. |
| [GO_TO_COMMUNITY_PLAN.md](GO_TO_COMMUNITY_PLAN.md) | OSS launch + governance plan. |
| [adr/](adr/) | Accepted/Proposed Architecture Decision Records. |
| [tasks/](tasks/) | Task trackers (ARCHITECTURE_TASKS, DEVELOPMENT_TASKS, TEST_TASKS, LAUNCH_TASKS). |

## Tech stack (resolved)

Per [ADR-001](adr/ADR-001-implementation-language.md) and [ADR-002](adr/ADR-002-plugin-runtime.md):

- **Language:** Go (`go 1.24`, `CGO_ENABLED=0` default)
- **CLI:** [`cobra`](https://github.com/spf13/cobra) + [`viper`](https://github.com/spf13/viper)
- **WASM plugin host:** [`wazero`](https://github.com/tetratelabs/wazero) (pure-Go); `wasmtime-go` reserved as `-tags forge_wasmtime` escape hatch
- **Logging/tracing:** `log/slog` + OpenTelemetry-Go
- **Tests:** `go test` + [`gotestsum`](https://github.com/gotestyourself/gotestsum) + golden files; `-race` mandatory in CI
- **Lint:** [`golangci-lint`](https://golangci-lint.run/) (staticcheck, govet, gosec, errcheck, ineffassign, gocritic) + `gofmt` + `goimports`
- **Supply-chain:** `govulncheck` + `go mod verify` + `syft` SBOM at release; `go-licenses` audit

## Quickstart (contributors)

```bash
git clone https://github.com/teragrid/forge.git
cd forge
make tools     # one-time: golangci-lint, govulncheck, goimports, gotestsum
make all       # lint + test + build
./dist/forge --version
```

## Layout

```
cmd/forge/        # CLI entry point (main package)
internal/         # Internal packages (not importable by consumers)
  cli/            # cobra command tree
pkg/              # (reserved) Public Go API surface
adr/              # Architecture Decision Records
tasks/            # Task trackers per role
.github/          # CI/CD workflows
```

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). All commits must be DCO-signed (`git commit -s`).

## License

Apache-2.0 — see [LICENSE](LICENSE) and [NOTICE](NOTICE).
