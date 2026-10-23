-- Chat channels + deliveries ([T], tenant-scoped, FR-NTF-005) ---------------

-- name: CreateChatChannel :exec
INSERT INTO chat_channels (id, org_id, provider, name, url_enc, url_hint, categories, enabled, created_by)
VALUES (@id, @org_id, @provider, @name, @url_enc, @url_hint, @categories, @enabled, @created_by);

-- name: ListChatChannels :many
SELECT id, org_id, provider, name, url_enc, url_hint, categories, enabled, created_by, created_at
FROM chat_channels
WHERE org_id = @org_id
ORDER BY created_at, id;

-- name: GetChatChannel :one
SELECT id, org_id, provider, name, url_enc, url_hint, categories, enabled, created_by, created_at
FROM chat_channels
WHERE org_id = @org_id AND id = @id;

-- name: UpdateChatChannel :execrows
UPDATE chat_channels
SET name = @name, categories = @categories, enabled = @enabled
WHERE org_id = @org_id AND id = @id;

-- name: DeleteChatChannel :execrows
DELETE FROM chat_channels
WHERE org_id = @org_id AND id = @id;

-- name: CreateChatDelivery :exec
INSERT INTO chat_deliveries (id, org_id, channel_id, category, title, body, link)
VALUES (@id, @org_id, @channel_id, @category, @title, @body, @link);

-- name: GetChatDelivery :one
SELECT id, org_id, channel_id, category, title, body, link, status, attempts, last_error, created_at, sent_at
FROM chat_deliveries
WHERE org_id = @org_id AND id = @id;

-- name: ListChatDeliveries :many
SELECT d.id, d.org_id, d.channel_id, c.name AS channel_name, d.category, d.title, d.body, d.link,
       d.status, d.attempts, d.last_error, d.created_at, d.sent_at
FROM chat_deliveries d
JOIN chat_channels c ON c.org_id = d.org_id AND c.id = d.channel_id
WHERE d.org_id = @org_id
ORDER BY d.created_at DESC, d.id
LIMIT @lim;

-- name: RecordChatAttempt :execrows
UPDATE chat_deliveries
SET attempts = attempts + 1, status = @status, last_error = @last_error,
    sent_at = CASE WHEN @status::text = 'sent' THEN now() ELSE sent_at END
WHERE org_id = @org_id AND id = @id;

-- name: GetTaskPath :one
-- Org slug, project key and task number for a live task (chat deep link).
SELECT o.slug, p.key, t.number
FROM tasks t
JOIN projects p ON p.org_id = t.org_id AND p.id = t.project_id
JOIN organizations o ON o.id = t.org_id
WHERE t.org_id = @org_id AND t.id = @id AND t.deleted_at IS NULL;
