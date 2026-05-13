# forge context

Generate project context for LLM prompts.

## Synopsis

```
forge context generate [--root <path>] [--out <file>] [--format json|md]
forge context show     [--root <path>]
```

## Description

`forge context generate` collects project metadata (language, dependencies,
forge config, recent commits) and formats it as a structured context block
for inclusion in LLM prompts. This is used automatically by other forge verbs.

## Examples

```bash
forge context generate
forge context generate --format md --out context.md
forge context show
```
