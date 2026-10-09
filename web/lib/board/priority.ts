import type { Priority } from '@/lib/api/types';

// Shared priority presentation for board cards, the list view and filters.

export const PRIORITIES: Priority[] = ['urgent', 'high', 'medium', 'low', 'none'];

const STYLES: Record<Priority, string> = {
  urgent: 'bg-signal text-signal-foreground',
  high: 'bg-signal/15 text-signal-ink',
  medium: 'bg-warning/20 text-foreground',
  low: 'bg-cobalt/12 text-cobalt',
  none: 'bg-secondary text-muted-foreground',
};

export function priorityClass(p: Priority): string {
  return STYLES[p] ?? STYLES.none;
}

export function priorityLabel(p: Priority): string {
  return p === 'none' ? 'No priority' : p.charAt(0).toUpperCase() + p.slice(1);
}

/** Number of lit bars (0–4) in the signal-strength priority glyph. */
export function priorityLevel(p: Priority | string): number {
  return p === 'urgent' ? 4 : p === 'high' ? 3 : p === 'medium' ? 2 : p === 'low' ? 1 : 0;
}
