# Forge — Launch Tasks

> Companion to `../GO_TO_COMMUNITY_PLAN.md`.
> Tracker for the Section-D tasks (LAUNCH-S{0..5}-NN) from the master breakdown.

ID and conventions follow `ARCHITECTURE_TASKS.md`.

---

## S0 — Stealth (LAUNCH-S0-01..04)

| ID | Task | Owner | Acceptance |
|----|------|-------|------------|
| LAUNCH-S0-01 | Identify + invite 3 critic reviewers (solo dev / platform engineer / security skeptic) | Founder | Confirmed yes from each |
| LAUNCH-S0-02 | Run 60-min spec-review session with each | Founder | Notes captured per critic; objections logged in spec §8 |
| LAUNCH-S0-03 | Close ≥80% of spec §8 open questions | Founder + WG leads | §8 status table updated |
| LAUNCH-S0-04 | Draft launch essay (do not publish) | Founder | First draft committed to private repo |

---

## S0-DOC — Community Documentation Foundation (LAUNCH-S0-DOC-01..12)

> **Prerequisites for S1.** Early adopters and contributors cannot self-serve without these docs.
> Owner: Founder + Tech writer. Reference files: `CONTRIBUTING.md`, `README.md`, `docs/`.

| ID | Document | Audience | Acceptance |
|----|----------|----------|------------|
| LAUNCH-S0-DOC-01 | `GETTING_STARTED.md` — zero to first `forge ship` in < 10 min | Early adopters | ✅ File committed to repo root |
| LAUNCH-S0-DOC-02 | `CONTRIBUTING.md` deep expansion — prerequisites, pre-push gate steps, how to add a verb, test expectations, DCO sign-off | Contributors | New contributor follows the guide end-to-end without Slack pings; `make test-all` documented and green |
| LAUNCH-S0-DOC-03 | `docs/PLUGIN_AUTHORING.md` — scaffold a plugin, declare capabilities, write compliance tests, publish to Registry | Plugin authors | Author follows the guide to publish a stub plugin end-to-end; `forge plugin list` shows it |
| LAUNCH-S0-DOC-04 | `docs/VERBS.md` — reference table: one row per verb with synopsis, all flags, examples, and error codes | All users | Every verb in `forge --help` has a matching row; reviewed by a non-author dev |
| LAUNCH-S0-DOC-05 | `CODE_OF_CONDUCT.md` — Contributor Covenant 2.1; enforcement email configured | Community | File committed to repo root; enforcement email is monitored before S1 invites go out |
| LAUNCH-S0-DOC-06 | `docs/INSTALLATION.md` — `go install`, Homebrew tap, Scoop/winget tap, and binary-release paths | All users | Each install path validated on a clean machine or CI matrix job |
| LAUNCH-S0-DOC-07 | `LICENSE` + `NOTICE` review — Apache-2.0 header present in every `.go` source file; `NOTICE` enumerates all third-party deps | Legal / contributors | `go-licenses` (or equivalent) reports zero unresolved deps; CI gate added |
| LAUNCH-S0-DOC-08 | `docs/ARCHITECTURE_OVERVIEW.md` — 2-page onboarding summary of `ARCHITECTURE.md` for first-time contributors | Contributors | Reviewed by a first-time contributor; they can navigate the codebase after reading it without further assistance |
| LAUNCH-S0-DOC-09 | `.github/ISSUE_TEMPLATE/{bug,vulnerability,flake,incident}.yml` — structured issue templates (DEV-M1-41) | All users | Each template tested end-to-end on a staging repo; vulnerability template auto-closes to private security advisory |
| LAUNCH-S0-DOC-10 | `.github/PULL_REQUEST_TEMPLATE.md` — PR checklist: tests added, `Fixes: #NNN` trailer, `Signed-off-by`, scope declaration | Contributors | Template rendered on every new PR; each item links to the relevant `CONTRIBUTING.md` section |
| LAUNCH-S0-DOC-11 | `CHECKLIST.md` — manual `forge ship` pre-flight checklist (DEV-M0-20 pre-automation stand-in) | Maintainers | ✅ File committed to repo root; referenced from `CONTRIBUTING.md` |
| LAUNCH-S0-DOC-12 | `docs/ERROR_CODES.md` — every `FORGE-XXXX` code with description, typical trigger, and resolution steps | Developers | ✅ Generated from `cmd/gen-errors` output; `make docs-sync` CI gate validates freshness |

---

## S1 — Friends & Family Alpha (LAUNCH-S1-01..09)

| ID | Task | Owner | Acceptance |
|----|------|-------|------------|
| LAUNCH-S1-01 | Curate alpha invite list (10–20 devs across §11.1.1 personas) | Founder | List committed (private) |
| LAUNCH-S1-02 | Personal invite email template + send | Founder | All sent within 1 week of M0 GA |
| LAUNCH-S1-03 | Stand up private Discord/Slack channel | Community lead | Channel + welcome + code-of-conduct posted |
| LAUNCH-S1-04 | Schedule + run 1:1 onboarding call per alpha (30 min each) | Founder | Recording or notes per call |
| LAUNCH-S1-05 | Time-to-first-`ship` metric instrumented | DevSecOps | Dashboard widget live |
| LAUNCH-S1-06 | Issue-volume + drop-off tracking | Community lead | Weekly digest |
| LAUNCH-S1-07 | Closing 30-min interview with each alpha | Founder | Synthesis doc per interview |
| LAUNCH-S1-08 | Top-10 cliffs ticketed in DEV-M1 backlog | Founder | Backlog updated |
| LAUNCH-S1-09 | Decide go/no-go for S2 based on exit criteria | Founder | Documented decision |

---

## S2 — Private Beta (LAUNCH-S2-01..12)

| ID | Task | Owner | Acceptance |
|----|------|-------|------------|
| LAUNCH-S2-01 | Public landing page (waitlist-only) | Founder + designer | Page live; analytics in place |
| LAUNCH-S2-02 | Waitlist signup → invite-batch automation (25/week) | DevSecOps | First batch sent |
| LAUNCH-S2-03 | Beta onboarding doc + private docs site | Tech writer | Linked from invite email |
| LAUNCH-S2-04 | Office-hour cadence (2x/week, alternating timezones) | Founder + maintainer | Calendar published |
| LAUNCH-S2-05 | Triage SLA practice (per §16.5.7) — drill with team | Maintainer team | Drill recording reviewed |
| LAUNCH-S2-06 | "What shipped this week" newsletter | Tech writer | First issue out within 2 weeks of S2 start |
| LAUNCH-S2-07 | Anonymous-telemetry opt-in flow tested with beta cohort | DevSecOps | ≥30% opt-in rate |
| LAUNCH-S2-08 | Learning-loop opt-in flow tested with beta cohort | DevSecOps | ≥10% opt-in; privacy invariant green |
| LAUNCH-S2-09 | Eval harness reaches ≥80% scenario coverage | Quality lead | Coverage report committed |
| LAUNCH-S2-10 | First case study (a beta user) drafted | Tech writer | Draft reviewed |
| LAUNCH-S2-11 | Identify ≥5 "champion" beta users for S3 amplification | Community lead | Named list + each has agreed |
| LAUNCH-S2-12 | Decide go/no-go for S3 based on exit criteria | Founder | Documented decision |

---

## S3 — Public Beta launch (LAUNCH-S3-01..20)

| ID | Task | Owner | Acceptance |
|----|------|-------|------------|
| LAUNCH-S3-01 | Finalize launch essay; review by 5 voices | Founder | Final draft signed off |
| LAUNCH-S3-02 | Hero demo video (90 sec) | Founder + editor | Captioned; under 100 MB |
| LAUNCH-S3-03 | 3 deep-dive demo videos (`new`, `ship`, `scan-fix`) | Founder | Each 2–4 min |
| LAUNCH-S3-04 | README rewrite (lead with `forge ship`; one-paste install) | Founder + tech writer | Reviewed by 3 devs unfamiliar with the project |
| LAUNCH-S3-05 | Plugin Registry seeded with ≥3 community plugins | Community lead | Public listing |
| LAUNCH-S3-06 | RFC #1 (workflow) + RFC #2 (plugin interface) merged | Founder + WG | Merged; visible in `rfcs/` repo |
| LAUNCH-S3-07 | Pre-brief 5 friendly journalists/podcasters under embargo | Founder | Embargo agreements in writing |
| LAUNCH-S3-08 | `BENCHMARKS.md` published from eval harness data | Quality lead | Reproducible commands documented |
| LAUNCH-S3-09 | Status page live + incident runbook | DevSecOps | Tabletop exercise completed |
| LAUNCH-S3-10 | Day-1 staffing rota (3 maintainers) | Founder | Calendar shared |
| LAUNCH-S3-11 | Repo flipped public + tag `v0.M2.0` | Founder | Public; release notes published |
| LAUNCH-S3-12 | Launch essay published | Founder | Live URL |
| LAUNCH-S3-13 | Show HN post (founder, with disclosure) | Founder | Posted |
| LAUNCH-S3-14 | Lobsters post (community member, not founder) | Community member | Posted |
| LAUNCH-S3-15 | Pre-briefed creators publish | Creators | Each piece tracked |
| LAUNCH-S3-16 | Day-1 issue-triage SLA: every top-level HN/Lobsters comment replied within 30 min | Founder + 2 maintainers | Tracked in incident channel |
| LAUNCH-S3-17 | Day-1 retro at end of day | Founder + maintainers | Notes + Day-2 plan |
| LAUNCH-S3-18 | Daily "what we shipped today" thread for launch week | Community lead | 7 threads |
| LAUNCH-S3-19 | "First 7 days" video series (one per day) | Tech writer + founder | 7 videos published |
| LAUNCH-S3-20 | +30d review against success criteria (§7.4) | Founder | Public retro published |

---

## S4 — 1.0 Launch (LAUNCH-S4-01..10)

| ID | Task | Owner | Acceptance |
|----|------|-------|------------|
| LAUNCH-S4-01 | "Forge 1.0" announcement post | Founder | Co-authored with one community maintainer |
| LAUNCH-S4-02 | Conference talk #1 (DevX/AI tooling track) | Founder | Talk delivered + recording posted |
| LAUNCH-S4-03 | Conference talk #2 | Founder or maintainer | Talk delivered |
| LAUNCH-S4-04 | Conference talk #3 | Founder or maintainer | Talk delivered |
| LAUNCH-S4-05 | Podcast tour: 5–8 shows | Founder + 1 maintainer | All recorded + released |
| LAUNCH-S4-06 | Sponsorship program announcement (GitHub Sponsors / OpenCollective) | Founder | Page live; transparent split rules |
| LAUNCH-S4-07 | First T2 maintainers nominated + confirmed | Working group leads | Roster published |
| LAUNCH-S4-08 | First Working-Group rosters published | Founder | Public |
| LAUNCH-S4-09 | Quarterly RFC office hours scheduled (next 4 quarters) | Founder | Calendar published |
| LAUNCH-S4-10 | +90d review against success criteria | Founder | Public retro |

---

## S5 — Foundation handoff (LAUNCH-S5-01..06)

| ID | Task | Owner | Acceptance |
|----|------|-------|------------|
| LAUNCH-S5-01 | Trigger evaluation: ≥10K stars OR ≥100 active maintainers | Founder | Signal acknowledged |
| LAUNCH-S5-02 | Foundation conversations (CNCF / OpenJS / Linux Foundation) | Founder + legal advisor | Decision documented |
| LAUNCH-S5-03 | Steering committee election process | Community | 7 members elected |
| LAUNCH-S5-04 | Founder transitions to "BDFL emeritus" | Founder | Charter updated |
| LAUNCH-S5-05 | Foundation legal entity established (if applicable) | Legal advisor | Filings complete |
| LAUNCH-S5-06 | Public handoff post | Founder + steering committee | Published |

---

*Task file version: 0.1 — companion to spec v0.10.6.*
