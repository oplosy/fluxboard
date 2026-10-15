-- 0046_task_approvals_rls rollback.
DROP POLICY IF EXISTS tenant_isolation ON task_approval_approvers;
ALTER TABLE task_approval_approvers DISABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON task_approvals;
ALTER TABLE task_approvals DISABLE ROW LEVEL SECURITY;
