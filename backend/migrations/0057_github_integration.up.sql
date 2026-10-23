-- 0057_github_integration -- GitHub pull-request linking (FR-TASK-016, roadmap
-- R-10, ADR-038). One integration per org; the webhook secret is stored
-- AES-256-GCM encrypted (HMAC verification needs the plaintext). Linked PRs are
-- one row per (task, repo, number). Delivery ids dedupe redelivered webhooks.
-- DDL only; RLS lands in 0058 (the DDL/RLS split).
-- Times = timestamptz UTC.

CREATE TABLE github_integrations (                 -- [T]
  org_id     uuid PRIMARY KEY REFERENCES organizations(id) ON DELETE CASCADE,
  secret_enc text NOT NULL,
  repos      text[] NOT NULL DEFAULT '{}',          -- lowercase owner/name; empty = any repo
  created_by uuid NOT NULL REFERENCES users(id),    -- actor for moves on merge
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

-- Projects whose linked tasks move to the done column when the PR merges.
CREATE TABLE github_merge_projects (               -- [T]
  org_id     uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  project_id uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  PRIMARY KEY (org_id, project_id)
);

CREATE TABLE task_pull_requests (                  -- [T]
  org_id     uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  task_id    uuid NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
  repo       text NOT NULL,
  number     int NOT NULL,
  title      text NOT NULL,
  state      text NOT NULL CHECK (state IN ('open','closed','merged')),
  url        text NOT NULL,
  pr_updated_at timestamptz NOT NULL,               -- GitHub's updated_at; guards out-of-order deliveries
  PRIMARY KEY (org_id, task_id, repo, number)
);
CREATE INDEX task_pull_requests_pr_idx ON task_pull_requests (org_id, repo, number);

CREATE TABLE github_deliveries (                   -- [T]
  org_id      uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  delivery_id text NOT NULL,
  received_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (org_id, delivery_id)
);
CREATE INDEX github_deliveries_received_idx ON github_deliveries (received_at);
