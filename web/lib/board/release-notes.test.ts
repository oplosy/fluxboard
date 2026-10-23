import { describe, expect, it } from 'vitest';

import type { Board, TaskCard } from '@/lib/api/types';
import type { Release } from '@/lib/api/releases';
import { doneTasksOf, releaseNotesMarkdown } from './release-notes';

function card(p: Partial<TaskCard>): TaskCard {
  return {
    id: 't', number: 1, title: 'x', priority: 'none', labels: [], comment_count: 0,
    subtask_total: 0, subtask_done: 0, created_at: '', rank: 'a', version: 1, ...p,
  } as TaskCard;
}

const release: Release = {
  id: 'r1', name: 'v1.2', description: 'Search and export', release_date: '2026-11-04', released_at: null,
  total: 3, done: 2, points: 0, done_points: 0, created_at: '',
};

const board = {
  board_id: 'b',
  columns: [
    { id: 'todo', name: 'Todo', role: 'todo', tasks: [card({ id: 'a', number: 3, release_id: 'r1' })] },
    {
      id: 'shipped', name: 'Shipped', role: 'done',
      tasks: [card({ id: 'b', number: 7, title: 'Add export', release_id: 'r1' }), card({ id: 'c', number: 8, release_id: 'r2' })],
    },
    { id: 'last', name: 'Archive', tasks: [card({ id: 'd', number: 9, release_id: 'r1' })] },
  ],
} as unknown as Board;

describe('release notes', () => {
  it('uses the column with the done role, not the last column', () => {
    expect(doneTasksOf(board, 'r1').map((t) => t.number)).toEqual([7]);
  });

  it('renders a Markdown list with the planned date and description', () => {
    expect(releaseNotesMarkdown(release, doneTasksOf(board, 'r1'), 'WEB')).toBe(
      '## v1.2 (2026-11-04)\n\nSearch and export\n\n- WEB-7: Add export',
    );
  });

  it('prefers the released day and says when nothing is done', () => {
    const shipped = { ...release, description: '', released_at: '2026-11-06T10:00:00Z' };
    expect(releaseNotesMarkdown(shipped, [], 'WEB')).toBe('## v1.2 (2026-11-06)\n\n_No completed tasks yet._');
  });
});
