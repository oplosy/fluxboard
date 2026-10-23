import { apiFetch } from './client';

/** One release with derived progress (GET .../projects/{id}/releases -> { releases }). */
export interface Release {
  id: string;
  name: string;
  description: string;
  /** Planned day, YYYY-MM-DD. */
  release_date: string | null;
  /** Set once marked released. */
  released_at: string | null;
  /** Tasks in the release. */
  total: number;
  /** Of those, tasks in the done column. */
  done: number;
  /** Summed estimates of the release's tasks. */
  points: number;
  /** Of those, points in the done column. */
  done_points: number;
  created_at: string;
}

/** Payload for creating or updating a release. */
export interface ReleaseInput {
  name: string;
  description?: string;
  release_date?: string | null;
  /** Update only: true marks it released, false reopens it. */
  released?: boolean;
}

/** List a project's releases oldest-first (FR-PROJ-010). */
export async function listReleases(orgId: string, projectId: string): Promise<Release[]> {
  const res = await apiFetch<{ releases: Release[] }>(`/orgs/${orgId}/projects/${projectId}/releases`);
  return res.releases ?? [];
}

/** Create an unreleased release (201, LEAD). */
export function createRelease(orgId: string, projectId: string, input: ReleaseInput): Promise<Release> {
  return apiFetch(`/orgs/${orgId}/projects/${projectId}/releases`, { method: 'POST', body: input });
}

/** Edit a release, or mark it released / reopen it (LEAD). */
export function updateRelease(orgId: string, projectId: string, releaseId: string, input: ReleaseInput): Promise<Release> {
  return apiFetch(`/orgs/${orgId}/projects/${projectId}/releases/${releaseId}`, { method: 'PATCH', body: input });
}

/** Delete a release; its tasks stay (204, LEAD). */
export function deleteRelease(orgId: string, projectId: string, releaseId: string): Promise<void> {
  return apiFetch(`/orgs/${orgId}/projects/${projectId}/releases/${releaseId}`, { method: 'DELETE' });
}

/** File tasks under a release, null clears it (204, CONTRIBUTOR). */
export function assignRelease(
  orgId: string,
  projectId: string,
  input: { task_ids: string[]; release_id: string | null },
): Promise<void> {
  return apiFetch(`/orgs/${orgId}/projects/${projectId}/releases/assign`, { method: 'POST', body: input });
}
