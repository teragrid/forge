# forge new

Scaffold a new AI-augmented project from a template or a Tech Stack Decision (TSD) blueprint.

## Synopsis

```
# Classic mode — pick a built-in template by name
forge new <template> <path> [--module <mod>] [--force]

# TSD mode — describe the feature; Forge reads .forge/tsd.yml automatically
forge new "<description>" [--name <name>] [--force]

# TSD mode — explicit TSD file
forge new --tsd <file> "<description>" [--name <name>] [--force]

# List available templates
forge new --list
```

## Mode selection

| Condition | Mode activated |
|-----------|---------------|
| `--tsd <file>` flag present | TSD mode (explicit) |
| `.forge/tsd.yml` exists in current directory | TSD mode (auto-detected) |
| Neither condition | Classic mode |

## Templates (classic mode)

| Template | Language | What you get |
|---|---|---|
| `go-service` | Go | HTTP API with Forge CI gates, JWT auth, SQL migrations |
| `ts-service` | TypeScript | Node.js service with multi-tenant RLS, JWT, vitest |
| `next-app` | TypeScript | Next.js 14, Tailwind CSS, App Router, Vitest, Playwright |
| `langchain-agents` | Python | Multi-agent system: LangGraph + LangChain + LangServe + LangSmith |
| `regulated` | Go / TS | Compliance scaffolds: `regulated/soc2`, `regulated/hipaa`, `regulated/finserv` |

### `langchain-agents` — multi-agent system

Scaffolds a production-grade multi-agent system with:

- **LangGraph** `StateGraph` for agent orchestration (supervisor → researcher → executor)
- **LangChain** tools (web search via Tavily, vector store retrieval)
- **LangServe** FastAPI endpoint with playground UI at `/agents/playground`
- **LangSmith** tracing wired in from day one
- **pgvector** long-term memory with tenant isolation
- **Prompt-injection guardrail** (`src/shared/guardrails.py`) + security test suite
- **Forge conventions** for agent code: idempotent nodes, state-only channels, async throughout

## TSD mode

When a `.forge/tsd.yml` blueprint is present (or `--tsd` is set), `forge new` reads the file, resolves the module list via the built-in knowledge base, and composes the matching scaffold modules into the target directory.

```bash
# Create the blueprint first
forge tsd init

# Then scaffold — TSD file is detected automatically
forge new "campaign analytics service"

# Or point at any TSD file explicitly
forge new --tsd path/to/my-stack.tsd.yml "checkout API"
```

Use `forge templates list` to browse available community blueprints and enterprise modules.

## Examples

```bash
# Classic mode
forge new ts-service my-app
forge new next-app my-app
forge new go-service my-app --module github.com/yourname/my-app
forge new go-service .      --module github.com/yourname/my-app --force
forge new my-agents --template langchain-agents
forge new my-saas --template ts-service --dry-run

# TSD mode (auto-detect)
forge new "billing dashboard"

# TSD mode (explicit file)
forge new --tsd infra/stack.tsd.yml "payment processing"
```

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--tsd` | `""` | Path to a TSD file; activates TSD mode |
| `--name` | `""` | Override the project name (default: basename of target path) |
| `--module` | `""` | Go module path (required for `go-service` in classic mode) |
| `--force` | `false` | Overwrite existing files |
| `--list` | `false` | List available templates and exit |
| `--json` | `false` | Emit machine-readable JSON |

## Error codes

- `FORGE-1100` — invalid usage (unknown template or missing required flag)
- `FORGE-1101` — target directory not empty and `--force` not set

## See also

- [forge tsd](tsd.md) — create and lint the TSD blueprint
- [forge templates](templates.md) — browse available templates and modules
- [PLUGIN_AUTHORING.md](../PLUGIN_AUTHORING.md)
