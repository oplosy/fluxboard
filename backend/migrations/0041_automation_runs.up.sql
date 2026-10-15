-- 0041_automation_runs -- Run history for automation rules (FR-AUTO, ADR-036):
-- every rule evaluation that fires is recorded, so "why did this task change?"
-- has an answer. DDL only; RLS lands in 0042 (the DDL/RLS split).
--
-- status: pending (claimed, action not finished) | success | failed | skipped
-- (loop-guard depth limit or the per-org action budget). depth is the cascade
-- depth: 0 = a human write fired the rule, n = fired by an automation write.
-- Time triggers (task.due_soon | task.overdue) fire at most once per task per
-- rule: the partial unique index is the claim the scheduled job inserts first.
-- Times = timestamptz UTC.

CREATE TABLE automation_runs (                     -- [T]
  id         uuid PRIMARY KEY,
  org_id     uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  rule_id    uuid NOT NULL REFERENCES automation_rules(id) ON DELETE CASCADE,
  trigger    text NOT NULL,
  task_id    uuid NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
  actor_id   uuid REFERENCES users(id),             -- the user whose write fired it; null for time triggers
  status     text NOT NULL CHECK (status IN ('pending','success','failed','skipped')),
  error      text,
  depth      integer NOT NULL DEFAULT 0,
  created_at timestamptz NOT NULL DEFAULT now()
);
-- Per-rule history (settings UI) and the per-org budget window count.
CREATE INDEX automation_runs_rule_idx ON automation_runs (rule_id, created_at DESC);
CREATE INDEX automation_runs_org_idx ON automation_runs (org_id, created_at DESC);
-- Once per task per rule for the time triggers.
CREATE UNIQUE INDEX automation_runs_time_once_idx ON automation_runs (rule_id, task_id)
  WHERE trigger IN ('task.due_soon', 'task.overdue');

-- Activity written by an automation names the rule behind it.
ALTER TABLE task_activity
  ADD COLUMN automation_rule_id uuid REFERENCES automation_rules(id) ON DELETE SET NULL;
