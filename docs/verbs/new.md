# forge new

Scaffold a new AI-augmented project from a template.

## Synopsis

```
forge new <name> [--template <name>] [--root <path>] [--dry-run]
```

## Templates

| Template | Language | What you get |
|---|---|---|
| `go-service` | Go | HTTP API with Forge CI gates, JWT auth, SQL migrations |
| `ts-service` | TypeScript | Node.js service with multi-tenant RLS, JWT, vitest |
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

## Examples

```bash
forge new my-service --template go-service
forge new my-agents --template langchain-agents
forge new my-saas --template ts-service --dry-run
```

## See also

- [PLUGIN_AUTHORING.md](../PLUGIN_AUTHORING.md)
