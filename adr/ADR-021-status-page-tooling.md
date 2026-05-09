# ADR-021 — Status-page tooling

- **Status:** Proposed
- **Tracker:** ARCH-DEC-21
- **Spec/Arch anchor:** Arch §18.5 #6, Arch §18.8
- **Decision date:** TBD
- **Deciders:** DevSecOps
- **Consulted:** Quality WG, founder

## Context

Forge needs a public incident state surface. Constraints:

- Zero recurring infra spend pre-1.0.
- No vendor lock-in for the Quality Dashboard (TEST-19) consumer.
- Auditable state-machine transitions: `identified → investigating → mitigated → fixed → post-mortem published`.
- Webhook events the dashboard can subscribe to.
- Survive its own deploy hosting going down (≥ 99.9 % avail target).

## Decision

Use **`cstate`** (a static-site status page generator, MIT-licensed) hosted on **GitHub Pages** at `status.forge.sh`. State-of-truth is the `forge-sh/status` repo; on-call updates incidents via PRs (or a thin `forge incident` CLI verb that writes the same files).

### Architecture

```
forge-sh/status repo
├── content/issues/<incident-id>.md    (one Markdown file per incident; cstate format)
├── content/systems/                   (CLI, Registry, Telemetry, Docs subsystems)
└── .github/workflows/
    ├── deploy-status.yml              (build + push to gh-pages)
    └── webhook-fanout.yml             (POST state transitions to dashboards)
```

### Incident lifecycle (state machine)

```
identified ──▶ investigating ──▶ mitigated ──▶ fixed ──▶ post-mortem-published
                       │              │
                       └──▶ monitoring ┘   (transient state; back to investigating on recurrence)
```

Illegal transitions (e.g. `identified → fixed` skipping `mitigated`) are rejected by the workflow's pre-merge linter.

### Webhook payload

```json
{
  "incident_id": "INC-042",
  "from": "investigating",
  "to": "mitigated",
  "at": "2026-05-09T10:00:00Z",
  "actor": "@on-call-handle",
  "signature": "sigstore-bundle-base64"
}
```

- HMAC-signed with a per-environment shared secret rotated quarterly (per ADR-022 two-key rotation).
- Replay-protected via `at` + `incident_id` deduplication on the dashboard side.

### Linkage to other ADRs

- ADR-017 — `severity:S0`/`S1` issue close fires a webhook into the status repo's intake.
- ADR-018 — vulnerability disclosure publishes a status entry when public.
- ADR-020 — `post-mortem-published` transition requires the `INC-<n>` PM file to exist; verifier (per ADR-016 + ADR-020) cross-checks.

## Alternatives considered

### Option A — Statuspage.io (rejected pre-1.0)

Pros: rich UI; battle-tested.
Cons: ~$29/mo minimum; vendor lock-in; Atlassian-owned.

### Option B — Instatus (rejected pre-1.0)

Pros: cheaper than Statuspage; nicer UX than cstate.
Cons: still paid; same lock-in concern.

### Option C — Self-hosted Cachet (rejected)

Pros: open source; rich features.
Cons: PHP + DB stack to operate; uptime burden.

### Option D — Pure GitHub Issues with a `status` label (rejected)

Pros: trivial.
Cons: no state-machine enforcement; no public summary surface; no webhook out of the box.

## Consequences

### Positive

- Zero recurring spend.
- Static-site = trivially mirror-able for redundancy.
- Quality Dashboard receives signed, deduplicated state transitions.

### Negative / accepted trade-offs

- cstate UI is plain; acceptable pre-1.0.
- GitHub Pages outage = status page outage; mitigated by mirroring `gh-pages` to a Cloudflare Pages backup post-1.0.

### Follow-ups created

- DEV-M2-25 — status-page wiring + webhook fanout.
- OPS-21 — per-release status-page check.
- TEST-19 — Quality Dashboard consumes webhook stream.

## Compliance hooks

- CI gate: incident state transitions validated by linter (DEV-M2-25 TC-25-02).
- Test: invalid-signature webhook rejected (TEST-29 TC-29-related; DEV-M2-25 TC-25-02).
- Audit: weekly check that every open incident has an update ≤ 7 days old.

## References

- Arch §18.5 #6, §18.8.
- cstate: <https://github.com/cstate/cstate>.
