-- 0066_chat_channels_rls rollback.
DROP POLICY IF EXISTS tenant_isolation ON chat_deliveries;
ALTER TABLE chat_deliveries DISABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON chat_channels;
ALTER TABLE chat_channels DISABLE ROW LEVEL SECURITY;
