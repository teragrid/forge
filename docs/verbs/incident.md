# forge incident

Manage on-call incidents and status-page webhooks.

## Synopsis

```
forge incident open   --title <text> --severity <SEV1-4>
forge incident update <id> --status <investigating|identified|monitoring|resolved>
forge incident close  <id>
forge incident list   [--open]
```

## Description

`forge incident` integrates with the status-page webhook (FORGE_WEBHOOK_URL)
to broadcast incident updates. See [STATUS_PAGE.md](../STATUS_PAGE.md).

## Examples

```bash
forge incident open --title "API latency spike" --severity SEV2
forge incident update INC-001 --status identified
forge incident close INC-001
forge incident list --open
```
