import { API_BASE, apiFetch } from './client';

/** Org GitHub integration settings (GET .../integrations/github, ADMIN). The secret is never returned. */
export interface GithubSettings {
  configured: boolean;
  /** Path of the public webhook endpoint, e.g. /api/v1/integrations/github/{orgId}/webhook. */
  webhook_path: string;
  /** Allowed repositories as owner/name; empty means any repository. */
  repos: string[];
  /** Projects whose linked tasks move to the done column when the PR merges. */
  move_on_merge_project_ids: string[];
}

/** A pull request linked to a task (FR-TASK-016). */
export interface LinkedPullRequest {
  repo: string;
  number: number;
  title: string;
  state: 'open' | 'closed' | 'merged';
  url: string;
  updated_at: string;
}

export function getGithubSettings(orgId: string): Promise<GithubSettings> {
  return apiFetch(`/orgs/${orgId}/integrations/github`);
}

/** Create or rotate the webhook secret; the plaintext is returned once. */
export async function rotateGithubSecret(orgId: string): Promise<string> {
  const res = await apiFetch<{ secret: string }>(`/orgs/${orgId}/integrations/github/secret`, {
    method: 'POST',
  });
  return res.secret;
}

export function updateGithubSettings(
  orgId: string,
  input: { repos: string[]; move_on_merge_project_ids: string[] },
): Promise<GithubSettings> {
  return apiFetch(`/orgs/${orgId}/integrations/github`, { method: 'PUT', body: input });
}

export function disconnectGithub(orgId: string): Promise<void> {
  return apiFetch(`/orgs/${orgId}/integrations/github`, { method: 'DELETE' });
}

export async function listTaskPullRequests(
  orgId: string,
  taskId: string,
): Promise<LinkedPullRequest[]> {
  const res = await apiFetch<{ pull_requests: LinkedPullRequest[] }>(
    `/orgs/${orgId}/tasks/${taskId}/pull-requests`,
  );
  return res.pull_requests ?? [];
}

/** Absolute webhook URL to paste into GitHub (webhook_path is rooted at the API origin). */
export function githubWebhookUrl(webhookPath: string): string {
  try {
    return new URL(API_BASE).origin + webhookPath;
  } catch {
    return webhookPath;
  }
}
