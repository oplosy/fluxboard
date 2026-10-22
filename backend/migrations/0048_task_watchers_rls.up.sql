-- 0048_task_watchers_rls -- Row-Level Security for task_watchers. Same pattern
-- as 0026: predicate on the per-transaction GUC app.current_tenant; unset GUC
-- means zero rows (fail-closed). ENABLE (not FORCE).

ALTER TABLE task_watchers ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON task_watchers
  USING (org_id = current_setting('app.current_tenant', true)::uuid)
  WITH CHECK (org_id = current_setting('app.current_tenant', true)::uuid);
