'use client';

import { useEffect, useState } from 'react';
import { useDroppable } from '@dnd-kit/core';
import { SortableContext, verticalListSortingStrategy } from '@dnd-kit/sortable';
import { Plus } from 'lucide-react';

import type { BoardColumn, Label, Priority, TaskCard } from '@/lib/api/types';
import { PRIORITIES, priorityLabel } from '@/lib/board/priority';
import { cn } from '@/lib/utils';
import { Card } from './card';

export interface QuickTaskInput {
  title: string;
  priority: Priority;
  assignee_id: string | null;
  start_date: string | null;
  due_date: string | null;
}

export interface AssigneeOption {
  user_id: string;
  name: string;
}

// One board column: a droppable region wrapping a sortable list of cards, a WIP
// indicator, and an inline "add task" composer.
export function Column({
  column,
  tasks,
  cardHref,
  selectMode,
  selectedIds,
  onToggleSelect,
  onAddTask,
  addPending,
  members,
  orgId,
  labels,
  onMutated,
  composeSignal,
  flash = false,
  isDone = false,
}: {
  column: BoardColumn;
  tasks: TaskCard[];
  cardHref: (task: TaskCard) => string;
  selectMode: boolean;
  selectedIds: Set<string>;
  onToggleSelect: (id: string) => void;
  onAddTask: (columnId: string, input: QuickTaskInput) => void;
  addPending: boolean;
  members: AssigneeOption[];
  orgId: string;
  labels: Label[];
  onMutated: () => void;
  composeSignal?: number;
  /** Briefly pulses the column (e.g. a card was just shipped into it). */
  flash?: boolean;
  /** Treat as the board's "done" lane (signal accent). */
  isDone?: boolean;
}) {
  const { setNodeRef, isOver } = useDroppable({ id: column.id });
  const [composing, setComposing] = useState(false);
  const [title, setTitle] = useState('');
  const [priority, setPriority] = useState<Priority>('none');
  const [assigneeId, setAssigneeId] = useState('');
  const [startDate, setStartDate] = useState('');
  const [dueDate, setDueDate] = useState('');

  const overLimit = column.wip_limit != null && tasks.length >= column.wip_limit;

  // External trigger (board "c" shortcut) opens this column's composer.
  useEffect(() => {
    if (composeSignal) setComposing(true);
  }, [composeSignal]);

  function resetComposer() {
    setTitle('');
    setPriority('none');
    setAssigneeId('');
    setStartDate('');
    setDueDate('');
    setComposing(false);
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

  return (
    <div
      className={cn(
        'flex w-[300px] shrink-0 flex-col rounded-lg border bg-secondary/50 p-1.5 transition-[border-color,background-color,box-shadow] duration-300',
        isOver ? 'border-dashed border-signal/70 bg-signal/[0.04]' : 'border-transparent',
        flash && 'animate-[column-flash_0.9s_var(--ease-out-expo)]',
      )}
    >
      <div className="px-2.5 pb-2.5 pt-2">
        <div className="flex items-center gap-2">
          <span className={cn('h-2 w-2 rounded-full', isDone ? 'bg-signal' : 'bg-foreground/30')} />
          <h3 className="truncate font-display text-[14px] font-semibold tracking-[-0.01em]">{column.name}</h3>
          <span
            key={tasks.length}
            className={cn(
              'ml-auto animate-scale-in rounded-sm px-1.5 py-0.5 font-mono text-[10px] tabular-nums',
              overLimit ? 'bg-warning/25 text-foreground' : 'bg-background text-muted-foreground',
            )}
          >
            {tasks.length}
            {column.wip_limit != null ? ` / ${column.wip_limit}` : ''}
          </span>
        </div>
        {column.wip_limit != null ? (
          <div className="mt-2 h-[3px] overflow-hidden rounded-full bg-foreground/10" aria-hidden>
            <div
              className={cn('h-full origin-left rounded-full transition-[transform,background-color] duration-700 ease-out-expo', overLimit ? 'bg-warning' : 'bg-foreground/50')}
              style={{ transform: `scaleX(${fill})` }}
            />
          </div>
        ) : null}
      </div>

      <div
        ref={setNodeRef}
        className="flex min-h-[3rem] flex-1 flex-col gap-2 px-0.5 pb-1"
      >
        <SortableContext items={tasks.map((t) => t.id)} strategy={verticalListSortingStrategy}>
          {tasks.map((t, i) => (
            <Card
              key={t.id}
              index={i}
              task={t}
              href={cardHref(t)}
              selectMode={selectMode}
              selected={selectedIds.has(t.id)}
              onToggle={onToggleSelect}
              assigneeName={t.assignee_id ? members.find((m) => m.user_id === t.assignee_id)?.name : undefined}
              orgId={orgId}
              members={members}
              labels={labels}
              onMutated={onMutated}
            />
          ))}
        </SortableContext>

        {composing ? (
          <div className="origin-top animate-scale-in rounded-md border border-foreground/30 bg-card p-3 shadow-[0_12px_28px_-16px_hsl(var(--ink)/0.4)]">
            <textarea
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
              className="w-full resize-none bg-transparent text-sm outline-none placeholder:text-muted-foreground"
            />
            <div className="mt-2 grid grid-cols-2 gap-2">
              <select
                value={priority}
                onChange={(e) => setPriority(e.target.value as Priority)}
                aria-label="Priority"
                className="w-full rounded-lg border border-input bg-card px-2 py-1.5 text-xs outline-none focus-visible:border-foreground focus-visible:shadow-[inset_0_-2px_0_hsl(var(--signal))] focus-visible:outline-none transition-[border-color,box-shadow] duration-300 hover:border-foreground/40"
              >
                {PRIORITIES.map((p) => (
                  <option key={p} value={p}>
                    {priorityLabel(p)}
                  </option>
                ))}
              </select>
              <select
                value={assigneeId}
                onChange={(e) => setAssigneeId(e.target.value)}
                aria-label="Assignee"
                className="w-full rounded-lg border border-input bg-card px-2 py-1.5 text-xs outline-none focus-visible:border-foreground focus-visible:shadow-[inset_0_-2px_0_hsl(var(--signal))] focus-visible:outline-none transition-[border-color,box-shadow] duration-300 hover:border-foreground/40"
              >
                <option value="">Unassigned</option>
                {members.map((m) => (
                  <option key={m.user_id} value={m.user_id}>
                    {m.name}
                  </option>
                ))}
              </select>
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
            className="group/add flex items-center gap-1.5 rounded-md border border-dashed border-transparent px-2.5 py-2 text-left text-xs text-muted-foreground transition-[border-color,background-color,color] hover:border-foreground/25 hover:bg-background hover:text-foreground"
          >
            <Plus className="h-3.5 w-3.5 transition-transform duration-500 ease-spring group-hover/add:rotate-90" /> Add task
          </button>
        )}
      </div>
    </div>
  );
}
