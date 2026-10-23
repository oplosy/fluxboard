-- Saved board views ([T], tenant-scoped, FR-PROJ-011) -----------------------

-- name: CreateSavedView :exec
INSERT INTO saved_views (id, org_id, project_id, owner_id, name, shared, filters)
VALUES (@id, @org_id, @project_id, @owner_id, @name, @shared, @filters);

-- name: GetSavedView :one
SELECT id, org_id, project_id, owner_id, name, shared, filters, created_at
FROM saved_views
WHERE org_id = @org_id AND id = @id;

-- name: ListVisibleSavedViews :many
-- The project's shared views plus the caller's own personal ones.
SELECT id, org_id, project_id, owner_id, name, shared, filters, created_at
FROM saved_views
WHERE org_id = @org_id AND project_id = @project_id AND (shared OR owner_id = @user_id)
ORDER BY shared DESC, created_at, id;

-- name: UpdateSavedView :execrows
UPDATE saved_views SET name = @name, shared = @shared, filters = @filters
WHERE org_id = @org_id AND id = @id;

-- name: DeleteSavedView :execrows
DELETE FROM saved_views
WHERE org_id = @org_id AND id = @id;
