# Spec: langchain-agents scaffold template

> **Slug**: `langchain-agents-template`
> **Status**: draft
> **Created**: 2026-05-19

## Feature

Add a `langchain-agents` scaffold template to `forge new`.  When a user runs
`forge new <project> --template langchain-agents`, Forge generates a
production-grade multi-agent Python project using LangGraph, LangChain,
LangServe, and LangSmith.

---

## Context & motivation

Teams building LLM-powered applications need a repeatable, secure starting
point that:
- wires in observability (LangSmith) from day one,
- enforces tenant isolation in memory (pgvector workspace_id filter),
- includes a prompt-injection guardrail and security test suite,
- integrates with `forge scan security` and `forge scan prompt-injection` CI gates,
- follows all Forge scaffold conventions (AGENTS.md, .forge/conventions.json,
  .forge/instructions/).

---

## Acceptance criteria

- [ ] `forge new my-agents --template langchain-agents` exits 0 and creates the
      expected directory tree.
- [ ] The generated project passes `forge doctor` with no errors.
- [ ] Running `go test ./internal/scaffold/...` continues to pass after the
      template is added.
- [ ] `forge scan security` in the generated project finds no critical
      vulnerabilities in the scaffold files.
- [ ] `forge scan prompt-injection` detects all injection surfaces and reports
      green for the included guardrail.
- [ ] The `.github/workflows/ci.yml` template includes jobs: `typecheck`,
      `lint`, `test`, and `security`.
- [ ] All Python package directories include `__init__.py` so the project is
      importable without PYTHONPATH tricks.
- [ ] The test suite in `tests/` passes with `pytest -x` on a fresh checkout
      (mocked LLM calls, no API keys required).
- [ ] `ROLLBACK.md` provides working SQL to query LangGraph checkpoints.
- [ ] `forge ship` on a feature added to a `langchain-agents` project correctly
      extracts `agents`, `tools`, and `memory_backend` fields into `spec.yml`.

---

## Out of scope

- Production-ready LLM provider connectors beyond OpenAI and Anthropic.
- MCP (Model Context Protocol) integration.
- UI / front-end scaffolding.
- Kubernetes deployment manifests (covered by a separate `k8s-deploy` template).
- Fine-tuning or model training pipelines.

---

## Architecture

```
forge new my-agents --template langchain-agents
       │
       └─ internal/scaffold.Render("langchain-agents", ...)
                │
                ├─ pyproject.toml
                ├─ forge.config.yml          ← agent/llm/memory/security config
                ├─ docker-compose.yml        ← app + pgvector/pgvector:pg16 + redis
                ├─ AGENTS.md                 ← universal AI-agent entry point
                ├─ .forge/conventions.json   ← 12 binding conventions
                ├─ .forge/instructions/      ← global + agents instruction files
                ├─ .github/workflows/ci.yml  ← 4-job CI pipeline
                └─ src/
                    ├─ server.py             ← LangServe FastAPI
                    ├─ graph/
                    │   ├─ state.py          ← AgentState TypedDict
                    │   └─ workflow.py       ← StateGraph + checkpointing
                    ├─ agents/
                    │   ├─ orchestrator.py   ← supervisor
                    │   ├─ researcher.py     ← vector store + web search
                    │   └─ executor.py       ← task execution
                    ├─ shared/
                    │   ├─ config.py         ← pydantic-settings
                    │   ├─ errors.py         ← typed error hierarchy
                    │   ├─ guardrails.py     ← validate_input() + PromptInjectionError
                    │   ├─ llm.py            ← get_llm() factory
                    │   └─ logging.py        ← structlog JSON
                    ├─ memory/
                    │   └─ vector_store.py   ← pgvector / Chroma abstraction
                    └─ tools/
                        └─ web_search.py     ← Tavily wrapper
```

---

## Plan & tasks

### Phase 0 — Template structure (done)
- [x] Create `internal/scaffold/templates/langchain-agents/` directory tree.

### Phase 1 — Root config files (done)
- [x] `forge.config.yml.tmpl` — agents/llm/memory/observability/security config
- [x] `pyproject.toml.tmpl` — Python packaging, deps, ruff/mypy/pytest config
- [x] `README.md.tmpl` — ASCII diagram + quick start + env vars table
- [x] `docker-compose.yml.tmpl` — app + pgvector + redis + airgap profile
- [x] `.env.example.tmpl`
- [x] `.gitleaks.toml.tmpl`
- [x] `.gitignore.tmpl`
- [x] `ROLLBACK.md.tmpl`
- [x] `AGENTS.md.tmpl`, `CLAUDE.md.tmpl`, `.cursorrules.tmpl`, `.windsurfrules.tmpl`

### Phase 2 — Forge integration files (done)
- [x] `.forge/conventions.json.tmpl` — 12 binding conventions
- [x] `.forge/hygiene.yml.tmpl`
- [x] `.forge/instructions/global.instructions.md.tmpl`
- [x] `.forge/instructions/agents.instructions.md.tmpl`

### Phase 3 — CI pipeline (done)
- [x] `.github/workflows/ci.yml.tmpl` — typecheck + lint + test + security jobs

### Phase 4 — Python source (done)
- [x] `src/__init__.py`, `src/agents/__init__.py`, `src/graph/__init__.py`
- [x] `src/shared/__init__.py`, `src/memory/__init__.py`, `src/tools/__init__.py`
- [x] `src/server.py.tmpl`
- [x] `src/graph/state.py.tmpl`, `src/graph/workflow.py.tmpl`
- [x] `src/agents/orchestrator.py.tmpl`, `researcher.py.tmpl`, `executor.py.tmpl`
- [x] `src/shared/config.py.tmpl`, `errors.py.tmpl`, `logging.py.tmpl`
- [x] `src/shared/guardrails.py.tmpl`, `llm.py.tmpl`
- [x] `src/memory/vector_store.py.tmpl`
- [x] `src/tools/web_search.py.tmpl`

### Phase 5 — Test suite (done)
- [x] `tests/__init__.py`, `tests/security/__init__.py`
- [x] `tests/test_orchestrator.py.tmpl`
- [x] `tests/test_workflow.py.tmpl`
- [x] `tests/security/test_prompt_injection.py.tmpl`

### Phase 6 — Documentation (done)
- [x] `docs/verbs/new.md` — add langchain-agents to Templates table
- [x] `docs/spec-schema.md` — add Python/multi-agent spec extraction rules + example

### Phase 7 — Spec files (this file)
- [x] `internal/cli/.forge/specs/langchain-agents-template/spec.md`
- [x] `internal/cli/.forge/specs/langchain-agents-template/spec.yml`

---

## Security considerations

- All user input enters `validate_input()` in `src/shared/guardrails.py` before
  any LLM call — enforced by convention `injection-tests-for-prompts`.
- Secrets are never hardcoded; `.gitleaks.toml` and `forge scan secrets` gate CI.
- `workspace_id` is enforced as a metadata filter on all pgvector queries —
  convention `workspace-id-filter`.
- The `forge scan prompt-injection` CI job runs against all system prompts and
  user-input boundaries automatically.

---

## References

- [LangGraph docs](https://langchain-ai.github.io/langgraph/)
- [LangChain docs](https://python.langchain.com/)
- [LangSmith docs](https://docs.smith.langchain.com/)
- [LangServe docs](https://python.langchain.com/docs/langserve/)
- `internal/scaffold/scaffold.go`
- `docs/spec-schema.md`
- `AGENTS.md` (repo root)
