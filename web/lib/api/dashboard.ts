import { apiFetch } from './client';

/** Widget kinds of the org dashboard (FR-AN-005). */
export type WidgetType = 'project_status' | 'overdue' | 'throughput' | 'workload';

export interface DashboardWidget {
  id: string;
  type: WidgetType;
  /** Empty = every project the caller can see. */
  project_ids: string[];
}

export interface DashboardProject {
  project_id: string;
  key: string;
  name: string;
  color: string;
  open: number;
  done: number;
  overdue: number;
}

export interface DashboardAssignee {
  project_id: string;
  /** null = unassigned. */
  user_id: string | null;
  open: number;
  overdue: number;
}

export interface DashboardWeekly {
  project_id: string;
  /** Monday of the ISO week, YYYY-MM-DD. */
  week: string;
  completed: number;
}

/** GET /orgs/{orgId}/dashboard: per-project rows for the projects the caller can see. */
export interface DashboardData {
  week_from: string;
  weeks: number;
  projects: DashboardProject[];
  assignees: DashboardAssignee[];
  weekly: DashboardWeekly[];
}

export function getDashboard(orgId: string): Promise<DashboardData> {
  return apiFetch(`/orgs/${orgId}/dashboard`);
}

/** The caller's saved widgets (a default set when none is saved). */
export async function getDashboardLayout(orgId: string): Promise<DashboardWidget[]> {
  const res = await apiFetch<{ widgets: DashboardWidget[] }>(`/orgs/${orgId}/dashboard/layout`);
  return res.widgets ?? [];
}

export async function saveDashboardLayout(orgId: string, widgets: DashboardWidget[]): Promise<DashboardWidget[]> {
  const res = await apiFetch<{ widgets: DashboardWidget[] }>(`/orgs/${orgId}/dashboard/layout`, {
    method: 'PUT',
    body: { widgets },
  });
  return res.widgets ?? [];
}
