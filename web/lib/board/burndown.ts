import type { SprintBurndown } from '@/lib/api/sprints';

// Chart rows for a sprint burndown (FR-AN-003): the measured remaining work
// per day plus a straight ideal line from the total at start to 0 at the
// planned end. Dates are YYYY-MM-DD in UTC, as the API sends them.

function addDays(day: string, n: number): string {
  const d = new Date(`${day}T00:00:00Z`);
  d.setUTCDate(d.getUTCDate() + n);
  return d.toISOString().slice(0, 10);
}

export interface BurndownRow {
  date: string;
  remaining?: number;
  ideal?: number;
}

/** Rows from the sprint start through the later of the last day and the planned end. */
export function burndownRows(b: SprintBurndown, unit: 'points' | 'tasks'): BurndownRow[] {
  const total = unit === 'points' ? b.total_points : b.total_tasks;
  const actual = new Map(b.days.map((d) => [d.date, unit === 'points' ? d.remaining_points : d.remaining_tasks]));
  const lastActual = b.days.at(-1)?.date ?? b.start;
  const last = b.end && b.end > lastActual ? b.end : lastActual;
  const span = b.end && b.end > b.start ? Math.round((Date.parse(b.end) - Date.parse(b.start)) / 86_400_000) : 0;
  const rows: BurndownRow[] = [];
  for (let i = 0, day = b.start; day <= last && i < 366; i++, day = addDays(day, 1)) {
    const row: BurndownRow = { date: day, remaining: actual.get(day) };
    if (b.end && day <= b.end) row.ideal = span > 0 ? Math.round(total * (1 - i / span) * 10) / 10 : 0;
    rows.push(row);
  }
  return rows;
}
