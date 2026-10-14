-- 0040_task_recurrence_rls -- Row-Level Security for task_recurrences. Same
-- pattern as 0035: predicate on the per-transaction GUC app.current_tenant set
-- via SET LOCAL by TenantPool; unset GUC means zero rows (fail-closed).

ALTER TABLE task_recurrences ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON task_recurrences
  USING (org_id = current_setting('app.current_tenant', true)::uuid)
  WITH CHECK (org_id = current_setting('app.current_tenant', true)::uuid);
