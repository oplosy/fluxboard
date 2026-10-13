-- 0037_column_roles -- Workflow roles on board columns (ADR-033 decision 5,
-- FR-AGT-006). NULL ⇒ the role is inferred from the column name in code, so
-- existing boards need no data change. One column per role per board.

ALTER TABLE board_columns ADD COLUMN role text
  CHECK (role IN ('backlog', 'todo', 'in_progress', 'review', 'done'));
CREATE UNIQUE INDEX board_columns_role_idx ON board_columns (board_id, role) WHERE role IS NOT NULL;
