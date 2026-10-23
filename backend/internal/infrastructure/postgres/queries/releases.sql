-- Releases ([T], tenant-scoped, FR-PROJ-010) --------------------------------

-- name: CreateRelease :exec
INSERT INTO releases (id, org_id, project_id, name, description, release_date, created_by)
VALUES (@id, @org_id, @project_id, @name, @description, sqlc.narg('release_date'), @created_by);

-- name: GetRelease :one
SELECT id, org_id, project_id, name, description, release_date, released_at, created_by, created_at
FROM releases
WHERE org_id = @org_id AND id = @id;

-- name: ListReleasesByProject :many
SELECT id, org_id, project_id, name, description, release_date, released_at, created_by, created_at
FROM releases
WHERE org_id = @org_id AND project_id = @project_id
ORDER BY created_at, id;

-- name: UpdateRelease :execrows
UPDATE releases
SET name = @name, description = @description,
    release_date = sqlc.narg('release_date'), released_at = sqlc.narg('released_at')
WHERE org_id = @org_id AND id = @id;

-- name: DeleteRelease :execrows
DELETE FROM releases
WHERE org_id = @org_id AND id = @id;

-- name: SetTaskRelease :execrows
UPDATE tasks SET release_id = sqlc.narg('release_id')
WHERE org_id = @org_id AND id = @id AND deleted_at IS NULL;
