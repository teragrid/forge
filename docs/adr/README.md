# Architecture Decision Records (ADR) & Design Architecture Briefs (DAB)

This directory contains:

- **ADR-TEMPLATE.md** — Lightweight ADR format for single architecture decisions.
- **dab-full/** — Full Design Architecture Brief (9 sections) for large-scale features.
- **dab-light/** — Condensed DAB for medium features; same sections with reduced detail.

## When to use each

| Change size | Template |
|---|---|
| Small (< 200 LOC, single package) | `ADR-TEMPLATE.md` |
| Medium (multi-package, new API surface) | `dab-light/` |
| Large (new subsystem, cross-cutting, or external integration) | `dab-full/` |

## How `forge ship arch` uses these

The `arch` checkpoint (step 2 of 6 in `forge ship`) runs a 3-round, 6-role
self-debate and writes the output to `.forge/specs/<slug>/arch.md`.

The LLM prompt for that checkpoint embeds the appropriate DAB template as
scaffolding, ensuring the output covers all required sections before being
consumed by the downstream `test`, `breakdown`, `code`, and `ship` checkpoints.
