# forge insights

Local telemetry rollup and usage analytics from the audit ledger.

## Synopsis

```
forge insights [--root <path>] [--since <date>] [--json]
```

## Description

`forge insights` reads the append-only audit ledger (`.forge/audit.log`) and
produces a per-verb action-count rollup — entirely offline, no remote calls.

Use it to understand how forge is being used in a project: which verbs run
most often, when they last ran, and how actions break down per verb.

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--root <path>` | `.` (cwd) | Project root containing `.forge/audit.log` |
| `--since <YYYY-MM-DD>` | all time | Filter events after this date |
| `--json` | `false` | Emit machine-readable JSON |

## Output fields (JSON)

```json
{
  "generated_at": "2024-05-13T10:00:00Z",
  "total_events": 42,
  "since": "2024-01-01",
  "verbs": [
    {
      "verb": "scan",
      "count": 18,
      "last_seen": "2024-05-12T09:15:00Z",
      "action_breakdown": { "run": 18 }
    },
    {
      "verb": "ship",
      "count": 3,
      "last_seen": "2024-05-10T14:00:00Z",
      "action_breakdown": { "run": 3 }
    }
  ]
}
```

## Examples

```bash
# Show all-time rollup
forge insights

# Filter to events since January 2024
forge insights --since 2024-01-01

# Machine-readable JSON, pipe to jq
forge insights --json | jq '.verbs[] | select(.verb == "ship")'

# Check a non-default project root
forge insights --root /path/to/my-project
```

## Error codes

| Code | Meaning |
|------|---------|
| `FORGE-3900` | Stats / insights operation failed |

## See also

- `forge audit` — full audit ledger query and verification
- `forge doctor` — environment health check
- `forge spend` — LLM token budget tracking
