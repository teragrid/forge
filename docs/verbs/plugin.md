# forge plugin

Install, list, and manage WASM plugins.

## Synopsis

```
forge plugin install <name|url> [--from-bundle <dir>]
forge plugin list
forge plugin remove <name> [--allow-irreversible]
forge plugin info <name>
```

## Description

Forge plugins are WASM modules that extend forge's scan, fix, and ship
capabilities. The WASM sandbox enforces capability isolation per ADR-002.

## Examples

```bash
forge plugin install gosec
forge plugin install --from-bundle /opt/forge-bundle
forge plugin list
forge plugin remove gosec --allow-irreversible
```
