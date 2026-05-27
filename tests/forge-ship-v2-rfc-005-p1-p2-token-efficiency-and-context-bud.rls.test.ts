Below are the failing Row-Level Security (RLS) test stubs for verifying the specified behavior. These tests are written for use in Supabase/PostgreSQL environments. The tests verify that:

1. **Tenant-A** cannot read **Tenant-B**'s rows.
2. The **Service Role** has unrestricted access and can read all rows.
3. The **Anon Role** is denied access.

These tests are designed to fail so that developers can fix the implementation to meet the requirements.

---

### Table Setup for Tests (`forge_ship_test`)

```sql
-- Schema setup: Simulates a Forge Ship records table
CREATE TABLE forge_ship_v2 (
  id SERIAL PRIMARY KEY,
  tenant_id UUID NOT NULL,   -- Tenant-specific identifier
  data JSONB NOT NULL,       -- Context or feature-related data
  created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Example data for the test:
INSERT INTO forge_ship_v2 (tenant_id, data)
VALUES
  ('tenant-A-uuid', '{"context": "tenant-A data"}'), -- Tenant A's data
  ('tenant-B-uuid', '{"context": "tenant-B data"}'), -- Tenant B's data
  ('tenant-A-uuid', '{"context": "tenant-A other data"}'); -- Another Tenant A row
```

---

### RLS Policy Setup

```sql
-- Enable Row-Level Security
ALTER TABLE forge_ship_v2 ENABLE ROW LEVEL SECURITY;

-- RLS policy stub for tenants (failing test scenarios)
-- Policy 1: Allow tenant to access their own rows
CREATE POLICY tenant_can_access_own_rows
ON forge_ship_v2
FOR SELECT
USING (tenant_id = current_setting('app.current_tenant')::UUID);

-- Policy 2: Service role can access all rows (placeholder, failing initially)
CREATE POLICY service_role_can_access_all_rows
ON forge_ship_v2
FOR SELECT
TO role_that_does_not_exist -- This will fail as the role does not exist
USING (true);

-- Policy 3: Anon users cannot access any rows
CREATE POLICY anon_role_cannot_access
ON forge_ship_v2
FOR SELECT
TO anon
USING (false);
```

---

### Failing Tests

#### 1. Tenant-A Cannot Read Tenant-B Rows

**Test Case:**
Verify that a user assigned to **Tenant-A** cannot read rows belonging to **Tenant-B**.

```sql
BEGIN;

-- Set the current tenant to Tenant-A
SELECT set_config('app.current_tenant', 'tenant-A-uuid', false);

-- Expect no rows related to Tenant-B (should fail initially, as no RLS is effective)
SELECT * FROM forge_ship_v2 WHERE tenant_id = 'tenant-B-uuid';

-- This should return no rows but will currently fail as policies are not working yet.

ROLLBACK;
```

---

#### 2. Service Role Can Read All Rows

**Test Case:**
Verify that a **Service Role** can read all rows, irrespective of the tenant.

```sql
BEGIN;

-- Set the role to 'service_role'
SET ROLE service_role;

-- Expect all rows to be accessible (should fail initially due to RLS misconfiguration)
SELECT * FROM forge_ship_v2;

-- This should return all rows, but it will fail as the policy for `service_role_can_access_all_rows` is not defined correctly.

RESET ROLE;

ROLLBACK;
```

---

#### 3. Anon Role Is Denied Access

**Test Case:**
Verify that an **Anon Role** is denied access to any row in the `forge_ship_v2` table.

```sql
BEGIN;

-- Set the role to 'anon'
SET ROLE anon;

-- Expect no rows to be accessible (should fail initially, as policies are not properly denying access)
SELECT * FROM forge_ship_v2;

-- This should return no rows, but it will fail if the policy for anon access isn't properly enforced.

RESET ROLE;

ROLLBACK;
```

---

### Summary

These test stubs will fail at first due to incorrect or incomplete RLS policy implementation for `forge_ship_v2`. The developer will need to:

1. Fix the policy for tenant isolation (ensure tenants cannot access rows not belonging to them).
2. Correctly implement the `service_role_can_access_all_rows` policy to allow unrestricted access for the Service Role.
3. Enforce the `anon_role_cannot_access` policy to completely deny the Anon Role from reading any rows.

Running these tests should initially fail, ensuring the development team addresses all required RLS policies.