-- 0053_releases -- Releases / versions (FR-PROJ-010, roadmap R-05). A release
-- groups the tasks shipping together in one project; tasks join via
-- tasks.release_id (nullable: no release). released_at set = released.
-- Progress and release notes are derived from the tasks, never stored.
-- DDL only; RLS lands in 0054 (the DDL/RLS split).
-- Times = timestamptz UTC; release_date is a calendar date (no time of day).

CREATE TABLE releases (                            -- [T]
  id           uuid PRIMARY KEY,
  org_id       uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  project_id   uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  name         text NOT NULL,
  description  text NOT NULL DEFAULT '',
  release_date date,
  released_at  timestamptz,
  created_by   uuid NOT NULL REFERENCES users(id),
  created_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX releases_project_idx ON releases (org_id, project_id, created_at);

-- Deleting a release leaves its tasks in place, just without a release.
ALTER TABLE tasks ADD COLUMN release_id uuid REFERENCES releases(id) ON DELETE SET NULL;
CREATE INDEX tasks_release_idx ON tasks (org_id, project_id, release_id) WHERE deleted_at IS NULL;
