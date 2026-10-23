-- 0058_github_integration_rls rollback.
DROP POLICY IF EXISTS tenant_isolation ON github_integrations;
ALTER TABLE github_integrations DISABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON github_merge_projects;
ALTER TABLE github_merge_projects DISABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON task_pull_requests;
ALTER TABLE task_pull_requests DISABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON github_deliveries;
ALTER TABLE github_deliveries DISABLE ROW LEVEL SECURITY;
