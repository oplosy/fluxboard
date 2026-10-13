-- 0037_column_roles rollback.
DROP INDEX IF EXISTS board_columns_role_idx;
ALTER TABLE board_columns DROP COLUMN IF EXISTS role;
