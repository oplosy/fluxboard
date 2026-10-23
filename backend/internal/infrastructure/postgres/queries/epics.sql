-- Epics ([T], tenant-scoped, FR-PROJ-008) ----------------------------------

-- name: CreateEpic :exec
INSERT INTO epics (id, org_id, project_id, name, color, created_by)
VALUES (@id, @org_id, @project_id, @name, @color, @created_by);

-- name: GetEpic :one
SELECT id, org_id, project_id, name, color, created_by, created_at
FROM epics
WHERE org_id = @org_id AND id = @id;

-- name: ListEpicsByProject :many
SELECT id, org_id, project_id, name, color, created_by, created_at
FROM epics
WHERE org_id = @org_id AND project_id = @project_id
ORDER BY created_at, id;

-- name: UpdateEpic :execrows
UPDATE epics SET name = @name, color = @color
WHERE org_id = @org_id AND id = @id;

-- name: DeleteEpic :execrows
DELETE FROM epics
WHERE org_id = @org_id AND id = @id;

-- name: SetTaskEpic :execrows
UPDATE tasks SET epic_id = sqlc.narg('epic_id')
WHERE org_id = @org_id AND id = @id AND deleted_at IS NULL;

-- name: EpicProgress :many
-- Tasks and points per epic, and of those in the done column, counted in SQL
-- so listing epics does not load every task of the project.
SELECT epic_id::uuid AS id,
       COUNT(*)::int AS total,
       COUNT(*) FILTER (WHERE column_id = sqlc.narg('done_column'))::int AS done,
       COALESCE(SUM(estimate), 0)::int AS points,
       COALESCE(SUM(estimate) FILTER (WHERE column_id = sqlc.narg('done_column')), 0)::int AS done_points
FROM tasks
WHERE org_id = @org_id AND project_id = @project_id AND deleted_at IS NULL AND epic_id IS NOT NULL
GROUP BY epic_id;
