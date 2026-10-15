-- 0045_task_approvals -- Task approval requests (research report §7, P1; ADR-037).
--
-- A request names one or more approvers and an optional deadline. Each
-- approver decides once (approved / changes_requested, with a comment).
-- task_approvals.state is derived from the approver rows by the repository in
-- the same statement that records a decision: any changes_requested wins,
-- otherwise all approved means approved, otherwise pending.
--
-- stale_at: an approved request goes stale when its task, completed at that
-- moment, is reopened or has its content edited. A trigger sets it so every
-- writer (REST, bulk, automations, import, agents) is covered.

CREATE TABLE task_approvals (                        -- [T]
  id           uuid PRIMARY KEY,
  org_id       uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  task_id      uuid NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
  requested_by uuid NOT NULL REFERENCES users(id),
  note         text NOT NULL DEFAULT '',
  deadline     timestamptz,
  state        text NOT NULL DEFAULT 'pending'
               CHECK (state IN ('pending', 'approved', 'changes_requested', 'cancelled')),
  stale_at     timestamptz,
  decided_at   timestamptz,
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now()
);
-- One open request per task; settled ones stay as history.
CREATE UNIQUE INDEX task_approvals_one_pending ON task_approvals (task_id) WHERE state = 'pending';
CREATE INDEX task_approvals_task_idx ON task_approvals (task_id, created_at DESC);
CREATE TRIGGER task_approvals_set_updated_at BEFORE UPDATE ON task_approvals
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE task_approval_approvers (               -- [T]
  approval_id uuid NOT NULL REFERENCES task_approvals(id) ON DELETE CASCADE,
  org_id      uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  user_id     uuid NOT NULL REFERENCES users(id),
  decision    text CHECK (decision IN ('approved', 'changes_requested')),
  comment     text NOT NULL DEFAULT '',
  decided_at  timestamptz,
  PRIMARY KEY (approval_id, user_id)
);
CREATE INDEX task_approval_approvers_waiting_idx ON task_approval_approvers (org_id, user_id)
  WHERE decision IS NULL;

-- SECURITY DEFINER: a writer without the tenant GUC must still mark the
-- approval stale, so RLS cannot be allowed to hide the row. It only touches
-- approvals of the very task being written, scoped by org_id as well.
CREATE FUNCTION tasks_stale_approvals() RETURNS trigger AS $$
BEGIN
  UPDATE task_approvals SET stale_at = now()
   WHERE task_id = NEW.id AND org_id = NEW.org_id AND state = 'approved' AND stale_at IS NULL;
  RETURN NULL;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER SET search_path = public;

-- Runs after tasks_stamp_completed (BEFORE), so NEW.completed_at is final.
CREATE TRIGGER tasks_stale_approvals AFTER UPDATE ON tasks
  FOR EACH ROW
  WHEN (OLD.completed_at IS NOT NULL AND (
        NEW.completed_at IS NULL
        OR NEW.title IS DISTINCT FROM OLD.title
        OR NEW.description IS DISTINCT FROM OLD.description
        OR NEW.assignee_id IS DISTINCT FROM OLD.assignee_id
        OR NEW.priority IS DISTINCT FROM OLD.priority
        OR NEW.due_date IS DISTINCT FROM OLD.due_date))
  EXECUTE FUNCTION tasks_stale_approvals();
