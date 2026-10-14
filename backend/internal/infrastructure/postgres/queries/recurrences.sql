-- Task recurrence rules ([T], tenant-scoped, research report P1) ---------------

-- name: GetTaskRecurrence :one
SELECT id, org_id, task_id, freq, weekdays, month_day, timezone, mode, paused, last_on, created_by
FROM task_recurrences
WHERE org_id = @org_id AND task_id = @task_id;

-- name: UpsertTaskRecurrence :exec
-- One rule per template task; an edit replaces the rule in place so its id, and
-- with it the identity of every occurrence already generated, survives.
INSERT INTO task_recurrences (id, org_id, task_id, freq, weekdays, month_day, timezone, mode, paused, last_on, created_by)
VALUES (@id, @org_id, @task_id, @freq, @weekdays, sqlc.narg('month_day'), @timezone, @mode, @paused, @last_on, @created_by)
ON CONFLICT (task_id) DO UPDATE
SET freq = EXCLUDED.freq, weekdays = EXCLUDED.weekdays, month_day = EXCLUDED.month_day,
    timezone = EXCLUDED.timezone, mode = EXCLUDED.mode, paused = EXCLUDED.paused,
    last_on = EXCLUDED.last_on;

-- name: DeleteTaskRecurrence :execrows
DELETE FROM task_recurrences WHERE org_id = @org_id AND task_id = @task_id;

-- name: ListActiveRecurrences :many
-- Active rules whose template is live, with what an occurrence copies from the
-- template, the template's board, and the completion (or trashing)
-- of the series' latest instance for "after completion" rules.
SELECT r.id, r.task_id, r.freq, r.weekdays, r.month_day, r.timezone, r.mode, r.last_on, r.created_by,
       t.project_id, t.title, t.description, t.assignee_id, t.priority,
       c.board_id,
       latest.completed_at AS latest_completed_at, latest.deleted_at AS latest_deleted_at
FROM task_recurrences r
JOIN tasks t ON t.id = r.task_id
JOIN board_columns c ON c.id = t.column_id
LEFT JOIN LATERAL (
  SELECT i.completed_at, i.deleted_at FROM tasks i
  WHERE i.org_id = r.org_id AND (i.id = r.task_id OR i.recurrence_id = r.id)
  ORDER BY CASE WHEN i.id = r.task_id THEN NULL ELSE i.occurrence_on END DESC NULLS LAST
  LIMIT 1
) latest ON true
WHERE r.org_id = @org_id AND NOT r.paused AND t.deleted_at IS NULL;

-- name: LockRecurrence :one
-- Serializes generation per rule; the caller skips dates on or before last_on.
SELECT last_on, paused FROM task_recurrences
WHERE org_id = @org_id AND id = @id
FOR UPDATE;

-- name: CreateOccurrenceTask :execrows
-- (recurrence_id, occurrence_on) is unique: a repeated generation inserts nothing.
INSERT INTO tasks (id, org_id, project_id, column_id, number, title, description,
                   assignee_id, priority, due_date, rank, created_by, recurrence_id, occurrence_on)
VALUES (@id, @org_id, @project_id, @column_id, @number, @title, @description,
        sqlc.narg('assignee_id'), @priority, @due_date, @rank, @created_by, @recurrence_id, @occurrence_on::date)
ON CONFLICT (recurrence_id, occurrence_on) WHERE recurrence_id IS NOT NULL DO NOTHING;

-- name: AdvanceRecurrence :exec
UPDATE task_recurrences SET last_on = GREATEST(last_on, @last_on::date)
WHERE org_id = @org_id AND id = @id;
