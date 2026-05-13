# Pilot Case Study — Forge in Production

> Early-access pilot programme — Q1 2025

---

## Overview

During the Forge early-access programme, we worked with three pilot teams to
integrate Forge into their existing AI-assisted development workflows. This
document summarises the outcomes, friction points, and lessons learned.

_Note: Company names are anonymised at the request of participants._

---

## Pilot 1 — FinTech API team (12 engineers)

**Stack:** Go microservices, PostgreSQL, deployed on Fly.io  
**AI tooling in use:** GitHub Copilot + occasional GPT-4 sessions  
**Forge verbs adopted:** `scan`, `review`, `ship`, `deploy`

### Challenge

The team was spending 20–30 minutes per PR manually checking for:
- Row-level security policy gaps in Supabase migrations
- Secrets accidentally committed in generated code
- Dependency vulnerabilities introduced by Copilot suggestions

### Outcome after 6 weeks

- **Secrets incidents:** 3 per sprint → 0 (blocked by `forge scan secrets` in pre-commit)
- **PR review cycle:** 40 min avg → 22 min avg (LLM-powered `forge review` pre-screens)
- **Dependency CVEs merged:** 2 per month → 0 (blocked by `forge scan supply-chain`)
- **Developer NPS for Forge:** 8.2 / 10

### Key feedback

> "The `forge ship` checkpoint flow is the right mental model. It forces us to
> think about each stage instead of just hitting deploy."  
> — Lead engineer

---

## Pilot 2 — Healthcare SaaS (5 engineers)

**Stack:** TypeScript / Next.js, Supabase, deployed on Vercel  
**AI tooling in use:** Cursor with Claude  
**Forge verbs adopted:** `scan`, `adopt`, `doctor`, `hygiene`

### Challenge

Team was using Cursor heavily for feature development but had no systematic
way to audit what the AI had generated for PHI handling compliance.

### Outcome after 4 weeks

- `forge adopt` integrated Forge into existing project in < 10 minutes
- `forge scan secrets` caught 2 PHI-adjacent field names in API responses
- `forge hygiene report` gave the CTO a weekly compliance snapshot they could
  share with their HIPAA auditor
- **Blocker found:** `forge doctor` reported missing `.forge/config.toml` scan
  rules on their CI environment — led to a config fix

### Key feedback

> "We didn't realise our Cursor sessions were producing code that logged
> `user.email` in debug paths. Forge caught it before it shipped."  
> — Founder

---

## Pilot 3 — Open-source library maintainer (1 engineer)

**Stack:** Go library, GitHub Actions CI  
**AI tooling in use:** GitHub Copilot  
**Forge verbs adopted:** `scan`, `review`, `postmortem`, `eval`

### Challenge

Solo maintainer wanted to maintain code quality and security posture without
spending hours on manual review of AI-generated contributions.

### Outcome after 3 weeks

- `forge review` on each PR reduced review time from 25 min → 8 min
- `forge eval` regression suite gave confidence that AI-generated refactors
  didn't break existing behaviour
- Identified 1 supply-chain issue in a transitive dependency

### Key feedback

> "As a solo maintainer, `forge review` is like having a junior reviewer on
> call 24/7. It doesn't replace my judgment but it catches the obvious stuff."  
> — Maintainer

---

## Lessons learned

1. **`forge adopt` is the critical first step** — teams that skipped it and
   tried to configure manually had significantly more friction.
2. **Pre-commit hooks drive adoption** — adding `forge scan` to pre-commit
   made the tool habitual rather than optional.
3. **Token budgets need education** — teams were surprised when scans hit the
   T2 tier. Document the budget model prominently.
4. **`forge doctor` is underused** — make it part of onboarding.

---

*If you're interested in the next pilot cohort, see CONTRIBUTING.md.*
