-- 0058_goals_rls -- Row-Level Security for goals and goal_links. Same pattern as
-- 0050: predicate on the per-transaction GUC app.current_tenant set via SET
-- LOCAL by TenantPool. Unset GUC means zero rows (fail-closed). ENABLE (not FORCE).

ALTER TABLE goals ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON goals
  USING (org_id = current_setting('app.current_tenant', true)::uuid)
  WITH CHECK (org_id = current_setting('app.current_tenant', true)::uuid);

ALTER TABLE goal_links ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON goal_links
  USING (org_id = current_setting('app.current_tenant', true)::uuid)
  WITH CHECK (org_id = current_setting('app.current_tenant', true)::uuid);
