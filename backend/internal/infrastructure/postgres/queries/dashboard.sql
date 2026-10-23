-- Org dashboard (FR-AN-005, ADR-038). [T]; every query is scoped to the caller's
-- visible project ids, computed by ProjectRepository.List. "Done" is
-- tasks.completed_at (trigger-stamped, 0038); tasks already done before that
-- migration have no stamp and count as open.

-- name: DashboardProjectCounts :many
SELECT project_id,
       COUNT(*) FILTER (WHERE completed_at IS NULL)::int AS open_count,
       COUNT(*) FILTER (WHERE completed_at IS NOT NULL)::int AS done_count,
       COUNT(*) FILTER (WHERE completed_at IS NULL AND due_date < @now)::int AS overdue_count
FROM tasks
WHERE org_id = @org_id AND project_id = ANY(@project_ids::uuid[]) AND deleted_at IS NULL
GROUP BY project_id;

-- name: DashboardAssigneeCounts :many
-- Open tasks per project and assignee (assignee_id NULL = unassigned).
SELECT project_id, assignee_id,
       COUNT(*)::int AS open_count,
       COUNT(*) FILTER (WHERE due_date < @now)::int AS overdue_count
FROM tasks
WHERE org_id = @org_id AND project_id = ANY(@project_ids::uuid[])
  AND deleted_at IS NULL AND completed_at IS NULL
GROUP BY project_id, assignee_id;

-- name: DashboardWeeklyCompleted :many
-- Completed tasks per ISO week (Monday) from the nightly rollup.
SELECT project_id, date_trunc('week', day)::date AS week, SUM(completed_count)::int AS completed
FROM project_stats_daily
WHERE org_id = @org_id AND project_id = ANY(@project_ids::uuid[]) AND day >= @from_day
GROUP BY project_id, week
ORDER BY week;

-- name: GetDashboardLayout :one
SELECT widgets FROM dashboard_layouts WHERE org_id = @org_id AND user_id = @user_id;

-- name: UpsertDashboardLayout :exec
INSERT INTO dashboard_layouts (org_id, user_id, widgets)
VALUES (@org_id, @user_id, @widgets)
ON CONFLICT (org_id, user_id) DO UPDATE SET widgets = EXCLUDED.widgets, updated_at = now();
