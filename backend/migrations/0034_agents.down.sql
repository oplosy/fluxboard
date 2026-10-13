-- 0034_agents rollback. The agents' backing users rows stay: tasks, comments
-- and activity may still reference them, and they cannot sign in.
DROP INDEX IF EXISTS api_keys_agent_idx;
DELETE FROM api_keys WHERE agent_id IS NOT NULL;
ALTER TABLE api_keys DROP COLUMN IF EXISTS agent_id;
DROP TABLE IF EXISTS agents;
ALTER TABLE plans DROP COLUMN IF EXISTS max_agents;
