-- 0047_task_watchers rollback.
DROP INDEX IF EXISTS task_watchers_user_idx;
DROP TABLE IF EXISTS task_watchers;
