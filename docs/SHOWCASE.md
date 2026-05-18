# Built with Forge

> Real products built by vibe-coders and founders using the Forge AI-native framework.
> Every project here went from AI-generated code to a production-grade, shippable product using `forge new`, `forge ship`, and the Forge quality pipeline.

---

## Why this page exists

The best way to understand what Forge makes possible is to see it in the wild. These projects prove the premise: you don't need a senior dev team to ship enterprise-grade software. You need Forge.

If you've built something with Forge, [submit it](#submit-your-project) — we'd love to feature it here.

---

## Featured projects

### [PromotAI](https://promotiai.com)

> **The AI marketing platform for founders and growth teams.**

| | |
|---|---|
| **What it does** | PromotAI is an AI-native marketing platform that generates, schedules, and optimises content campaigns across channels — from social posts to email sequences to landing pages — using AI that learns from your brand voice. |
| **Built by** | [Teragrid](https://teragrid.io) |
| **Template used** | `forge new ts-service` → customised with `forge new regulated/soc2` guardrails |
| **Forge features used** | `forge ship` quality gate · secret scanning · AI spend controls · tamper-proof audit trail · prompt-injection hardening |
| **From vibe-code to prod** | First working version scaffolded and shipped in under a week with zero dedicated DevOps |

**What PromotAI demonstrates about Forge:**
- A non-trivial SaaS product (multi-tenant, AI-heavy, enterprise security requirements) built entirely on AI-generated code managed by Forge
- `forge scan prompt-injection` caught and patched 3 prompt-injection surfaces before launch that would have let users manipulate the AI to generate off-brand or harmful content
- `forge audit show` provides the change log PromotAI's enterprise customers ask for during onboarding due diligence
- AI spend caps via `forge spend set` kept development costs predictable during a rapid iteration phase

> *"We went from 'the AI wrote it but I don't know if it's safe to show customers' to 'here's our security posture and audit trail' in one sprint."*

---

## More projects

*This section grows as the community submits projects. Be the next one here.*

| Project | Category | What it does | Forge features |
|---|---|---|---|
| [PromotAI](https://promotiai.com) | AI Marketing | AI-powered campaign generation and scheduling | Full pipeline: scan, ship, audit, spend |
| *Your project here* | | | |

---

## Submit your project

Built something with Forge? We want to feature it.

**Requirements to be listed:**
- The project is live (has a URL) or is open source (has a repo link)
- It was built using at least `forge new` or `forge init` plus `forge ship`
- You're happy to be named as the builder

**How to submit:**

Open a [GitHub Discussion](https://github.com/teragrid/forge/discussions) with the title `[Showcase] Your Project Name` and include:

```
Project name:
URL or repo link:
What it does (1-2 sentences):
Builder / company:
Template used (forge new <template>):
Which Forge features you relied on:
One sentence on what Forge made possible that you couldn't have done otherwise:
```

Alternatively, open a PR that adds your project to the table in this file.

---

## What "built with Forge" means

A project qualifies as "built with Forge" if Forge was part of the core development loop — not just installed and forgotten. That typically means:

- The project was scaffolded with `forge new` **or** adopted with `forge init`
- `forge ship` runs as part of the deploy process (locally or in CI)
- At least one Forge scan, audit, or spend feature is actively used

---

*See also: [COMMUNITY_PLUGINS.md](COMMUNITY_PLUGINS.md) for Forge extensions built by the community.*
