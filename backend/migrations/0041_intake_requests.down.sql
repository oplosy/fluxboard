-- 0041_intake_requests rollback. Tasks created by accepted requests stay.
DROP TABLE IF EXISTS intake_requests;
ALTER TABLE project_forms DROP COLUMN IF EXISTS schema_version;
