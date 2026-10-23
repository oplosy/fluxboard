-- 0060_dashboard_layouts_rls rollback.
DROP POLICY IF EXISTS tenant_isolation ON dashboard_layouts;
ALTER TABLE dashboard_layouts DISABLE ROW LEVEL SECURITY;
