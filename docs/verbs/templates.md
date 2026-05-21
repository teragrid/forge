# forge templates

Browse available community templates and the enterprise module catalogue.

## Synopsis

```
forge templates list [--json]
```

## Description

`forge templates list` prints every template and enterprise module Forge knows about, grouped by mode (`classic` or `tsd`). Use this to discover what you can scaffold before running `forge new`.

**Classic-mode templates** are fixed, opinionated stacks — pick one by name and Forge generates a complete project.

**TSD-mode templates** are community blueprints that pair with a `.forge/tsd.yml` file. Forge's knowledge base (172 entries covering reference architectures, compliance standards, and best practices) drives module selection and composition.

## Sub-commands

### `forge templates list`

```bash
forge templates list           # human-readable table
forge templates list --json    # machine-readable JSON
```

**Example output:**

```
ID                        MODE     TAGS                              DESCRIPTION
enterprise-cloud-native   tsd      enterprise, saas, tsd, community  TSD-driven enterprise SaaS scaffold (multi-tenant, RBAC, audit-log, feature-flags)
go-cloud-native           tsd      go, cloud-native, gcp             Go + Chi + Neon + GCP cloud-native service
marketplace-platform      tsd      marketplace, payments, nextjs     Next.js + Go + Adyen marketplace with multi-tenant payments
data-platform             tsd      data, python, dbt                 Python + FastAPI + dbt + Metabase data platform
ts-service                classic  typescript, vitest                TypeScript + Vitest + Forge CI gates
next-app                  classic  nextjs, react, tailwind           Next.js 14, TypeScript, Tailwind CSS, App Router, Vitest, Playwright
go-service                classic  go, http, healthz                 Go HTTP service with graceful shutdown, /healthz, /readyz
```

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `--json` | `false` | Emit machine-readable JSON |

## Using a template

**Classic mode** — pass the template ID directly to `forge new`:

```bash
forge new ts-service my-app
forge new go-service my-api --module github.com/yourname/my-api
```

**TSD mode** — create a TSD file first, then scaffold:

```bash
forge tsd init                    # writes .forge/tsd.yml
forge new "billing service"       # reads .forge/tsd.yml automatically
```

## Enterprise module catalogue

Enterprise modules are the building blocks Forge composes when running in TSD mode. Each module covers one concern. Examples:

| Module ID | What it provides |
|-----------|-----------------|
| `core/rbac` | Role-based access control with row-level security |
| `core/audit-log` | Tamper-proof audit ledger |
| `core/feature-flags` | Runtime feature flag system |
| `frontend/nextjs-15-supabase` | Next.js 15 + Supabase auth wired together |
| `backend/go-chi-neon` | Go + Chi router + Neon serverless Postgres |
| `payments/stripe-subscriptions` | Stripe subscription billing with webhooks |
| `ai/openai-agents` | OpenAI Agents SDK integration with guardrails |
| `infra/gcp-cloud-run` | Cloud Run deployment manifests and CI pipeline |
| `observability/datadog` | Datadog APM, logs, and metrics wired to the app |

The full module catalogue is driven by Forge's built-in knowledge base. Modules are selected automatically based on your `.forge/tsd.yml` choices.

## See also

- [forge tsd](tsd.md) — create and lint the TSD blueprint
- [forge new](new.md) — scaffold using a template or TSD file
- [PLUGIN_AUTHORING.md](../PLUGIN_AUTHORING.md) — how to publish your own template
