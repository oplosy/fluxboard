import { describe, expect, it } from 'vitest';

import type { DashboardData } from '@/lib/api/dashboard';
import { coveredProjects, weeklyTotals, workloadTotals } from './dashboard';

const p = (id: string) => ({ project_id: id, key: id, name: id, color: '', open: 1, done: 1, overdue: 0 });
const data: DashboardData = {
  week_from: '2026-08-31',
  weeks: 3,
  projects: [p('a'), p('b')],
  assignees: [
    { project_id: 'a', user_id: 'u1', open: 2, overdue: 1 },
    { project_id: 'b', user_id: 'u1', open: 3, overdue: 0 },
    { project_id: 'b', user_id: null, open: 4, overdue: 0 },
  ],
  weekly: [
    { project_id: 'a', week: '2026-09-07', completed: 2 },
    { project_id: 'b', week: '2026-09-07', completed: 5 },
    { project_id: 'b', week: '2026-09-14', completed: 1 },
  ],
};

describe('dashboard helpers', () => {
  it('covers all projects when none is selected, and ignores ids the caller cannot see', () => {
    expect(coveredProjects(data, [])).toHaveLength(2);
    expect(coveredProjects(data, ['b', 'gone']).map((x) => x.project_id)).toEqual(['b']);
  });

  it('sums completed per week over the covered projects and fills empty weeks', () => {
    expect(weeklyTotals(data, coveredProjects(data, ['a']))).toEqual([
      { week: '2026-08-31', completed: 0 },
      { week: '2026-09-07', completed: 2 },
      { week: '2026-09-14', completed: 0 },
    ]);
    expect(weeklyTotals(data, data.projects).map((r) => r.completed)).toEqual([0, 7, 1]);
  });

  it('sums open work per assignee, busiest first', () => {
    expect(workloadTotals(data, data.projects)).toEqual([
      { userId: null, open: 4, overdue: 0 },
      { userId: 'u1', open: 5, overdue: 1 },
    ].sort((a, b) => b.open - a.open));
    expect(workloadTotals(data, coveredProjects(data, ['a']))).toEqual([{ userId: 'u1', open: 2, overdue: 1 }]);
  });
});
