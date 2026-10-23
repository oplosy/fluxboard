-- 0051_task_estimate -- Story points (FR-TASK-015, roadmap R-02). An optional
-- whole-number estimate per task; NULL = not estimated. Sums per column,
-- sprint and epic are derived from the tasks, never stored.
-- No new table, so no RLS migration: tasks already carries its policy.

ALTER TABLE tasks ADD COLUMN estimate integer
  CHECK (estimate >= 0 AND estimate <= 1000);
