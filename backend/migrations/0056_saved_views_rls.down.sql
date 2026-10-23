-- 0056_saved_views_rls rollback.
DROP POLICY IF EXISTS tenant_isolation ON saved_views;
ALTER TABLE saved_views DISABLE ROW LEVEL SECURITY;
