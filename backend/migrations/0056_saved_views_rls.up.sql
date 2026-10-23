-- 0056_saved_views_rls -- Row-Level Security for saved_views. Same pattern as
-- 0054: predicate on the per-transaction GUC app.current_tenant set via SET
-- LOCAL by TenantPool. Unset GUC means zero rows (fail-closed). ENABLE (not
-- FORCE). Who sees a personal view is decided in the application layer.

ALTER TABLE saved_views ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON saved_views
  USING (org_id = current_setting('app.current_tenant', true)::uuid)
  WITH CHECK (org_id = current_setting('app.current_tenant', true)::uuid);
