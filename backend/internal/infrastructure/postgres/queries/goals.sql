-- Goals ([T], tenant-scoped, FR-PROJ-014) -----------------------------------

-- name: CreateGoal :exec
INSERT INTO goals (id, org_id, title, description, owner_id, target_date)
VALUES (@id, @org_id, @title, @description, @owner_id, sqlc.narg('target_date'));

-- name: GetGoal :one
SELECT id, org_id, title, description, owner_id, target_date, created_at
FROM goals
WHERE org_id = @org_id AND id = @id;

-- name: ListGoals :many
SELECT id, org_id, title, description, owner_id, target_date, created_at
FROM goals
WHERE org_id = @org_id
ORDER BY created_at, id;

-- name: UpdateGoal :execrows
UPDATE goals
SET title = @title, description = @description, target_date = sqlc.narg('target_date')
WHERE org_id = @org_id AND id = @id;

-- name: DeleteGoal :execrows
DELETE FROM goals
WHERE org_id = @org_id AND id = @id;

-- name: AddGoalLink :exec
INSERT INTO goal_links (id, org_id, goal_id, epic_id, task_id)
VALUES (@id, @org_id, @goal_id, sqlc.narg('epic_id'), sqlc.narg('task_id'))
ON CONFLICT DO NOTHING;

-- name: DeleteGoalLink :execrows
DELETE FROM goal_links
WHERE org_id = @org_id AND goal_id = @goal_id
  AND epic_id IS NOT DISTINCT FROM sqlc.narg('epic_id')
  AND task_id IS NOT DISTINCT FROM sqlc.narg('task_id');

-- name: ListGoalLinks :many
SELECT l.goal_id, l.epic_id, l.task_id, COALESCE(e.project_id, t.project_id)::uuid AS project_id
FROM goal_links l
LEFT JOIN epics e ON e.id = l.epic_id
LEFT JOIN tasks t ON t.id = l.task_id
WHERE l.org_id = @org_id AND (sqlc.narg('goal_id')::uuid IS NULL OR l.goal_id = sqlc.narg('goal_id')::uuid)
ORDER BY l.created_at, l.id;
