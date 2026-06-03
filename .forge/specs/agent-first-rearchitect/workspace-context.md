# Workspace Context Snapshot
Generated: 2026-06-02T16:24:09Z

## Tech Stack
- GitHub Actions CI
- Go module (github.com/teragrid/forge, go 1.25.0)
- Make

## Project Structure
- bin/, cmd/, dist/, docs/, forge-knowledge/, internal/, packages/, private/, scripts/, tests/

## Recent Changes (last 10 commits)
```
8b94027 fix(cmdship): resolve pre-push lint gate for RFC-005 P3 files
cb00b47 Merge branch 'feature/ship-hooks-learning-loop' into main
c7c558c feat(cmdship): RFC-005 P3 — PII filter, A/B steering, drift detect, compound checkpoints, immutable audit, incremental re-run
6db2fe7 feat(cmdship): RFC-005 §6 test phase quality framework — merge to main
8a4f6ac feat(cmdship): test phase quality framework P1+P2 (RFC-005 §6)
65cac00 Merge pull request #21 from teragrid/fix/error-codes-spec-tracking
dd9d040 fix: track spec files in git and regenerate ERROR_CODES.md
27b2c41 fix: track spec files in git and regenerate ERROR_CODES.md
7830663 Merge pull request #20 from teragrid/dependabot/go_modules/go-modules-2624566978
fb1c99e Merge pull request #19 from teragrid/feature/ship-hooks-learning-loop
```

## Existing Feature Specs (avoid duplicates)
- agent-first-rearchitect, explain, forge-ship-v2-rfc-005-p1-p2-token-efficiency-and-context-bud, llm-driven-automation-testing-mcp-integration, test-phase-quality-framework

## Project Conventions
(from AGENTS.md) > This file provides context and instructions for AI coding assistants working > in the Forge repository. It is read automatically by GitHub Copilot, Claude, > Cursor, Windsurf, and other tools that support AGENTS.md / CLAUDE.md conventions. --- **Forge** is a single-binary Go CLI that bundles the scan-fix-learn loop, LLM gateway, plugin runtime, and ship workflow for AI-generated code. Module path: `github.com/teragrid/forge`. --- ``` cmd/forge/          — main entry point (thin wrapper aroun [truncated]

