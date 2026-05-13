# forge deploy

Deploy the project to a configured cloud adapter.

## Synopsis

```
forge deploy run   [--root <path>] [--tag <version>] [--dry-run] [--json]
forge deploy status [--root <path>] [--json]
```

## Description

`forge deploy` wraps a configured deployment adapter and records each
deployment to `.forge/deploy-history.json`.

Supported adapters:

| Adapter | Tool required | Env vars |
|---------|--------------|----------|
| `fly` | `flyctl` | `FLY_API_TOKEN` |
| `railway` | `railway` | `RAILWAY_TOKEN` |
| `render` | *(none — uses deploy hook URL)* | `RENDER_DEPLOY_HOOK_URL` |
| `heroku` | `heroku` CLI | `HEROKU_API_KEY` |
| `aws-ecs` | `aws` CLI | `AWS_ACCESS_KEY_ID` + `AWS_SECRET_ACCESS_KEY` |
| `shell` | *(any shell command)* | *(per config)* |

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--root` | cwd | Project root (directory containing `.forge/`) |
| `--tag` | `HEAD` | Version tag to deploy |
| `--dry-run` | false | Preview without executing |
| `--json` | false | Emit JSON output |

## Configuration

Set the adapter in `.forge/deploy.json`:

```json
{
  "adapter": "fly",
  "target": "my-app-name"
}
```

For AWS ECS, `target` must be `"cluster/service"`:

```json
{
  "adapter": "aws-ecs",
  "target": "prod-cluster/api-service",
  "env": { "AWS_REGION": "us-east-1" }
}
```

## Examples

```bash
# Deploy the latest build
forge deploy run --tag v1.2.3

# Preview without deploying
forge deploy run --dry-run

# Check last deployment
forge deploy status
```

## Error codes

| Code | Meaning |
|------|---------|
| `FORGE-5300` | Deploy operation failed |

## See also

- `forge rollback` — revert to a previous version
- [docs/DISTRIBUTION.md](../DISTRIBUTION.md)
