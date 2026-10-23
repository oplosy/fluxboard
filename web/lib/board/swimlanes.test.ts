import { describe, expect, it } from 'vitest';

import type { TaskCard } from '@/lib/api/types';
import { laneDropId, laneKeyOf, lanesFor, parseLaneDropId } from './swimlanes';

function card(p: Partial<TaskCard>): TaskCard {
  return {
    id: 't', number: 1, title: 'x', priority: 'none', labels: [], comment_count: 0,
    subtask_total: 0, subtask_done: 0, created_at: '', rank: 'a', version: 1, ...p,
  } as TaskCard;
}

const ctx = {
  members: [
    { user_id: 'u-zed', name: 'Zed' },
    { user_id: 'u-amy', name: 'Amy' },
  ],
  epics: [
    { id: 'e2', name: 'Second' },
    { id: 'e1', name: 'First' },
  ],
};

describe('lanesFor', () => {
  it('orders assignees by name with Unassigned last, skipping empty lanes', () => {
    const tasks = [card({ assignee_id: 'u-zed' }), card({}), card({ assignee_id: 'u-amy' })];
    expect(lanesFor(tasks, 'assignee', ctx).map((l) => l.label)).toEqual(['Amy', 'Zed', 'Unassigned']);
  });

  it('keeps epic order and labels an epic missing from the list', () => {
    const tasks = [card({ epic_id: 'e1' }), card({ epic_id: 'e2' }), card({ epic_id: 'gone' }), card({})];
    expect(lanesFor(tasks, 'epic', ctx).map((l) => l.label)).toEqual(['Second', 'First', 'Unknown epic', 'No epic']);
  });

  it('orders priorities from urgent to none', () => {
    const tasks = [card({ priority: 'low' }), card({ priority: 'urgent' }), card({ priority: 'none' })];
    expect(lanesFor(tasks, 'priority', ctx).map((l) => l.key)).toEqual(['urgent', 'low', 'none']);
  });

  it('has no lanes when not grouping', () => {
    expect(lanesFor([card({})], 'none', ctx)).toEqual([]);
  });
});

describe('lane keys and drop ids', () => {
  it('maps a missing value to the none lane', () => {
    expect(laneKeyOf(card({ assignee_id: undefined }), 'assignee')).toBe('none');
    expect(laneKeyOf(card({ epic_id: null }), 'epic')).toBe('none');
  });

  it('round-trips a drop id and rejects a card id', () => {
    expect(parseLaneDropId(laneDropId('u-1', 'c-1'))).toEqual({ laneKey: 'u-1', columnId: 'c-1' });
    expect(parseLaneDropId('0190-card-id')).toBeNull();
  });
});
