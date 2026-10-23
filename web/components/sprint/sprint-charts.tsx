'use client';

import {
  Bar,
  BarChart,
  CartesianGrid,
  Legend,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts';

import type { Sprint, SprintBurndown } from '@/lib/api/sprints';
import { burndownRows } from '@/lib/board/burndown';

// Sprint burndown and velocity charts (FR-AN-003). One measured series in a
// validated blue (light #2a78d6 / dark #3987e5) against a neutral gray
// reference series; the ideal line is also dashed so it never relies on color.
const VIZ_VARS =
  '[--viz-1:#2a78d6] dark:[--viz-1:#3987e5] [--viz-ref:#898781] [--viz-grid:#e1e0d9] dark:[--viz-grid:#2c2c2a]';
const AXIS_TICK = { fontSize: 11, fill: 'hsl(var(--muted-foreground))' };
// Legend text stays in ink; only the swatch carries the series color.
const legendText = (value: string) => <span className="text-foreground">{value}</span>;
const TOOLTIP_STYLE = {
  backgroundColor: 'hsl(var(--card))',
  border: '1px solid hsl(var(--border))',
  borderRadius: 8,
  fontSize: 12,
  color: 'hsl(var(--foreground))',
};

function shortDay(day: string): string {
  return new Date(`${day}T00:00:00Z`).toLocaleDateString(undefined, { month: 'short', day: 'numeric', timeZone: 'UTC' });
}

export function BurndownChart({ burndown }: { burndown: SprintBurndown }) {
  const unit = burndown.total_points > 0 ? 'points' : 'tasks';
  const rows = burndownRows(burndown, unit);
  const label = unit === 'points' ? 'Remaining points' : 'Remaining tasks';
  return (
    <figure className={VIZ_VARS}>
      <figcaption className="mb-2 text-xs text-muted-foreground">
        {label} per day (UTC){burndown.end ? '' : ' · no planned end, so no ideal line'}
      </figcaption>
      <div className="h-56 w-full">
        <ResponsiveContainer width="100%" height="100%">
          <LineChart data={rows} margin={{ top: 4, right: 8, bottom: 0, left: 0 }}>
            <CartesianGrid vertical={false} stroke="var(--viz-grid)" />
            <XAxis dataKey="date" tickFormatter={shortDay} tick={AXIS_TICK} tickLine={false} axisLine={false} minTickGap={16} />
            <YAxis allowDecimals={false} tick={AXIS_TICK} tickLine={false} axisLine={false} width={32} />
            <Tooltip contentStyle={TOOLTIP_STYLE} labelFormatter={(d) => shortDay(String(d))} />
            <Legend iconType="plainline" wrapperStyle={{ fontSize: 12 }} formatter={legendText} />
            {burndown.end ? (
              <Line
                type="linear"
                dataKey="ideal"
                name="Ideal"
                stroke="var(--viz-ref)"
                strokeWidth={2}
                strokeDasharray="4 4"
                dot={false}
                activeDot={false}
              />
            ) : null}
            <Line
              type="stepAfter"
              dataKey="remaining"
              name={label}
              stroke="var(--viz-1)"
              strokeWidth={2}
              dot={false}
              activeDot={{ r: 4 }}
              connectNulls={false}
            />
          </LineChart>
        </ResponsiveContainer>
      </div>
      <table className="sr-only">
        <caption>{label} per day</caption>
        <thead>
          <tr>
            <th>Date</th>
            <th>Remaining</th>
            <th>Ideal</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((r) => (
            <tr key={r.date}>
              <td>{r.date}</td>
              <td>{r.remaining ?? ''}</td>
              <td>{r.ideal ?? ''}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </figure>
  );
}

/** Committed vs done per completed sprint, oldest first. */
export function VelocityChart({ sprints }: { sprints: Sprint[] }) {
  const done = sprints.filter((s) => s.status === 'completed');
  if (done.length === 0) return null;
  const unit = done.some((s) => s.completed_points_total > 0) ? 'points' : 'tasks';
  const rows = done.map((s) => ({
    id: s.id,
    name: s.name,
    committed: unit === 'points' ? s.completed_points_total : s.completed_total,
    done: unit === 'points' ? s.completed_points_done : s.completed_done,
  }));
  const avg = Math.round((rows.reduce((sum, r) => sum + r.done, 0) / rows.length) * 10) / 10;
  return (
    <figure className={VIZ_VARS}>
      <figcaption className="mb-2 flex items-baseline justify-between gap-3 text-xs text-muted-foreground">
        <span>Velocity in {unit}, committed vs done per completed sprint</span>
        <span className="shrink-0">
          avg <span className="font-semibold text-foreground">{avg}</span> {unit}
        </span>
      </figcaption>
      <div className="h-48 w-full">
        <ResponsiveContainer width="100%" height="100%">
          <BarChart data={rows} barGap={2} barSize={24} margin={{ top: 4, right: 8, bottom: 0, left: 0 }}>
            <CartesianGrid vertical={false} stroke="var(--viz-grid)" />
            <XAxis dataKey="name" tick={AXIS_TICK} tickLine={false} axisLine={false} minTickGap={8} />
            <YAxis allowDecimals={false} tick={AXIS_TICK} tickLine={false} axisLine={false} width={32} />
            <Tooltip contentStyle={TOOLTIP_STYLE} cursor={{ fill: 'hsl(var(--secondary))' }} />
            <Legend iconType="square" wrapperStyle={{ fontSize: 12 }} formatter={legendText} />
            <Bar dataKey="committed" name="Committed" fill="var(--viz-ref)" radius={[4, 4, 0, 0]} />
            <Bar dataKey="done" name="Done" fill="var(--viz-1)" radius={[4, 4, 0, 0]} />
          </BarChart>
        </ResponsiveContainer>
      </div>
      <table className="sr-only">
        <caption>Velocity in {unit}</caption>
        <thead>
          <tr>
            <th>Sprint</th>
            <th>Committed</th>
            <th>Done</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((r) => (
            <tr key={r.id}>
              <td>{r.name}</td>
              <td>{r.committed}</td>
              <td>{r.done}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </figure>
  );
}
