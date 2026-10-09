'use client';

import Link from 'next/link';
import { useSortable } from '@dnd-kit/sortable';
import { CSS } from '@dnd-kit/utilities';
import { CalendarDays, MessageSquare, MoreHorizontal } from 'lucide-react';
import { useState, type MouseEvent as ReactMouseEvent, type SyntheticEvent } from 'react';

import type { Label, Priority, Task, TaskCard } from '@/lib/api/types';
import { PRIORITIES, priorityLabel } from '@/lib/board/priority';
import { attachLabel, detachLabel, getTask, updateTask } from '@/lib/api/tasks';
import { cn } from '@/lib/utils';
import { Avatar } from '@/components/ui/avatar';
import { PriorityBars } from './priority-bars';

// Due-date presentation: overdue (red), due today (amber), upcoming (muted).
function dueState(due: string): { label: string; className: string } {
  const day = new Date(due);
  day.setHours(0, 0, 0, 0);
  const today = new Date();
  today.setHours(0, 0, 0, 0);
  const label = day.toLocaleDateString(undefined, { month: 'short', day: 'numeric' });
  if (day.getTime() < today.getTime()) return { label, className: 'bg-destructive/12 text-destructive' };
  if (day.getTime() === today.getTime()) return { label: 'Today', className: 'bg-warning/25 text-foreground' };
  return { label, className: 'bg-secondary text-muted-foreground' };
}

/** Tiny progress ring for subtasks; the arc animates as the count changes. */
function SubtaskRing({ done, total }: { done: number; total: number }) {
  const r = 5.5;
  const c = 2 * Math.PI * r;
  const pct = total ? done / total : 0;
  return (
    <svg viewBox="0 0 14 14" className="h-3.5 w-3.5 -rotate-90" aria-hidden>
      <circle cx="7" cy="7" r={r} fill="none" className="stroke-foreground/15" strokeWidth="2" />
      <circle
        cx="7"
        cy="7"
        r={r}
        fill="none"
        strokeWidth="2"
        strokeLinecap="round"
        className={cn('transition-[stroke-dashoffset] duration-700 ease-out-expo', pct === 1 ? 'stroke-success' : 'stroke-foreground')}
        strokeDasharray={c}
        strokeDashoffset={c * (1 - pct)}
      />
    </svg>
  );
}

// A single kanban card. Sortable (drag handle = whole card). In select mode a
// checkbox replaces drag interaction and toggles bulk selection.
export function Card({
  task,
  href,
  selectMode,
  selected,
  onToggle,
  assigneeName,
  orgId,
  members,
  labels,
  onMutated,
  index = 0,
}: {
  task: TaskCard;
  href: string;
  selectMode: boolean;
  selected: boolean;
  onToggle: (id: string) => void;
  assigneeName?: string;
  orgId: string;
  members: { user_id: string; name: string }[];
  labels: Label[];
  onMutated: () => void;
  /** Position in the column — staggers the first-paint entrance. */
  index?: number;
}) {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({
    id: task.id,
    disabled: selectMode,
  });

  const style = {
    transform: CSS.Translate.toString(transform),
    transition,
    animationDelay: `${Math.min(index, 10) * 40}ms`,
  };
  const due = task.due_date ? dueState(task.due_date) : null;

  return (
    <div
      ref={setNodeRef}
      style={style}
      className={cn(
        'group relative animate-[card-in_0.5s_var(--ease-out-expo)_backwards] rounded-md border bg-card p-3 text-sm',
        'transition-[box-shadow,border-color,opacity,transform] duration-300 ease-out-expo',
        isDragging
          ? 'border-dashed border-foreground/30 bg-transparent opacity-50 shadow-none [&>*]:invisible'
          : 'border-border shadow-[0_1px_2px_hsl(var(--ink)/0.05)]',
        selected && 'border-signal ring-2 ring-signal/30',
        !isDragging &&
          !selectMode &&
          'cursor-grab hover:-translate-y-0.5 hover:border-foreground/25 hover:shadow-[0_12px_28px_-14px_hsl(var(--ink)/0.35)] active:cursor-grabbing',
      )}
      {...(selectMode ? {} : attributes)}
      {...(selectMode ? {} : listeners)}
    >
      {/* Signal edge grows on hover */}
      <span
        aria-hidden
        className="absolute inset-y-2 left-0 w-[2px] origin-center scale-y-0 rounded-full bg-signal transition-transform duration-300 ease-out-expo group-hover:scale-y-100"
      />
      <div className="flex items-start gap-2">
        {selectMode ? (
          <input
            type="checkbox"
            checked={selected}
            onChange={() => onToggle(task.id)}
            className="mt-0.5 h-4 w-4 rounded border-input"
            aria-label={`Select task ${task.title}`}
          />
        ) : null}
        <div className="min-w-0 flex-1">
          <div className="mb-1.5 flex items-center gap-2">
            <span className="font-mono text-[10.5px] text-muted-foreground">#{task.number}</span>
            <PriorityBars priority={task.priority} />
            <span className="ml-auto">
              {!selectMode ? (
                <QuickMenu task={task} orgId={orgId} members={members} labels={labels} onMutated={onMutated} />
              ) : null}
            </span>
          </div>
          {selectMode ? (
            <p className="line-clamp-3 font-medium leading-snug">{task.title}</p>
          ) : (
            <Link
              href={href}
              onClick={(e) => e.stopPropagation()}
              className="line-clamp-3 font-medium leading-snug decoration-signal decoration-2 underline-offset-[3px] hover:underline"
            >
              {task.title}
            </Link>
          )}
          {task.labels.length > 0 ? (
            <div className="mt-2.5 flex flex-wrap gap-1">
              {task.labels.slice(0, 3).map((l) => (
                <span
                  key={l.id}
                  className="inline-flex items-center gap-1 rounded-sm border border-border px-1.5 py-[1px] text-[10px] font-medium text-foreground/80"
                  title={l.name}
                >
                  <span className="h-1.5 w-1.5 shrink-0 rounded-full" style={{ backgroundColor: l.color }} />
                  <span className="max-w-24 truncate">{l.name}</span>
                </span>
              ))}
              {task.labels.length > 3 ? (
                <span className="rounded-sm px-1 py-[1px] font-mono text-[10px] text-muted-foreground">+{task.labels.length - 3}</span>
              ) : null}
            </div>
          ) : null}
          {due || task.comment_count > 0 || task.subtask_total > 0 || task.assignee_id ? (
            <div className="mt-3 flex items-center gap-2.5 text-[11px] text-muted-foreground">
              {due ? (
                <span
                  className={cn('inline-flex items-center gap-1 rounded-sm px-1.5 py-0.5 font-mono text-[10px] font-medium', due.className)}
                  title={`Due ${new Date(task.due_date as string).toLocaleDateString()}`}
                >
                  <CalendarDays className="h-3 w-3" />
                  {due.label}
                </span>
              ) : null}
              {task.comment_count > 0 ? (
                <span className="inline-flex items-center gap-1 font-mono" title={`${task.comment_count} comments`}>
                  <MessageSquare className="h-3 w-3" />
                  {task.comment_count}
                </span>
              ) : null}
              {task.subtask_total > 0 ? (
                <span className="inline-flex items-center gap-1 font-mono" title={`${task.subtask_done}/${task.subtask_total} subtasks done`}>
                  <SubtaskRing done={task.subtask_done} total={task.subtask_total} />
                  {task.subtask_done}/{task.subtask_total}
                </span>
              ) : null}
              {task.assignee_id ? <Avatar name={assigneeName ?? task.assignee_id} size="sm" className="ml-auto" /> : null}
            </div>
          ) : null}
        </div>
      </div>
    </div>
  );
}

// Hover quick menu (Jira-style): change priority, assignee, or labels without
// opening the detail page. Loads the full task once for the PATCH base, then
// applies in place and refreshes the board.
function QuickMenu({
  task,
  orgId,
  members,
  labels,
  onMutated,
}: {
  task: TaskCard;
  orgId: string;
  members: { user_id: string; name: string }[];
  labels: Label[];
  onMutated: () => void;
}) {
  const [open, setOpen] = useState(false);
  const [full, setFull] = useState<Task | null>(null);
  const [busy, setBusy] = useState(false);

  async function ensureFull(): Promise<Task | null> {
    if (full) return full;
    try {
      const t = await getTask(orgId, task.id);
      setFull(t);
      return t;
    } catch {
      return null;
    }
  }

  function openMenu(e: ReactMouseEvent) {
    e.stopPropagation();
    setOpen((v) => !v);
    if (!open) void ensureFull();
  }

  async function patch(p: {
    priority?: Priority;
    assignee_id?: string | null;
    start_date?: string | null;
    due_date?: string | null;
  }) {
    const base = await ensureFull();
    if (!base) return;
    setBusy(true);
    try {
      const next = await updateTask(orgId, task.id, {
        title: base.title,
        description: base.description,
        assignee_id: p.assignee_id !== undefined ? p.assignee_id : (base.assignee_id ?? null),
        priority: (p.priority ?? base.priority) as Priority,
        start_date: p.start_date !== undefined ? p.start_date : (base.start_date ?? null),
        due_date: p.due_date !== undefined ? p.due_date : (base.due_date ?? null),
      });
      setFull(next);
      onMutated();
    } finally {
      setBusy(false);
    }
  }

  async function toggleLabel(labelId: string, attached: boolean) {
    setBusy(true);
    try {
      if (attached) await detachLabel(orgId, task.id, labelId);
      else await attachLabel(orgId, task.id, labelId);
      onMutated();
    } finally {
      setBusy(false);
    }
  }

  const attachedIds = new Set(task.labels.map((l) => l.id));
  const stop = (e: SyntheticEvent) => e.stopPropagation();

  return (
    <div className="relative shrink-0" onPointerDown={stop} onClick={stop}>
      <button
        type="button"
        onClick={openMenu}
        aria-label="Quick actions"
        title="Quick actions"
        className={cn(
          'flex h-5 w-5 items-center justify-center rounded-sm text-muted-foreground transition-[opacity,background-color,color] hover:bg-secondary hover:text-foreground focus-visible:opacity-100 group-hover:opacity-100',
          open ? 'bg-secondary text-foreground opacity-100' : 'opacity-0',
        )}
      >
        <MoreHorizontal className="h-3.5 w-3.5" />
      </button>
      {open ? (
        <>
          <div className="fixed inset-0 z-30" onClick={() => setOpen(false)} aria-hidden />
          <div className="glass absolute right-0 top-full z-40 mt-1 w-56 origin-top-right animate-scale-in space-y-2 rounded-lg p-2.5">
            <label className="block px-1 font-mono text-[10px] font-medium uppercase tracking-[0.14em] text-muted-foreground">
              Priority
            </label>
            <div className="flex flex-wrap gap-1 px-1">
              {PRIORITIES.filter((p) => p !== 'none').map((p) => (
                <button
                  key={p}
                  type="button"
                  disabled={busy}
                  onClick={() => void patch({ priority: p })}
                  className={cn(
                    'inline-flex items-center gap-1.5 rounded-sm px-1.5 py-1 text-[10px] font-medium transition-[background-color,color,transform] duration-200 active:scale-95',
                    task.priority === p
                      ? 'bg-foreground text-background'
                      : 'bg-secondary text-secondary-foreground hover:bg-foreground/10',
                  )}
                >
                  {priorityLabel(p)}
                </button>
              ))}
            </div>
            <label className="block px-1 font-mono text-[10px] font-medium uppercase tracking-[0.14em] text-muted-foreground">
              Assignee
            </label>
            <select
              value={task.assignee_id ?? ''}
              disabled={busy}
              onChange={(e) => void patch({ assignee_id: e.target.value || null })}
              className="w-full rounded-lg border border-input bg-card px-2 py-1.5 text-xs outline-none focus-visible:border-foreground focus-visible:shadow-[inset_0_-2px_0_hsl(var(--signal))] transition-[border-color,box-shadow] duration-300 hover:border-foreground/40"
            >
              <option value="">Unassigned</option>
              {members.map((m) => (
                <option key={m.user_id} value={m.user_id}>
                  {m.name}
                </option>
              ))}
            </select>
            <div className="grid grid-cols-2 gap-1 px-1">
              <label className="block font-mono text-[10px] font-medium uppercase tracking-[0.14em] text-muted-foreground">
                Start
                <input
                  type="date"
                  disabled={busy}
                  defaultValue={task.start_date ? task.start_date.slice(0, 10) : ''}
                  onChange={(e) =>
                    void patch({ start_date: e.target.value ? new Date(e.target.value).toISOString() : null })
                  }
                  className="mt-0.5 w-full rounded-lg border border-input bg-card px-1.5 py-1 text-[11px] font-normal normal-case outline-none focus-visible:border-foreground focus-visible:shadow-[inset_0_-2px_0_hsl(var(--signal))] transition-[border-color,box-shadow] duration-300 hover:border-foreground/40"
                />
              </label>
              <label className="block font-mono text-[10px] font-medium uppercase tracking-[0.14em] text-muted-foreground">
                Due
                <input
                  type="date"
                  disabled={busy}
                  defaultValue={task.due_date ? task.due_date.slice(0, 10) : ''}
                  onChange={(e) =>
                    void patch({ due_date: e.target.value ? new Date(e.target.value).toISOString() : null })
                  }
                  className="mt-0.5 w-full rounded-lg border border-input bg-card px-1.5 py-1 text-[11px] font-normal normal-case outline-none focus-visible:border-foreground focus-visible:shadow-[inset_0_-2px_0_hsl(var(--signal))] transition-[border-color,box-shadow] duration-300 hover:border-foreground/40"
                />
              </label>
            </div>
            {labels.length > 0 ? (
              <>
                <span className="block px-1 font-mono text-[10px] font-medium uppercase tracking-[0.14em] text-muted-foreground">
                  Labels
                </span>
                <div className="max-h-28 space-y-0.5 overflow-y-auto px-1">
                  {labels.map((l) => (
                    <label
                      key={l.id}
                      className="flex cursor-pointer items-center gap-2 rounded-md px-1 py-1 text-xs hover:bg-secondary"
                    >
                      <input
                        type="checkbox"
                        checked={attachedIds.has(l.id)}
                        disabled={busy}
                        onChange={() => void toggleLabel(l.id, attachedIds.has(l.id))}
                        className="h-3.5 w-3.5 rounded border-input"
                      />
                      <span className="h-2 w-2 shrink-0 rounded-full" style={{ backgroundColor: l.color }} />
                      <span className="truncate">{l.name}</span>
                    </label>
                  ))}
                </div>
              </>
            ) : null}
          </div>
        </>
      ) : null}
    </div>
  );
}
