-- 0049_epics -- Epics (FR-PROJ-012, roadmap R-01). An epic groups related
-- tasks of one project; tasks join via tasks.epic_id (nullable: no epic).
-- Progress (done / total) is derived from the tasks, never stored.
-- DDL only; RLS lands in 0050 (the DDL/RLS split).
-- Times = timestamptz UTC. Every tenant-owned table carries org_id for RLS.

CREATE TABLE epics (                               -- [T]
  id          uuid PRIMARY KEY,
  org_id      uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  project_id  uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  name        text NOT NULL,
  color       text NOT NULL DEFAULT '',
  created_by  uuid NOT NULL REFERENCES users(id),
  created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX epics_project_idx ON epics (org_id, project_id, created_at);

-- Deleting an epic leaves its tasks in place, just without an epic.
ALTER TABLE tasks ADD COLUMN epic_id uuid REFERENCES epics(id) ON DELETE SET NULL;
CREATE INDEX tasks_epic_idx ON tasks (org_id, project_id, epic_id) WHERE deleted_at IS NULL;
