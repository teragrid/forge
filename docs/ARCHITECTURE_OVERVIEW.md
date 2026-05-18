# Architecture Overview

> A concise onboarding guide to the Forge codebase for first-time contributors.
> After reading this you should be able to navigate the repo and find the right
> file for any change.  For the full engineering blueprint see
> [ARCHITECTURE.md](ARCHITECTURE.md).

> **Just want to *use* Forge, not build it?** Read
> [GETTING_STARTED.md](../GETTING_STARTED.md) instead. This document is for
> people who want to understand the internals or contribute code.

---

## 1. What Forge Actually Does

### The one-paragraph version

AI coding tools (Copilot, Cursor, Claude Code) are great at writing code, but
they don't notice when that code leaks an API key, skips a test, or breaks a
project convention. Forge is a small command-line program that sits between
your AI tool and your Git repository. Before any AI-generated code reaches a
remote branch, Forge runs a local **Scan → Fix → Learn loop** that catches the
mistakes humans usually miss in PR review.

### The three pillars

Three ideas hold the whole system together. Every architectural decision below
traces back to one of them.

### 1.1  Convention as Code

Project conventions live in `.forge/instructions/` as plain-text files. Both
the CLI linter **and** the LLM read the **same** source. If a rule changes, you
update one file and both enforcement paths stay in sync automatically — no more
docs that say one thing while the linter does another.

### 1.2  Scan-Fix-Learn Loop

Every `forge ship` run triggers a pipeline:

1. The **Scanner** families (secrets, RLS, prompt-injection, supply-chain) read
   your staged diff.
2. If a finding is fixable, a **Codemod** patches it in place automatically.
3. The outcome is written to the **Audit Ledger** (`.forge/audit.log`) — a
   hash-chained JSONL file that records what the AI did and why. Each entry
   links to the previous, so history can't be silently rewritten.
4. Telemetry (opt-in) aggregates anonymised patterns so future AI suggestions
   improve over time.

### 1.3  Safe Extensibility

All custom scanners, codemods, and templates are **WebAssembly plugins**. They
run inside a `wazero` sandbox with an explicit capability allow-list — a plugin
that declares only `fs:read` literally cannot make a network call, regardless
of what its code contains. This is how Forge stays single-binary while still
being extensible.

---

## 2. Repository Layout

```
forge/
├── cmd/
│   ├── forge/          # Entry point — thin main() that calls internal/cli
│   └── gen-errors/     # Generator for docs/ERROR_CODES.md
├── internal/
│   ├── cli/            # Cobra command tree (one subdir per verb)
│   │   ├── cmdship/    # forge ship
│   │   ├── cmdscan/    # forge scan
│   │   ├── cmdnew/     # forge new
│   │   └── ...         # one cmd<verb>/ per verb
│   ├── plugin/         # Plugin ABI, manifest validation, wazero WASM runtime
│   ├── audit/          # Append-only hash-chained audit ledger
│   ├── codemod/        # Built-in codemods (gitignore-marker, gitleaks-baseline, …)
│   ├── eval/           # Scenario-based evaluation harness
│   ├── errcode/        # FORGE-XXXX error-code registry (panic on duplicate)
│   ├── logobs/         # slog wrapper with secret-redaction and --explain bypass
│   ├── manifest/       # .forge/manifest reader (scratch/managed sections)
│   ├── scaffold/       # forge new template engine
│   ├── telemetry/      # Opt-in span writer (ADR-006)
│   ├── llmbudget/      # Token-spend tracker
│   ├── incident/       # Incident lifecycle (ADR-021)
│   ├── failure/        # Failure-register data model (ADR-016)
│   └── verbmeta/       # Verb manifest registry (powers forge explain)
├── docs/               # Long-form design docs, ADRs, task trackers
├── tasks/              # Dev / Launch / Ops / Test / Arch task lists
├── CHECKLIST.md        # Pre-ship manual gate checklist
└── Makefile            # Developer convenience targets
```

### How to find the right file

| "I want to change how `forge X` behaves" | Go to `internal/cli/cmdX/` |
|---|---|
| "I want to add a new scanner" | `internal/cli/cmdscan/` + `internal/plugin/` |
| "I want to change the audit log format" | `internal/audit/` |
| "I want to understand error codes" | `internal/errcode/` + `docs/ERROR_CODES.md` |
| "I want to add a codemod" | `internal/codemod/` |
| "I want to add a new `forge new` template" | `internal/scaffold/` |

---

## 3. The `forge ship` Pipeline (The Critical Flow)

`forge ship` is the primary user-facing workflow.  Every other verb can be
thought of as a single stage of `ship` called in isolation.

```
forge ship --description "add rate-limiter middleware"
     │
     ├─ 1. SPEC      Write/validate .forge/specs/<slug>/spec.md
     ├─ 2. TEST      AI writes failing tests; committed before code
     ├─ 3. BREAKDOWN AI produces .forge/specs/<slug>/tasks.md
     ├─ 4. CODE      AI iterates until go test -race ./... is green
     └─ 5. SHIP      forge scan all + forge lint + forge eval → PR
```

The spec is the contract that gates every subsequent step.  If the spec is
missing or incomplete, `forge ship` exits with `FORGE-1600` before touching
any code.

---

## 4. Plugin Architecture

Plugins implement one of three Go interfaces defined in `internal/plugin/`:

| Kind | Interface | Purpose |
|------|-----------|---------|
| `scanner` | `Scanner` | Inspect code/config and return findings |
| `codemod` | `Codemod` | Rewrite files to fix a finding or apply an upgrade |
| `template` | `Template` | Scaffold files for `forge new` |

**In-process plugins** (built into the binary) register themselves at
`init()` time via `plugin.Default().Register(...)`.

**WASM plugins** are discovered from `.forge/plugins.json` at startup,
loaded by `wazero`, and wrapped in the same Go interface — callers cannot
tell the difference.

The capability sandbox is enforced at the WASM host level.  A plugin
that declares `capabilities = ["fs:read"]` cannot call `fd_write` even if its
source code tries to.

---

## 5. Cross-Cutting Concerns

| Concern | Package | Notes |
|---------|---------|-------|
| Structured logging | `internal/logobs` | `slog` JSON + TTY; redacts secrets; `--explain` unlocks prompt tracing |
| Error codes | `internal/errcode` | Every public error has a `FORGE-XXXX` code; duplicates panic at init |
| Config loading | cobra flags + env vars | Layered: defaults → env → flags (viper file loading planned) |
| Telemetry | `internal/telemetry` | Opt-in; file-based spans; `forge telemetry enable/disable/rotate-id` |
| Token budget | `internal/llmbudget` | Hard cap on LLM spend per run; `forge spend status` |

---

## 6. Next Steps

- **Build the project:** `make build` → `./bin/forge`
- **Run all tests:** `make test`
- **Add a feature:** read [CONTRIBUTING.md](../CONTRIBUTING.md) first
- **Understand a specific verb:** read [docs/VERBS.md](VERBS.md)
- **Write a plugin:** read [docs/PLUGIN_AUTHORING.md](PLUGIN_AUTHORING.md)
