-- 0036_task_claims -- Optimistic versions and agent claims on tasks (ADR-033
-- decisions 3–4, FR-AGT-004/005).
--
-- version: bumped by trigger on every change to the task's content, so every
-- writer (REST, bulk, automations, agents) counts without each query having to.
-- Rank-only reorders, claim/lease bookkeeping and updated_at do not bump it: an
-- agent's write should not be refused because someone dragged a card within its
-- column or the agent renewed its own lease.
--
-- claimed_by + lease_expires_at: an agent's claim. A claim past its lease is
-- inert (any agent may take the task); the sweeper job clears stale ones.

ALTER TABLE tasks ADD COLUMN version bigint NOT NULL DEFAULT 1;
ALTER TABLE tasks ADD COLUMN claimed_by uuid REFERENCES users(id);
ALTER TABLE tasks ADD COLUMN lease_expires_at timestamptz;
ALTER TABLE tasks ADD CONSTRAINT tasks_claim_has_lease
  CHECK ((claimed_by IS NULL) = (lease_expires_at IS NULL));
CREATE INDEX tasks_lease_idx ON tasks (org_id, lease_expires_at) WHERE claimed_by IS NOT NULL;

CREATE FUNCTION tasks_bump_version() RETURNS trigger AS $$
DECLARE
  ignored text[] := ARRAY['version', 'updated_at', 'rank', 'claimed_by', 'lease_expires_at', 'search_vector'];
BEGIN
  IF (to_jsonb(NEW) - ignored) IS DISTINCT FROM (to_jsonb(OLD) - ignored) THEN
    NEW.version := OLD.version + 1;
  ELSE
    NEW.version := OLD.version; -- never set by callers
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER tasks_bump_version BEFORE UPDATE ON tasks
  FOR EACH ROW EXECUTE FUNCTION tasks_bump_version();
