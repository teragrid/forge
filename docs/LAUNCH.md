# Forge — Launch Post

> **Draft** — Target publish date: v1.0.0 GA

---

## Introducing Forge: The LLM-Native Ship Workflow for AI-Generated Code

After months of building in public, we're excited to announce **Forge v1.0.0**
— a single-binary CLI that brings discipline, safety, and observability to
AI-assisted software development.

---

### The problem we're solving

Every team using AI coding assistants faces the same challenges:

- **Secrets leaking into generated code** — LLMs confidently produce plausible-looking but dangerous output.
- **No systematic review** — AI suggestions bypass the code-review discipline that humans have built over decades.
- **Token sprawl** — LLM costs grow unbounded without budgets and caching.
- **"It worked in dev"** — AI-generated code often fails on edge cases, auth boundaries, and production constraints.

Forge wraps the entire AI-assisted workflow — from spec to ship — in a set of
opinionated, auditable checkpoints.

---

### What Forge does

```
forge ship           # runs 5 checkpoints: spec → test → breakdown → code → ship
forge scan all       # 14 scanner families: secrets, RLS, prompt-injection, supply-chain, cost, …
forge fix --apply    # LLM-powered auto-fix with confidence tiers
forge review         # structured LLM PR review with JSON output
forge ask            # project-aware Q&A against your codebase
forge adopt          # scaffold forge workflow into any existing project
forge doctor         # diagnose misconfiguration and drift
```

---

### Key design decisions

1. **Single binary, zero CGO** — ships for Linux/macOS/Windows via Brew, Scoop, and winget.
2. **Plugin runtime** — WASM-isolated scanners and codemods that can't access your filesystem unless explicitly granted.
3. **Cheap-first LLM routing** — T0 (local/fast) → T1 (mid) → T2 (frontier) escalation keeps costs low.
4. **Audit ledger** — hash-chained append-only log of every LLM call and file mutation.
5. **Reversibility** — `forge undo` rolls back any forge-managed change within the configured window.

---

### Getting started

```bash
# macOS
brew install teragrid/tap/forge

# Windows
scoop install forge

# Linux
curl -fsSL https://get.forge.dev | sh

# Verify
forge doctor
```

Full installation guide: [docs/INSTALLATION.md](docs/INSTALLATION.md)

---

### What's next

- **M2 — Ecosystem**: Plugin registry, deploy adapters, eval harness, learning loop.
- **M3 — Quality & Launch**: i18n, T2 adapter coverage, two-key enforcement, regulated-industry templates.

Follow us on GitHub: [github.com/teragrid/forge](https://github.com/teragrid/forge)

Star the repo if Forge is useful to your team. Feedback and contributions are
welcome — see [CONTRIBUTING.md](CONTRIBUTING.md).

---

*The Forge Team*
