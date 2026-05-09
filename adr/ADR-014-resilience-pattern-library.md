# ADR-014 — Resilience-pattern library

- **Status:** Proposed
- **Tracker:** ARCH-DEC-14
- **Spec/Arch anchor:** Arch §17.1, Arch §17.4
- **Decision date:** TBD
- **Deciders:** Core engineer
- **Consulted:** Quality WG

## Context

Forge calls external systems (LLM providers, registries, deploy adapters, package mirrors). The §17.1 resilience contract requires every external call to honour a deadline, a retry policy, a circuit breaker, and a bulkhead. Today these patterns are open-coded inconsistently. §17.4 establishes seven CI invariants that must be lint-enforced.

## Decision

Forge will build an **in-tree resilience library** (`forge-foundation::resilience`) modelled on `failsafe-rs` / `resilience4j`, providing:

- `Timeout(budget)` — hard deadline; default per-call 30 s, override per capability.
- `Retry(policy)` — bounded attempts (default 3) with **decorrelated jitter** backoff (50 ms base, 5 s cap).
- `CircuitBreaker(failure_rate, window, half_open_probes)` — state machine open/half-open/closed; default 50 % failure over 20-call window opens; one half-open probe.
- `Bulkhead(max_concurrent, max_queue)` — per-provider concurrency cap with fail-fast queue.
- `IdempotencyKey(scope)` — wraps a retried operation with a caller-supplied or auto-generated key.

These compose as `Resilience::for_provider("openai").timeout(...).retry(...).circuit_breaker(...).bulkhead(...).call(|| async { ... })`. Capabilities (LLM gateway, deploy adapters, registry resolver) MUST go through this façade — direct `await` on raw I/O in capability namespaces is a lint error.

A **lint rule** (`forge-lint::resilience`) walks the AST of crates under `services/`, `adapters/`, and `plugins-host/`, flagging:

1. `await` on a non-`Resilience::*`-wrapped expression.
2. Missing `Timeout` in a `Resilience` chain.
3. `Retry` without `IdempotencyKey` for non-idempotent verbs.
4. Catch-all `unwrap()`/`expect()` on the result of an external call.
5. Bare `context.WithTimeout` on outbound calls (must use `resilience.Timeout`).
6. `loop { … sleep_then_retry }` patterns (must use `Retry`).
7. Network/process invocation without a `Bulkhead`.

These map 1:1 to the seven §17.4 CI invariants.

## Alternatives considered

### Option A — Adopt `failsafe` crate as-is (rejected)

Pros: zero authoring cost.
Cons: smaller surface (no bulkhead); no IdempotencyKey type; no AST lint integration.

### Option B — One-off helpers per capability (status quo, rejected)

Cons: drift; bug-fix-once-fix-everywhere costs grow super-linearly; lint impossible.

### Option C — Service-mesh-style sidecar (rejected)

Cons: Forge is a single-binary CLI; no place for a sidecar.

## Consequences

### Positive

- Single library to instrument, profile, and harden.
- Lint enforces the contract; new contributors cannot accidentally regress.
- Composes with the chaos-drill harness (ADR-015) to inject faults at well-defined seams.

### Negative / accepted trade-offs

- Adds an authoring step ("must wrap external call"); mitigated by the lint's auto-fix suggestions.
- Default policies will need tuning per capability — handled via per-capability config in `forge.toml`.

### Follow-ups created

- DEV-M1-39 — resilience-pattern library + lint enforcement.
- TEST-31 — resilience invariants lint coverage.
- ADR-015 — chaos-drill harness (consumes the resilience seams).

## Compliance hooks

- CI gate: `go run ./cmd/forge-lint resilience` on every PR.
- Test: each of the 7 §17.4 invariants has positive + negative fixtures (TEST-31).
- Bench: wrapper overhead < 1 µs per call.

## References

- Arch §17.1, §17.4.
- `failsafe-rs`: <https://docs.rs/failsafe>.
- AWS "Exponential Backoff and Jitter": <https://aws.amazon.com/blogs/architecture/exponential-backoff-and-jitter/>.
