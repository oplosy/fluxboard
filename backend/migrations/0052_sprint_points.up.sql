-- 0052_sprint_points -- Velocity in story points (FR-AN-003, roadmap R-03).
-- Completing a sprint snapshots its total and done points next to the task
-- counts of 0021. Sprints completed before this migration keep 0: their
-- points at completion are unknown.
-- No new table, so no RLS migration: sprints already carries its policy.

ALTER TABLE sprints
  ADD COLUMN completed_points_total int NOT NULL DEFAULT 0,
  ADD COLUMN completed_points_done  int NOT NULL DEFAULT 0;
