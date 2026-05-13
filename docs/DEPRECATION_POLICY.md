# Forge — Deprecation Policy

> Effective from v1.0.0 · ADR reference: ADR-024 (reversibility contract)

---

## 1. Scope

This document governs the deprecation lifecycle for:

- **Public CLI verbs and flags** — any `forge <verb>` or `--flag` exposed to end users.
- **Public Go packages** — any package under `internal/` that is re-exported or documented in `doc.go`.
- **Plugin interfaces** — `Scanner`, `Codemod`, `Provider`, `Template` interfaces in `internal/plugin`.
- **Config keys** — keys in `.forge/config.toml` and environment variables prefixed `FORGE_`.
- **JSON schemas** — the `Finding`, `Manifest`, `Scenario`, and `Telemetry` payload schemas.

---

## 2. Deprecation lifecycle

All deprecations go through three stages:

| Stage | Minimum duration | User visibility |
|-------|-----------------|-----------------|
| **Deprecated** | 1 minor release | Warning printed to stderr on first use; docs updated with `> [!WARNING]` banner |
| **Soft-removed** | 1 minor release | Warning escalated to error by default; `--allow-deprecated` flag re-enables |
| **Hard-removed** | N/A | Symbol/flag/key deleted; `BREAKING.md` entry required; major version bump |

### Example timeline (semantic versioning)

```
v1.0.0  Feature introduced
v1.3.0  Feature deprecated  → stderr warning
v1.4.0  Soft-removed        → error unless --allow-deprecated
v2.0.0  Hard-removed        → deleted; BREAKING.md entry
```

---

## 3. Announcement requirements

Before a symbol enters the **Deprecated** stage, the following must exist:

1. A `BREAKING.md` entry describing the deprecated symbol, the reason, and the migration path.
2. A GitHub issue tagged `deprecation` linking to the BREAKING.md entry.
3. A CHANGELOG entry under the relevant version.
4. A deprecation notice in the relevant doc page or `--help` output.

---

## 4. Backward-compat alias mechanism

Deprecated CLI verbs are preserved via the alias mechanism in `internal/cli/aliases.go`.
Aliases print a deprecation warning on first use and are removed at hard-removal time.

```
forge old-verb  →  "WARN: 'forge old-verb' is deprecated, use 'forge new-verb' instead"
                   (then runs forge new-verb)
```

---

## 5. Plugin interface stability

Plugin interfaces (`internal/plugin`) follow a **separate, more conservative** schedule:
- Interfaces are never changed without a 2-minor-release deprecation window.
- New optional methods are added via interface embedding (never mutation of existing interfaces).
- WASM binary compatibility is maintained for the full major version.

---

## 6. Config key migration

Deprecated config keys are read and transparently mapped to their replacements for one full minor release, then hard-removed. The `forge doctor` command will report any deprecated keys in use.

---

## 7. Emergency deprecation

In the case of a security vulnerability, a symbol may be hard-removed immediately without the standard deprecation window. Such removals are documented in `SECURITY.md` and a patch release is issued.

---

## 8. Governance

Deprecation proposals require an ADR amendment (see `docs/adr/_TEMPLATE.md`) and approval from two maintainers (see `CODEOWNERS`).

---

*Last updated: v1.0.0 — see CHANGELOG.md for history.*
