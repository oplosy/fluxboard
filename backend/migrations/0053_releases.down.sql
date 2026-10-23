-- 0053_releases rollback.
DROP INDEX IF EXISTS tasks_release_idx;
ALTER TABLE tasks DROP COLUMN IF EXISTS release_id;
DROP TABLE IF EXISTS releases;
