-- Tasks ([T], tenant-scoped) ------------------------------------------------

-- name: CreateTask :exec
INSERT INTO tasks (id, org_id, project_id, column_id, number, title, description,
                   assignee_id, priority, start_date, due_date, rank, created_by)
VALUES (@id, @org_id, @project_id, @column_id, @number, @title, @description,
        sqlc.narg('assignee_id'), @priority, sqlc.narg('start_date'), sqlc.narg('due_date'), @rank, @created_by);

-- name: GetTask :one
-- Live tasks only; trashed tasks are addressable through the Trash queries.
SELECT id, org_id, project_id, column_id, number, title, description,
       assignee_id, priority, start_date, due_date, sprint_id, rank, created_by, created_at, updated_at,
       version, claimed_by, lease_expires_at
FROM tasks
WHERE org_id = @org_id AND id = @id AND deleted_at IS NULL;

-- name: ListTasksByColumn :many
SELECT id, org_id, project_id, column_id, number, title, description,
       assignee_id, priority, start_date, due_date, sprint_id, rank, created_by, created_at, updated_at,
       version, claimed_by, lease_expires_at
FROM tasks
WHERE org_id = @org_id AND column_id = @column_id AND deleted_at IS NULL
ORDER BY rank;

-- name: ListTasksByProject :many
SELECT t.id, t.org_id, t.project_id, t.column_id, t.number, t.title, t.description,
       t.assignee_id, t.priority, t.start_date, t.due_date, t.sprint_id, t.rank, t.created_by, t.created_at, t.updated_at,
       t.version, t.claimed_by, t.lease_expires_at
FROM tasks t
JOIN board_columns c ON c.id = t.column_id
WHERE t.org_id = @org_id AND t.project_id = @project_id AND t.deleted_at IS NULL
ORDER BY c.rank, t.rank;

-- name: UpdateTask :execrows
UPDATE tasks
SET title = @title, description = @description,
    assignee_id = sqlc.narg('assignee_id'), priority = @priority,
    start_date = sqlc.narg('start_date'), due_date = sqlc.narg('due_date')
WHERE org_id = @org_id AND id = @id AND deleted_at IS NULL
  -- Optimistic concurrency (FR-AGT-005): with a version, a stale write matches
  -- no row and the repo reports a conflict.
  AND (sqlc.narg('if_version')::bigint IS NULL OR version = sqlc.narg('if_version'));

-- name: MoveTask :execrows
-- Relocate a task. The (column_id, rank) unique index makes a concurrent
-- identical move raise a unique violation → domain.ErrConflict → 409 (FR-PROJ-005).
UPDATE tasks SET column_id = @column_id, rank = @rank
WHERE org_id = @org_id AND id = @id AND deleted_at IS NULL
  AND (sqlc.narg('if_version')::bigint IS NULL OR version = sqlc.narg('if_version'));

-- Soft-delete / Trash (FR-TASK-009) -----------------------------------------

-- name: SoftDeleteTask :execrows
UPDATE tasks SET deleted_at = @deleted_at
WHERE org_id = @org_id AND id = @id AND deleted_at IS NULL;

-- name: RestoreTask :execrows
-- Restore only succeeds if the original (column_id, rank) slot is still free;
-- the unique index otherwise raises a conflict the repo maps to 409.
UPDATE tasks SET deleted_at = NULL
WHERE org_id = @org_id AND id = @id AND deleted_at IS NOT NULL;

-- name: GetTrashedTask :one
SELECT id, org_id, project_id, column_id, number, title, description,
       assignee_id, priority, start_date, due_date, sprint_id, rank, created_by, created_at, updated_at,
       version, claimed_by, lease_expires_at
FROM tasks
WHERE org_id = @org_id AND id = @id AND deleted_at IS NOT NULL;

-- name: ListTrashedTasks :many
SELECT id, org_id, project_id, column_id, number, title, description,
       assignee_id, priority, start_date, due_date, sprint_id, rank, created_by, created_at, updated_at,
       version, claimed_by, lease_expires_at
FROM tasks
WHERE org_id = @org_id AND project_id = @project_id AND deleted_at IS NOT NULL
ORDER BY deleted_at DESC;

-- name: PurgeExpiredTrash :execrows
-- Hard-delete tasks trashed before the cutoff (nightly purge job, per-tenant).
DELETE FROM tasks
WHERE org_id = @org_id AND deleted_at IS NOT NULL AND deleted_at < @cutoff;

-- Full-text search (FR-TASK-007) --------------------------------------------

-- name: SearchTasks :many
-- Org-wide task search over title+description with optional facet filters. An
-- empty @query skips the text match (facet-only listing, newest first).
-- Visibility is enforced in-DB: a caller sees a task only if the caller is an
-- org admin (@see_all), or a member of the project, or the project is 'org'-
-- visible and the caller may see those (@see_org_visible: org MEMBER+, never
-- GUEST — FR-TEN-010). Archived projects are excluded. count(*) OVER() yields
-- the unpaged total for pagination. The label filter is an EXISTS, not a join:
-- joining task_labels returned a task once per label (duplicate hits, inflated
-- total).
SELECT t.id, t.org_id, t.project_id, t.column_id, t.number, t.title, t.description,
       t.assignee_id, t.priority, t.start_date, t.due_date, t.sprint_id, t.rank, t.created_by,
       t.created_at, t.updated_at, t.version, t.claimed_by, t.lease_expires_at,
       count(*) OVER() AS total_count
FROM tasks t
JOIN projects p ON p.id = t.project_id
WHERE t.org_id = @org_id
  AND t.deleted_at IS NULL
  AND p.archived_at IS NULL
  AND (@query::text = '' OR t.search_vector @@ websearch_to_tsquery('simple', @query))
  AND (@see_all::bool OR (@see_org_visible::bool AND p.visibility = 'org') OR EXISTS (
        SELECT 1 FROM project_members pm
        WHERE pm.project_id = p.id AND pm.user_id = @user_id))
  AND (sqlc.narg('project_id')::uuid IS NULL OR t.project_id = sqlc.narg('project_id'))
  AND (sqlc.narg('assignee_id')::uuid IS NULL OR t.assignee_id = sqlc.narg('assignee_id'))
  AND (sqlc.narg('column_id')::uuid IS NULL OR t.column_id = sqlc.narg('column_id'))
  AND (sqlc.narg('priority')::text IS NULL OR t.priority = sqlc.narg('priority'))
  AND (sqlc.narg('label_id')::uuid IS NULL OR EXISTS (
        SELECT 1 FROM task_labels tl
        WHERE tl.task_id = t.id AND tl.org_id = t.org_id AND tl.label_id = sqlc.narg('label_id')))
ORDER BY CASE WHEN @query::text = '' THEN 0
              ELSE ts_rank(t.search_vector, websearch_to_tsquery('simple', @query)) END DESC,
         t.created_at DESC
LIMIT @lim OFFSET @off;

-- Bulk actions (FR-TASK-008) ------------------------------------------------
-- Each runs inside one repo transaction over a task-id array; RLS + org_id scope
-- every row. Bulk move relocates to a single column but must give each task a
-- distinct rank, so the repo issues per-task MoveTask calls instead of a set-based
-- UPDATE (the (column_id, rank) unique index forbids sharing a rank).

-- name: BulkAssignTasks :execrows
UPDATE tasks SET assignee_id = sqlc.narg('assignee_id')
WHERE org_id = @org_id AND deleted_at IS NULL AND id = ANY(@ids::uuid[]);

-- name: ValidateTaskIDs :one
-- Count how many of the given ids are live tasks in this org (bulk pre-check).
SELECT count(*) FROM tasks
WHERE org_id = @org_id AND deleted_at IS NULL AND id = ANY(@ids::uuid[]);

-- Agent claims (ADR-033, FR-AGT-004) ----------------------------------------
-- A claim is held while lease_expires_at is in the future. Each statement is a
-- single conditional UPDATE, so two agents racing for a task cannot both win.

-- name: ClaimTask :execrows
-- Claim (or renew) a task for an agent and assign it. Blocked by another
-- agent's live lease, or by an assignee who is neither this agent nor the
-- holder of a lapsed claim (a person, or someone assigned by a person).
UPDATE tasks
SET claimed_by = @agent_id, lease_expires_at = @expires_at, assignee_id = @agent_id
WHERE org_id = @org_id AND id = @id AND deleted_at IS NULL
  AND (claimed_by IS NULL OR claimed_by = @agent_id OR lease_expires_at <= now())
  AND (assignee_id IS NULL OR assignee_id = @agent_id OR assignee_id = claimed_by);

-- name: HeartbeatTask :execrows
-- Extend a live claim the agent holds; a lapsed or foreign claim matches nothing.
UPDATE tasks SET lease_expires_at = @expires_at
WHERE org_id = @org_id AND id = @id AND deleted_at IS NULL
  AND claimed_by = @agent_id AND lease_expires_at > now();

-- name: ReleaseTask :execrows
-- Give a claim back: the task is unassigned again if the agent still holds it.
UPDATE tasks
SET claimed_by = NULL, lease_expires_at = NULL,
    assignee_id = CASE WHEN assignee_id = @agent_id THEN NULL ELSE assignee_id END
WHERE org_id = @org_id AND id = @id AND deleted_at IS NULL AND claimed_by = @agent_id;

-- name: ReleaseExpiredClaims :execrows
-- Sweeper: clear lapsed claims and the assignment they made.
UPDATE tasks
SET claimed_by = NULL, lease_expires_at = NULL,
    assignee_id = CASE WHEN assignee_id = claimed_by THEN NULL ELSE assignee_id END
WHERE org_id = @org_id AND claimed_by IS NOT NULL AND lease_expires_at <= now();
