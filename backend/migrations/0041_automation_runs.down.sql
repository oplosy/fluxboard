-- 0041_automation_runs rollback.
ALTER TABLE task_activity DROP COLUMN IF EXISTS automation_rule_id;
DROP TABLE IF EXISTS automation_runs;
