-- GitHub integration ([T], tenant-scoped, FR-TASK-016) -----------------------

-- name: GetGithubIntegration :one
SELECT org_id, secret_enc, repos, created_by, created_at, updated_at
FROM github_integrations
WHERE org_id = @org_id;

-- name: UpsertGithubSecret :exec
INSERT INTO github_integrations (org_id, secret_enc, created_by)
VALUES (@org_id, @secret_enc, @created_by)
ON CONFLICT (org_id) DO UPDATE
SET secret_enc = EXCLUDED.secret_enc, created_by = EXCLUDED.created_by, updated_at = now();

-- name: SetGithubRepos :execrows
UPDATE github_integrations SET repos = @repos::text[], updated_at = now()
WHERE org_id = @org_id;

-- name: DeleteGithubIntegration :exec
DELETE FROM github_integrations WHERE org_id = @org_id;

-- name: ClearGithubMergeProjects :exec
DELETE FROM github_merge_projects WHERE org_id = @org_id;

-- name: AddGithubMergeProjects :execrows
INSERT INTO github_merge_projects (org_id, project_id)
SELECT @org_id, p.id FROM projects p
WHERE p.org_id = @org_id AND p.id = ANY(@project_ids::uuid[]);

-- name: ListGithubMergeProjects :many
SELECT project_id FROM github_merge_projects WHERE org_id = @org_id;

-- name: ResolveTasksByKeys :many
SELECT t.id, t.project_id
FROM tasks t
JOIN projects p ON p.id = t.project_id AND p.org_id = t.org_id
WHERE t.org_id = @org_id AND t.deleted_at IS NULL
  AND p.key || '-' || t.number::text = ANY(@keys::text[]);

-- name: RecordGithubDelivery :execrows
INSERT INTO github_deliveries (org_id, delivery_id)
VALUES (@org_id, @delivery_id)
ON CONFLICT DO NOTHING;

-- name: PruneGithubDeliveries :exec
DELETE FROM github_deliveries
WHERE org_id = @org_id AND received_at < now() - interval '30 days';

-- name: UpsertTaskPullRequest :exec
INSERT INTO task_pull_requests (org_id, task_id, repo, number, title, state, url, pr_updated_at)
VALUES (@org_id, @task_id, @repo, @number, @title, @state, @url, @pr_updated_at)
ON CONFLICT (org_id, task_id, repo, number) DO UPDATE
SET title = EXCLUDED.title, state = EXCLUDED.state, url = EXCLUDED.url, pr_updated_at = EXCLUDED.pr_updated_at
WHERE EXCLUDED.pr_updated_at >= task_pull_requests.pr_updated_at;

-- A PR edited so that it no longer names a task keeps that task's link but
-- still gets the fresh title/state.
-- name: RefreshPullRequestRows :exec
UPDATE task_pull_requests
SET title = @title, state = @state, url = @url, pr_updated_at = @pr_updated_at
WHERE org_id = @org_id AND repo = @repo AND number = @number
  AND pr_updated_at <= @pr_updated_at;

-- name: ListTaskPullRequests :many
SELECT repo, number, title, state, url, pr_updated_at
FROM task_pull_requests
WHERE org_id = @org_id AND task_id = @task_id
ORDER BY pr_updated_at DESC, repo, number;
