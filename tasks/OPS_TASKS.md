# Forge — Continuous-Ops Tasks

> Companion to all plan documents.
> Tracker for the Section-E tasks (OPS-NN) from the master breakdown — **no end-state**, run on the stated cadence forever.

ID and conventions follow `ARCHITECTURE_TASKS.md`.

---

| ID | Task | Cadence | Anchor |
|----|------|---------|--------|
| OPS-01 | Weekly maintainer triage call | Weekly | Spec §16.5.7 |
| OPS-02 | Weekly false-positive review (scan + lint + hygiene) | Weekly | DEV plan §4 |
| OPS-03 | Weekly NSM dashboard review | Weekly | Launch §0 |
| OPS-04 | Monthly RFC review meeting | Monthly | Spec §16.2 |
| OPS-05 | Quarterly contribution-standards retro | Quarterly | Spec §16.5 |
| OPS-06 | Quarterly threat-model review | Quarterly | Arch §15 |
| OPS-07 | Quarterly NFR-budget review (raise the bar where possible) | Quarterly | Arch §14 |
| OPS-08 | Annual roadmap RFC | Yearly | Spec §6 |
| OPS-09 | Per-release sigstore rotation check | Per release | Arch §15 |
| OPS-10 | Per-release reproducible-build verification | Per release | Spec §16.5.6 |
| OPS-11 | Weekly hygiene-corpus refresh (add new LLM-scratch patterns) | Weekly | Spec §4 hygiene |
| OPS-12 | Weekly secrets-corpus refresh (add new provider key shapes) | Weekly | Spec §4 Repo Hygiene Layer (`.gitleaks.toml` standards) |
| OPS-13 | Weekly allowlist-expiry sweep (auto-PR closes expired `.gitleaks.toml` entries) | Weekly | Spec §16.5.4 #12 |
| OPS-14 | Quarterly `.gitignore` template audit (sync per-stack fragments with upstream best practice) | Quarterly | Spec §4 Repo Hygiene Layer (`.gitignore` standards) |
| OPS-15 | Per-release upstream gitleaks rule-pack pull (re-base Forge rules on latest upstream) | Per release | Spec §4 Repo Hygiene Layer (`.gitleaks.toml` standards) |
| OPS-16 | Weekly on-call triage rota (assign next-week triager from Reviewers + Maintainers; publish in `docs/oncall/`) | Weekly | Arch §18.2 / Spec §16.5.8 |
| OPS-17 | Monthly chaos-drill — exercise one §17.3 cross-cutting failure scenario end-to-end; write a drill report; auto-quarantine reveals a regression test if the drill fails | Monthly | Arch §17.3 |
| OPS-18 | Per-S0/S1 post-mortem publication SLA (≤ 7 days after incident closure; PR adds a §17.2 register entry + at least one durable action item) | Per incident | Arch §18.6 |
| OPS-19 | Monthly bug-lifecycle dashboard review (open S0/S1, time-to-first-response, time-to-fix, reopen rate, % S0/S1 with published post-mortem) | Monthly | Arch §18.8 |
| OPS-20 | Quarterly resilience-register review (every §17.2 row revisited; add new components, retire obsolete ones, refresh test anchors) | Quarterly | Arch §17.2 |
| OPS-21 | Per-release status-page check (verify every status-page incident from the cycle has a closed issue + post-mortem if S0/S1) | Per release | Arch §18.5 #6 |

---

*Task file version: 0.3 — companion to spec v0.10.9.*
