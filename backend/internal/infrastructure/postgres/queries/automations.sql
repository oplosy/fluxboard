-- Automation rules ([T], tenant-scoped, FR-AUTO) --------------------------

-- name: CreateAutomationRule :exec
INSERT INTO automation_rules (id, org_id, name, enabled, trigger, trigger_config, action, action_config, created_by)
VALUES (@id, @org_id, @name, @enabled, @trigger, @trigger_config, @action, @action_config, @created_by);

-- name: GetAutomationRule :one
SELECT id, org_id, name, enabled, trigger, trigger_config, action, action_config, created_by, created_at
FROM automation_rules
WHERE org_id = @org_id AND id = @id;

-- name: ListAutomationRules :many
SELECT id, org_id, name, enabled, trigger, trigger_config, action, action_config, created_by, created_at
FROM automation_rules
WHERE org_id = @org_id
ORDER BY created_at;

-- name: ListEnabledAutomationRules :many
SELECT id, org_id, name, enabled, trigger, trigger_config, action, action_config, created_by, created_at
FROM automation_rules
WHERE org_id = @org_id AND enabled AND trigger = @trigger
ORDER BY created_at;

-- name: UpdateAutomationRule :execrows
UPDATE automation_rules
SET name = @name, enabled = @enabled, trigger = @trigger, trigger_config = @trigger_config,
    action = @action, action_config = @action_config
WHERE org_id = @org_id AND id = @id;

-- name: DeleteAutomationRule :execrows
DELETE FROM automation_rules
WHERE org_id = @org_id AND id = @id;

-- Run history ([T], FR-AUTO) --------------------------------------------------

-- name: InsertAutomationRun :execrows
-- ON CONFLICT DO NOTHING hits the once-per-task-per-rule index for time
-- triggers: 0 rows means another run already claimed this (rule, task).
INSERT INTO automation_runs (id, org_id, rule_id, trigger, task_id, actor_id, status, error, depth)
VALUES (@id, @org_id, @rule_id, @trigger, @task_id, sqlc.narg('actor_id'), @status, sqlc.narg('error'), @depth)
ON CONFLICT DO NOTHING;

-- name: FinishAutomationRun :exec
UPDATE automation_runs SET status = @status, error = sqlc.narg('error')
WHERE org_id = @org_id AND id = @id;

-- name: ListAutomationRuns :many
SELECT r.id, r.org_id, r.rule_id, r.trigger, r.task_id, r.actor_id, r.status, r.error, r.depth, r.created_at,
       t.title AS task_title, t.number AS task_number, p.key AS project_key
FROM automation_runs r
JOIN tasks t ON t.id = r.task_id
JOIN projects p ON p.id = t.project_id
WHERE r.org_id = @org_id AND r.rule_id = @rule_id
ORDER BY r.created_at DESC
LIMIT @max_rows;

-- name: CountAutomationActionsSince :one
-- The per-org action budget window: runs that executed (or are executing).
SELECT count(*) FROM automation_runs
WHERE org_id = @org_id AND created_at >= @since AND status IN ('pending', 'success', 'failed');

-- name: ListTasksDueBetween :many
-- Open, live tasks due in [from_at, to_at) that the rule has not yet fired on.
SELECT t.id FROM tasks t
WHERE t.org_id = @org_id AND t.deleted_at IS NULL AND t.completed_at IS NULL
  AND t.due_date IS NOT NULL AND t.due_date >= @from_at AND t.due_date < @to_at
  AND NOT EXISTS (SELECT 1 FROM automation_runs r WHERE r.rule_id = @rule_id AND r.task_id = t.id)
ORDER BY t.due_date
LIMIT 500;
