# ADR-015 — Chaos-drill harness

- **Status:** Proposed
- **Tracker:** ARCH-DEC-15
- **Spec/Arch anchor:** Arch §17.3, OPS-17
- **Decision date:** TBD
- **Deciders:** Quality WG + DevSecOps
- **Consulted:** Core engineering

## Context

The 8 cross-cutting failure scenarios catalogued in Arch §17.3 (provider outage mid-`ship`, ENOSPC, concurrent ship lock, plugin panic, ledger tamper, cassette drift, secret-leak-via-debug, prod migration drift) must be **provably exercised** monthly (per OPS-17). External chaos tools (Chaos Mesh, Toxiproxy, Litmus) target Kubernetes; Forge is a CLI.

## Decision

Forge will build an **in-tree synthetic-fault harness** (`forge chaos drill`) that injects faults at the resilience-library seams (per ADR-014) and at well-defined sandbox boundaries (FS, plugin host, network). Each scenario is described by a `drill.yml` pinned to a §17.3 row.

### `drill.yml` schema (acceptance artefact)

```yaml
api_version: forge.sh/v1
kind: ChaosDrill
metadata:
  id: drill-provider-outage-mid-ship
  scenario_anchor: Arch §17.3 #1
  owner: quality-wg
spec:
  setup:
    fixture: fixtures/stripe-demo
    forge_version: ">=0.10.0"
  faults:
    - at: capability=llm.gateway
      kind: timeout-after
      after_ms: 1500
      applies_to_call: "complete"
    - at: capability=fs.write
      kind: enospc
      probability: 0.0   # not used in this scenario
  expect:
    exit_code: !=0
    error_code: FORGE-2001
    artifacts:
      - path: .forge/checkpoints/<run-id>.json
        must_exist: true
    no_partial_writes_to:
      - workspace/src
  drill_report_path: .forge/drills/<run-id>.json
```

### Drill report schema

```json
{
  "$id": "https://forge.sh/schemas/drill-report.schema.json",
  "type": "object",
  "required": ["drill_id", "scenario_anchor", "started_at", "ended_at",
               "outcome", "containment_trace", "expected_error_code",
               "actual_error_code"],
  "properties": {
    "outcome": { "enum": ["pass", "fail", "unknown"] }
  }
}
```

`outcome=unknown` is mandatory when the harness itself errored — never silently `pass`.

### Execution model

- Faults injected via `Resilience::with_fault_injector(...)` (test-only feature flag in capability traits).
- Plugin-panic scenarios use the WASM host's trap interface (per ADR-002).
- Ledger-tamper scenarios mutate a sandbox copy of `.forge/ledger/` then run the verifier.
- Cassette-drift scenarios swap a known-bad cassette and assert `forge eval` fails.

### Catalog

`forge/chaos/drills/` ships one `drill.yml` per §17.3 row. Adding a new scenario = adding a row to §17.3 + a new `drill.yml` + a CI run; the failure-register verifier (per ADR-016) checks both.

## Alternatives considered

### Option A — External tool (Toxiproxy + bash) (rejected)

Pros: existing project.
Cons: only covers network faults; misses FS / plugin / ledger seams; dual-language CI overhead.

### Option B — Property-based testing only (rejected)

Pros: bug-finding power.
Cons: hard to pin to scenario IDs; harder to read drill reports; doesn't replace targeted scenarios.

### Option C — Manual quarterly tabletops (rejected)

Pros: trains humans.
Cons: not CI-enforceable; doesn't catch silent regressions.

## Consequences

### Positive

- Drills are version-controlled, reviewable, replayable.
- Drill reports feed the Quality Dashboard (TEST-19).
- Per-scenario traceability anchors §17.3 rows directly to passing CI runs.

### Negative / accepted trade-offs

- Authoring fault injectors per capability is a recurring cost; shared with ADR-014.
- Some scenarios (prod migration drift) require a sandboxed Postgres — covered by Docker Compose harness.

### Follow-ups created

- DEV-M2-23 — chaos-drill harness implementation.
- TEST-27 — chaos-drill regression suite.
- OPS-17 — monthly chaos drill schedule.

## Compliance hooks

- CI gate: monthly workflow runs all 8 catalogued drills (OPS-17).
- Test: every §17.3 row has at least one passing drill (TEST-27).
- Lint: drill report must include `outcome` and `actual_error_code`.

## References

- Arch §17.3, §17.4.
- ADR-014.
- "Principles of Chaos Engineering": <https://principlesofchaos.org/>.
