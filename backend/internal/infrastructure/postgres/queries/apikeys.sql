-- API keys — Phase 6 (docs/build/PHASE-6-ADMIN-OBS.md §4, FR-API-001/002).
-- api_keys is [T] (RLS). Create/List/Revoke run under a tenant tx (TenantPool).
-- GetByHash + TouchLastUsed authenticate an inbound key BEFORE any tenant context
-- exists, so their repo methods run on the owner pool (RLS-bypassing); the hash is
-- globally unique + unguessable, making the cross-org lookup safe.

-- name: CreateAPIKey :exec
INSERT INTO api_keys (id, org_id, prefix, key_hash, name, scopes, created_by)
VALUES (@id, @org_id, @prefix, @key_hash, @name, @scopes, @created_by);

-- name: ListAPIKeysByOrg :many
-- Org keys only; an agent's keys are listed with the agent (agents.sql).
SELECT id, org_id, prefix, key_hash, name, scopes, created_by,
       last_used_at, revoked_at, created_at, agent_id
FROM api_keys
WHERE org_id = @org_id AND agent_id IS NULL
ORDER BY created_at DESC;

-- name: GetAPIKeyByHash :one
-- Auth-path lookup (owner pool). Revoked keys ARE returned so the caller maps them
-- to 401 rather than a silent miss. An agent's key carries the agent's role and
-- status (ADR-033); the owner pool reads agents across tenants like api_keys.
SELECT k.id, k.org_id, k.prefix, k.key_hash, k.name, k.scopes, k.created_by,
       k.last_used_at, k.revoked_at, k.created_at, k.agent_id,
       a.role AS agent_role, a.status AS agent_status
FROM api_keys k
LEFT JOIN agents a ON a.id = k.agent_id
WHERE k.key_hash = @key_hash;

-- name: RevokeAPIKey :execrows
-- rows-affected 0 ⇒ absent or already revoked ⇒ caller returns ErrNotFound.
UPDATE api_keys
SET revoked_at = now()
WHERE org_id = @org_id AND id = @id AND revoked_at IS NULL;

-- name: TouchAPIKeyLastUsed :exec
UPDATE api_keys SET last_used_at = now() WHERE key_hash = @key_hash;
