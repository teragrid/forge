# forge new

Scaffold a new AI-augmented project from a template.

## Synopsis

```
forge new <name> [--template <name>] [--root <path>] [--dry-run]
```

## Templates

- `go-api` — Go HTTP API with forge CI gates
- `go-cli` — Go CLI with cobra
- `python-service` — Python FastAPI service
- `node-service` — Node.js Express service
- regulated/soc2, regulated/hipaa, regulated/finserv — compliance scaffolds

## Examples

```bash
forge new my-service --template go-api
forge new my-cli --template go-cli --dry-run
```

## See also

- [PLUGIN_AUTHORING.md](../PLUGIN_AUTHORING.md)
