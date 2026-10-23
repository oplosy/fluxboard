-- 0051_task_estimate rollback.
ALTER TABLE tasks DROP COLUMN IF EXISTS estimate;
