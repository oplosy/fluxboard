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
