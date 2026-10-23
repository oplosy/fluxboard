-- 0052_sprint_points rollback.
ALTER TABLE sprints
  DROP COLUMN IF EXISTS completed_points_done,
  DROP COLUMN IF EXISTS completed_points_total;
