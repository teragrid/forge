# BREAKING.md — Forge Breaking-Change Policy

This document describes how breaking changes are identified, communicated,
and managed across Forge releases (DEV-M0-31).

---

## Versioning model

Forge follows **Semantic Versioning 2.0.0** (semver.org):

| Version component | Meaning |
|---|---|
| `MAJOR` bump | Backwards-incompatible public API or CLI surface change |
| `MINOR` bump | New feature, fully backwards-compatible |
| `PATCH` bump | Bug fix, fully backwards-compatible |

Pre-1.0 releases (`0.y.z`) treat the `MINOR` component as the breaking-change
signal — i.e. a `0.y` → `0.(y+1)` bump may include breaking changes.

### Default-behaviour changes

A change that flips a **default** sits between the rows above, and this section
exists because the table alone gave the wrong answer in practice: it reads as
`MAJOR`, which sets a bar so high that the realistic alternative becomes
shipping the change as a patch and hoping. That is exactly what happened in
1.8.2 (see the worked example below).

A change that alters a default **MAY** ship as `MINOR` when **all** of these
hold:

1. The previous behaviour remains available through a documented opt-out — a
   flag, a config key, or both.
2. The opt-out is named **in the failure message itself**, not only in the
   changelog. Users meet a new default by being broken by it, not by reading
   release notes, so the answer has to be at the point of failure.
3. The failure is loud and specific. A default that changes what a command
   *silently* produces is still `MAJOR`, because no opt-out helps someone who
   never learns they need one.
4. `CHANGELOG.md` carries a `Breaking Changes` section and the commit uses the
   Conventional Commits `!` marker.

Anything that **removes** a capability — a verb, a flag, a config key, an error
code — remains `MAJOR` regardless of how loudly it fails.

### Worked example: the 1.8.2 → 1.9.0 correction

1.8.2 made `four-stage-testing-gate` blocking by default, which fails pipelines
that previously passed, and shipped it as a **patch**. That told every
`^1.8.0` consumer it was a safe automatic upgrade. It was not.

It was re-released unchanged as **1.9.0** to correct the signal. Note what the
correction could and could not do:

- **Fixed:** consumers pinned to `~1.8.1` no longer take the change automatically.
- **Not fixed:** anyone on `^1.8.0` who had already upgraded. npm publishes are
  permanent; a version number cannot be recalled once it is out.

The lesson is in that asymmetry. A version number is a machine-readable promise
made *before* anyone can check it, so the cost of getting it wrong is paid by
users who had no way to know. Decide the component before tagging, not after
the first bug report.

---

## What counts as a breaking change

A breaking change is any change that requires a consumer to modify their
usage in order to continue working correctly. This includes:

### CLI surface breaks
- Removing or renaming a verb (`forge ship`, `forge scan`, etc.)
- Changing the semantics of a positional argument or required flag
- Changing the exit-code contract of a command
- Changing `--json` output field names, types, or removing fields declared in `OutputFields`

### Config / env breaks
- Removing or renaming a `forge.yml` config key
- Removing or renaming a `FORGE_*` environment variable
- Changing the default value of a config key in a way that silently alters behaviour

### Plugin API breaks
- Removing or renaming a hook, event type, or message-schema field
- Changing the plugin discovery path or manifest format

### Error-code breaks
- Removing a published `FORGE-XXXX` error code (consumers may gate on them in CI)
- Changing the numeric value of an existing error code

---

## One-minor alias retention rule

When a verb, flag, config key, or JSON field is renamed:

1. The **old name continues to work** for **one minor version** after the rename.
2. Its use emits a `FORGE-WARN` deprecation notice to stderr.
3. The alias is **removed** in the following minor release.

Example:
- `0.10.0` renames `forge hygiene` → `forge clean` — `forge hygiene` still works with deprecation warning.
- `0.11.0` removes `forge hygiene`.

---

## How to find breaking changes in a release

1. **`CHANGELOG.md`** — every breaking change is in the `Breaking Changes` section
   (commits matching `^.*!:.*$` per `.goreleaser.yml`).
2. **GitHub Release notes** — auto-generated from CHANGELOG, tagged `BREAKING`.
3. **`git log --grep "!:" v0.9.0..v0.10.0`** — all breaking-change commits.
4. **`make changelog`** — regenerates `CHANGELOG.md` from git history locally.

---

## `make changelog` target

The `make changelog` target uses `git-cliff` or `goreleaser changelog` to
regenerate `CHANGELOG.md` from the conventional-commits history:

```sh
make changelog          # regenerate from HEAD → last tag
make changelog TAG=v0.9.0   # regenerate a historical range
```

The target is defined in `Makefile` and is run automatically by the release
pipeline (`.goreleaser.yml` changelog block).

---

## Communicating breaking changes to users

1. **Pre-release deprecation issue**: open a GitHub Issue with label
   `type: deprecation` at least one release cycle before the removal.
2. **Release PR description**: include a "Migration guide" section for any
   MAJOR-bump changes.
3. **Docs update**: update `docs/VERBS.md`, `docs/PLUGIN_AUTHORING.md`, or the
   relevant ADR if the change touches a documented interface.

---

## Emergency breaking changes (security)

Security-driven breaking changes (e.g. removing an insecure default) may be
shipped in a `PATCH` release without the one-minor alias retention rule.
These will always be clearly labelled `SECURITY BREAKING` in the changelog
and release notes.

See `docs/SECURITY.md` for the full vulnerability disclosure and patching policy.

---

## v1.7.0 — LLM-first rearchitecture (piped-output migration)

**Feature:** `internal/llmresponse` + `forge ship --human` + 10 MCP tools
**PR:** `feature/agent-first-rearchitect` → main

### What changed

| Area | Old behaviour | New behaviour |
|------|---------------|---------------|
| stdout (non-TTY / `--json`) | Mixed ANSI text | JSON envelope (one object per line) |
| Interactive gates | `y/N` prompt always printed | Suppressed when `FORGE_LLM_MODE=1` or non-TTY |
| Error output | Plain text | JSON with `code`, `message`, and `remedy` fields |
| MCP tools | 4 tools | 10 tools (`forge_ship_checkpoint`, `forge_get_errors`, `forge_set_budget`, `forge_list_specs`, `forge_get_spec`, `forge_check_health` added) |

### Migration guide (for piped/scripted users)

If you pipe `forge ship` output (e.g. `forge ship … | jq`), the output is
now a JSON envelope when stdout is not a TTY. No flag change needed — the
auto-detection is automatic.

To opt out and keep human-readable text even when piped:

```sh
forge ship spec --name auth-email --human
```

To opt in explicitly from a TTY:

```sh
forge ship code --name auth-email --json
# or:
FORGE_LLM_MODE=1 forge ship code --name auth-email
```

### New `FORGE_LLM_MODE` environment variable

`FORGE_LLM_MODE=1` is the canonical way for LLM agents (Claude, GPT-4o,
Copilot) to signal that they are the consumer. It:
- Emits JSON envelopes on all output
- Suppresses all interactive `y/N` gates (equivalent to `--yes`)
- Disables ANSI colour codes
- Adds `remedy` to every error response
