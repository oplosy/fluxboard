import { apiFetch } from './client';

/** One epic with derived progress (GET .../projects/{id}/epics -> { epics }). */
export interface Epic {
  id: string;
  name: string;
  /** "" or #rrggbb. */
  color: string;
  /** Tasks in the epic. */
  total: number;
  /** Of those, tasks in the done column. */
  done: number;
  created_at: string;
}

/** Payload for creating or updating an epic. */
export interface EpicInput {
  name: string;
  color?: string;
}

/** List a project's epics oldest-first. */
export async function listEpics(orgId: string, projectId: string): Promise<Epic[]> {
  const res = await apiFetch<{ epics: Epic[] }>(`/orgs/${orgId}/projects/${projectId}/epics`);
  return res.epics ?? [];
}

/** Create an epic (201, LEAD). */
export function createEpic(orgId: string, projectId: string, input: EpicInput): Promise<Epic> {
  return apiFetch(`/orgs/${orgId}/projects/${projectId}/epics`, { method: 'POST', body: input });
}

/** Rename or recolor an epic (LEAD). */
export function updateEpic(orgId: string, projectId: string, epicId: string, input: EpicInput): Promise<Epic> {
  return apiFetch(`/orgs/${orgId}/projects/${projectId}/epics/${epicId}`, { method: 'PATCH', body: input });
}

/** Delete an epic; its tasks stay (204, LEAD). */
export function deleteEpic(orgId: string, projectId: string, epicId: string): Promise<void> {
  return apiFetch(`/orgs/${orgId}/projects/${projectId}/epics/${epicId}`, { method: 'DELETE' });
}

/** File tasks under an epic, null clears it (204, CONTRIBUTOR). */
export function assignEpic(
  orgId: string,
  projectId: string,
  input: { task_ids: string[]; epic_id: string | null },
): Promise<void> {
  return apiFetch(`/orgs/${orgId}/projects/${projectId}/epics/assign`, { method: 'POST', body: input });
}
