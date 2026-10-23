-- 0060_dashboard_layouts_rls -- Row-Level Security for dashboard_layouts. Same
-- pattern as 0056: predicate on the per-transaction GUC app.current_tenant set
-- via SET LOCAL by TenantPool. Unset GUC means zero rows (fail-closed). ENABLE
-- (not FORCE). Whose layout a caller may touch is decided in the application.

ALTER TABLE dashboard_layouts ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON dashboard_layouts
  USING (org_id = current_setting('app.current_tenant', true)::uuid)
  WITH CHECK (org_id = current_setting('app.current_tenant', true)::uuid);
