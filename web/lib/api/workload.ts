import { apiFetch } from './client';

/** One person's load (GET .../projects/{id}/workload, FR-AN-004). */
export interface WorkloadRow {
  /** null = unassigned work. */
  user_id: string | null;
  open_tasks: number;
  open_points: number;
  /** Open tasks whose start-to-due window overlaps the range. */
  scheduled_tasks: number;
  scheduled_points: number;
  scheduled_unestimated: number;
  /** Open tasks due before the range starts. */
  overdue_tasks: number;
  logged_minutes: number;
}

/** Workload for the UTC days from..to (YYYY-MM-DD, inclusive, ≤ 92 days). */
export async function getWorkload(orgId: string, projectId: string, from: string, to: string): Promise<WorkloadRow[]> {
  const qs = new URLSearchParams({ from, to });
  const res = await apiFetch<{ rows: WorkloadRow[] }>(`/orgs/${orgId}/projects/${projectId}/workload?${qs}`);
  return res.rows ?? [];
}
