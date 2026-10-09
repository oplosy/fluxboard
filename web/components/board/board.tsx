'use client';

import { useState, useEffect } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  DndContext,
  DragOverlay,
  KeyboardSensor,
  PointerSensor,
  closestCorners,
  useSensor,
  useSensors,
  type DragEndEvent,
  type DragStartEvent,
} from '@dnd-kit/core';
import { sortableKeyboardCoordinates } from '@dnd-kit/sortable';
import { Plus } from 'lucide-react';

import type { Board as BoardData, BulkActionInput, TaskCard } from '@/lib/api/types';
import { getBoard, bulkTasks, createColumn, createTask, moveTask } from '@/lib/api/board';
import { listLabels } from '@/lib/api/labels';
import { listOrgMembers } from '@/lib/api/orgs';
import { listSprints } from '@/lib/api/sprints';
import { useOrg } from '@/lib/org/context';
import { useAuth } from '@/lib/auth/context';
import { ApiError } from '@/lib/api/client';
import { useToast } from '@/components/ui/toast';
import { between } from '@/lib/board/rank';
import { burst } from '@/components/canvas/burst-layer';
import { Skeleton } from '@/components/ui/skeleton';
import { Column, type QuickTaskInput } from './column';
import { PriorityBars } from './priority-bars';
import { BoardToolbar, EMPTY_FILTER, type BoardFilter } from './board-toolbar';

interface MoveVars {
  taskId: string;
  columnId: string;
  rank: string;
  snapshot: BoardData;
}

// A column counts as "done" when its name says so; otherwise the last column is.
function doneColumnId(board: BoardData): string | undefined {
  const named = board.columns.find((c) => /\b(done|complete[d]?|shipped|closed|released)\b/i.test(c.name));
  return (named ?? board.columns[board.columns.length - 1])?.id;
}

// Move `taskId` to (destColId, newRank) in the cached board projection, keeping
// each column sorted by rank. Pure — returns a new board.
function applyMove(board: BoardData, taskId: string, destColId: string, newRank: string): BoardData {
  let moving: TaskCard | undefined;
  const stripped = board.columns.map((col) => {
    const idx = col.tasks.findIndex((t) => t.id === taskId);
    if (idx < 0) return col;
    moving = { ...(col.tasks[idx] as TaskCard) };
    return { ...col, tasks: col.tasks.filter((t) => t.id !== taskId) };
  });
  if (!moving) return board;
  const placed = { ...moving, rank: newRank };
  return {
    ...board,
    columns: stripped.map((col) => {
      if (col.id !== destColId) return col;
      const tasks = [...col.tasks, placed].sort((a, b) => (a.rank < b.rank ? -1 : a.rank > b.rank ? 1 : 0));
      return { ...col, tasks };
    }),
  };
}

export function Board({ projectId, projectKey }: { projectId: string; projectKey: string }) {
  const { orgId, slug, isAdmin } = useOrg();
  const { userId } = useAuth();
  const { toast } = useToast();
  const queryClient = useQueryClient();

  const boardKey = ['board', projectId] as const;

  const { data: board, isLoading, isError } = useQuery({ queryKey: boardKey, queryFn: () => getBoard(orgId, projectId) });
  const { data: labels } = useQuery({ queryKey: ['labels', orgId], queryFn: () => listLabels(orgId) });
  const { data: sprints } = useQuery({
    queryKey: ['sprints', projectId],
    queryFn: () => listSprints(orgId, projectId),
  });
  const { data: membersPage } = useQuery({
    queryKey: ['members', orgId],
    queryFn: () => listOrgMembers(orgId, { limit: 100 }),
  });
  const members = (membersPage?.items ?? []).map((m) => ({ user_id: m.user_id, name: m.name || m.email }));

  const [filter, setFilter] = useState<BoardFilter>(EMPTY_FILTER);
  const [selectMode, setSelectMode] = useState(false);
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set());
  const [activeId, setActiveId] = useState<string | null>(null);
  const [addingColumn, setAddingColumn] = useState(false);
  const [newColumnName, setNewColumnName] = useState('');
  // Bumped to open the quick composer of the first column (keyboard "c").
  const [composeSignal, setComposeSignal] = useState(0);
  // Column that just received a shipped card (pulses briefly).
  const [flashCol, setFlashCol] = useState<string | null>(null);

  // Board shortcuts: "/" focuses the filter, "c" starts a task in the first
  // column. Ignored while typing, and never hijacks browser modifiers.
  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.metaKey || e.ctrlKey || e.altKey) return;
      const el = e.target as HTMLElement | null;
      if (el && (el.tagName === 'INPUT' || el.tagName === 'TEXTAREA' || el.tagName === 'SELECT' || el.isContentEditable)) return;
      if (e.key === '/') {
        e.preventDefault();
        document.getElementById('board-filter')?.focus();
      } else if (e.key === 'c') {
        setComposeSignal((n) => n + 1);
      }
    }
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, []);

  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 5 } }),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }),
  );

  const moveMutation = useMutation({
    mutationFn: (v: MoveVars) => moveTask(orgId, v.taskId, { column_id: v.columnId, rank: v.rank }),
    onError: (err, v) => {
      queryClient.setQueryData(boardKey, v.snapshot);
      const conflict = err instanceof ApiError && err.status === 409;
      toast({
        title: conflict ? 'Card moved elsewhere' : 'Move failed',
        description: conflict
          ? 'Someone changed this board — it has been refreshed.'
          : 'Please try again.',
        variant: 'error',
      });
    },
    onSettled: () => queryClient.invalidateQueries({ queryKey: boardKey }),
  });

  const createTaskMutation = useMutation({
    mutationFn: (v: { columnId: string } & QuickTaskInput) =>
      createTask(orgId, projectId, {
        column_id: v.columnId,
        title: v.title,
        description: '',
        priority: v.priority,
        assignee_id: v.assignee_id,
        start_date: v.start_date,
        due_date: v.due_date,
      }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: boardKey }),
    onError: (err) =>
      toast({
        title: 'Couldn’t add task',
        description: err instanceof ApiError && err.status === 403 ? 'You don’t have access to add tasks here.' : 'Please try again.',
        variant: 'error',
      }),
  });

  const bulkMutation = useMutation({
    mutationFn: (input: BulkActionInput) => bulkTasks(orgId, input),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: boardKey });
      setSelectedIds(new Set());
    },
    onError: () => toast({ title: 'Bulk action failed', description: 'Please try again.', variant: 'error' }),
  });

  const addColumnMutation = useMutation({
    mutationFn: (name: string) => createColumn(orgId, projectId, { name }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: boardKey });
      setNewColumnName('');
      setAddingColumn(false);
    },
    onError: () => toast({ title: 'Couldn’t add column', variant: 'error' }),
  });

  if (isLoading) return <BoardSkeleton />;
  if (isError || !board) return <p className="text-sm text-destructive">Couldn’t load the board.</p>;

  function visible(tasks: TaskCard[]): TaskCard[] {
    const q = filter.text.trim().toLowerCase();
    return tasks.filter((t) => {
      if (q && !t.title.toLowerCase().includes(q)) return false;
      if (filter.priority && t.priority !== filter.priority) return false;
      if (filter.assignee === 'me' && t.assignee_id !== userId) return false;
      if (filter.assignee === 'unassigned' && t.assignee_id) return false;
      if (filter.sprint === 'backlog' && t.sprint_id) return false;
      if (filter.sprint !== '' && filter.sprint !== 'backlog' && t.sprint_id !== filter.sprint) return false;
      return true;
    });
  }

  function toggleSelect(id: string) {
    setSelectedIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }

  function onDragStart(e: DragStartEvent) {
    setActiveId(String(e.active.id));
  }

  function onDragEnd(e: DragEndEvent) {
    setActiveId(null);
    const { active, over } = e;
    if (!over) return;
    const activeTaskId = String(active.id);
    const overId = String(over.id);

    const current = queryClient.getQueryData<BoardData>(boardKey);
    if (!current) return;

    const activeTask = current.columns.flatMap((c) => c.tasks).find((t) => t.id === activeTaskId);
    if (!activeTask) return;

    // Destination column: `over` is either a column id or a card id.
    let destCol = current.columns.find((c) => c.id === overId);
    if (!destCol) destCol = current.columns.find((c) => c.tasks.some((t) => t.id === overId));
    if (!destCol) return;

    const destTasks = destCol.tasks.filter((t) => t.id !== activeTaskId);
    let insertIndex: number;
    if (overId === destCol.id) insertIndex = destTasks.length;
    else {
      const oi = destTasks.findIndex((t) => t.id === overId);
      insertIndex = oi < 0 ? destTasks.length : oi;
    }
    const prev = destTasks[insertIndex - 1]?.rank ?? '';
    const next = destTasks[insertIndex]?.rank ?? '';

    let newRank: string;
    try {
      newRank = between(prev, next);
    } catch {
      // Neighbours out of order (shouldn't happen with sorted ranks) — bail.
      return;
    }

    queryClient.setQueryData(boardKey, applyMove(current, activeTaskId, destCol.id, newRank));
    moveMutation.mutate({ taskId: activeTaskId, columnId: destCol.id, rank: newRank, snapshot: current });

    // Shipping a card into the done lane from elsewhere earns a burst.
    const fromCol = current.columns.find((c) => c.tasks.some((t) => t.id === activeTaskId));
    if (destCol.id === doneColumnId(current) && fromCol?.id !== destCol.id) {
      const r = active.rect.current.translated;
      if (r) burst(r.left + r.width / 2, r.top + r.height / 2, 0.9);
      setFlashCol(destCol.id);
      window.setTimeout(() => setFlashCol(null), 900);
    }
  }

  const doneId = doneColumnId(board);
  const cardHref = (t: TaskCard) => `/app/${slug}/projects/${projectKey}/tasks/${t.number}`;
  const activeTask = activeId
    ? board.columns.flatMap((c) => c.tasks).find((t) => t.id === activeId)
    : undefined;

  return (
    <div>
      <BoardToolbar
        filter={filter}
        onFilter={setFilter}
        selectMode={selectMode}
        onToggleSelectMode={() => {
          setSelectMode((m) => !m);
          setSelectedIds(new Set());
        }}
        selectedCount={selectedIds.size}
        columns={board.columns}
        labels={labels ?? []}
        sprints={sprints ?? []}
        bulkPending={bulkMutation.isPending}
        onBulkMove={(columnId) => bulkMutation.mutate({ action: 'move', task_ids: [...selectedIds], column_id: columnId })}
        onBulkAssignMe={() =>
          bulkMutation.mutate({ action: 'assign', task_ids: [...selectedIds], assignee_id: userId })
        }
        onBulkUnassign={() =>
          bulkMutation.mutate({ action: 'assign', task_ids: [...selectedIds], assignee_id: null })
        }
        onBulkLabel={(labelId) => bulkMutation.mutate({ action: 'label', task_ids: [...selectedIds], label_id: labelId })}
        onClearSelection={() => setSelectedIds(new Set())}
      />

      <DndContext
        sensors={sensors}
        collisionDetection={closestCorners}
        onDragStart={onDragStart}
        onDragEnd={onDragEnd}
      >
        <div className="scrollbar-thin -mx-1 flex items-start gap-3 overflow-x-auto px-1 pb-5">
          {board.columns.map((col, i) => (
            <Column
              key={col.id}
              column={col}
              tasks={visible(col.tasks)}
              cardHref={cardHref}
              selectMode={selectMode}
              selectedIds={selectedIds}
              onToggleSelect={toggleSelect}
              onAddTask={(columnId, input) => createTaskMutation.mutate({ columnId, ...input })}
              addPending={createTaskMutation.isPending}
              members={members}
              orgId={orgId}
              labels={labels ?? []}
              onMutated={() => queryClient.invalidateQueries({ queryKey: boardKey })}
              composeSignal={i === 0 ? composeSignal : 0}
              flash={flashCol === col.id}
              isDone={col.id === doneId}
            />
          ))}

          {isAdmin ? (
            <div className="w-[300px] shrink-0">
              {addingColumn ? (
                <div className="origin-top animate-scale-in rounded-lg border border-foreground/30 bg-card p-3">
                  <input
                    autoFocus
                    value={newColumnName}
                    onChange={(e) => setNewColumnName(e.target.value)}
                    onKeyDown={(e) => {
                      if (e.key === 'Enter' && newColumnName.trim()) addColumnMutation.mutate(newColumnName.trim());
                      if (e.key === 'Escape') setAddingColumn(false);
                    }}
                    placeholder="Column name…"
                    className="w-full rounded-md border border-input bg-card px-3 py-2 text-sm outline-none focus-visible:border-foreground focus-visible:shadow-[inset_0_-2px_0_hsl(var(--signal))] focus-visible:outline-none transition-[border-color,box-shadow] duration-300 hover:border-foreground/40"
                  />
                  <div className="mt-1 flex gap-2">
                    <button
                      onClick={() => newColumnName.trim() && addColumnMutation.mutate(newColumnName.trim())}
                      disabled={addColumnMutation.isPending || !newColumnName.trim()}
                    className="rounded-sm bg-foreground px-3 py-1.5 text-xs font-medium text-background transition-[background-color,transform] hover:bg-signal hover:text-signal-foreground active:scale-95 disabled:opacity-50"
                    >
                      Add
                    </button>
                    <button
                      onClick={() => setAddingColumn(false)}
                      className="text-xs text-muted-foreground hover:text-foreground"
                    >
                      Cancel
                    </button>
                  </div>
                </div>
              ) : (
                <button
                  onClick={() => setAddingColumn(true)}
                  className="group/col flex h-12 w-full items-center gap-2 rounded-lg border border-dashed border-foreground/20 px-3 text-sm text-muted-foreground transition-[border-color,color,background-color] duration-300 hover:border-foreground/50 hover:bg-secondary/40 hover:text-foreground"
                >
                  <Plus className="h-4 w-4 transition-transform duration-500 ease-spring group-hover/col:rotate-90" /> Add column
                </button>
              )}
            </div>
          ) : null}
        </div>

        <DragOverlay dropAnimation={{ duration: 320, easing: 'cubic-bezier(0.16, 1, 0.3, 1)' }}>
          {activeTask ? (
            <div className="w-[284px] origin-center animate-[card-lift_0.25s_var(--ease-out-expo)_forwards] cursor-grabbing rounded-md border border-foreground/40 bg-card p-3 text-sm shadow-[0_30px_60px_-20px_hsl(var(--ink)/0.5)]">
              <div className="mb-1.5 flex items-center gap-2">
                <span className="font-mono text-[10.5px] text-muted-foreground">#{activeTask.number}</span>
                <PriorityBars priority={activeTask.priority} />
              </div>
              <p className="line-clamp-3 font-medium leading-snug">{activeTask.title}</p>
            </div>
          ) : null}
        </DragOverlay>
      </DndContext>
    </div>
  );
}

function BoardSkeleton() {
  return (
    <div className="flex gap-3 overflow-hidden" aria-busy="true" aria-label="Loading board">
      {[4, 2, 3, 1].map((n, c) => (
        <div key={c} className="w-[300px] shrink-0 rounded-lg bg-secondary/50 p-2">
          <Skeleton className="mx-1 mb-3 mt-1 h-4 w-24" />
          <div className="space-y-2">
            {Array.from({ length: n }).map((_, i) => (
              <Skeleton key={i} className="h-[76px] w-full rounded-md" style={{ animationDelay: `${(c + i) * 90}ms` }} />
            ))}
          </div>
        </div>
      ))}
    </div>
  );
}
