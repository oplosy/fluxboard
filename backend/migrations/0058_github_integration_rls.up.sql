-- 0058_github_integration_rls -- Row-Level Security for the GitHub integration
-- tables. Same pattern as 0050: predicate on the per-transaction GUC
-- app.current_tenant set via SET LOCAL by TenantPool. Unset GUC means zero rows
-- (fail-closed). ENABLE (not FORCE).

ALTER TABLE github_integrations ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON github_integrations
  USING (org_id = current_setting('app.current_tenant', true)::uuid)
  WITH CHECK (org_id = current_setting('app.current_tenant', true)::uuid);

ALTER TABLE github_merge_projects ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON github_merge_projects
  USING (org_id = current_setting('app.current_tenant', true)::uuid)
  WITH CHECK (org_id = current_setting('app.current_tenant', true)::uuid);

ALTER TABLE task_pull_requests ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON task_pull_requests
  USING (org_id = current_setting('app.current_tenant', true)::uuid)
  WITH CHECK (org_id = current_setting('app.current_tenant', true)::uuid);

ALTER TABLE github_deliveries ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON github_deliveries
  USING (org_id = current_setting('app.current_tenant', true)::uuid)
  WITH CHECK (org_id = current_setting('app.current_tenant', true)::uuid);
