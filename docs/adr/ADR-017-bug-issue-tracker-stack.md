# ADR-017 — Bug/issue tracker stack

- **Status:** Proposed
- **Tracker:** ARCH-DEC-17
- **Spec/Arch anchor:** Arch §18.1, §18.3, §18.4
- **Decision date:** TBD
- **Deciders:** DevSecOps + Community WG
- **Consulted:** Founder, security WG

## Context

Forge needs a single, low-friction place for the public bug funnel. It must:

- Be where contributors already are.
- Carry templates for bug / vulnerability / flake / incident.
- Auto-apply severity-guess + area-guess labels.
- Enforce a `Fixes: #NNN` trailer on PRs labelled `bug`.
- Provide a documented bypass procedure (`gate-bypass`) requiring two-maintainer approval.

## Decision

The bug tracker stack is **GitHub Issues** + **GitHub Actions auto-triage bot** + **commit-trailer enforcement**, all configured under `.github/`.

### Components

1. **Issue templates** under `.github/ISSUE_TEMPLATE/`:
   - `bug.yml` — required: `forge --version`, `os`, repro steps, expected, actual, error code if any.
   - `vulnerability.yml` — points to private advisory channel (per ADR-018) and auto-closes if filed publicly.
   - `flake.yml` — for eval/test flakiness; auto-links to ADR-023 quarantine doc.
   - `incident.yml` — for production incidents; opens an issue + creates a `docs/postmortems/INC-<n>-DRAFT.md` skeleton.
2. **Severity taxonomy** (labels):
   - `severity:S0` (data loss / wide outage), `severity:S1` (functional break), `severity:S2` (degraded), `severity:S3` (cosmetic), `severity:S4` (info).
   - First-response SLAs: S0 = 1 h, S1 = 8 h, S2 = 2 business days, S3/S4 = best effort.
3. **Area labels** auto-applied from CODEOWNERS path matching (`area:scan`, `area:llm-gateway`, `area:hygiene`, …).
4. **Auto-triage bot** (single GH Action `triage.yml`) runs on `issues.opened`:
   - Applies `severity:?` label requesting human confirmation if not given by the template.
   - Applies area label(s).
   - Posts a templated comment with SLA + escalation path.
5. **`Fixes:` trailer enforcement** (Action `pr-trailer-check.yml`):
   - PRs labelled `bug` must contain `Fixes: #NNN` (or `Closes:`/`Resolves:`) referencing an open `bug`-labelled issue.
   - PRs not labelled `bug` (`docs`, `chore`, `feat`, `refactor`, `test`) are exempt.
6. **`gate-bypass` workflow**:
   - PR carries label `gate-bypass` only when an existing CI gate must be skipped (e.g. emergency hotfix).
   - Action `gate-bypass-approval.yml` requires ≥ 2 maintainer approvals AND links to a tracking incident issue.
   - The PR description must include a `Bypass-rationale:` line (linted).
   - The post-mortem template (per ADR-020) must list the bypass in §3 (timeline) and §6 (action items).

### Non-goals

- No paid issue tracker pre-1.0.
- No requirement for bug reporters to have a GitHub account beyond what GitHub already requires; anonymous reports are accepted via the security advisory channel only.

## Alternatives considered

### Option A — Linear or Jira (rejected)

Pros: rich workflow.
Cons: paid tools; closed to non-team contributors; OSS norm violation.

### Option B — GitLab Issues (rejected)

Pros: single forge.
Cons: existing repo lives on GitHub; migration cost not justified pre-1.0.

### Option C — Open-source self-hosted tracker (Plane/Redmine) (rejected)

Pros: full control.
Cons: ops cost; community friction.

## Consequences

### Positive

- Zero new infrastructure; bot is just a workflow file.
- Triage data flows directly into OPS-19 monthly bug-lifecycle dashboard.
- `Fixes:` trailer + sub-issue links keep regression tests provably traceable.

### Negative / accepted trade-offs

- GitHub-platform lock-in for issues; mitigated by `gh issue list` JSON export held in `OPS-19` archive.
- Auto-triage label is best-effort; a human triager always confirms within SLA.

### Follow-ups created

- DEV-M1-41 — issue templates + auto-triage bot + trailer enforcement.
- OPS-19 — monthly bug-lifecycle dashboard.
- ADR-022 — two-key rule (consumed by `gate-bypass`).

## Compliance hooks

- CI gate: `pr-trailer-check.yml` on every PR.
- CI gate: `gate-bypass-approval.yml` on PRs labelled `gate-bypass`.
- Test: synthetic stealth-fix PR is blocked (TEST-29 TC-29-07).

## References

- Arch §18.1, §18.3, §18.4.
- Conventional Commits (prior art for trailers): <https://www.conventionalcommits.org/>.
