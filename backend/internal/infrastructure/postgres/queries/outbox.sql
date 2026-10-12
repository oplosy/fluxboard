-- Transactional outbox — Phase 4 / 09 §2 --------------------------------------
-- outbox [T]: producers INSERT within their write tx (the webhook does so in
-- webhook_repo); the worker's outbox:drain claims undrained rows and enqueues to
-- Asynq with TaskID = outbox id (at-most-once enqueue).

-- name: InsertOutbox :exec
INSERT INTO outbox (id, org_id, kind, payload)
VALUES (@id, @org_id, @kind, @payload);

-- name: LockOutboxBatch :many
-- Lock up to @lim undrained rows for the drain tx. FOR UPDATE SKIP LOCKED lets
-- concurrent drainers make progress without contending. The lock dies with the
-- tx, so a crash before MarkOutboxDrained leaves the rows undrained.
SELECT id, org_id, kind, payload, created_at, drained_at FROM outbox
WHERE org_id = @org_id AND drained_at IS NULL
ORDER BY created_at
LIMIT @lim
FOR UPDATE SKIP LOCKED;

-- name: MarkOutboxDrained :exec
-- Stamp drained_at on rows already handed off to Asynq, in the same tx that
-- locked them.
UPDATE outbox SET drained_at = now()
WHERE org_id = @org_id AND id = ANY(@ids::uuid[]);
