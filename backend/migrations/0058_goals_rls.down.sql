-- 0058_goals_rls rollback.
DROP POLICY IF EXISTS tenant_isolation ON goal_links;
ALTER TABLE goal_links DISABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON goals;
ALTER TABLE goals DISABLE ROW LEVEL SECURITY;
