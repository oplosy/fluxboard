-- 0047_task_watchers -- Users who follow a task (FR-TASK-014). A watcher gets
-- comment notifications for the task like its creator and assignee do. One row
-- per (task, user); the row disappears with the task or the user.
-- DDL only; RLS lands in 0048 (the DDL/RLS split).

CREATE TABLE task_watchers (                          -- [T]
  task_id    uuid NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
  user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  org_id     uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (task_id, user_id)
);
CREATE INDEX task_watchers_user_idx ON task_watchers (org_id, user_id);
