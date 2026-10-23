-- 0054_releases_rls -- Row-Level Security for releases. Same pattern as 0050:
-- predicate on the per-transaction GUC app.current_tenant set via SET LOCAL by
-- TenantPool. Unset GUC means zero rows (fail-closed). ENABLE (not FORCE).
-- tasks is already covered.

ALTER TABLE releases ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON releases
  USING (org_id = current_setting('app.current_tenant', true)::uuid)
  WITH CHECK (org_id = current_setting('app.current_tenant', true)::uuid);
