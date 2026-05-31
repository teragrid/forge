# Workspace Context Snapshot
Generated: 2026-05-28T16:57:50Z

## Tech Stack
- GitHub Actions CI
- Go module (github.com/teragrid/forge, go 1.24.0)
- Make

## Project Structure
- bin/, cmd/, dist/, docs/, forge-knowledge/, internal/, packages/, private/, scripts/, tests/

## Recent Changes (last 10 commits)
```
05285a2 fix: forge bugfix exits non-zero on LLM failure (closes #18)
ef163cc fix: forge help command groups empty — use \ template variable
0e56c63 feat: v1.5.0 — RFC-005 P1/P2 complete + forge companion + command groups
13fd1a5 fix(clean): merge hygiene.yml patterns and add manifest sync
510fa39 fix(ship): write checkpoint completion markers; P1 parallel arch debate + DAG pipeline
7c324bd fix(ship): wire digest into post-checkpoint path; suppress unused lint on buildDigestContext/readCheckpointDigest
f6afdbb feat(ship): RFC-005 P1+P2 - digest, domain profiles, snapshots, adaptive budget
7f8321b fix(ship): wire classifyComplexity into ShipResult.Complexity field
42b7c9e chore: RFC-005 move (deleted from docs/rfcs/) + forge-ship artefacts from pipeline run
b9c9622 feat(ship): implement RFC-005 P1+P2 — L1/L3/L4/L5/L7 + P0 KB budget fix
```

## Existing Feature Specs (avoid duplicates)
- explain, forge-ship-v2-rfc-005-p1-p2-token-efficiency-and-context-bud

## Project Conventions
(from AGENTS.md) > This file provides context and instructions for AI coding assistants working > in the Forge repository. It is read automatically by GitHub Copilot, Claude, > Cursor, Windsurf, and other tools that support AGENTS.md / CLAUDE.md conventions. --- **Forge** is a single-binary Go CLI that bundles the scan-fix-learn loop, LLM gateway, plugin runtime, and ship workflow for AI-generated code. Module path: `github.com/teragrid/forge`. --- ``` cmd/forge/          — main entry point (thin wrapper aroun [truncated]

