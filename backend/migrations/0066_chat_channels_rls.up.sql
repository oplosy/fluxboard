-- 0066_chat_channels_rls -- Row-Level Security for chat_channels and
-- chat_deliveries. Same pattern as 0050: predicate on the per-transaction GUC
-- app.current_tenant set via SET LOCAL by TenantPool. Unset GUC means zero rows
-- (fail-closed). ENABLE (not FORCE).

ALTER TABLE chat_channels ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON chat_channels
  USING (org_id = current_setting('app.current_tenant', true)::uuid)
  WITH CHECK (org_id = current_setting('app.current_tenant', true)::uuid);

ALTER TABLE chat_deliveries ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON chat_deliveries
  USING (org_id = current_setting('app.current_tenant', true)::uuid)
  WITH CHECK (org_id = current_setting('app.current_tenant', true)::uuid);
