-- 0048_task_watchers_rls rollback.
DROP POLICY IF EXISTS tenant_isolation ON task_watchers;
ALTER TABLE task_watchers DISABLE ROW LEVEL SECURITY;
