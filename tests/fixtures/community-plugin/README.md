# community-plugin-demo

A sample community WASM plugin demonstrating the Forge RFC-001 sandbox capability model (G-130).

## Manifest

| Field       | Value                           |
|-------------|---------------------------------|
| Name        | `community-plugin-demo`         |
| Version     | `0.1.0`                         |
| Kind        | `scanner`                       |
| Runtime     | WASM (wazero, CGO-free)         |
| Capabilities| `fs:read` only                  |

## What it does

Scans source files for TODO/FIXME annotations and reports them as scanner
findings. It demonstrates:

1. **Manifest-based capability declaration** — only `fs:read` requested; no
   `fs:write`, `net:http`, or `exec` capabilities.
2. **Sandbox enforcement** — write attempts outside the declared capability
   are blocked by `internal/fssandbox`.
3. **Scan-family registration** — the plugin registers itself under the
   `dx` scanner family via the forge plugin contract.
4. **Finding schema** — findings conform to `plugin.Finding` with `RuleID`,
   `Severity`, `Message`, `File`, `Line`.

## Expected findings on fixture code

Given a file containing `// TODO: remove this`, the plugin emits:

```json
{
  "rule_id": "community/todo-annotation",
  "severity": "low",
  "message": "TODO annotation found — consider filing a ticket.",
  "file": "src/app.go",
  "line": 42
}
```

## Running

```sh
forge plugin install ./tests/fixtures/community-plugin
forge scan dx --report
```

## Source

The `.wasm` binary is compiled from `community_plugin_demo.wat` (WebAssembly
text format) — a minimal hand-written stub for CI purposes. Real community
plugins are compiled from Go/Rust/TinyGo via:

```sh
tinygo build -o community_plugin_demo.wasm -target=wasi .
```

## Security notes

- All file reads are mediated through the wazero sandbox host functions.
- The plugin cannot spawn subprocesses (`exec` capability not granted).
- Network calls are blocked (`net:http` not in capabilities).
- See `docs/PLUGIN_AUTHORING.md` for the full capability taxonomy.
