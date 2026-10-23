import { describe, expect, it } from 'vitest';

import type { Board, Task, TaskCard } from '@/lib/api/types';
import { moveCard, patchCard } from './board-cache';

function card(id: string, rank: string, p: Partial<TaskCard> = {}): TaskCard {
  return {
    id, number: 1, title: id, priority: 'none', labels: [], comment_count: 0,
    subtask_total: 0, subtask_done: 0, created_at: '', rank, version: 1, ...p,
  } as TaskCard;
}

const board = {
  board_id: 'b',
  columns: [
    { id: 'todo', name: 'Todo', tasks: [card('a', 'b'), card('b', 'd')] },
    { id: 'done', name: 'Done', tasks: [card('c', 'c')] },
  ],
} as unknown as Board;

describe('moveCard', () => {
  it('moves a card and keeps the destination sorted by rank', () => {
    const next = moveCard(board, 'a', 'done', 'd')!;
    expect(next.columns[0]!.tasks.map((t) => t.id)).toEqual(['b']);
    expect(next.columns[1]!.tasks.map((t) => [t.id, t.rank])).toEqual([['c', 'c'], ['a', 'd']]);
  });

  it('returns null for an unknown card or column', () => {
    expect(moveCard(board, 'zz', 'done', 'a')).toBeNull();
    expect(moveCard(board, 'a', 'gone', 'a')).toBeNull();
  });
});

describe('patchCard', () => {
  const task = { id: 'b', column_id: 'todo', title: 'Renamed', priority: 'high', estimate: 5, version: 4 } as unknown as Task;

  it('copies the editable fields onto the card', () => {
    const t = patchCard(board, task)!.columns[0]!.tasks[1]!;
    expect([t.title, t.priority, t.estimate, t.version, t.rank]).toEqual(['Renamed', 'high', 5, 4, 'd']);
  });

  it('returns null when the task moved to a column the cache does not know of', () => {
    expect(patchCard(board, { ...task, column_id: 'done' })).toBeNull();
    expect(patchCard(board, { ...task, id: 'zz' })).toBeNull();
  });
});
