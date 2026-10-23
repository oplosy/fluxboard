-- 0058_wiki_pages_rls rollback.
DROP POLICY IF EXISTS tenant_isolation ON wiki_page_tasks;
ALTER TABLE wiki_page_tasks DISABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON wiki_pages;
ALTER TABLE wiki_pages DISABLE ROW LEVEL SECURITY;
