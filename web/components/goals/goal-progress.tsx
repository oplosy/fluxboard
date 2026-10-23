import { cn } from '@/lib/utils';
import type { Goal } from '@/lib/api/goals';

const STATUS: Record<Goal['status'], { label: string; className: string }> = {
  done: { label: 'Done', className: 'bg-success/15 text-success' },
  at_risk: { label: 'At risk', className: 'bg-warning/20 text-foreground' },
  on_track: { label: 'On track', className: 'bg-cobalt/12 text-cobalt' },
};

export function GoalStatusBadge({ status }: { status: Goal['status'] }) {
  const s = STATUS[status];
  return <span className={cn('whitespace-nowrap rounded-full px-2 py-0.5 text-[11px] font-medium', s.className)}>{s.label}</span>;
}

// Progress bar + counts shared by the goals list and the goal page.
export function GoalProgress({ goal }: { goal: Pick<Goal, 'title' | 'total' | 'done' | 'points' | 'done_points'> }) {
  const pct = goal.total ? Math.round((goal.done / goal.total) * 100) : 0;
  return (
    <div className="flex flex-wrap items-center gap-x-3 gap-y-2">
      <div
        className="h-1.5 min-w-32 flex-1 overflow-hidden rounded-full bg-secondary"
        role="progressbar"
        aria-valuenow={pct}
        aria-valuemin={0}
        aria-valuemax={100}
        aria-label={`${goal.title} progress`}
      >
        <div className="h-full rounded-full bg-success transition-[width] duration-500" style={{ width: `${pct}%` }} />
      </div>
      <span className="font-mono text-[11px] text-muted-foreground">
        {goal.done}/{goal.total} done
        {goal.points > 0 ? ` · ${goal.done_points}/${goal.points} pts` : ''}
      </span>
    </div>
  );
}

export function formatDay(day: string): string {
  return new Date(`${day.slice(0, 10)}T00:00:00Z`).toLocaleDateString(undefined, {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
    timeZone: 'UTC',
  });
}
