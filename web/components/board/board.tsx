'use client';

import { useState, useEffect, useSyncExternalStore } from 'react';
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
import { ChevronDown, Plus } from 'lucide-react';

import type { Board as BoardData, BulkActionInput, Priority, TaskCard } from '@/lib/api/types';
import { getBoard, bulkTasks, createColumn, createTask, moveTask } from '@/lib/api/board';
import { listLabels } from '@/lib/api/labels';
import { listOrgMembers } from '@/lib/api/orgs';
import { listSprints } from '@/lib/api/sprints';
import { assignEpic, listEpics } from '@/lib/api/epics';
import { getTask, updateTask } from '@/lib/api/tasks';
import { useOrg } from '@/lib/org/context';
import { useAuth } from '@/lib/auth/context';
import { useOrgAgents } from '@/lib/org/use-agents';
import { ApiError } from '@/lib/api/client';
import { useToast } from '@/components/ui/toast';
import { between } from '@/lib/board/rank';
import { moveCard } from '@/lib/board/board-cache';
import { doneColumnId, statusTone } from '@/lib/board/status';
import {
  NO_VALUE,
  laneDropId,
  laneKeyOf,
  lanesFor,
  parseLaneDropId,
  type GroupBy,
} from '@/lib/board/swimlanes';
import { cn } from '@/lib/utils';
import { burst } from '@/components/canvas/burst-layer';
import { Skeleton } from '@/components/ui/skeleton';
import { Column, type QuickTaskInput } from './column';
import { PriorityBars } from './priority-bars';
import { BoardToolbar, EMPTY_FILTER, type BoardFilter } from './board-toolbar';
import { SavedViewsMenu } from './saved-views-menu';

interface MoveVars {
  taskId: string;
  columnId: string;
  rank: string;
  snapshot: BoardData;
}

interface LaneMoveVars extends MoveVars {
  by: Exclude<GroupBy, 'none'>;
  laneKey: string;
}

// Set the swimlane field of one cached card (optimistic half of a lane move).
function withLaneField(board: BoardData, taskId: string, by: GroupBy, laneKey: string): BoardData {
  const value = laneKey === NO_VALUE ? null : laneKey;
  return {
    ...board,
    columns: board.columns.map((col) => ({
      ...col,
      tasks: col.tasks.map((t) => {
        if (t.id !== taskId) return t;
        if (by === 'assignee') return { ...t, assignee_id: value ?? undefined };
        if (by === 'epic') return { ...t, epic_id: value };
        return { ...t, priority: laneKey as Priority };
      }),
    })),
  };
}

// The swimlane choice is a per-viewer convenience, kept in this browser only.
function groupKey(projectId: string): string {
  return `fluxboard:board-group:${projectId}`;
}

const noSubscribe = () => () => {};
function readGroupBy(projectId: string): GroupBy {
  try {
    const saved = localStorage.getItem(groupKey(projectId));
    return saved === 'assignee' || saved === 'epic' || saved === 'priority' ? saved : 'none';
  } catch {
    return 'none'; // Storage unavailable (private mode): stay ungrouped.
  }
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
  const { data: epics } = useQuery({
    queryKey: ['epics', projectId],
    queryFn: () => listEpics(orgId, projectId),
  });
  const { data: membersPage } = useQuery({
    queryKey: ['members', orgId],
    queryFn: () => listOrgMembers(orgId, { limit: 100 }),
  });
  const { agents } = useOrgAgents();
  // Agents are assignable like people (ADR-033); they come last, marked.
  const members = [
    ...(membersPage?.items ?? []).map((m) => ({ user_id: m.user_id, name: m.name || m.email })),
    ...agents.map((a) => ({ user_id: a.id, name: `${a.name} (agent)` })),
  ];

  const [filter, setFilter] = useState<BoardFilter>(EMPTY_FILTER);
  const storedGroupBy = useSyncExternalStore(noSubscribe, () => readGroupBy(projectId), () => 'none' as GroupBy);
  const [chosenGroupBy, setChosenGroupBy] = useState<GroupBy | null>(null);
  const groupBy = chosenGroupBy ?? storedGroupBy;
  const [collapsed, setCollapsed] = useState<Set<string>>(new Set());
  // The saved view last applied or saved (FR-PROJ-011); null = none.
  const [activeViewId, setActiveViewId] = useState<string | null>(null);
  function setGroupBy(g: GroupBy) {
    setChosenGroupBy(g);
    setCollapsed(new Set());
    try {
      localStorage.setItem(groupKey(projectId), g);
    } catch {
      // Storage blocked: the choice lasts for this visit only.
    }
  }
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

  // A drop into another swimlane: move the card, then set the lane's field.
  // Sequential so the field update reads the version the move produced.
  const laneMoveMutation = useMutation({
    mutationFn: async (v: LaneMoveVars) => {
      await moveTask(orgId, v.taskId, { column_id: v.columnId, rank: v.rank });
      const value = v.laneKey === NO_VALUE ? null : v.laneKey;
      if (v.by === 'epic') {
        await assignEpic(orgId, projectId, { task_ids: [v.taskId], epic_id: value });
        return;
      }
      const t = await getTask(orgId, v.taskId);
      await updateTask(orgId, v.taskId, {
        title: t.title,
        description: t.description,
        assignee_id: v.by === 'assignee' ? value : (t.assignee_id ?? null),
        priority: v.by === 'priority' ? (v.laneKey as Priority) : (t.priority as Priority),
        start_date: t.start_date ?? null,
        due_date: t.due_date ?? null,
        version: t.version,
      });
    },
    onError: (err, v) => {
      queryClient.setQueryData(boardKey, v.snapshot);
      toast({
        title: 'Couldn’t move the card to that lane',
        description:
          err instanceof ApiError && err.status === 409
            ? 'The task changed meanwhile — the board has been refreshed.'
            : 'Please try again.',
        variant: 'error',
      });
    },
    onSettled: () => {
      queryClient.invalidateQueries({ queryKey: boardKey });
      queryClient.invalidateQueries({ queryKey: ['epics', projectId] });
    },
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
      if (filter.epic === 'none' && t.epic_id) return false;
      if (filter.epic !== '' && filter.epic !== 'none' && t.epic_id !== filter.epic) return false;
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

    // Destination column: `over` is a column id, a lane-scoped column id
    // (swimlanes) or a card id.
    const laneDrop = parseLaneDropId(overId);
    const dropColId = laneDrop ? laneDrop.columnId : overId;
    let destCol = current.columns.find((c) => c.id === dropColId);
    if (!destCol) destCol = current.columns.find((c) => c.tasks.some((t) => t.id === overId));
    if (!destCol) return;

    const destTasks = destCol.tasks.filter((t) => t.id !== activeTaskId);
    const overTask = destTasks.find((t) => t.id === overId);
    const destLane = groupBy === 'none' ? NO_VALUE : laneDrop ? laneDrop.laneKey : overTask ? laneKeyOf(overTask, groupBy) : laneKeyOf(activeTask, groupBy);
    let insertIndex: number;
    if (overTask) insertIndex = destTasks.indexOf(overTask);
    else if (laneDrop) {
      // Dropped on a lane's empty space: after that lane's last card in the column.
      const lastInLane = destTasks.map((t) => laneKeyOf(t, groupBy)).lastIndexOf(laneDrop.laneKey);
      insertIndex = lastInLane < 0 ? destTasks.length : lastInLane + 1;
    } else insertIndex = destTasks.length;
    const prev = destTasks[insertIndex - 1]?.rank ?? '';
    const next = destTasks[insertIndex]?.rank ?? '';

    let newRank: string;
    try {
      newRank = between(prev, next);
    } catch {
      // Neighbours out of order (shouldn't happen with sorted ranks) — bail.
      return;
    }

    const moved = moveCard(current, activeTaskId, destCol.id, newRank) ?? current;
    if (groupBy !== 'none' && destLane !== laneKeyOf(activeTask, groupBy)) {
      queryClient.setQueryData(boardKey, withLaneField(moved, activeTaskId, groupBy, destLane));
      laneMoveMutation.mutate({
        taskId: activeTaskId, columnId: destCol.id, rank: newRank, snapshot: current, by: groupBy, laneKey: destLane,
      });
    } else {
      queryClient.setQueryData(boardKey, moved);
      moveMutation.mutate({ taskId: activeTaskId, columnId: destCol.id, rank: newRank, snapshot: current });
    }

    // Shipping a card into the done lane from elsewhere earns a burst.
    const fromCol = current.columns.find((c) => c.tasks.some((t) => t.id === activeTaskId));
    if (destCol.id === doneColumnId(current) && fromCol?.id !== destCol.id) {
      const r = active.rect.current.translated;
      if (r) burst(r.left + r.width / 2, r.top + r.height / 2, 0.9);
      setFlashCol(destCol.id);
      window.setTimeout(() => setFlashCol(null), 900);
    }
  }

  const cardHref = (t: TaskCard) => `/app/${slug}/projects/${projectKey}/tasks/${t.number}`;
  const columnProps = (col: (typeof board.columns)[number]) => ({
    column: col,
    projectKey,
    cardHref,
    selectMode,
    selectedIds,
    onToggleSelect: toggleSelect,
    onAddTask: (columnId: string, input: QuickTaskInput) => createTaskMutation.mutate({ columnId, ...input }),
    addPending: createTaskMutation.isPending,
    members,
    orgId,
    labels: labels ?? [],
    epics: epics ?? [],
    onMutated: () => queryClient.invalidateQueries({ queryKey: boardKey }),
    flash: flashCol === col.id,
    tone: statusTone(board, col.id),
  });
  const lanes =
    groupBy === 'none'
      ? []
      : lanesFor(board.columns.flatMap((c) => visible(c.tasks)), groupBy, { members, epics: epics ?? [] });
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
        epics={epics ?? []}
        groupBy={groupBy}
        onGroupBy={setGroupBy}
        viewsSlot={
          <SavedViewsMenu
            orgId={orgId}
            projectId={projectId}
            state={{ filter, groupBy }}
            onApply={(v) => {
              setFilter(v.filter);
              setGroupBy(v.groupBy);
            }}
            activeId={activeViewId}
            onActiveChange={setActiveViewId}
          />
        }
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
        {groupBy !== 'none' ? (
          <div className="scrollbar-thin -mx-1 overflow-x-auto px-1 pb-5">
            <div className="flex w-max gap-2.5">
              {board.columns.map((col) => {
                const tasks = visible(col.tasks);
                const points = tasks.reduce((sum, t) => sum + (t.estimate ?? 0), 0);
                return (
                  <div key={col.id} className="flex h-8 w-[272px] shrink-0 items-center gap-2 px-3.5">
                    <span className={cn('h-2 w-2 shrink-0 rounded-full', statusTone(board, col.id).dot)} aria-hidden />
                    <h3 className="truncate text-[11px] font-semibold uppercase tracking-[0.06em] text-muted-foreground">
                      {col.name}
                    </h3>
                    <span className="text-[11px] tabular-nums text-muted-foreground">{tasks.length}</span>
                    {points > 0 ? (
                      <span className="ml-auto text-[11px] tabular-nums text-muted-foreground">{points} pts</span>
                    ) : null}
                  </div>
                );
              })}
            </div>
            {lanes.length === 0 ? (
              <p className="px-3 py-6 text-sm text-muted-foreground">No tasks match the filters.</p>
            ) : null}
            {lanes.map((lane) => {
              const inLane = (t: TaskCard) => laneKeyOf(t, groupBy) === lane.key;
              const laneTasks = board.columns.flatMap((c) => visible(c.tasks).filter(inLane));
              const lanePoints = laneTasks.reduce((sum, t) => sum + (t.estimate ?? 0), 0);
              const isCollapsed = collapsed.has(lane.key);
              return (
                <section key={lane.key} aria-label={`${lane.label} swimlane`} className="mt-3">
                  <button
                    type="button"
                    aria-expanded={!isCollapsed}
                    onClick={() =>
                      setCollapsed((prev) => {
                        const next = new Set(prev);
                        if (next.has(lane.key)) next.delete(lane.key);
                        else next.add(lane.key);
                        return next;
                      })
                    }
                    className="sticky left-0 mb-1.5 flex items-center gap-2 rounded-md px-2 py-1 text-[13px] font-medium hover:bg-secondary/60"
                  >
                    <ChevronDown
                      className={cn('h-3.5 w-3.5 text-muted-foreground transition-transform', isCollapsed && '-rotate-90')}
                      aria-hidden
                    />
                    <span>{lane.label}</span>
                    <span className="text-[11px] font-normal tabular-nums text-muted-foreground">
                      {laneTasks.length} {laneTasks.length === 1 ? 'task' : 'tasks'}
                      {lanePoints > 0 ? ` · ${lanePoints} pts` : ''}
                    </span>
                  </button>
                  {isCollapsed ? null : (
                    <div className="flex w-max items-start gap-2.5">
                      {board.columns.map((col) => (
                        <Column
                          key={col.id}
                          {...columnProps(col)}
                          tasks={visible(col.tasks).filter(inLane)}
                          dropId={laneDropId(lane.key, col.id)}
                          inLane
                        />
                      ))}
                    </div>
                  )}
                </section>
              );
            })}
          </div>
        ) : (
        <div className="scrollbar-thin -mx-1 flex items-start gap-2.5 overflow-x-auto px-1 pb-5">
          {board.columns.map((col, i) => (
            <Column
              key={col.id}
              {...columnProps(col)}
              tasks={visible(col.tasks)}
              composeSignal={i === 0 ? composeSignal : 0}
            />
          ))}

          {isAdmin ? (
            <div className="w-[272px] shrink-0">
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
                  className="flex h-9 w-full items-center gap-2 rounded-lg px-3 text-[13px] text-muted-foreground transition-[color,background-color] duration-300 hover:bg-secondary/60 hover:text-foreground"
                >
                  <Plus className="h-4 w-4" /> Add column
                </button>
              )}
            </div>
          ) : null}
        </div>
        )}

        <DragOverlay dropAnimation={{ duration: 320, easing: 'cubic-bezier(0.16, 1, 0.3, 1)' }}>
          {activeTask ? (
            <div className="w-[256px] origin-center animate-[card-lift_0.25s_var(--ease-out-expo)_forwards] cursor-grabbing rounded-md border border-foreground/30 bg-card px-3 py-2.5 text-sm shadow-[0_24px_48px_-20px_hsl(var(--ink)/0.5)]">
              <p className="line-clamp-3 leading-snug">{activeTask.title}</p>
              <div className="mt-2.5 flex items-center gap-2">
                <span className="font-mono text-[11px] font-medium text-muted-foreground">
                  {projectKey}-{activeTask.number}
                </span>
                <PriorityBars priority={activeTask.priority} />
              </div>
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
        <div key={c} className="w-[272px] shrink-0 rounded-lg bg-secondary/45 p-2">
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
