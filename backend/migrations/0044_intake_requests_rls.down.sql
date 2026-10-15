-- 0044_intake_requests_rls rollback.
DROP POLICY IF EXISTS tenant_isolation ON intake_requests;
ALTER TABLE intake_requests DISABLE ROW LEVEL SECURITY;
