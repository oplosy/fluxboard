-- 0045_task_approvals rollback.
DROP TRIGGER IF EXISTS tasks_stale_approvals ON tasks;
DROP FUNCTION IF EXISTS tasks_stale_approvals();
DROP TABLE IF EXISTS task_approval_approvers;
DROP TABLE IF EXISTS task_approvals;
