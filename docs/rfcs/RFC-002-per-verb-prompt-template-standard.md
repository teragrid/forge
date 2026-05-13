# RFC-002 — Per-Verb Prompt Template Standard

| Field | Value |
|-------|-------|
| RFC | 002 |
| Title | Per-Verb Prompt Template Standard |
| Author | Forge Maintainers |
| Status | **Accepted** |
| Created | 2025-02-10 |
| Implemented in | `internal/prompttemplates` |

---

## Summary

Standardise how each Forge verb constructs its LLM prompts by introducing a
`prompttemplates` package that holds one system + user template pair per verb.
This makes prompt engineering reviewable, testable, and overridable.

---

## Motivation

Without a standard, each verb constructs its own ad-hoc prompt strings, making
it impossible to:

- Review prompt changes in PRs (they're buried in Go code).
- Override prompts for specific projects (no extension point).
- Measure prompt quality systematically (no eval harness hook).
- Localise prompts for non-English projects.

---

## Design

### Template format

Each verb has two `text/template` strings:
- `system` — the LLM system prompt (persona, instructions, output format).
- `user` — the user-turn prompt (renders `{{.UserInput}}`, `{{.Context}}`, etc.).

Templates are registered in `internal/prompttemplates/prompttemplates.go`.

### Override mechanism

Project-specific overrides live in `.forge/prompts/<verb>.system.tmpl` and
`.forge/prompts/<verb>.user.tmpl`. The loader checks for overrides first,
falling back to the built-in templates.

### Eval integration

The eval harness (`internal/eval`) can run prompt templates through reference
scenarios to detect regressions in LLM output quality.

---

## Alternatives considered

- **Separate `.tmpl` files embedded via `//go:embed`** — considered but rejected for v1.0 in favour of keeping everything in one Go file for reviewability. Will revisit in M3.
- **Jinja2-style templates** — rejected; adds a non-Go dependency.

---

## Decision

Accepted. Implemented in `internal/prompttemplates/prompttemplates.go`.
Override support is planned for M3.2.
