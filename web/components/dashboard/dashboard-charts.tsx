'use client';

import { Bar, BarChart, CartesianGrid, Legend, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts';

// Org dashboard charts (FR-AN-005). Same validated palette as the sprint charts:
// categorical slot 1 blue, slot 2 orange. Two series (open vs done) are stacked
// with a surface gap and a legend; single-series charts need no legend (the card
// title names the measure). Every chart has a screen-reader table of its numbers.
const VIZ_VARS =
  '[--viz-1:#2a78d6] dark:[--viz-1:#3987e5] [--viz-2:#eb6834] dark:[--viz-2:#d95926] [--viz-grid:#e1e0d9] dark:[--viz-grid:#2c2c2a] [--viz-surface:#fcfcfb] dark:[--viz-surface:#1a1a19]';
const AXIS_TICK = { fontSize: 11, fill: 'hsl(var(--muted-foreground))' };
const legendText = (value: string) => <span className="text-foreground">{value}</span>;
const TOOLTIP_STYLE = {
  backgroundColor: 'hsl(var(--card))',
  border: '1px solid hsl(var(--border))',
  borderRadius: 8,
  fontSize: 12,
  color: 'hsl(var(--foreground))',
};
const CURSOR = { fill: 'hsl(var(--secondary))' };

function shortDay(day: string): string {
  return new Date(`${day}T00:00:00Z`).toLocaleDateString(undefined, { month: 'short', day: 'numeric', timeZone: 'UTC' });
}

function Empty() {
  return <p className="py-8 text-center text-sm text-muted-foreground">No data for these projects yet.</p>;
}

interface Row {
  name: string;
  [series: string]: string | number;
}

function SrTable({ caption, head, rows, keys }: { caption: string; head: string[]; rows: Row[]; keys: string[] }) {
  return (
    <table className="sr-only">
      <caption>{caption}</caption>
      <thead>
        <tr>
          {head.map((h) => (
            <th key={h}>{h}</th>
          ))}
        </tr>
      </thead>
      <tbody>
        {rows.map((r) => (
          <tr key={r.name}>
            <td>{r.name}</td>
            {keys.map((k) => (
              <td key={k}>{r[k]}</td>
            ))}
          </tr>
        ))}
      </tbody>
    </table>
  );
}

/** Horizontal bars, one row per category; `series` lists 1 or 2 measures (2 = stacked). */
function HBars({
  rows,
  series,
  caption,
}: {
  rows: Row[];
  series: { key: string; name: string; color: string }[];
  caption: string;
}) {
  if (rows.length === 0) return <Empty />;
  const stacked = series.length > 1;
  const height = Math.max(120, rows.length * 32 + (stacked ? 40 : 8));
  return (
    <figure className={VIZ_VARS}>
      <div className="w-full" style={{ height }}>
        <ResponsiveContainer width="100%" height="100%">
          <BarChart data={rows} layout="vertical" barSize={16} margin={{ top: 4, right: 8, bottom: 0, left: 0 }}>
            <CartesianGrid horizontal={false} stroke="var(--viz-grid)" />
            <XAxis type="number" allowDecimals={false} tick={AXIS_TICK} tickLine={false} axisLine={false} />
            <YAxis type="category" dataKey="name" tick={AXIS_TICK} tickLine={false} axisLine={false} width={96} interval={0} />
            <Tooltip contentStyle={TOOLTIP_STYLE} cursor={CURSOR} />
            {stacked ? <Legend iconType="square" wrapperStyle={{ fontSize: 12 }} formatter={legendText} /> : null}
            {series.map((s, i) => (
              <Bar
                key={s.key}
                dataKey={s.key}
                name={s.name}
                fill={s.color}
                stackId={stacked ? 'a' : undefined}
                stroke={stacked ? 'var(--viz-surface)' : undefined}
                strokeWidth={stacked ? 2 : 0}
                radius={i === series.length - 1 ? [0, 4, 4, 0] : 0}
              />
            ))}
          </BarChart>
        </ResponsiveContainer>
      </div>
      <SrTable caption={caption} head={['Name', ...series.map((s) => s.name)]} rows={rows} keys={series.map((s) => s.key)} />
    </figure>
  );
}

export function StatusChart({ rows }: { rows: { name: string; open: number; done: number }[] }) {
  return (
    <HBars
      rows={rows}
      caption="Open and done tasks by project"
      series={[
        { key: 'done', name: 'Done', color: 'var(--viz-1)' },
        { key: 'open', name: 'Open', color: 'var(--viz-2)' },
      ]}
    />
  );
}

export function OverdueChart({ rows }: { rows: { name: string; overdue: number }[] }) {
  return <HBars rows={rows} caption="Overdue tasks by project" series={[{ key: 'overdue', name: 'Overdue', color: 'var(--viz-1)' }]} />;
}

export function WorkloadChart({ rows }: { rows: { name: string; open: number }[] }) {
  return <HBars rows={rows} caption="Open tasks by assignee" series={[{ key: 'open', name: 'Open tasks', color: 'var(--viz-1)' }]} />;
}

export function ThroughputChart({ rows }: { rows: { week: string; completed: number }[] }) {
  const data: Row[] = rows.map((r) => ({ name: r.week, completed: r.completed }));
  return (
    <figure className={VIZ_VARS}>
      <div className="h-48 w-full">
        <ResponsiveContainer width="100%" height="100%">
          <BarChart data={data} barSize={24} margin={{ top: 4, right: 8, bottom: 0, left: 0 }}>
            <CartesianGrid vertical={false} stroke="var(--viz-grid)" />
            <XAxis dataKey="name" tickFormatter={shortDay} tick={AXIS_TICK} tickLine={false} axisLine={false} minTickGap={8} />
            <YAxis allowDecimals={false} tick={AXIS_TICK} tickLine={false} axisLine={false} width={32} />
            <Tooltip contentStyle={TOOLTIP_STYLE} cursor={CURSOR} labelFormatter={(d) => `Week of ${shortDay(String(d))}`} />
            <Bar dataKey="completed" name="Completed" fill="var(--viz-1)" radius={[4, 4, 0, 0]} />
          </BarChart>
        </ResponsiveContainer>
      </div>
      <SrTable caption="Completed tasks per week (weeks start Monday)" head={['Week of', 'Completed']} rows={data} keys={['completed']} />
    </figure>
  );
}
