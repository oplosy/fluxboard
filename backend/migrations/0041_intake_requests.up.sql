-- 0041_intake_requests -- Request inbox (research report §6.3/§7). A public
-- form submission becomes a tracked request instead of a task: the submitter's
-- real name/email live on the request (never on a user row), the owner triages
-- it (accept -> task, decline with reason), and the submitter is mailed an
-- acknowledgment and, once the linked task completes, a completion email.
--
-- form_id is SET NULL on form delete so history survives; form_schema_version +
-- form_snapshot freeze what the submitter saw. content_hash (sha256 of the
-- normalized title/description/priority) with lower(submitter_email) is the
-- dedupe key within a window; the repo takes an advisory lock per
-- (form, email, hash) so concurrent identical posts collapse to one row.
-- ack_sent_at / outcome_sent_at are the mail sweeper's idempotency marks.
-- DDL only; RLS lands in 0042 (the DDL/RLS split).

ALTER TABLE project_forms ADD COLUMN schema_version integer NOT NULL DEFAULT 1;

CREATE TABLE intake_requests (                      -- [T]
  id                  uuid PRIMARY KEY,
  org_id              uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  project_id          uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  form_id             uuid REFERENCES project_forms(id) ON DELETE SET NULL,
  form_schema_version integer NOT NULL,
  form_snapshot       jsonb NOT NULL DEFAULT '{}',
  title               text NOT NULL,
  description         text NOT NULL DEFAULT '',
  priority            text NOT NULL DEFAULT 'none',
  submitter_name      text NOT NULL DEFAULT '',
  submitter_email     text NOT NULL DEFAULT '',
  content_hash        bytea NOT NULL,
  status              text NOT NULL DEFAULT 'new'
                      CHECK (status IN ('new', 'accepted', 'declined', 'done')),
  owner_id            uuid REFERENCES users(id) ON DELETE SET NULL,
  decline_reason      text NOT NULL DEFAULT '',
  task_id             uuid REFERENCES tasks(id) ON DELETE SET NULL,
  decided_by          uuid REFERENCES users(id) ON DELETE SET NULL,
  decided_at          timestamptz,
  received_at         timestamptz NOT NULL DEFAULT now(),
  ack_sent_at         timestamptz,
  outcome_sent_at     timestamptz,
  created_at          timestamptz NOT NULL DEFAULT now(),
  updated_at          timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX intake_requests_inbox_idx ON intake_requests (org_id, project_id, status, received_at DESC);
CREATE INDEX intake_requests_dedupe_idx ON intake_requests (form_id, lower(submitter_email), content_hash, received_at DESC);
CREATE INDEX intake_requests_pending_mail_idx ON intake_requests (org_id)
  WHERE submitter_email <> '' AND (ack_sent_at IS NULL OR (outcome_sent_at IS NULL AND status <> 'new'));
CREATE TRIGGER intake_requests_set_updated_at BEFORE UPDATE ON intake_requests
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();
