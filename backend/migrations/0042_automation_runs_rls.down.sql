-- 0042_automation_runs_rls rollback.
DROP POLICY IF EXISTS tenant_isolation ON automation_runs;
ALTER TABLE automation_runs DISABLE ROW LEVEL SECURITY;
