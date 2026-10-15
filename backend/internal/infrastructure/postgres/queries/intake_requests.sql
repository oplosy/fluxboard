-- Request inbox ([T], tenant-scoped, research report §6.3/§7) -----------------

-- name: LockIntakeDedupe :exec
-- Serializes concurrent identical submissions (same form + email + content) for
-- the rest of the tx so the duplicate check below cannot be raced.
SELECT pg_advisory_xact_lock(hashtextextended(@key::text, 0));

-- name: FindRecentIntakeDuplicate :one
SELECT id FROM intake_requests
WHERE org_id = @org_id AND form_id = @form_id
  AND lower(submitter_email) = lower(@submitter_email::text)
  AND content_hash = @content_hash AND received_at >= @since
ORDER BY received_at DESC
LIMIT 1;

-- name: CountRecentIntakeForForm :one
SELECT count(*) FROM intake_requests
WHERE org_id = @org_id AND form_id = @form_id AND received_at >= @since;

-- name: InsertIntakeRequest :exec
INSERT INTO intake_requests (id, org_id, project_id, form_id, form_schema_version, form_snapshot,
                             title, description, priority, submitter_name, submitter_email,
                             content_hash, owner_id)
VALUES (@id, @org_id, @project_id, @form_id, @form_schema_version, @form_snapshot,
        @title, @description, @priority, @submitter_name, @submitter_email,
        @content_hash, sqlc.narg('owner_id'));

-- name: GetIntakeRequest :one
SELECT id, org_id, project_id, form_id, form_schema_version, form_snapshot, title, description,
       priority, submitter_name, submitter_email, status, owner_id, decline_reason, task_id,
       decided_by, decided_at, received_at, ack_sent_at, outcome_sent_at
FROM intake_requests
WHERE org_id = @org_id AND id = @id;

-- name: ListIntakeRequests :many
SELECT id, org_id, project_id, form_id, form_schema_version, form_snapshot, title, description,
       priority, submitter_name, submitter_email, status, owner_id, decline_reason, task_id,
       decided_by, decided_at, received_at, ack_sent_at, outcome_sent_at
FROM intake_requests
WHERE org_id = @org_id AND project_id = @project_id
  AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status')::text)
ORDER BY received_at DESC
LIMIT @lim;

-- name: TriageIntakeRequest :execrows
-- Owner / priority edits; only while the request is still open (new).
UPDATE intake_requests
SET owner_id = sqlc.narg('owner_id'), priority = @priority
WHERE org_id = @org_id AND id = @id AND status = 'new';

-- name: AcceptIntakeRequest :execrows
-- The status guard makes a double-click / concurrent accept create one task:
-- the loser matches zero rows and the caller rolls its task back.
UPDATE intake_requests
SET status = 'accepted', task_id = @task_id, decided_by = @decided_by, decided_at = now()
WHERE org_id = @org_id AND id = @id AND status = 'new';

-- name: DeclineIntakeRequest :execrows
UPDATE intake_requests
SET status = 'declined', decline_reason = @decline_reason, decided_by = @decided_by, decided_at = now()
WHERE org_id = @org_id AND id = @id AND status = 'new';

-- name: CompleteIntakeRequests :many
-- accepted -> done once the linked task has been completed (tasks.completed_at,
-- trigger-stamped since 0038). Returns the flipped ids for logging.
UPDATE intake_requests r
SET status = 'done'
FROM tasks t
WHERE r.org_id = @org_id AND r.status = 'accepted'
  AND t.id = r.task_id AND t.org_id = r.org_id
  AND t.completed_at IS NOT NULL AND t.deleted_at IS NULL
RETURNING r.id;

-- name: ListIntakeMailDue :many
-- Requests owing the submitter an acknowledgment, or an outcome mail once
-- decided (declined / done). Never anything for an address-less submission.
SELECT r.id, r.title, r.submitter_name, r.submitter_email, r.status, r.decline_reason,
       r.ack_sent_at, r.outcome_sent_at, COALESCE(f.name, r.form_snapshot->>'name', '')::text AS form_name
FROM intake_requests r
LEFT JOIN project_forms f ON f.id = r.form_id AND f.org_id = r.org_id
WHERE r.org_id = @org_id AND r.submitter_email <> ''
  AND (r.ack_sent_at IS NULL OR (r.outcome_sent_at IS NULL AND r.status IN ('declined', 'done')))
ORDER BY r.received_at
LIMIT @lim;

-- name: MarkIntakeAcked :exec
UPDATE intake_requests SET ack_sent_at = now()
WHERE org_id = @org_id AND id = @id AND ack_sent_at IS NULL;

-- name: MarkIntakeOutcomeSent :exec
UPDATE intake_requests SET outcome_sent_at = now()
WHERE org_id = @org_id AND id = @id AND outcome_sent_at IS NULL;
