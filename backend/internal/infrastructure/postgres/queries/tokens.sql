-- name: CreateOneTimeToken :exec
INSERT INTO one_time_tokens (id, purpose, user_id, token_hash, payload, expires_at)
VALUES (@id, @purpose, @user_id, @token_hash, @payload, @expires_at);

-- name: ConsumeOneTimeToken :one
UPDATE one_time_tokens
SET used_at = now()
WHERE purpose = @purpose
  AND token_hash = @token_hash
  AND used_at IS NULL
  AND expires_at > now()
RETURNING id, purpose, user_id, token_hash, payload, expires_at, used_at, created_at;

-- name: GetActiveOneTimeToken :one
-- Read-only validity check: lets a flow validate its input (e.g. the new
-- password) before burning the single-use token with ConsumeOneTimeToken.
SELECT id, purpose, user_id, token_hash, payload, expires_at, used_at, created_at
FROM one_time_tokens
WHERE purpose = @purpose
  AND token_hash = @token_hash
  AND used_at IS NULL
  AND expires_at > now();

-- name: InvalidateUserOneTimeTokens :exec
-- Retires every outstanding token of one purpose for a user, so older links
-- from earlier emails stop working once one of them has been used.
UPDATE one_time_tokens
SET used_at = now()
WHERE user_id = @user_id
  AND purpose = @purpose
  AND used_at IS NULL;
