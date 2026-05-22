# Section 04 — Data Design

> **Template**: DAB Full
> Define all data entities, storage decisions, migration strategy, and consistency model.

---

## 4.1 Data Entity Catalogue

| Entity | Description | Owner | Classification |
|--------|-------------|-------|----------------|
| TODO | TODO | TODO | Public / PII / Sensitive |

---

## 4.2 Entity Schemas

### `TODOEntity`

```
TODOEntity {
  id         UUID          PK, immutable
  created_at TIMESTAMPTZ   NOT NULL, DEFAULT now()
  updated_at TIMESTAMPTZ   NOT NULL
  deleted_at TIMESTAMPTZ   NULL  -- soft-delete
  -- TODO: add domain fields
}
```

> Indices:
> - `idx_todo_created_at` — time-based range queries

---

## 4.3 Relationships (ER Diagram)

```
TODOEntity 1──< TODORelated
```

> TODO: Replace with actual ER diagram or Mermaid `erDiagram`.

---

## 4.4 Storage Technology Decisions

| Store | Technology | Justification | Owner |
|-------|-----------|---------------|-------|
| Primary DB | TODO (PostgreSQL / MySQL / …) | TODO | Platform |
| Cache | TODO (Redis / Memcached / …) | TODO | Platform |
| Object store | TODO (S3 / GCS / …) | TODO | Platform |

---

## 4.5 Consistency Model

| Aggregate | Model | Rationale |
|-----------|-------|-----------|
| TODOEntity | Strong / Eventual | TODO |

---

## 4.6 Migration Strategy

> All migrations must satisfy:
> 1. **Backwards-compatible** — existing consumers must not break before the migration is fully rolled out.
> 2. **Rollback-safe** — the rollback script must be tested in staging before production.
> 3. **Zero-downtime** — use `ADD COLUMN` / `CREATE INDEX CONCURRENTLY`; avoid table locks.

### Migration steps

1. TODO — e.g., add column `new_field TEXT NULL`
2. TODO — deploy code that writes to both old and new columns
3. TODO — backfill old data
4. TODO — drop old column / make new column NOT NULL

### Rollback script

```sql
-- TODO: paste rollback SQL here
```

---

## 4.7 Data Lifecycle & Retention

| Entity | Retention | Deletion | Notes |
|--------|-----------|----------|-------|
| TODOEntity | TODO days | Soft-delete → hard-delete after N days | GDPR erasure support |

---

## 4.8 Data Access Patterns

| Pattern | Frequency | Strategy |
|---------|-----------|----------|
| Read by ID | High | PK lookup, cached |
| List / paginate | Medium | Cursor-based pagination |
| Write | Medium | Write-through cache |
| Aggregate / report | Low | Read replica |

---

*Next section: [05-detailed-design.md](05-detailed-design.md)*
