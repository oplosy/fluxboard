-- Sprints ([T], tenant-scoped, FR-SPRINT) ----------------------------------

-- name: CreateSprint :exec
INSERT INTO sprints (id, org_id, project_id, name, goal, status, started_at, ended_at, created_by)
VALUES (@id, @org_id, @project_id, @name, @goal, @status, @started_at, @ended_at, @created_by);

-- name: GetSprint :one
SELECT id, org_id, project_id, name, goal, status, started_at, ended_at,
       completed_total, completed_done, created_by, created_at,
       completed_points_total, completed_points_done
FROM sprints
WHERE org_id = @org_id AND id = @id;

-- name: ListSprintsByProject :many
SELECT id, org_id, project_id, name, goal, status, started_at, ended_at,
       completed_total, completed_done, created_by, created_at,
       completed_points_total, completed_points_done
FROM sprints
WHERE org_id = @org_id AND project_id = @project_id
ORDER BY created_at;

-- name: GetActiveSprint :one
SELECT id, org_id, project_id, name, goal, status, started_at, ended_at,
       completed_total, completed_done, created_by, created_at,
       completed_points_total, completed_points_done
FROM sprints
WHERE org_id = @org_id AND project_id = @project_id AND status = 'active'
LIMIT 1;

-- name: UpdateSprint :execrows
UPDATE sprints
SET name = @name, goal = @goal, status = @status,
    started_at = @started_at, ended_at = @ended_at,
    completed_total = @completed_total, completed_done = @completed_done,
    completed_points_total = @completed_points_total, completed_points_done = @completed_points_done
WHERE org_id = @org_id AND id = @id;

-- name: DeleteSprint :execrows
DELETE FROM sprints
WHERE org_id = @org_id AND id = @id;

-- name: SetTaskSprint :execrows
UPDATE tasks SET sprint_id = sqlc.narg('sprint_id')
WHERE org_id = @org_id AND id = @id AND deleted_at IS NULL;

-- name: ClearProjectSprints :execrows
UPDATE tasks SET sprint_id = NULL
WHERE org_id = @org_id AND project_id = @project_id AND sprint_id = @sprint_id
  AND deleted_at IS NULL;

-- name: ListSprintBurndownTasks :many
-- The sprint's live tasks with their points and completion stamp (FR-AN-003).
-- completed_at is set only while a task sits in the done column (0038).
SELECT estimate, completed_at FROM tasks
WHERE org_id = @org_id AND sprint_id = @sprint_id AND deleted_at IS NULL;
