import type { Board, Task, TaskCard } from '@/lib/api/types';

// Pure updates of the cached board projection, shared by drag-drop (optimistic)
// and the realtime bridge (events from other people), so neither has to refetch
// the whole board for a single card.

/**
 * Move `taskId` to (destColId, newRank), keeping each column sorted by rank.
 * Returns null when the card is not on this board (the caller refetches).
 */
export function moveCard(board: Board, taskId: string, destColId: string, newRank: string): Board | null {
  if (!board.columns.some((c) => c.id === destColId)) return null;
  let moving: TaskCard | undefined;
  const stripped = board.columns.map((col) => {
    const idx = col.tasks.findIndex((t) => t.id === taskId);
    if (idx < 0) return col;
    moving = { ...(col.tasks[idx] as TaskCard) };
    return { ...col, tasks: col.tasks.filter((t) => t.id !== taskId) };
  });
  if (!moving) return null;
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

/**
 * Copy a fetched task's editable fields onto its card. Returns null when the
 * card is not on this board or the task now sits in another column (a move the
 * cache has not seen: the caller refetches).
 */
export function patchCard(board: Board, task: Task): Board | null {
  const col = board.columns.find((c) => c.tasks.some((t) => t.id === task.id));
  if (!col || col.id !== task.column_id) return null;
  return {
    ...board,
    columns: board.columns.map((c) =>
      c.id !== col.id
        ? c
        : {
            ...c,
            tasks: c.tasks.map((t) =>
              t.id !== task.id
                ? t
                : {
                    ...t,
                    title: task.title,
                    priority: task.priority as TaskCard['priority'],
                    assignee_id: task.assignee_id,
                    start_date: task.start_date ?? null,
                    due_date: task.due_date ?? null,
                    estimate: task.estimate ?? null,
                    epic_id: task.epic_id ?? null,
                    release_id: task.release_id ?? null,
                    version: task.version,
                  },
            ),
          },
    ),
  };
}
