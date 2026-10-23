import { describe, expect, it } from 'vitest';

import { fromViewFilters, sameView, toViewFilters, type BoardViewState } from './saved-views';

const state: BoardViewState = {
  filter: { text: ' login ', priority: 'high', assignee: 'me', sprint: 'backlog', epic: '' },
  groupBy: 'epic',
};

describe('saved views', () => {
  it('round-trips the board state, trimming the text', () => {
    const f = toViewFilters(state);
    expect(f).toEqual({ text: 'login', priority: 'high', assignee: 'me', sprint: 'backlog', epic: '', group_by: 'epic' });
    expect(fromViewFilters(f)).toEqual({ ...state, filter: { ...state.filter, text: 'login' } });
  });

  it('tells whether the board still matches the view', () => {
    const f = toViewFilters(state);
    expect(sameView(state, f)).toBe(true);
    expect(sameView({ ...state, groupBy: 'none' }, f)).toBe(false);
    expect(sameView({ ...state, filter: { ...state.filter, priority: '' } }, f)).toBe(false);
  });

  it('fills defaults for fields an older view lacks', () => {
    expect(fromViewFilters({ text: 'x' } as never)).toEqual({
      filter: { text: 'x', priority: '', assignee: 'any', sprint: '', epic: '' },
      groupBy: 'none',
    });
  });
});
