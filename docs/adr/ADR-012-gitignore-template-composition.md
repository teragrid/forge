# ADR-012 — `.gitignore` template composition

- **Status:** Proposed
- **Tracker:** ARCH-DEC-12
- **Spec/Arch anchor:** Spec §4 Repo Hygiene Layer (`.gitignore` standards), ADR-011
- **Decision date:** TBD
- **Deciders:** Core engineer
- **Consulted:** Community WG

## Context

The de-facto practice of pasting one giant `.gitignore` per stack from `gitignore.io` produces:

- Stale rules with no upgrade path.
- Conflicts when projects mix stacks (Next.js + Python tooling + Terraform).
- No way to safely update Forge-shipped rules without clobbering user additions.

ARCH-DEC-11 establishes the manifest + ownership model. This ADR defines **how fragments compose into the managed block** and how the **version stamp + markers** work.

## Decision

`.gitignore` files written by Forge use **per-stack fragments concatenated in declared `precedence` order between machine-readable markers**, plus a trailing user-additions block.

### File layout

```gitignore
# ============================================================
# >>> forge:gitignore v1.0.0  (manifest sha256: <hex>)
# DO NOT EDIT THIS BLOCK BY HAND.
# Run `forge upgrade gitignore` to update; `forge doctor` to check drift.
# ============================================================

# >>> forge:gitignore.core v1.0.0
.DS_Store
*.swp
# <<< forge:gitignore.core

# >>> forge:gitignore.nextjs v1.3.0
node_modules/
.next/
# <<< forge:gitignore.nextjs

# >>> forge:gitignore.supabase v2.0.1
supabase/.branches/
supabase/.temp/
# <<< forge:gitignore.supabase

# ============================================================
# <<< forge:gitignore
# ============================================================

# >>> user-additions
# Anything below this line is yours. Forge will preserve it byte-identically.
my-local-scratch/
# <<< user-additions
```

### Composition rules

1. Fragment order is the sorted ascending order of `precedence` (ties broken by `id`).
2. A fragment with `conflicts: [<id>]` cannot be composed with the listed IDs; resolution = pick the higher-precedence fragment, log warning, write `# conflict: dropped <id>` line.
3. The outer `forge:gitignore vX.Y.Z` version is bumped on every fragment-set or composition-rule change; SHA-256 of the manifest is embedded.
4. Per-fragment markers are mandatory — they are the unit of drift detection.
5. Lines starting with `# >>>` or `# <<<` outside the user block are reserved; user additions to the user block cannot use these tokens (linter rejects).

### Drift detection

`forge doctor` compares each managed fragment block's SHA-256 against the manifest. Mismatch → drift report. The user-additions block is excluded from drift; its bytes are preserved across upgrade.

## Alternatives considered

### Option A — Single monolithic block, no per-fragment markers (rejected)

Pros: simpler render.
Cons: surgical updates impossible; whole-block diffs noisy; conflict resolution invisible.

### Option B — Per-stack files (`.gitignore.nextjs`) included by Git (rejected)

Pros: clean separation.
Cons: Git does not natively `include` other gitignore files (only via the `core.excludesFile` global config); breaks expectations.

### Option C — A separate sidecar describing managed regions (rejected)

Pros: zero in-file noise.
Cons: drift detection requires reading two files atomically; user editors don't render the relationship.

## Consequences

### Positive

- Surgical upgrades + drift detection are first-class.
- Mixed-stack projects compose without manual merging.
- User-additions block gives users a stable, sacred space.

### Negative / accepted trade-offs

- Marker comments add visual noise; mitigated by a brief header explaining `forge upgrade gitignore`.
- A user who deletes a marker by hand will be told (drift) but not silently fixed; intentional.

### Follow-ups created

- DEV-M0-25 — `.gitignore` fragment registry.
- DEV-M1-37 — `forge upgrade gitignore` writer with marker rules.
- DEV-M1-38 — `forge doctor` drift detection.

## Compliance hooks

- CI gate: rendered `.gitignore` round-trips through the writer with byte-identical output (idempotency).
- Test: hand-edit inside a managed block fails `forge doctor` (DEV-M1-38 TC-38-02).
- Test: user-additions block bytes preserved across two upgrades (TEST-26 TC-26-02).

## References

- Spec §4 Repo Hygiene Layer.
- ADR-011 (hygiene manifest).
