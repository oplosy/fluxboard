-- 0050_epics_rls -- Row-Level Security for epics. Same pattern as 0022:
-- predicate on the per-transaction GUC app.current_tenant set via SET LOCAL by
-- TenantPool. Unset GUC means zero rows (fail-closed). ENABLE (not FORCE).
-- tasks is already covered.

ALTER TABLE epics ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON epics
  USING (org_id = current_setting('app.current_tenant', true)::uuid)
  WITH CHECK (org_id = current_setting('app.current_tenant', true)::uuid);
