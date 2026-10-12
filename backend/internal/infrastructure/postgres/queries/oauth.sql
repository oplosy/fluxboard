-- name: GetOAuthIdentity :one
-- Looks up an external identity by (provider, subject). domain.ErrNotFound when
-- absent drives the link-or-create branch in the OAuth callback (04 §4).
SELECT id, user_id, provider, provider_sub, created_at
FROM oauth_identities
WHERE provider = @provider AND provider_sub = @provider_sub;

-- name: CreateOAuthIdentity :exec
INSERT INTO oauth_identities (id, user_id, provider, provider_sub)
VALUES (@id, @user_id, @provider, @provider_sub);

-- name: ListOAuthIdentitiesByUser :many
-- The caller's linked sign-in identities, for the personal data export (B09).
SELECT id, user_id, provider, provider_sub, created_at
FROM oauth_identities
WHERE user_id = @user_id
ORDER BY created_at;
