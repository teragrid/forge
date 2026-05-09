# ADR-005 — Eval scenario format

- **Status:** Proposed
- **Tracker:** ARCH-DEC-05
- **Spec/Arch anchor:** Arch §13 ADR-005, Spec §7 (eval harness), Arch §17.2 (eval-harness row)
- **Decision date:** TBD
- **Deciders:** Quality WG
- **Consulted:** Core engineering, plugin WG

## Context

Forge eval scenarios drive deterministic regression of LLM-touching code paths, scan engines, and ship workflows. They must be:

- Authorable by humans + LLMs.
- Diff-friendly in PRs.
- Replayable byte-identically (cassette-pinned per ADR-023).
- Composable across providers (OpenAI, Anthropic, Bedrock, etc.).
- Validatable by JSON schema in CI.

## Decision

Eval scenarios will be authored as **`scenario.yml`** files matching the published JSON schema below, executed in a `forge eval` subprocess fixture (TEST-02). Each scenario pins a model, a seed, an HTTP cassette signature, and an outcome assertion graph.

### `scenario.yml` schema (excerpt — full schema at `forge/schemas/scenario.schema.json`)

```yaml
api_version: forge.sh/v1
kind: EvalScenario
metadata:
  id: ship-reference-stripe
  description: Reference `forge ship` of the Stripe demo app.
  owner: quality-wg
spec:
  inputs:
    workspace_fixture: fixtures/stripe-demo
    forge_version: ">=0.10.0"
    env:
      FORGE_LLM_PROVIDER: openai
      FORGE_LLM_MODEL: gpt-4o-mini
  determinism:
    seed: 42
    cassette: cassettes/ship-reference-stripe.har
    cassette_sha256: <hex>
  steps:
    - run: forge ship --dry-run
      expect:
        exit_code: 0
        stdout_contains: ["plan ok", "no irreversible ops"]
        artifacts:
          - path: .forge/plan.json
            json_schema: schemas/plan.schema.json
  oracles:
    - name: token-budget
      max_tokens: 8000
    - name: latency-budget
      max_seconds: 300
    - name: no-pii-leak
      regex_disallow: ["sk-live", "pk_live", "[A-Za-z0-9]{40}"]
  quarantine:
    quorum: 3
    threshold_disagree: 1
```

JSON schema (`forge/schemas/scenario.schema.json`) mirrors this with `$id: https://forge.sh/schemas/scenario.schema.json` and is validated by CI on every PR touching `eval/`.

## Alternatives considered

### Option A — Code-as-tests (Go `_test.go` only) (rejected)

Pros: full language power.
Cons: not LLM-authorable; hostile to non-Go contributors; cassette pinning is per-test ad hoc.

### Option B — Promptfoo / lm-eval-harness fork (rejected)

Pros: existing ecosystem.
Cons: scope mismatch (LLM-only); no first-class workspace fixtures or `forge ship`-style multi-step orchestration.

### Option C — Cucumber/Gherkin `.feature` files (rejected)

Pros: human-readable.
Cons: Gherkin's natural-language step matching is too lossy for byte-identical replay; worse for LLM authoring than YAML.

## Consequences

### Positive

- One schema covers scan, ship, deploy, hygiene, and provider scenarios.
- Cassette + seed pin → deterministic, replayable.
- Quarantine policy (per ADR-023) is part of the scenario itself, not buried in CI config.

### Negative / accepted trade-offs

- YAML edge cases (Norway problem, indentation) need the schema linter to be strict.
- HAR cassette format is verbose; `--cassette-format=ndjson` mode planned post-1.0.

### Follow-ups created

- DEV-M1-22 — `forge eval` runner + cassette engine.
- DEV-M1-23 — JSON schema publication + CI validation.
- TEST-13 — first reference scenario `ship-reference-stripe`.

## Compliance hooks

- CI gate: every `scenario.yml` validated against schema on PR.
- CI gate: cassette SHA-256 verified on every eval run.
- Test: deterministic re-run produces byte-identical reports (TEST-13, ADR-023).

## References

- Arch §13 ADR-005, §17.2.
- HAR spec: <http://www.softwareishard.com/blog/har-12-spec/>.
