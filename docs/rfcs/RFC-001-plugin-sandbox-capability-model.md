# RFC-001 — Plugin Sandbox Capability Model

| Field | Value |
|-------|-------|
| RFC | 001 |
| Title | Plugin Sandbox Capability Model |
| Author | Forge Maintainers |
| Status | **Accepted** |
| Created | 2025-01-15 |
| Implemented in | ADR-002, `internal/plugin` |

---

## Summary

Define a fine-grained capability model for Forge plugins that allows plugin
authors to declare the resources they need (filesystem paths, network endpoints,
subprocesses) while preventing privilege escalation and sandbox escape.

---

## Motivation

Forge loads third-party plugins at runtime. Without a capability model, any
plugin can read the entire filesystem, exfiltrate secrets, or spawn arbitrary
subprocesses. We need a model that is:

1. **Declarative** — capabilities are listed in the plugin manifest.
2. **Minimal** — plugins get only what they declare; nothing more.
3. **Auditable** — `forge plugin list` shows the granted capabilities.
4. **Enforceable** — the WASM runtime (wazero) enforces the sandbox.

---

## Design

### Capability namespaces

```
fs:<path>      — read/write access to a filesystem path (glob patterns allowed)
net:<host>     — outbound HTTP(S) to a hostname
exec:<binary>  — spawn a specific binary via the allow-list
env:<key>      — read an environment variable
```

### Manifest declaration

```toml
# .forge/plugin.toml
name    = "forge-scanner-cost"
version = "1.0.0"
kind    = "scanner"

[[capabilities]]
  type = "fs:read"
  path = "**/*.go"

[[capabilities]]
  type = "net:https"
  host = "pricing.cloud.google.com"
```

### Enforcement

- **In-process plugins** are granted capabilities at registration time via a capability token checked before each API call.
- **WASM plugins** are sandboxed by wazero's filesystem and network interceptors; only declared paths/hosts are mounted.
- Undeclared capability use returns `FORGE-4060: capability not granted`.

---

## Alternatives considered

- **No sandbox** — rejected; unsafe for third-party plugins.
- **OS-level sandbox** (seccomp, AppArmor) — rejected; complex, platform-specific.
- **Separate process with Unix socket** — possible future work; too heavy for v1.0.

---

## Decision

Accepted. Implemented in `internal/plugin/plugin.go` (capability field on `Manifest`) and `internal/fssandbox` (enforcement). WASM enforcement via wazero arrives in M2.
