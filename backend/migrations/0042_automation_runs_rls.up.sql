-- 0042_automation_runs_rls -- Row-Level Security for automation_runs. Same
-- pattern as 0018: predicate on the per-transaction GUC app.current_tenant set
-- via SET LOCAL by TenantPool; unset GUC means zero rows (fail-closed).

ALTER TABLE automation_runs ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON automation_runs
  USING (org_id = current_setting('app.current_tenant', true)::uuid)
  WITH CHECK (org_id = current_setting('app.current_tenant', true)::uuid);
