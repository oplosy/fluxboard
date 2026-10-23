-- 0057_wiki_pages -- Project wiki pages (FR-PROJ-013, roadmap R-13). A page is
-- a title + Markdown body (same format as task descriptions) owned by one
-- project; version backs optimistic concurrency (a stale edit is a 409).
-- wiki_page_tasks is the mention index: the task numbers of the project that
-- the body names as KEY-N, rebuilt on every save, so a task can list the pages
-- that mention it without scanning bodies. It stores numbers, not task ids, so
-- it survives task moves to trash and restores.
-- DDL only; RLS lands in 0058 (the DDL/RLS split).
-- Times = timestamptz UTC.

CREATE TABLE wiki_pages (                          -- [T]
  id          uuid PRIMARY KEY,
  org_id      uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  project_id  uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  title       text NOT NULL,
  body        text NOT NULL DEFAULT '',
  version     int NOT NULL DEFAULT 1,
  created_by  uuid NOT NULL REFERENCES users(id),
  updated_by  uuid NOT NULL REFERENCES users(id),
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX wiki_pages_project_idx ON wiki_pages (org_id, project_id, lower(title));

CREATE TABLE wiki_page_tasks (                     -- [T]
  org_id      uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  project_id  uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  page_id     uuid NOT NULL REFERENCES wiki_pages(id) ON DELETE CASCADE,
  task_number int NOT NULL,
  PRIMARY KEY (page_id, task_number)
);
CREATE INDEX wiki_page_tasks_task_idx ON wiki_page_tasks (org_id, project_id, task_number);
