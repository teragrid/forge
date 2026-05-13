# Contributing to Forge

Thanks for considering a contribution! Forge is **spec-first**: nothing lands
without a written spec, a failing test, and a DCO sign-off.  This guide walks
you through the full process from a fresh clone to an open PR.

---

## 1. Prerequisites

| Tool | Version | Why |
|------|---------|-----|
| **Go** | ≥ 1.24 | The only runtime Forge uses (CGO disabled by default) |
| **Git** | any recent | DCO sign-off + pre-push hook |
| **Make** | any | Convenience wrapper for the full gate |

Install the project-specific toolchain (one-time):

```bash
make tools   # installs golangci-lint, govulncheck, goimports, gotestsum
make hooks   # registers .githooks/pre-push so git runs the gate automatically
```

---

## 2. Ground Rules

1. **DCO sign-off required.** Every commit must be signed with `git commit -s`.
   The DCO bot blocks unsigned commits automatically.
2. **Two-maintainer review** on every PR (per
   [ADR-022](docs/adr/ADR-022-two-key-enforcement.md)).
   See [CODEOWNERS](CODEOWNERS) for who to request.
3. **Spec hierarchy:** `FORGE_FRAMEWORK_SPEC.md` → `ARCHITECTURE.md` → ADRs →
   code. Deviating from any layer requires updating that layer first.
4. **No code without a failing test.** See §4 below.

---

## 3. Local Dev Loop

```bash
make tools     # one-time: install golangci-lint, govulncheck, goimports, gotestsum
make hooks     # one-time: register the repo's pre-push quality gate
make fmt       # gofmt + goimports
make lint      # golangci-lint (includes gosec, errcheck, gocritic)
make test      # go test -race ./...
make check     # full gate: fmt + vet + lint + build + test + vuln + mod verify
make build     # produces ./bin/forge
make all       # lint + test + build
```

The **pre-push hook** (`make hooks`) runs the same 7 checks as `make check`
automatically before every push.  A failing push prints which check failed and
how to fix it.  Emergency bypass (avoid this):

```bash
SKIP_PRE_PUSH=1 git push
```

The full CI gate is encoded in
[`.github/workflows/ci.yml`](.github/workflows/ci.yml).  If `make check` is
green locally on a fresh clone, CI will be green too.

---

## 4. Test Design First (mandatory)

Before you (or your AI) write a single line of feature code, produce a brief
test design that covers the applicable points from this 9-point matrix:

| # | Category | What to check |
|---|----------|---------------|
| 1 | **Happy path** | the intended scenario succeeds end-to-end |
| 2 | **Boundary** | empty/null, zero, max, min, exactly-at-threshold, off-by-one |
| 3 | **Negative** | invalid input, unauthorised access, wrong configuration |
| 4 | **Idempotency** | same operation twice yields identical final state without error |
| 5 | **Concurrency** | two concurrent writers; out-of-order event arrival |
| 6 | **Cross-tenant / authz** | user A cannot read or modify user B's data |
| 7 | **Regression** | the original bug's exact reproduction must be a permanent test |
| 8 | **Data-accuracy** | real row/event inserts → query back → assert numeric/structural correctness |
| 9 | **False-positive guard** | at least one case where the check **must not** trigger |

> **Vibe-Coding Tip:** Paste this table into your AI prompt and ask it:
> *"Act as a QA expert. Before writing the fix, map out which of these 9
> categories apply and write the failing tests first."*

CI enforces that test file timestamps are ≤ the production file timestamps
they cover.  Write the test first, commit, then write the code.  Ensure
`go test -race ./...` is clean before pushing.

---

## 5. Adding a New CLI Verb

Each `forge <verb>` command lives in its own isolated package:

1. **Update the spec** — open or edit the relevant section of
   `FORGE_FRAMEWORK_SPEC.md`.
2. **Open an ADR** if the verb introduces a new external dependency, network
   contract, or breaking behaviour.
3. **Create the package** — `internal/cli/cmd<verb>/`.
4. **Register it** — add one line to `internal/cli/root.go`:
   `root.AddCommand(cmd<verb>.New())`.
5. **Write tests first** — unit tests in `cmd<verb>/<verb>_test.go` + an E2E
   fixture in `internal/cli/journey_test.go`.
6. **Update the task tracker** —
   [`tasks/DEVELOPMENT_TASKS.md`](tasks/DEVELOPMENT_TASKS.md) acceptance +
   test-case rows.

---

## 6. Plugin Contributions

Plugins are WASM components hosted on `wazero` (per
[ADR-002](docs/adr/ADR-002-plugin-runtime.md)).  They may be authored in any
language that compiles to the WASM Component Model — Go (via TinyGo), Rust,
JavaScript, Python, C/C++.  See
[`docs/PLUGIN_AUTHORING.md`](docs/PLUGIN_AUTHORING.md) for the full walkthrough.

---

## 7. Reporting Security Issues

See [`docs/SECURITY.md`](docs/SECURITY.md) for the private disclosure process.
**Do not** open a public issue for vulnerabilities — use the private security
advisory flow instead.

---

## 8. Pre-Ship Checklist

Before opening a PR, work through [`CHECKLIST.md`](CHECKLIST.md).  It maps
every gate from spec §16.5.4 to a concrete local command so you can verify
green before pushing.  The pre-push hook runs the same gates automatically.

---

## 9. License

By contributing you agree your contributions are licensed under the
[Apache License 2.0](LICENSE).
