-- 0039_task_recurrence rollback. Generated occurrences stay as plain tasks.
DROP INDEX IF EXISTS tasks_occurrence_uniq;
ALTER TABLE tasks DROP COLUMN IF EXISTS occurrence_on;
ALTER TABLE tasks DROP COLUMN IF EXISTS recurrence_id;
DROP TABLE IF EXISTS task_recurrences;
