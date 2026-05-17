# Forge Verbs Reference

> Complete reference for every `forge` CLI command.  One row per verb in the
> summary table; detailed flags, examples, and error codes follow.

---

## Quick Reference

| Verb | Synopsis | Error range |
|------|----------|-------------|
| `forge version` | Print binary version and build metadata | — |
| `forge doctor` | Check the local environment for prerequisites | `FORGE-1000..1099` |
| `forge new` | Scaffold a new project from a template | `FORGE-1100..1199` |
| `forge clean` | Apply manifest-driven repo hygiene | `FORGE-1200..1299` |
| `forge explain` | Introspect any verb or plugin manifest | `FORGE-1300..1399` |
| `forge scan` | Run security / quality scanners | `FORGE-1400..1499` |
| `forge lint` | Check conventions and hygiene markers | `FORGE-1500..1599` |
| `forge ship` | Full five-checkpoint delivery pipeline: `forge ship auth/email` (positional arg); `--resume` to continue; checkpoint 5 renamed from `verify` → `ship` (G-003) | `FORGE-1600..1699` |
| `forge test` | Run any of 13 test families (unit/integration/e2e/journey/perf/load/soak/…) | `FORGE-4300..4399` |
| `forge upgrade` | Apply built-in or plugin codemods (renamed from `forge migrate-code`) | `FORGE-2000..2099` |
| `forge audit` | Query / verify the audit ledger; `forge audit erase` (renamed from `forge gdpr erase`); `forge audit export` (renamed from `forge compliance export`) | `FORGE-3400..3499` |
| `forge eval` | Run deterministic evaluation scenarios | `FORGE-3600..3699` |
| `forge plugin` | Manage installed WASM plugins | `FORGE-3700..3799` |
| `forge postmortem` | Draft or review incident postmortems | `FORGE-3800..3899` |
| `forge insights` | Local telemetry rollup and statistics; `forge insights cli` finds unused verbs | `FORGE-3900..3999` |
| `forge incident` | Manage the incident lifecycle | `FORGE-4000..4099` |
| `forge telemetry` | Opt in/out and rotate telemetry ID | `FORGE-4100..4199` |
| `forge spend` | Track and cap LLM token spend | `FORGE-2400..2499` |
| `forge fixtures` | Generate JSON test fixture files | `FORGE-6000..6099` |
| `forge backup` | Create a point-in-time backup snapshot before risky operations | `FORGE-6100..6199` |
| `forge ci` | Post-push CI monitor: watch, fix, and record lessons from CI runs | `FORGE-6200..6299` |
| `forge learn` | Manage the learning loop; sub-verbs: `teach` (renamed from `forge teach`), `session` (renamed from `forge session digest`), `instructions` (renamed from `forge instructions evolve`), `promote`, `antipatterns` | `FORGE-5200..5299` |
| `forge context` | Manage project context bundles; `forge context generate` (renamed from `forge generate ai-context`) | — |
| `forge agents` | Manage LLM agents; `forge agents stop` (replaces `forge agents stop --workspace`) | — |
| `forge ask` | Ask questions about the project via LLM; `forge ask error <code>` looks up error docs | `FORGE-4900..4999` |

> **Deprecation notice (G-090):** The following old verbs print a deprecation hint and delegate to the new name:
> `forge migrate-code` → `forge upgrade`,
> `forge teach` → `forge learn teach`,
> `forge session` → `forge learn session`,
> `forge instructions` → `forge learn instructions`,
> `forge gdpr` → `forge audit`,
> `forge compliance` → `forge audit`.

For the full error-code catalogue see [`docs/ERROR_CODES.md`](ERROR_CODES.md).

---

## Detailed Reference

### `forge version`

Print the version string baked into the binary at build time.

```bash
forge version
# forge v0.2.0-m2-preview
```

---

### `forge doctor`

Check that the local environment is correctly configured.  Exits `0` if all
required tools are present; non-zero with a `FORGE-1xxx` code on any failure.

```bash
forge doctor           # human-readable output
forge doctor --json    # machine-readable JSON
```

**What is checked:**
- `git` is on `$PATH`
- `go` is on `$PATH` (version ≥ 1.24)
- `.gitignore` managed block is present (if inside a Forge project)
- An LLM provider credential is reachable

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `--json` | `false` | Emit machine-readable JSON |

---

### `forge new`

Scaffold a new project from a built-in or plugin-provided template.

```bash
forge new ts-service my-app
forge new next-app my-app
forge new go-service my-app --module github.com/yourname/my-app
forge new go-service .      --module github.com/yourname/my-app --force
```

**Built-in templates:**

| Template | Stack |
|----------|-------|
| `ts-service` | TypeScript + Vitest + Forge CI gates |
| `next-app` | Next.js 14, TypeScript, Tailwind CSS, App Router, Vitest, Playwright |
| `go-service` | Go HTTP service with graceful shutdown, `/healthz`, `/readyz` |

The `go-service` template generates a standard Forge project layout
including a pre-configured `.gitignore`, `.gitleaks.toml`, and
`.forge/manifest`.

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `--module` | `""` | Go module path (required for `go-service`) |
| `--force` | `false` | Overwrite existing files |
| `--json` | `false` | Emit machine-readable JSON |

**Error codes:**
- `FORGE-1100` — unknown template name
- `FORGE-1101` — target directory not empty and `--force` not set

---

### `forge clean`

Apply the `.forge/manifest` hygiene rules to the working tree.

```bash
forge clean --check   # dry-run: report what would change, exit 1 if dirty
forge clean --apply   # apply all fixes in place
```

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `--check` | `true` | Dry-run; exits non-zero if any fix is needed |
| `--apply` | `false` | Apply fixes to disk |
| `--json` | `false` | Emit machine-readable JSON |

---

### `forge explain`

Print the manifest and capability description for any verb or plugin.  Designed
to be consumed directly by an LLM as structured context.

```bash
forge explain           # list all verbs
forge explain ship      # describe the `ship` verb
forge explain --json    # machine-readable JSON for all verbs
```

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `--json` | `false` | Emit machine-readable JSON |

---

### `forge scan`

Run one or all scanner families against the project.

```bash
forge scan all                       # run every family
forge scan secrets                   # secrets only
forge scan rls                       # RLS / tenant-isolation only
forge scan prompt-injection          # LLM prompt-injection risks only
forge scan supply-chain              # dependency pinning / vuln risks only
forge scan all --root ./sub-project  # override project root
forge scan all --json                # machine-readable JSON report
```

**Families:**

| Family | What it checks |
|--------|----------------|
| `secrets` | Hard-coded API keys, tokens, passwords using built-in regex + gitleaks |
| `rls` | SQL / migration files for missing tenant-column predicates |
| `prompt-injection` | LLM prompt strings for ignore-previous / role-override patterns |
| `supply-chain` | Unpinned dependencies, curl-pipe-shell, loose Go `replace` directives |

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `--root` | cwd | Project root directory |
| `--json` | `false` | Emit machine-readable JSON |

**Error codes:**
- `FORGE-1400` — scanner initialisation failure
- `FORGE-1401` — scan finding (non-zero exit)
- `FORGE-1402` — scanner plugin error

---

### `forge lint`

Check project conventions, hygiene manifest markers, and gitignore managed
blocks.

```bash
forge lint           # human-readable
forge lint --json    # machine-readable JSON
```

**What is checked:**
- `.forge/manifest` syntax and required sections
- `.gitignore` managed block presence and correct markers
- `.gitleaks.toml` baseline rule presence

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `--json` | `false` | Emit machine-readable JSON |

---

### `forge ship`

Run the full five-checkpoint delivery pipeline (Spec → Test → Breakdown → Code →
Ship), or run a single checkpoint in isolation.

```bash
forge ship auth/email                    # slugifies to auth-email; runs all checkpoints
forge ship auth/email --resume           # resume from last incomplete checkpoint
forge ship spec   --json                 # only Spec checkpoint
forge ship test   --json                 # only Test checkpoint
forge ship breakdown                     # only Breakdown checkpoint
forge ship code                          # only Code checkpoint
forge ship ship   --json                 # only Ship (hygiene + scan) checkpoint
forge ship --description "add rate-limiter middleware"   # legacy flag (deprecated; use positional arg)
```

> **Note (G-001):** The positional `<feature>` argument is the preferred form.
> `--description` is a deprecated alias and will be removed in the next minor version.
> `forge ship verify` is also a deprecated alias for `forge ship ship`.

**Checkpoint subcommands:**

| Subcommand | Checkpoint | What happens |
|------------|------------|-------------|
| `spec` | 1 | Validates `.forge/specs/<slug>/spec.md`; creates `spec.yml` if absent |
| `test` | 2 | AI writes failing tests; commits them before code (TDD gate) |
| `breakdown` | 3 | AI produces `.forge/specs/<slug>/tasks.md` and per-task context bundles |
| `code` | 4 | AI iterates until `go test -race ./...` is green |
| `ship` | 5 | `forge scan all` + `forge lint` + `forge eval`; ship readiness check |
| ~~`verify`~~ | *(deprecated)* | Alias for `ship`; will be removed in the next minor |

**Flags (all subcommands share these):**

| Flag | Default | Description |
|------|---------|-------------|
| `--dry-run` | `true` | Validate without applying changes (MVP default) |
| `--description` | `""` | Plain-English description (deprecated — use positional arg) |
| `--resume` | `false` | Continue from the first incomplete checkpoint |
| `--yes` | `false` | Non-interactive mode; auto-advance through all checkpoints |
| `--json` | `false` | Emit machine-readable NDJSON events |

**Error codes:**
- `FORGE-1600` — spec missing or incomplete
- `FORGE-1601` — tests were written after the code (gate violation)
- `FORGE-1602` — scan or lint finding blocks ship

---

---

### `forge test`

Run one or more of the 13 test families against the current workspace.
Every subcommand is a dry-run plan in MVP; M1 wires each to the real test
runner / chaos harness / load generator.

```bash
forge test unit                          # unit tests only
forge test integration --json            # integration tests, JSON output
forge test e2e                           # end-to-end
forge test journey --json                # user-journey tests
forge test perf --bench-count 10         # performance benchmarks
forge test load --duration 5m --workers 50
forge test soak --duration 2h
forge test chaos --timeout 30m
forge test mutation                      # mutation testing
forge test smoke                         # quick smoke check
forge test regression                    # regression suite
forge test contract                      # consumer-driven contract tests
forge test snapshot                      # snapshot / golden-file tests
forge test all --fail-fast               # run all 13 families in order
```

**Test family subcommands:**

| Subcommand | Family | Purpose |
|------------|--------|---------|
| `unit` | Unit | Fast in-process unit tests (`go test ./...`) |
| `integration` | Integration | Tests that call real databases / services |
| `regression` | Regression | Guard against previously-fixed bugs |
| `e2e` | E2E | End-to-end CLI/API exercisers |
| `journey` | Journey | Multi-step user-journey flows (mirrors `journey_test.go`) |
| `smoke` | Smoke | Minimal post-deploy liveness check |
| `contract` | Contract | Consumer-driven contract tests (Pact / OpenAPI) |
| `perf` | Perf | Throughput and latency benchmarks |
| `load` | Load | Sustained concurrency load test |
| `soak` | Soak | Extended duration stability test |
| `chaos` | Chaos | Fault-injection / resilience drills |
| `mutation` | Mutation | Mutation testing to evaluate suite quality |
| `snapshot` | Snapshot | Golden-file / snapshot regression tests |
| `all` | — | Run all 13 families in recommended order |

**Execution order for `forge test all`:**
smoke → unit → regression → snapshot → contract → integration → journey →
e2e → perf → load → chaos → soak → mutation

**Flags (all subcommands share these):**

| Flag | Default | Description |
|------|---------|-------------|
| `--dry-run` | `true` | Plan without executing (MVP default) |
| `--parallel` | `false` | Run families in parallel |
| `--workers` | `10` | Goroutine / connection pool size |
| `--duration` | `1h` | Target duration for soak / load tests |
| `--timeout` | `10m` | Per-family timeout |
| `--bench-count` | `5` | Benchmark iterations for perf family |
| `--fail-fast` | `false` | Stop after the first failing family |
| `--root` | `""` | Workspace root (defaults to `$PWD`) |
| `--json` | `false` | Emit machine-readable JSON |

**JSON output shape:**

```json
{
  "dry_run": true,
  "families": [
    { "family": "unit", "status": "pending", "test_count": 0,
      "detail": "...", "duration_ms": 0 }
  ],
  "passed": 0, "failed": 0, "skipped": 13,
  "duration": "0s",
  "ready": true,
  "message": "..."
}
```

**Error codes:**
- `FORGE-4300` — one or more test families failed
- `FORGE-4301` — unknown test family name
- `FORGE-4302` — invalid flag combination

> **Vibe Coding Tip:** Run `forge test journey --json` after every `forge ship`
> to validate full user-journey flows end-to-end without leaving the terminal.

#### 4-Phase Test Lifecycle Subcommands

`forge test` also drives a **create → approve → run → ci** lifecycle that
scaffolds tests with an LLM/vibe-coder, gates on human approval, executes them
locally, then triggers or guides CI/CD setup.

```bash
forge test create rate-limiter               # LLM generates test scaffolding (dry-run)
forge test approve rate-limiter              # review and approve generated tests
forge test run rate-limiter                  # run approved tests locally
forge test ci rate-limiter --env staging     # trigger/guide CI/CD run
forge test --feature rate-limiter            # full lifecycle (create→approve→run→ci)

# Live mode (writes .forge/tests/<feature>/pending.json)
forge test create rate-limiter --dry-run=false

# Auto-approve + generate CI config
forge test --feature rate-limiter --auto-approve --generate-config
```

**Lifecycle subcommand flags:**

| Flag | Default | Applies to | Description |
|------|---------|------------|-------------|
| `--feature <name>` | `""` | parent / all | Feature slug to scope tests to |
| `--description <text>` | `""` | `create` | Natural-language spec for LLM |
| `--env <env>` | `staging` | `ci` | Target environment for CI run |
| `--auto-approve` | `false` | parent | Skip manual approve step |
| `--generate-config` | `false` | `ci` | Write `.github/workflows/forge-test.yml` |
| `--dry-run` | `true` | all | Plan without side effects |
| `--json` | `false` | all | Emit machine-readable JSON |
| `--root` | `""` | all | Workspace root (defaults to `$PWD`) |

**Lifecycle JSON shapes:**

`forge test create`:
```json
{
  "dry_run": true,
  "feature": "rate-limiter",
  "generated": [
    { "family": "unit", "path": "tests/rate-limiter/unit_test.go",
      "estimated_lines": 120 }
  ],
  "ready": true,
  "message": "..."
}
```

`forge test approve`:
```json
{ "feature": "rate-limiter", "approved": 5, "ready": true }
```

`forge test run`:
```json
{
  "feature": "rate-limiter",
  "families": [{ "family": "unit", "status": "pending", "test_count": 0 }],
  "ready": true
}
```

`forge test ci`:
```json
{
  "feature": "rate-limiter",
  "has_ci": false,
  "config_generated": false,
  "setup_steps": [
    { "order": 1, "description": "Create a GitHub repository", "required": true }
  ],
  "ready": false
}
```

**State files written by lifecycle (live mode only):**

| Phase | File | Contents |
|-------|------|----------|
| `create` | `.forge/tests/<feature>/pending.json` | List of generated files |
| `approve` | `.forge/tests/<feature>/approved.json` | Approved file list |

**CI providers detected:**

| Provider | Detection file |
|----------|---------------|
| GitHub Actions | `.github/workflows/` directory |
| GitLab CI | `.gitlab-ci.yml` |
| CircleCI | `.circleci/config.yml` |
| Jenkins | `Jenkinsfile` |
| Drone | `.drone.yml` |
| Azure Pipelines | `azure-pipelines.yml` |
| Buildkite | `.buildkite/pipeline.yml` |

**Error codes (lifecycle):**
- `FORGE-4303` — test generation failed; spec missing or LLM unavailable
- `FORGE-4304` — tests not approved; run `forge test approve` first
- `FORGE-4305` — CI/CD pipeline not configured; follow setup guidance

---

### `forge upgrade`

Apply codemods to the working tree.

```bash
forge upgrade list              # show available codemods
forge upgrade gitignore-marker  # apply a specific codemod (dry-run by default)
forge upgrade gitignore-marker --apply
```

**Built-in codemods:**

| Codemod | What it does |
|---------|-------------|
| `gitignore-marker` | Adds or repairs the Forge managed block in `.gitignore` |
| `gitleaks-baseline` | Upgrades `.gitleaks.toml` to the current baseline rule pack |
| `dependabot-baseline` | Adds or updates `.github/dependabot.yml` |
| `pre-commit-baseline` | Adds or updates `.pre-commit-config.yaml` |

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `--apply` | `false` | Apply changes to disk (default is dry-run) |
| `--json` | `false` | Emit machine-readable JSON |

---

### `forge audit`

Inspect the hash-chained audit ledger at `.forge/audit.log`.

```bash
forge audit show                   # print recent entries
forge audit verify                 # verify the chain is intact
forge audit query --since main --limit 50
forge audit query --actor llm --json
```

**Subcommands:**

| Subcommand | Description |
|-----------|-------------|
| `show` | Print the most recent audit entries |
| `verify` | Re-compute and verify every hash in the chain |
| `append` | Manually append a signed entry |
| `query` | Filter entries by actor, time range, or event type |

**`query` flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `--since` | `""` | ISO-8601 timestamp or git ref |
| `--limit` | `20` | Maximum number of records to return |
| `--actor` | `""` | Filter by actor name (`llm`, `human`, `ci`) |
| `--json` | `false` | Emit machine-readable JSON |

---

### `forge eval`

Run deterministic YAML-scenario tests against plugins or prompt templates.

```bash
forge eval .forge/eval/scenarios/      # run all scenarios in a directory
forge eval .forge/eval/scenarios/ --json
```

Each scenario file declares an input, the expected output, and the plugin to
invoke.  `forge eval` exits non-zero if any scenario diverges from its expected
output.

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `--json` | `false` | Emit machine-readable JSON report |

---

### `forge plugin`

Manage installed WASM plugins.

```bash
forge plugin list                       # list installed plugins
forge plugin show my-scanner            # print manifest for one plugin
forge plugin install my-scanner@v1.2   # install a plugin
forge plugin upgrade my-scanner        # upgrade to latest
forge plugin remove my-scanner         # uninstall
```

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `--json` | `false` | Emit machine-readable JSON |

---

### `forge incident`

Manage the local incident lifecycle.

```bash
forge incident new --title "API latency spike"
forge incident list
forge incident update INC-001 --status investigating
forge incident close INC-001 --resolution "rolled back deploy"
```

**Flags (`new`):**

| Flag | Description |
|------|-------------|
| `--title` | Short title for the incident (required) |
| `--severity` | `sev1` / `sev2` / `sev3` (default: `sev2`) |

---

### `forge telemetry`

Manage local telemetry collection.

```bash
forge telemetry status      # show current opt-in state
forge telemetry enable      # opt in to telemetry
forge telemetry disable     # opt out
forge telemetry rotate-id   # generate a new anonymous participant ID
```

Telemetry is **off by default**.  No data is sent without explicit opt-in.

---

### `forge spend`

Track and cap LLM token spend per session.

```bash
forge spend status              # print current spend and limit
forge spend set --limit 50000   # set a hard token-count cap
forge spend reset               # clear the current session counter
```

---

### `forge postmortem`

Facilitate drafting and reviewing incident postmortems with the CI gate.

```bash
forge postmortem path/to/postmortem.md   # validate structure + completeness
```

---

### `forge insights`

Generate a local rollup of telemetry data for the current project.

```bash
forge insights           # human-readable summary
forge insights --json    # machine-readable JSON
```

---

### `forge ci`

Post-push CI monitor — watch, fix, and record lessons from GitHub Actions runs
(spec §13.6, DEV-M3-31).  Usually invoked automatically by `.githooks/post-push`,
but all sub-commands are available directly for agents and manual use.

```bash
# Watch CI for the current HEAD commit (polls until pass/fail or timeout):
forge ci watch

# Watch a specific SHA in an explicit repo:
forge ci watch --sha abc1234 --repo org/myrepo --timeout 10m

# Propose an LLM-assisted fix for a failed run:
forge ci fix --run-id 12345678

# Record a CI failure as a lesson to .forge/learned/gotchas.jsonl:
forge ci gotcha --run-id 12345678 --note "forgot to run go mod tidy"

# All sub-commands support --json for machine-readable output:
forge ci watch --json
forge ci gotcha --run-id 12345678 --json
```

**Environment variables:**

| Variable | Default | Purpose |
|----------|---------|---------|
| `GITHUB_TOKEN` | — | GitHub API token (falls back to `gh auth token`) |
| `FORGE_CI_DISABLE` | `0` | Set to `1` to disable post-push hook entirely |
| `FORGE_CI_TIMEOUT` | `300` | Max seconds to wait for CI in `.githooks/post-push` |
| `FORGE_CI_POLL_INTERVAL` | `10` | Poll interval in seconds |
| `FORGE_AUTOFIX` | `0` | Set to `1` to auto-invoke `forge ci fix` on failure |

**Exit codes for `forge ci watch`:**

| Code | Meaning |
|------|---------|
| `0` | CI passed |
| `1` | CI failed |
| `2` | Timed out waiting for CI to complete |

**Error codes:** `FORGE-6200..6299`
