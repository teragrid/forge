# ADR-001 — Implementation language

- **Status:** Proposed
- **Tracker:** ARCH-DEC-01
- **Spec/Arch anchor:** Arch §13 ADR-001, NFRs §14 (cold-start ≤ 80 ms, RSS ≤ 60 MB)
- **Decision date:** TBD
- **Deciders:** Founder + 1 reviewer
- **Consulted:** Core engineering, plugin WG

## Context

Forge ships as a single static binary that must:

- Cold-start in ≤ 80 ms (§14).
- Hold RSS ≤ 60 MB at idle (§14).
- Run a high-throughput scan engine (≥ 100 files/sec, §14) with file-IO-bound and CPU-bound modes.
- Cross-compile cleanly to `linux/{x64,arm64}`, `darwin/{x64,arm64}`, `windows/{x64,arm64}`.
- Embed a WASM runtime (per ADR-002) without dragging in a heavy GC.
- Run securely in CI under restricted sandboxes (no JIT-only deployment surfaces).
- Be maintainable by a small core team (≤ 5 engineers) plus community contributors.
- **Be fluently producible by LLM-assisted ("vibe-coding") contributors**, who will author both T3 plugins *and* T1/T2 core modules. The corpus + grammar of the chosen language directly bounds the rate at which AI-assisted PRs land cleanly without compile-fix-loop churn.

Tech-stack choice is one of the highest-leverage decisions in the project — it constrains hiring, performance ceiling, supply-chain posture, plugin ergonomics, and 5-year maintenance cost. This ADR therefore evaluates **five** realistic candidates across **eleven** dimensions before recommending one.

### Candidates

| # | Language | Toolchain anchor |
|---|----------|------------------|
| A | **Rust** (stable) | `cargo` + `rustup`, MSRV pinned |
| B | **Go** ✅ *(chosen — see Decision)* | `go` 1.22+, modules, `go.mod` toolchain directive |
| C | **Zig** | `zig` 0.13+ (pre-1.0) |
| D | **TypeScript on Node/Bun** | Node 20 LTS or Bun 1.x, `pkg`/`bun build --compile` for single binary |
| E | **C++** (C++20) | CMake + Conan/vcpkg |

Excluded without scoring: Python (cold-start budget infeasible without compilation); .NET AOT (cross-compile to Apple Silicon is rough; smaller systems-CLI mindshare); OCaml/Haskell (hireable pool too small for OSS contributor model).

---

## Evaluation framework

Each dimension is scored **1 (worst) – 5 (best)** with a fixed weight reflecting how decisive that dimension is for Forge specifically. Dimensions and weights are deliberately published so the decision is replayable when conditions change.

| Dim | Dimension | Weight | Why this weight for Forge |
|-----|-----------|:-----:|---------------------------|
| D1 | Cold-start & runtime perf | **5** | Hard NFR (§14 80 ms / 60 MB / 100 files/s); miss = product fails |
| D2 | Single-binary distribution | **5** | Spec §4 distribution layer + ADR-003 |
| D3 | WASM host maturity | **5** | Plugin runtime (ADR-002) is core to extensibility story |
| D4 | Supply-chain & security posture | **4** | Forge sells trust; CVEs in deps = brand damage |
| D5 | Community adoption & contributor pool | **4** | OSS sustainability; contributor velocity |
| D6 | Speed to market / dev cycle | **4** | Pre-1.0 must ship in 6–9 months |
| D7 | Bug rate / safety | **4** | Memory bugs in a CLI that writes user files = catastrophic |
| D8 | Maintenance cost (5-year) | **3** | Smaller team = high per-engineer load |
| D9 | Cross-platform breadth & quality | **3** | 6 target triples; Windows is real |
| D10 | Tooling & build ergonomics | **2** | Daily DX; reversible |
| D11 | Market competitors using it | **2** | Signal, not gospel; reduces unknown-unknowns |
| D12 | LLM / vibe-coding fluency | **4** | Both plugins **and** core modules will be authored vibe-coded; LLM corpus size + language simplicity directly drives merged-PR throughput |

**Max possible weighted score:** 45 × 5 = **225**.

---

## Dimension-by-dimension comparison

### D1 — Cold-start & runtime performance (weight 5)

| Lang | Score | Evidence |
|------|:----:|----------|
| Rust | **5** | Static-linked binary cold-starts in ~5–15 ms; no GC pauses; SIMD via `std::simd`. `ripgrep`, `fd`, `bat`, `uv` routinely beat 80 ms with margin. |
| Go  | 3 | Cold-start ~20–40 ms typical; GC stop-the-world rare but variance is real under scan load; meets budget but consumes most of it. |
| Zig | **5** | Same league as Rust; smaller binaries; explicit allocators. |
| TS  | 1 | Node cold-start 50–150 ms before user code; Bun better (~20–40 ms) but compiled-binary mode is new and large. |
| C++ | **5** | Best-in-class when tuned; LTO + `-Os` produces small, fast binaries. |

### D2 — Single-binary distribution (weight 5)

| Lang | Score | Evidence |
|------|:----:|----------|
| Rust | **5** | Native; `cargo build --release` → static binary; `cross` for awkward triples. |
| Go  | **5** | Native; gold-standard cross-compile (`GOOS`/`GOARCH`). |
| Zig | **5** | Native; `zig build` cross-compiles natively without a sysroot. |
| TS  | 2 | Requires `pkg`/`bun --compile`/`nexe`; binaries 40–80 MB; signing flow rough. |
| C++ | 3 | Possible but painful: glibc ABI, `manylinux`-style baselines, brittle CMake. |

### D3 — WASM host maturity (component model) (weight 5)

| Lang | Score | Evidence |
|------|:----:|----------|
| Rust | **5** | `wasmtime` is written in Rust → first-class API + best component-model support; Bytecode Alliance tracks ahead. |
| Go  | 3 | `wazero` is excellent for core-WASM but component-model coverage trails wasmtime by 6–12 months. |
| Zig | 2 | No mature embeddable wasmtime binding; would need C-API plumbing. |
| TS  | 2 | Browser-class engines fine for core WASM; component model partial. |
| C++ | 4 | wasmtime C-API is solid; second-class compared to native Rust binding. |

### D4 — Supply-chain & security posture (weight 4)

| Lang | Score | Evidence |
|------|:----:|----------|
| Rust | 4 | `cargo` is centralised → easier SBOM; `cargo-deny` / `cargo-audit` / `cargo-vet` mature; transitive-dep sprawl is a real concern. |
| Go  | **5** | Smallest median dep tree of the five; checksum DB; `govulncheck` first-party. |
| Zig | 3 | No central registry; vendoring norm — clean but immature ecosystem hygiene tools. |
| TS  | 1 | npm transitive trees in the thousands; well-documented supply-chain attack surface (event-stream, ua-parser-js, xz-style risks). |
| C++ | 2 | Conan/vcpkg fragmentation; CVE bookkeeping per-distro is painful. |

### D5 — Community adoption & contributor pool (weight 4)

| Lang | Score | Evidence (rough public figures, 2025) |
|------|:----:|---------------------------------------|
| Rust | 4 | StackOverflow "most-loved" 8 years running; ~3M devs; surging in CLIs/infra. |
| Go  | **5** | ~4–5M devs; dominant for OSS CLIs/devops; lowest onboarding ramp. |
| Zig | 1 | <100 k devs; pre-1.0; thin contributor pool. |
| TS  | **5** | Largest dev pool by far; near-universal familiarity. |
| C++ | 3 | Huge total pool, but small overlap with "wants to contribute to a CLI tool on weekends". |

### D6 — Speed to market / dev cycle (weight 4)

| Lang | Score | Evidence |
|------|:----:|----------|
| Rust | 3 | Slower compile loop; borrow-checker cost amortises but is real in months 1–3. |
| Go  | **5** | Famously fast iteration; minimal type-system overhead. |
| Zig | 2 | Fast compile, but missing batteries (HTTP, async) cost time. |
| TS  | 4 | Vast library ecosystem accelerates; DX is excellent. |
| C++ | 1 | Slow compile + manual memory + brittle build = months of yak-shaving. |

### D7 — Bug rate / memory safety (weight 4)

| Lang | Score | Evidence |
|------|:----:|----------|
| Rust | **5** | Memory-safe by default; ~70 % of CVEs at MS/Google were memory issues — Rust eliminates this class. |
| Go  | 4 | GC eliminates use-after-free; data races still possible (race detector helps). |
| Zig | 3 | Manual memory; safer than C but no borrow-checker. |
| TS  | 3 | GC + types catch a lot, but `any`/runtime errors remain common. |
| C++ | 1 | Decades of evidence on the memory-bug rate; modern C++ helps but doesn't fix. |

### D8 — Maintenance cost over 5 years (weight 3)

| Lang | Score | Evidence |
|------|:----:|----------|
| Rust | 4 | Refactors are safer; compiler catches breakage early; edition system manages churn. |
| Go  | **5** | Stable language, almost no breaking changes; "boring" by design. |
| Zig | 2 | Pre-1.0 → expect breaking changes through the project's first 2 years. |
| TS  | 2 | Ecosystem churn (build tooling, framework fashions) imposes ongoing tax. |
| C++ | 3 | Standard is stable; toolchain + build-system churn drains time. |

### D9 — Cross-platform breadth & quality (weight 3)

| Lang | Score | Evidence |
|------|:----:|----------|
| Rust | 4 | All 6 target triples Tier-1 or Tier-2; Windows is first-class. |
| Go  | **5** | Cleanest cross-compile of any language; identical experience across OSes. |
| Zig | **5** | Cross-compile is a flagship feature. |
| TS  | 3 | Code is portable; binary packaging less so (signing/notarisation). |
| C++ | 2 | Each platform demands custom CMake glue + per-OS CI. |

### D10 — Tooling & build ergonomics (weight 2)

| Lang | Score | Evidence |
|------|:----:|----------|
| Rust | **5** | `cargo` + `clippy` + `rustfmt` + `rust-analyzer` — best-in-class integrated story. |
| Go  | **5** | `go build` / `go test` / `gofmt` — minimalist and uniform. |
| Zig | 3 | `zig build` is elegant but minimal; LSP improving. |
| TS  | 3 | Many overlapping tools (tsc/swc/esbuild/biome); fragmentation cost. |
| C++ | 1 | CMake/Bazel/Meson; clang-tidy/clangd; great if curated, painful otherwise. |

### D11 — Market competitors / prior art (weight 2)

| Lang | Score | Comparable shipped projects |
|------|:----:|-----------------------------|
| Rust | **5** | `uv` (Astral), `ruff`, `biome`, `deno`, `turbopack`, `rustup`, `gitoxide`, `bat`, `ripgrep`. Prior art for "fast cross-platform dev CLI" is overwhelmingly Rust in 2024–2026. |
| Go  | **5** | `gh`, `terraform`, `kubectl`, `hugo`, `helm`, `cobra`-built CLIs. Equally proven, slightly different tradeoff (devops-flavoured). |
| Zig | 2 | `bun` (uses Zig); few mainstream CLIs. |
| TS  | 3 | `npm`, `pnpm`, `vercel`, `wrangler`. Proven for package managers; less so for perf-critical CLIs. |
| C++ | 3 | `git`, `clang`, `cmake`, `ccache`. Proven, but newer entrants prefer Rust/Go. |

### D12 — LLM / vibe-coding fluency (weight 4)

Forge's contribution model assumes a meaningful share of PRs (plugins **and** core modules) will be vibe-coded — LLM-drafted, human-reviewed, iterated through compile/test loops. The score below reflects two sub-factors:

1. **Corpus density** — how much idiomatic code in this language exists in LLM training sets.
2. **Compile-loop friction** — how often an LLM-drafted patch needs human-mediated rounds to satisfy the type/borrow/lifetime checker.

| Lang | Score | Evidence |
|------|:----:|----------|
| Rust | 3 | Largest pain point for LLMs in CLI work today: borrow-checker, lifetimes, async-trait sharp edges. Drafts compile-clean less often; iterations cost reviewer time. Mitigated by `cargo check` loops in agent harnesses but not eliminated. |
| Go  | **5** | Highest LLM fluency among compiled languages: tiny grammar, no lifetimes, idiomatic patterns are uniform across the corpus. First-pass compile rate from frontier models is consistently the best of the five. |
| Zig | 1 | Tiny corpus; pre-1.0 churn means the corpus that exists is partially wrong; LLMs hallucinate APIs frequently. |
| TS  | **5** | Largest corpus by absolute volume; LLMs are extremely fluent. Dragged down for Forge by D1/D2/D3, but on this axis alone TS is best-in-class. |
| C++ | 3 | Big corpus but template/SFINAE/lifetime errors are exactly the class LLMs handle worst; mid-loop yak-shaving common. |

---

## Weighted scorecard

```
score(lang) = Σ ( dim_score(lang, d) × weight(d) )    for d in D1..D12
```

| Dim (weight) | Rust | Go | Zig | TS | C++ |
|--------------|:----:|:--:|:---:|:--:|:---:|
| D1 perf (5)            | 25  | 15  | 25  | 5   | 25  |
| D2 single-binary (5)   | 25  | 25  | 25  | 10  | 15  |
| D3 WASM host (5)       | 25  | 15  | 10  | 10  | 20  |
| D4 supply-chain (4)    | 16  | 20  | 12  | 4   | 8   |
| D5 community (4)       | 16  | 20  | 4   | 20  | 12  |
| D6 speed-to-market (4) | 12  | 20  | 8   | 16  | 4   |
| D7 bug rate (4)        | 20  | 16  | 12  | 12  | 4   |
| D8 5-yr maint (3)      | 12  | 15  | 6   | 6   | 9   |
| D9 cross-plat (3)      | 12  | 15  | 15  | 9   | 6   |
| D10 tooling (2)        | 10  | 10  | 6   | 6   | 2   |
| D11 prior art (2)      | 10  | 10  | 4   | 6   | 6   |
| D12 LLM fluency (4)    | 12  | 20  | 4   | 20  | 12  |
| **TOTAL / 225**        | **195** | **201** | **131** | **124** | **123** |

**Go wins outright (201 vs Rust 195).** D12 — LLM/vibe-coding fluency, weighted at 4 because both plugins **and** core modules will be vibe-coded — inverts the previous 2-point Rust lead into a 6-point Go lead. The other three remain eliminated on aggregate.

---

## Tie-break: Rust vs Go

With D12 included Go wins outright (201 vs 195) so a strict tie-break is no longer required. The Rust-favouring axes (D1 cold-start under load, D3 wasmtime component-model host, D7 data-race elimination) remain real and are tracked under §Reversal trigger; the Go-favouring axes (D5 community, D6 speed-to-market, D8 5-yr churn, D9 cross-compile cleanliness, **D12 vibe-coding fluency**) are structural and not tunable.

| Tie-break axis | Verdict | Reasoning |
|----------------|---------|-----------|
| Structural-NFR (D1 + D3 + D7) | Rust | Go GC variance vs 80 ms; wazero component-model lag; data-race class. Mitigations exist (GOGC, `sync.Pool`, `-race` gate) and are bench-measurable. |
| Community-velocity (D5 + D6 + D8 + D9) | **Go** | Larger contributor pool; faster iteration; near-zero language churn over 5 yrs; cleanest cross-compile across all 6 target triples without cgo. |
| **Vibe-coding contribution model (D12)** | **Go** | Forge's stated contribution model assumes LLM-assisted PRs for both plugins **and** core modules. Go has the highest first-pass compile rate among compiled candidates; Rust's borrow-checker is the dominant friction surface for AI-drafted patches. This axis is the decisive Go-favouring tilt. |
| Pre-1.0 ship-or-die context | **Go** | A single-founder + small-WG project pre-1.0 is bottlenecked on contributor activation, not on shaving GC pauses. The §0 "community-first" posture (`forge/FORGE_FRAMEWORK_SPEC.md` §0) makes contributor-pool size + LLM throughput strategic, not tactical, axes. |
| Mitigability of the Rust-favouring axes | **Go** | D1 GC variance → GOGC + `sync.Pool` + work-stealing; D3 wazero gap → closable as component-model spec stabilises (escape hatch: `wasmtime-go` behind a feature flag); D7 data-race risk → bounded by `go test -race` + concurrency review checklist. The Go-favouring axes are not similarly tunable in the other direction. |

**Decision:** Go. The vibe-coding axis (D12) plus community-velocity together dominate the residual Rust-favouring NFR risks, and those risks are explicitly watched in §Reversal trigger.

## Decision

Forge will be implemented in **Go** (current Go release, minimum version pinned via `go.mod`'s `toolchain` and `go` directives), using:

- **`wazero`** as the WASM host (pure Go, no cgo — preserves D9 cross-compile cleanliness; component-model coverage tracked per ADR-002).
- **`cobra` + `viper`** for CLI parsing and config.
- Standard-library **goroutines + `context`** for concurrency (no third-party async runtime).
- **`log/slog`** + **OpenTelemetry-Go** for structured logs and traces.
- **`go test`** + **`gotestsum`** + golden-file snapshots for tests.
- **`golangci-lint`** (with `staticcheck`, `govet`, `gosec`) and **`gofmt`** + **`goimports`** for formatting.
- **`govulncheck`** + **`go mod verify`** + **`syft`** SBOM for supply-chain hygiene.

No cgo in the default build (CGO_ENABLED=0). Any cgo dependency requires an ADR amendment so the cross-compile guarantee stays auditable.

### Scope

- **In scope:** the `forge` CLI binary, all T1 core libraries, the WASM host.
- **Out of scope:** plugin authors' choice of language (any language compiling to the WASM component model is fine — see ADR-002).

### Reversal trigger (when to revisit this ADR)

Revisit if **any** of the following becomes true (these are the explicit Rust-favouring axes accepted as risks above):

- **NFR breach (D1):** median scan-engine cold-start or per-file latency regresses past §14 budgets over 4 consecutive weekly bench runs and a documented GC-tuning effort (GOGC, `sync.Pool`, work-stealing) cannot recover the budget.
- **Concurrency-correctness incident (D7):** ≥ 1 production-impacting data-race CVE confirmed against `forge` core in any 12-month window despite mandatory `-race` CI, OR systemic race patterns in the core that lint cannot mechanically prevent.
- **WASM host blocker (D3):** the plugin contract (ADR-002) requires component-model features that wazero will not ship within 2 release cycles, AND `wasmtime-go` (cgo) is rejected because cgo cost-of-cross-compile is unacceptable.
- **Memory-budget breach:** RSS at idle exceeds §14's 60 MB ceiling under representative workloads and tuning cannot recover it.

Reversal would be evaluated as a fresh ADR that supersedes this one. Plausible reversal paths: (a) rewrite hot loops in a Rust subprocess called over IPC; (b) full rewrite to Rust if multiple triggers fire concurrently. Both are explicit, not implicit.

## Consequences

### Positive

- Largest practical contributor pool of the five candidates (D5) → directly serves the spec §0 community-first posture.
- **Highest first-pass LLM compile rate among compiled candidates (D12)** → vibe-coded plugin and core PRs land with the least compile-fix-loop overhead.
- Fastest median PR-cycle time (D6) → Forge can hit pre-1.0 milestones with a small core team.
- Cleanest cross-compile across all 6 §14 target triples without cgo (D9) → release pipeline (ADR-003) stays simple.
- Smallest median dependency tree of the candidates (D4) → SBOM/`govulncheck`/`go mod verify` cover the supply chain with first-party tools.
- ~Zero language-level breaking change in 5 yrs (D8) → 5-year maintenance cost is the lowest of the candidates.
- Strong prior-art bench (`gh`, `terraform`, `kubectl`, `hugo`, `helm`, `cosign`) reduces unknown-unknowns for a CLI of this shape.

### Negative / accepted trade-offs

- **Cold-start headroom is tighter** than Rust (D1): meets §14's 80 ms budget but with less margin under burst load. Mitigated by: (a) `CGO_ENABLED=0` static binary, (b) GOGC tuning + `sync.Pool` in scan engine, (c) bench gate enforces > 5 % regression block (§16.5.6), (d) reversal trigger above watches sustained breach.
- **WASM component-model coverage in `wazero` trails `wasmtime`** by ~6–12 months (D3). Mitigated by: (a) ADR-002 amended to make `wazero` the runtime with a documented feature-coverage matrix; (b) component features beyond wazero's coverage are deferred or polyfilled; (c) `wasmtime-go` (cgo) reserved as escape hatch behind a feature flag — never the default build.
- **Data-race class is not eliminated by the language** (D7). Mitigated by: (a) `go test -race` is a mandatory CI gate on every PR; (b) `goroutine`-spawning code reviewed against an explicit "no shared mutable state without channel/mutex" checklist documented in CONTRIBUTING.md; (c) static-analyser `staticcheck` + `gosec` in `golangci-lint`.
- **GC pauses can leak into latency tails.** Mitigated by: (a) keep allocations off the scan hot path (`sync.Pool` for buffers); (b) p99 cold-start tracked separately from median in the bench gate.
- **Generics ecosystem is younger than Rust's traits** — accepted; Forge's domain (CLI + scan + WASM host) does not lean heavily on generic abstraction.

### Follow-ups created

- DEV-M0-01 — repo skeleton with `go.mod` + standard layout (`cmd/forge`, `internal/`, `pkg/`).
- DEV-M0-04 — pin minimum Go version via `go.mod` `toolchain` + `go` directives; document upgrade cadence.
- DEV-M0-05 — `golangci-lint` config (enable `staticcheck`, `govet`, `gosec`, `errcheck`, `ineffassign`, `gocritic`); `govulncheck` + `go mod verify` in CI.
- DEV-M0-XX — `gotestsum` + race-detector CI gate (`go test -race ./...`).
- DEV-M0-XX — bench harness for cold-start + scan throughput; PR gate per §16.5.6.
- DEV-M0-XX — SBOM generation via `syft` on every release.
- TEST-01 — choose Go test harness (`go test` + `gotestsum` + golden-file snapshots; consider `testify` only where assertion verbosity hurts readability).

## Compliance hooks

- CI gate: `go build` cross-compile matrix across all 6 §14 target triples (`GOOS`/`GOARCH`) with `CGO_ENABLED=0`.
- CI gate: cold-start + scan-throughput bench (`go test -bench` or dedicated harness) fails on regression > 5 % per §16.5.6.
- CI gate: `go test -race ./...` is mandatory on every PR.
- CI gate: `govulncheck ./...` and `go mod verify` on every PR.
- CI gate: `golangci-lint run` with the enabled set above; new lint warnings block merge.
- Lint: `gofmt -d` + `goimports -l` clean.
- Release: `syft` SBOM attached to every GH Release.
- Audit: quarterly contributor-velocity retro AND quarterly NFR-bench review feed the reversal trigger above.

## References

- Arch §13 ADR-001, §14 NFRs.
- Spec §0 community-first posture.
- `wazero` component-model status: <https://wazero.io/>.
- `wasmtime-go` (cgo escape hatch): <https://github.com/bytecodealliance/wasmtime-go>.
- Go scheduler / GC tuning: <https://tip.golang.org/doc/gc-guide>.
- Prior-art Go CLIs: `gh`, `terraform`, `kubectl`, `hugo`, `helm`, `cosign`.
- Counter-evidence (Rust prior art): `uv`, `ruff`, `biome`, `deno`, `gitoxide`, `ripgrep` — on the table for a future reversal ADR if triggers fire.
