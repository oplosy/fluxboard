-- 0036_task_claims rollback.
DROP TRIGGER IF EXISTS tasks_bump_version ON tasks;
DROP FUNCTION IF EXISTS tasks_bump_version();
DROP INDEX IF EXISTS tasks_lease_idx;
ALTER TABLE tasks DROP CONSTRAINT IF EXISTS tasks_claim_has_lease;
ALTER TABLE tasks DROP COLUMN IF EXISTS lease_expires_at;
ALTER TABLE tasks DROP COLUMN IF EXISTS claimed_by;
ALTER TABLE tasks DROP COLUMN IF EXISTS version;
