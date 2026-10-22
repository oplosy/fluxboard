-- 0049_epics rollback.
DROP INDEX IF EXISTS tasks_epic_idx;
ALTER TABLE tasks DROP COLUMN IF EXISTS epic_id;
DROP TABLE IF EXISTS epics;
