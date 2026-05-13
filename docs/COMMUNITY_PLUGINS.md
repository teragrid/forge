# Community Plugins

> First-party and community-maintained plugins for Forge.
> Install any plugin with `forge plugin install <name>`.

---

## What is a Forge plugin?

A Forge plugin is a WASM binary (or Go in-process module) that implements one
of the four plugin interfaces:

| Interface | Purpose |
|-----------|---------|
| `Scanner` | Adds findings to `forge scan` |
| `Codemod` | Transforms code via `forge fix` |
| `Provider` | Wraps an LLM or external API |
| `Template` | Adds scaffold templates to `forge new` |

See [docs/PLUGIN_AUTHORING.md](PLUGIN_AUTHORING.md) for the full authoring guide.

---

## First 3 community plugins

### 1. `forge-scanner-ratelimit`

**Kind:** Scanner  
**Author:** community  
**Install:** `forge plugin install forge-scanner-ratelimit`

Scans Go and TypeScript services for missing rate-limit middleware on public
HTTP endpoints. Reports findings at the endpoint level with a suggested
fix referencing the appropriate framework middleware.

**Capabilities declared:**
- `fs:read **/*.go **/*.ts`

**Source:** `examples/plugins/scanner-ratelimit/` _(reference implementation)_

---

### 2. `forge-codemod-log-redact`

**Kind:** Codemod  
**Author:** community  
**Install:** `forge plugin install forge-codemod-log-redact`

Rewrites log statements that accidentally log struct fields containing PII
(email, phone, ssn, card number) to use `[REDACTED]` placeholders. Works
with `zerolog`, `zap`, `logrus`, and `slog`.

**Capabilities declared:**
- `fs:read **/*.go`
- `fs:write **/*.go`

**Source:** `examples/plugins/codemod-log-redact/` _(reference implementation)_

---

### 3. `forge-template-worker`

**Kind:** Template  
**Author:** community  
**Install:** `forge plugin install forge-template-worker`

Adds `forge new --template worker` — scaffolds a background-job worker with:
- A job queue interface backed by Redis or in-memory
- Structured logging
- Graceful shutdown with configurable drain timeout
- A `forge scan` config pre-tuned for worker services

**Capabilities declared:**
- `fs:write ./**`

**Source:** `examples/plugins/template-worker/` _(reference implementation)_

---

## Submitting a community plugin

1. Implement the relevant interface from `internal/plugin/plugin.go`.
2. Write a `plugin.toml` manifest with capability declarations.
3. Build to WASM: `GOOS=wasip1 GOARCH=wasm go build -o plugin.wasm .`
4. Run `forge plugin validate plugin.wasm` to check compliance.
5. Open a PR adding your plugin to this list.

See [PLUGIN_AUTHORING.md](PLUGIN_AUTHORING.md) for the full guide.

---

## Plugin registry

The official registry index is maintained at `internal/plugin/registry_index.go`.
Community plugins submitted via PR are reviewed for capability safety before
being added to the registry.
