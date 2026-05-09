# ADR-024 — Reversibility contract

- **Status:** Proposed
- **Tracker:** ARCH-DEC-24
- **Spec/Arch anchor:** Arch §17.1 #5 (reversibility), Arch §17.4 (CI invariants)
- **Decision date:** TBD
- **Deciders:** Core engineer
- **Consulted:** Quality WG, security WG

## Context

The §17.1 resilience contract requires every state-mutating operation to be reversible. Without an explicit contract:

- "Reversible" means different things in different subsystems (FS = trash; DB = down-migration; scan-fix = ad hoc).
- Cross-platform safe-delete pitfalls (Windows file handles, MAX_PATH, case-insensitive FS) silently corrupt undo state.
- Retention windows are inconsistent.

## Decision

Forge guarantees reversibility through a **`.forge/trash/<run-id>/` per-invocation trash directory** plus a **`forge undo <run-id>`** verb that dispatches to per-domain inverse handlers. The contract spans three domains:

### Domains + inverse semantics

| Domain | Forward op | Inverse op | Trash artefact |
|--------|-----------|-----------|----------------|
| **FS** | write/move/delete a file | restore prior bytes (or recreate as deleted) | full byte-copy in `.forge/trash/<run-id>/fs/<path>` |
| **DB migration** | apply forward migration | apply paired down-migration | `.forge/trash/<run-id>/db/<migration-id>.down.sql` (pinned at apply time) |
| **Scan `--apply`** | rewrite source files | restore prior bytes | covered by FS domain |

Each inverse handler is registered via a `Reversible` trait; a forward op without a registered inverse is a **lint error** (`forge-lint::reversibility`).

### Retention policy

- **Default:** 14 days from `run-id` creation.
- **Configurable** per-workspace in `forge.toml` → `[reversibility] trash_retention_days = 14`.
- Garbage collection runs on every `forge` invocation (cheap stat) and on a nightly `forge gc` if scheduled.
- A retained `run-id` survives at least one full retention window; mid-window deletion is forbidden (lint check).
- A `retention_extended_until` field can pin a `run-id` past the default window (e.g. for active-incident debugging).

### Cross-platform safe-delete strategy

- **POSIX:** `fs::rename` into `.forge/trash/<run-id>/fs/`; preserves inode + permissions.
- **Windows:** `fs::rename` then defer file-handle release via `MoveFileEx(MOVEFILE_REPLACE_EXISTING | MOVEFILE_WRITE_THROUGH)`; long-path support via `\\?\` prefix; on `ERROR_SHARING_VIOLATION`, retry under exponential backoff (≤ 3 attempts) before failing with `FORGE-2601`.
- **Case-insensitive FS guard:** trash path includes original-case original-path AND a SHA-prefix to disambiguate two files differing only in case.
- Symlinks: trashed as the link, not the target.

### `forge undo` semantics

- `forge undo <run-id>` — full reverse of the run.
- `forge undo <run-id> --domain fs` — partial reverse of one domain.
- `forge undo <run-id> --dry-run` — print the inverse plan; no writes.
- Idempotent: undoing an already-undone run is a no-op (records a `noop` ledger entry).
- Concurrency: serialised via the same advisory lock as forward `--apply` (DEV-M2-22).
- A protected branch (per ADR-022) refuses undo without `--force` + two-Maintainer review.

### Audit ledger interaction

Every forward + inverse op writes a signed entry to the audit ledger (per Arch §17.2 row 5 + ADR-022). A run-id's complete lineage (`apply ←→ undo ←→ undo-of-undo`) is reconstructible by ledger query.

## Alternatives considered

### Option A — Native FS undelete (e.g. `trash-cli`) (rejected)

Pros: zero implementation.
Cons: cross-platform inconsistency; no DB or scan-fix coverage.

### Option B — Snapshot-based reversibility (zfs/btrfs snapshots) (rejected)

Pros: atomic.
Cons: requires specific FS; not portable.

### Option C — Git-based snapshots (`git stash`-like) (rejected)

Pros: dev-familiar.
Cons: doesn't cover DB; pollutes user's git history.

## Consequences

### Positive

- One mental model for "undo" across FS, DB, and scan-fix.
- Inverse handlers are first-class and lint-enforced.
- Cross-platform pitfalls handled at the foundation layer.

### Negative / accepted trade-offs

- Disk usage for trash; mitigated by retention default + per-workspace cap (`max_trash_bytes` in `forge.toml`).
- DB migrations without a paired down-migration cannot be applied — accepted as a deliberate friction.

### Follow-ups created

- DEV-M3-22 — reversibility contract implementation.
- DEV-M2-22 — migration-runner pairing.
- ADR-016 — register entry FR-NNN (reversibility failures).

## Compliance hooks

- Lint: `forge-lint::reversibility` rejects forward ops without an inverse.
- Test: round-trip apply→undo→assert byte-identical state per domain (DEV-M3-22 TC-22-01).
- Test: Windows long-path round-trip (DEV-M3-22 TC-22-06).
- Test: undo of undo is a no-op (DEV-M3-22 TC-22-04).

## References

- Arch §17.1 #5, §17.4.
- ADR-022 (two-key on protected-branch undo).
