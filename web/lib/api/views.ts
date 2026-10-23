import { apiFetch } from './client';

/** Board filters and grouping as a saved view stores them (FR-PROJ-011). */
export interface ViewFilters {
  text: string;
  priority: string;
  assignee: 'any' | 'me' | 'unassigned';
  sprint: string;
  epic: string;
  group_by: 'none' | 'assignee' | 'epic' | 'priority';
}

/** One saved view (GET .../projects/{id}/views -> { views }). */
export interface SavedView {
  id: string;
  name: string;
  /** Visible to the whole project; managed by LEADs. */
  shared: boolean;
  /** Created by the caller. */
  mine: boolean;
  filters: ViewFilters;
  created_at: string;
}

export interface SavedViewInput {
  name: string;
  shared: boolean;
  filters: ViewFilters;
}

/** The project's shared views and the caller's personal ones, shared first. */
export async function listViews(orgId: string, projectId: string): Promise<SavedView[]> {
  const res = await apiFetch<{ views: SavedView[] }>(`/orgs/${orgId}/projects/${projectId}/views`);
  return res.views ?? [];
}

/** Save a view (201): personal for anyone, shared for LEADs. */
export function createView(orgId: string, projectId: string, input: SavedViewInput): Promise<SavedView> {
  return apiFetch(`/orgs/${orgId}/projects/${projectId}/views`, { method: 'POST', body: input });
}

/** Rename a view, replace its filters or change its sharing. */
export function updateView(orgId: string, projectId: string, viewId: string, input: SavedViewInput): Promise<SavedView> {
  return apiFetch(`/orgs/${orgId}/projects/${projectId}/views/${viewId}`, { method: 'PATCH', body: input });
}

/** Delete a view (204). */
export function deleteView(orgId: string, projectId: string, viewId: string): Promise<void> {
  return apiFetch(`/orgs/${orgId}/projects/${projectId}/views/${viewId}`, { method: 'DELETE' });
}
