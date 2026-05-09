# ADR-006 — Telemetry transport

- **Status:** Proposed
- **Tracker:** ARCH-DEC-06
- **Spec/Arch anchor:** Arch §13 ADR-006, Spec §11 (telemetry & privacy), Arch §15 (security)
- **Decision date:** TBD
- **Deciders:** DevSecOps
- **Consulted:** Security WG, Founder

## Context

Forge needs operational telemetry (cold-start, error rates, version skew) to maintain NFRs §14 and detect regressions in the field. Privacy law (GDPR, CCPA) and developer trust require:

- Opt-in only, with clear UX.
- No PII / no source-code content.
- Standard wire format that operators can route to their own observability stack.
- Local-first: telemetry never blocks a CLI invocation.

## Decision

Forge will use **OTLP/HTTP (protobuf) over HTTPS** as the sole telemetry transport, opt-in via either:

- Interactive: `forge telemetry enable` (writes `~/.config/forge/telemetry.toml`).
- Non-interactive: `FORGE_TELEMETRY=1` env var (CI-friendly).

Default endpoint: `https://telemetry.forge.sh/v1/{traces,metrics,logs}`. Operators can override with `FORGE_OTLP_ENDPOINT=...`. Without explicit opt-in, **no network call** is made.

### Payload schema (acceptance artefact)

| Field | Type | Notes |
|-------|------|-------|
| `service.name` | string | Always `"forge"`. |
| `service.version` | string | Semver. |
| `os.type`, `os.arch` | string | From `uname`/equivalent. |
| `forge.command` | string | Verb only (e.g. `"ship"`), never args. |
| `forge.exit_code` | int | |
| `forge.duration_ms` | int | |
| `forge.error_code` | string | `FORGE-NNNN` only; no message. |
| `install_id` | uuid v4 | Generated locally on first opt-in; rotatable via `forge telemetry rotate-id`. |

**Forbidden fields:** workspace path, file names, env-var values, model prompts/responses, secrets, plugin source URLs.

A per-payload regex redactor (shared with the secrets redactor, Arch §17.2 row 6) runs as a final guard before transmit; redaction failure aborts send and logs locally.

## Alternatives considered

### Option A — Vendor-specific SDK (Datadog, Honeycomb) (rejected)

Pros: zero-config dashboards.
Cons: lock-in; bigger binary; harder for self-hosters.

### Option B — Custom JSON over POST (rejected)

Pros: simplest.
Cons: re-invents OTLP poorly; no reuse of operator collectors.

### Option C — Default-on with per-host opt-out (rejected)

Pros: better data quality.
Cons: violates the explicit consent norm Forge wants to set; legal risk in jurisdictions requiring opt-in.

## Consequences

### Positive

- Operators can point `FORGE_OTLP_ENDPOINT` at their own collector → zero data egress.
- Standard format → no SDK lock-in.
- Local-first: outage of `telemetry.forge.sh` never affects users.

### Negative / accepted trade-offs

- Opt-in means lower sample density; mitigated by published "why we ask" page.
- Maintaining the redaction regex pack is recurring work — shared with secrets redactor.

### Follow-ups created

- DEV-M1-25 — telemetry emitter + redactor.
- DEV-M1-26 — `forge telemetry {enable,disable,status,rotate-id}` verbs.
- TEST-21 — fixture corpus for redaction (shared with secrets).

## Compliance hooks

- Test: with telemetry disabled, no syscalls hit network namespace (TEST-21).
- Test: payload schema fuzzer never produces a forbidden field.
- CI gate: redactor coverage of secrets fixture corpus must be 100 %.
- Docs: `docs/PRIVACY.md` lists every field above.

## References

- Arch §13 ADR-006, §15.
- OTLP/HTTP spec: <https://opentelemetry.io/docs/specs/otlp/>.
