-- Agents — org principals backed by a users row (ADR-033, FR-AGT-002/003).
-- agents is [T] (RLS via TenantPool); users is global. Create writes the users
-- row, the agents row and the first key in one tenant tx (agent_repo.go).

-- name: CreateAgentUser :exec
-- No password, an unroutable address, no membership: never signs in, never a seat.
INSERT INTO users (id, email, name, email_verified)
VALUES (@id, @email, @name, true);

-- name: CreateAgent :exec
INSERT INTO agents (id, org_id, name, role, created_by)
VALUES (@id, @org_id, @name, @role, @created_by);

-- name: ListAgents :many
SELECT id, org_id, name, role, status, created_by, created_at, updated_at, deleted_at
FROM agents
WHERE org_id = @org_id AND deleted_at IS NULL
ORDER BY created_at;

-- name: GetAgent :one
SELECT id, org_id, name, role, status, created_by, created_at, updated_at, deleted_at
FROM agents
WHERE org_id = @org_id AND id = @id AND deleted_at IS NULL;

-- name: CountAgents :one
-- Live agents, paused included: pausing must not dodge the plan cap.
SELECT count(*) FROM agents WHERE org_id = @org_id AND deleted_at IS NULL;

-- name: UpdateAgent :execrows
UPDATE agents
SET name = @name, role = @role, status = @status
WHERE org_id = @org_id AND id = @id AND deleted_at IS NULL;

-- name: DeleteAgent :execrows
UPDATE agents
SET deleted_at = now(), status = 'paused'
WHERE org_id = @org_id AND id = @id AND deleted_at IS NULL;

-- name: CreateAgentAPIKey :exec
INSERT INTO api_keys (id, org_id, prefix, key_hash, name, scopes, created_by, agent_id)
VALUES (@id, @org_id, @prefix, @key_hash, @name, @scopes, @created_by, @agent_id);

-- name: ListAgentAPIKeys :many
-- Active keys of every agent in the org; the repo groups them by agent.
SELECT id, org_id, prefix, key_hash, name, scopes, created_by,
       last_used_at, revoked_at, created_at, agent_id
FROM api_keys
WHERE org_id = @org_id AND agent_id IS NOT NULL AND revoked_at IS NULL
ORDER BY created_at;

-- name: RevokeAgentAPIKey :execrows
UPDATE api_keys
SET revoked_at = now()
WHERE org_id = @org_id AND agent_id = @agent_id AND id = @id AND revoked_at IS NULL;

-- name: RevokeAgentAPIKeys :exec
UPDATE api_keys
SET revoked_at = now()
WHERE org_id = @org_id AND agent_id = @agent_id AND revoked_at IS NULL;
