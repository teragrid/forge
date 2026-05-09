# Forge — Go-to-Community / Launch Plan

> Companion to `FORGE_FRAMEWORK_SPEC.md` v0.10.6, `ARCHITECTURE.md` v0.1, `DEVELOPMENT_AND_TEST_PLAN.md` v0.1.
> Status: **Draft / Pre-RFC**.
>
> Posture: **community-first, commercial-later** (spec §0). This plan optimizes for trust, contributor activation, and a defensible quality narrative — *not* for vanity stars.

---

## 0. North-star metric (NSM)

**Weekly active `forge ship` runs across distinct projects.** (spec §19 NSM)

This single number captures the ultimate goal — *ship fast with high quality, via vibe-coding* — better than stars, downloads, or signups. Every launch tactic is evaluated on whether it moves it.

---

## 1. Stage map

| Stage | Audience | Trigger | Goal |
|-------|----------|---------|------|
| **S0 — Stealth** | Founder + 3 trusted reviewers | spec v1.0 | Pre-RFC critique; close §8 open questions |
| **S1 — Friends & Family Alpha** | 10–20 hand-picked devs | M0 binary published | Find the cliffs in `forge new` and `--quick ship` |
| **S2 — Private Beta** | 100–300 invited devs (waitlist) | M1 exit (full `forge ship`) | Validate token economy, scan engine, plugin loader on real apps |
| **S3 — Public Beta** | Open GitHub + HN/Lobsters launch | M2 (Registry live, 3 community plugins) | Activate the contributor flywheel; first 50 maintainers-in-waiting |
| **S4 — 1.0 Launch** | Broader dev community (conference talks, podcasts) | M3 exit | Establish Forge as a credible category-defining option |
| **S5 — Foundation handoff** | Community-elected stewards | ≥10K stars or ≥100 active maintainers | Move governance to Stage 3 (spec §16.1) |

---

## 2. The narrative (one paragraph, memorize it)

> *Forge is the framework you'd build if you assumed an LLM was sitting next to every developer. One command — `forge ship` — turns intent into specs, then failing tests, then green code, then a reviewed PR. Eight scanners and a convention linter run on every change so quality stays ahead of velocity. Plugins are first-class, governance is tiered, and contribution standards exist to protect users — not to gatekeep contributors. Local-first, single binary, opt-in everything. Ship fast. Ship safe. Ship with vibes.*

This 6-sentence pitch is the source of truth for: README first paragraph, landing page hero, conference talk abstract, every podcast intro.

---

## 3. Channels (ranked by expected NSM lift)

| Rank | Channel | Stage | Why |
|------|---------|-------|-----|
| 1 | **Hand-curated 1:1 outreach** to specific devs | S1, S2 | Trust-building; depth feedback; future maintainers |
| 2 | **Show HN + Lobsters** with a working demo video | S3 | Aligned audience; values reversibility, opt-in, sandboxing |
| 3 | **GitHub README + repo-as-marketing** | S3+ | First impression of seriousness — README mirrors §11.2 principles |
| 4 | **Long-form essay** ("Why we built Forge") on founder's blog | S3 | Owns the philosophical framing |
| 5 | **Recorded `forge ship` walkthroughs** (2–4 min each) | S3+ | Demonstrate the dogfood claim |
| 6 | **Conference talks** (DevX/AI tooling tracks) | S4 | Establishes the category |
| 7 | **Podcast tour** (5–8 dev/AI shows) | S4 | Reaches the long tail of practitioners |
| 8 | **Office hours / livestreams** (weekly, recorded) | S3+ | Lowers contribution activation energy |
| 9 | **Newsletter cross-posts** (TLDR, Hacker Newsletter, Pointer) | S4 | Earned, not paid |
| 10 | **Sponsored dev events** (workshops at conferences) | S5+ | Last, not first — earn the right with substance |

**Anti-channels (explicitly avoid until S5):**
- Paid acquisition / influencer marketing.
- Twitter/X "build in public" stunts that drift from the NSM.
- Vanity benchmarks against named competitors.

---

## 4. Stage S0 — Stealth (T-12 → T-8 weeks before M0 GA)

**Audience:** Founder + 3 invited critics across ranges (one solo dev, one platform engineer, one infra/security skeptic).

**Activities:**
- Share `FORGE_FRAMEWORK_SPEC.md` + `ARCHITECTURE.md` privately.
- Run a 60-min review session per critic; capture every objection in `§8 Open Questions`.
- Close ≥80% of open questions before S1.
- Draft (not publish) the launch essay so it ages.

**Exit:** Spec at "no major objections" from all three critics.

---

## 5. Stage S1 — Friends & Family Alpha (M0 GA → +4 weeks)

**Audience:** 10–20 hand-picked developers across personas (solo, agency, platform team — see spec §11.1.1 P1–P6).

**Activation:**
- 1:1 onboarding call (30 min). Watch them run `forge new` and `forge ship --quick`.
- Issue a personal "alpha invite" via signed-binary link (no public download yet).
- Private Discord/Slack channel for raw feedback.

**Asks (per alpha user):**
- Run `forge ship` on one real change in your project.
- File ≥3 issues (any kind) in the first 2 weeks.
- 30-min closing interview.

**Metrics to watch:**
- Time-to-first-`ship` (target: ≤30 min from binary download).
- Issue volume and category mix (expect heavy bias to UX cliffs in M0).
- Drop-off after first failed run (target: ≤30%; mitigate with `forge doctor`).

**Exit:** ≥7 of 10 alpha users have run `forge ship` ≥3 times in their own repo.

---

## 6. Stage S2 — Private Beta (M1 GA → +8 weeks)

**Audience:** 100–300 invited devs from a waitlist (collected via founder's network + S1 referrals).

**Mechanics:**
- Public landing page goes up with **waitlist only**, not download.
- Weekly batch of invites (25/week) so support load is bounded.
- Invitees get: signed binary, private docs, 1 month of free hosted-eval credits (if applicable).

**Programmatic asks:**
- Opt into anonymous telemetry to bootstrap the NSM dashboard.
- Optional: opt into the learning loop (test the privacy story end-to-end).

**Operations:**
- Office hours: 2x/week (one EU-friendly, one US-friendly).
- Triage SLA practiced internally per §16.5.7 (first response ≤7d).
- Weekly "what shipped" digest emailed to beta cohort.

**Exit:**
- ≥40% of invited users active week-over-week for 2+ weeks.
- ≥5 community-authored plugins exist (even unpublished).
- Eval harness data shows token-cost regression caught on a real PR.

---

## 7. Stage S3 — Public Beta launch (M2 GA)

This is **the** moment. One week of coordinated activity.

### 7.1 Pre-launch (T-4 → T-0 weeks)

| When | Action |
|------|--------|
| T-4w | Launch essay finalized; private review by 5 voices (incl. one critic from S0) |
| T-3w | Demo videos recorded (90-sec hero + 3 deep-dives) |
| T-3w | README rewritten to lead with `forge ship`; install in 1 paste |
| T-2w | Plugin Registry seeded with ≥3 community plugins from S2 |
| T-2w | RFC #1 (workflow) and RFC #2 (plugin interface) merged and visible |
| T-1w | Pre-brief 5 friendly journalists/podcasters under embargo (no PR firm; founder-led) |
| T-1w | Run `forge eval` matrix; publish results in `BENCHMARKS.md` |
| T-3d | Status page + incident-response runbook live |
| T-1d | All maintainers on standby for Day-1 triage |

### 7.2 Launch day

| Time (founder-local) | Action |
|----------------------|--------|
| 06:00 | Repo flipped public; tag `v0.M2.0` |
| 07:00 | Launch essay published on founder's blog |
| 07:30 | Show HN post (founder, with disclosure) |
| 08:00 | Lobsters post by a community member (not founder) |
| 08:30 | Pre-briefed creators publish their pieces |
| 09:00–22:00 | Founder + 2 maintainers monitor HN/Lobsters/issues; reply to every top-level comment in ≤30 min |
| 22:00 | End-of-day retrospective + Day-2 plan |

### 7.3 Launch week

- Daily standup; rotate "first responder" duty.
- Post a daily "what we shipped today" thread (with PR links).
- Weekly office-hour recording becomes the "first 7 days of Forge" video series.

### 7.4 Launch success criteria (measured at +30d)

- Repo: ≥3,000 stars *and* ≥50 forks-with-PRs (forks alone don't count).
- Activity: NSM ≥500 weekly `forge ship` runs across ≥150 distinct projects.
- Community: ≥30 merged community PRs; ≥5 community plugins published; ≥3 RFC drafts opened by non-founder authors.
- Health: Median first-response time on issues ≤48h; zero unmitigated security findings.

---

## 8. Stage S4 — 1.0 Launch (M3 GA, ~3 months after S3)

**Theme:** "Forge 1.0 — production-ready, contributor-owned."

**Differentiators to emphasize at 1.0:**
1. NFR budgets are CI-enforced (Arch §14).
2. Contribution standards (§16.5) are CI-automated, not manual reviewer toil.
3. Closed threat model with public mitigations.
4. ≥X T2 adapters covering top 5 cloud + top 3 LLM providers.

**Activities:**
- Conference talk circuit (target: 3 talks in Q post-launch).
- Podcast tour (5–8 shows) with the founder + 1 community maintainer.
- Detailed "What changed since beta" post.
- Sponsorship program announcement (GitHub Sponsors / OpenCollective per §17).

**Success criteria (+90d):**
- NSM ≥2,000 weekly runs.
- ≥10 active maintainers across ≥2 working groups (foundation for Stage 3 governance).
- ≥1 paying enterprise pilot (validates commercial-later posture without diluting OSS).

---

## 9. Stage S5 — Foundation & beyond

Triggered by: ≥10K stars **or** ≥100 active maintainers (whichever first).

- Begin foundation conversation (CNCF / OpenJS / Linux Foundation per spec §16.1 Stage 3).
- Elect 7-member steering committee.
- Founder steps to "BDFL emeritus" — emergency override only.

---

## 10. Contributor activation (continuous, all stages)

The launch is not just users — it's contributors. Activation funnel:

```
visitor → reader (README/docs) → installer → first run → first PR (T3) →
recurring contributor → reviewer → maintainer → core maintainer → steward
```

**Friction-reducing tactics (each tied to a §16.5 promise):**
- "Ship as a plugin first" (§16.5.9) is the headline of CONTRIBUTING.md.
- `good-first-issue` label permanently kept ≥10 deep, with mentor assigned.
- Every accepted PR triggers a personal thank-you from a maintainer (no automation here — keep it human).
- Public "Contributor of the month" (working-group rotation; never tied to commit volume alone).
- Quarterly "RFC office hours" — founder-hosted, recorded.

**Activation metrics (weekly):**
- # of new contributors (first PR ever)
- # of recurring contributors (≥3 PRs in last 90d)
- Median time from PR open to first reviewer comment
- # of plugins promoted T3 → T2

---

## 11. Risks (launch-specific)

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|-------------|
| HN frontpage but binary breaks on first install | M | H | Pre-launch install matrix on 6 OS/arch combos; rollback plan with downloadable `v0.M-1` |
| Wave of low-quality drive-by PRs floods maintainers | H | M | §16.5 standards visible in CONTRIBUTING.md; auto-comment on PR with which gates failed |
| Security finding in launch week | M | H | Pre-launch external pentest + bug-bounty pre-positioned; status page ready |
| Competing project launches in same window | M | M | Don't react. Stay on the narrative (§2). |
| Founder burnout from Day-1 triage | H | H | 3-person rotation; office hours bounded; no "always-on" expectation |
| Hosted aggregator (opt-in) misperceived as required | M | H | "Local-first, hosted-optional" stated in hero; `--no-telemetry` proudly documented |
| Token costs scare off price-sensitive devs | M | M | Cheap-tier routing default; eval harness publishes per-feature cost; "BYO key" emphasized |

---

## 12. Anti-goals (what we explicitly will NOT do)

- Will not market against named competitors.
- Will not claim AI capabilities the eval harness doesn't substantiate.
- Will not accept paid sponsorship that buys roadmap influence.
- Will not run a closed-beta gate longer than necessary — gating creates resentment.
- Will not let star count drive decisions; the NSM does.

---

*Plan version: 0.1 — companion to spec v0.10.6.*
