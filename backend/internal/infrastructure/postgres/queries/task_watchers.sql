-- Task watchers ([T], tenant-scoped, FR-TASK-014) ------------------------------

-- name: AddTaskWatcher :exec
INSERT INTO task_watchers (task_id, user_id, org_id)
VALUES (@task_id, @user_id, @org_id)
ON CONFLICT (task_id, user_id) DO NOTHING;

-- name: DeleteTaskWatcher :exec
DELETE FROM task_watchers
WHERE org_id = @org_id AND task_id = @task_id AND user_id = @user_id;

-- name: ListTaskWatchers :many
SELECT u.id, u.email, u.name, tw.created_at
FROM task_watchers tw
JOIN users u ON u.id = tw.user_id
WHERE tw.org_id = @org_id AND tw.task_id = @task_id
ORDER BY tw.created_at;
