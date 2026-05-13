# Forge RFC Process

> RFCs (Requests for Comments) are the mechanism for proposing and ratifying
> significant changes to Forge — new features, protocol changes, breaking
> changes, and governance decisions.

---

## When to write an RFC

Write an RFC when you want to make a change that is:

- **Novel** — introduces a new concept or subsystem
- **Breaking** — changes a public interface, wire format, or CLI contract
- **Controversial** — likely to generate significant debate
- **Cross-cutting** — affects multiple packages or teams

You do **not** need an RFC for:
- Bug fixes
- Documentation improvements
- Internal refactors that don't change public contracts
- New scanners or codemods that implement existing interfaces

---

## RFC lifecycle

```
Draft → Review → Final Comment Period (7 days) → Accepted | Rejected | Withdrawn
```

1. **Draft** — Author opens a PR adding `docs/rfcs/RFC-NNN-title.md`.
2. **Review** — Community and maintainers comment. Author revises.
3. **FCP** — A maintainer marks the RFC "Final Comment Period". 7 days for final objections.
4. **Decision** — Maintainers merge (Accepted) or close (Rejected/Withdrawn).
5. **Implementation** — An Accepted RFC gets an issue + implementation PR.

---

## Template

Copy `docs/rfcs/RFC-000-template.md` to start a new RFC.

---

## Index of RFCs

| RFC | Title | Status |
|-----|-------|--------|
| RFC-001 | Plugin sandbox capability model | Accepted |
| RFC-002 | Per-verb prompt template standard | Accepted |
| RFC-003 | Learning loop opt-in share protocol | Accepted |
