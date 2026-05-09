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
| LAUNCH-S0-DOC-01 | `GETTING_STARTED.md` — zero to first `forge ship` in < 10 min | Early adopters | Reviewed by one person with no prior Forge knowledge; they reach a successful `forge ship` without asking for help |
| LAUNCH-S0-DOC-02 | `CONTRIBUTING.md` deep expansion — prerequisites, pre-push gate steps, how to add a verb, test expectations, DCO sign-off | Contributors | New contributor follows the guide end-to-end without Slack pings; `make test-all` documented and green |
| LAUNCH-S0-DOC-03 | `docs/PLUGIN_AUTHORING.md` — scaffold a plugin, declare capabilities, write compliance tests, publish to Registry | Plugin authors | Author follows the guide to publish a stub plugin end-to-end; `forge plugin list` shows it |
| LAUNCH-S0-DOC-04 | `docs/VERBS.md` — reference table: one row per verb with synopsis, all flags, examples, and error codes | All users | Every verb in `forge --help` has a matching row; reviewed by a non-author dev |
| LAUNCH-S0-DOC-05 | `CODE_OF_CONDUCT.md` — Contributor Covenant 2.1; enforcement email configured | Community | File committed to repo root; enforcement email is monitored before S1 invites go out |
| LAUNCH-S0-DOC-06 | `docs/INSTALLATION.md` — `go install`, Homebrew tap, Scoop/winget tap, and binary-release paths | All users | Each install path validated on a clean machine or CI matrix job |
| LAUNCH-S0-DOC-07 | `LICENSE` + `NOTICE` review — Apache-2.0 header present in every `.go` source file; `NOTICE` enumerates all third-party deps | Legal / contributors | `go-licenses` (or equivalent) reports zero unresolved deps; CI gate added |
| LAUNCH-S0-DOC-08 | `docs/ARCHITECTURE_OVERVIEW.md` — 2-page onboarding summary of `ARCHITECTURE.md` for first-time contributors | Contributors | Reviewed by a first-time contributor; they can navigate the codebase after reading it without further assistance |
| LAUNCH-S0-DOC-09 | `.github/ISSUE_TEMPLATE/{bug,vulnerability,flake,incident}.yml` — structured issue templates (DEV-M1-41) | All users | Each template tested end-to-end on a staging repo; vulnerability template auto-closes to private security advisory |
| LAUNCH-S0-DOC-10 | `.github/PULL_REQUEST_TEMPLATE.md` — PR checklist: tests added, `Fixes: #NNN` trailer, `Signed-off-by`, scope declaration | Contributors | Template rendered on every new PR; each item links to the relevant `CONTRIBUTING.md` section |
| LAUNCH-S0-DOC-11 | `CHECKLIST.md` — manual `forge ship` pre-flight checklist (DEV-M0-20 pre-automation stand-in) | Maintainers | Referenced from `CONTRIBUTING.md`; reviewed and signed off by two maintainers |
| LAUNCH-S0-DOC-12 | `docs/ERROR_CODES.md` — every `FORGE-XXXX` code with description, typical trigger, and resolution steps | Developers | Generated from `cmd/gen-errors` output; `make docs-sync` CI gate validates freshness |

---

## S0-PROCESS — Feature & Idea Submission Process (LAUNCH-S0-PROCESS-01..08)

> **How community ideas become framework features.** Documents the full lifecycle from raw idea to shipped capability, including both the community-vote path and the core-team triage path. Must be live before S1 so early adopters have a clear feedback channel from day one.
> Owner: Founder + Community lead. Reference: `GOVERNANCE.md`, `rfcs/`, `.github/ISSUE_TEMPLATE/`.

| ID | Document / Artifact | Audience | Acceptance |
|----|---------------------|----------|------------|
| LAUNCH-S0-PROCESS-01 | `GOVERNANCE.md` — decision-making structure: Core Team composition, Working Groups, BDFL tie-break, voting quorum rules, sponsorship criteria for becoming a maintainer | All community members | File committed to repo root; linked from `CONTRIBUTING.md` and `README.md`; reviewed by one external contributor before S1 |
| LAUNCH-S0-PROCESS-02 | `docs/FEATURE_SUBMISSION.md` — step-by-step guide: (1) search existing issues + RFCs, (2) open a `feature-request` issue using the template, (3) gather community reactions (👍 / 👎 / comment), (4) threshold for automatic promotion to RFC draft (≥10 👍 within 30 days), (5) Core Team triage path for strategic items regardless of vote count | Community members | Validated end-to-end by a non-author: they submit a test idea, see it triaged, and understand the outcome without asking for help |
| LAUNCH-S0-PROCESS-03 | `.github/ISSUE_TEMPLATE/feature_request.yml` — structured template: problem statement, proposed solution, alternatives considered, use-case persona, willingness to contribute (yes / mentored / no), affected verbs | Community members | Template renders on GitHub with required-field validation; submitted issue automatically receives `triage:new` + area label from auto-triage bot (DEV-M1-41) |
| LAUNCH-S0-PROCESS-04 | `rfcs/README.md` + `rfcs/_TEMPLATE.md` — RFC lifecycle: Draft → Review (14-day comment window) → Final Comment Period (7 days) → Accepted / Rejected / Withdrawn; every accepted RFC gets a tracking issue and a changelog entry | RFC authors | Template is self-contained; a new RFC author can submit without help; at least one synthetic RFC goes through the full lifecycle as a proof-of-process before S1 |
| LAUNCH-S0-PROCESS-05 | `docs/ROADMAP.md` — publicly visible, rolling 3-milestone roadmap: each item tagged with origin (`core-team`, `community-vote`, `RFC-NNN`, or `pilot-user`), current status (planned / in-progress / shipped / deferred), and the GitHub issue or RFC that tracks it | All users | Roadmap updated at the start of each milestone; stale items (>90 days without status change) trigger an automated staleness label; reviewed by one community champion at S2 |
| LAUNCH-S0-PROCESS-06 | **Community-vote mechanism**: GitHub Reactions (👍) as the lightweight ballot; auto-promote script runs weekly, converts issues with ≥10 👍 and no `wont-fix` label into RFC drafts and posts a summary comment | Community members | Script tested on a staging repo: seeded issue reaches threshold → RFC draft opened automatically; below-threshold issue → no action; `wont-fix` label → suppressed |
| LAUNCH-S0-PROCESS-07 | **Core-team triage cadence**: bi-weekly triage meeting (async-friendly: async vote in a pinned thread, 48-hour window, majority of active Core Team members); triage outcomes recorded in a public `triage-log.md` in the repo | Core Team + observers | First triage meeting conducted before S1; `triage-log.md` has ≥1 entry with outcome (`accept` / `defer` / `reject` + rationale); log visible to the community |
| LAUNCH-S0-PROCESS-08 | `docs/DECISION_RECORD.md` — explains when to use ADR vs. RFC vs. `feature_request` issue: ADR = internal implementation decision by maintainers; RFC = user-visible interface or protocol change; feature_request = idea needing community validation before design work begins; includes a 3-question decision tree | Contributors | Decision tree correctly routes 5 synthetic scenarios (boundary test) without ambiguity; cross-linked from `CONTRIBUTING.md`, `GOVERNANCE.md`, and the RFC template |

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
