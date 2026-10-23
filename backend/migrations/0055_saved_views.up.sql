-- 0055_saved_views -- Saved board views (FR-PROJ-011, roadmap R-07). A view
-- stores a named set of board filters and the swimlane grouping. Personal
-- views (shared = false) belong to owner_id alone; shared views are visible to
-- the whole project and managed by its LEADs. filters is a small JSON object
-- validated by the application (project.NormalizeViewFilters).
-- DDL only; RLS lands in 0056 (the DDL/RLS split).

CREATE TABLE saved_views (                         -- [T]
  id          uuid PRIMARY KEY,
  org_id      uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  project_id  uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  owner_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name        text NOT NULL,
  shared      boolean NOT NULL DEFAULT false,
  filters     jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX saved_views_project_idx ON saved_views (org_id, project_id, created_at);
