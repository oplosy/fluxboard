-- 0040_task_recurrence_rls rollback.
DROP POLICY IF EXISTS tenant_isolation ON task_recurrences;
ALTER TABLE task_recurrences DISABLE ROW LEVEL SECURITY;
