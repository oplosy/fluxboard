import { describe, expect, it } from 'vitest';

import type { SprintBurndown } from '@/lib/api/sprints';
import { burndownRows } from './burndown';

const base: SprintBurndown = {
  start: '2026-10-05',
  end: '2026-10-09',
  total_points: 8,
  total_tasks: 2,
  days: [
    { date: '2026-10-05', remaining_points: 8, remaining_tasks: 2 },
    { date: '2026-10-06', remaining_points: 3, remaining_tasks: 1 },
  ],
};

describe('burndownRows', () => {
  it('extends to the planned end with an ideal line from total to 0', () => {
    const rows = burndownRows(base, 'points');
    expect(rows.map((r) => r.date)).toEqual(['2026-10-05', '2026-10-06', '2026-10-07', '2026-10-08', '2026-10-09']);
    expect(rows.map((r) => r.ideal)).toEqual([8, 6, 4, 2, 0]);
    expect(rows.map((r) => r.remaining)).toEqual([8, 3, undefined, undefined, undefined]);
  });

  it('uses task counts for the tasks unit', () => {
    expect(burndownRows(base, 'tasks').map((r) => r.remaining).slice(0, 2)).toEqual([2, 1]);
  });

  it('runs past the end when overdue, and has no ideal line without an end', () => {
    const overdue = burndownRows({ ...base, end: '2026-10-05' }, 'points');
    expect(overdue.map((r) => r.ideal)).toEqual([0, undefined]);
    expect(burndownRows({ ...base, end: null }, 'points').every((r) => r.ideal === undefined)).toBe(true);
  });
});
