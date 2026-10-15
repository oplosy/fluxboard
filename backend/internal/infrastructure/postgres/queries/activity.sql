-- Task activity log ([T], tenant-scoped, FR-TASK-002) -----------------------

-- name: AppendActivity :exec
INSERT INTO task_activity (id, org_id, task_id, actor_id, field, old_value, new_value, automation_rule_id)
VALUES (@id, @org_id, @task_id, @actor_id, @field,
        sqlc.narg('old_value'), sqlc.narg('new_value'), sqlc.narg('automation_rule_id'));

-- name: ListActivityByTask :many
-- automation_rule_name answers "why did this change?" for rule-made entries.
SELECT a.id, a.org_id, a.task_id, a.actor_id, a.field, a.old_value, a.new_value, a.created_at,
       a.automation_rule_id, r.name AS automation_rule_name
FROM task_activity a
LEFT JOIN automation_rules r ON r.id = a.automation_rule_id
WHERE a.org_id = @org_id AND a.task_id = @task_id
ORDER BY a.created_at DESC;
