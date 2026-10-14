-- Project stats rollup (FR-AN-001). project_stats_daily [T]: nightly
-- recompute of yesterday's grain (set-semantics UPSERT): created and completed
-- counts, cycle time, and a current per-column open-task snapshot.

-- name: ListRollupProjectIDs :many
-- Non-archived, non-deleted projects for an org (rollup iteration).
SELECT id FROM projects
WHERE org_id = @org_id AND archived_at IS NULL AND deleted_at IS NULL;

-- name: CountTasksCreatedOnDay :one
-- [@day, @next_day) is the UTC day window (bounds computed by the caller).
SELECT COUNT(*) FROM tasks
WHERE org_id = @org_id AND project_id = @project_id
  AND created_at >= @day AND created_at < @next_day;

-- name: ColumnOpenCounts :many
-- Current open (non-trashed) task count per column for a project.
SELECT column_id, COUNT(*) AS cnt FROM tasks
WHERE org_id = @org_id AND project_id = @project_id AND deleted_at IS NULL
GROUP BY column_id;

-- name: UpsertProjectStat :exec
INSERT INTO project_stats_daily
  (org_id, project_id, day, created_count, completed_count, column_snapshot, avg_cycle_seconds)
VALUES (@org_id, @project_id, @day, @created_count, @completed_count, @column_snapshot, @avg_cycle_seconds)
ON CONFLICT (project_id, day) DO UPDATE SET
  created_count     = EXCLUDED.created_count,
  completed_count   = EXCLUDED.completed_count,
  column_snapshot   = EXCLUDED.column_snapshot,
  avg_cycle_seconds = EXCLUDED.avg_cycle_seconds;

-- name: CompletionsOnDay :one
-- Tasks that entered their done column in [@day, @next_day) and are still
-- there (tasks.completed_at, 0038). Cycle time runs from creation to completion.
SELECT COUNT(*) AS completed,
       COALESCE(SUM(EXTRACT(EPOCH FROM completed_at - created_at)), 0)::bigint AS cycle_seconds
FROM tasks
WHERE org_id = @org_id AND project_id = @project_id AND deleted_at IS NULL
  AND completed_at >= @day AND completed_at < @next_day;
