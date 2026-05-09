# Forge — Architecture Blueprint (DAB)

> Companion to `FORGE_FRAMEWORK_SPEC.md` v0.10.9. This document turns the spec's *what* into the *how*: the runtime topology, the module boundaries, the data shapes, the extension points, and the contracts that hold it all together.
>
> Status: **Draft / Pre-RFC**. Every architectural choice traces back to a principle in §11.2 of the spec or to an open question in §8.

---

## 0. Reading guide

```
§1  Architectural posture        — the five non-negotiables
§2  System context (C4-L1)       — Forge in the developer's world
§3  Container view (C4-L2)       — the 9 long-lived processes/artifacts
§4  Component view (C4-L3)       — modules inside each container
§5  Layered module map           — Foundation / Security / Testing / Observability / LLM-Native / Deployment
§6  The `forge ship` pipeline    — the defining runtime flow
§7  Data architecture            — on-disk layout, manifests, registries
§8  Extension architecture       — plugin loader, capability surface, sandboxing
§9  LLM provider abstraction     — token economy, routing, caching
§10 Scan-Fix-Learn loop          — engine, confidence model, learning aggregation
§11 Cross-cutting concerns       — config, secrets, telemetry, errors, i18n
§12 Deployment & runtime targets — local dev, CI, hosted control plane
§13 Tech stack decisions (ADRs)  — language, build, distribution
§14 NFRs & budgets               — perf, cost, footprint
§15 Threat model summary         — STRIDE highlights
§16 Open architectural questions — pulled from spec §8
§17 Failure modes & resilience   — unhappy paths per layer, recovery contracts
§18 Bug & issue lifecycle        — intake from core + community, triage, SLA, post-mortem
```

---

## 1. Architectural posture (the five non-negotiables)

These constrain every module decision below.

| # | Posture | Consequence |
|---|---------|-------------|
| 1 | **LLM-first, not LLM-bolted-on** | Every module exposes a `--explain` introspection surface and a structured `manifest.json`. The CLI grammar is the LLM's API. |
| 2 | **Convention as code, not as docs** | Conventions ship as executable instructions packs (`.forge/instructions/`) loaded by both the CLI and the LLM. The linter and the LLM read the same file. |
| 3 | **Local-first, hosted-optional** | Nothing in core requires a Forge-operated server. Hosted control plane (telemetry, learning aggregator, registry) is opt-in and feature-flagged. |
| 4 | **Single binary, plugin-extensible** | Core ships as one statically-linked binary. Everything else (scanners, adapters, recipes) loads via the plugin API. |
| 5 | **Reversible by default** | Every `--apply` has a `--dry-run`; every migration has a codemod; every default has an escape hatch. No silent magic. |

---

## 2. System context (C4 Level 1)

```
                          ┌──────────────────────────┐
                          │      Developer (IDE)     │
                          │   "vibe-coding" via LLM  │
                          └────────────┬─────────────┘
                                       │ forge <verb>
                                       ▼
   ┌──────────────────┐         ┌──────────────┐         ┌─────────────────┐
   │  LLM provider(s) │◀────────│  Forge CLI   │────────▶│   Project repo  │
   │  (OpenAI/Claude/ │  prompt │ (single bin) │  read/  │ (.forge/, src/, │
   │   local model)   │  + ctx  │              │  write  │   tests/)       │
   └──────────────────┘         └──────┬───────┘         └─────────────────┘
                                       │
                ┌──────────────────────┼─────────────────────┐
                ▼                      ▼                     ▼
       ┌─────────────┐         ┌──────────────┐      ┌────────────────┐
       │  Plugin     │         │  Forge       │      │  Hosted (opt.) │
       │  Registry   │         │  Workspace   │      │  - Aggregator  │
       │  (mirror-   │         │  cache       │      │  - Eval cloud  │
       │   able)     │         │  (~/.forge)  │      │  - Telemetry   │
       └─────────────┘         └──────────────┘      └────────────────┘
```

**Actors:** Developer, LLM provider(s), Plugin author, Maintainer (review SLAs §16.5.7), Operator (CI), End-user (of the developer's app).

**External systems:** Git host, CI runner, package registry (npm/PyPI/etc. for project deps), cloud target (deploy adapter), LLM provider HTTP API.

---

## 3. Container view (C4 Level 2)

The 9 long-lived containers/artifacts. *Container = independently deployable/installable unit.*

| # | Container | Form | Owner | Notes |
|---|-----------|------|-------|-------|
| C1 | **`forge` CLI** | Single static binary | Core | All verbs route through here. |
| C2 | **Forge workspace cache** | `~/.forge/` directory | CLI-managed | Per-user; holds plugin downloads, model caches, eval baselines. |
| C3 | **Project-local Forge state** | `.forge/` in repo | CLI-managed | Specs, instructions, waivers, locks, generated configs. Committed to git. |
| C4 | **Plugin Registry** | Static JSON index + tarballs | Community / hosted-mirrored | T3/T2 plugins; signed manifests. |
| C5 | **LLM provider adapter pool** | In-process plugins | Plugin authors | Implements `ILlmProvider`; sandboxed. |
| C6 | **Scan engine + scanner plugins** | In-process | Core engine, plugin scanners | 8 scanner families (auth, RLS, secrets, supply-chain, perf, accessibility, prompt-injection, cost). |
| C7 | **Learning loop aggregator** (optional) | Hosted service | Forge org | Receives anonymized convention deltas; feeds back nightly digests. |
| C8 | **Eval harness** | Local CLI mode + optional hosted runner | Core / Forge org | Runs reference scenarios; emits cost+quality deltas. |
| C9 | **Docs site + RFC repo** | Static site (`docs/`) + GitHub repo | Community | `forge docs` writes here; RFC PRs land here. |

> **Containers C7, C8 (hosted), C9 are opt-in.** The framework runs end-to-end with only C1, C2, C3 present.

---

## 4. Component view (C4 Level 3) — inside the CLI

```
┌─────────────────────────── forge CLI (C1) ─────────────────────────────┐
│                                                                        │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌────────────┐ │
│  │ Verb router  │─▶│ Command      │─▶│ Workflow     │─▶│ Output     │ │
│  │ (11 ns §4)   │  │ dispatcher   │  │ orchestrator │  │ formatter  │ │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘  │ (text/json/│ │
│         │                 │                 │          │  --explain)│ │
│         │                 ▼                 ▼          └────────────┘ │
│         │         ┌──────────────┐  ┌──────────────┐                  │
│         │         │ Plugin       │  │ ship pipeline│                  │
│         │         │ loader       │  │ (§6)         │                  │
│         │         └──────┬───────┘  └──────┬───────┘                  │
│         │                │                 │                          │
│         ▼                ▼                 ▼                          │
│  ┌───────────────────────────────────────────────────┐                │
│  │            Foundation services                    │                │
│  │  config · secrets · fs · git · proc · telemetry   │                │
│  │  error-codes (FORGE-XXXX) · i18n · logger         │                │
│  └───────────────────────────────────────────────────┘                │
│                                                                       │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌───────────┐ │
│  │ LLM gateway  │  │ Scan engine  │  │ Migration    │  │ Eval      │ │
│  │ + token      │  │ (8 families) │  │ runner       │  │ harness   │ │
│  │  ledger      │  │              │  │              │  │           │ │
│  └──────────────┘  └──────────────┘  └──────────────┘  └───────────┘ │
└────────────────────────────────────────────────────────────────────────┘
```

Each box is a Go module (per ADR-001) with a typed interface and a manifest exposed to `--explain`. Plugins (T3) are WASM components and may be authored in any language compiling to the component model (per ADR-002).

---

## 5. Layered module map (matches spec §4)

| Layer | Modules | Public API surface | Plugin extension point |
|-------|---------|--------------------|------------------------|
| **L1 Foundation** | config, secrets, fs, git, proc, errors, i18n, logger | `forge.foundation` SDK | none (stable core) |
| **L2 Security** | auth scaffold, RLS validator, secrets scanner, signer/verifier (DCO + sigstore) | `IAuthProvider`, `IRLSChecker` | adapter plugins |
| **L3 Testing** | spec parser, test scaffolder, regression guard, eval driver | `forge.testing` SDK | test-recipe plugins |
| **L4 Observability** | structured logger, OpenTelemetry exporter, audit log writer, cost ledger | OTel-compatible | exporter plugins |
| **L5 LLM-Native** | prompt registry, context builder, `ship` workflow engine, scan→fix loop, learning loop client | `ILlmProvider`, `IPromptPack`, `IInstructionPack` | provider/prompt/scanner plugins |
| **L6 Deployment** | build orchestrator, deploy adapters, rollback runner, backup/insights agents | `IDeployTarget` | cloud-adapter plugins |

---

## 6. The `forge ship` pipeline — the defining runtime flow

This is the architectural centerpiece. Spec §4 + §11.2 #12.

```
$ forge ship "add team-invite endpoint"

┌─[1] SPEC ───────────────────────────────────────────────────────────┐
│ • LLM drafts .forge/specs/add-team-invite/spec.md from prompt+ctx   │
│ • Spec includes: intent, acceptance criteria, principle served      │
│ • Human approves (or --quick auto-approves trivial)                 │
│ • Hash recorded → enforced as immutable input downstream            │
└─────────────────────────────────────────────────────────────────────┘
                              │ pass
                              ▼
┌─[2] TEST ───────────────────────────────────────────────────────────┐
│ • LLM writes failing tests from acceptance criteria                 │
│ • Test files committed BEFORE production code (timestamp gate)      │
│ • Test runner executes — expects RED                                │
└─────────────────────────────────────────────────────────────────────┘
                              │ red as expected
                              ▼
┌─[3] BREAKDOWN ──────────────────────────────────────────────────────┐
│ • LLM produces task list in .forge/specs/.../tasks.md               │
│ • Each task: scope, files-touched estimate, principle reference     │
└─────────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─[4] CODE ───────────────────────────────────────────────────────────┐
│ • Per-task: LLM proposes diff → diff applied → tests rerun          │
│ • Loop until tests GREEN or task escalated for human                │
│ • Convention linter runs each iteration; new patterns flagged       │
└─────────────────────────────────────────────────────────────────────┘
                              │ green
                              ▼
┌─[5] SHIP ───────────────────────────────────────────────────────────┐
│ • forge scan all --since main         (8 scanner families)          │
│ • forge lint                          (convention)                  │
│ • forge docs sync                     (no diff allowed)             │
│ • forge eval --regression             (cost/quality gates)          │
│ • Commit + PR with auto-filled body from spec                       │
└─────────────────────────────────────────────────────────────────────┘
```

**Architectural properties:**
- Each checkpoint is a **separate process invocation** with structured output → orchestrator can resume/replay any single stage.
- All artifacts (`spec.md`, `tasks.md`, test files, scan reports) are git-committed → auditable.
- The pipeline is **the** regression test for the framework itself (§16.5.2 dogfood rule).

---

## 7. Data architecture

### 7.1 Project-local layout (`.forge/` in repo)

```
.forge/
├── forge.yml                  # project config (single source of truth)
├── instructions/              # convention packs (read by linter AND LLM)
│   ├── defaults.md            # framework defaults
│   ├── project.md             # project overrides
│   └── anti-patterns.md       # things never to do
├── specs/
│   └── <change-slug>/
│       ├── spec.md            # immutable once approved
│       ├── tasks.md
│       └── ship.json          # checkpoint state (resumable)
├── waivers/
│   └── <scan-id>.yml          # rationale + expiry for any waived scan finding
├── prompts/                   # project-overridable prompt templates
├── locks/
│   ├── plugins.lock           # signed plugin pins
│   └── llm.lock               # model+version pins for reproducibility
└── audit/
    └── <yyyy-mm>.jsonl        # append-only ledger of forge actions
```

### 7.2 User-global cache (`~/.forge/`)

```
~/.forge/
├── plugins/<name>/<version>/  # downloaded plugin tarballs (sigstore-verified)
├── models/                    # local model weights (when applicable)
├── eval-baselines/            # per-project eval snapshots
└── token-ledger.jsonl         # append-only LLM cost ledger
```

### 7.3 Manifests (the LLM-readable contracts)

Every plugin, scanner, adapter ships a `manifest.json`:

```jsonc
{
  "name": "scanner-rls",
  "version": "1.4.0",
  "tier": "T3",
  "capabilities": ["scan.rls", "fix.rls.suggest"],
  "unsupported": [],
  "extension_points_used": ["forge.scan.register"],
  "permissions": ["fs.read:src/**", "net.none"],
  "explain": "Validates that every Postgres table reachable from public schema..."
}
```

`forge plugin <name> --explain` returns this verbatim. The LLM uses it for routing decisions.

---

## 8. Extension architecture

### 8.1 Capability model

Plugins declare **capabilities** (verbs they implement) and **permissions** (resources they need). The loader enforces both.

| Capability namespace | Examples | Hosted by |
|----------------------|----------|-----------|
| `scan.*` | `scan.auth`, `scan.rls`, `scan.secrets`, `scan.cost` | scanner plugins |
| `fix.*` | `fix.rls.suggest`, `fix.secrets.rotate` | scanner+fixer plugins |
| `gen.*` | `gen.endpoint`, `gen.migration`, `gen.adapter` | generator plugins |
| `adapt.llm.*` | `adapt.llm.openai`, `adapt.llm.anthropic`, `adapt.llm.local` | provider plugins |
| `adapt.deploy.*` | `adapt.deploy.aws`, `adapt.deploy.fly`, `adapt.deploy.railway` | deploy plugins |
| `learn.*` | `learn.share`, `learn.consume` | learning-loop client |
| `instruct.*` | `instruct.pack.<topic>` | convention packs |

### 8.2 Sandboxing

- Default: **deny all I/O** except declared permissions.
- Network egress: explicit per-host allow-list.
- Filesystem: glob-scoped to declared paths.
- Process spawn: blocked by default; opt-in capability `proc.spawn` with command allow-list.
- Telemetry: any plugin attempting outbound network without `net.allow:<host>` triggers `FORGE-1207` and is blocked.

### 8.3 Loader pipeline

```
discover (forge.yml + ~/.forge/plugins/) →
verify signature (sigstore) →
parse manifest →
permission-check against project policy →
load into in-process registry →
expose via verb router
```

---

## 9. LLM provider abstraction

### 9.1 Interface

```ts
interface ILlmProvider {
  name: string;
  models: ModelDescriptor[];
  complete(req: CompletionRequest): AsyncIterable<CompletionChunk>;
  embed?(req: EmbedRequest): Promise<number[]>;
  costPerToken(model: string): { input: number; output: number };
  capabilities: { streaming: boolean; tools: boolean; jsonMode: boolean };
}
```

### 9.2 Routing & token economy

```
Caller (workflow) → Gateway → Router → Provider plugin → External API
                       │         │
                       │         ├─ tier-aware (cheap-first, escalate on fail)
                       │         ├─ cache (semantic hash of {prompt, ctx})
                       │         └─ budget guard (per-command + per-day)
                       │
                       └─ Token ledger (append-only ~/.forge/token-ledger.jsonl)
```

**Tier policy (default):**
1. Try cached completion (semantic hash hit).
2. Try cheap/local model.
3. On low-confidence or test-still-red, escalate to premium.
4. Hard budget cap → `FORGE-2401` ("budget exceeded; rerun with `--budget=...`").

### 9.3 Caching keys

`sha256(canonicalize(prompt) || canonicalize(context_files) || model_id)` — context invalidation is automatic on file change.

---

## 10. Scan-Fix-Learn loop

### 10.1 Engine

```
scan → findings (with confidence ∈ [0,1] and rule_id) →
classify → {auto-fixable, suggest-only, advisory} →
(optional) propose patch via LLM with rule context →
apply (--apply) or print (--dry-run) →
record outcome → feed learning loop
```

### 10.2 Confidence model

| Confidence | UX |
|------------|----|
| ≥ 0.9 | Auto-block in CI; auto-fix on `forge fix --apply` |
| 0.6–0.9 | Block with override flag (`--accept-suggestion`); fix shown as diff |
| < 0.6 | Advisory only; never blocks CI |

### 10.3 Learning loop (opt-in)

- Aggregator (C7) receives **only** anonymized rule-frequency counts and convention-pack diffs — never source code.
- Nightly digest published as a downloadable instructions-pack delta.
- Project opt-in via `forge.yml: learning.share: true`.

---

## 11. Cross-cutting concerns

| Concern | Approach |
|---------|----------|
| **Config** | Layered: defaults → `forge.yml` → env vars → CLI flags. `forge config explain <key>` shows the resolved value + source. |
| **Secrets** | Never read into LLM context. Loader rewrites secret refs to `${SECRET:NAME}` placeholders before any prompt build. |
| **Telemetry** | Off by default. `forge telemetry on` flips a single flag; `--explain` prints exact payloads. |
| **Errors** | All errors carry `FORGE-XXXX` codes (4-digit, namespaced 1xxx foundation / 2xxx workflow / 3xxx scan / 4xxx LLM / 5xxx deploy). Codes never reused. |
| **i18n** | English-only at M0. Strings centralized in `internal/i18n/` for future translation. |
| **Logging** | Structured JSON; human formatter on TTY. Never logs prompts unless `--explain`. |

---

## 12. Deployment & runtime targets

| Target | Form | Notes |
|--------|------|-------|
| Developer laptop | Brew/scoop/winget single binary | Primary target |
| CI runner | Same binary, `--ci` mode | Stricter defaults: `--no-color`, `--json`, `--budget=` enforced |
| Hosted control plane (opt) | Containerized Aggregator + Eval runner + Registry mirror | Helm chart + Terraform module shipped in `deploy/` |
| Air-gapped enterprise | Mirror Registry + local LLM provider | All hosted-deps replaceable with on-prem equivalents |

---

## 13. Tech stack ADRs (status: draft, see §16)

| ADR | Decision | Driver |
|-----|----------|--------|
| ADR-001 | **Implementation language: Go** (pure-Go default; no cgo) | Largest contributor pool + best LLM/vibe-coding fluency; gold-standard cross-compile across all 6 §14 triples |
| ADR-002 | **Plugin runtime: WASM component model on `wazero`** (pure-Go; `wasmtime-go` reserved as build-tag escape hatch) | Sandboxing; language-agnostic plugins; preserves zero-cgo build |
| ADR-003 | **Distribution: GitHub Releases + brew/scoop/winget tap** | Zero-infra to ship; auto-update via `forge upgrade` |
| ADR-004 | **Registry storage: signed JSON index in a Git repo + CDN** | Mirror-able, no central database required |
| ADR-005 | **Eval harness: pluggable scenario format (`scenario.yml`) executed in subprocess** | Deterministic, reusable across providers |
| ADR-006 | **Telemetry transport: OTLP over HTTPS, opt-in** | Standard; works with existing observability stacks |
| ADR-007 | **Spec format: Markdown with YAML frontmatter** | Human + LLM readable; diff-friendly |

---

## 14. NFRs & budgets

| NFR | Budget | Measured by |
|-----|--------|-------------|
| `forge new` time-to-running-app | ≤ 60s on cold cache | `forge eval --scenario new` |
| `forge ship` round-trip on reference app | ≤ 5 min | `forge eval --scenario ship-reference` |
| Cold-start of CLI | ≤ 80ms | benchmark suite |
| Memory footprint at idle | ≤ 60 MB | RSS at end of `forge --help` |
| Token cost of `forge ship` reference scenario | ≤ $0.20 default-tier | token ledger |
| Scan engine throughput | ≥ 100 files/sec (RLS family) | benchmark suite |
| Plugin load time per plugin | ≤ 50ms | startup trace |

Performance regressions >5% per spec §16.5.6 → block PR.

---

## 15. Threat model summary (STRIDE)

| Threat | Mitigation |
|--------|------------|
| **S**poofing — fake plugin published to mirror | Sigstore signing required; signature verified at load |
| **T**ampering — modified `forge ship` checkpoint state | `ship.json` is hash-chained; replay detects mismatch |
| **R**epudiation — "I didn't run that destructive command" | Audit ledger is append-only and signed |
| **I**nformation disclosure — secrets in LLM prompt | Secret-redactor in context builder; CI test enforces zero-leak invariant |
| **D**oS — runaway LLM cost | Budget guard + cache + tiered routing |
| **E**levation of privilege — plugin escapes sandbox | WASM component model; permission allow-list; capability auditing |

Full threat model → separate `THREAT_MODEL.md` (TBD M1).

---

## 16. Open architectural questions (from spec §8)

| Q | Question | Owner | Decision needed by |
|---|----------|-------|--------------------|
| Q1 | Implementation language (Rust vs Go) | Founder + 1 reviewer | M0 W2 — **resolved by ADR-001 (Go)** |
| Q2 | WASM component model maturity for plugins | Plugin WG | M0 W4 |
| Q5 | Registry: own infra vs piggyback on npm/crates.io | Community WG | M1 |
| Q9 | Hosted aggregator: SaaS or self-hostable from day 1? | Founder | M1 |
| Q14 | Eval harness deterministic seed strategy | Quality WG | M1 |
| Q19 | Air-gapped enterprise: in-scope for OSS or commercial-only? | Founder | M2 |
| Q22 | License: Apache-2.0 vs MIT vs BSL-then-Apache | Legal advisor | Before M0 first push |

---

## 17. Failure modes, unhappy paths & resilience

Resilience here means: **every component has a named failure mode, a detection signal, a containment boundary, and a recovery path — and they are all tested.** No component ships a happy path without its unhappy twin. This section is the architectural register; the tests live under `TEST-NN` / `DEV-MN-NN` per the task files.

### 17.1 Resilience contract (applies to every long-lived process and every plugin)

1. **Fail closed, not open.** When a check cannot complete, the default is *deny + report*, never *allow + warn*. Scan engine timeout = block PR; sandbox unsure = deny capability; LLM provider unreachable = abort current checkpoint, never silently fall through.
2. **Bound every wait.** No unbounded `await` — every external call (LLM, network, FS, subprocess, plugin) has an explicit timeout, retry policy, and budget cap. Defaults are codified in `forge.config.ts`; overridable per command via `--timeout` / `--budget`.
3. **Make state recoverable.** Every multi-step verb writes a hash-chained checkpoint (`.forge/state/<verb>.json`) before each side-effect. Crash + rerun = resume, never restart-from-scratch silently.
4. **Surface, don't swallow.** Every caught error becomes a `FORGE-XXXX` code with: what failed, what was attempted, what to do next, and a `--explain` link. No `catch { /* ignore */ }` anywhere; lint enforces this.
5. **Be reversible.** Every `--apply` writes its inverse to `.forge/trash/<run-id>/` (FS) or to a reversed migration file (DB) before applying. `forge undo <run-id>` is always available for the most recent N runs.
6. **Degrade explicitly.** When a non-essential subsystem (telemetry, learning loop, hosted aggregator) fails, the user sees a one-line warning, the operation continues, and the failure is logged. Essential subsystems (scan engine, sandbox, audit ledger) are fail-closed.
7. **Never leak secrets in error paths.** Every error path runs through the same redactor as the happy path (TEST-25 invariant).

### 17.2 Per-layer failure register

| Layer / Component | Unhappy path | Detection | Containment | Recovery | Test anchor |
|---|---|---|---|---|---|
| **CLI router** | unknown verb / typo | router lookup miss | print suggestions, exit 64 | user retries; suggestions ranked by edit distance | DEV-M0-10 |
| **Config loader** | malformed file / type clash between layers | parser + validator | `FORGE-1101`, no partial config applied | user fixes file; `forge config explain` pinpoints layer | DEV-M0-02; TEST-15 |
| **Filesystem service** | path escapes grant; symlink cycle; perm denied; ENOSPC mid-write | sandbox check + fs syscall | `FORGE-1501..1599`; partial-write detected via temp+rename | rollback via `.forge/trash/`; `forge undo` | DEV-M0-05 |
| **Process service** | spawn outside allow-list; child OOM-killed; orphan after parent crash | allow-list + waitpid + reaper | deny / SIGKILL group | reaper sweeps on next start | DEV-M0-07 |
| **Audit ledger** | tampered prior entry; disk full; concurrent writer | hash-chain verify; advisory lock | `FORGE-1801`; refuse new appends until verified | recover from last valid entry; quarantine bad tail | DEV-M0-08 |
| **Secrets redactor** | regex misses a novel pattern; entropy fallback false-positive on user data | TEST-12 100-run loop + entropy meter | redacted-by-default placeholder | user adds `--no-redact` allow-list with rationale | DEV-M0-09; TEST-12; TEST-25 |
| **`forge ship` orchestrator** | crash mid-checkpoint; checkpoint corrupted; `git` state changed under us | hash-chain on `ship.json`; pre-checkpoint git SHA snapshot | `FORGE-2101..2199`; refuse resume on SHA mismatch | `forge ship --restart` (audited); manual `forge ship abort` | DEV-M1-01 |
| **LLM gateway** | provider 5xx; quota exhausted; context-window overflow; partial stream then disconnect | HTTP status + token count + stream-end sentinel | retry with backoff; tier-route; cap by budget | escalate next tier; abort checkpoint with `FORGE-2401`/`2402`/`2403` | DEV-M0-17; DEV-M1-08 |
| **LLM cache** | poisoned entry; key collision; cache disk corrupted | per-entry signature; checksum on read | reject entry → cache miss → live call | rebuild on next run; `forge cache purge` | DEV-M1-07 |
| **Scan engine** | scanner panics; rule-pack version mismatch; scan timeout | per-scanner subprocess; manifest version pin; deadline | isolate failed scanner, continue others, mark family `unknown` (NOT `clean`) — gate fails closed | rerun the one scanner; pin earlier rule-pack via lockfile | DEV-M1-11 |
| **`forge fix --apply`** | LLM proposes destructive diff; partial apply; concurrent edit by user | confidence floor + dry-run preview + git-clean precondition | refuse on protected branch; refuse if working tree dirty | `forge undo`; revert via `.forge/trash/` | DEV-M1-16 |
| **Plugin loader** | unsigned/tampered; capability over-claim; sandbox escape attempt; runtime panic | sigstore verify; manifest validator; WASM trap; fuzz suite | refuse load; on runtime trap, isolate plugin, host continues | `forge plugin remove`; auto-disable after N panics | DEV-M1-18; TEST-14 |
| **Migration runner** | partial apply; reverse fails; double-apply attempted; clock skew on lock | transaction boundary + advisory lock + applied-set check | rollback in same tx; refuse on already-applied; refuse on lock contention | `forge migrate status` + audited `forge migrate repair` | DEV-M2-22 |
| **Deploy adapter** | provider 5xx mid-deploy; healthcheck times out; rollback also fails | adapter healthcheck + canary | freeze deploy; alert; preserve previous revision active | `forge deploy rollback --to <rev>`; emergency runbook | DEV-M2-05; DEV-M3-17 |
| **Storage adapter** | network partition; eventual-consistency read-after-write miss | per-op timeout + RAW probe | retry-with-backoff; surface stale-read warning | provider-defined; documented per adapter | DEV-M2-07 |
| **Telemetry pipeline** | endpoint unreachable; payload schema drift | local outbound queue + schema validator | drop with warn; never retried indefinitely | next run resumes if queue not full | DEV-M0-30 |
| **Learning loop client** | accidental source-byte capture; PII in payload | DEV-M2-09 invariant test + redactor | reject payload; emit `FORGE-3101` | user can `forge learn purge`; opt-out is one flag | DEV-M2-09 |
| **Hygiene engine (`forge clean`)** | manifest drift; tracked secret file detected; user races a delete | manifest diff + `git ls-files` cross-check + dry-run-first | exit non-zero with file list; never delete without `--apply` | `forge undo` from `.forge/trash/`; `forge upgrade gitignore` | DEV-M0-15; TEST-23 |
| **Gitleaks gate** | rule false-positive; allowlist expired; bypass token misused | per-rule fixture suite + frozen-clock test + commit-token surfacing | block PR; expired entry → fail; bypass token visible in PR template | author files RFC for rule tuning; nightly sweeper prunes expired | TEST-21; TEST-24; DEV-M1-36 |
| **Eval harness** | flaky scenario; provider non-determinism; cassette stale | per-scenario seed pin + cassette signature + 3-run quorum | mark scenario quarantine, do not gate on it; auto-open issue | refresh cassette nightly; revisit seed | TEST-04; TEST-19 |
| **Plugin Registry / mirror** | mirror compromised; CDN outage; signature trust-root rotation | sig verify + mirror redundancy + trust-root pinned | refuse install on sig fail; fall back to next mirror | air-gapped install path | DEV-M2-01; DEV-M3-13 |

### 17.3 Cross-cutting failure scenarios (multi-component)

These are scripted in the eval harness and rehearsed on a regular cadence (see `OPS_TASKS.md`).

1. **Provider outage mid-`ship`.** Primary LLM down at the Code checkpoint → tier-router escalates → if all tiers down, ship aborts with `FORGE-2402`, checkpoint preserved, `--resume` works once provider returns.
2. **Disk full mid-apply.** `forge fix --apply` runs out of disk during write → temp-file write fails → no partial commit → `FORGE-1503` with disk-usage hint.
3. **Concurrent ship on the same branch.** Two devs run `forge ship` on the same checkout → second acquires advisory lock failure → `FORGE-2104` with PID of the holder.
4. **Plugin panic during scan.** One scanner plugin panics → host catches WASM trap → that scanner family marked `unknown` → gate fails closed (per §17.1 #1).
5. **Audit ledger tampering detected.** On startup, hash-chain verify fails → CLI refuses any state-mutating verb → only `forge audit verify` and `forge audit recover --to <entry>` are permitted.
6. **Cosmic-ray cassette drift.** Nightly cassette refresh produces a diff that wasn't requested → eval scenario quarantines itself + opens an issue (per DEV-M3-07 bot) instead of silently flipping the gate.
7. **Secret leak attempt via debug flag.** `forge ship --explain --debug` against code containing a real secret → redactor still redacts (TEST-25-02) → `--explain` shows redaction marker, never raw value.
8. **Migration applied on prod, reverse missing in repo.** `forge doctor --prod` flags drift → refuses further deploys until reverse committed or `forge migrate repair` invoked with rationale.

### 17.4 Resilience invariants enforced in CI

| Invariant | Enforced by |
|---|---|
| Every external call has a timeout | Lint rule (forbids `await` on un-wrapped calls in capability namespaces) |
| Every `catch` either rethrows or maps to `FORGE-XXXX` | Lint rule + AST check |
| Every `--apply` has a `--dry-run` and an inverse | Per-verb manifest check |
| Every long-running verb writes a checkpoint before each side-effect | Per-verb manifest check |
| Every error code is documented and tested | DEV-M0-03 + TEST-01 false-positive guard |
| Every gate fails closed on engine error | Per-gate contract test |
| No subsystem swallows a secret on the error path | TEST-25-02 |

### 17.5 What this section does NOT cover

Threat-actor-driven failures (malicious plugin author, hostile mirror, supply-chain attack) live in §15 + `THREAT_MODEL.md` (DEV-M3-01). §17 is about *operational and developmental* failures — bugs, outages, mis-configuration, race conditions — not adversarial intent. The two registers are kept separate so neither can hide behind the other.

---

## 18. Bug & issue lifecycle (core team + community)

Forge is community-first (spec §0). Bugs and issues will arrive from many channels; the architecture has to make triage **fast, fair, and learnable**. This section defines the channels, the severity model, the SLAs, and the post-mortem contract. It is the *operational counterpart* to §17 (which defines what we want to be resilient *to*).

### 18.1 Intake channels

| Channel | Who uses it | Where it lands | Notes |
|---|---|---|---|
| `forge report` (CLI verb) | any user | opens a pre-filled GitHub issue with `forge doctor --bundle` attached | redacts secrets via the standard redactor before bundling |
| GitHub Issues — `bug` template | community + core | `repo/issues` | required: repro repo or `--bundle`, expected vs actual, `forge --version` |
| GitHub Issues — `vulnerability` template | community + core | private security advisory (per GitHub Security) | never accepted on the public tracker |
| `huntr.dev` / bug bounty | external researchers | private intake; routed by Security WG | DEV-M3-03 |
| Discussions / Discord | community | not a bug tracker — moderators triage promising threads into issues | nothing is "fixed" unless it has an issue |
| Internal core-team incidents | maintainers | `repo/issues` with `incident` label + linked post-mortem | even internal-only incidents are public unless they reveal a vuln |
| Eval-harness self-reported flakes | the system itself | DEV-M3-07 bot opens `flake` issues automatically | quarantined, not silenced |

**Rule:** every fix that lands in the codebase has a public issue link in its commit trailer (`Fixes: #NNN`). No "stealth fix." Enforced by the contribution-standards bot (DEV-M3-07).

### 18.2 Severity model

| Severity | Definition | First-response SLA | Time-to-fix target |
|---|---|---|---|
| **S0 — Catastrophic** | data loss, secret leak, sandbox escape proven, supply-chain compromise, prod-deploy adapter corrupting state | < 4 h (24/7 rota at GA; best-effort pre-GA) | hotfix patch within 24 h; full RCA within 7 d |
| **S1 — Critical** | core verb (`ship`/`scan`/`new`/`migrate`/`deploy`) broken on a supported platform; CI gate falsely passing; signing pipeline broken | < 24 h | patch within 7 d |
| **S2 — Major** | non-core verb broken; scanner family produces high-rate false positives; doc seriously misleading | < 7 d | next minor (≤ 6 wk) |
| **S3 — Minor** | cosmetic; small ergonomic gap; rare edge case with workaround | < 14 d | next minor or marked `help-wanted` |
| **S4 — Tracking** | feature request, RFC seed, "would be nice" | acknowledged, may sit in backlog | not promised |

Severity is assigned by the on-call triager (rotates weekly among Reviewers + Maintainers per spec §16.5.8). Severity can be raised by any maintainer with a one-line rationale; lowering requires two.

### 18.3 Triage flow

```
[issue arrives]
      │
      ├─→ [auto-triage bot]  applies labels: severity-guess, area-guess, version-affected
      │
      ▼
[on-call triager]  within first-response SLA:
      ├─ confirm severity                (or correct it)
      ├─ confirm reproducibility         (request repro if missing → `needs-repro` label, 14d auto-close)
      ├─ assign component owner          (CODEOWNERS for the touched area)
      ├─ link to existing failure-mode register entry (§17.2) if applicable, or open one
      ▼
[component owner]
      ├─ if S0/S1: drop other work, apply hotfix process (§18.5)
      ├─ if S2/S3: queue for next milestone, write failing test first
      ├─ if not a bug: convert to RFC (S4) or close with rationale + alternative
      ▼
[fix lands]  must include:
      ├─ failing-test-then-passing commit pair (TEST-17)
      ├─ entry in §17.2 if a new failure mode was discovered
      ├─ changelog entry citing the issue id
      ├─ post-mortem if S0/S1 (§18.6)
```

### 18.4 Two-key rule for irreversible operations

During an incident, *no single maintainer* may:

- force-push to `main` or any release branch
- publish a release artifact (sigstore signing requires two key custodians per DEV-M0-28)
- rotate the trust root for the Plugin Registry
- merge a PR that disables a CI gate (even temporarily)
- ship a hotfix that bypasses §16.5.4 gates without a `gate-bypass` issue + justification

All five require a second maintainer's written approval *in the issue thread* (audit trail). The bot (DEV-M3-07) enforces this on PRs; the registry + signing ones are enforced by the underlying systems.

### 18.5 Hotfix process (S0 / S1 only)

1. **Cut a hotfix branch** from the latest release tag (`hotfix/<version>`). Never from `main` directly.
2. **Failing test first** — even under incident pressure. The test is the proof the fix works *and* the regression guard for next time.
3. **Two-key release** — sigstore signing requires the two-key rule (§18.4).
4. **Forward-port to `main`** in a separate PR, same SHA range, before declaring the incident resolved.
5. **Auto-update the failure register** — the post-mortem PR (§18.6) edits §17.2 to include the newly-discovered failure path.
6. **Status page update** at each transition: identified → mitigated → fixed → post-mortem published.

### 18.6 Post-mortem contract (mandatory for S0 / S1, optional for S2)

A post-mortem is **a learning artifact, not a blame artifact**. The framework's design itself should be the primary subject — *"what about the way we build this made this bug possible?"*

Template (committed at `docs/postmortems/YYYYMMDD-<slug>.md`):

```
1. Summary             (≤200 words; user-visible impact + duration)
2. Timeline            (UTC; first symptom → mitigation → fix → post-mortem)
3. Root cause          (the architectural / process gap, not just "the bug")
4. Why our gates missed it (which §16.5.4 gate or §17 invariant should have caught it?)
5. Detection delay     (could we have known earlier? what would have shown us?)
6. Action items        (each with an owner + tracking issue id; at least one must be a §17 register update or a new test)
7. What worked well    (kept honest; not optional)
8. Customer comms      (link to status-page entry + any direct outreach)
```

**Action-item rule:** every post-mortem ships at least one *durable* action — a new test, a new lint rule, a new CI gate, or an entry in §17.2. "Be more careful next time" is not an action item.

### 18.7 Community-reporter feedback loop

The person who reported the bug must be:

1. **Acknowledged** within the first-response SLA, *by a human, not a bot*.
2. **Updated** at each state transition (triaged → assigned → fix-in-progress → fixed → released).
3. **Credited** in the changelog and in the post-mortem (with their consent, per DCO sign-off attribution).
4. **Asked** whether the fix matches their expectation before the issue is closed (one-line confirmation suffices).
5. **Invited** to upgrade their contributor tier per spec §16.5.8 if they showed sustained signal (good repros, follow-up patches).

A reporter who never hears back twice in a row is treated as a process-defect, not a low-signal contributor.

### 18.8 Visibility & metrics (on the Quality Dashboard, TEST-19)

| Metric | Source | Alert threshold |
|---|---|---|
| Open S0/S1 count | issue tracker | > 0 for > SLA |
| Median time-to-first-response (per severity) | issue events | > 1.5× SLA |
| Median time-to-fix (per severity) | issue + PR events | > target |
| Issues without a linked test on close | bot scan | > 0 |
| Post-mortems published / S0+S1 closed | docs/postmortems vs labels | < 100% |
| Community vs core reporter ratio | issue author groups | tracked, not gated |
| Reopen rate (≤ 30 d) | issue events | > 5% |
| Eval-harness self-flakes auto-quarantined | bot | tracked |

### 18.9 What this section does NOT cover

- **Adversarial vuln intake** — handled per §15 + the security policy in `SECURITY.md`. S0 vulns enter via private advisory, not the public tracker.
- **Long-term roadmap shaping** — that's the RFC process (spec §16.2), not the bug tracker.
- **Contributor conflict resolution** — Code of Conduct + maintainer escalation, not §18.

---

*Architecture doc version: 0.2 — companion to spec v0.10.9.*
