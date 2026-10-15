-- Task approvals ([T], tenant-scoped, ADR-037) ---------------------------------

-- name: InsertTaskApproval :exec
INSERT INTO task_approvals (id, org_id, task_id, requested_by, note, deadline, state)
VALUES (@id, @org_id, @task_id, @requested_by, @note, sqlc.narg('deadline'), 'pending');

-- name: InsertTaskApprovalApprover :exec
INSERT INTO task_approval_approvers (approval_id, org_id, user_id)
VALUES (@approval_id, @org_id, @user_id);

-- name: GetTaskApproval :one
SELECT id, org_id, task_id, requested_by, note, deadline, state, stale_at, decided_at, created_at, updated_at
FROM task_approvals
WHERE org_id = @org_id AND task_id = @task_id AND id = @id;

-- name: LockTaskApproval :one
-- Serializes concurrent decisions on one request.
SELECT id, org_id, task_id, requested_by, note, deadline, state, stale_at, decided_at, created_at, updated_at
FROM task_approvals
WHERE org_id = @org_id AND task_id = @task_id AND id = @id
FOR UPDATE;

-- name: ListTaskApprovals :many
SELECT id, org_id, task_id, requested_by, note, deadline, state, stale_at, decided_at, created_at, updated_at
FROM task_approvals
WHERE org_id = @org_id AND task_id = @task_id
ORDER BY created_at DESC, id DESC;

-- name: ListApprovalApprovers :many
SELECT approval_id, user_id, decision, comment, decided_at
FROM task_approval_approvers
WHERE org_id = @org_id AND approval_id = ANY(@approval_ids::uuid[])
ORDER BY approval_id, user_id;

-- name: UpdateTaskApprovalState :exec
UPDATE task_approvals
SET state = @state, decided_at = sqlc.narg('decided_at')
WHERE org_id = @org_id AND id = @id;

-- name: SetApproverDecision :exec
UPDATE task_approval_approvers
SET decision = sqlc.narg('decision'), comment = @comment, decided_at = sqlc.narg('decided_at')
WHERE org_id = @org_id AND approval_id = @approval_id AND user_id = @user_id;

-- name: ListWaitingApprovals :many
SELECT a.id AS approval_id, a.task_id, t.project_id, t.number AS task_number, t.title, a.requested_by, a.deadline, a.created_at
FROM task_approval_approvers p
JOIN task_approvals a ON a.id = p.approval_id AND a.org_id = p.org_id
JOIN tasks t ON t.id = a.task_id AND t.org_id = a.org_id
WHERE p.org_id = @org_id AND p.user_id = @user_id AND p.decision IS NULL
  AND a.state = 'pending' AND t.deleted_at IS NULL
ORDER BY a.deadline ASC NULLS LAST, a.created_at ASC;
