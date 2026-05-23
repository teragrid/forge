# forge bugfix

Post-delivery bug fix workflow: diagnose a bug, generate a surgical patch, and write a regression test — all from one command.

## Synopsis

```
forge bugfix (--bug "<description>" | --finding <id> | --test "<pattern>")
             [--root <path>] [--apply] [--json]
```

Exactly one of `--bug`, `--finding`, or `--test` must be provided.

## Description

`forge bugfix` covers the gap between `forge fix` (which fixes scan findings) and a raw code edit. It targets bugs that appear **after** initial delivery — caught during code review, found via a failing test, or reported as a plain-language description.

With an LLM configured, Forge:

1. Analyses the codebase context (`.forge/context.md` + source) to identify the root cause.
2. Generates a minimal, surgical patch targeting exactly the defective code.
3. Writes a regression test that would have caught the original bug.
4. Appends a record to `.forge/audit.log` (timestamp, source, root cause, file).

**Dry-run by default.** Nothing is written to disk until you add `--apply`.

## Sources

| Flag | What it accepts |
|---|---|
| `--bug "<text>"` | A plain-language bug description, e.g. `"Checkout total is wrong when a discount code is applied"` |
| `--finding <id>` | A finding ID from `.forge/review-results.json` (produced by `forge review`) |
| `--test "<pattern>"` | The name or pattern of a failing test, e.g. `"TestCheckout_DiscountApplied"` |

## Flags

| Flag | Default | Description |
|---|---|---|
| `--root` | `.` | Root of the project to analyse |
| `--apply` | false | Write the patch and regression test to disk; append to `.forge/audit.log` |
| `--json` | false | Output structured JSON instead of human-readable text |

## Examples

```bash
# Dry-run from a plain bug report
forge bugfix --bug "Checkout total is wrong when a discount code is applied"

# Dry-run from a failing test
forge bugfix --test "TestCheckout_DiscountApplied"

# Apply a fix from a review finding
forge bugfix --finding FORGE-REV-003 --apply

# Output structured JSON (useful in CI or scripts)
forge bugfix --bug "Login fails for SSO users" --json
```

## Output (text mode)

```
○ root cause  nil pointer dereference in discount.Apply() when coupon.Percent is unset
✓ fix         pkg/checkout/discount.go  (confidence: high)
  ─── patch preview ────────────────────────────────────────────
  -  total -= coupon.Percent * total / 100
  +  if coupon != nil && coupon.Percent > 0 {
  +      total -= coupon.Percent * total / 100
  +  }
  ──────────────────────────────────────────────────────────────
✓ test        pkg/checkout/discount_test.go  (regression guard generated)

run with --apply to write changes to disk
```

## Output (JSON mode, `--json`)

```json
{
  "root":            ".",
  "mode":            "dry-run",
  "source":          "bug",
  "input":           "Checkout total is wrong when a discount code is applied",
  "root_cause":      "nil pointer dereference in discount.Apply() when coupon.Percent is unset",
  "fix": {
    "file":          "pkg/checkout/discount.go",
    "patch":         "...",
    "confidence":    "high"
  },
  "regression_test": {
    "file":          "pkg/checkout/discount_test.go",
    "code":          "..."
  },
  "applied":         false,
  "summary":         "Guarded coupon.Percent access; added nil-check before discount calculation."
}
```

## Audit log

When `--apply` is used, Forge appends an entry to `.forge/audit.log`:

```
[2026-05-24T14:32:01Z] bugfix | source=bug | input="Checkout total is wrong..." | root_cause="nil pointer dereference in discount.Apply()" | file=pkg/checkout/discount.go
```

## Error codes

| Code | Meaning |
|---|---|
| `FORGE-6550` | Bugfix operation failed (LLM error or patch generation failure) |
| `FORGE-6551` | No source flag provided — supply `--bug`, `--finding`, or `--test` |
| `FORGE-6552` | Finding ID not found in `.forge/review-results.json` |

## See also

- [`forge fix`](fix.md) — auto-apply fixes for `forge scan` findings
- [`forge review`](../VERBS.md) — code review that produces finding IDs
- [`forge scan`](scan.md) — security and quality scans
- [`forge incident`](incident.md) — structured incident management
