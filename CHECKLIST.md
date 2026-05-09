# Forge — Pre-Ship Checklist

> **Who uses this?** Anyone landing a change into `main` — whether they are the
> Founder, a Core Team maintainer, or an invited contributor.  
> **Reference:** spec §16.5.4 gates 1–12, `CONTRIBUTING.md`, `DEVELOPMENT_PLAN.md`.

Run `make check` locally before pushing. The pre-push git hook (`make hooks`) runs
the same gates automatically. CI must be green before merging.

---

## § A — Change specification

- [ ] **A1 — Spec present.**  
  `.forge/specs/<change-name>/spec.md` exists and describes intent, acceptance
  criteria, and which framework principle the change serves.  
  *Exemption:* typos, dependency bumps, log-string changes → use `forge ship --quick "..."`.

- [ ] **A2 — Spec references the correct spec/ADR section.**  
  The spec cites at least one section from `FORGE_FRAMEWORK_SPEC.md` or `ARCHITECTURE.md`.

---

## § B — Tests

- [ ] **B1 — Tests precede code (or are co-committed).**  
  Every new test file's git timestamp is ≤ the production code it covers.  
  CI enforces this; `make check` reports violations.

- [ ] **B2 — All test types that apply are present.**  
  Consult the 9-point matrix in `tasks/DEVELOPMENT_TASKS.md`:  
  happy · boundary · negative · idempotency · concurrency · cross-tenant ·
  regression · data-accuracy · false-positive guard.

- [ ] **B3 — `go test -race ./...` is green.**

---

## § C — Scans (run locally: `forge scan all --since main`)

- [ ] **C1 — No new secret findings.**  
  `forge scan secrets --since main` exits 0.  
  *Waiver path:* `.forge/waivers/<rule-id>.json` with `rationale` + `expires`.

- [ ] **C2 — No new RLS/authz findings.**

- [ ] **C3 — No new prompt-injection findings.**

- [ ] **C4 — No new supply-chain findings.**

*Any finding requires either a fix or a waiver with expiry ≤ 90 days. Waivers
are reviewed at each milestone.*

---

## § D — Lint & conventions

- [ ] **D1 — `golangci-lint run` exits 0.**  
  Includes `gosec` (G306 etc.), `revive`, and the project-specific rules.

- [ ] **D2 — `gofmt -l ./...` and `goimports -l ./...` output nothing.**

- [ ] **D3 — Convention lint clean.**  
  `forge lint` exits 0 (checks manifest markers, gitignore managed block, etc.).

---

## § E — Public API & backward compatibility

- [ ] **E1 — Public API delta declared.**  
  Any exported symbol removed or signature changed requires a `BREAKING.md` entry
  with a migration guide.

- [ ] **E2 — Deprecated verbs keep their alias for one minor version.**  
  If you rename or remove a CLI verb, add an alias that prints a deprecation warning
  and delegates to the new implementation.

---

## § F — Token budget

- [ ] **F1 — `forge eval` regression ≤ 10%.**  
  Run `make bench` and compare against the baseline in `bench/baseline.json`.  
  A ≥ 10 % increase requires an explicit comment in the PR description citing the reason.

---

## § G — Documentation

- [ ] **G1 — Inline docs updated.**  
  Every exported symbol touched has a doc comment that reflects the new behaviour.

- [ ] **G2 — Task tracker updated.**  
  The relevant `tasks/DEVELOPMENT_TASKS.md` row is marked ✅ and the acceptance note
  cites the commit SHA.

- [ ] **G3 — `make docs-check` exits 0.**  
  The error-code doc matches the registered codes; no orphan code in the doc.

---

## § H — Commit hygiene

- [ ] **H1 — DCO sign-off present on every commit.**  
  `git log --oneline` shows no commit without `Signed-off-by:`.

- [ ] **H2 — Commit message follows Conventional Commits.**  
  Format: `<type>(<scope>): <summary>` — types: `feat`, `fix`, `docs`, `chore`,
  `refactor`, `test`, `ci`.  
  *Bug fixes* must include a `Fixes: #NNN` trailer.

- [ ] **H3 — No merge commits in the PR branch.**  
  Rebase on `main` before opening the PR.

---

## § I — Repo hygiene

- [ ] **I1 — `forge clean --check` exits 0.**  
  No unmanaged scratch or LLM-generated temp files are tracked.

- [ ] **I2 — No secret files tracked.**  
  `forge clean --check` also verifies that `.env`, `.env.local`, and similar files are
  in `.gitignore` and not tracked by git.

- [ ] **I3 — `forge doctor` exits 0.**  
  Local environment is healthy; no `.gitignore` managed-block drift reported.

---

## § J — Security review (T1 core changes only)

- [ ] **J1 — Threat-model updated if attack surface changed.**  
  `THREAT_MODEL.md` threat table links to the new mitigation test.

- [ ] **J2 — `govulncheck ./...` exits 0.**

---

## § K — Release gate (tag only)

> Only required when cutting a release tag. Skip for normal PRs.

- [ ] **K1 — `CHANGELOG.md` entry written.**
- [ ] **K2 — Version constant updated in `cmd/forge/main.go`.**
- [ ] **K3 — `make release-check` exits 0.**
- [ ] **K4 — Release artifact signature verified (`cosign verify`).**

---

## Quick reference: gate-to-CI-job mapping

| Gate | `make check` | CI job |
|------|-------------|--------|
| B3 tests | `go test -race` | `test` |
| C1-C4 scans | `forge scan all` | `scan` |
| D1 lint | `golangci-lint` | `lint` |
| D2 format | `gofmt`+`goimports` | `lint` |
| G3 docs | `make docs-check` | `docs` |
| J2 vuln | `govulncheck` | `vuln` |

See `.github/workflows/ci.yml` for the full matrix.
