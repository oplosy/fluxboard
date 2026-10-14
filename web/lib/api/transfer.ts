import { apiFetch } from './client';

// Project CSV import (preview → idempotent commit) and export (FR-TASK-012/013).

export const IMPORT_FIELDS = [
  { field: 'title', label: 'Title' },
  { field: 'description', label: 'Description' },
  { field: 'status', label: 'Status / column' },
  { field: 'assignee', label: 'Assignee email' },
  { field: 'due_date', label: 'Due date' },
  { field: 'priority', label: 'Priority' },
  { field: 'labels', label: 'Labels' },
] as const;

export type ImportField = (typeof IMPORT_FIELDS)[number]['field'];
export type ImportMapping = Partial<Record<ImportField, string>>;

export interface ImportRow {
  line: number;
  title: string;
  description?: string;
  column_id: string;
  column_name: string;
  assignee_id?: string;
  assignee_email?: string;
  due_date?: string;
  priority: string;
  labels: string[];
  errors: string[];
}

export interface ImportPreview {
  headers: string[];
  mapping: ImportMapping;
  rows: ImportRow[];
  valid: number;
  invalid: number;
}

export interface ImportResult {
  created: number;
  existing: number;
  tasks: { id: string; number: number; title: string }[];
}

/** Parse + validate without writing. Omit mapping to get a suggested one. */
export function previewImport(
  orgId: string,
  projectId: string,
  csv: string,
  mapping?: ImportMapping,
) {
  return apiFetch<ImportPreview>(`/orgs/${orgId}/projects/${projectId}/import/preview`, {
    method: 'POST',
    body: { csv, mapping },
  });
}

/** Create the tasks. Reuse idempotencyKey on retry: rows are never duplicated. */
export function importTasks(
  orgId: string,
  projectId: string,
  csv: string,
  mapping: ImportMapping,
  idempotencyKey: string,
) {
  return apiFetch<ImportResult>(`/orgs/${orgId}/projects/${projectId}/import`, {
    method: 'POST',
    body: { csv, mapping },
    headers: { 'Idempotency-Key': idempotencyKey },
  });
}

/** Download a project export as a file (bearer auth, so not a plain link). */
export async function downloadExport(
  orgId: string,
  projectId: string,
  projectKey: string,
  format: 'csv' | 'json',
) {
  const data = await apiFetch<unknown>(
    `/orgs/${orgId}/projects/${projectId}/export?format=${format}`,
  );
  // Response.text() drops the server's BOM; put it back so Excel reads UTF-8.
  const text =
    typeof data === 'string' ? String.fromCharCode(0xfeff) + data : JSON.stringify(data, null, 2);
  const blob = new Blob([text], {
    type: format === 'csv' ? 'text/csv;charset=utf-8' : 'application/json',
  });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = `${projectKey}-tasks-${new Date().toISOString().slice(0, 10)}.${format}`;
  a.click();
  URL.revokeObjectURL(url);
}
