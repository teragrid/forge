# ADR-011 — Hygiene-manifest schema

- **Status:** Proposed
- **Tracker:** ARCH-DEC-11
- **Spec/Arch anchor:** Spec §4 (Repo Hygiene Layer), Spec §16.5.4 #11
- **Decision date:** TBD
- **Deciders:** Core engineer
- **Consulted:** Community WG, plugin WG

## Context

`forge upgrade hygiene` rewrites managed blocks of `.gitignore`, `.gitleaks.toml`, `.editorconfig`, and similar repo-hygiene files. A central manifest must declare:

- Which fragments compose for which detected stack(s).
- Who owns each fragment (for review / on-call routing).
- The version stamp that lets `forge doctor` detect drift.
- Ownership boundary between Forge-managed bytes and user-editable bytes.

## Decision

Forge will read a single per-workspace **`hygiene.manifest.yml`** (committed to the user's repo) plus per-fragment manifest files in the Forge install (and in plugins). The schema is published as `forge/schemas/hygiene-manifest.schema.json` and version-tagged.

### Workspace manifest (`hygiene.manifest.yml` — written by `forge new` / `forge upgrade hygiene`)

```yaml
api_version: forge.sh/v1
kind: HygieneManifest
metadata:
  forge_version: "0.10.9"
  generated_at: "2026-05-09T00:00:00Z"
spec:
  stacks:
    - nextjs
    - supabase
    - python  # if mixed-stack
  fragments:
    - id: gitignore.nextjs
      version: "1.3.0"
      sha256: <hex>
    - id: gitignore.supabase
      version: "2.0.1"
      sha256: <hex>
    - id: gitleaks.core
      version: "1.0.0"
      sha256: <hex>
  ownership:
    # Forge owns: bytes between the `# >>> forge:<id> v<ver>` and `# <<< forge:<id>` markers.
    # User owns: everything else, including a `# user-additions` section reserved at the end of each managed file.
    user_block_marker: "# user-additions"
```

### Fragment manifest (in Forge install or plugin)

```yaml
api_version: forge.sh/v1
kind: HygieneFragment
metadata:
  id: gitignore.nextjs
  version: "1.3.0"
  owner: forge-core@forge.sh
spec:
  applies_to:
    files: [".gitignore"]
    detector: stacks.nextjs
  body_path: fragments/gitignore/nextjs.txt
  precedence: 100
  conflicts: []
  successor: null  # for breaking-change migrations
```

### Ownership rules

1. **Managed bytes** (between markers) are rewritten by `forge upgrade hygiene` without warning.
2. **User-additions block** (between `# user-additions` markers) is preserved byte-identically across upgrades.
3. **Outside any marker** = user-owned; Forge never touches.
4. Drift inside a managed block (detected by SHA mismatch) is reported by `forge doctor` and refuses upgrade without `--force`.
5. When a fragment declares `successor`, `forge upgrade hygiene` migrates and records both old + new IDs in the workspace manifest until the next upgrade.

## Alternatives considered

### Option A — No manifest, infer everything from disk (rejected)

Pros: zero new file in user repos.
Cons: drift detection requires roundtripping the whole hygiene file each run; ownership ambiguity.

### Option B — Per-file sidecars (`.gitignore.forge.json`) (rejected)

Pros: locality.
Cons: clutters root; duplicates metadata; doesn't solve cross-file ordering (e.g. fragment dependencies).

### Option C — Single manifest in user `$HOME` (rejected)

Pros: cleanest workspace.
Cons: workspace-portability lost (clone → state lost); CI cannot reproduce.

## Consequences

### Positive

- One file declares the workspace's hygiene contract; deterministic.
- Per-fragment versioning enables surgical upgrades.
- Plugins can ship their own fragments via the same schema.

### Negative / accepted trade-offs

- Adds one tracked file to user repos; mitigated by being short + diff-friendly.
- Successor migrations need careful authoring → covered by TEST-25.

### Follow-ups created

- DEV-M0-25 — `hygiene.manifest.yml` writer + reader.
- DEV-M0-26 — fragment registry + JSON schema publication.
- TEST-25 — fragment successor-migration regression suite.

## Compliance hooks

- CI gate: `hygiene.manifest.yml` validated by JSON schema on every PR.
- Test: drift detection on a hand-edited managed block (TEST-22 family).
- Test: user-additions block byte-identical across two upgrades (TEST-26).

## References

- Spec §4, §16.5.4 #11.
- Prior art: `editorconfig` ownership conventions.
