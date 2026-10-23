'use client';

import { useMemo, useState } from 'react';
import { useDroppable } from '@dnd-kit/core';
import { SortableContext, verticalListSortingStrategy } from '@dnd-kit/sortable';
import { Plus } from 'lucide-react';

import type { BoardColumn, Label, Priority, TaskCard } from '@/lib/api/types';
import type { Epic } from '@/lib/api/epics';
import { PRIORITIES, priorityLabel } from '@/lib/board/priority';
import type { StatusTone } from '@/lib/board/status';
import { cn } from '@/lib/utils';
import { Card } from './card';
import { Select } from '@/components/ui/select';
import { Textarea } from '@/components/ui/textarea';

export interface QuickTaskInput {
  title: string;
  priority: Priority;
  assignee_id: string | null;
  start_date: string | null;
  due_date: string | null;
}

// Cards drawn per column before "Show more": a 3,000-task board drew ~118k DOM
// nodes and took seconds to paint; counts and points still cover every task.
export const CARD_PAGE = 50;

export interface AssigneeOption {
  user_id: string;
  name: string;
}

// One board column: a droppable region wrapping a sortable list of cards, a WIP
// indicator, and an inline "add task" composer.
export function Column({
  column,
  tasks,
  projectKey,
  cardHref,
  selectMode,
  selectedIds,
  onToggleSelect,
  onAddTask,
  addPending,
  members,
  orgId,
  labels,
  epics,
  onMutated,
  composeSignal,
  flash = false,
  tone,
  dropId,
  inLane = false,
}: {
  column: BoardColumn;
  tasks: TaskCard[];
  projectKey: string;
  cardHref: (task: TaskCard) => string;
  selectMode: boolean;
  selectedIds: Set<string>;
  onToggleSelect: (id: string) => void;
  onAddTask: (columnId: string, input: QuickTaskInput) => void;
  addPending: boolean;
  members: AssigneeOption[];
  orgId: string;
  labels: Label[];
  epics: Epic[];
  onMutated: () => void;
  composeSignal?: number;
  /** Briefly pulses the column (e.g. a card was just shipped into it). */
  flash?: boolean;
  /** Status colour of this lane (lib/board/status). */
  tone: StatusTone;
  /** Droppable id; defaults to the column id (swimlanes pass a lane-scoped one). */
  dropId?: string;
  /** Inside a swimlane: no header or composer, the board draws one header row. */
  inLane?: boolean;
}) {
  const { setNodeRef, isOver } = useDroppable({ id: dropId ?? column.id });
  const [composingManual, setComposing] = useState(false);
  // External trigger (board "c" shortcut): each new signal value opens the
  // composer until it is closed.
  const [closedSignal, setClosedSignal] = useState(0);
  const composing = composingManual || (!!composeSignal && composeSignal !== closedSignal);
  const [title, setTitle] = useState('');
  const [priority, setPriority] = useState<Priority>('none');
  const [assigneeId, setAssigneeId] = useState('');
  const [startDate, setStartDate] = useState('');
  const [dueDate, setDueDate] = useState('');

  const overLimit = column.wip_limit != null && tasks.length >= column.wip_limit;


  function resetComposer() {
    setTitle('');
    setPriority('none');
    setAssigneeId('');
    setStartDate('');
    setDueDate('');
    setComposing(false);
    setClosedSignal(composeSignal ?? 0);
  }

  function submit() {
    const t = title.trim();
    if (!t) return;
    onAddTask(column.id, {
      title: t,
      priority,
      assignee_id: assigneeId || null,
      start_date: startDate ? new Date(startDate).toISOString() : null,
      due_date: dueDate ? new Date(dueDate).toISOString() : null,
    });
    resetComposer();
  }

  const fill = column.wip_limit ? Math.min(1, tasks.length / column.wip_limit) : 0;
  const points = tasks.reduce((sum, t) => sum + (t.estimate ?? 0), 0);
  const [limit, setLimit] = useState(CARD_PAGE);
  const shown = tasks.length > limit ? tasks.slice(0, limit) : tasks;
  const hidden = tasks.length - shown.length;
  const memberNames = useMemo(() => new Map(members.map((m) => [m.user_id, m.name])), [members]);
  const epicsById = useMemo(() => new Map(epics.map((e) => [e.id, e])), [epics]);

  return (
    <div
      className={cn(
        'flex w-[272px] shrink-0 flex-col rounded-lg bg-secondary/45 p-1.5 transition-[background-color,box-shadow] duration-300',
        isOver && 'bg-signal/[0.05] ring-1 ring-inset ring-signal/40',
        flash && 'animate-[column-flash_0.9s_var(--ease-out-expo)]',
      )}
    >
      {inLane ? null : (
        <div className="px-2 pb-2 pt-1.5">
          <div className="flex h-6 items-center gap-2">
            <span className={cn('h-2 w-2 shrink-0 rounded-full', tone.dot)} aria-hidden />
            <h3 className="truncate font-sans text-[11px] font-semibold uppercase tracking-[0.06em] text-muted-foreground">
              {column.name}
            </h3>
            <span
              key={tasks.length}
              className={cn(
                'animate-scale-in rounded-full px-1.5 text-[11px] font-medium tabular-nums',
                overLimit ? 'bg-warning/25 text-foreground' : 'text-muted-foreground',
              )}
            >
              {tasks.length}
              {column.wip_limit != null ? ` / ${column.wip_limit}` : ''}
            </span>
            {points > 0 ? (
              <span className="ml-auto text-[11px] tabular-nums text-muted-foreground" title="Story points in this column">
                {points} pts
              </span>
            ) : null}
          </div>
          {column.wip_limit != null ? (
            <div className="mt-1.5 h-[3px] overflow-hidden rounded-full bg-foreground/10" aria-hidden>
              <div
                className={cn('h-full origin-left rounded-full transition-[transform,background-color] duration-700 ease-out-expo', overLimit ? 'bg-warning' : 'bg-foreground/40')}
                style={{ transform: `scaleX(${fill})` }}
              />
            </div>
          ) : null}
        </div>
      )}

      <div
        ref={setNodeRef}
        className="flex min-h-[3rem] flex-1 flex-col gap-1.5 pb-0.5"
      >
        <SortableContext items={shown.map((t) => t.id)} strategy={verticalListSortingStrategy}>
          {shown.map((t, i) => (
            <Card
              key={t.id}
              index={i}
              task={t}
              href={cardHref(t)}
              projectKey={projectKey}
              selectMode={selectMode}
              selected={selectedIds.has(t.id)}
              onToggle={onToggleSelect}
              assigneeName={t.assignee_id ? memberNames.get(t.assignee_id) : undefined}
              orgId={orgId}
              members={members}
              labels={labels}
              epic={t.epic_id ? epicsById.get(t.epic_id) : undefined}
              onMutated={onMutated}
            />
          ))}
        </SortableContext>
        {hidden > 0 ? (
          <button
            type="button"
            onClick={() => setLimit((n) => n + CARD_PAGE)}
            className="rounded-md px-2 py-1.5 text-left text-[13px] text-muted-foreground transition-colors hover:bg-foreground/[0.06] hover:text-foreground"
          >
            Show {Math.min(CARD_PAGE, hidden)} more <span className="tabular-nums">({hidden} hidden)</span>
          </button>
        ) : null}

        {inLane ? null : composing ? (
          <div className="origin-top animate-scale-in rounded-md border border-foreground/30 bg-card p-3 shadow-[0_12px_28px_-16px_hsl(var(--ink)/0.4)]">
            <Textarea
              autoFocus
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter' && !e.shiftKey) {
                  e.preventDefault();
                  submit();
                }
                if (e.key === 'Escape') resetComposer();
              }}
              rows={2}
              placeholder="Task title…"
              autoResize maxRows={6} className="rounded-none border-0 bg-transparent p-0 hover:border-0 focus-visible:border-0 focus-visible:shadow-none"
            />
            <div className="mt-2 grid grid-cols-2 gap-2">
              <Select
                size="sm"
                value={priority}
                onValueChange={(v) => setPriority(v as Priority)}
                aria-label="Priority"
                options={PRIORITIES.map((p) => ({ value: p, label: priorityLabel(p) }))}
              />
              <Select
                size="sm"
                value={assigneeId}
                onValueChange={setAssigneeId}
                aria-label="Assignee"
                options={[{ value: '', label: 'Unassigned' }, ...members.map((m) => ({ value: m.user_id, label: m.name }))]}
              />
              <input
                type="date"
                value={startDate}
                onChange={(e) => setStartDate(e.target.value)}
                aria-label="Start date"
                className="w-full rounded-lg border border-input bg-card px-2 py-1.5 text-xs outline-none focus-visible:border-foreground focus-visible:shadow-[inset_0_-2px_0_hsl(var(--signal))] focus-visible:outline-none transition-[border-color,box-shadow] duration-300 hover:border-foreground/40"
              />
              <input
                type="date"
                value={dueDate}
                onChange={(e) => setDueDate(e.target.value)}
                aria-label="Due date"
                className="w-full rounded-lg border border-input bg-card px-2 py-1.5 text-xs outline-none focus-visible:border-foreground focus-visible:shadow-[inset_0_-2px_0_hsl(var(--signal))] focus-visible:outline-none transition-[border-color,box-shadow] duration-300 hover:border-foreground/40"
              />
            </div>
            <div className="mt-2 flex items-center gap-2">
              <button
                onClick={submit}
                disabled={addPending || !title.trim()}
                className="rounded-sm bg-foreground px-3 py-1.5 text-xs font-medium text-background transition-[background-color,transform] hover:bg-signal hover:text-signal-foreground active:scale-95 disabled:opacity-50"
              >
                {addPending ? 'Adding…' : 'Add'}
              </button>
              <button
                onClick={resetComposer}
                className="text-xs text-muted-foreground hover:text-foreground"
              >
                Cancel
              </button>
              <span className="ml-auto font-mono text-[10px] text-muted-foreground">↵ add · esc</span>
            </div>
          </div>
        ) : (
          <button
            onClick={() => setComposing(true)}
            className="group/add flex items-center gap-1.5 rounded-md px-2 py-1.5 text-left text-[13px] text-muted-foreground transition-[background-color,color] hover:bg-foreground/[0.06] hover:text-foreground"
          >
            <Plus className="h-3.5 w-3.5" /> Create task
          </button>
        )}
      </div>
    </div>
  );
}
