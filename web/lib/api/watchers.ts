import { apiFetch } from './client';

/** A user watching a task (FR-TASK-014). */
export interface TaskWatcher {
  user_id: string;
  name: string;
  email: string;
  created_at: string;
}

/** List a task's watchers, oldest first. */
export async function listTaskWatchers(orgId: string, taskId: string): Promise<TaskWatcher[]> {
  const res = await apiFetch<{ watchers: TaskWatcher[] }>(`/orgs/${orgId}/tasks/${taskId}/watchers`);
  return res.watchers ?? [];
}

/** Watch the task as the caller (204, idempotent). */
export function watchTask(orgId: string, taskId: string): Promise<void> {
  return apiFetch(`/orgs/${orgId}/tasks/${taskId}/watchers/me`, { method: 'PUT' });
}

/** Stop watching the task as the caller (204, idempotent). */
export function unwatchTask(orgId: string, taskId: string): Promise<void> {
  return apiFetch(`/orgs/${orgId}/tasks/${taskId}/watchers/me`, { method: 'DELETE' });
}
