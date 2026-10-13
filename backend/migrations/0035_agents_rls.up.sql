-- 0035_agents_rls -- Row-Level Security for agents. Same pattern as 0031:
-- predicate on the per-transaction GUC app.current_tenant set via SET LOCAL by
-- TenantPool; unset GUC means zero rows (fail-closed). ENABLE, not FORCE (ADR-014).

ALTER TABLE agents ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON agents
  USING (org_id = current_setting('app.current_tenant', true)::uuid)
  WITH CHECK (org_id = current_setting('app.current_tenant', true)::uuid);
