import { describe, expect, it } from 'vitest';

import type { Board } from '@/lib/api/types';
import { doneColumnId, statusTone } from './status';

const board = (names: string[]) =>
  ({ columns: names.map((name, i) => ({ id: `c${i}`, name, tasks: [] })) }) as unknown as Board;

describe('status tones', () => {
  it('prefers a column named done, else the last one', () => {
    expect(doneColumnId(board(['To do', 'Done', 'Archive']))).toBe('c1');
    expect(doneColumnId(board(['Backlog', 'Doing', 'Live']))).toBe('c2');
  });

  it('keeps the first lane neutral and the done lane success', () => {
    const b = board(['To do', 'In progress', 'Review', 'Done']);
    expect(statusTone(b, 'c0').dot).toBe('bg-muted-foreground/60');
    expect(statusTone(b, 'c1').dot).toBe('bg-cobalt');
    expect(statusTone(b, 'c2').dot).toBe('bg-warning');
    expect(statusTone(b, 'c3').dot).toBe('bg-success');
  });
});
