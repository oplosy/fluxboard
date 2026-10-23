// Date-range presets for the workload view (FR-AN-004). Days are YYYY-MM-DD
// in UTC, both ends inclusive, matching the API.

export type RangePreset = 'this-week' | 'next-2-weeks' | 'this-month';

export interface DayRange {
  from: string;
  to: string;
}

function iso(d: Date): string {
  return d.toISOString().slice(0, 10);
}

function utcDay(now: Date): Date {
  return new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), now.getUTCDate()));
}

function addDays(d: Date, n: number): Date {
  const out = new Date(d);
  out.setUTCDate(out.getUTCDate() + n);
  return out;
}

/** The days a preset covers on the given date; weeks run Monday to Sunday. */
export function presetRange(preset: RangePreset, now: Date): DayRange {
  const today = utcDay(now);
  const monday = addDays(today, -((today.getUTCDay() + 6) % 7));
  if (preset === 'this-week') return { from: iso(monday), to: iso(addDays(monday, 6)) };
  if (preset === 'next-2-weeks') return { from: iso(today), to: iso(addDays(today, 13)) };
  const first = new Date(Date.UTC(today.getUTCFullYear(), today.getUTCMonth(), 1));
  const last = new Date(Date.UTC(today.getUTCFullYear(), today.getUTCMonth() + 1, 0));
  return { from: iso(first), to: iso(last) };
}

/** Number of days in a range, both ends included. */
export function rangeDays(r: DayRange): number {
  return Math.round((Date.parse(r.to) - Date.parse(r.from)) / 86_400_000) + 1;
}

export function formatMinutes(min: number): string {
  if (min <= 0) return '—';
  const h = Math.floor(min / 60);
  const m = min % 60;
  return h ? `${h}h${m ? ` ${m}m` : ''}` : `${m}m`;
}
