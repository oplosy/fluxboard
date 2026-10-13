-- 0034_agents -- Agents as org principals (ADR-033, FR-AGT-002/003).
--
-- An agent is backed by a users row (same id) so every existing users(id) FK —
-- task creator/assignee, comment author, activity actor, AI run caller — can
-- point at it and it shows as itself. That row has no password, an unroutable
-- RFC 2606 .invalid address and no membership: it can never sign in and is
-- never a seat. The org-side state (name, role, paused) lives in agents [T].
-- Deleting an agent is soft (deleted_at) so its history keeps a name; it frees
-- the plan slot and revokes its keys.

ALTER TABLE plans ADD COLUMN max_agents int NOT NULL DEFAULT 1; -- -1 = unlimited
UPDATE plans SET max_agents = CASE code WHEN 'pro' THEN 10 WHEN 'business' THEN -1 ELSE 1 END;

CREATE TABLE agents (                                    -- [T]
  id          uuid PRIMARY KEY REFERENCES users(id),
  org_id      uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  name        text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 40),
  role        text NOT NULL DEFAULT 'MEMBER' CHECK (role IN ('MEMBER','GUEST')),
  status      text NOT NULL DEFAULT 'active' CHECK (status IN ('active','paused')),
  created_by  uuid REFERENCES users(id) ON DELETE SET NULL,
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now(),
  deleted_at  timestamptz
);
CREATE UNIQUE INDEX agents_org_name_live ON agents (org_id, lower(name)) WHERE deleted_at IS NULL;
CREATE TRIGGER agents_set_updated_at BEFORE UPDATE ON agents
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- An agent's keys; NULL = an org key (FR-API-001).
ALTER TABLE api_keys ADD COLUMN agent_id uuid REFERENCES agents(id) ON DELETE CASCADE;
CREATE INDEX api_keys_agent_idx ON api_keys (agent_id) WHERE agent_id IS NOT NULL;
