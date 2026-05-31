# Spec: LLM-Driven Automation Testing — MCP Integration

## Status Summary

| Field | Value |
|-------|-------|
| Lifecycle | Draft |
| Version Scope | MINOR |
| Owner | platform-team |
| Last Updated | 2026-05-28 |
| Checkpoint Progress | 1/7 |
| Reference | [awesome-mcp-servers](https://github.com/punkpeye/awesome-mcp-servers) |

---

## What

Establish a **complete LLM-driven automated test suite** for both Forge CLI
and PromotAI SaaS, using MCP servers as connectable tools that allow the LLM
to generate tests, execute them, and report results — all within the same
`forge ship test` / CI loop.

Scope covers five layers:

1. **Browser/UI layer** — Playwright + WebdriverIO MCP for PromotAI E2E
2. **API/backend layer** — pytest + testcontainers for FastAPI/Supabase
3. **LLM quality layer** — promptfoo + LangSmith Evaluators for agents
4. **Security layer** — Semgrep/Trivy MCP for SAST/container scanning
5. **Terminal/CLI layer** — console-automation MCP + Forge eval harness

---

## Why

### Current problems

| Pain point | Consequence |
|-----------|-------------|
| LLM generates code without running tests | Bugs reach users before a regression guard exists |
| `forge bugfix` previously exited 0 on LLM failure (issue #18) | AI tools applied direct file edits, bypassing quality gates |
| PromotAI has no automated E2E tests | Every release requires manual QA |
| No LLM eval pipeline | Prompt degradation after model upgrades goes undetected |
| No DAST for staging | Security vulnerabilities only discovered after deploy |

### Expected benefits

- **80% reduction in manual QA time** by having the LLM generate and run test suites autonomously
- **Zero-regression releases**: every PR must pass all 5 test layers before merge
- **LLM quality drift detection**: catch degradation before it impacts users
- **Audit trail**: every test run is recorded in `forge audit` and LangSmith

---

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    forge ship test (checkpoint)              │
│   LLM receives: spec.md + code diff + existing tests         │
│   LLM generates: test stubs covering 9 test-design dimensions│
└────────────────┬────────────────────────────────────────────┘
                 │ MCP tool calls
    ┌────────────┼──────────────┬──────────────┐
    ▼            ▼              ▼              ▼
┌───────┐  ┌─────────┐  ┌──────────┐  ┌────────────┐
│Browser│  │API/DB   │  │LLM Eval  │  │Security    │
│Layer  │  │Layer    │  │Layer     │  │Layer       │
│       │  │         │  │          │  │            │
│MS     │  │pytest + │  │promptfoo │  │semgrep-mcp │
│Playwright│testcont.│  │langsmith │  │trivy       │
│MCP    │  │httpx    │  │evaluators│  │bandit      │
│       │  │RLS tests│  │ragas     │  │zap/nuclei  │
└───────┘  └─────────┘  └──────────┘  └────────────┘
    │            │              │              │
    └────────────┴──────────────┴──────────────┘
                 │
         forge scan security
         coverage gate ≥ 80%
         forge qa 33 cases
                 │
         CI green → PR merge
```

---

## Selected MCP Servers

### Layer 1 — Browser/E2E Automation

| MCP Server | Source | Role | Stack |
|-----------|--------|------|-------|
| `microsoft/playwright-mcp` | [github](https://github.com/microsoft/playwright-mcp) | Official Playwright — LLM interacts with web via accessibility snapshots | PromotAI E2E |
| `executeautomation/playwright-mcp-server` | awesome-mcp | Playwright for browser automation + web scraping | PromotAI E2E |
| `webdriverio/mcp` | [github](https://github.com/webdriverio/mcp) | Browser + mobile (Android/iOS Appium) via WebDriver protocol | Mobile smoke tests |
| `automaticabs/mcp-server-playwright` | awesome-mcp | Python Playwright MCP — well-suited for LangGraph agents | Python stack |
| `operative_sh/web-eval-agent` | awesome-mcp | Autonomously debugs web applications using browser-use agents | Staging debug |
| `serkan-ozal/browser-devtools-mcp` | awesome-mcp | Test, debug, and validate web apps via Chrome DevTools | Dev validation |

**Why `microsoft/playwright-mcp` is the primary choice:**
- Official Microsoft support → long-term maintenance
- Uses accessibility snapshots (not screenshots) → fewer tokens, deterministic results
- Integrates directly with `forge ship test` checkpoint via MCP tool call

### Layer 2 — API/Database Testing

| Tool | Role |
|------|------|
| `pytest` + `pytest-asyncio` | Unit + integration tests for FastAPI routes |
| `testcontainers-python` | Spin up real Postgres/Redis/Supabase in Docker |
| `httpx` / `TestClient` | API-level assertions with real request/response cycles |
| `hypothesis` | Property-based testing for billing and pricing logic |
| `factory_boy` + `faker` | Realistic test data generation |
| RLS policy tests (Supabase) | Verify row-level security isolation per tenant |

### Layer 3 — LLM Quality Evaluation

| Tool | Role |
|------|------|
| `promptfoo` | YAML-defined prompt regression suite, CI-native, multi-model |
| LangSmith Datasets + Evaluators | Golden-dataset eval for LangGraph agents |
| `ragas` | RAG pipeline quality (faithfulness, context recall) for pgvector |
| `trulens` | Tracing + grading LangGraph agents against a custom rubric |
| `avansaber/tailtest-cline` | Adversarial test generation — writes tests, runs them, classifies failures as `real_bug / environment / test_bug` |

**Why `promptfoo` is the top priority:**
- Zero infrastructure: runs locally and in CI with `promptfoo eval`
- Compares multiple models simultaneously (GPT-4o vs Claude vs Gemini)
- Built-in LLM-as-judge + custom rubric support
- Output: HTML report + JSON diff → easy to attach to PR comments

### Layer 4 — Security Testing

| Tool | Role | When |
|------|------|------|
| `gitleaks` | Secret scanning (already configured) | Pre-commit + CI |
| `semgrep` | Multi-language SAST (Go + Python + TS) | CI gate |
| `bandit` | Python-specific SAST | CI gate |
| `trivy` | Container CVE scan | Before push to registry |
| `nuclei` | Template-based DAST (staging only) | Post-deploy on staging |
| `religa/multi-mcp` | Multi-model OWASP Top 10 code review | PR review |

### Layer 5 — Terminal/CLI Testing (Forge-specific)

| MCP Server | Role |
|-----------|------|
| `ooples/mcp-console-automation` | 40 tools for session management, SSH, testing, monitoring — "Playwright for terminal" |
| `aybelatchane/mcp-server-terminal` | Playwright for terminals — TUI/CLI via Terminal State Tree |
| Forge eval harness (`internal/eval`) | JSON scenario → assert exit code/stdout/stderr for each verb |
| Forge QA suite (33 cases) | Pre-push smoke tests across all verbs |

---

## Acceptance Criteria

### AC-01 — Forge: LLM generates tests when `forge ship test` runs
```gherkin
Given a spec.md and code diff exist under .forge/specs/<feature>/
When  forge ship test is executed
Then  the LLM generates at least: 1 happy-path, 1 boundary, and 1 negative test case
And   the test file is saved to the correct package directory
And   go test compiles and runs all new tests without errors
```

### AC-02 — Forge: MCP Playwright tests execute in CI
```gherkin
Given microsoft/playwright-mcp is configured in .forge/mcp.yml
When  forge ship test runs with --mcp-layer=browser
Then  Playwright opens the PromotAI staging URL
And   all E2E scenarios in tests/e2e/ are executed
And   the command exits non-zero if any scenario fails
And   screenshots are saved to .forge/artifacts/screenshots/
```

### AC-03 — PromotAI: API tests use testcontainers
```gherkin
Given testcontainers-python is configured in conftest.py
When  pytest tests/integration/ is executed
Then  a Postgres container is spun up with schema migrations applied
And   Supabase RLS policies are verified: user A cannot read user B's data
And   all containers are cleaned up after the test run
And   coverage is ≥ 80% for all changed files
```

### AC-04 — PromotAI: promptfoo eval runs in CI
```gherkin
Given promptfoo.yml defines a golden dataset for each LangGraph agent
When  a prompt template is changed or a model is upgraded
Then  promptfoo eval runs automatically in CI
And   if score drops > 5% from baseline → CI fails with a detailed diff report
And   the report is attached to the PR comment
```

### AC-05 — Security gate cannot be bypassed
```gherkin
Given a PR containing new code
When  forge scan security runs
Then  if semgrep/bandit detects HIGH severity findings → exit non-zero
And   trivy scans the container image before pushing to the registry
And   no secret passes the gitleaks check
```

### AC-06 — LLM test-gen covers all 9 dimensions
```gherkin
Given forge ship test receives a spec describing a new feature
When  the LLM generates a test suite
Then  the suite covers at least 7 of the 9 dimensions:
      happy-path, boundary, negative, idempotency, concurrency,
      cross-tenant/authz, backward-compat, data-accuracy, false-positive-guard
And   each dimension is tagged in the test comment
```

### AC-07 — Forge eval harness: no regressions broken
```gherkin
Given a new commit to internal/cli/cmd*/
When  go test ./tests/task_tests/ runs
Then  all 7 scenario JSON files pass (ship-flow, auth-flow, migration, etc.)
And   no scenario exits with a code different from its expectation
```

---

## Non-Functional Requirements

- **CI speed**: Total test suite runtime (excluding DAST) ≤ 10 minutes
- **Parallel execution**: Browser E2E tests run on at least 4 parallel workers
- **Reproducibility**: Test results are deterministic (seeded faker, mocked external APIs)
- **Observability**: Every test run is traced via OpenTelemetry → Grafana
- **Cost cap**: promptfoo eval must not exceed $2/run (use gpt-4o-mini as judge)
- **Isolation**: testcontainers must not share state between test cases

---

## Implementation Plan

### Phase 1 — Foundation (Sprint 1)
1. Configure `microsoft/playwright-mcp` in `.forge/mcp.yml`
2. Scaffold `tests/e2e/` directory with Playwright config targeting PromotAI staging
3. Add `testcontainers-python` + `conftest.py` fixtures to the PromotAI backend
4. Add RLS isolation tests for Supabase (user A cannot access user B's data)

### Phase 2 — LLM Quality Gate (Sprint 2)
5. Create `promptfoo.yml` with a golden dataset for 3 LangGraph agents
6. Integrate promptfoo eval into GitHub Actions CI
7. Write LangSmith evaluators for agent output quality
8. Capture baseline scores for all agents

### Phase 3 — Security Hardening (Sprint 3)
9. Add semgrep rules for Python FastAPI (SQL injection, auth bypass)
10. Configure trivy in the Docker build CI pipeline
11. Set up nuclei DAST scan for the staging environment
12. Integrate `religa/multi-mcp` into the PR review workflow

### Phase 4 — Forge Test-Gen Integration (Sprint 4)
13. Update `forge ship test` checkpoint prompt to generate tests covering 9 dimensions
14. Integrate `ooples/mcp-console-automation` for terminal-level testing
15. Expand `internal/eval/scenarios/` with 5 new scenarios (bugfix, heal, explain, etc.)
16. Add Grafana panels for test coverage trend and promptfoo score trend

---

## Out of Scope

- Load testing / performance benchmarking (separate spike)
- Mobile app testing on physical devices (WebdriverIO emulator only)
- Penetration testing / red team exercises (manual security review)
- Test data management for the production database
- A/B testing framework for PromotAI features

