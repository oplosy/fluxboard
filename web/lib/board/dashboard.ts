import type { DashboardData, DashboardProject, WidgetType } from '@/lib/api/dashboard';

export const WIDGET_TITLES: Record<WidgetType, string> = {
  project_status: 'Open vs done by project',
  overdue: 'Overdue tasks by project',
  throughput: 'Completed per week',
  workload: 'Open work by assignee',
};

/** Selected ids that the caller can still see; empty selection = all projects. */
export function coveredProjects(data: DashboardData, projectIds: string[]): DashboardProject[] {
  if (projectIds.length === 0) return data.projects;
  const chosen = new Set(projectIds);
  return data.projects.filter((p) => chosen.has(p.project_id));
}

/** Completed tasks per week over the covered projects, oldest first, empty weeks as 0. */
export function weeklyTotals(data: DashboardData, covered: DashboardProject[]): { week: string; completed: number }[] {
  const ids = new Set(covered.map((p) => p.project_id));
  const start = new Date(`${data.week_from}T00:00:00Z`);
  const rows = Array.from({ length: data.weeks }, (_, i) => {
    const d = new Date(start);
    d.setUTCDate(d.getUTCDate() + 7 * i);
    return { week: d.toISOString().slice(0, 10), completed: 0 };
  });
  const byWeek = new Map(rows.map((r) => [r.week, r]));
  for (const w of data.weekly) {
    if (ids.has(w.project_id)) {
      const row = byWeek.get(w.week);
      if (row) row.completed += w.completed;
    }
  }
  return rows;
}

/** Open and overdue tasks per assignee (null = unassigned) over the covered projects, busiest first. */
export function workloadTotals(
  data: DashboardData,
  covered: DashboardProject[],
): { userId: string | null; open: number; overdue: number }[] {
  const ids = new Set(covered.map((p) => p.project_id));
  const byUser = new Map<string | null, { userId: string | null; open: number; overdue: number }>();
  for (const a of data.assignees) {
    if (!ids.has(a.project_id)) continue;
    const row = byUser.get(a.user_id) ?? { userId: a.user_id, open: 0, overdue: 0 };
    row.open += a.open;
    row.overdue += a.overdue;
    byUser.set(a.user_id, row);
  }
  return [...byUser.values()].sort((x, y) => y.open - x.open);
}
