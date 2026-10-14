-- 0038_task_completed_at rollback.
DROP TRIGGER IF EXISTS tasks_stamp_completed ON tasks;
DROP FUNCTION IF EXISTS tasks_stamp_completed();
DROP FUNCTION IF EXISTS board_done_column(uuid);
DROP INDEX IF EXISTS tasks_completed_idx;
ALTER TABLE tasks DROP COLUMN IF EXISTS completed_at;
