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

-- name: ReleaseProgress :many
-- Tasks and points per release, and of those in the done column (see EpicProgress).
SELECT release_id::uuid AS id,
       COUNT(*)::int AS total,
       COUNT(*) FILTER (WHERE column_id = sqlc.narg('done_column'))::int AS done,
       COALESCE(SUM(estimate), 0)::int AS points,
       COALESCE(SUM(estimate) FILTER (WHERE column_id = sqlc.narg('done_column')), 0)::int AS done_points
FROM tasks
WHERE org_id = @org_id AND project_id = @project_id AND deleted_at IS NULL AND release_id IS NOT NULL
GROUP BY release_id;
