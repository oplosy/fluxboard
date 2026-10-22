import type { Board } from '@/lib/api/types';

// Status (board column) colours, shared by the board, list and task detail so a
// lane reads the same everywhere: first lane neutral, done lane success, the
// lanes in between cycle through the accent tokens.

export interface StatusTone {
  /** Solid dot / bar. */
  dot: string;
  /** Tinted pill (background + text). */
  pill: string;
}

const NEUTRAL: StatusTone = { dot: 'bg-muted-foreground/60', pill: 'bg-secondary text-secondary-foreground' };
const DONE: StatusTone = { dot: 'bg-success', pill: 'bg-success/15 text-success' };
const MIDDLE: StatusTone[] = [
  { dot: 'bg-cobalt', pill: 'bg-cobalt/12 text-cobalt' },
  { dot: 'bg-warning', pill: 'bg-warning/20 text-foreground' },
  { dot: 'bg-signal', pill: 'bg-signal/15 text-signal-ink' },
];

// A column counts as "done" when its name says so; otherwise the last column is.
export function doneColumnId(board: Board): string | undefined {
  const named = board.columns.find((c) => /\b(done|complete[d]?|shipped|closed|released)\b/i.test(c.name));
  return (named ?? board.columns[board.columns.length - 1])?.id;
}

export function statusTone(board: Board, columnId: string): StatusTone {
  if (columnId === doneColumnId(board)) return DONE;
  const i = board.columns.findIndex((c) => c.id === columnId);
  if (i <= 0) return NEUTRAL;
  return MIDDLE[(i - 1) % MIDDLE.length] as StatusTone;
}
