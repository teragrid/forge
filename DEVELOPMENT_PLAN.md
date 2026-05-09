# Forge — Development Plan

> Companion to `FORGE_FRAMEWORK_SPEC.md` v0.10.6, `ARCHITECTURE.md` v0.1, and `TEST_PLAN.md` v0.1.
> Status: **Draft / Pre-RFC**.

The plan is organized around the spec's four milestones (§6 Roadmap M0–M3) and enforces the spec's quality bar (§11.2 #12 "Spec before test", §16.5.4 universal CI gates). Test investments are detailed in the companion [TEST_PLAN.md](TEST_PLAN.md).

---

## 0. Operating principles

1. **The framework ships through itself.** Every Forge feature is built using `forge ship` once the workflow exists (§16.5.2 dogfood). Pre-`ship`, we use a manually-followed checklist that mirrors the same five checkpoints.
2. **No code without a failing test.** Even foundational code (config loader, CLI parser) lands as: `spec.md` → red test → green code. The pre-`ship` checklist enforces it.
3. **Every PR is a regression case for the next PR.** Every bug fix adds a test that fails on the pre-fix code. The eval harness accumulates these scenarios.
4. **Performance is a test, not a hope.** NFR budgets (Architecture §14) are asserted in CI from M0 W3 onward.
5. **Security gates run from day one.** Even the hello-world CLI binary passes `forge scan all` (initially against itself, bootstrapped).
6. **Repo hygiene is a first-class output.** Every checkpoint runs `forge clean --check`; LLM-generated scratch never leaves a contributor's machine (spec §4 hygiene, §16.5.4 #11).

---

## 0.1 Tech-stack baseline (resolved by ADRs)

The stack below is the contractual baseline for every task in this plan. Any deviation requires an ADR amendment.

| Layer | Choice | ADR |
|-------|--------|-----|
| Implementation language | **Go** (current release; min version pinned via `go.mod` `toolchain` + `go` directives; `CGO_ENABLED=0` default) | [ADR-001](adr/ADR-001-implementation-language.md) |
| WASM plugin host | **`wazero`** (pure-Go); `wasmtime-go` reserved as build-tag-gated escape hatch (`-tags forge_wasmtime`) | [ADR-002](adr/ADR-002-plugin-runtime.md) |
| CLI / config | **`cobra`** + **`viper`** | ADR-001 |
| Concurrency | stdlib **goroutines + `context`** (no third-party async runtime) | ADR-001 |
| Logging / tracing | **`log/slog`** + **OpenTelemetry-Go** | ADR-001 |
| Tests | **`go test`** + **`gotestsum`** + golden-file snapshots; **`-race`** mandatory in CI | ADR-001, TEST-01 |
| Lint / format | **`golangci-lint`** (staticcheck, govet, gosec, errcheck, ineffassign, gocritic) + **`gofmt`** + **`goimports`** | ADR-001 |
| Supply-chain | **`govulncheck`** + **`go mod verify`** + **`syft`** SBOM at release | ADR-001, ADR-008 |
| License audit | **`go-licenses check`** in CI | [ADR-008](adr/ADR-008-license.md) |
| Repo layout | `cmd/forge/`, `internal/`, `pkg/` (standard Go project layout) | ADR-001 follow-up |

---

## 1. Milestone overview

| Milestone | Theme | Exit criteria | Indicative duration |
|-----------|-------|---------------|---------------------|
| **M0 — Bootstrap** | Single-binary CLI, foundation services, manual `ship` checklist | `forge new` + `forge ship --quick` work end-to-end on one reference app; CI green; binary published | 6–10 weeks |
| **M1 — Workflow & Scan** | `forge ship` automated, scan engine + 4 scanner families, plugin loader, LLM gateway with one provider | `forge ship` ships a real change in ≤5 min; 4 scanners block CI on a seeded vuln | 8–12 weeks |
| **M2 — Ecosystem** | Plugin Registry, T2 adapters (2 LLM providers, 2 deploy targets), learning loop client, eval harness | 3 community plugins published; learning loop digests one nightly cycle | 10–14 weeks |
| **M3 — Hardening & 1.0** | Threat model closed, perf budgets locked, docs site live, RFC process operational, contribution standards enforced in CI | All §16.5.4 gates active; v1.0.0 release | 8–12 weeks |

---

## 2. Workstreams (run in parallel within each milestone)

| ID | Workstream | Lead role | Spec anchors |
|----|-----------|-----------|--------------|
| WS-A | Foundation & CLI grammar | Core engineer | §4 Command Surface, Arch §4–§5 |
| WS-B | LLM integration & `ship` workflow | Core engineer + LLM specialist | §4 LLM-Native, Arch §6, §9 |
| WS-C | Scan-Fix-Learn loop | Security engineer | §4 scan-and-fix, Arch §10 |
| WS-D | Plugin system & Registry | Platform engineer | §20, Arch §8 |
| WS-E | Testing, eval, NFR budgets | QE | §13, Arch §14 |
| WS-F | Docs, RFC repo, instructions packs | Tech writer + Founder | §11.2, §16.2 |
| WS-G | DevSecOps (CI/CD, signing, release) | DevSecOps | Arch §13, §15 |
| WS-H | Community standards enforcement | Maintainer + Founder | §16.5 |

---

## 3. Per-milestone development plan

### 3.1 M0 — Bootstrap (W1–W10)

| Week | WS-A Foundation | WS-B LLM/ship | WS-C Scan | WS-D Plugin | WS-E Testing | WS-F Docs | WS-G DevSecOps |
|------|-----------------|----------------|-----------|-------------|---------------|-----------|----------------|
| 1 | ADR-001 lang decision **(resolved: Go)**; repo skeleton (`cmd/forge`, `internal/`, `pkg/`, `go.mod` w/ toolchain pin) | — | — | — | Test harness scaffold (`go test` + `gotestsum`) | RFC repo bootstrap | CI skeleton (`golangci-lint` + `go test -race` + `govulncheck`) |
| 2 | Config loader, error codes | LLM gateway scaffold (1 provider mock) | — | — | Unit test conventions | First instructions pack | Sigstore signing pipeline |
| 3 | CLI verb router (3 namespaces: `new/doctor/explain`) | — | — | — | NFR benchmarks scaffolded | `forge --help` doc-gen | Release pipeline dry-run |
| 4 | `forge new` happy path; `forge clean` MVP (manifest-based) | — | — | — | `new-app` eval scenario | Quickstart doc | Brew/scoop tap |
| 5 | Secrets handling, audit ledger | Manual `ship` checklist drafted | — | — | Secret-redaction test (seeded) | "How we work" doc | — |
| 6 | `doctor`, `explain`, `--json` schema | LLM provider interface frozen | — | — | Contract test for `ILlmProvider` | RFC #1: ship workflow | — |
| 7 | — | One real provider plugin (OpenAI or Claude) | — | — | Provider integration test (live, gated) | — | Telemetry opt-in flag plumbed |
| 8 | — | `forge ship --quick` (one-step) MVP | — | — | `ship-reference` eval | — | — |
| 9 | Bug-fix sweep | E2E manual on reference app | — | — | Regression cases for every bug | — | First public release artifact |
| 10 | M0 buffer | M0 buffer | — | — | M0 acceptance suite | M0 release notes | M0 binary published |

**M0 exit criteria:**
- `forge new my-app && cd my-app && forge ship --quick "add hello endpoint"` produces a running, tested change.
- `forge clean --check` green at end of every reference run (no orphan files).
- All §16.5.4 gates 1–8 enforced (gates 9–11 deferred to M1).
- Binary downloadable + signature verifiable.
- Docs site has Quickstart + first 3 RFCs.

### 3.2 M1 — Workflow & Scan (W11–W22)

Headline deliverables:
- Full 5-checkpoint `forge ship` (Spec → Test → Breakdown → Code → Ship) — replaces the manual checklist.
- Scan engine + 4 scanner families: secrets, RLS, prompt-injection, supply-chain.
- Plugin loader (WASM-based) + 2 in-tree plugins as proofs.
- LLM gateway: caching, tier routing, budget guard.
- Convention linter (`forge lint`) + first instruction pack of defaults.
- `forge clean` integrated as a `ship` checkpoint (auto-runs after Code, blocks Ship if unmanifested files remain).
- All universal gates (§16.5.4) live in CI.

**M1 exit criteria:**
- Reference app ships a real change via `forge ship` in ≤5 min.
- Seeded vulnerability suite — every scanner catches its target with confidence ≥0.9.
- One external pilot user has shipped a change via `forge ship`.

### 3.3 M2 — Ecosystem (W23–W36)

Headline deliverables:
- Plugin Registry (signed JSON index in Git + CDN).
- T2 adapters: 2 LLM providers, 2 deploy targets (e.g. Fly + Railway), 1 storage adapter.
- Eval harness public; reference scenarios published.
- Learning loop client + opt-in aggregator MVP.
- Remaining 4 scanner families: auth, perf, accessibility, cost.
- `forge upgrade` codemod runner.

**M2 exit criteria:**
- 3 community-authored T3 plugins published to Registry.
- Learning loop completes one nightly aggregation cycle without privacy regression.
- Eval harness used to gate at least one PR's token-budget regression.

### 3.4 M3 — Hardening & 1.0 (W37–W48)

Headline deliverables:
- Closed threat model (`THREAT_MODEL.md`) with mitigations tested.
- Performance budgets locked into CI; regression-blocks active.
- Docs site complete (every public verb, every error code, every extension point).
- RFC process operational with ≥3 accepted RFCs.
- Contribution-standards CI bot live (auto-comments which §16.5.4 gates failed and links to docs).
- v1.0.0 released.

**M3 exit criteria:**
- All NFR budgets (Arch §14) asserted in CI.
- All §16.5 standards enforced automatically (no manual reviewer toil).
- ≥10 maintainers across ≥2 working groups.
- Public announcement (see Go-to-Community plan).

---

## 4. Risks & mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|-------------|
| LLM provider API breaks `ship` flow | M | H | Provider compliance suite; pin model versions in `.forge/locks/llm.lock` |
| WASM plugin model immature on a target OS | M | M | Fallback to in-process plugins for trusted core scanners; gate WASM behind feature flag |
| Convention linter false positives stall adoption | H | M | Confidence thresholds; `--accept-suggestion`; weekly false-positive review |
| Eval costs exceed budget | M | M | Cache + cheap-tier-first routing; fixture-based PR-CI |
| Contributor onboarding too steep | M | H | "ship as a plugin first" path (§16.5.9); first-PR-friendly labels; mentorship roster |
| Single-maintainer bus factor | H | H | Force pair-review on T1; document everything in spec; recruit 2 co-maintainers by end of M1 |
| LLM scratch files trash the repo over time | H | M | `forge clean` mandatory checkpoint in `ship`; CI gate §16.5.4 #11; weekly hygiene digest |

---

*Plan version: 0.1 — companion to spec v0.10.6, architecture v0.1, test plan v0.1.*
