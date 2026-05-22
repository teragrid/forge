# Section 06 — Integration Design (Light)

> **Template**: DAB Light

---

## Integration Inventory

| System | Direction | Protocol | Auth |
|--------|-----------|----------|------|
| TODO | Inbound | HTTPS REST | Bearer JWT |

---

## OpenAPI Contract

> Authoritative contract: `.forge/specs/<slug>/openapi.yaml`
>
> Run `forge ship arch` to (re)generate the stub. The arch checkpoint uses **KB injection**
> (ADR-026): top-5 matching knowledge-base entries are appended to the LLM system prompt
> (tags: `openapi`, `architecture`, `api-contract`, `supabase`).
> Edit `openapi.yaml` to match the actual API before running `forge ship test` and `forge ship breakdown`.

### API style

> Choose one — `forge ship` auto-detects from path prefixes in `openapi.yaml`:

- [ ] **Standard REST** — paths follow `/api/v1/{resource}`
- [ ] **Supabase RPC** — paths use `/rest/v1/rpc/{function_name}`; add `anon` / `service_role` security schemes

**Declared style**: _TODO_

### Versioning

- Strategy: URI versioning (`/api/v1/`, `/api/v2/`) or Supabase RPC function versioning (`rpc/{fn}_v2`)
- Breaking changes: 90-day deprecation window required; bump API version

### Contract governance

- [ ] **API style declared** in `openapi.yaml` paths (REST or Supabase RPC — see Section 02 row 5)
- [ ] All paths and schemas are defined in `openapi.yaml`
- [ ] Security schemes and scopes declared (Supabase: `anon` + `service_role` keys)
- [ ] No breaking changes without version bump
- [ ] If Supabase RPC: PostgreSQL function, `GRANT EXECUTE`, and RLS policy tasks present in Section 04

---

## Events (if applicable)

| Event | Type | Direction |
|-------|------|-----------|
| TODO | TODO | Published |

---

*Next: [07-infrastructure-design.md](07-infrastructure-design.md)*
