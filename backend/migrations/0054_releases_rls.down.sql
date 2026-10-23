-- 0054_releases_rls rollback.
DROP POLICY IF EXISTS tenant_isolation ON releases;
ALTER TABLE releases DISABLE ROW LEVEL SECURITY;
