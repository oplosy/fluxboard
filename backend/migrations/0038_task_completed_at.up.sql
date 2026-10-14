-- 0038_task_completed_at -- When a task entered its board's done column
-- (FR-AN-001, research report I01). Feeds the nightly rollup's completed count
-- and cycle time.
--
-- Set by trigger, so every writer (REST, bulk, automations, agents, column
-- delete) counts without each query having to. Entering the done column stamps
-- now(); leaving it clears the stamp, so done → reopen → done counts the last
-- completion. Later column renames, role changes or reorders do not rewrite it.
-- Tasks already in a done column keep NULL: their completion time is unknown,
-- and a guess would show as fake history.

ALTER TABLE tasks ADD COLUMN completed_at timestamptz;
CREATE INDEX tasks_completed_idx ON tasks (org_id, project_id, completed_at)
  WHERE completed_at IS NOT NULL;

-- board_done_column mirrors project.DoneColumnID (domain/project/column_roles.go):
-- the explicit done role, else the left-most unroled column named done /
-- completed / complete, else the last column. Keep the two in step.
CREATE FUNCTION board_done_column(p_board uuid) RETURNS uuid AS $$
  SELECT COALESCE(
    (SELECT id FROM board_columns WHERE board_id = p_board AND role = 'done'),
    (SELECT id FROM board_columns
      WHERE board_id = p_board AND role IS NULL
        AND translate(lower(name), ' -_', '') IN ('done', 'completed', 'complete')
      ORDER BY rank LIMIT 1),
    (SELECT id FROM board_columns WHERE board_id = p_board ORDER BY rank DESC LIMIT 1));
$$ LANGUAGE sql STABLE;

CREATE FUNCTION tasks_stamp_completed() RETURNS trigger AS $$
BEGIN
  IF TG_OP = 'UPDATE' AND NEW.column_id IS NOT DISTINCT FROM OLD.column_id THEN
    NEW.completed_at := OLD.completed_at; -- never set by callers
    RETURN NEW;
  END IF;
  IF NEW.column_id = board_done_column(
       (SELECT board_id FROM board_columns WHERE id = NEW.column_id)) THEN
    NEW.completed_at := now();
  ELSE
    NEW.completed_at := NULL;
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER tasks_stamp_completed BEFORE INSERT OR UPDATE ON tasks
  FOR EACH ROW EXECUTE FUNCTION tasks_stamp_completed();
