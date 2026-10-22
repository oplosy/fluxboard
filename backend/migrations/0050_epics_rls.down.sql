-- 0050_epics_rls rollback.
DROP POLICY IF EXISTS tenant_isolation ON epics;
ALTER TABLE epics DISABLE ROW LEVEL SECURITY;
