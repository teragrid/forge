# forge tsd

Manage the Tech Stack Decision (TSD) file — a `.forge/tsd.yml` blueprint that records every architectural choice for your project before scaffolding runs.

## Synopsis

```
forge tsd init     [--force] [--json]
forge tsd validate [--tsd <file>] [--json]
```

## Why a TSD file?

When you build a complex project, dozens of architectural decisions happen implicitly — often scattered across README notes, Slack messages, or just in someone's head. The TSD file makes those decisions explicit and machine-readable:

- **Frontend framework** — `nextjs-15-supabase`, `react-vite`, `none`
- **Backend language** — `go`, `typescript`, `python`
- **Database** — `postgresql`, `neon`, `sqlite`
- **Auth provider** — `supabase`, `auth0`, `clerk`, `none`
- **Payments** — `stripe`, `adyen`, `none`
- **AI layer** — `openai`, `anthropic`, `none`
- **Infra target** — `gcp-cloud-run`, `vercel`, `fly-io`, `none`
- **Observability** — `datadog`, `opentelemetry`, `none`

`forge new` reads this file and composes the exact matching modules into a production-grade scaffold. Every module covers one concern; Forge merges them cleanly and resolves file conflicts automatically.

## Sub-commands

### `forge tsd init`

Interactive wizard that writes (or overwrites) `.forge/tsd.yml`.

```bash
forge tsd init              # interactive prompts → writes .forge/tsd.yml
forge tsd init --force      # overwrite an existing .forge/tsd.yml
forge tsd init --json       # emit machine-readable JSON summary after writing
```

The wizard asks ~10 questions (takes under 2 minutes). You can edit the resulting YAML by hand — run `forge tsd validate` afterwards to confirm it's still valid.

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `--force` | `false` | Overwrite an existing TSD file without prompting |
| `--json` | `false` | Emit a machine-readable JSON summary after writing |

**Example `.forge/tsd.yml` output:**

```yaml
version: v1
project_type: saas
frontend: nextjs-15-supabase
backend: go
database: postgresql
auth: supabase
payments: stripe
ai_layer: openai
infra: gcp-cloud-run
observability: datadog
```

### `forge tsd validate`

Lint a TSD file against the v1 schema. Exits non-zero if required fields are missing or unknown keys are present. Prints warnings for keys Forge doesn't recognise (but doesn't fail on warnings alone).

```bash
forge tsd validate                         # validates .forge/tsd.yml
forge tsd validate --tsd my-stack.yml     # validate a specific file
forge tsd validate --json                  # machine-readable output
```

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `--tsd` | `.forge/tsd.yml` | Path to the TSD file to validate |
| `--json` | `false` | Emit machine-readable JSON |

**Example output (valid):**

```
✓ TSD file is valid  (.forge/tsd.yml, v1, 9 keys)
```

**Example output (invalid):**

```
✗ FORGE-6500  missing required field: backend
  hint: set `backend` to one of: go, typescript, python
```

## Typical workflow

```bash
# 1. Create the TSD blueprint
forge tsd init

# 2. (Optional) Review / edit by hand
#    edit .forge/tsd.yml

# 3. Validate before scaffolding
forge tsd validate

# 4. Browse available templates
forge templates list

# 5. Scaffold — reads .forge/tsd.yml automatically
forge new "campaign analytics service"
```

## Error codes

| Code | Meaning |
|------|---------|
| `FORGE-6500` | Invalid usage of `forge tsd` |
| `FORGE-6501` | Failed to write TSD file |

## See also

- [forge new](new.md) — scaffold using a TSD blueprint or classic template
- [forge templates](templates.md) — browse available modules and community blueprints
