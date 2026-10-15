-- 0044_intake_requests_rls -- Row-Level Security for intake_requests. Same
-- pattern as 0040: predicate on the per-transaction GUC app.current_tenant set
-- via SET LOCAL by TenantPool; unset GUC means zero rows (fail-closed).

ALTER TABLE intake_requests ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON intake_requests
  USING (org_id = current_setting('app.current_tenant', true)::uuid)
  WITH CHECK (org_id = current_setting('app.current_tenant', true)::uuid);
