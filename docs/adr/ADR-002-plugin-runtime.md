# ADR-002 — Plugin runtime

- **Status:** Proposed
- **Tracker:** ARCH-DEC-02
- **Spec/Arch anchor:** Arch §13 ADR-002, Spec §16.5.1 (T3 plugin tier), Arch §15 (sandboxing)
- **Decision date:** TBD
- **Deciders:** Plugin WG lead
- **Consulted:** Core engineering, security WG

## Context

Plugins (T3 tier) are untrusted-by-default code authored by the community. They must:

- Run sandboxed: no ambient FS / network access; capabilities granted explicitly.
- Be language-agnostic so the contributor pool is not bottlenecked by ADR-001's choice of Go.
- Load in ≤ 50 ms per plugin (§14 NFR).
- Be deterministically reproducible from a signed artefact (per ADR-004 registry).
- Survive a panic without taking down the host.

## Decision

Forge will use the **WebAssembly Component Model** running on **`wazero`** (pure-Go, no cgo) as the default plugin runtime. Plugins are distributed as `.wasm` component binaries with a WIT (WebAssembly Interface Type) world named `forge:plugin@1`. Capabilities (FS read, FS write, network, secrets) are passed in as imports; nothing is ambient.

A documented **component-model feature-coverage matrix** (`forge/docs/wasm-feature-matrix.md`, owned by the Plugin WG) tracks which component-model features `wazero` currently supports. Plugin templates and the `forge:plugin@1` WIT world are constrained to features inside that matrix.

**Escape hatch:** `wasmtime-go` (cgo binding to wasmtime) is reserved as a build-tag-gated alternative runtime (`-tags forge_wasmtime`) for any feature that becomes hard-required and is not yet in `wazero`. The default release build is always pure-Go (`CGO_ENABLED=0`); switching the default would require an ADR amendment.

### Scope

- **In scope:** all T3 plugins, including scan rules, deploy adapters, AI providers, hygiene fragments.
- **Out of scope:** T1 core code (linked statically into the binary) and T2 adapters that ship in-repo and can opt into native code paths.

## Alternatives considered

### Option A — In-process dynamic libraries (`.so`/`.dll`/`.dylib`) (rejected)

Pros: zero-overhead calls; native perf.
Cons: no sandbox — a buggy plugin segfaults the host; ABI versioning hell; cross-platform binary distribution is painful; security review burden unbounded.

### Option B — Subprocess + JSON-RPC over stdio (rejected)

Pros: trivial sandbox via OS process boundary; any language.
Cons: per-plugin load + IPC overhead violates the 50 ms budget; capability passing is awkward; harder to share read-only memory for scan corpora.

### Option C — WASM core spec (no component model) (rejected)

Pros: simpler tooling, broader runtime support today.
Cons: interface composition is ad hoc (manual ABI per host); WIT/component model is the evolving standard and worth committing to early.

## Consequences

### Positive

- Sandbox by construction; capability-secure design.
- Language-agnostic plugin authoring (Go via TinyGo, Rust, JS via componentize-js, Python via componentize-py, C/C++).
- Deterministic — required for cassette-based eval (see ADR-005, ADR-023).
- `wazero` is pure Go — zero cgo — preserving ADR-001's cross-compile cleanliness across all 6 §14 target triples.

### Negative / accepted trade-offs

- `wazero` component-model coverage trails `wasmtime` by ~6–12 months in some surfaces; mitigated by (a) the published feature-coverage matrix above, (b) the `-tags forge_wasmtime` escape hatch, and (c) ADR-001's reversal trigger that watches for sustained blockers.
- Component model tooling is still maturing in some languages; mitigated by shipping reference templates only for TinyGo + Rust at M0.
- Per-call FFI cost is non-zero (~µs); acceptable for plugin-call frequencies (1–1000/s, not 1M/s).
- WIT version evolution requires a versioning policy → DEV-M2-01 plugin manifest.

### Follow-ups created

- DEV-M0-02 — vendor `wazero` + write `forge:plugin@1` WIT; publish initial `wasm-feature-matrix.md`.
- DEV-M0-02b — build-tag plumbing for `forge_wasmtime` escape hatch; CI compiles both variants.
- DEV-M0-08 — `hello-plugin` spike repo (acceptance criterion of ARCH-DEC-02).
- TEST-04 — plugin load-time benchmark.

## Compliance hooks

- Test: plugin load ≤ 50 ms (TEST-04).
- Test: panic-isolation — a plugin that traps must not abort the host (TEST-05).
- Lint: WIT-world version bumps require a CHANGELOG entry.
- Lint: any use of a component-model feature outside `wasm-feature-matrix.md` fails CI.
- CI gate: default release build is `CGO_ENABLED=0`; cgo creep blocks merge.
- CI gate: signed plugin signature verified before load (ties to ADR-004).

## References

- Arch §13 ADR-002, §15.
- WebAssembly component model: <https://component-model.bytecodealliance.org/>.
- `wazero`: <https://wazero.io/>.
- `wasmtime-go` (escape hatch): <https://github.com/bytecodealliance/wasmtime-go>.
