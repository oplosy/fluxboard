import { apiFetch } from './client';

/** A live task of the page's project named as KEY-N in the body. */
export interface WikiTask {
  number: number;
  title: string;
}

/** A wiki page; `body` and `tasks` are absent in listings. */
export interface WikiPage {
  id: string;
  title: string;
  body?: string;
  /** Send back on save; a stale one is refused with 409. */
  version: number;
  created_by: string;
  updated_by: string;
  created_at: string;
  updated_at: string;
  tasks?: WikiTask[];
}

const base = (orgId: string, projectId: string) => `/orgs/${orgId}/projects/${projectId}/wiki`;

/** List a project's pages by title; `q` keeps titles containing it (FR-PROJ-013). */
export async function listWikiPages(orgId: string, projectId: string, q = ''): Promise<WikiPage[]> {
  const res = await apiFetch<{ pages: WikiPage[] }>(`${base(orgId, projectId)}${q ? `?q=${encodeURIComponent(q)}` : ''}`);
  return res.pages ?? [];
}

/** One page with its body and the tasks it mentions. */
export function getWikiPage(orgId: string, projectId: string, pageId: string): Promise<WikiPage> {
  return apiFetch(`${base(orgId, projectId)}/${pageId}`);
}

/** Create a page (201, CONTRIBUTOR). */
export function createWikiPage(orgId: string, projectId: string, input: { title: string; body?: string }): Promise<WikiPage> {
  return apiFetch(base(orgId, projectId), { method: 'POST', body: input });
}

/** Save a page; 409 when `version` is not the stored one (CONTRIBUTOR). */
export function updateWikiPage(
  orgId: string,
  projectId: string,
  pageId: string,
  input: { title: string; body: string; version: number },
): Promise<WikiPage> {
  return apiFetch(`${base(orgId, projectId)}/${pageId}`, { method: 'PATCH', body: input });
}

/** Delete a page (204, project LEAD or author). */
export function deleteWikiPage(orgId: string, projectId: string, pageId: string): Promise<void> {
  return apiFetch(`${base(orgId, projectId)}/${pageId}`, { method: 'DELETE' });
}

/** Pages that name a task of the project ("Mentioned in"). */
export async function listPagesMentioningTask(orgId: string, projectId: string, taskNumber: number): Promise<WikiPage[]> {
  const res = await apiFetch<{ pages: WikiPage[] }>(`${base(orgId, projectId)}/mentions?task_number=${taskNumber}`);
  return res.pages ?? [];
}
