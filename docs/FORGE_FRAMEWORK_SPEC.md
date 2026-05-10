# Forge Framework — Product Spec & Go-to-Market Plan

> *"From vibe to production — without the 3am incident"*
>
> *Sub-headline:* The LLM-native framework that makes AI-generated code survive contact with real users — security, multi-tenancy, audit, and observability built in, not bolted on.
>
> **Ultimate goal:** *enable developers to **ship fast with high quality via vibe-coding** — the speed of "vibe it and ship it" with the quality of a framework that has scanners, tests, multi-tenancy, audit, and learning loops built in.* Every feature, every default, every roadmap item is judged against this single sentence: *doesrit raise the ceiling of what a vibe-coded app can ship to production with confidence, without slowing the developer down?* If the answer is no, it doesn't ship.

**Ideal Customer Profile (ICP) — the person Forge is for:**
- A solo founder, indie hacker, or small-team lead
- Already uses Cursor / Copilot / Claude / Windsurf to write the majority of their code
- Has shipped at least one prototype that *almost* made it to production but stalled at security/multi-tenancy/migrations
- Values shipping speed AND craftsmanship — refuses to choose
- Will pay $0 today; might pay something in 3 years if Forge becomes critical infrastructure

If you are this person: Forge is built for you. If you are an enterprise architect at a Fortune 500 — Forge will reach you in a few years; please wait.

---

## 0. Strategic Posture: Community-First, Commercial-Later

**This is the founding commitment of the Forge project. All other decisions in this document are subordinate to it.**

Forge is an open-source-first project. The primary objective for the foreseeable future (minimum 24 months, likely longer) is to **build a thriving community of developers and early adopters**. Commercialization is explicitly deferred and is NOT a near-term goal.

### What This Means in Practice

- **No paywalled features in the first 24 months.** Every feature shipped is fully open source under Apache 2.0.
- **No "open core" tricks.** We will not artificially gate functionality to create future commercial leverage.
- **No VC funding pressure.** The project is bootstrapped by the founder. No investor is allowed to dictate roadmap toward monetization.
- **No commercial entity at launch.** Forge starts as a pure OSS project. A commercial entity is only formed if and when the community is large enough to sustain it without compromise.
- **Every roadmap decision is filtered through one question:** *"Does this make the community stronger?"* If the answer is no, it is deprioritized — even if it would generate revenue.
- **Sustainability comes from sponsorships and grants**, not product sales, for the entire community-building phase.

### Why This Matters

Most "open source" frameworks that pursued commercialization too early lost their community (e.g., HashiCorp's BSL relicensing, Elastic's SSPL move). Forge avoids this by treating community trust as the highest-value asset and refusing to compromise it for short-term revenue.

The Spring framework took ~5 years before SpringSource was even founded. Django was a community project for 7 years before Django Software Foundation became substantive. Forge follows that pattern deliberately.

### Commercial Phase: Long-Term Horizon Only

Commercialization is documented in §15 strictly as a **future option**, not a near-term plan. It activates only when **all** of the following are true:
- 10,000+ GitHub stars
- 500+ verified production apps
- 100+ external contributors
- A sustainable maintainer team funded by sponsorships
- Clear, unsolicited pull from enterprises asking for paid offerings

If any of these conditions is not met, the project remains in pure community mode indefinitely. **There is no deadline pressure to monetize.**

---

## 1. Executive Summary

**The Problem (hypothesis to be validated, not a marketing claim):** Vibe-coding has dramatically accelerated prototype and MVP development, but a large fraction of vibe-coded projects stall before reaching production due to missing enterprise fundamentals — security, multi-tenancy, observability, proper migrations, and audit logging. *Validation plan:* a public survey of 500+ vibe-coding developers in M0 + 30 longitudinal case studies through M1, with results published openly. We will revise the framing once we have data.

**The Opportunity:** No framework currently occupies the intersection of **LLM-native design × Enterprise-grade foundations × Open source**. Spring Boot conquered Java. Django conquered Python. The equivalent for the AI/vibe-coding era does not yet exist.

**The Solution:** Forge — an opinionated, LLM-native framework that provides enterprise-grade foundations out of the box, designed specifically for developers who use AI assistants to build software. Its output is production-ready, industry-standard software that can withstand the scrutiny of SaaS, banking, finance, and other quality-demanding domains.

**The ultimate goal in one line: *ship fast with high quality, via vibe-coding.*** Speed and quality have historically been a trade-off; Forge collapses that trade-off by making the *quality* parts (auth, multi-tenancy, audit, RLS, tests, scanners, observability) effectively free — generated, scanned, and learned, not hand-written. The vibe-coder keeps the speed of "prompt it and ship it"; the framework supplies the quality floor that prevents the 3am incident. If a feature speeds the developer up but lowers the quality floor, it does not ship in Forge. If a feature raises the quality floor but slows the developer down, it does not ship in Forge. Both conditions, every time.

**Three loops, one framework — `generate → scan → learn`:**
1. **Generate** — `forge new` and `forge generate` scaffold modules with auth, multi-tenancy, audit, tests, and LLM context built in. The inner cycle of `generate` is **`forge ship <feature>`** — a single orchestrating command that walks the developer through Spec → Test → Breakdown → Code → Ship in one interactive flow (see §4 for details). One verb, four checkpoints, no shortcuts.
2. **Scan & fix** — `forge scan` continuously detects security/performance/correctness/cost/accessibility/compliance issues in code your AI just wrote, and proposes diff-first fixes via PR (never auto-merged; see §4 Scan-and-fix layer).
3. **Learn** — every accepted/rejected suggestion, every revert, every closed bug feeds the project's own convention library and `.forge/instructions/` so the next AI prompt is sharper than the last (see §4 Continuous learning loop). The framework is *a student of the codebase, not a lecturer.*

**The defined way of working — *one command, four checkpoints*: `forge ship <feature>`** (TDD-shaped vibe-coding). Forge does not just provide tools — it prescribes the *order* in which a vibe-coder uses them, and ships that order as a single orchestrating command. `forge ship <feature>` walks the developer through four checkpoints in one interactive flow, pausing for review between each: **(1) Spec** — a short YAML/Markdown intent file capturing the user-visible behavior, the data shape, the authz model, and the acceptance criteria; **(2) Test** — the framework turns the spec into failing executable tests (unit + integration + RLS + scan baseline) *before* a single line of feature code is written; **(3) Breakdown** — the spec is decomposed into a checklist of small, reviewable, AI-friendly tasks (each with its own tightly-scoped LLM context bundle); **(4) Code & Ship** — the developer (with their AI tool of choice) implements task-by-task; the final ship checkpoint runs the full test suite + `forge scan` + a timestamp/git check that no test was written *after* its production code. One PR, one audit trail. *This is how Forge enforces "high quality" without slowing down "ship fast": the spec is the contract, the tests are the proof, the tasks are the speed, and a single command makes skipping any of them impossible by accident.*

### Anti-Goals (What Forge Is NOT)

Defining boundaries up front prevents scope creep and clarifies positioning:

- **Forge is NOT a no-code / low-code platform.** It generates real code that developers own and modify. No proprietary runtime, no vendor lock-in to a hosted execution environment.
- **Forge is NOT an AI agent or autonomous coder.** It does not replace the developer's intent — it amplifies it. Cursor/Copilot/Devin remain the agents; Forge is the framework they generate code into.
- **Forge does NOT manage LLM credentials or connections.** The developer's IDE or dev-tool (VS Code Copilot, Claude Code, Cursor, Windsurf) already has an LLM configured. Forge reads that existing configuration for any framework-orchestrated calls it needs to make (`forge ship` checkpoints: spec/test/breakdown generation; scan-fix proposals). Forge never stores API keys, never opens its own provider account, and never requires separate credential setup. If no IDE LLM is detected, `forge doctor` reports it and `forge ship` prompts the developer to configure their tool of choice before proceeding. In CI/CD pipelines (GitHub Actions, GitLab, etc.) where no IDE is present, the CI secrets vault supplies the same env vars the IDE would have used — Forge reads those the same way.
- **Forge is NOT a UI component library.** It is unopinionated about UI choices (works with shadcn, MUI, Tailwind, custom). Component libraries are a separate concern.
- **Forge is NOT a backend-as-a-service.** It uses BaaS (Supabase, etc.) via adapters but does not host or operate infrastructure for you.
- **Forge is NOT a CMS.** It is a framework for building applications, not for managing content.
- **Forge is NOT trying to be language-agnostic at v1.0.** TypeScript-first, deliberately. Other languages come via separate, focused adapters — not a lowest-common-denominator abstraction.
- **Forge is NOT a competitor to Next.js, NestJS, or Hono.** It is a layer above them — opinions, conventions, and modules — not a replacement runtime.
- **Forge does NOT promise zero-knowledge enterprise compliance.** It provides templates and enforced patterns that make compliance achievable, but certification (SOC 2, PCI-DSS, HIPAA) remains the responsibility of the operator.

---

## 2. Feasibility Analysis

### The Production Gap

| Stage        | Vibe-coding Today         | With Forge                        |
|--------------|---------------------------|-----------------------------------|
| Prototype    | ✅ Excellent               | ✅ Excellent                       |
| MVP          | ⚠️ Inconsistent quality   | ✅ Solid                           |
| Production   | ❌ Missing security/observability/migrations | ✅ Generated scaffolds + `forge scan` catches missing patterns; reversible migrations |
| Enterprise   | ❌ No RLS, multi-tenancy, audit log | ✅ RLS + audit + multi-tenancy default; compliance scanners enforce templates |
| Maintenance  | ❌ Same AI-introduced bugs recur PR after PR | ✅ Continuous learning loop turns each fixed bug into a regression test + sharpened LLM context |

### Why Now

- Karpathy coined "vibe coding" in early 2025; developer adoption is explosive
- LLM-generated code quality has crossed a threshold where production use is realistic
- Spring Boot took ~5 years to dominate its ecosystem — the window is open now
- No incumbent is defending this exact positioning (LLM-native + enterprise)

### Key Risks

| Risk | Probability | Impact | Mitigation |
|------|-------------|--------|------------|
| Cursor/Copilot builds their own framework | High | High | Build symbiosis — Forge as first-class citizen in LLM workflows |
| Framework fatigue | Medium | High | Zero-boilerplate DX; `forge new` → running in 5 minutes |
| Hard to maintain multi-language support | High | Medium | TypeScript-first; expand via community plugins |
| Community fails to grow | Medium | Very High | RFC-driven design, early adopter program, strong content strategy |
| Premature commercialization erodes community trust | Medium | Very High | §0 explicit commitment; commercial gates; refuse VC |
| Founder burnout before community self-sustains | High | High | Sponsorship funding from month 6+; recruit co-maintainers early; honest pace |
| Sponsorship funding insufficient | Medium | Medium | Diversified sources (GitHub Sponsors, OpenCollective, grants, cloud credits); accept slower pace if needed |

---

## 3. Market Research — Competitive Analysis

### Tier 1: Vibe-Coding / AI-Native Tools (Direct)

| Product | Strengths | Weaknesses | Threat Level |
|---------|-----------|------------|--------------|
| **Bolt.new** | Extremely fast prototyping | Not enterprise-grade, no conventions | Low |
| **v0.dev** (Vercel) | Excellent UI generation | UI layer only, no backend architecture | Low |
| **create-t3-app** | TypeScript-first, opinionated stack | No LLM-native design, not enterprise | Medium |
| **Wasp** (React+Node DSL) | Full-stack DSL, rapid generation | Small ecosystem, not production-proven | Low |
| **RedwoodJS** | Full-stack, test-first philosophy | Not AI-native, declining traction | Low |

### Tier 2: Traditional Enterprise Frameworks (Philosophy Comparison)

| Framework | Language | Philosophy | Key Lesson |
|-----------|----------|------------|------------|
| **Spring Boot** | Java | Convention + DI + AOP + auto-config | Auto-configuration is the killer feature |
| **Django** | Python | Batteries included | Admin panel drove viral adoption |
| **Rails** | Ruby | Convention over configuration | Scaffolding = developer velocity |
| **Laravel** | PHP | Elegant syntax + rich ecosystem | Community marketplace drives lock-in |
| **NestJS** | TypeScript | Decorator-based, module system | Familiar patterns = lower adoption friction |

### Tier 3: AI-Native Development Infrastructure

| Tool | Role | Relationship to Forge |
|------|------|-----------------------|
| **Cursor / Windsurf** | IDE layer | Complementary — Forge instructions = better completions |
| **Devin / OpenHands** | Autonomous agents | Forge provides the scaffold they build on |
| **GitHub Copilot** | Code completion | Symbiotic — Forge `.instructions.md` = higher quality output |
| **Supabase / PlanetScale** | Backend-as-a-service | Forge adapters for these platforms |

### Competitive Positioning

```
                    LLM-Native
                         │
          Bolt.new        │        ← FORGE →
          v0.dev          │    (target position)
                          │
Simple ─────────────────────────────── Enterprise
                          │
          create-t3-app   │        NestJS
          RedwoodJS        │        Spring Boot
                          │
                    Convention-First
```

**Verdict: Blue Ocean.** No current framework occupies the top-right quadrant.

---

## 4. Product Specification

### Core Design Philosophy (5 Pillars)

```
1. LLM-First Design
   Every API, convention, and file structure is designed with LLM context windows
   in mind. Code that LLMs generate should be correct Forge code by default.

2. Enterprise by Default
   Security, observability, multi-tenancy, and audit logging are not add-ons.
   They are present from `forge new` day one.

3. Convention as Code
   Conventions are enforced by tooling (linter, type system, CI gates),
   not by documentation that nobody reads.

4. Radical Extensibility
   The core stays minimal and stable; everything else is a plugin. Like Spring's
   starters and React's ecosystem, the community must be able to build, publish,
   and compose extensions without ever touching core. The framework's value
   compounds with the size of its ecosystem — not with the size of its core.

5. Future-Proof Architecture
   Forge is designed for what software development becomes next, not just what
   it is today. The same core must serve vibe-coding (now), chat-interface apps
   (emerging), multi-agent systems (near future), and outcomes/value-based
   business models (next paradigm). The architecture treats today's patterns
   as one instance of broader, future-resilient abstractions.
```

### Feature Matrix — v1.0

#### Foundation Layer
- **Schema-first development** — DB schema is the single source of truth; types, validators, API shapes, and forms are derived from it
- **Multi-tenancy built-in** — workspace/organization/account isolation models; RLS policies auto-generated and tested
- **Authentication & RBAC** — JWT + refresh tokens + role/permission/policy model; session revocation; impersonation w/ audit
- **Authorization primitives** — guards, decorators, policy functions; deny-by-default at the data layer (RLS) AND the service layer
- **Audit logging** — every mutation produces an audit trail with actor, intent, before/after diff, correlation ID
- **Migrations** — versioned, tested, reversible; type-safe schema snapshots; zero-downtime runner; preview-on-PR with diff
- **Configuration & environments** — typed config, environment promotion model (local → staging → prod), secret references (no inline secrets)
- **Dependency injection / module system** — explicit, lightweight container; no decorators-only magic; testable in isolation
- **Domain modules** — `forge generate module` scaffolds schema + service + controller + tests + LLM instructions in one command
- **Background jobs & scheduling** — durable queue adapter, cron, retries with backoff, dead-letter handling
- **Event bus & outbox** — typed events, transactional outbox pattern, at-least-once delivery semantics
- **Caching layer** — typed cache primitives (in-memory, Redis adapter), cache-key conventions, invalidation events
- **File & media storage** — pluggable adapter (S3/R2/Supabase Storage), virus scan hook, signed URLs, quota enforcement
- **Internationalization (i18n)** — typed message catalogs, locale negotiation, date/number/currency formatting
- **Email & notifications** — provider-agnostic adapter, templated messages, delivery audit, opt-out compliance
- **Search** — full-text search adapter (Postgres FTS, Meilisearch, Typesense)
- **API surface** — REST controllers, OpenAPI auto-generation; optional GraphQL/tRPC plugins
- **Webhooks (inbound & outbound)** — signature verification, retry policy, dead-letter, replay tooling
- **Realtime** — pub/sub primitives (Postgres LISTEN/NOTIFY, WebSocket/SSE adapter)
- **Idempotency & request safety** — idempotency keys on all mutations; safe-retry semantics
- **Validation everywhere** — Zod-based schemas at the boundary (HTTP, queue, webhook, env)
- **CLI kernel** — `forge` is itself a plugin host: every command (`generate`, `migrate`, `lint`, `deploy`, `doctor`, `agents`) is a plugin

#### Repo Hygiene Layer (LLM-Scratch Containment)

> *LLMs trash repos.* Every coding agent — Forge's own `ship` flow included — produces drafts, scratch scripts, throwaway test outputs, intermediate fixups, and "just to look around" files. Without a deliberate hygiene layer, the repository accretes hundreds of `_*.txt`, `patch_*.js`, `fix_*.sql`, `*_SUMMARY*.md` artifacts within months — and the agent's own context window starts re-reading its own garbage. Forge treats repo hygiene as a first-class, framework-enforced concern.

- **Hygiene manifest (`.forge/hygiene.yml`)** — declares the *known, owned* file patterns of the repo: source globs, generated artefact globs (with their generator commands), test-output globs (with their TTL), and explicit *quarantine* patterns (e.g. `_*`, `patch_*`, `fix_*`, `*_output.*`, `*_SUMMARY*.md`, `tmp_*`, `scratch/**`). Anything not matched by any rule is **unmanifested** and surfaced as a hygiene finding.
- **`forge clean` command** — three modes: `--check` (CI-safe, exits non-zero on findings, prints a table), `--dry-run` (default for humans, shows the proposed deletes/moves), and `--apply` (executes; every move is logged to the audit ledger and recoverable from `.forge/trash/<run-id>/` for a configurable retention window).
- **Per-tool ownership tags** — every framework verb that *creates* files (`forge generate`, `forge ship`, `forge scan`, `forge eval`, `forge migrate suggest`) tags its outputs with a `forge-owner: <verb>` header (or sidecar metadata for binaries). `forge clean` distinguishes *forge-owned* artefacts (safe to recreate) from *user-edited* files (never auto-deleted, only flagged).
- **LLM scratch interception** — `forge ship` runs every LLM-proposed shell command inside a sandboxed working directory. Any file the LLM writes outside declared output paths lands in `.forge/llm-scratch/<task-id>/` instead of the repo root, and is reviewed at the Code → Ship checkpoint transition. Scratch that is not promoted to a real path is deleted at the end of the task.
- **Hygiene as a `ship` checkpoint** — between *Code* and *Ship*, the workflow runs `forge clean --check`. Any unmanifested file blocks Ship until the contributor either (a) adds a manifest rule justifying the file, (b) moves it to a managed output path, or (c) explicitly deletes it. There is no "ignore for now" knob; only manifest-or-delete.
- **Pre-commit + CI gate** — `forge clean --check` runs in the pre-commit hook and as a required CI gate (§16.5.4 #11). PRs introducing unmanifested files are blocked.
- **Hygiene report in `forge insights`** — weekly digest: top unmanifested patterns, oldest stale generated artefacts, contributors who introduced the most hygiene debt, and a one-click "open hygiene PR" action that adds the missing manifest rules.
- **Privacy invariant** — quarantined and scratch files are *never* sent to any LLM context bundle. The hygiene engine and the LLM context builder share the same manifest as their source of truth.

##### `.gitignore` standards (managed, not hand-rolled)

- **`forge new` ships a curated baseline `.gitignore`** assembled from per-stack fragments under `templates/gitignore/{node,next,python,supabase,terraform,docker,os,editor,llm-scratch}.gitignore`. The framework version-stamps the file (`# forge-managed: v<X> — do not edit between markers`) and supports a user-owned section below the marker for project-specific entries.
- **Mandatory hygiene block** — every Forge `.gitignore` must contain (and `forge clean --check` enforces): `.env`, `.env.*`, `!*.example`, `!*.template`, `.forge/llm-scratch/`, `.forge/trash/`, `.forge/cache/`, `.forge/eval-runs/`, `node_modules/`, build outputs, OS junk (`.DS_Store`, `Thumbs.db`), editor temp (`*.swp`, `*~`), and the LLM-scratch globs that mirror the hygiene-manifest quarantine list (`_*.txt`, `_*.json`, `_*.sql`, `patch_*.js`, `fix_*.js`, `*_output.*`, `*_SUMMARY*.md`, `scratch/**`, `tmp_*`).
- **Negation discipline** — `.example` and `.template` files are *always* re-included via negation patterns so secret templates remain trackable; `forge clean --check` fails if a known template path is shadowed by a broader ignore. Recovery codemod: `forge upgrade gitignore`.
- **Drift detection** — `forge doctor` diffs the managed block against the current template version and offers `forge upgrade gitignore` (idempotent, preserves the user-owned section).
- **Secret-file guard** — the manifest names every file that *must never be tracked* (`.env`, `.env.local`, `.env.staging`, `.env.production`, any `*.pem`, `*.key`, `*.pfx`, service-account JSONs). `forge clean --check` runs `git ls-files` against this list and fails if any appear; pairs with the gitleaks scan below to catch already-leaked content.

##### `.gitleaks.toml` standards (secret scanning, framework-managed)

- **`forge new` ships `.gitleaks.toml`** extending the upstream gitleaks default rules with Forge-aware additions: Supabase service-role key shape (`eyJ...` JWT with `role:"service_role"`), Stripe live keys (`sk_live_*`, `pk_live_*`, `rk_live_*`, `whsec_*`), PayPal live client IDs (`A...` 80-char), OpenAI/Anthropic/Google AI provider keys, Twilio/SendGrid, social tokens (Facebook long-lived, Zalo OA), private-key PEM headers, and a generic high-entropy fallback tuned for `.env` value positions.
- **Allowlist standards** — allowlist entries require `description`, `path` (or `regex`), and an expiry `# review-by: YYYY-MM-DD`. `forge scan security --since main` fails when an allowlist entry passes its review date; PRs adding allowlists without expiry are rejected by gate (§16.5.4 #11).
- **Default scope** — scans full history on `main`, scans the PR diff in CI (fast path), and runs the diff scope as a pre-commit hook. Pre-commit can be bypassed with `--no-verify` only if the commit message contains an explicit `gitleaks-bypass: <reason>` token, which is recorded and shown in PR review.
- **Fixture safety** — the framework ships a `tests/fixtures/secrets-corpus/` of *fake* keys (clearly tagged `FORGE_FAKE_*`) used by hygiene/gitleaks tests; the gitleaks config explicitly allowlists this directory with a perpetual review date.
- **Ownership** — the file is framework-managed between markers; a `[forge.user]` section is reserved for project rules. `forge upgrade gitleaks` ports new upstream rules without clobbering user rules.
- **Privacy invariant (extends the hygiene one)** — raw match values from gitleaks output are never written to logs, telemetry, or LLM context. Findings carry only file path, line, rule ID, and a redacted preview (first 4 + last 4 chars).

> **Principle:** *the framework that produces the files is the framework that cleans them up.* A repo that uses `forge ship` for one year should be no messier than a repo that uses it for one week. Repo hygiene is not a chore — it is a CI gate.

#### Security Layer (OWASP Top 10 by default)
- Automatic input validation via schema-derived types
- Parameterized queries everywhere — SQL injection impossible by convention
- CSRF, XSS, clickjacking headers via framework middleware
- Secrets scanning in generated CI workflows
- Rate limiting templates for all public endpoints
- Dependency vulnerability scanning in `forge check`

#### Testing Layer
- Test scaffolding auto-generated per module (`forge generate test [module]`)
- RLS policy test templates
- Schema alignment tests (TypeScript types ↔ database)
- Integration test templates with real DB
- Revert experiment tests (verify fix, not just "no error")
- Security test templates (authz boundary tests)

#### Observability Layer
- Structured JSON logging (correlation IDs, trace context)
- Distributed tracing stubs (OpenTelemetry-compatible)
- Health check endpoints (`/health`, `/ready`, `/live`)
- Error boundary patterns with context capture
- Performance budget enforcement

#### LLM-Native Layer (Killer Feature)

Forge doesn't *use* LLMs as a feature — it is built around them. LLM capabilities are leveraged at every layer of the framework AND every layer of the apps developers build on it.

**Authoring time (LLMs help build the app):**
- **`.forge/instructions/`** — domain-specific prompt files per module, GitHub Copilot `.instructions.md` format, auto-updated by `forge generate`
- **Context bundles** — `forge context generate` produces compressed, semantically dense snapshots; ~60% fewer tokens vs. raw file reads
- **Convention linter** — `forge lint` detects LLM-generated code violating Forge patterns, with actionable fix messages; CI gate blocks merge
- **Anti-pattern registry** — every convention ships with its anti-pattern and why it fails (so LLMs learn what NOT to do)
- **Per-plugin LLM instructions** — every plugin ships its own LLM context (see §20.9), composed automatically into the project

**`forge ship <feature>` — the single command that runs the defined way of working (Spec → Test → Breakdown → Code → Ship):**

Forge codifies *how* the developer works, not just what tools they have. Every non-trivial change — a new feature, a behavior tweak, a bug fix — runs through **one orchestrating command, `forge ship`**, which walks the developer through four checkpoints in order, pausing for review between each. The sub-commands (`forge spec ...`) still exist as resume points and escape hatches, but the *default, recommended, documented* way to ship anything is the single command. The workflow is the framework's contract for "ship fast with high quality": the spec is the intent, the tests are the proof, the tasks are the speed, the code is the consequence.

```bash
# the entire workflow, one command, four interactive checkpoints:
forge ship <feature>                 # walks Spec → Test → Breakdown → Code → Ship
forge ship <feature> --resume        # picks up at the next un-completed checkpoint
forge ship <feature> --yes           # non-interactive (CI / agent mode); fails fast on any checkpoint
forge ship <feature> --quick         # trivial-change escape hatch (collapses Spec+Test+Tasks into one regression test); logged + flagged at >20% project usage
```

**What `forge ship` does, checkpoint by checkpoint** (all artifacts land in `.forge/specs/<feature>/`):

| # | Checkpoint | Sub-command (resume point) | Artifact produced | Pause for human review? |
|---|------------|----------------------------|-------------------|--------------------------|
| 1 | **Spec** | `forge spec new <feature>` | `spec.md` (intent + acceptance criteria) + `spec.yml` (typed data shape, authz model, events, scan policy) | Yes — developer (or LLM) drafts; `forge ship` opens it in `$EDITOR`, validates shape, refuses to advance until required fields are present and references resolve. |
| 2 | **Test** | `forge spec test` | `tests/spec.test.ts` + `tests/spec.integration.test.ts` + `tests/spec.rls.test.ts` + `tests/spec.scan.baseline.json` | Yes — generated tests are shown as a diff; developer accepts or edits; `forge ship` runs them and **expects red** (failing). Green tests at this stage abort the workflow with "your spec is already implemented or your tests don't actually test the spec." |
| 3 | **Breakdown** | `forge spec breakdown` | `tasks.md` (ordered checklist, 3–12 tasks) + per-task `.forge/context/<task>.md` (tight LLM context bundle) | Yes — task list is shown; developer can split, merge, reorder, or accept. `forge ship` warns if any task's context bundle exceeds the token budget (cure: split, don't truncate). |
| 4 | **Code** | `forge spec next` (loop) | feature code + updated `.forge/instructions/` | Per-task — `forge ship` picks the next ready task, hands it + its context bundle to the configured LLM (or just opens the file for human implementation), then runs only the tests touched by that task. Loops until all tasks are checked. |
| 5 | **Ship** | `forge spec ship` (auto-runs after task 4 completes) | mergeable PR | Final — runs the full test suite + `forge scan` + convention linter + the timestamp/git check that no test was authored *after* its corresponding production code. Green = mergeable PR with auto-generated description pulling from `spec.md` + `tasks.md`. Red = `forge ship` reports which checkpoint to return to and why. |

**Why one command, not five:**
- **Discoverability** — a new developer learns *one* verb (`forge ship`) and the framework teaches them the workflow by walking them through it. They don't need to memorize five commands and their order.
- **Order is enforced, not remembered** — the most common TDD failure is humans skipping the "write the test first" step. `forge ship` makes skipping a step impossible without an explicit `--skip-checkpoint=test` flag (which is logged, surfaces in `forge insights`, and counts toward the workflow-smell threshold).
- **Resumable, never lost** — every checkpoint writes its artifact to disk before pausing. Crash, reboot, lunch break, week-long context switch — `forge ship <feature> --resume` picks up exactly where the workflow stopped, with the same context bundle and the same task pointer.
- **Agent-friendly by construction** — `forge ship --yes` runs the entire workflow non-interactively for AI agents (Devin, Cursor agent mode, custom Copilot agents). Each checkpoint emits a structured JSON event (`spec.created`, `tests.generated`, `tasks.broken-down`, `task.completed`, `ship.passed|failed`) that an orchestrator can hook into.
- **Single PR, single audit trail** — `forge ship` produces *one* PR per feature, with `spec.md` + `tasks.md` + tests + code + scan results all in the same diff. Reviewers see intent, proof, plan, and execution together.
- **Sub-commands remain first-class** — power users can still call `forge spec test`, `forge spec breakdown`, etc. directly to re-run an individual checkpoint without rewinding the whole workflow. `forge ship` is the orchestrator; the sub-commands are the operations it composes.

**Properties that survive the collapse to one command:**
- **Spec is executable, not aspirational.** The spec file is parsed by every other Forge command — generators, scanners, eval harness, instructions evolution. A spec without tests is rejected. A test without a spec is flagged as orphan.
- **Tests precede code, always.** Checkpoint 5 (Ship) refuses to mark a feature done if any test was authored *after* its corresponding production code (timestamp + git history check). This eliminates the most common TDD failure mode: tests written to fit the code that already exists.
- **Tasks are LLM-sized.** Each task is scoped to fit comfortably in a single AI prompt with its bundled context. Forge warns when a task's context bundle exceeds the configured token budget — the cure is to split the task, not to truncate the context.
- **The workflow is the audit trail.** `spec.md` + `tests/` + `tasks.md` + git history together form a self-documenting record of *why* the change exists, *what* it promised, and *how* it was verified. PR templates pull from this automatically.
- **Escape hatch (rare).** Trivial changes (typos, dependency bumps, log-message tweaks) use `forge ship --quick` which collapses Spec+Test+Tasks into a single regression test + PR description. The escape hatch is logged; if a project uses it for >20% of changes, `forge insights` flags it as a workflow smell.
- **LLM-native at every checkpoint.** Each checkpoint ships its own prompt template (`.forge/prompts/ship-*.prompt.ts`), so any LLM — Copilot, Claude, Cursor, Aider, local Ollama — can drive the workflow uniformly. The workflow does not depend on a specific vendor.
- **Backed by the learning loop.** Features shipped via `forge ship` that later get reverted or hot-fixed feed `.forge/learned/spec-failures.jsonl` — future runs of `forge ship` get smarter about the questions to ask at the Spec checkpoint (e.g. "the last 3 specs that omitted an idempotency clause were reverted within a week").

> **Principle:** *One command, four checkpoints, no shortcuts.* `forge ship` is the verb of the framework; everything else is an operation it composes. Spec is the intent. Tests are the proof. Tasks are the speed. Code is the consequence. A vibe-coder who runs `forge ship` cannot accidentally ship a liability — the framework will not let them.

**Build & test time (LLMs help fix and improve):**
- **`forge fix`** — LLM-powered auto-fix for lint failures, type errors, failing tests, broken migrations; produces a diff with explanation
- **`forge explain <error>`** — explains any error in context of the project, with citations to relevant code and instructions
- **`forge generate test --from-bug <issue>`** — converts a bug report into a regression test, then proposes the fix
- **AI code review** — `forge review` runs an LLM pass on PR diffs against project conventions, security checklist, and anti-patterns
- **Migration assistant** — `forge migrate suggest` proposes schema migrations from natural-language intent, with rollback included
- **Doc & changelog generation** — `forge docs sync` keeps READMEs, OpenAPI descriptions, and CHANGELOG aligned with code

**Runtime (apps built on Forge use LLMs natively):**
- **First-class LLM provider adapter** — pluggable (OpenAI, Anthropic, local, others); typed prompts, streaming, structured outputs, tool calling
- **Prompt registry** — versioned, testable prompts as code (`prompts/refund_email.prompt.ts`), with eval harness
- **Eval harness** — `forge eval` runs prompt suites with assertions; CI gate prevents prompt regressions
- **AI cost & token budgets** — per-workspace, per-actor, per-capability budgets enforced at the runtime layer; prevents runaway spend
- **Capability metadata for LLMs** — every Capability (§21) ships an LLM-readable manifest, so the same business logic is callable from chat, agents, and external tools without rewrites
- **Semantic search & embeddings** — first-class adapter (pgvector / Pinecone / etc.), embedding lifecycle managed by framework
- **Guardrails** — structured-output validation, refusal handling, jailbreak detection, PII redaction, content moderation hooks

**Token economy (efficient token use is a first-class framework concern):**

Every LLM call — whether made by Forge tooling or by an app built on Forge — flows through a **token-aware execution layer**. The goal: *cheapest model that meets the quality bar, smallest context that answers the question, zero tokens spent twice.*

- **Prompt compiler** — prompts written as typed templates are compiled at build time: dead-branch elimination, whitespace/comment stripping, schema-to-JSON-Schema minification, automatic few-shot pruning. Typical savings: 20–40% input tokens vs. hand-written prompts.
- **Context budgeter** — every prompt declares a `maxInputTokens`; the framework auto-truncates retrieval results, summarizes overflow, and warns at build time if static context already exceeds the budget.
- **Semantic + exact response cache** — deterministic prompts hit an exact KV cache; near-duplicate prompts hit an embedding-similarity cache (configurable threshold). Cache keys include model + temperature + tool schema for correctness.
- **Prompt-prefix / KV-cache awareness** — system prompts and few-shots are placed first and kept stable across calls so providers (Anthropic prompt caching, OpenAI cached input, vLLM prefix cache) bill them at discounted rates; framework warns when an edit would invalidate a hot prefix.
- **Model router & cascade** — declare a quality tier (`cheap` / `balanced` / `frontier`); router picks the cheapest model that passes the eval harness for that prompt. Optional cascade: try cheap model first, escalate only on low-confidence / failed structured-output / explicit `escalate()`.
- **Speculative + batched calls** — `llm.batch([...])` coalesces concurrent requests; speculative decoding via small draft models supported where the provider allows.
- **Streaming-first + early-stop** — structured-output streaming with early termination as soon as the schema is satisfied; saves output tokens on long generations.
- **Embedding deduplication & reuse** — content-addressed embedding store; identical chunks are embedded once across the workspace, not per-document.
- **Retrieval, not stuffing** — RAG primitives are the default; full-document context is opt-in and emits a build-time warning above a configurable size.
- **Tool-call minimization** — tool schemas are compiled to the smallest valid JSON Schema; unused tools are auto-pruned per call site; framework prefers a single planning call + parallel tool execution over chatty back-and-forth.
- **Token telemetry & per-feature attribution** — every call is tagged with capability + actor + tenant; `forge insights` shows cost-per-feature and flags regressions in tokens-per-outcome (not just tokens-per-call).
- **CI cost gate** — `forge eval` reports tokens + $ per scenario; PRs that increase cost beyond a threshold without a quality gain are blocked, mirroring the perf-budget pattern.
- **Token budgets as code** — budgets per workspace / capability / tenant are typed config; exceeding a soft budget triggers downgrade to the next-cheaper model, exceeding hard budget returns a typed `BudgetExceededError` the app can handle gracefully.
- **Self-optimizing prompts** — opt-in: `forge optimize prompts` uses an LLM-driven loop (DSPy-style) to shorten/restructure prompts while holding eval scores constant.

> **Principle:** *"Every token spent is a token measured."* Forge treats LLM tokens like database queries — observable, budgeted, cached, and optimized by default, not as an afterthought.

**Scan-and-fix layer (`forge scan` — the unified issue-detection-and-remediation pipeline):**

A single command, multiple specialized scanners, one consistent UX. Every scanner runs in three modes: `--report` (find only), `--suggest` (LLM-written PR description with diff), `--apply` (open a PR; never auto-merge per §8 Q8). Each scanner ships its own LLM context so it understands *why* something is wrong in this project, not just *what* matches a generic rule.

| Scanner | What it finds | What it fixes |
|---------|--------------|---------------|
| `forge scan security` | OWASP Top 10, missing authz on routes, RLS gaps, secrets in code, vulnerable deps (CVE feed), misconfigured CORS/CSP, unsigned webhooks | Add missing guards, generate RLS policies, rotate leaked secrets, upgrade deps with codemods, harden middleware |
| `forge scan performance` | N+1 queries, missing indexes, oversized payloads, unbatched LLM calls, hot-path allocations, sync work in async paths, missing cache | Add indexes (with migration), batch queries, paginate endpoints, add cache primitives, convert to streaming |
| `forge scan reliability` | Missing idempotency keys, missing outbox writes, retry-without-backoff, unbounded queues, missing circuit breakers, missing health checks | Wrap mutations with idempotency, add outbox pattern, add backoff, generate health endpoints |
| `forge scan correctness` | Float math on monetary values, untyped state machines, missing audit emission, schema/type drift, unhandled promise rejections | Convert to `Money` primitive, generate state-machine, add audit emission, regenerate types |
| `forge scan accessibility` | Missing ARIA, color-contrast failures, keyboard-trap forms, missing alt text on generated UI | Apply codemods to generated React components |
| `forge scan cost` | LLM calls without budget, oversized prompts, missing cache hits, expensive model when cheap would pass eval | Add budget config, compress prompts, switch to cached prefix, downgrade model with eval check |
| `forge scan compliance` | Missing PII tags, missing data-residency tags, missing consent records, missing right-to-erasure handlers (GDPR/HIPAA-relevant) | Add tags, generate consent module, scaffold erasure handler |
| `forge scan dx` | Stale `.forge/instructions/`, missing tests for new modules, undocumented public APIs, broken doc anchors | Regenerate instructions, scaffold missing tests, add stub docs |

**Cross-cutting properties:**
- **Diff-first** — every finding shows a proposed unified diff before any change
- **Confidence scored** — each finding ships with a `confidence: high|medium|low`; only `high` is eligible for `--apply` by default
- **Replayable** — findings are stored as structured JSON in `.forge/scan-history/`; CI can compare runs and fail on regressions
- **Composable** — `forge scan all` runs every scanner; `forge scan --since main` runs only on PR diff
- **Extensible** — third-party scanners register as plugins (§20); `forge scan myco-pii` is just a published package
- **Pre-commit + CI gate** — opt-in pre-commit hook runs the fast scanners (`security`, `correctness`); CI runs the full suite with annotation comments on the PR

> **Principle:** *find it → explain it → propose the fix → never apply without a human OK.* Scanners are the framework's autoimmune system; the developer remains the surgeon.

**Continuous learning loop (the framework gets smarter as the developer vibe-codes):**

Forge treats every interaction with the developer as a training signal — not for a model, but for the project's *own* convention library. Vibe-coding is a feedback loop, and Forge closes the loop locally so the framework gets sharper with every PR, every fix, every reverted commit.

- **Convention learning** — when `forge lint` flags a pattern and the developer either accepts or rejects the fix, the decision is recorded in `.forge/learned/conventions.jsonl`. After N similar rejections, `forge learn promote` proposes a new project-local lint rule (with the rationale derived from the rejection comments). The developer accepts → the rule joins the project conventions; LLMs see it on the next prompt.
- **Anti-pattern mining from reverts** — git reverts and hot-fix commits are mined for the diff between "what the LLM wrote" and "what the developer ended up with." Recurring patterns become candidate anti-patterns; `forge learn antipatterns` opens a PR adding them to `.forge/instructions/anti-patterns.md`.
- **Prompt-quality feedback** — every `forge fix` / `forge generate` / `forge review` PR is tagged. When a PR is closed without merge, modified heavily before merge, or reverted within 7 days, the framework records the prompt + response + outcome. Aggregated, these become eval-harness scenarios that prevent the same mistake twice.
- **Per-project context evolution** — `.forge/instructions/*.instructions.md` files are not write-once. `forge instructions evolve` reads the project's recent PR/issue/review history and proposes targeted edits (with citations to the commits that motivated each edit). Every change is a reviewable PR; nothing edits instructions silently.
- **Test-from-bug** — every closed bug issue triggers an offer to generate a regression test that would have caught it; the test joins the suite and the LLM sees the bug pattern in future suggestions.
- **Pair-programming memory** — opt-in: an ambient `.forge/session/` log of accepted/rejected suggestions during a coding session. At session end, `forge session digest` produces a one-screen summary of "what your AI got wrong today" — and offers to bake the lessons into instructions.
- **Federated convention sharing** — opt-in: a project can publish anonymized convention/anti-pattern frequencies to the Forge Registry. Other projects pull aggregate trends ("89% of Forge projects in fintech now require idempotency on POSTs") as starter context. **Never** code, **never** PII — only convention IDs and counts. Single flag to disable.
- **Eval harness grows with the project** — every accepted scan finding becomes a prompt-eval scenario; the eval suite expands organically and protects against future regressions.
- **`forge teach` (manual override)** — the developer can directly say *"never suggest X again in this project"* or *"always prefer Y over Z"*. Stored in `.forge/learned/preferences.yml`, surfaced as additional system context to every LLM call in the project.

Concrete example — what the learned state actually looks like on disk:

```jsonl
// .forge/learned/conventions.jsonl  (one JSON object per line; append-only)
{"ts":"2026-04-12T10:14Z","rule":"money-no-float","pattern":"price: number","verdict":"rejected","count":7,"rationale":"team uses Money primitive"}
{"ts":"2026-04-18T09:02Z","rule":"money-no-float","pattern":"price: number","verdict":"promoted-to-lint","count":12,"pr":"#418"}
```

```yaml
# .forge/learned/preferences.yml  (curated by `forge teach`; reviewed in PR)
never_suggest:
  - id: float-money
    pattern: "\\b(price|amount|total)\\s*:\\s*number\\b"
    reason: "Use Money primitive (see src/shared/money.ts)"
prefer:
  - over: "new Date()"
    use: "clock.now()"
    reason: "Deterministic in tests"
```

> **Principle:** *the framework is a student of the codebase, not a lecturer.* Every interaction makes the next interaction better. The intelligence accrues to the developer's project — not to a SaaS backend they don't control.

**Operate time (LLMs help run, heal, and improve):**
- **Self-healing runtime hooks** — incidents (uncaught exceptions, failed jobs, schema drift, slow queries) emit a structured event; an optional `@forge/healer` plugin consults LLMs to propose a PR with the fix
- **Auto-triage** — failing CI runs, error reports, and Sentry alerts are summarized + clustered + assigned by LLM with a suggested first action
- **Auto-rollback advisor** — when a deploy regresses an SLO, an LLM correlates the diff with the failure and recommends the minimal revert
- **Performance & cost optimizer** — `forge insights` periodically scans logs/traces and produces an LLM-written report of hot paths, N+1 queries, oversized payloads, with proposed fixes
- **Observability assistant** — `forge ask "why is /checkout slow?"` queries logs/traces/metrics and answers in natural language with evidence
- **Drift detector & self-update** — detects schema/code/dep drift; LLM proposes a remediation PR (e.g., regenerated types, refreshed views, dep upgrade with codemod)
- **Documentation healer** — when code changes outpace docs, `forge docs heal` opens a PR to resync

> **Principle:** every error, failed test, or anomaly is an opportunity for the framework (or its plugins) to propose a fix. LLM assistance is opt-in per project but pervasive across the stack.

#### Deployment Layer
- Docker + docker-compose templates
- GitHub Actions workflows (test → staging → production)
- Cloud adapters: DigitalOcean, AWS, GCP, Vercel, Railway
- Environment promotion model (local → staging → production)
- Zero-downtime migration runner
- Rollback procedures per migration

---

### Command Surface — the verbs of the framework

> *A framework is its CLI before it is anything else.* The command surface is the part of Forge that every developer touches every day; if it is incoherent, no amount of internal architecture can rescue the developer experience. This section is the **audited, reorganized, philosophy-aligned command set** — the canonical source of truth for what Forge ships and *why each verb exists.*

**Design rules the command surface obeys** (derived from §11 design principles + the ultimate goal):

| Rule | Manifestation |
|------|---------------|
| **One verb per intent** | Each top-level command answers exactly one question the developer is asking. No "swiss army" verbs. |
| **One way to do it (with an escape hatch)** | The default path is single and well-lit; every default has a documented `--flag` override. |
| **Agent-readable by default** | Every command supports `--json` (structured output), `--yes` (non-interactive), and `--explain` (why it ran what it ran). Every error has a stable `error_code` and a docs link. |
| **Errors must teach** | Failure messages always include cause + fix + `error_code` + docs link. No "an error occurred." |
| **LLM-native at every step** | Every command ships its own prompt template under `.forge/prompts/<command>-*.prompt.ts`; vendor-neutral. |
| **Boring by default** | Defaults are the safe, well-understood choice. Surprises require an explicit flag. |
| **The Way is one command** | The defined methodology (Spec → Test → Breakdown → Code → Ship) is invoked through a single orchestrating verb (`forge ship`); checkpoints are sub-commands of that verb, not free-standing peers. |

**The command set, organized by namespace** — 9 namespaces, each mapped to a pillar of the philosophy:

| # | Namespace | Pillar served | Top-level commands |
|---|-----------|--------------|---------------------|
| 1 | **Project lifecycle** | Onboarding & exit (no lock-in) | `forge new` · `forge adopt` · `forge eject` · `forge doctor` |
| 2 | **The Way: `ship`** | The defined methodology — TDD-shaped vibe-coding | `forge ship <feature>` (orchestrator) + `ship spec\|test\|breakdown\|code\|verify\|status\|resume`; flags `--quick` `--yes` `--from=<checkpoint>` `--skip-checkpoint=<name>` |
| 3 | **Generate** | Scaffolding building blocks (loop 1 of 3: *generate*) | `forge generate <kind>` (module / test / trpc / graphql / migration / fixtures) · `forge add <primitive>` (auth / billing / storage / plugin) · `forge fixtures` |
| 4 | **Scan & Fix** | The autoimmune system (loop 2 of 3: *scan*) | `forge scan <family>` (security / performance / correctness / cost / accessibility / compliance / reliability / dx / all) · `forge fix` (apply high-confidence diffs from latest scan) · `forge lint` (fast convention linter, pre-commit) · `forge review` (LLM PR review) · `forge check` (pre-flight: typecheck + lint + test) |
| 5 | **Learn** | The brain (loop 3 of 3: *learn*) | `forge learn promote` · `forge learn antipatterns` · `forge learn teach` · `forge learn session` · `forge learn instructions` · `forge learn share` · `forge optimize <kind>` (prompts / cost / queries) · `forge eval` (prompt regression suite) |
| 6 | **Context & Ask** | LLM ergonomics (token economy + Q&A) | `forge context generate\|show\|budget` · `forge ask <question>` · `forge explain <error\|symbol>` |
| 7 | **Data: `migrate`** | Database lifecycle | `forge migrate up\|down\|status\|suggest\|repair` (`--dry-run` flag) · `forge check schema` (alignment verification) |
| 8 | **Audit & Compliance** | Regulated-domain proofs | `forge audit verify` (audit-log integrity) · `forge audit export` (compliance package: SOC2 / PCI / HIPAA / GDPR-ready bundle) · `forge audit erase <subject>` (right-to-erasure) |
| 9 | **Operate** | Production ops | `forge deploy` · `forge rollback --to <ref>` · `forge backup` · `forge insights` (cost/latency/quality dashboard) · `forge agents <subverb>` (start/stop/list runtime agents) · `forge upgrade` (codemods when Forge itself releases breaking changes) |
| 10 | **Plugin** | Extensibility (§20) | `forge plugin add\|remove\|list\|search\|inspect\|docs` |
| 11 | **Docs** | Keep docs honest | `forge docs sync` (regenerate from code) · `forge docs heal` (fix broken anchors/links) |
| 12 | **Hygiene** | Repo health (LLM-scratch containment) | `forge clean --check\|--dry-run\|--apply` · `forge hygiene report` · `forge hygiene manifest <add\|validate>` |

> Twelve namespaces is more than nine; the last three (Plugin, Docs, Hygiene) are intentionally separated because they touch *different mental models* — Plugin is about extending the framework, Docs is about explaining what it does, Hygiene is about preventing the framework (and its LLMs) from trashing the repo it operates on. Collapsing them would be premature normalization.

**The four "first-day" verbs** — what a brand-new Forge developer learns in the first hour, in this exact order:

```bash
forge new my-saas         # 1. bootstrap a project (5 minutes to running app)
cd my-saas
forge ship auth/email     # 2. ship your first feature, the right way (Spec → Test → Breakdown → Code → Ship)
forge scan all            # 3. let the autoimmune system audit what you (and your AI) just wrote
forge insights            # 4. see what your tokens, dollars, and quality scores look like
```

Everything else is reachable but not required for hour one. This is the **discoverability contract**: a developer who knows only those four verbs can ship a production-quality vertical slice on day one.

**Rename / consolidation map** (from earlier sections of this document — the audit cleaned these up):

| Was (deprecated alias kept for one minor) | Is now | Why |
|--------------------------------------------|--------|-----|
| `forge spec new <feature>` | `forge ship spec` | Sub-checkpoints belong under the orchestrator verb; reduces top-level surface area. |
| `forge spec test` | `forge ship test` | Same. |
| `forge spec breakdown` | `forge ship breakdown` | Same. |
| `forge spec next` | `forge ship code` | "Code" matches the checkpoint name (Spec → Test → Breakdown → **Code** → Ship). |
| `forge spec ship` | `forge ship verify` | The orchestrator is `forge ship`; the final checkpoint that runs full suite + scan + git/timestamp check is `verify`. |
| `forge spec status` | `forge ship status` | Same namespace. |
| `forge spec quick` | `forge ship --quick` | Was already a flag; promote it. |
| `forge teach` | `forge learn teach` | Teaching the framework *is* the learning loop. |
| `forge session digest` | `forge learn session` | Session lessons feed `.forge/learned/`; same loop. |
| `forge instructions evolve` | `forge learn instructions` | Instructions evolve *because* the framework learned. |
| `forge optimize prompts` | `forge optimize prompts` (kept) — also surfaced as part of `forge learn` | Prompt optimization is a learning-loop product but reads cleaner as a primary verb when invoked manually. |
| `forge gdpr erase <subject>` | `forge audit erase <subject>` | GDPR is one of many regimes; erasure is an audit-trail operation. |
| `forge compliance export` | `forge audit export` | Compliance is a sub-concern of audit. |
| `forge migrate-code` | `forge upgrade` | "Migrate code" was confusable with DB migrations; "upgrade" is unambiguous. |
| `forge generate ai-context` | `forge context generate` | Context belongs in the `context` namespace, not `generate`. |
| `forge agents stop --workspace` | `forge agents stop` (with `--workspace` flag) | Promoted to `agents` namespace alongside `start`/`list`. |

**Universal flags** (every command, no exceptions — agent-readable principle):

- `--json` — structured output (NDJSON for streams, JSON for single-shot); stable schema per command, versioned in `.forge/cli-schemas/<command>.schema.json`
- `--yes` — non-interactive; abort on any prompt; intended for CI and AI-agent orchestration
- `--explain` — print the *why* (which prompt, which template, which convention triggered) without executing
- `--dry-run` — show what would happen, change nothing
- `--workspace=<id>` — operate against a specific workspace (multi-tenant projects)
- `--no-color` / `--quiet` / `--verbose=<level>` — output verbosity controls
- `--profile=<name>` — pick a named CLI profile (`fast` / `safe` / `paranoid`) that adjusts confidence thresholds, scan strictness, and LLM cost ceilings

**Stable error codes** (every failure carries a `FORGE-XXXX` code; documented in `forge ask error <code>` and at `forge.dev/errors/<code>`). Examples: `FORGE-1001` (spec missing required field), `FORGE-2003` (test green at red-tests checkpoint — already implemented), `FORGE-3007` (scan finding above confidence threshold blocks ship), `FORGE-4002` (skip-checkpoint flag required), `FORGE-5009` (token budget exceeded for task context bundle).

> **Principle:** *the CLI is the framework's grammar.* If a developer cannot guess the verb for what they want to do, the verb is in the wrong namespace. Quarterly CLI audits run `forge insights cli` to find verbs nobody uses, verbs everyone misspells, and verbs whose `--json` schema has drifted from the docs — and fix all three.

---

## 5. Technical Architecture

### System Overview

```
┌──────────────────────────────────────────────────────────┐
│                    forge CLI                             │
│ new | generate | migrate | test | lint | fix | review |  │
│ eval | insights | ask | optimize | deploy                │
└───────────────────────┬──────────────────────────────────┘
                        │
┌───────────────────────▼──────────────────────────────────┐
│                  Core Runtime                            │
│  ┌───────────┐  ┌───────────┐  ┌───────────┐            │
│  │  Schema   │  │   Auth    │  │  Tenancy  │            │
│  │  Engine   │  │  Module   │  │  Module   │            │
│  └───────────┘  └───────────┘  └───────────┘            │
│  ┌───────────┐  ┌───────────┐  ┌───────────┐            │
│  │ Security  │  │   Queue   │  │  Billing  │            │
│  │  Guards   │  │ /Events   │  │  Module   │            │
│  └───────────┘  └───────────┘  └───────────┘            │
│  ┌───────────┐  ┌───────────┐  ┌───────────┐            │
│  │ Capability│  │ Idempotency│ │   Audit   │            │
│  │  Registry │  │  /Outbox  │  │   /Trace  │            │
│  └───────────┘  └───────────┘  └───────────┘            │
└───────────────────────┬──────────────────────────────────┘
                        │
┌───────────────────────▼──────────────────────────────────┐
│              LLM Runtime & Token Economy                 │
│  ┌───────────┐  ┌───────────┐  ┌───────────┐            │
│  │  Prompt   │  │  Model    │  │  Context  │            │
│  │ Compiler  │  │  Router/  │  │ Budgeter  │            │
│  │ +Registry │  │  Cascade  │  │           │            │
│  └───────────┘  └───────────┘  └───────────┘            │
│  ┌───────────┐  ┌───────────┐  ┌───────────┐            │
│  │ Response  │  │ Embedding │  │  Token    │            │
│  │  Cache    │  │  Store    │  │ Budgets + │            │
│  │ (KV+sem)  │  │  (dedup)  │  │ Telemetry │            │
│  └───────────┘  └───────────┘  └───────────┘            │
│  ┌───────────┐  ┌───────────┐  ┌───────────┐            │
│  │   Eval    │  │ Guardrails│  │  Healer/  │            │
│  │  Harness  │  │ /Redaction│  │ Insights  │            │
│  └───────────┘  └───────────┘  └───────────┘            │
└───────────────────────┬──────────────────────────────────┘
                        │
┌───────────────────────▼──────────────────────────────────┐
│               LLM Context System                         │
│   .forge/instructions/ + context-bundles/ + lint-rules/  │
└───────────────────────┬──────────────────────────────────┘
                        │
┌───────────────────────▼──────────────────────────────────┐
│              Adapter Layer (pluggable)                   │
│  DB:      Supabase | PlanetScale | Neon | raw PostgreSQL │
│  LLM:     OpenAI | Anthropic | Bedrock | local (vLLM)    │
│  Vector:  pgvector | Pinecone | Qdrant                   │
│  Billing: Stripe | Paddle | LemonSqueezy                 │
│  Email:   Resend | SendGrid | Postmark                   │
│  Storage: S3 | R2 | Supabase Storage                     │
└──────────────────────────────────────────────────────────┘
```

> **Note on Scan & Learn placement:** The `forge scan` scanners (security/performance/correctness/...) and the continuous-learning state store (`.forge/learned/`, `.forge/scan-history/`, `.forge/session/`) are *cross-cutting* — they ride on the CLI tier (as `forge scan` / `forge teach` / `forge learn` commands), persist project-local state next to source code, register as plugins via the Extension API (§20), and feed evolved instructions back into the LLM Runtime tier. They are deliberately not drawn as a single box because they wrap every other layer rather than sitting at one level.

### Technology Choices

**Phase 1 — TypeScript/Node.js**
- Runtime: Node.js 20+ (LTS)
- Language: TypeScript 5+ (strict mode enforced)
- Framework adapters: Next.js 15, Hono, Fastify
- Database: PostgreSQL (primary), via Supabase or direct Prisma/Drizzle
- Testing: Vitest (unit/integration), Playwright (e2e)
- Package manager: pnpm (workspaces for monorepo support)

**Phase 2 — Python adapter**
- FastAPI + SQLAlchemy + Alembic
- Same conceptual model, different runtime

**Phase 3 — Go adapter**
- High-performance microservice use cases
- Community-driven

### Scaffolded Project Structure

```
my-app/
├── .forge/
│   ├── instructions/           # LLM context files (per domain)
│   │   ├── global.instructions.md
│   │   ├── auth.instructions.md
│   │   ├── billing.instructions.md
│   │   └── [module].instructions.md
│   ├── conventions.json        # Machine-readable convention registry
│   ├── context-bundles/        # Compressed context snapshots
│   └── lint-rules/             # Custom convention lint rules
├── src/
│   ├── modules/                # Domain modules
│   │   ├── auth/
│   │   │   ├── auth.service.ts
│   │   │   ├── auth.controller.ts
│   │   │   ├── auth.types.ts
│   │   │   └── auth.test.ts
│   │   ├── workspace/
│   │   ├── billing/
│   │   └── [feature]/
│   ├── shared/
│   │   ├── guards/             # Auth/RBAC guards
│   │   ├── decorators/
│   │   ├── middleware/
│   │   └── types/
│   └── infrastructure/
│       ├── database/
│       ├── queue/
│       └── storage/
├── migrations/
│   ├── 20260101000000_init.sql
│   └── [timestamp]_[description].sql
├── tests/
│   ├── unit/
│   ├── integration/
│   └── security/               # RLS + authz boundary tests
├── .github/
│   └── workflows/
│       ├── ci.yml              # Test + lint on PR
│       ├── deploy-staging.yml
│       └── deploy-production.yml
├── forge.config.ts             # Framework configuration
├── .forge-conventions          # Convention enforcement config
└── docker-compose.yml
```

### Convention Enforcement System

```typescript
// forge.config.ts
export default defineForgeConfig({
  tenancy: {
    model: 'workspace',           // workspace | organization | account
    isolationLevel: 'row',        // row (RLS) | schema | database
  },
  auth: {
    provider: 'supabase',         // supabase | next-auth | custom
    sessionStrategy: 'jwt',
  },
  security: {
    owasp: 'strict',              // strict | standard | custom
    rateLimit: true,
    auditLog: true,
  },
  testing: {
    coverage: { statements: 80, branches: 70 },
    rlsTests: true,               // auto-generate RLS boundary tests
    schemaAlignment: true,        // TypeScript ↔ DB sync tests
  },
  llm: {
    instructionsFormat: 'github-copilot',  // or 'cursor' | 'windsurf'
    contextBundles: true,
    conventionLinter: 'error',    // error | warn | off
  },
});
```

---

## 6. Development Roadmap

Four milestones, sequenced by capability rather than calendar dates. Each milestone has a clear exit criterion; we ship when the criterion is met.

| Milestone | Exit Criterion | Scope | Headcount Assumption |
|-----------|---------------|-------|----------------------|
| **M0 — Foundations** | `forge new my-saas` produces a running app with auth, multi-tenancy, migrations, and audit log | Monorepo bootstrap; schema-first migrations + type-gen; multi-tenancy + RLS auto-gen; auth + RBAC; CLI kernel; `forge explain`; RFC process; CONTRIBUTING/COC | 1 founder full-time |
| **M1 — Alpha** | **20+ external developers ship a "qualifying app"** (definition below) to production on Forge | Security layer (OWASP gates); test scaffolding (RLS, schema-alignment, integration); `.forge/instructions/` + `forge lint`; **`forge scan` (security + correctness scanners) with pre-commit + CI gates (§4)**; CI/CD templates; docs site; 3 reference apps (Next.js + Hono + Fastify); `forge adopt` for incremental adoption | 1 founder + 2–3 part-time contributors |
| **M2 — Beta** | 50+ apps in beta; community publishes ≥10 adapters (incl. ≥3 scanner plugins) | Billing (Stripe) + observability (logs/traces/health); LLM runtime layer (provider adapter, prompt registry, eval harness); `forge fix` + `forge review`; **remaining `forge scan` families (performance/reliability/accessibility/cost/compliance/dx)**; **continuous learning loop MVP (`.forge/learned/`, convention learning, anti-pattern mining from reverts, `forge teach`) (§4)**; plugin/adapter API; third-party security audit | Core team of 3–4 funded by sponsorships |
| **M3 — v1.0** | Stable API contract; 25+ production apps; ecosystem flywheel turning | API stability + deprecation policy; Forge Registry MVP; multi-agent + chat-runtime plugins (§21); regulated-industry templates (§12); **opt-in federated convention sharing**; **scanner-plugin marketplace**; migration guides from create-t3-app/NestJS; launch | Core team + active community |

**Definition of a "qualifying app" (M1 exit gate):**
1. Built by a developer who is **not** on the Forge core team
2. Has ≥1 paying user OR ≥5 active non-paying users for ≥30 consecutive days
3. Uses at least 4 of: auth, multi-tenancy, migrations, audit log, billing, RLS, `.forge/instructions/`
4. Developer has answered a 10-minute survey on what worked and what didn't
5. App is publicly listed in the Forge Showcase (with developer's consent)

**Milestone dependencies:** M0 → M1 requires the docs site to score ≥7/10 in 5 onboarding interviews. M1 → M2 requires the plugin/adapter API to be stable enough that 3 external contributors can ship adapters without core-team help. M2 → M3 requires the third-party security audit to pass with no Critical findings.

**Out of v1.0 scope** (post-1.0 milestones): Python adapter, Go adapter, hosted Forge Cloud, full multi-agent coordinator. The validation builds in §21.8 (chat-first SaaS, multi-agent CRM, outcome-billed underwriter, hybrid stress test) gate the v1.0 release.

**Worst-case timeline (no sponsorship, founder-only):** M0 = 6 months, M1 = +12 months, M2 = +18 months, M3 = +18 months. Total ≈ 4.5 years to v1.0. **Best-case (funded core team by month 12):** ≈18–24 months. We publish actual progress quarterly so the community can adjust expectations honestly.

**v1.0 KPIs:** 5,000+ GitHub stars · 100+ contributors · 25+ production apps · 50+ community plugins · ecosystem health metrics per §20.11 on track.

---

## 7. Go-to-Market Strategy

### Phase 1: Community Seeding (Pre-launch → v1.0)

**GitHub Strategy**
- README-driven development — the README must be compelling before the code is complete
- GitHub Discussions for RFC process — community shapes design from day one
- `good-first-issue` labels for onboarding contributors immediately
- Showcase section listing production apps built on Forge

**Content Strategy**
- Blog series: *"Build [X] production-ready with Forge in [Y] minutes"*
  - SaaS subscription app
  - Multi-tenant B2B dashboard
  - Banking-grade API service
- Benchmark post: *"Vibe-coded app vs Forge app — production readiness scorecard"*
- Video: *"Why your AI-generated app will fail in production (and how Forge fixes it)"*
- Live stream: Build a complete SaaS from zero → deployed in one day using Forge

**Developer Relations**
- Primary audience: indie hackers, solo founders, small engineering teams
- Partner with Cursor, Windsurf — publish official "Forge cursor rules" 
- Submit talks to NodeConf, TSConf, Next.js Conf, EpicWebConf

**Viral Mechanics**
- `forge new` offers a **"Built with Forge" footer badge as opt-in** during init (default: prompt with friendly explanation; never silently enabled). *Lesson learned from Vercel's opt-out attribution backlash — we earn the badge through delight, not friction.*
- Forge Registry — marketplace for community modules (discoverability loop)
- Template gallery: SaaS starter, banking API, healthcare app, marketplace
- **Showcase reciprocity** — every app in the public Forge Showcase gets a free domain banner / homepage feature for 30 days; developers who showcase a Forge app get a steering-committee observer invite

### Phase 1.5: Launch Sequence (one-shot, M1 → public alpha)

The launch is a coordinated 7-day cascade, not a single drop. Each step is gated on the previous one landing well; we will postpone if signal is weak.

| Day | Channel | Asset |
|-----|---------|-------|
| T-30 | Waitlist | Single landing page with email capture, ICP statement, GIF demo, target: 1,000 signups |
| T-7 | Private beta | Email waitlist; collect 5 testimonials |
| Day 0 (Tue 8am ET) | Hacker News (Show HN) | "Show HN: Forge — the LLM-native framework that makes AI-generated code production-ready" |
| Day 0 (Tue 11am ET) | X/Twitter thread | Founder thread with 30-second screen recording; tag Cursor/Copilot/Anthropic |
| Day 1 | Product Hunt | Coordinated launch w/ first-100 supporters lined up |
| Day 2–3 | YouTube short + TikTok | 60-second "why your AI-generated app dies in production" demo |
| Day 3 | Lenny's Newsletter / TLDR / Bytes | Pitch as guest post or sponsored issue |
| Day 5 | Long-form post | Substack/Dev.to: "What we learned building the production layer for vibe coding" |
| Day 7 | Live build stream | 4-hour Twitch/YouTube live: SaaS from zero → deployed using Forge |

**Counter-positioning content (planned, not aggressive):** one honest comparison post per major adjacent tool (`create-t3-app`, NestJS, Wasp) — written without bashing, focused on *when to use which*. Builds credibility with skeptics.

### Phase 2: Community Growth (Post-v1.0)

**Ecosystem Building**
- Forge Certified Developer program — exam + badge
- Forge Certified Company — for enterprise adoption credibility
- Community grants — fund contributors building adapters and templates
- Annual Forge Summit — virtual conference

**Partnership Strategy**
- Supabase — official Forge adapter, co-marketing
- PlanetScale / Neon — database adapters
- DigitalOcean / Railway — one-click Forge deploy buttons
- Cursor / Windsurf — official "Forge mode" in IDE settings

### Phase 3: Commercialization — LONG-TERM OPTION ONLY

> **READ THIS FIRST:** This phase is documented for completeness but is **explicitly NOT part of the near-term plan**. Per §0, commercialization activates only when all gating conditions are met. Until then, Forge remains 100% community-driven, 100% open source, with sustainability funded by sponsorships and grants (§17).

**Activation gates (ALL must be true):**
- 10,000+ GitHub stars
- 500+ verified production apps
- 100+ external contributors
- Sustainable maintainer team via sponsorships
- Unsolicited enterprise demand for paid offerings

**If gates are not met, this section does not apply. Period.**

**Possible Future Pricing Tiers (illustrative only — subject to community input via RFC)**

```
Free (OSS forever):
├── Core framework (all features)
├── CLI tools
├── Community adapters
├── Basic documentation
└── Community Discord (free tier)

Forge Pro — $29/developer/month:
├── Forge Studio — GUI for schema design, migration management
├── AI Convention Checker — cloud-hosted lint with AI fix suggestions  
├── Team collaboration — shared context bundles, team conventions
├── Audit dashboard — visual audit log explorer
└── Priority support (48h SLA)

Forge Enterprise — $500/organization/month:
├── All Pro features
├── On-premise deployment of Forge Studio
├── Compliance module templates (SOC 2, HIPAA, PCI-DSS)
├── Custom convention rules with enforcement
├── SSO + SCIM provisioning
├── SLA support (4h response)
├── Security audit tooling
└── Training + certification for teams

Forge Cloud — usage-based:
├── Managed Forge app hosting
├── Automated migration execution
├── Zero-downtime deployment pipeline
├── Integrated monitoring dashboard
└── Disaster recovery snapshots
```

**Revenue Projections — DELIBERATELY OMITTED**

This spec intentionally does not include revenue projections. Setting revenue targets at this stage would distort decision-making toward monetization rather than community value. Forge will not optimize for revenue until §0 commercial activation gates are met, and even then, projections will be modeled with community input.

---

## 8. Open Questions to Resolve Before Building

The following decisions must be made before writing line one of code:

1. **Language scope:** TypeScript-only for v1.0, or Python in parallel?
   - Recommendation: TypeScript-only — largest vibe-coding market, fullstack, fastest to ship

2. **Framework adapter vs. standalone:** Layer on Next.js/Hono, or standalone runtime?
   - Recommendation: Adapter-first (Next.js 15 for v1.0) — reduces adoption friction; **ship a Hono reference app at v1.0** to defuse Vercel-lock-in perception (Solution Architect concern)

3. **Database opinion:** PostgreSQL-only or multi-DB from day one?
   - Recommendation: PostgreSQL-first with adapter API for others — avoids lowest-common-denominator design

4. **LLM instructions format:** GitHub Copilot `.instructions.md`, Cursor rules, or custom DSL?
   - Recommendation: GitHub Copilot format as primary, with adapters for Cursor/Windsurf

5. **Governance model:** Solo maintainer, foundation, or company-backed OSS?
   - Recommendation: Solo → small core team → foundation once community is established

6. **Monorepo structure:** Single repo with packages, or separate repos?
   - Recommendation: pnpm monorepo (`forge-cli`, `forge-core`, `forge-adapters/*`)

7. **Domain for initial domain modules:** Which enterprise domains get first-class modules?
   - Recommendation: Auth, Tenancy, Billing, Audit — covers 80% of SaaS use cases

8. **Self-healing default mode:** Does `forge fix` / `@forge/healer` apply changes automatically or always open a PR?
   - Recommendation: **PR-only by default; no auto-merge ever**. Auto-apply is opt-in per repo and refuses to run on protected branches. (Security concern — LLM-driven code mutation without HITL is an RCE class vulnerability.)

9. **Key management for field-level encryption:** Which KMS does the v1.0 "high-trust" template ship against?
   - Recommendation: Envelope encryption with adapter pattern — native AWS KMS / GCP KMS / HashiCorp Vault adapters at v1.0; BYOK supported. Key rotation is a first-class CLI command.

10. **LLM eval determinism:** What does "reproducible eval" mean operationally?
    - Recommendation: pin model + version + temperature=0 + tool schemas + RAG seed + embedding model+version. Eval harness records all of these in the result manifest; CI fails on drift.

11. **"Built with Forge" badge:** Default opt-in or opt-out?
    - Recommendation: **Opt-in with a delightful prompt at `forge new`**. Vercel's opt-out attribution caused public backlash; we will not repeat that mistake.

12. **Telemetry collection:** Does the CLI collect anonymous usage data?
    - Recommendation: **Opt-in only, with explicit prompt on first run**, transparent payload schema published, single-flag global disable, never collected from CI environments.

13. **AI-tool instructions — single format or multi-tool?**
    - Recommendation: **Multi-tool from day one.** Every project ships `AGENTS.md`, `CLAUDE.md`, `.cursorrules`, `.windsurfrules`, and `.forge/instructions/*.instructions.md` (Copilot format) generated from a single source of truth (`forge generate ai-context`). Picking one tool would alienate two-thirds of the ICP overnight.

14. **Incremental adoption path — `forge adopt` in M0 or M2?**
    - Recommendation: **MVP `forge adopt` ships in M1.** Greenfield-only is a fatal limitation for Priya (P2). Initial scope: detect framework (Next.js/Hono/Fastify) + offer to add one Forge primitive (auth, audit, or RLS) at a time, non-destructively.

15. **Eject path — do we support `forge eject`?**
    - Recommendation: **Yes, from day one.** Without an eject command, "convention over configuration" feels like a cage (lesson from create-react-app). `forge eject` inlines all framework code into the project and removes the dependency. Once ejected, you are on your own — documented clearly.

16. **Frontend scaffolding — in scope or out?**
    - Recommendation: **Data-layer scaffolding (typed API client, React Query hooks, Zod-validated form components) is in scope; UI library choice stays unopinionated.** Without this, Riley (P5) keeps using create-t3-app.

17. **Offline / air-gapped support — nice-to-have or hard requirement?**
    - Recommendation: **Hard requirement for core features.** Auth, RLS, migrations, audit, generators, lint must work with zero outbound network calls. LLM features degrade gracefully + support local providers (Ollama, vLLM) as first-class adapters. Required for Maya (P6) + any regulated buyer.

18. **Package manager lock-in — pnpm only, or all four?**
    - Recommendation: **Generated apps work with npm/yarn/pnpm/bun; CI tests all four.** Internal monorepo uses pnpm but that's an implementation detail, not a user-facing constraint.

19. **`forge explain` — a real command or marketing fluff?**
    - Recommendation: **A real command, shipped in M0.** Critical for trust on first run: `forge explain <path>` tells the developer what each scaffolded file does and why it exists. This is the antidote to "47 files I didn't write, I trust nothing."

20. **`forge scan` auto-apply default — never, only `high` confidence, or fully opt-in?**
    - Recommendation: **Never auto-apply by default.** `--apply` opens a PR (consistent with §8 Q8). Only `high` confidence findings are eligible for `--apply`; `medium`/`low` require `--force` plus an explicit per-scanner flag in `forge.config.ts`. CI annotates the PR; humans merge.

21. **Continuous learning — local-only, federated opt-in, or off by default?**
    - Recommendation: **Local-only by default; federated sharing strictly opt-in with anonymized counts only (no code, no PII, no identifiers).** `forge teach` and `.forge/learned/` work fully offline. Federated sharing requires `forge learn share enable` plus a one-time review of the exact payload schema. Disable with one flag; revoke at any time. Honors §8 Q12 + §8 Q17.

22. **Scanner customization — can developers disable or tune scanners per project/module?**
    - Recommendation: **Yes, mirroring the §11.1.2 escape-hatch promise.** Three layers of override: (a) `forge.config.ts` sets per-scanner severity thresholds and disables families; (b) inline `// forge-disable-next-scan: <rule-id> — <reason>` requires a non-empty reason and is itself surfaced in `forge scan dx` reports; (c) `.forge/scan-ignore.yml` for path-globbed exclusions. CI annotates every disable with the reason so suppression is visible, never silent.

---

## 9. Name & Branding Options

| Name | Domain | Tagline | Notes |
|------|--------|---------|-------|
| **Forge** | forge.dev | From vibe to production | Clean, strong, familiar metaphor |
| **Anvil** | anvil.sh | Where ideas become products | Less known |
| **Scaffold** | scaffold.dev | The foundation your app deserves | Descriptive but generic |
| **Bedrock** | bedrock.dev | Enterprise foundations for everyone | Conflicts with AWS Bedrock |
| **Keel** | keel.dev | Keep your app from capsizing | Nautical metaphor, memorable |

**Recommendation:** **Forge** — the metalworking metaphor (raw material → shaped product) maps perfectly to "vibe → production". Short, memorable, available as an npm package concept.

---

## 10. Immediate Next Steps (Sprint 0)

1. **Reserve GitHub organization** and repo (`forgeframework/forge`)
2. **Write RFC-001** — publish as GitHub Discussion to gather early community feedback
3. **Validate CLI DX** — manually run through the target UX: `forge new`, `forge generate`, `forge migrate`
4. **Competitive teardown** — install and build with create-t3-app, Wasp, RedwoodJS; document gaps
5. **Identify 10 alpha users** — solo founders or small teams willing to build with early versions
6. **Draft .forge/instructions/ spec** — this is the feature that differentiates Forge from all competitors

---

## 11. Design Phase (Pre-Architecture)

Before writing any framework code, the following design artifacts must be produced and reviewed publicly via RFC:

### 11.1 Design Artifacts

| Artifact | Owner | Purpose |
|----------|-------|---------|
| **Product Vision Document** | Founder | One-page articulation of the "why" — circulated to all early contributors |
| **Persona Definitions** | Founder + DevRel | 3 primary personas (drafted below in §11.1.1; refined via interviews in M0) |
| **Job-to-be-Done Map** | Founder | What "job" does each persona hire Forge to do? |
| **User Journey Maps** | DevRel | First 5 minutes, first hour, first week, first production deploy |
| **Information Architecture** | DevRel + Founder | Docs structure, CLI command tree, configuration surface |
| **Visual Identity** | Designer | Logo, color palette, typography, README aesthetic |
| **Naming Conventions** | Architect | File names, module names, command names — locked before code starts |

### 11.1.1 Initial Persona Drafts (to be validated, not assumed)

**P1 — "Sam, the Solo Founder"** (primary ICP)
- Background: ex-engineer, building an AI-flavored SaaS solo, 2–6 hours of focused coding per day
- Stack: Next.js + Supabase + Cursor; uses Claude/GPT for 70%+ of code
- JTBD: *"Help me ship a paid SaaS that won't embarrass me when my first 10 customers push it."*
- Pain quote: *"I've shipped three prototypes this year. Two never got past auth + multi-tenancy. The third leaked a customer's data because RLS wasn't right and I didn't know."*
- Wins with Forge when: `forge new` → paid customer in <14 days without writing his own auth/billing/tenancy.

**P2 — "Priya, the Small-Team Lead"** (secondary ICP)
- Background: tech lead at a 5–20-person startup, ships 3–5 internal tools / customer apps per quarter
- Stack: mixed; team uses Cursor + Copilot inconsistently
- JTBD: *"Standardize how my team builds, so juniors + AI produce code I'm willing to merge without rewriting."*
- Pain quote: *"Every PR from my AI-using teammates needs the same 8 corrections. I want the framework to enforce what I keep typing in review."*
- Wins with Forge when: `forge lint` + `.forge/instructions/` cut PR review time in half.

**P3 — "Marcus, the Enterprise Architect"** (tertiary ICP — explicitly NOT v1.0 target)
- Background: staff engineer at a regulated company evaluating frameworks for a 24-month modernization
- JTBD: *"Find a framework I can defend in front of security, audit, and procurement."*
- Pain quote: *"I love what Forge promises but I can't bet a 7-figure roadmap on something with no SOC 2, no LTS, no foundation behind it."*
- Why we name him: he is the v3.0+ persona. Designing with him in mind from day one prevents architectural debt that locks him out later. We do not market to him until M3+.

**P4 — "Jordan, the Agent-First Builder"** (rapidly growing primary ICP for 2026+)
- Background: PM-turned-builder; doesn't write code by hand; drives Devin / Claude Code / Cursor agent / OpenHands
- Stack: whatever the agent picks; reviews PRs, doesn't author code
- JTBD: *"Give my agent a framework it can drive end-to-end without me babysitting every step."*
- Pain quote: *"My agent generated 200 lines, half of them wrong, and I can't tell which half. I need a framework that catches the agent's mistakes before they reach me."*
- Wins with Forge when: agent reads `AGENTS.md` + `.forge/instructions/`, produces code that passes `forge lint` + `forge review` on the first try, and the framework's CLI returns machine-parseable `--json` for every command.

**P5 — "Riley, the Frontend-Heavy Vibe-Coder"** (large under-served slice of the ICP)
- Background: design-engineer; 80% of value is the UI; vibe-codes mostly with v0/Bolt/Cursor for components
- Stack: Next.js + shadcn + Tailwind + Supabase
- JTBD: *"Generate the boring backend stuff for me so I can focus on the UI."*
- Pain quote: *"I love `create-t3-app` because it gives me a typed API client + React hooks for free. Forge's spec is all about audit logs and reconciliation — cool, but where's my form generator?"*
- Wins with Forge when: `forge generate module` produces typed API client + React Query hooks + Zod-validated form components alongside the backend module.

**P6 — "Maya, the Air-Gapped / Regulated-Env Dev"** (small but strategic)
- Background: builds in environments with no outbound internet (defence, healthcare, on-prem fintech)
- JTBD: *"Use the framework without leaking a single byte to a hosted LLM."*
- Pain quote: *"Half the spec assumes an OpenAI API key. We don't have one. We can't get one."*
- Wins with Forge when: every LLM-powered command has an offline mode (local model adapter, deterministic-fallback rule-based mode, or graceful degradation with a clear message); core framework features (auth, RLS, migrations, audit) work with zero LLM calls.

### 11.1.2 The Developer Promise (DX commitments owed to every persona)

Forge is built *for* developers, not *at* developers. The single guiding goal across every commitment below is the same one stated in the document header: ***ship fast with high quality, via vibe-coding.*** No commitment is allowed to lower the quality floor; no commitment is allowed to slow the vibe-coder down. These ten DX commitments are non-negotiable:

1. **Nothing magical.** Every generated file is human-readable, every command can be explained in one sentence, every default has a documented reason. `forge explain <file>` tells you what each scaffolded file does and why it exists.
2. **You own the code.** No proprietary runtime, no hidden compilation step, no callback to a Forge service required to run your app. `forge eject` removes the framework entirely with a single command and leaves a working app behind.
3. **Escape hatches everywhere.** Every convention has a per-module override; every generator supports `--no-template` to give you a blank file with the right name; every linter rule supports inline `// forge-disable-next-line: rule-name` with a required justification comment.
4. **Agent-readable by default.** Every CLI command supports `--json`; every error includes a stable `error_code` field; every doc page has a stable anchor; the project ships an `AGENTS.md` + `CLAUDE.md` + `.cursorrules` + `.forge/instructions/` (plural, not singular) so any AI agent or human can drive the framework on day one.
5. **Tests appear by accident.** `forge generate <anything>` always emits the test file alongside the code, prefilled with the happy path + boundary cases. Removing tests is a deliberate `--no-test` flag, not a default. *And for any non-trivial change, the single command `forge ship <feature>` (§4) emits the failing tests **before** any feature code is written — the framework will not let you ship code that wasn't preceded by a red test.*
6. **Adopt incrementally.** `forge adopt` runs in an existing Next.js / Hono / Fastify project and offers to add Forge primitives one at a time. You never have to greenfield-rewrite to try Forge.
7. **Offline-capable.** Auth, RLS, migrations, audit, generators, and lint all work with zero outbound network calls. LLM-powered features (`forge fix`, `forge review`, `forge ask`) degrade gracefully when no model is configured, and support local providers (Ollama, vLLM) as first-class adapters.
8. **Package manager agnostic.** pnpm is the recommended monorepo tool internally, but generated apps work with npm, yarn, pnpm, and bun. We test all four in CI.
9. **Your AI tool, not ours.** Forge ships first-class instructions for GitHub Copilot, Claude Code, Cursor, Windsurf, and Aider. We do not pick favorites; we adapt to the developer's existing tool.
10. **Frontend is not an afterthought.** Every generated module produces matching typed API client + React Query hooks + Zod-validated form components. UI primitives stay unopinionated, but data-layer scaffolding for the UI is first-class.

> **The single rule that overrides everything else:** *if a developer has to fight the framework to do the obvious thing, the framework is wrong, not the developer.*

### 11.2 Design Principles (locked before architecture)

1. **Boring by default** — every default choice should be the safe, well-understood option
2. **One way to do it (with an escape hatch)** — the default path is single and well-lit, but every convention has a documented override; LLM context stays small while developers stay free
3. **Errors must teach** — every error message includes the cause, the fix, a stable `error_code`, and a docs link
4. **Convention names are domain language** — `workspace`, `audit_log`, `tenant_id` — not framework jargon
5. **Generated code is human-readable** — no clever metaprogramming that LLMs (or humans) cannot reason about
6. **Configuration is colocated** — module config lives with the module, not in a giant root file
7. **Backwards compatibility is a feature** — breaking changes are paid for in community trust
8. **Agent-readable by default** — every CLI command supports `--json`; every error has a stable code; every doc has a stable anchor
9. **Tests are emitted, not requested** — generators always produce the test file alongside the code; opting out is the deliberate path
10. **Offline-first for core; LLM-augmented for delight** — the framework must be useful with zero LLM calls; LLM features add power, never gate it
11. **Scan over scold, fix over flag, learn over lecture** — when the framework detects an issue it proposes a concrete diff (not a stern message); when it sees the developer reject the proposal repeatedly it updates its own conventions (not its conviction). The framework is a *student of the codebase*, not a lecturer.
12. **Spec before test, test before code, task before commit — enforced by one command** — the framework prescribes the *order* of work, not just the tools, and ships that order as a single orchestrating verb: `forge ship <feature>`. Vibe-coding without a spec is gambling; a spec without failing tests is fiction; tests without task-sized scopes overwhelm the LLM. `forge ship` (§4 LLM-Native Authoring) is the single defined way to ship a non-trivial change — one command, four checkpoints, no shortcuts. Skipping a checkpoint requires an explicit, logged `--skip-checkpoint=...` flag, not silence.

### 11.3 Design Validation Method

- **Wizard-of-Oz testing** — manually simulate the framework's output for 5 sample apps before any code is written
- **README-first development** — README is written and reviewed before implementation begins
- **CLI dry-run** — every CLI command is mocked and reviewed for ergonomics before implementation
- **5 real developers** review the design package and complete a usability interview

---

## 12. High-Trust / Regulated-Industries Module ("Financial-Grade for Everyone")

This is Forge's strategic differentiator — but it is **not financial-only**. It is *financial-grade engineering applied broadly*. The same primitives serve fintech, healthcare, govtech, education, marketplace, identity, supply chain, energy, legal-tech, and any app where correctness, auditability, and trust matter more than ship-fast-break-things.

> **Framing:** Forge offers "financial-grade by default" as a *quality bar*, not a *vertical*. If your app handles money, health records, identity, contracts, votes, energy meters, or anything a regulator or auditor cares about — these primitives are for you. Everyone else still benefits from the rigor (better debuggability, cleaner audits, fewer 3am incidents).

**Where these primitives apply (illustrative, not exhaustive):**

| Domain | Why these primitives matter |
|--------|----------------------------|
| Fintech / banking / payments | Regulator-mandated; money type, audit chain, reconciliation are non-negotiable |
| Healthcare / health-tech | PHI handling, HIPAA, immutable medical records, consent tracking |
| Govtech / civic | Auditability, data residency, accessibility, evidence preservation |
| Identity / KYC / AML | Field-level encryption, lineage, right-to-erasure, evidence packs |
| Marketplaces & e-commerce | Double-entry ledger for payouts, idempotent orders, dispute trails |
| HR / payroll | Sensitive PII, audit, time-machine queries for compliance |
| Legal-tech / contracts | Immutability, hash-chained provenance, e-signature audit |
| Education / EdTech | Student PII (FERPA), assessment integrity, grade audit trails |
| Supply chain / logistics | Provenance, tamper-evident records, multi-party reconciliation |
| Energy / IoT / metering | Idempotent ingestion, reconciliation, regulator-grade audit |
| Any LLM/agent app (§21) | Outcome ledger, attribution chain, agent action audit, kill switch |
| **Any SaaS that wants fewer outages** | Idempotency, outbox, structured audit make incidents tractable |

### 12.1 Core Capabilities (Domain-Agnostic)

These capabilities are written in domain-neutral terms. The fintech examples are illustrative; the same primitives ship for every regulated/high-trust domain.

#### Transaction & Operation Integrity
- **Strict ACID guarantees** — sensitive operations enforce serializable isolation by default; opt-out is explicit and audited
- **Saga pattern primitives** — for distributed transactions across services (any domain: order fulfillment, claims processing, multi-step underwriting, document workflows)
- **Idempotency keys built-in** — every mutation endpoint requires an idempotency token; safe retries by construction
- **Outbox pattern** — atomic write to DB + event queue for reliable event publishing (works for payments, notifications, webhooks, agent events)
- **Typed quantity primitives** — `Money` (minor units + currency), `Quantity` (value + unit), `Duration`, `Percentage`; never floating-point arithmetic on values that matter; enforced by linter
- **Workflow / state machine primitives** — typed state transitions with audit; replaces ad-hoc status fields

#### Audit & Compliance
- **Immutable audit log** — append-only, hash-chained (each entry includes previous entry's hash)
- **Audit completeness test** — CI fails if any mutation handler skips audit emission
- **Time-machine queries** — view system state at any point in past (event-sourcing optional adapter)
- **Data lineage tracking** — every derived field knows its source and transformation
- **Evidence export** — `forge compliance export` generates auditor-ready evidence packs

#### Encryption & Data Protection
- **Field-level encryption** — annotate sensitive columns; framework handles key rotation
- **Encryption-at-rest enforcement** — DB connection refuses unencrypted volumes
- **PII tagging** — every PII field is tagged; access requires explicit grant + logged
- **Right-to-erasure** — `forge gdpr erase <user_id>` cascades through all PII fields
- **Data residency adapter** — route writes to region-specific DB clusters

#### Regulatory Compliance Templates
- **SOC 2 Type II** — controls mapping, evidence collection, audit log format
- **PCI-DSS** — cardholder data isolation, tokenization adapter, network segmentation patterns
- **HIPAA** — PHI tagging, BAA-ready audit logs, access controls
- **SOX** — change management workflow, segregation of duties enforcement
- **GLBA** — financial data safeguarding patterns
- **GDPR / CCPA** — consent management, data subject rights workflows
- **ISO 27001** — security control templates

#### Financial-Grade Operational Patterns (apply to any domain)
- **Reconciliation framework** — scheduled jobs that compare expected vs. actual state (ledger balances, inventory, message delivery, agent outcomes, anything countable)
- **Double-entry ledger** — built-in module for any app handling balances (money, credits, points, tokens, hours, energy units, inventory)
- **Webhook signature verification** — enforced for all webhook endpoints (Stripe, Plaid, GitHub, Slack, social platforms, custom)
- **Replay attack protection** — timestamp + nonce validation built into the webhook adapter
- **Rate limiting per tenant** — not just per IP; prevents tenant DoS in any multi-tenant app
- **SLO & error-budget primitives** — declare SLOs in code; framework tracks burn-rate and triggers alerts/auto-rollback advisor (§4 LLM layer)

### 12.2 Reference Architectures (Multiple Templates)

Forge ships templates for several high-trust domains. Each is a thin layer over the same Core Capabilities; pick the closest match and customize.

```bash
forge new my-app --template fintech-grade        # Banking/payments/lending
forge new my-app --template healthcare-grade     # HIPAA, PHI, BAA-ready audit
forge new my-app --template govtech-grade        # Auditability, residency, accessibility
forge new my-app --template marketplace-grade    # Double-entry ledger for payouts, dispute trails
forge new my-app --template identity-grade       # KYC/AML, evidence packs, right-to-erasure
forge new my-app --template agent-grade          # Multi-agent app w/ outcome ledger + kill switch (§21)
forge new my-app --template high-trust           # Generic: audit + idempotency + reconciliation + encryption
```

Example (fintech-grade) generates:
```
├── Strict ACID + idempotency on all transaction endpoints
├── Double-entry ledger module pre-wired
├── Hash-chained audit log
├── PCI-DSS compliant card tokenization adapter
├── Field-level encryption for PII (SSN, account numbers)
├── Reconciliation jobs (hourly + daily)
├── Webhook handlers with replay protection
├── Compliance evidence pack generator
└── Penetration test runner (forge pentest)
```

The other templates differ in *which* compliance map ships pre-wired, *which* PII tags are pre-defined, and *which* reference modules are scaffolded — but the underlying primitives are identical.

### 12.3 Why This Matters for the Community

The high-trust angle is **first and foremost a community wedge**, not a commercial one:
- High-trust domains (fintech, health, gov, identity, marketplace, supply chain, energy) are underserved by OSS frameworks — they need primitives they can audit and trust
- Compliance templates and high-trust primitives are a **gift to the community**: shipping them as Apache 2.0 OSS lowers the barrier dramatically for startups in any regulated/sensitive domain
- This builds deep loyalty and credibility in high-stakes domains where Forge's reputation will compound
- Even "normal" SaaS apps benefit — idempotency, audit, reconciliation, and structured incident response make any production system more debuggable and trustworthy
- Whether or not commercialization ever happens, these modules ship as fully open source under Apache 2.0

*(A possible long-term commercial angle exists here per §15, but it is explicitly NOT the reason these modules are built. They are built because the community needs them.)*

---

## 13. Test Strategy & Quality Gates

> **Closed-loop testing.** Tests are not just authored — they grow automatically. Every accepted `forge scan` finding becomes a prompt-eval scenario; every closed bug triggers `forge generate test --from-bug`; every revert mined by the continuous learning loop (§4) seeds a new regression. The test suite *gets denser as the project ages*, not staler.

### 13.1 The Forge Test Pyramid

```
                    ┌─────────────────┐
                    │  E2E (5%)       │   Playwright, full user journeys
                    └─────────────────┘
                ┌─────────────────────────┐
                │  Integration (20%)      │   Real DB, real adapters, RLS tests
                └─────────────────────────┘
        ┌─────────────────────────────────────┐
        │  Contract (15%)                     │   API contracts, schema alignment
        └─────────────────────────────────────┘
    ┌─────────────────────────────────────────────┐
    │  Unit (50%)                                 │   Pure functions, business logic
    └─────────────────────────────────────────────┘
┌─────────────────────────────────────────────────────┐
│  Convention / Lint (10%)                            │   Forge convention enforcement
└─────────────────────────────────────────────────────┘
```

### 13.2 Mandatory Test Categories (auto-generated per module)

For every Forge module, the framework auto-generates test scaffolding for:

1. **Happy path** — intended behavior succeeds
2. **Boundary cases** — empty/null/zero/max/min/off-by-one
3. **Negative cases** — invalid input, unauthorized, wrong tenant
4. **Idempotency / replay** — same operation twice, webhook redelivery
5. **Concurrency / race** — two writers, out-of-order arrivals
6. **Cross-tenant authz** — tenant A cannot affect tenant B (RLS enforcement)
7. **Backward-compatibility** — regression tests for fixed bugs
8. **Data accuracy** — real row inserts → query back → assert correctness
9. **False-positive guard** — proves new check is narrowly scoped
10. **Revert experiment** — test fails on pre-fix code, passes on post-fix code

### 13.3 Quality Gates (CI-enforced)

Pull requests are blocked unless:

| Gate | Threshold | Enforcement |
|------|-----------|-------------|
| Unit test coverage | ≥ 80% statements, ≥ 70% branches | Vitest + c8 |
| Integration tests | All passing | Vitest with real DB |
| RLS boundary tests | All passing | Custom test runner |
| Schema alignment | TypeScript ↔ DB types match | `forge check schema` |
| Convention lint | Zero violations | `forge lint` |
| Security scan | No high/critical CVEs | npm audit + Snyk |
| Migration safety | Dry-run on staging snapshot | `forge migrate --dry-run` |
| Audit completeness | All mutations emit audit | `forge audit verify` |
| Performance budget | No regression > 10% | Benchmark suite |
| Documentation | All public APIs documented | TSDoc coverage check |

### 13.4 Pre-Push Gate

Locally enforced via `pnpm prepush`:
- Type check, lint, unit tests, integration tests, RLS tests
- **Never push without green local tests** — institutional rule

---

## 14. Deployment Strategy

### 14.1 Deployment Model: Three-Environment Promotion

```
local → staging → production
  ↑        ↑           ↑
  │        │           └── Real users, blue/green or canary
  │        └────────────── Mirror of prod schema, smoke tests run here
  └─────────────────────── Developer machine, ephemeral DB
```

### 14.2 Migration Safety

Migrations are the highest-risk deployment step. Forge enforces:

- **Versioned & timestamped** — `YYYYMMDDHHMMSS_description.sql`
- **Reversible by default** — every migration has an `up` and `down`
- **Dry-run on staging snapshot** — CI blocks if migration fails on a recent prod snapshot
- **Two-phase deploys** — schema additive → code deploy → schema cleanup (separate releases)
- **Online migration linter** — flags `DROP COLUMN`, `ALTER TYPE`, long locks
- **Migration repair tool** — `forge migrate repair` for fixing drift between environments

### 14.3 Deployment Patterns Supported

| Pattern | Use Case | Adapter |
|---------|----------|---------|
| **Rolling** | Default for stateless apps | All cloud adapters |
| **Blue/Green** | Critical apps, fast rollback | DO, AWS, Vercel |
| **Canary** | Risk-averse, gradual rollout | AWS, GCP |
| **Shadow** | Validate new code against prod traffic | Custom |

### 14.4 Rollback Playbook (Auto-Generated)

Every Forge app ships with a `ROLLBACK.md` runbook:
1. **Detect** — health check failures, error rate spikes, alert thresholds
2. **Decide** — rollback criteria checklist (code-only? DB-impacting? data-loss risk?)
3. **Execute** — `forge rollback --to <release-id>` or manual revert procedure
4. **Verify** — post-rollback smoke tests, audit log inspection
5. **Postmortem** — blameless template included

### 14.5 Observability at Deployment Time

- Deployment events tagged with commit SHA + author
- Synthetic smoke tests run post-deploy automatically
- Auto-rollback if synthetic tests fail within 5 minutes
- Slack/Discord/PagerDuty notifications via adapter

---

## 15. Licensing Strategy

Licensing is one of the most consequential and irreversible decisions for an OSS framework with commercial intent.

### 15.1 Recommended Approach: Apache 2.0 Core + Commercial Add-ons

**Core Framework: Apache License 2.0**

Why Apache 2.0:
- Permissive — maximum adoption, including by enterprises
- Includes patent grant — protects users from contributor patent claims
- Compatible with commercial use — companies can use Forge in proprietary products
- Industry standard for enterprise frameworks (Spring, Kafka, Kubernetes)
- Allows Forge Inc. to sell hosted/managed versions

**Why NOT MIT:**
- No explicit patent grant — risk for enterprise adopters

**Why NOT AGPL:**
- Hostile to enterprise adoption (forces source disclosure on network use)
- Would kill the SaaS adoption funnel

**Why NOT BSL (Business Source License):**
- Damages "open source" credibility
- Reserved for products where the open core IS the commercial product (e.g., CockroachDB)
- Forge's commercial value is in tooling/services around the framework, not the framework itself

### 15.2 Commercial Components: Polyform Shield License or Proprietary

Forge Studio, Compliance modules, AI Convention Checker (cloud) ship under:
- **Polyform Shield 1.0.0** for some tooling — prevents direct competitors from offering it as a service
- **Proprietary commercial license** for SaaS components

### 15.3 Contributor License Agreement (CLA)

- Required for all contributors
- DCO (Developer Certificate of Origin) sign-off as minimum
- CLA assigns relicensing rights to Forge Inc. (enables commercial dual-licensing if needed)
- **Transparent rationale** — published prominently in CONTRIBUTING.md

### 15.4 Trademark Policy

- "Forge" and the Forge logo are trademarks of Forge Inc.
- Community can use freely for adapters/templates
- Cannot be used in product names that imply official endorsement
- Trademark policy modeled on Linux Foundation projects

---

## 16. Community Governance Model

> **Scanner & learning-loop governance.** Scanner plugins (§4, §20) are community-curated like adapters: published to the Forge Registry, audited for the SemVer + Stable-Anchors contract, and verified nightly against a Forge reference suite. The federated convention-sharing aggregator (§4 learning loop) is operated under the same trust constraints as the Registry — payload schema is public, ingestion is reviewable, and any contributor can audit what aggregate counts they have shared.

### 16.1 Phased Governance

Governance evolves with community size:

**Stage 1 (0–1K stars): BDFL (Benevolent Dictator)**
- Founder makes all final decisions
- RFC process for transparency, but no voting
- Optimizes for speed and design coherence

**Stage 2 (1K–10K stars): Core Maintainer Team**
- 3–5 trusted contributors granted commit access
- Founder retains tiebreaker on architectural decisions
- RFC process formalized; lazy consensus for non-controversial RFCs

**Stage 3 (10K+ stars): Steering Committee**
- 7-member elected steering committee (1-year terms)
- Working groups for major areas (Core, Adapters, Docs, Security)
- Founder steps back to "BDFL emeritus" — emergency override only
- Consider moving to a foundation (CNCF / OpenJS / Linux Foundation)

### 16.2 RFC Process

All non-trivial changes require an RFC:
- Filed as a Markdown PR to `forgeframework/rfcs` repo
- Minimum 14-day comment period for major RFCs
- Lazy consensus: no objections after period = accepted
- Architectural RFCs require core maintainer approval

### 16.3 Code of Conduct & Moderation

- Adopt Contributor Covenant 2.1
- Dedicated moderation team (3+ members, separate from technical leadership)
- Public incident report log (anonymized)
- Clear escalation path: report → review → action → appeal

### 16.4 Decision-Making Principles

- **Default to public** — discussions in GitHub Issues/Discussions, not DMs
- **Document the why** — every accepted RFC includes rationale and rejected alternatives
- **No surprise breaks** — breaking changes require RFC with migration path
- **Diversity in maintainers** — actively recruit across geographies, backgrounds, experience levels

### 16.5 Community Contribution Standards (Core Modules)

> *Forge is a community-first project (§0) with a quality-floor obligation to its users. These standards are the bridge: they make it **possible** for any developer to contribute to core modules, while making it **impossible** for a contribution to lower the quality floor that the framework promises.* If a contribution cannot meet these standards, it should ship as a **plugin** (§20) — the plugin surface exists precisely to keep the contribution path open without compromising the core.

#### 16.5.1 What is "core"?

Three concentric trust tiers; standards rise as the tier moves inward:

| Tier | Examples | Who can merge | Standards apply |
|------|----------|---------------|-----------------|
| **T3 — Plugins (community)** | Third-party scanners, adapters, generators in the Registry | Plugin author + Registry audit | §16.5.4 (Registry baseline) |
| **T2 — Adapters & Recipes (contrib)** | Cloud adapters (AWS/GCP/etc.), provider adapters (LLM, queue, storage), starter templates, official example apps | Any 2 maintainers from the relevant working group | §16.5.4 + §16.5.5 |
| **T1 — Core modules** | Foundation Layer · Security Layer · LLM-Native Layer · `ship` workflow · Scan engine · Learning loop · Migration runner · CLI grammar · Public API surface | RFC + 2 core maintainers + 1 security reviewer if §16.5.6 triggers | All of §16.5 |

> **Default routing:** new contributions start at T3 (plugin). Promotion to T2 or T1 requires a graduation RFC documenting adoption, stability, and demand. *Trust is earned with green CI runs over time, not granted on the first PR.*

#### 16.5.2 The contribution flow (one path, well-lit)

```
Idea → Discussion → (RFC if T1/T2) → Branch → forge ship <change> → CI green → Review → Merge
                                              └─ same workflow contributors use for their own apps ─┘
```

**Forge eats its own dog food.** Every core-module change is itself shipped through `forge ship` (§4): the change *has a spec*, the change *has failing tests written before the code*, the change *has a task breakdown*. The PR template auto-fills from `.forge/specs/<change-name>/`. *A contributor who learns to contribute to Forge has, by construction, learned to use Forge.*

#### 16.5.3 Eligibility — who can contribute

- **Anyone, on day one, at T3 (plugins).** No CLA on first PR; no corporate gatekeeping. Sign the DCO (Developer Certificate of Origin) on commit; no CLA required for T3.
- **T2 (adapters, recipes):** ≥3 merged T3 contributions OR maintainer sponsorship. DCO on every commit.
- **T1 (core):** ≥5 merged T2 contributions, or RFC authorship of an accepted T1 change, or invited by a working-group lead. DCO + signed-commits required (`git commit -S`).
- **Maintainer status:** earned through sustained T2/T1 contribution over ≥6 months *and* demonstrated review quality (not just commit volume). Nominated by an existing maintainer; confirmed by lazy consensus of the working group.

> Anti-pattern explicitly rejected: drive-by "rewrite half of core in my favorite style" PRs. The standards below make these PRs immediately and visibly out of scope, without any reviewer needing to write an awkward "no thanks" comment.

#### 16.5.4 Universal contribution standards (apply to T1, T2, T3)

Every PR, regardless of tier, must satisfy these gates **before** a human review begins. CI enforces them; reviewers do not waste cycles on them.

| # | Gate | Enforced by |
|---|------|-------------|
| 1 | **Has a spec.** `.forge/specs/<change>/spec.md` describes intent, acceptance criteria, and which core principle the change serves. Trivial changes (typos, deps, log strings) use `forge ship --quick`, which auto-generates a one-line spec and the regression test. | `forge ship verify` blocks PR on missing spec |
| 2 | **Tests precede code.** Timestamp + git-history check: every test file's first commit is older than (or in the same commit as) the production code it covers. | `forge ship verify` |
| 3 | **All scans clean OR explicitly waived.** `forge scan all --since main` is green. Any waived finding is annotated with rationale + expiry date in `.forge/waivers/`. | CI gate |
| 4 | **Convention-aligned.** `forge lint` passes; new patterns require either an existing convention reference or a new convention proposal in the same PR. | CI gate |
| 5 | **Public-API delta is declared.** A `BREAKING.md` / `CHANGELOG.md` fragment lists added/removed/changed public surface, including CLI verbs, flags, error codes, and `--json` schemas. PRs with undeclared public-API changes are blocked. | CI diff against last release |
| 6 | **Token budget is observed.** Any LLM-touching change reports tokens-per-scenario delta from `forge eval`; regressions >10% require justification. | `forge eval` cost gate |
| 7 | **Docs honest.** `forge docs sync` produces no diff (i.e. docs already reflect the code). Adding a public verb without docs is blocked. | `forge docs heal` in CI |
| 8 | **DCO signed; commits signed where required.** Every commit has `Signed-off-by:`; T1 commits additionally have `git commit -S` signature. | DCO bot + branch protection |
| 9 | **Backward compatibility.** Behavior changes require a deprecation cycle (one minor with alias kept; remove in next major). New `error_code`s never reuse a retired number. | RFC + CI alias check |
| 10 | **No silent telemetry.** Any new outbound network call (telemetry, learning-loop share, eval upload) is opt-in, documented, and a single flag away from disabled. | Security review (§16.5.6) |
| 11 | **Repo hygiene clean.** `forge clean --check` exits zero. No unmanifested LLM scratch (`_*`, `patch_*`, `fix_*`, `*_output.*`, `*_SUMMARY*.md`, `scratch/**`, etc.) is added by the PR. New legitimate generated artefacts must be declared in `.forge/hygiene.yml` in the same PR. The PR does not weaken the framework-managed `.gitignore` block, does not shadow `.example`/`.template` negations, and does not introduce a tracked file matching the secret-file guard list. | CI gate (`forge clean --check`) |
| 12 | **Secrets clean.** `forge scan security --secrets --since main` (gitleaks under the hood, framework-managed `.gitleaks.toml`) is green on the PR diff. Allowlist additions require `description` + `path/regex` + `# review-by: YYYY-MM-DD`; expired allowlists fail the gate. Bypassing pre-commit secret scan requires an explicit `gitleaks-bypass: <reason>` commit-message token, which is surfaced in review. | CI gate (gitleaks) |

#### 16.5.5 Adapter & recipe standards (T2 additions)

Beyond §16.5.4:

- **Reference compliance suite passes.** Adapters implement a typed interface (e.g. `ILlmProvider`, `IQueue`, `IStorage`); a shared compliance suite proves the adapter behaves identically to the reference. Adapters that need to opt out of a capability declare it in `manifest.yml` (`unsupported: ["streaming"]`) — opting out is allowed; misrepresenting capabilities is not.
- **Two real-world consumers.** A new T2 adapter must show two production projects (or one production + one official example) that depend on it, otherwise it stays at T3.
- **Maintenance commitment.** A named maintainer (or maintainer-pair) takes responsibility for the adapter for ≥12 months. Unmaintained T2 adapters are auto-demoted to T3 with a quarterly review.

#### 16.5.6 Core-module standards (T1 additions)

Beyond §16.5.4 + §16.5.5, T1 changes additionally require:

- **Accepted RFC** in `forgeframework/rfcs` with ≥14-day comment period (§16.2). The RFC names the principle(s) (§11.2) and the goal (ultimate goal: ship-fast-with-high-quality) the change serves; if neither column is filled, the RFC is closed as out of scope.
- **Two core-maintainer approvals**, at least one from the working group that owns the touched module.
- **Security review trigger** (mandatory): a third reviewer from the Security working group is required if the PR touches **any** of:
  - `auth/`, `rls/`, `audit/`, `crypto/`, secrets handling, webhook signature validation
  - The migration runner, the scan engine, or the `--apply` code path
  - Anything that changes a default that affects multi-tenancy isolation
  - Any new outbound network egress
- **Migration path included.** Behavior changes ship with a codemod (`forge upgrade`) and a `BREAKING.md` entry, both in the same PR. RFCs without a migration path are not accepted.
- **Performance budget honored.** Cold-start, `forge new` time-to-running-app, `forge ship` round-trip on the reference app, and CI-suite wall time each have a published budget; regressions >5% are blocked or require an RFC waiver.
- **Learning-loop neutrality.** Core changes do not silently teach the framework new conventions; any change to default conventions or instructions must be a reviewable PR to `.forge/instructions/` defaults, not a behavior change in code.

#### 16.5.7 Review SLAs (the maintainer's promise back to contributors)

- **First maintainer response:** 7 calendar days for T3, 5 for T2, 3 for T1. After SLA breach, the contributor may ping `@forge/triage` and the PR moves to the weekly triage call.
- **Time to merge or explicit decline:** 30 / 21 / 14 days for T3 / T2 / T1 from "ready for review." A decline is always explained in writing and links to the principle / standard the PR fails.
- **Stale PRs:** auto-labelled `needs-rebase` after 30 days; auto-closed after 90 days of contributor inactivity (reopen-on-push allowed).
- **No silent rejection.** If a PR is closed without merge, the closing comment cites the specific standard it failed and (where possible) suggests the lower tier where it would belong (often: "ship as a plugin").

#### 16.5.8 Recognition & path-to-maintainer

Contribution is unpaid by default; recognition is the currency. Forge formalizes it:

- **CONTRIBUTORS.md** — every merged PR earns an entry; never deleted, even if the contributor leaves.
- **Working-group rosters** — public, with current focus areas; vacancies are visible.
- **Maintainer ladder:** Contributor → Reviewer (commit access to a single area) → Maintainer (commit access to a working group) → Core Maintainer (architectural authority). Each rung documents what was earned, what is now expected, and how to step down without stigma.
- **Sponsorship channels** (§17): GitHub Sponsors / OpenCollective directed at named maintainers and working groups, with transparent split rules. Funding never changes review priority.

#### 16.5.9 The contributor's escape hatch — "ship as a plugin first"

The single most important standard is also the kindest one: **if your change is rejected for core, it can almost always ship as a plugin.** The plugin API (§20) is intentionally broad enough to host scanners, generators, adapters, learning-loop hooks, CLI sub-commands, and instructions packs. This means:

- Contributors are never *blocked* — only *routed*.
- Core stays small, opinionated, and stable.
- Successful plugins create the demand signal that justifies T3 → T2 → T1 promotion via RFC.
- The framework *learns from its own ecosystem* about which conventions deserve to become core defaults — closing the loop with the §4 learning-loop philosophy.

> **Principle:** *the standards exist to protect users, not to gatekeep contributors.* Every "no" to core is paired with a "yes" to a plugin path. Every plugin that earns adoption earns a promotion path. The community owns the core, eventually — by demonstrating, contribution by contribution, that they understand the quality floor it defends.

---

## 17. Funding & Sustainability

OSS frameworks die more often from maintainer burnout than from technical failure.

### 17.1 Phase 0 (Months 1–6): Founder Bootstrap

- Founder self-funds — 6 months of focused work
- Estimated cost: founder's salary equivalent + ~$2K/mo for tooling, hosting, design
- Goal: reach Phase 1 community traction milestones (1K stars, 10 production users)

### 17.2 Phase 1 (Months 6+): Sponsorship & Grants — Primary Funding Model

**This is the long-term funding model for Forge, not a transitional phase.** Sponsorships and grants are the intended sustainability mechanism for the entire community-building era (which has no fixed end date).

| Source | Estimated $ | Notes |
|--------|-------------|-------|
| GitHub Sponsors | $500–$5K/mo | Individual + small company sponsors |
| OpenCollective | $1K–$10K/mo | Transparent backer program |
| Sovereign Tech Fund | €50K–€200K/yr | German government OSS grant |
| NLnet | €5K–€50K | EU OSS infrastructure grant |
| AI Safety / OSS grants | Varies | Mozilla, OpenAI, Anthropic grants for AI-tooling OSS |
| Cloud credits | $10K–$100K | AWS, GCP, Azure startup programs |

**Sponsor tiers:**
- Individual: $5/mo (badge in README contributors)
- Backer: $50/mo (logo in BACKERS.md)
- Bronze: $250/mo (logo on website)
- Silver: $1K/mo (logo on homepage)
- Gold: $5K/mo (steering committee observer seat)

### 17.3 Phase 2: Commercial Revenue — LONG-TERM OPTION ONLY

Per §0, commercial revenue is **not a near-term funding strategy**. It activates only when §0 commercial gates are met. Until then, sponsorships and grants (§17.2) fund the project.

If and when commercial activation occurs, possible revenue streams (subject to community RFC) include training & certification, optional managed offerings, and enterprise support contracts. See §7 Phase 3 for illustrative tiers.

### 17.4 Venture Funding — STRONGLY DISCOURAGED

Venture funding is **explicitly NOT part of the Forge plan**. It carries unacceptable risks:
- VC pressure to monetize on VC timelines (3–5 years to liquidity event)
- Forces premature commercialization, breaking §0 commitment
- Track record: nearly every OSS framework that took VC eventually relicensed away from OSS-friendly licenses
- Founder loses control over OSS-vs-commercial trade-off decisions

**The Forge model is bootstrapped indefinitely.** Founder time + sponsorships + grants. If the project cannot sustain itself this way, it is acceptable to slow development pace rather than take VC.

### 17.5 Sustainability Safeguards

- **Maintainer burnout prevention** — vacation policy for paid maintainers, on-call rotation
- **Bus factor monitoring** — no single person owns critical paths
- **Financial transparency** — annual financial report published publicly
- **Mission lock** — articles of incorporation include OSS commitment

### 17.6 Zero-Funding Floor (the "if nothing comes through" plan)

Grants like Sovereign Tech Fund and NLnet have 6–9-month application cycles — they are funding *applications* in Phase 1, not Phase 1 *cash*. The plan must work even if no grant lands and no sponsor signs in year one.

**Zero-funding pace:** founder ships M0 in 6 months on personal savings, then drops to 1–2 days/week of Forge work while consulting to cover living costs. Under this scenario, M1 takes 18 months instead of 12. The project survives; the timeline stretches. **This is acceptable.** Forge's strategic advantage is patience — we never need to ship before we're ready.

---

## 18. Versioning & Stability Policy

Enterprise users will not adopt without explicit stability commitments.

### 18.1 Semantic Versioning (SemVer 2.0)

- **MAJOR** — breaking API changes (e.g., 1.x → 2.0)
- **MINOR** — new features, backwards-compatible (e.g., 1.0 → 1.1)
- **PATCH** — bug fixes, no API change (e.g., 1.0.0 → 1.0.1)

### 18.2 Stability Tiers

Each module is marked with a stability tier in its `forge.module.ts`:

| Tier | Meaning | Breaking Changes |
|------|---------|------------------|
| **Stable** | Production-ready | Only in MAJOR releases, with deprecation cycle |
| **Beta** | Feature-complete, may change | Allowed in MINOR releases with notice |
| **Alpha** | Early preview | Anything goes; opt-in via `experimental: true` |
| **Deprecated** | Will be removed | Removal scheduled 2 MAJOR versions out |

### 18.3 Deprecation Policy

- Deprecation announced at least 1 MAJOR version before removal
- Deprecation warnings printed at runtime AND build time
- Migration guide published with the deprecation
- Codemod provided where possible (`forge migrate-code`)

### 18.4 Long-Term Support (LTS)

- Every other MAJOR version is designated LTS
- LTS versions receive security patches for 24 months
- Enterprise tier gets LTS extension (36 months) + backported features

### 18.5 Release Cadence

- MAJOR: every 12–18 months
- MINOR: every 6–8 weeks
- PATCH: as needed, weekly batch for non-critical
- Security patches: emergency releases within 48h of disclosure

---

## 19. North Star Metric & Success Definition

> **Why this NSM:** Forge's ultimate goal is to enable developers to **ship fast with high quality via vibe-coding**. The NSM below directly measures both halves of that goal in one number: *fast* (the developer reached production), *high quality* (the app survived 30+ days), and *via vibe-coding* (the developer used an AI tool to write most of the code, captured in the qualifying-app survey §6). Speed without quality fails the NSM (apps die <30 days); quality without speed fails the NSM (developers never reach `forge deploy`). Only the combination scores.

### 19.1 North Star Metric (lagging)

**Number of Forge apps successfully running in production for 30+ consecutive days, built by external developers (not the core team).**

This single metric captures everything that matters for community-first success:
- **Adoption** (apps started by people other than us)
- **Quality** (apps survive production)
- **Persistence** (apps stay deployed, not abandoned)
- **Real-world value** (running 30+ days = solving a real problem)
- **Community ownership** (built by external devs, not by us)

**Notably absent: revenue.** Per §0, revenue is not a success metric for the community-building phase.

### 19.1.1 Leading Indicator (weekly check)

The NSM is lagging by 30+ days, so we also track one weekly leading indicator:

**Weekly count of `forge new` runs that reach `forge deploy` within 14 days, by non-core-team developers.**

If this number is rising week-over-week, the NSM will follow. If it is flat for 4 consecutive weeks, the onboarding funnel needs urgent investigation — not more features, not more marketing. The leading indicator is the founder's primary weekly dashboard.

### 19.2 Supporting Metrics

| Metric | Why It Matters | Target Y1 |
|--------|---------------|-----------|
| GitHub stars | Awareness | 5,000+ |
| Weekly active CLI users | Real engagement | 1,000+ |
| Production apps (verified) | North star | 100+ |
| Community contributors | Ecosystem health | 50+ |
| Community-built adapters | Extension flywheel | 20+ |
| Documentation NPS | User experience | 50+ |
| Time-to-first-deploy | Onboarding success | < 30 min |
| Convention violation rate | Framework quality | Decreasing trend |
| Scanner finding-to-fix-merge time (median) | Scan loop is *useful*, not noise | < 24h on opt-in projects |
| Scanner false-positive rate (per family) | Trust in `forge scan` | < 5% sustained |
| Projects with ≥1 promoted learned convention | Learning loop adoption | 30%+ of M2 projects |

### 19.3 Failure Criteria (Honest Self-Check)

The project should be reconsidered or pivoted if, by Month 18 (extended timeline reflects community-first patience):
- Fewer than 2,000 GitHub stars
- Fewer than 25 verified production apps built by external developers
- Fewer than 10 external contributors with merged PRs
- No active community discussion (Discord/Discussions averaging < 50 messages/week)

**Note:** "No paying customers" is explicitly NOT a failure criterion. Forge can succeed indefinitely as a pure community project funded by sponsorships, with no commercial revenue ever required.

### 19.4 Success Definition (3-Year Vision)

By 2029, Forge is:
- The default framework recommended by Cursor, Windsurf, and similar AI-native IDEs
- Powering 10,000+ production apps built primarily by external developers
- The reference framework for "production-grade vibe coding"
- A self-sustaining community with active maintainers across multiple time zones
- Funded sustainably through sponsorships and grants (commercial revenue optional, not required)
- Recognized in the same conversation as Spring, Django, Rails — for the AI era

**Explicitly NOT a success criterion:** revenue, ARR, paying customers, valuation. Those metrics belong to a different (potential, optional, far-future) chapter of the project.

---

## 20. Extensibility Architecture (Core Design Principle)

> **Inspiration:** Spring's success comes from Spring Boot starters and the broader ecosystem. React's dominance comes from its component model and the npm ecosystem around it. Rails is loved because of `gem install`. Forge follows this proven pattern: **a small, stable core surrounded by a vast extension ecosystem built by the community.**

### 20.1 Architectural Foundation: Small Core, Large Ecosystem

Forge's source code is organized so that the **core stays minimal and stable**, while everything else \u2014 even features the founder writes \u2014 is implemented as a plugin using the same public APIs available to community developers.

```
forge/
├── core/                          # Minimal, stable, rarely changes
│   ├── runtime/                   # Module loader, DI container, lifecycle
│   ├── extension-api/             # Public APIs for plugins
│   ├── schema-engine/             # Schema-first foundation
│   └── cli-kernel/                # CLI dispatcher (commands are plugins)
│
├── official-plugins/              # First-party plugins (use SAME APIs as community)
│   ├── @forge/auth                # Auth module — built as a plugin
│   ├── @forge/tenancy             # Multi-tenancy — built as a plugin  
│   ├── @forge/audit               # Audit logging — built as a plugin
│   ├── @forge/billing-stripe      # Stripe billing adapter
│   ├── @forge/database-postgres   # Postgres adapter
│   └── @forge/compliance-pcidss   # PCI-DSS compliance template
│
└── community-plugins/             # Community-published (npm, registry)
    ├── @acme/forge-auth-magic     # Magic link auth alternative
    ├── @acme/forge-billing-paddle # Paddle billing adapter
    └── @acme/forge-module-crm     # Full CRM module
```

**Critical rule: "If the core team can do it, the community can do it."**
- All first-party plugins use **only** public extension APIs
- No private/internal APIs available to first-party plugins that aren't available to community
- Forces the extension API to be powerful enough for any real use case

### 20.2 Extension Points (The Public API Surface)

Forge exposes well-defined extension points at every architectural layer:

| Extension Point | What You Can Build | Examples |
|----------------|-------------------|----------|
| **Modules** | Full domain modules with schema, services, controllers | CRM, e-commerce, LMS, fintech ledger |
| **Adapters** | Swap out backing services | Stripe → Paddle, Postgres → MySQL, S3 → R2 |
| **Capabilities** | Business actions invokable by humans, agents, or chat | `refund_order`, `qualify_lead`, `generate_report` |
| **Intent Handlers** | Map external triggers (HTTP, chat, voice, agent, schedule) to capabilities | REST binding, MCP tool binding, chat command binding |
| **Generators** | New `forge generate` commands | `forge generate graphql`, `forge generate trpc` |
| **Lint Rules** | Custom convention enforcement | Domain-specific patterns, team standards |
| **Scanners** (§4) | Custom `forge scan` families: detect-and-propose-fix pipelines with confidence scoring | `@acme/forge-scan-pii`, `@beta/forge-scan-hipaa`, `@org/forge-scan-perf-budgets` |
| **Migration Operators** | Custom migration step types | Encrypted column, partitioning, materialized views |
| **Middleware** | Request/response pipeline hooks | Custom auth, rate limiting, tracing |
| **Guards** | Authorization decorators | Role checks, feature flags, A/B test gates |
| **Event Subscribers** | React to framework events | Audit triggers, webhooks, analytics |
| **Outcome Recorders** | Capture business outcomes for attribution/billing | Lead qualified, ticket resolved, transaction completed |
| **CLI Commands** | New top-level `forge <cmd>` | `forge deploy`, `forge backup`, `forge fixtures` |
| **Config Providers** | New configuration sources | Vault, AWS Secrets Manager, etcd |
| **Test Helpers** | Reusable test utilities | Domain-specific fixtures, mock factories |
| **LLM Instructions** | Prompts for LLM-assisted generation | Domain-specific patterns for AI assistants |
| **Templates** | Full project starters | Fintech starter, healthcare starter, marketplace |
| **Themes / UI Kits** | (Optional, opinionated UI) | shadcn-based, MUI-based, custom design systems |

### 20.3 Plugin Anatomy

A Forge plugin is a standard npm package with a `forge.plugin.ts` manifest:

```typescript
// @acme/forge-billing-paddle/forge.plugin.ts
import { defineForgePlugin } from '@forge/core';

export default defineForgePlugin({
  name: '@acme/forge-billing-paddle',
  version: '1.0.0',
  forgeVersion: '^1.0.0',           // Compatible Forge versions

  // Declare extension points used
  provides: {
    adapters: ['billing'],
    cliCommands: ['paddle:sync'],
    migrations: ['./migrations'],
    instructions: ['./forge-instructions/paddle.md'],
  },

  // Lifecycle hooks
  onInstall: async (ctx) => { /* setup */ },
  onLoad: async (ctx) => { /* runtime init */ },
  onUninstall: async (ctx) => { /* cleanup */ },

  // Configuration schema (validated at load time)
  config: z.object({
    apiKey: z.string(),
    webhookSecret: z.string(),
    sandbox: z.boolean().default(false),
  }),
});
```

**Standard plugin structure:**
```
@acme/forge-billing-paddle/
├── forge.plugin.ts           # Manifest
├── src/
│   ├── adapter.ts            # Implements BillingAdapter interface
│   ├── webhook-handler.ts
│   └── cli/sync.ts
├── migrations/               # Plugin-specific migrations
├── forge-instructions/       # LLM context for this plugin
├── tests/
├── README.md
└── package.json
```

### 20.4 Plugin Installation & Composition

**Installation is one command:**
```bash
forge add @acme/forge-billing-paddle
```

This automatically:
1. Installs the npm package
2. Validates Forge version compatibility
3. Runs `onInstall` hook (creates config files, prompts for required env vars)
4. Registers extension points
5. Applies plugin migrations (if any)
6. Updates `forge.config.ts` with plugin entry
7. Adds plugin's LLM instructions to `.forge/instructions/`

**Composition rules:**
- Multiple plugins can extend the same point (e.g., multiple middleware)
- Conflict resolution via priority + explicit ordering in `forge.config.ts`
- Adapters are exclusive (only one billing adapter active at a time)
- Modules are namespaced — no collision between `@acme/crm` and `@beta/crm`

### 20.5 Stability & Compatibility Guarantees

**Core API Stability Tiers (Spring's lesson: stability builds ecosystems)**

| API Tier | Stability Promise | Use For |
|----------|------------------|---------|
| **Public Stable** | No breaking changes within MAJOR version | Plugin development — safe |
| **Public Beta** | May change in MINOR with notice | Experimental plugins |
| **Internal** | No stability promise; usage discouraged | Core team only |
| **Deprecated** | Removal in 2 MAJOR versions | Migration path provided |

**Plugin compatibility matrix:**
- Plugins declare `forgeVersion: '^1.0.0'` — enforced at install
- `forge doctor` checks all plugins for compatibility before deploys
- Plugin authors get advance notice (6 months minimum) of breaking changes
- Compatibility test suite available: plugins can run against Forge nightly to catch breakage early

### 20.6 Plugin Discovery: The Forge Registry

A dedicated discovery surface for community plugins (modeled on npm + crates.io + WordPress plugin directory):

```
registry.forge.dev
├─ Search & filter (by category, tier, downloads, rating)
├─ Plugin pages (README, install command, version history, dependencies)
├─ Quality signals:
│  ├─ Test coverage badge
│  ├─ Forge compatibility badge (auto-tested against latest Forge)
│  ├─ Security audit status
│  ├─ Maintenance status (last commit, open issues)
│  └─ Community rating + reviews
└─ Curated collections ("Fintech essentials", "Best auth plugins")
```

**Registry tiers:**
- **Community** — anyone can publish (default tier, requires CoC compliance)
- **Verified** — reviewed by core team for security/quality (badge)
- **Official** — maintained by Forge core team (`@forge/*` namespace)

**Open standard:** the registry is just an index over npm. Plugins are normal npm packages — you can `npm install` directly without using the registry. This avoids vendor lock-in for plugin distribution.

### 20.7 Developer Experience for Plugin Authors

Low-friction plugin authoring is critical for ecosystem growth:

```bash
# Scaffold a new plugin in 30 seconds
forge create plugin my-plugin --type=adapter --extends=billing
```

Generates: forge.plugin.ts manifest, adapter interface stub, test scaffolding, README template, GitHub Actions CI for compatibility testing, publishing checklist.

**Provided to plugin authors:**
- **Plugin SDK** — typed APIs, lifecycle hooks, helpers
- **Plugin Test Kit** — mock Forge runtime, fixtures, integration helpers
- **Compatibility Tester** — runs your plugin against multiple Forge versions in CI
- **Plugin Inspector** — `forge plugin inspect <name>` shows extension points used, conflicts, perf impact
- **Documentation generator** — `forge plugin docs` generates standard plugin README sections
- **Reference plugins** — every official plugin's source is the canonical example

### 20.8 Governance for Plugin Ecosystem

**Plugin namespace policy:**
- `@forge/*` reserved for official plugins
- `forge-plugin-*` and `@scope/forge-*` recommended for community (but not enforced)
- No "Forge" trademark in plugin names without permission

**Quality signals, not gates:**
- We do NOT moderate which plugins exist (npm doesn't either)
- We DO surface quality signals: tests passing, security audit, maintenance status
- We DO maintain a clear "Verified" tier for vetted plugins
- Bad-actor plugins handled per Code of Conduct (npm has the unpublish authority)

**Plugin promotion path:**
```
Community plugin
   ↓ (gains traction, used by 100+ apps)
Verified plugin (security audit passed)
   ↓ (becomes essential to ecosystem)
Official plugin candidate (RFC: should this be in @forge/*?)
   ↓ (community RFC approval)
Official plugin (maintained by core team OR by promoted maintainer)
```

This path keeps the official surface small (per §20.1) while creating a clear progression for community contributors.

### 20.9 LLM-Native Extensibility (Unique to Forge)

This is where Forge differs from Spring and React: every plugin ships **first-class LLM context**.

```
@acme/forge-module-crm/
└─ forge-instructions/
   ├─ module.instructions.md      # How to use this CRM module with LLMs
   ├─ patterns.md                 # Idiomatic usage patterns
   ├─ anti-patterns.md            # What NOT to do
   └─ examples/                   # Real-world examples for LLM context
```

When a developer runs `forge add @acme/forge-module-crm`:
- Plugin's instructions auto-merge into project's `.forge/instructions/`
- LLMs (Cursor, Copilot) immediately know how to use the new plugin correctly
- No manual prompt engineering required

**This is the killer feature for plugin authors:** their plugin works correctly with AI assistants out of the box. Other frameworks require users to manually teach the AI about each library.

### 20.10 Anti-Patterns to Avoid (Lessons from Other Ecosystems)

We explicitly reject patterns that have hurt other ecosystems:

- **No private APIs for first-party plugins.** (Lesson: jQuery's internal-vs-public confusion fragmented its ecosystem.)
- **No "core repo only" features.** Everything ships through the same plugin pipeline. (Lesson: WordPress core/plugin tension.)
- **No silent plugin loading.** All plugins explicit in `forge.config.ts`. (Lesson: Magic auto-loading in Rails caused debugging nightmares.)
- **No global namespace pollution.** All plugin APIs scoped to import. (Lesson: jQuery global `$`.)
- **No deeply-nested plugin dependencies.** Plugins should be flat and composable. (Lesson: webpack loader nightmare.)
- **No "thin core, heavy plugins" extremism.** Core must be powerful enough that simple apps don't need 20 plugins. (Lesson: early Express.js "choose everything yourself" overwhelm.)
- **No breaking changes without codemods.** If we break the plugin API, we ship the migration tool. (Lesson: AngularJS → Angular 2 ecosystem collapse.)

### 20.11 Ecosystem Success Metrics (added to North Star)

Beyond the metrics in §19, ecosystem health is measured by:

| Metric | Target Y1 | Target Y2 | Target Y3 |
|--------|-----------|-----------|-----------|
| Community-published plugins | 50+ | 200+ | 1,000+ |
| Plugins in "Verified" tier | 10+ | 50+ | 200+ |
| Apps using ≥3 community plugins | 30% | 50% | 70% |
| Average plugins per app | 3 | 6 | 10 |
| Time-to-first-plugin (new author) | < 2 hours | < 1 hour | < 30 min |
| Plugin author retention (publishes 2nd plugin) | 40% | 60% | 70% |

**Ecosystem health is the leading indicator of framework longevity.** A framework with 1,000 active plugins and 100 contributors is impossible to displace. A framework with 100,000 stars but no plugin ecosystem is one trend away from irrelevance.

---

## 21. Future-Proof Architecture

> **Thesis:** The era of "developer writes code in IDE → user clicks UI in browser" is one paradigm — not the only one. Forge is designed for the next 10 years, not the last 10. Three paradigm shifts are already underway, and Forge's architecture must absorb them without core rewrites:
> 1. **Chat-interface applications** replacing or augmenting traditional UIs
> 2. **Multi-agent systems** as first-class application architectures
> 3. **Outcome-based / value-based business models** replacing seat-licensing and usage-metering
>
> Forge's core abstractions are deliberately broader than today's needs so these futures slot in as plugins, not refactors.

### 21.1 Foundational Abstractions That Future-Proof Forge

The core is built around five primitives that are agnostic to interaction style and business model:

| Primitive | Today's Use | Future-Proofing |
|-----------|-------------|-----------------|
| **Intent** | HTTP request, form submission | Chat message, voice command, agent action, scheduled trigger |
| **Capability** | API endpoint, controller method | Tool callable by humans, agents, or other systems |
| **Actor** | Authenticated user | User, agent, system, scheduled job — all have identity + permissions |
| **Outcome** | Function return value | Measurable result with provenance, value attribution |
| **Trace** | Request log | End-to-end causal chain across humans, agents, systems |

Every Forge module is built on these primitives. A `POST /api/orders` is just one shape of *Intent → Capability → Outcome*. A chat message asking "refund my order" is the same shape. An autonomous agent triaging support tickets is the same shape.

### 21.2 Future #1: Chat-Interface Applications

Many apps will move from "forms and buttons" to "talk to the app." Forge prepares for this from day one.

#### Design Implications

- **Capability-first, UI-second.** Every capability (the thing the app does) is exposed independently of how it's invoked. A controller method is just one binding to a capability — a chat tool, a voice command, or an agent call are equivalent bindings.
- **Natural-language manifest per capability.** Each capability declares its purpose, parameters, and constraints in structured natural language for LLM consumption.
- **Conversational state is first-class.** Sessions, conversation memory, and context window management are core concerns, not bolted on.
- **Streaming-native.** Response streaming (SSE, WebSocket) is a default, not an exception.

#### Concrete Architecture Support

```typescript
// Same capability, multiple interaction surfaces
@Capability({
  name: 'refund_order',
  description: 'Refund an order, optionally partial. Triggers customer notification.',
  inputs: z.object({ orderId: z.string(), amount: z.number().optional() }),
  outcome: z.object({
    refundId: z.string(),
    amount: z.number(),
    status: z.enum(['pending', 'completed']),
  }),
  authz: ['order:refund'],
  audit: 'order.refund',
})
async refundOrder(input, ctx) { /* business logic */ }

// Forge auto-generates:
// - REST endpoint:    POST /capabilities/refund_order
// - Chat tool:        callable from any chat-interface plugin
// - Agent tool:       registered in tool catalog (MCP-compatible)
// - GraphQL mutation: if @forge/graphql plugin installed
// - CLI command:      forge invoke refund_order --order-id=...
```

#### Built-In Chat Plumbing

- **`@forge/chat-runtime`** (official plugin) — conversation persistence, message threading, role attribution
- **MCP (Model Context Protocol) server adapter** — expose any Forge app to MCP-aware clients (Claude Desktop, Cursor, etc.)
- **Conversation-scoped audit** — every action taken in a chat session is auditable as part of that conversation
- **Multi-modal inputs** — text, voice, image, file uploads handled uniformly through the Intent primitive

### 21.3 Future #2: Multi-Agent Systems as First-Class Citizens

The next paradigm beyond "single LLM with tools" is **systems of cooperating agents**, each with its own role, memory, and authority. Forge treats agents as first-class actors equivalent to human users.

#### Agent-as-Actor Model

```typescript
// Agents are real entities in the system
workspace_members: {
  id, workspace_id,
  actor_type: 'human' | 'agent' | 'system',
  identity_id, role, permissions, ...
}

// Same RLS, same audit log, same authz — unified for humans and agents
```

This means:
- An agent has a workspace membership, a role, and explicit permissions — just like a user
- RLS policies apply equally; an agent cannot access cross-tenant data any more than a user can
- Every agent action is in the audit log with `actor_type='agent'` and `actor_id=<agent-id>`
- Agents can be granted or revoked permissions at runtime
- Agents have rate limits, budgets, and resource quotas just like users

#### Built-In Agent Primitives (Optional Plugins)

- **`@forge/agent-runtime`** — lifecycle management for agents (spawn, pause, terminate, replay)
- **`@forge/agent-coordinator`** — multi-agent orchestration (sequential, parallel, hierarchical, consensus patterns)
- **`@forge/agent-memory`** — short-term + long-term + episodic memory backed by Forge's storage layer
- **`@forge/agent-toolbox`** — standardized tool definition format compatible with major LLM providers
- **`@forge/agent-sandbox`** — isolated execution environment for untrusted agent code

#### Multi-Agent Safety (Built-In)

Multi-agent systems introduce new failure modes that Forge addresses architecturally:

- **Causal traces across agents** — the Trace primitive captures full causal chains: "User asked X → Agent A delegated to Agent B → Agent B called capability Y → Outcome Z"
- **Cost & token budgets per agent** — prevents runaway loops; budgets are first-class config
- **Loop detection** — framework detects A→B→A cycles and circuit-breaks
- **Agent authority limits** — agents cannot exceed the permissions of the user who spawned them (delegation chain)
- **Reversibility tagging** — capabilities marked `reversible: false` (e.g., money transfer) require explicit human approval when invoked by agents above a threshold
- **Agent audit log** — separate, immutable, queryable for compliance review
- **Kill switch** — `forge agents stop --workspace=<id>` halts all agents in a workspace immediately

#### Standards Alignment

Forge will track and adopt emerging standards rather than inventing proprietary ones:
- **MCP (Model Context Protocol)** — native server + client adapters
- **OpenAI Assistants API / Responses API** — adapter for compatibility
- **Anthropic Computer Use / Tool Use** — first-class support
- **Future agent interop standards** — framework architecture (capability + intent abstractions) is designed to absorb whatever wins

### 21.4 Future #3: Outcome-Based / Value-Based Business Models

The shift from "per-seat SaaS" to "pay for outcomes" is one of the largest pricing-model shifts in software history. Examples already emerging:
- Pay per resolved support ticket (instead of per agent seat)
- Pay per qualified lead generated (instead of per CRM user)
- Pay per successful loan underwriting (instead of per loan officer)
- Revenue share on agent-generated revenue
- Risk-share / outcome-guarantee contracts

This fundamentally changes what the application must measure, attribute, and report. Forge bakes these capabilities into the core.

#### The Outcome Primitive

Every capability can declare an outcome shape with measurable, attributable results:

```typescript
@Capability({
  name: 'qualify_lead',
  outcome: {
    schema: z.object({
      leadId: z.string(),
      qualified: z.boolean(),
      score: z.number(),
    }),
    valueAttribution: {
      // What measurable business value did this produce?
      metric: 'qualified_lead',
      unit: 'lead',
      // Whose action produced this? (for revenue share, billing, attribution)
      attributable_to: ['actor', 'agent', 'workflow'],
    },
    // Was the outcome successful by business definition?
    successCriteria: (result) => result.qualified === true && result.score >= 70,
  },
})
```

#### Built-In Outcome Infrastructure

- **Outcome ledger** — immutable append-only record of every measurable outcome (analogous to the audit log, but for business value)
- **Attribution chain** — each outcome links back through the causal trace to the actor(s) and agent(s) that produced it
- **Outcome aggregation API** — query outcomes by tenant, time range, capability, actor, agent
- **Outcome-based billing adapter** — `@forge/billing-outcomes` (official plugin) generates invoices based on outcomes, not seats or API calls
- **Risk/guarantee patterns** — templates for outcome-guarantee contracts (with reconciliation, dispute handling, refund mechanics)
- **Provenance for outcomes** — each outcome carries cryptographic provenance suitable for SLA enforcement and audit

#### Why This Matters Architecturally

Most frameworks make outcome-based pricing painful because outcomes are scattered across logs, analytics events, and database state. In Forge, outcomes are a **first-class type**, declared at the capability layer and automatically captured. This makes outcome-based billing trivial; in other frameworks, it requires building custom event pipelines.

Apps built on Forge can switch from seat-licensing to outcome-pricing by changing a billing adapter — not by re-architecting their app.

### 21.5 Forge for Forge: Eating Our Own Dog Food

Forge applies these primitives to itself:
- The Forge CLI is built on the same Capability + Intent abstractions
- The Forge Registry exposes plugins as Capabilities (so agents can install plugins)
- The Forge maintainer team uses agents (with audit + budgets) for triage, docs generation, release notes
- This forces us to dogfood the future-proof primitives — if they don't work for our own tooling, they don't work

### 21.6 Future-Resilience Principles (Codified)

These guide every architectural decision and PR review:

1. **Don't bake today's interaction model into core.** REST is one binding for capabilities, not the only one.
2. **Don't assume "user" means human.** Use `actor` everywhere; humans, agents, systems are all actors.
3. **Don't bake today's billing model into core.** Seats, usage, outcomes are all valid; the framework supports all without preference.
4. **Don't assume synchronous request/response.** Long-running, streaming, multi-turn, asynchronous-result are all default-supported patterns.
5. **Don't conflate "who did this" with "who is responsible."** Delegation chains (user → agent → sub-agent → capability) are a first-class concept.
6. **Track outcomes, not just events.** Events are what happened; outcomes are why it mattered. Both are needed.
7. **Make new paradigms additive, not disruptive.** Adding chat-interface support to a Forge REST app should be `forge add @forge/chat-runtime`, not a rewrite.
8. **Standards over invention.** If MCP, OpenAPI, AsyncAPI, or another standard exists, adopt it. Invent only when no standard fits.
9. **Reversibility & approval as gradients.** As autonomy increases (human → agent → multi-agent), the framework progressively requires more explicit approval, audit, and reversibility for high-impact actions.
10. **Architectural humility.** We don't know exactly what 2030 looks like. The architecture's job is not to predict it but to absorb it without core rewrites.

### 21.7 Future-Proofing Anti-Patterns (Explicitly Rejected)

- **No "AI features" bolted onto a non-AI core.** If chat/agent support feels like an afterthought, the architecture is wrong.
- **No assumption that today's LLMs are tomorrow's LLMs.** Avoid hardcoded provider names, prompt formats, or model assumptions in core.
- **No "just for vibe-coding" feature creep.** If a feature only makes sense for the 2026 vibe-coding moment, it goes in a plugin, not core.
- **No conflating storage with state.** Conversational state, agent memory, and traditional DB state have different lifecycles — keep them separate.
- **No business-model lock-in.** A Forge app must be repricable (seat → usage → outcome) without core changes.

### 21.8 Validation Plan

Future-proofing claims must be validated with real builds before v1.0:

| Validation Build | Future #1 (Chat) | Future #2 (Agents) | Future #3 (Outcomes) |
|------------------|:----------------:|:------------------:|:--------------------:|
| **Chat-first SaaS** — customer support app where users only interact via chat | ✅ | — | — |
| **Multi-agent CRM** — sales, qualification, follow-up agents cooperating per workspace | — | ✅ | — |
| **Outcome-billed underwriter** — fintech app billed per successful underwriting decision | — | partial | ✅ |
| **Hybrid: agent-driven, chat-interface, outcome-billed** — the "2028 stress test" | ✅ | ✅ | ✅ |

If the hybrid build cannot be implemented cleanly using only public APIs and plugins (no core changes), the future-proof architecture has failed and must be reworked before v1.0.

---

## 22. Multi-Stakeholder Review Log

This spec is reviewed periodically by simulated cross-functional perspectives (PO, BA, Marketing, PM, SA, Security, QA, Finance) and revised in response. The log preserves the *rationale* for changes so future contributors don't reverse them blindly.

### v0.8 review (May 2026) — Findings → Actions

| Hat | Finding | Resolution |
|-----|---------|------------|
| **PO** | No prioritized cut for v1.0; personas listed as future artifacts in a 1500-line spec | Added named persona drafts in §11.1.1 (Sam / Priya / Marcus) with JTBD + pain quote; Marcus explicitly deferred to v3.0+ |
| **PO** | NSM is lagging; no weekly leading indicator | Added §19.1.1: weekly count of `forge new` → `forge deploy` within 14 days by external devs |
| **BA** | "85%+ of vibe-coded projects fail" is unsourced; will be challenged on launch day | Reframed §1 as a hypothesis with public validation plan (M0 survey + M1 case studies) |
| **BA** | Token-economy savings stated as facts (20–40%) without baseline | Marked as targets in §4 narrative; eval harness CI gate will publish actuals |
| **Marketing** | "Built with Forge" badge as opt-out repeats Vercel's mistake | Flipped to opt-in with delight prompt; documented the lesson in §7 |
| **Marketing** | No launch sequence; tagline lacked sub-headline | Added 7-day launch cascade table to §7; rewrote tagline + sub-headline at top of doc |
| **PM** | M1 exit criterion ("20+ apps") is qualitative; no headcount or timeline | Added "qualifying app" 5-criterion definition; added headcount column + worst-case timeline (4.5 yr no-funding, 18–24 mo funded) + milestone dependencies |
| **SA** | Adapter-first on Next.js risks Vercel-lock-in perception | Hono reference app added to M1 scope (§6, §8 Q2) |
| **SA** | KV-prefix cache claims depend on provider behavior | §8 Q10 records that eval determinism + provider variance must be designed via adapter, not assumed |
| **Security** | `forge fix` / healer = LLM code mutation = RCE class surface if auto-applied | §8 Q8 locks default to PR-only, no auto-merge, refuses on protected branches |
| **Security** | Field-level encryption named without KMS story | §8 Q9 commits to envelope encryption + AWS/GCP/Vault adapters + BYOK at v1.0 |
| **Security** | CLI telemetry not addressed | §8 Q12: opt-in only, transparent payload, never in CI |
| **QA** | Eval determinism not defined operationally | §8 Q10: pin model+version+temperature+tools+RAG seed+embeddings; CI fails on drift |
| **Finance** | STF/NLnet are 6–9 month application cycles, not Phase 1 cash | §17.6 added: zero-funding floor — founder consults part-time, M1 stretches to 18 months, project survives |

### v0.9 review (May 2026) — Findings → Actions (Developer / Vibe-Coder hats)

The single most important review pass. Forge exists to serve these people; if they're unhappy, nothing else matters.

| Hat | Finding | Resolution |
|-----|---------|------------|
| **Sam (Solo Vibe-Coder, P1)** | "47 files I didn't write, I trust nothing" — no way to inspect/understand the scaffold | §8 Q19: `forge explain <path>` shipped in M0; Developer Promise #1 codifies "nothing magical" |
| **Sam** | "I use Claude Code, not Copilot — why does the spec only mention `.instructions.md`?" | §8 Q13: ship `AGENTS.md` + `CLAUDE.md` + `.cursorrules` + `.windsurfrules` + Copilot format from one source; Developer Promise #9 |
| **Priya (Team Lead, P2)** | "I can't greenfield-rewrite my Next.js+Supabase app" | §8 Q14: `forge adopt` MVP in M1, non-destructive, one primitive at a time; Developer Promise #6 |
| **Priya** | "Hard-locking pnpm loses 1/3 of users on day one" | §8 Q18: generated apps support npm/yarn/pnpm/bun; CI tests all four; Developer Promise #8 |
| **Jordan (Agent-First, P4)** | "My agent needs `--json` on every command, stable error codes, stable doc anchors" | Added P4 persona to §11.1.1; Developer Promise #4; Design Principle #8 ("agent-readable by default") |
| **Alex (Skeptical Senior)** | "Convention without escape hatch = create-react-app graveyard" | §8 Q15: `forge eject` from day one; Developer Promise #3; Design Principle #2 rewritten as "one way to do it (with an escape hatch)" |
| **Maya (Air-Gapped, P6)** | "Half the spec assumes an OpenAI API key — we have none" | Added P6 persona; §8 Q17: offline-first hard requirement for core features + local-LLM adapters first-class; Developer Promise #7; Design Principle #10 |
| **Riley (Frontend-Heavy, P5)** | "Where's my typed API client + React hooks + form generator?" | Added P5 persona; §8 Q16: data-layer frontend scaffolding in scope; Developer Promise #10 |
| **Devon (Test-Skeptic)** | "I ship without tests; the framework can't lecture me, it has to make tests free" | Developer Promise #5: generators always emit the test file prefilled; Design Principle #9 ("tests are emitted, not requested") |
| **Cross-cutting** | The whole spec was implicitly business-facing; no "developer's bill of rights" | Added §11.1.2 "The Developer Promise" — 10 non-negotiable DX commitments + the single rule: *"if a developer has to fight the framework to do the obvious thing, the framework is wrong, not the developer."* |

**Net change:** Forge added three personas (Jordan/Riley/Maya), 10 DX commitments, 3 design principles (#8 #9 #10), and 7 new open questions — all anchored on the truth that *the developer is the customer*, not the enterprise procurement team.

### v0.10 review (May 2026) — Operator + Vibe-Coder follow-up

Focus: continuous code-quality scanning + framework-as-student-of-the-codebase. Findings driven by the developer/operator perspective: *"my AI writes the bug, my framework should find and fix it — and learn so it doesn't write the same bug tomorrow."*

| Hat | Finding | Resolution |
|-----|---------|------------|
| **Sam (P1)** | "My AI confidently shipped a SQL-injection-shaped query last week. Where's the seatbelt?" | §4 Scan-and-fix layer: `forge scan security` runs OWASP/RLS/secrets/CVE detection with diff-first proposed fixes; pre-commit hook opt-in |
| **Sam** | "My pet peeve: the same N+1 query keeps coming back across PRs." | §4 `forge scan performance` flags N+1, missing indexes, oversized payloads with codemods; finding history in `.forge/scan-history/` blocks regressions |
| **Priya (P2)** | "I want CI to fail when the diff introduces a high-severity finding, not after it's deployed." | `forge scan --since main --ci` annotates the PR; configurable severity threshold in `forge.config.ts`; mirrors the perf-budget + token-budget pattern |
| **Jordan (P4)** | "I run agents in a loop. The framework needs to teach the agent, not just review its output." | §4 Continuous learning loop: rejected suggestions + reverts + bug-to-test pipeline feed `.forge/instructions/`; `forge teach` lets the dev pin preferences directly |
| **Maya (P6)** | "Federated learning sounds nice but I cannot phone home." | §8 Q21: local-only by default; federated sharing strictly opt-in, anonymized counts only, single-flag disable; honors §8 Q17 offline-first |
| **Sec hat** | "LLM-driven auto-fix is an RCE class. Don't blur the line between scan and apply." | §8 Q20: never auto-apply by default; `--apply` opens a PR (no auto-merge per §8 Q8); only `high` confidence eligible; refuses on protected branches |
| **Devon (Test-Skeptic)** | "Every fixed bug should leave a regression test behind, automatically." | Continuous learning loop: closed bug → offer to generate test; accepted scan finding → eval-harness scenario added |
| **Cross-cutting** | The framework was "smart at start" but not "smarter with use." | The intelligence accrues to the project (`.forge/learned/`, `.forge/instructions/` evolution, `forge session digest`) — not to a SaaS backend. The framework is *a student of the codebase, not a lecturer.* |

**Net change:** Forge added a unified `forge scan` pipeline (8 scanner families: security, performance, reliability, correctness, accessibility, cost, compliance, dx) with diff-first / confidence-scored / replayable / composable / extensible properties; added a continuous learning loop (convention learning, anti-pattern mining from reverts, prompt-quality feedback, per-project context evolution, test-from-bug, pair-programming memory, opt-in federated convention sharing, growing eval harness, `forge teach`); added 2 open questions (§8 Q20 scan auto-apply default, §8 Q21 learning-loop privacy default).

### How this section is maintained

Every minor version bump runs the same review with the same hats. New findings are appended; resolved findings are kept (so we remember why). Reviewers may be human contributors or LLM-assisted; rationale is what matters, not author identity.

---

*Document version: 0.10.9 — May 2026*
*Status: Brainstorm / Pre-RFC*
*Posture: **Community-first, commercial-later** (see §0)*
*Ultimate goal: **Ship fast with high quality, via vibe-coding** (header + §1 + §11.1.2 + §19)*
*Defined way of working: **`forge ship <feature>`** — one orchestrating command, four checkpoints (Spec → Test → Breakdown → Code → Ship). TDD-shaped vibe-coding (§1 + §4 LLM-Native Authoring + §11.2 Principle #12 + §11.1.2 Promise #5)*
*Command surface: **12 namespaces, philosophy-aligned** (§4 Command Surface) — Hygiene namespace added in v0.10.7*
*Contribution model: **3-tier (T3 plugin / T2 adapter / T1 core)** with rising standards; routes contributors via "ship as a plugin first" rather than gatekeeping (§16.5)*
*Core principles: **Developer-first DX** (§11.1.2) + **Radical extensibility** (§20) + **Future-proof architecture** (§21) + **Token economy** (§4) + **Scan-fix-learn loop** (§4) + **One-command workflow** (§4) + **Audited CLI grammar** (§4 Command Surface) + **Tiered contribution standards** (§16.5) + **Repo-hygiene containment of LLM scratch** (§4 Repo Hygiene Layer + §16.5.4 #11) + **Framework-managed `.gitignore` and `.gitleaks.toml`** (§4 Repo Hygiene Layer + §16.5.4 #12)*
*Changelog:*
*• v0.10.9 — Companion architecture work: extended `ARCHITECTURE.md` with two new sections that close the resilience + operations gap. **§17 Failure modes, unhappy paths & resilience** introduces the resilience contract (fail-closed, bounded waits, recoverable state, surface-don't-swallow, reversibility, explicit degradation, no-secret-leak-on-error), a per-layer failure register (20 components × {unhappy path / detection / containment / recovery / test anchor}), 8 cross-cutting failure scenarios (provider outage mid-`ship`, disk-full mid-apply, concurrent ship, plugin panic during scan, ledger tamper, cassette drift, secret-leak-via-debug, prod migration drift), and 7 CI-enforced resilience invariants. **§18 Bug & issue lifecycle** defines intake channels (CLI `forge report`, GitHub issue templates, private security advisory, bug bounty, Discussions/Discord, internal incidents, eval-harness self-reports), the S0–S4 severity model with first-response + time-to-fix SLAs, the triage flow, the **two-key rule** for irreversible operations during incidents, the hotfix process (failing-test-first even under pressure), the **post-mortem contract** (mandatory for S0/S1, blameless, every PM ships at least one durable action — new test, lint rule, gate, or §17 register entry), the community-reporter feedback loop (human ack within SLA, credit, ask-before-close), and Quality-Dashboard metrics. Reading guide updated; ARCHITECTURE.md doc version bumped 0.1 → 0.2. No spec body changes — this release threads operational resilience into the architecture without altering product surface.*
*• v0.10.8 — Extended the §4 Repo Hygiene Layer with two framework-managed file standards: (a) **`.gitignore` standards** — `forge new` ships a curated baseline assembled from per-stack fragments, with a version-stamped framework-managed block plus a user-owned section; mandatory hygiene block enforced by `forge clean --check` (`.env*` with `!*.example`/`!*.template` negations, `.forge/llm-scratch/`, `.forge/trash/`, `.forge/cache/`, OS/editor junk, LLM-scratch globs mirroring the hygiene manifest); negation discipline (templates always trackable, fail if shadowed); drift detection via `forge doctor` + idempotent `forge upgrade gitignore`; secret-file guard list (`.env`, `.env.local`, `.env.staging`, `.env.production`, `*.pem`, `*.key`, `*.pfx`, service-account JSONs) cross-checked against `git ls-files`. (b) **`.gitleaks.toml` standards** — framework-managed config extending upstream defaults with Forge-aware rules (Supabase service-role JWT shape, Stripe live keys `sk_live_*`/`pk_live_*`/`rk_live_*`/`whsec_*`, PayPal live client IDs, OpenAI/Anthropic/Google/Twilio/SendGrid keys, social tokens, PEM headers, generic high-entropy `.env`-position fallback); allowlist standards require `description` + `path/regex` + `# review-by: YYYY-MM-DD` expiry; default scope (full history on `main`, PR diff in CI, pre-commit on diff); `--no-verify` bypass requires explicit `gitleaks-bypass: <reason>` commit token surfaced in review; fixture-safety carve-out for `tests/fixtures/secrets-corpus/` (clearly-tagged `FORGE_FAKE_*` keys); idempotent `forge upgrade gitleaks`; privacy invariant — raw match values never written to logs/telemetry/LLM context (only path + line + rule ID + redacted preview). Added universal CI gate **#12 Secrets clean** to §16.5.4 — `forge scan security --secrets --since main` must be green; expired allowlists fail; bypass token surfaced in PR review. Extended gate #11 to also block PRs that weaken the managed `.gitignore` block, shadow `.example`/`.template` negations, or track a file in the secret-file guard list. Updated footer fingerprint with the new core principle.*
*• v0.10.7 — Added the **Repo Hygiene Layer** (§4 Feature Matrix) — a first-class framework concern that contains the LLM-scratch sprawl every coding agent (including Forge's own `ship`) produces. Defines: hygiene manifest (`.forge/hygiene.yml`) declaring source / generated / scratch / quarantine globs; `forge clean` command with `--check` (CI gate, exit non-zero on findings), `--dry-run` (default), and `--apply` (recoverable via `.forge/trash/<run-id>/`); per-tool `forge-owner` tagging on every framework-generated file; LLM scratch interception (LLM writes outside declared paths land in `.forge/llm-scratch/<task-id>/` instead of repo root, deleted at task end if not promoted); hygiene as a `ship` checkpoint between Code and Ship (manifest-or-delete; no "ignore for now"); pre-commit + CI gate; weekly hygiene digest in `forge insights`; privacy invariant (quarantined/scratch files never enter LLM context). Added namespace **#12 Hygiene** to the §4 command surface table (`forge clean`, `forge hygiene report`, `forge hygiene manifest`). Added universal CI gate **#11 Repo hygiene clean** to §16.5.4 — PRs are blocked when `forge clean --check` finds unmanifested LLM scratch (`_*`, `patch_*`, `fix_*`, `*_output.*`, `*_SUMMARY*.md`, `scratch/**`, etc.); legitimate new generated artefacts must be declared in the same PR. Updated footer fingerprint: command-surface count 11 → 12, added "Repo-hygiene containment of LLM scratch" to core principles. Closing principle: *"the framework that produces the files is the framework that cleans them up."**
*• v0.10.6 — Added §16.5 "Community Contribution Standards (Core Modules)" — a 9-subsection contribution charter that makes contributing to core *possible for anyone* and *impossible to lower the quality floor*. Defines: (16.5.1) three concentric trust tiers (T3 Plugins / T2 Adapters & Recipes / T1 Core) with rising standards and explicit promotion path; (16.5.2) one well-lit contribution flow built on `forge ship` itself — Forge eats its own dog food; (16.5.3) eligibility ladder (T3 anyone with DCO; T2 needs ≥3 merged T3 contributions; T1 needs ≥5 T2 + signed commits); (16.5.4) ten universal CI-enforced gates that apply to every PR (has-spec, tests-precede-code timestamp check, scans clean, convention-aligned, public-API delta declared, token-budget observed, docs honest, DCO/signed commits, backward-compat with deprecation cycle, no silent telemetry); (16.5.5) T2 adapter standards (compliance suite, two real-world consumers, named maintainer for ≥12 months); (16.5.6) T1 core standards (RFC naming the principle/goal served, two core-maintainer approvals, mandatory security review trigger list — auth/RLS/audit/crypto/secrets/migration runner/scan engine/multi-tenancy defaults/network egress, migration path with codemod, performance budget, learning-loop neutrality); (16.5.7) maintainer review SLAs (first response 7/5/3 days, time-to-merge-or-decline 30/21/14 days, no silent rejection); (16.5.8) recognition & maintainer ladder (Contributor → Reviewer → Maintainer → Core Maintainer); (16.5.9) the kindest standard — "ship as a plugin first" — every "no" to core is paired with a "yes" to a plugin path. Closing principle: *"the standards exist to protect users, not to gatekeep contributors."* Footer fingerprint gains "Contribution model" line and "Tiered contribution standards" core principle.*
*• v0.10.5 — Audited, reorganized, and rebuilt the entire Forge command surface to reflect the philosophy, principles, methodology, and ultimate goal. Inventoried all `forge ...` commands referenced across the spec (~50 unique verbs scattered across §1, §4, §10, §13, §16); collapsed into **11 coherent namespaces** mapped to the framework's pillars (Project Lifecycle / `ship` / Generate / Scan & Fix / Learn / Context & Ask / `migrate` / Audit / Operate / Plugin / Docs). Added a new "Command Surface" subsection at the end of §4 (just before §5) containing: (1) seven design rules the CLI obeys (one verb per intent, one way + escape hatch, agent-readable, errors must teach, LLM-native, boring by default, the Way is one command); (2) the 11-namespace command table; (3) the "four first-day verbs" discoverability contract (`forge new` → `forge ship` → `forge scan all` → `forge insights`); (4) a rename/consolidation map covering 14 deprecated aliases (e.g. `forge spec test` → `forge ship test`, `forge teach` → `forge learn teach`, `forge gdpr erase` → `forge audit erase`, `forge migrate-code` → `forge upgrade`, `forge generate ai-context` → `forge context generate`); (5) seven universal flags every command supports (`--json`, `--yes`, `--explain`, `--dry-run`, `--workspace`, verbosity, `--profile`); (6) stable `FORGE-XXXX` error code scheme. Closed with: *"the CLI is the framework's grammar. If a developer cannot guess the verb for what they want to do, the verb is in the wrong namespace."* Added "Audited CLI grammar" + "Command surface" lines to the document fingerprint.*
*• v0.10.4 — Collapsed the Spec→Test→Breakdown→Code workflow into a single orchestrating command: **`forge ship <feature>`**. The §4 LLM-Native Authoring subsection was rewritten around this one command: a 5-row checkpoint table (Spec / Test / Breakdown / Code / Ship), `--resume` (pick up where you left off), `--yes` (non-interactive for CI/agent mode with structured JSON events per checkpoint), and `--quick` (audited escape hatch for trivial changes). Added a "Why one command, not five" rationale block (discoverability, order-enforced-not-remembered, resumable, agent-friendly, single-PR audit trail, sub-commands remain first-class as resume/escape points). Updated §1 three-loop block + §1 "defined way of working" paragraph to refer to the single command. Updated §11.2 Principle #12 to "...enforced by one command" with explicit `--skip-checkpoint=...` flag for the rare override. Updated §11.1.2 Promise #5 to reference `forge ship` directly. Bumped footer + reworded "Defined way of working" line to highlight the single command.*
*• v0.10.3 — Codified the framework's *defined way of working*: **Spec → Test → Tasks → Code** (TDD applied to vibe-coding). Added a new "defined way of working" paragraph in §1 right after the three-loop block. Inserted a full Spec→Test→Tasks→Code subsection in §4 LLM-Native Authoring (4-step table with `forge spec new` / `forge spec test` / `forge spec breakdown` / `forge spec next` + `forge spec ship`, the artifacts each step produces in `.forge/specs/<feature>/`, and 7 properties of the workflow including timestamp-checked test-before-code enforcement, LLM-sized task budgets, an audited escape hatch (`forge spec quick`), vendor-neutral prompt templates, and learning-loop integration via `.forge/learned/spec-failures.jsonl`). Closed the subsection with the principle: *"Spec is the intent. Tests are the proof. Tasks are the speed. Code is the consequence."* Added Design Principle #12 in §11.2: "Spec before test, test before code, task before commit." Extended Developer Promise #5 in §11.1.2 to make the Spec→Test→Tasks→Code workflow non-skippable for non-trivial changes. Bumped footer + added "Defined way of working" line to the document fingerprint.*
*• v0.10.2 — Promoted "ship fast with high quality via vibe-coding" to the framework's stated ultimate goal. Added an explicit goal callout to the document header (acts as the single decision-filter against which every feature/default/roadmap item is judged). Added a "speed × quality, no trade-off" paragraph to §1 Exec Summary explaining how the framework collapses the historical speed/quality trade-off (quality parts are generated/scanned/learned, not hand-written). Tied §11.1.2 Developer Promise preamble to the same goal so all 10 DX commitments inherit it. Added a "Why this NSM" preface to §19 explaining how the NSM measures both halves of the goal (speed = reached production; quality = survived 30+ days; vibe-coding = AI-assisted, captured in qualifying-app survey).*
*• v0.10.1 — Coherence pass integrating the v0.10 scan/learn pillars across the rest of the spec. §1 Exec Summary now states the "generate → scan → learn" three-loop value prop. §2 Feasibility Production/Enterprise rows mention scanners + compliance templates; added a "Maintenance" row about the learning loop. §6 Roadmap explicitly maps scanner families and learning-loop MVP to M1/M2/M3 milestones. §11.2 added Design Principle #11 ("scan over scold, fix over flag, learn over lecture — the framework is a student of the codebase, not a lecturer"). §5 added a clarifying note on Scan & Learn placement (cross-cutting via CLI + plugin API + LLM Runtime feedback). §13 Test Strategy opens with closed-loop testing callout. §16 Governance opens with scanner-plugin + federated-aggregator governance note. §19.2 added 3 supporting metrics (scanner finding-to-fix time, false-positive rate, learned-convention adoption). §20.2 Extension Points table added a first-class "Scanners" row. §4 Continuous learning loop now ships a concrete `.forge/learned/conventions.jsonl` + `.forge/learned/preferences.yml` example. Added §8 Q22 (scanner customization — yes, three-layer escape hatch with mandatory reasons).*
*• v0.10 — Added §4 Scan-and-fix layer: unified `forge scan` pipeline with 8 scanner families (security/performance/reliability/correctness/accessibility/cost/compliance/dx); each runs in --report/--suggest/--apply modes, diff-first, confidence-scored, replayable, composable, extensible via plugins; pre-commit + CI gate. Added §4 Continuous learning loop: convention learning from accepted/rejected lints, anti-pattern mining from reverts, prompt-quality feedback, per-project `.forge/instructions/` evolution, test-from-bug, pair-programming session digests, opt-in federated convention sharing (anonymized counts only), growing eval harness, `forge teach` for direct preferences. Added §8 Q20 (scan auto-apply default — never; PR-only) and Q21 (learning-loop privacy — local-only default, federated opt-in). Added v0.10 review log to §22. Principle: "the framework is a student of the codebase, not a lecturer."*
*• v0.9 — Developer / vibe-coder review pass (the most important review). Added 3 personas (Jordan agent-first, Riley frontend-heavy, Maya air-gapped) to §11.1.1. Added §11.1.2 "The Developer Promise" with 10 non-negotiable DX commitments + the rule "if a developer has to fight the framework to do the obvious thing, the framework is wrong." Added 3 design principles (agent-readable by default, tests emitted not requested, offline-first for core). Added 7 open questions (§8 Q13–Q19): multi-tool AI instructions (Copilot+Claude+Cursor+Windsurf+AGENTS.md), `forge adopt` for incremental adoption in M1, `forge eject` from day one, frontend data-layer scaffolding (typed client + React Query hooks + Zod forms), offline-first hard requirement for core, package-manager agnostic (npm/yarn/pnpm/bun), `forge explain <path>` shipped in M0. Added v0.9 review log to §22.*
*• v0.8 — Multi-stakeholder review pass (PO/BA/Marketing/PM/SA/Sec/QA/Finance). Added: ICP statement to header; reframed §1 "85%" claim as a hypothesis with public validation plan; added headcount assumptions + worst-case timeline + "qualifying app" definition + milestone dependencies to §6; flipped "Built with Forge" badge to opt-in (§7); added 7-day launch sequence (§7); added §11.1.1 with three named persona drafts (Sam/Priya/Marcus); added 5 new open questions to §8 (HITL default for forge fix, KMS adapter, eval determinism, badge default, telemetry opt-in); added §17.6 zero-funding floor; added §19.1.1 leading indicator; added §22 stakeholder review log.*
*• v0.7.1 — Updated §5 System Overview diagram to reflect token economy: added Capability Registry / Idempotency-Outbox / Audit-Trace blocks to Core Runtime; inserted dedicated "LLM Runtime & Token Economy" tier (Prompt Compiler+Registry, Model Router/Cascade, Context Budgeter, Response Cache, Embedding Store, Token Budgets+Telemetry, Eval Harness, Guardrails, Healer/Insights); expanded Adapter Layer with LLM and Vector adapters; expanded CLI line with fix/review/eval/insights/ask/optimize.*
*• v0.7 — Added "Token economy" block to LLM-Native Layer: prompt compiler, context budgeter, semantic + KV-prefix caching, model router & cascade, batched/speculative calls, streaming early-stop, embedding dedup, retrieval-not-stuffing, tool-call minimization, per-feature token attribution, CI cost gate, typed token budgets, self-optimizing prompts. Principle: "every token spent is a token measured."*
*• v0.6 — Expanded Foundation Layer feature matrix (DI, jobs, events, cache, storage, i18n, email, search, API, webhooks, realtime, idempotency, validation, CLI kernel); rewrote LLM-Native Layer to cover authoring/build/runtime/operate phases (forge fix, forge review, self-healing, auto-triage, drift detector, observability assistant); compressed §6 Roadmap from phased calendar plan to 4 capability-gated milestones (M0–M3); broadened §12 from "fintech-only" to "financial-grade for any high-trust domain" with multi-template generator (healthcare/govtech/marketplace/identity/agent/high-trust)*
*• v0.5 — Added 5th design pillar (Future-Proof Architecture); added §21 covering chat interfaces, multi-agent systems, outcome-based business models; added Capabilities / Intent Handlers / Outcome Recorders to extension points; fixed §20 formatting corruption*
*• v0.4 — Added 4th design pillar (Radical Extensibility); added §20 Extensibility Architecture (plugin system, extension points, marketplace, governance, DX); updated north star to include ecosystem health*
*• v0.3 — Added §0 Strategic Posture (community-first, commercial-later); reframed §7 Phase 3, §12.3, §17, §19 to align with community-first commitment; removed near-term revenue projections; explicitly discouraged VC funding*
*• v0.2 — Added Anti-Goals (§1), Design Phase (§11), Banking/Finance Module (§12), Test Strategy (§13), Deployment Strategy (§14), Licensing (§15), Governance (§16), Funding (§17), Versioning Policy (§18), North Star Metric (§19)*
*• v0.1 — Initial draft*
