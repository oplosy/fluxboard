'use client';

import { use, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { AlertTriangle } from 'lucide-react';

import { useOrg } from '@/lib/org/context';
import { useProjectByKey } from '@/lib/board/use-project';
import { memberName, useOrgMembers } from '@/lib/org/use-members';
import { useOrgAgents } from '@/lib/org/use-agents';
import { getWorkload } from '@/lib/api/workload';
import { formatMinutes, presetRange, rangeDays, type DayRange, type RangePreset } from '@/lib/board/date-range';
import { cn } from '@/lib/utils';
import { ProjectFrame } from '@/app/(app)/app/[orgSlug]/projects/[projectKey]/_components/project-frame';
import { Card, CardContent } from '@/components/ui/card';

const PRESETS: { value: RangePreset; label: string }[] = [
  { value: 'this-week', label: 'This week' },
  { value: 'next-2-weeks', label: 'Next 2 weeks' },
  { value: 'this-month', label: 'This month' },
];
const MAX_DAYS = 92;
const dateInput =
  'rounded-md border border-input bg-card px-2 py-1 text-xs outline-none transition-[border-color,box-shadow] duration-300 hover:border-foreground/40 focus-visible:border-foreground focus-visible:shadow-[inset_0_-2px_0_hsl(var(--signal))]';

// Workload (FR-AN-004): per person, open work in the project, the part
// scheduled in the chosen range, overdue tasks and time logged in the range.
export default function ProjectWorkloadPage(props: { params: Promise<{ projectKey: string }> }) {
  const params = use(props.params);
  const { orgId } = useOrg();
  const { byId } = useOrgMembers();
  const { names: agentNames } = useOrgAgents();
  const { project, isLoading, isError } = useProjectByKey(params.projectKey);

  const [preset, setPreset] = useState<RangePreset | null>('this-week');
  const [range, setRange] = useState<DayRange>(() => presetRange('this-week', new Date()));
  const days = rangeDays(range);
  const rangeValid = days >= 1 && days <= MAX_DAYS;

  const workloadQ = useQuery({
    queryKey: ['workload', project?.id, range.from, range.to],
    queryFn: () => getWorkload(orgId, project!.id, range.from, range.to),
    enabled: Boolean(project) && rangeValid,
  });

  if (isLoading) return <p className="px-6 py-8 text-sm text-muted-foreground">Loading workload…</p>;
  if (isError || !project)
    return <p className="px-6 py-8 text-sm text-destructive">Couldn&apos;t load this project.</p>;

  const usePoints = (workloadQ.data ?? []).some((r) => r.open_points > 0);
  const load = (r: { scheduled_points: number; scheduled_tasks: number }) =>
    usePoints ? r.scheduled_points : r.scheduled_tasks;
  const rows = [...(workloadQ.data ?? [])].sort((a, b) => {
    if (!a.user_id !== !b.user_id) return a.user_id ? -1 : 1; // unassigned last
    return load(b) - load(a) || memberName(byId, a.user_id, agentNames).localeCompare(memberName(byId, b.user_id, agentNames));
  });
  const maxLoad = Math.max(1, ...rows.map(load));

  return (
    <ProjectFrame project={project}>
      <p className="mb-4 text-sm text-muted-foreground">
        Who carries what: open work per person, the part scheduled in the range (its start-to-due window
        overlaps it), overdue tasks and time logged on this project in the range.
      </p>

      <div className="mb-4 flex flex-wrap items-center gap-2">
        {PRESETS.map((p) => (
          <button
            key={p.value}
            type="button"
            aria-pressed={preset === p.value}
            onClick={() => {
              setPreset(p.value);
              setRange(presetRange(p.value, new Date()));
            }}
            className={cn(
              'h-8 rounded-md border px-3 text-[13px] transition-colors',
              preset === p.value
                ? 'border-foreground/50 bg-secondary text-foreground'
                : 'border-input text-muted-foreground hover:border-foreground/40 hover:text-foreground',
            )}
          >
            {p.label}
          </button>
        ))}
        <span className="mx-1 hidden h-5 w-px bg-border sm:block" aria-hidden />
        <label className="flex items-center gap-1.5 text-xs text-muted-foreground">
          From
          <input
            type="date"
            value={range.from}
            onChange={(e) => {
              if (!e.target.value) return;
              setPreset(null);
              setRange((r) => ({ ...r, from: e.target.value }));
            }}
            className={dateInput}
          />
        </label>
        <label className="flex items-center gap-1.5 text-xs text-muted-foreground">
          To
          <input
            type="date"
            value={range.to}
            onChange={(e) => {
              if (!e.target.value) return;
              setPreset(null);
              setRange((r) => ({ ...r, to: e.target.value }));
            }}
            className={dateInput}
          />
        </label>
      </div>

      {!rangeValid ? (
        <p className="mb-4 flex items-center gap-2 text-sm text-destructive">
          <AlertTriangle className="h-4 w-4" aria-hidden /> Pick a range of 1 to {MAX_DAYS} days with the start
          before the end.
        </p>
      ) : (
        <Card>
          <CardContent className="overflow-x-auto p-0">
            {workloadQ.isLoading ? (
              <p className="px-4 py-6 text-sm text-muted-foreground">Loading…</p>
            ) : workloadQ.isError ? (
              <p className="px-4 py-6 text-sm text-destructive">Couldn&apos;t load the workload.</p>
            ) : rows.length === 0 ? (
              <p className="px-4 py-6 text-sm text-muted-foreground">No open work and no logged time in this range.</p>
            ) : (
              <table className="w-full min-w-[640px] text-sm">
                <caption className="sr-only">
                  Workload from {range.from} to {range.to}
                </caption>
                <thead>
                  <tr className="border-b text-left text-[11px] uppercase tracking-[0.06em] text-muted-foreground">
                    <th className="px-4 py-2 font-semibold">Person</th>
                    <th className="px-3 py-2 font-semibold">In range</th>
                    <th className="px-3 py-2 text-right font-semibold">Open total</th>
                    <th className="px-3 py-2 text-right font-semibold">Overdue</th>
                    <th className="px-4 py-2 text-right font-semibold">Logged</th>
                  </tr>
                </thead>
                <tbody className="divide-y">
                  {rows.map((r) => {
                    const name = memberName(byId, r.user_id, agentNames);
                    return (
                      <tr key={r.user_id ?? 'unassigned'}>
                        <td className={cn('px-4 py-2.5 font-medium', !r.user_id && 'text-muted-foreground')}>{name}</td>
                        <td className="px-3 py-2.5">
                          <div className="flex items-center gap-2">
                            <div className="h-2 w-28 shrink-0 overflow-hidden rounded-full bg-secondary" aria-hidden>
                              <div
                                className="h-full rounded-full bg-[#2a78d6] dark:bg-[#3987e5]"
                                style={{ width: `${(load(r) / maxLoad) * 100}%` }}
                              />
                            </div>
                            <span className="whitespace-nowrap tabular-nums">
                              {r.scheduled_tasks} {r.scheduled_tasks === 1 ? 'task' : 'tasks'}
                              {usePoints ? ` · ${r.scheduled_points} pts` : ''}
                            </span>
                            {r.scheduled_unestimated > 0 && usePoints ? (
                              <span className="whitespace-nowrap text-xs text-muted-foreground">
                                {r.scheduled_unestimated} unestimated
                              </span>
                            ) : null}
                          </div>
                        </td>
                        <td className="whitespace-nowrap px-3 py-2.5 text-right tabular-nums text-muted-foreground">
                          {r.open_tasks}
                          {usePoints ? ` · ${r.open_points} pts` : ''}
                        </td>
                        <td
                          className={cn(
                            'px-3 py-2.5 text-right tabular-nums',
                            r.overdue_tasks > 0 ? 'font-medium text-destructive' : 'text-muted-foreground',
                          )}
                        >
                          {r.overdue_tasks || '—'}
                        </td>
                        <td className="whitespace-nowrap px-4 py-2.5 text-right tabular-nums">{formatMinutes(r.logged_minutes)}</td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            )}
          </CardContent>
        </Card>
      )}
    </ProjectFrame>
  );
}
