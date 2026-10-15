-- 0046_task_approvals_rls -- Row-Level Security for task_approvals and
-- task_approval_approvers. Same pattern as 0040: predicate on the
-- per-transaction GUC app.current_tenant; unset GUC means zero rows.

ALTER TABLE task_approvals ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON task_approvals
  USING (org_id = current_setting('app.current_tenant', true)::uuid)
  WITH CHECK (org_id = current_setting('app.current_tenant', true)::uuid);

ALTER TABLE task_approval_approvers ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON task_approval_approvers
  USING (org_id = current_setting('app.current_tenant', true)::uuid)
  WITH CHECK (org_id = current_setting('app.current_tenant', true)::uuid);
