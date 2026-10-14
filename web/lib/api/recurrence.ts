import { apiFetch } from './client';

export type RecurrenceFreq = 'daily' | 'weekly' | 'monthly';
export type RecurrenceMode = 'schedule' | 'after_completion';

/** A task's repeat rule (GET/PUT .../tasks/{id}/recurrence). */
export interface Recurrence {
  id: string;
  freq: RecurrenceFreq;
  weekdays: number[]; // 0 = Sunday … 6
  month_day: number | null; // monthly; clamped to the month's end
  timezone: string; // IANA
  mode: RecurrenceMode;
  paused: boolean;
  last_on: string; // YYYY-MM-DD
}

export type RecurrenceInput = Omit<Recurrence, 'id' | 'last_on' | 'month_day'> & { month_day: number };

/** The task's rule, or null when it does not repeat. */
export function getRecurrence(orgId: string, taskId: string): Promise<Recurrence | null> {
  return apiFetch<Recurrence | null>(`/orgs/${orgId}/tasks/${taskId}/recurrence`);
}

/** Create, edit, pause or resume the rule (CONTRIBUTOR). Edits affect future occurrences only. */
export function setRecurrence(orgId: string, taskId: string, input: RecurrenceInput): Promise<Recurrence> {
  return apiFetch<Recurrence>(`/orgs/${orgId}/tasks/${taskId}/recurrence`, { method: 'PUT', body: input });
}

/** Stop repeating (204). Occurrences already created stay. */
export function stopRecurrence(orgId: string, taskId: string): Promise<void> {
  return apiFetch(`/orgs/${orgId}/tasks/${taskId}/recurrence`, { method: 'DELETE' });
}
