-- 0035_agents_rls rollback.
DROP POLICY IF EXISTS tenant_isolation ON agents;
ALTER TABLE agents DISABLE ROW LEVEL SECURITY;
