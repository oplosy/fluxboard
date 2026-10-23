-- 0058_wiki_pages_rls -- Row-Level Security for wiki_pages and wiki_page_tasks.
-- Same pattern as 0054: predicate on the per-transaction GUC
-- app.current_tenant set via SET LOCAL by TenantPool. Unset GUC means zero
-- rows (fail-closed). ENABLE (not FORCE).

ALTER TABLE wiki_pages ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON wiki_pages
  USING (org_id = current_setting('app.current_tenant', true)::uuid)
  WITH CHECK (org_id = current_setting('app.current_tenant', true)::uuid);

ALTER TABLE wiki_page_tasks ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON wiki_page_tasks
  USING (org_id = current_setting('app.current_tenant', true)::uuid)
  WITH CHECK (org_id = current_setting('app.current_tenant', true)::uuid);
