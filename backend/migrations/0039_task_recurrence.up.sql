-- 0039_task_recurrence -- Recurring tasks (research report P1).
--
-- A rule hangs off one task (the template). The worker copies the template into
-- a new task on each occurrence date. Occurrence dates are civil dates in the
-- rule's IANA timezone, so DST shifts never move or repeat a date.
--
-- last_on: the newest occurrence date already generated (or the day the rule
-- was set / resumed / edited). Nothing on or before it is generated again, which
-- is what makes edits and resumes affect future occurrences only.
--
-- tasks.(recurrence_id, occurrence_on) is each occurrence's stable identity; the
-- unique index makes a retried or concurrent generation a no-op instead of a
-- duplicate. Stopping a rule deletes it; its occurrences stay as plain tasks.

CREATE TABLE task_recurrences (                     -- [T]
  id         uuid PRIMARY KEY,
  org_id     uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  task_id    uuid NOT NULL UNIQUE REFERENCES tasks(id) ON DELETE CASCADE,
  freq       text NOT NULL CHECK (freq IN ('daily', 'weekly', 'monthly')),
  weekdays   smallint[] NOT NULL DEFAULT '{}',      -- weekly: 0 = Sunday … 6
  month_day  smallint CHECK (month_day BETWEEN 1 AND 31), -- monthly; clamped to month end
  timezone   text NOT NULL,
  mode       text NOT NULL CHECK (mode IN ('schedule', 'after_completion')),
  paused     boolean NOT NULL DEFAULT false,
  last_on    date NOT NULL,
  created_by uuid NOT NULL REFERENCES users(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX task_recurrences_active_idx ON task_recurrences (org_id) WHERE NOT paused;
CREATE TRIGGER task_recurrences_set_updated_at BEFORE UPDATE ON task_recurrences
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

ALTER TABLE tasks ADD COLUMN recurrence_id uuid REFERENCES task_recurrences(id) ON DELETE SET NULL;
ALTER TABLE tasks ADD COLUMN occurrence_on date;
CREATE UNIQUE INDEX tasks_occurrence_uniq ON tasks (recurrence_id, occurrence_on)
  WHERE recurrence_id IS NOT NULL;
