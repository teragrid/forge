# Status Page & Incident Runbook

> Forge project operational status and incident response procedures.
> ADR references: ADR-019 (on-call rota), ADR-020 (postmortem), ADR-021 (status page).

---

## Status page

**Public status page:** (to be provisioned before v1.0.0 launch)

Options evaluated per ADR-021:
- [Instatus](https://instatus.com) — recommended for open-source projects
- [Statuspage.io](https://www.atlassian.com/software/statuspage) — enterprise option
- Self-hosted [Upptime](https://upptime.js.org) — zero-cost, GitHub-native

### Components tracked

| Component | Description |
|-----------|-------------|
| Forge CLI binary | Download availability via Brew/Scoop/winget |
| Plugin registry CDN | `registry.forge.dev` availability |
| GitHub releases | Release artifact signing + verification |
| Documentation site | `docs.forge.dev` availability |

---

## Incident severity levels

| Sev | Name | Response SLA | Examples |
|-----|------|-------------|---------|
| SEV-1 | Critical | 15 min | Data loss, security breach, all users blocked |
| SEV-2 | High | 1 hour | Major feature broken for >50% users |
| SEV-3 | Medium | 4 hours | Feature degraded, workaround available |
| SEV-4 | Low | 24 hours | Minor issue, cosmetic, single user affected |

---

## Incident response runbook

### Step 1 — Detection & declaration

```bash
# Create an incident record
forge incident new --severity SEV-2 --title "Plugin registry CDN degraded" \
  --assignee @oncall-handle

# This writes to .forge/incidents/INC-YYYY-MMDD-NNN.json and notifies on-call.
```

### Step 2 — Communication

1. Post to the status page immediately (even if cause is unknown): "Investigating reports of [symptom]."
2. Update the status page every 15 minutes during SEV-1/SEV-2.
3. For SEV-1: page the on-call engineer via the rota (see `docs/adr/ADR-019-oncall-rota-model.md`).

### Step 3 — Mitigation

Use `forge rollback` to revert a bad release:

```bash
forge rollback --to v0.9.3 --adapter fly
# or
forge rollback --to v0.9.3 --adapter railway
```

### Step 4 — Resolution

```bash
forge incident update INC-2026-0512-001 --status resolved \
  --resolution "Reverted to v0.9.3 pending root-cause fix"
```

### Step 5 — Postmortem

For SEV-1 and SEV-2, a postmortem is required within 48 hours:

```bash
forge postmortem new --incident INC-2026-0512-001
# Creates docs/postmortems/INC-2026-0512-001.md from the ADR-020 template.
# Must be reviewed and merged within 48h of incident close.
```

Postmortem CI gate (`M2-24`) blocks PRs with invalid postmortem structure.

### Step 6 — Follow-up

- File action-item issues tagged `postmortem-action`.
- Update the failure register (`internal/failure/`) with the incident data.
- Run `forge chaos drill` quarterly to verify recovery procedures still work.

---

## On-call rota

See [ADR-019](adr/ADR-019-oncall-rota-model.md) for the on-call model.

- Rotation: weekly, 2-person on-call
- Handoff: Mondays 09:00 UTC
- Escalation: project maintainers (see CODEOWNERS)

---

## Webhook integration

When `forge incident new` creates an incident, it can POST a webhook to the
status page provider. Configure in `.forge/config.toml`:

```toml
[incident]
  webhook_url = "https://hooks.instatus.com/service/<token>"
  webhook_secret = "${FORGE_WEBHOOK_SECRET}"  # injected from env
```

The `FORGE_WEBHOOK_SECRET` must be rotated every 90 days (two-key enforcement,
see `docs/adr/ADR-022-two-key-enforcement.md`).

---

*Maintained by the Forge maintainers. See CODEOWNERS for contacts.*
