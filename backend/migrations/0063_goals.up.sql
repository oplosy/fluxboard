-- 0063_goals -- Goals (FR-PROJ-014, roadmap R-12). An org-level goal links
-- epics and tasks of any project; progress and status are derived on read from
-- the linked work, never stored. Deleting an epic or task drops its link.
-- DDL only; RLS lands in 0064 (the DDL/RLS split).
-- Times = timestamptz UTC; target_date is a calendar date (no time of day).

CREATE TABLE goals (                               -- [T]
  id          uuid PRIMARY KEY,
  org_id      uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  title       text NOT NULL,
  description text NOT NULL DEFAULT '',
  owner_id    uuid NOT NULL REFERENCES users(id),
  target_date date,
  created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX goals_org_idx ON goals (org_id, created_at);

CREATE TABLE goal_links (                          -- [T]
  id         uuid PRIMARY KEY,
  org_id     uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  goal_id    uuid NOT NULL REFERENCES goals(id) ON DELETE CASCADE,
  epic_id    uuid REFERENCES epics(id) ON DELETE CASCADE,
  task_id    uuid REFERENCES tasks(id) ON DELETE CASCADE,
  created_at timestamptz NOT NULL DEFAULT now(),
  CHECK ((epic_id IS NULL) <> (task_id IS NULL))   -- exactly one target
);
CREATE UNIQUE INDEX goal_links_epic_uq ON goal_links (goal_id, epic_id) WHERE epic_id IS NOT NULL;
CREATE UNIQUE INDEX goal_links_task_uq ON goal_links (goal_id, task_id) WHERE task_id IS NOT NULL;
CREATE INDEX goal_links_org_idx ON goal_links (org_id, goal_id);
