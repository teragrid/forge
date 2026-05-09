# ADR-007 — Spec format

- **Status:** Proposed
- **Tracker:** ARCH-DEC-07
- **Spec/Arch anchor:** Arch §13 ADR-007, Spec §2 (spec-driven workflow), Spec §9 (structured spec parsing)
- **Decision date:** TBD
- **Deciders:** Founder
- **Consulted:** Core engineering, product

## Context

Specs are the source of truth Forge consumes for `forge plan`/`ship`. They must:

- Be human-readable and diff-friendly.
- Carry machine-parseable metadata (status, owner, version, anchors).
- Embed structured fragments (acceptance criteria, schemas) without forcing a separate file per concept.
- Be authorable + revisable by both humans and LLMs.

## Decision

Specs will be **Markdown documents with a YAML frontmatter block** (`---` fence at the top of the file). The frontmatter carries machine-readable fields; the Markdown body carries the prose. A new file format `spec.md` is identical to `*.md` but is linted by `forge spec lint`.

### Required frontmatter fields

```yaml
---
api_version: forge.sh/v1
kind: Spec
id: SPEC-NNNN              # immutable
title: <short title>
status: draft | accepted | superseded | deprecated
owner: <github-handle-or-team>
anchors:
  arch: ["§3", "§16.5"]
  tasks: ["DEV-M1-04", "TEST-07"]
version: 0.1.0
created: YYYY-MM-DD
updated: YYYY-MM-DD
---
```

### Required body sections (lint-enforced)

1. `## Context`
2. `## Decision` (or `## Behaviour` for non-decision specs)
3. `## Acceptance criteria` — bulleted, each line prefixed `AC-NN:`
4. `## Out of scope`
5. `## Open questions`

Embedded YAML/JSON schema fragments use ` ```yaml ` / ` ```json ` fenced blocks with a leading `# $schema: …` comment so the linter can extract them.

## Alternatives considered

### Option A — Pure YAML / TOML spec files (rejected)

Pros: trivial to parse.
Cons: hostile to long-form prose; bad for diffs of multi-paragraph rationale; LLMs author worse YAML than Markdown.

### Option B — AsciiDoc (rejected)

Pros: better cross-references than Markdown.
Cons: poor LLM training-data coverage; smaller tooling ecosystem; GitHub renders Markdown natively.

### Option C — reStructuredText (rejected)

Pros: directives + roles for structured fragments.
Cons: same LLM/tooling concerns as AsciiDoc; whitespace-sensitive in ways that confuse contributors.

## Consequences

### Positive

- Renders natively on GitHub, GitLab, and most editors.
- Frontmatter is parseable in any language with `yaml`/`gopkg.in/yaml.v3`/`serde_yaml`.
- LLMs handle Markdown extremely well — minimal authoring friction.
- One file per spec; embedded schemas don't fragment the corpus.

### Negative / accepted trade-offs

- Frontmatter drift from body needs a linter (e.g. `updated` vs git log). The linter is part of M0 (DEV-M0-19).
- No native cross-file references — handled by the linter resolving anchors against the workspace.

### Follow-ups created

- DEV-M0-19 — `forge spec lint` (frontmatter schema + body sections + anchor resolution).
- DEV-M1-04 — spec parser library.
- TEST-07 — spec corpus + lint regression suite.

## Compliance hooks

- CI gate: `forge spec lint` runs on every PR touching `*.md` under `spec/` or `forge/`.
- Test: parse round-trip — frontmatter → struct → frontmatter is byte-identical (TEST-07).
- Lint: missing required section fails with file:line.

## References

- Arch §13 ADR-007.
- Hugo / Jekyll frontmatter conventions (prior art).
