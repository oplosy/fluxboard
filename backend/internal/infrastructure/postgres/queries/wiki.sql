-- Project wiki ([T], tenant-scoped, FR-PROJ-013) ----------------------------

-- name: CreateWikiPage :exec
INSERT INTO wiki_pages (id, org_id, project_id, title, body, created_by, updated_by)
VALUES (@id, @org_id, @project_id, @title, @body, @created_by, @created_by);

-- name: GetWikiPage :one
SELECT id, org_id, project_id, title, body, version, created_by, updated_by, created_at, updated_at
FROM wiki_pages
WHERE org_id = @org_id AND id = @id;

-- name: ListWikiPagesByProject :many
SELECT id, org_id, project_id, title, '' AS body, version, created_by, updated_by, created_at, updated_at
FROM wiki_pages
WHERE org_id = @org_id AND project_id = @project_id
  AND (@query::text = '' OR position(lower(@query::text) in lower(title)) > 0)
ORDER BY lower(title), id;

-- Optimistic concurrency: a stale if_version matches no row.
-- name: UpdateWikiPage :execrows
UPDATE wiki_pages
SET title = @title, body = @body, updated_by = @updated_by, version = version + 1
WHERE org_id = @org_id AND id = @id AND version = @if_version;

-- name: DeleteWikiPage :execrows
DELETE FROM wiki_pages
WHERE org_id = @org_id AND id = @id;

-- name: ClearWikiPageTasks :exec
DELETE FROM wiki_page_tasks
WHERE org_id = @org_id AND page_id = @page_id;

-- name: AddWikiPageTask :exec
INSERT INTO wiki_page_tasks (org_id, project_id, page_id, task_number)
VALUES (@org_id, @project_id, @page_id, @task_number)
ON CONFLICT DO NOTHING;

-- The live tasks a page mentions (soft-deleted tasks drop out).
-- name: ListWikiPageTaskRefs :many
SELECT t.number, t.title
FROM wiki_page_tasks m
JOIN tasks t ON t.org_id = m.org_id AND t.project_id = m.project_id AND t.number = m.task_number
WHERE m.org_id = @org_id AND m.page_id = @page_id AND t.deleted_at IS NULL
ORDER BY t.number;

-- name: ListWikiPagesMentioningTask :many
SELECT p.id, p.org_id, p.project_id, p.title, '' AS body, p.version, p.created_by, p.updated_by, p.created_at, p.updated_at
FROM wiki_page_tasks m
JOIN wiki_pages p ON p.id = m.page_id
WHERE m.org_id = @org_id AND m.project_id = @project_id AND m.task_number = @task_number
ORDER BY lower(p.title), p.id;
