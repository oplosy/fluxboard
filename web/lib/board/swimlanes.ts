import type { Priority, TaskCard } from '@/lib/api/types';
import { PRIORITIES, priorityLabel } from '@/lib/board/priority';

// Board swimlanes (FR-PROJ-009): horizontal lanes by assignee, epic or
// priority. A lane key is the task's value for that field, or NO_VALUE when
// it has none; dropping a card into another lane sets the field to the key.

export type GroupBy = 'none' | 'assignee' | 'epic' | 'priority';

export const NO_VALUE = 'none';

export interface Lane {
  key: string;
  label: string;
}

export interface LaneContext {
  members: { user_id: string; name: string }[];
  epics: { id: string; name: string }[];
}

/** The lane a task sits in. */
export function laneKeyOf(task: TaskCard, by: GroupBy): string {
  if (by === 'assignee') return task.assignee_id || NO_VALUE;
  if (by === 'epic') return task.epic_id || NO_VALUE;
  if (by === 'priority') return task.priority;
  return NO_VALUE;
}

/**
 * Lanes that hold at least one of tasks, in display order: assignees by name,
 * epics in their own order, priorities from urgent to none; the "no value"
 * lane comes last.
 */
export function lanesFor(tasks: TaskCard[], by: GroupBy, ctx: LaneContext): Lane[] {
  if (by === 'none') return [];
  const used = new Set(tasks.map((t) => laneKeyOf(t, by)));
  if (by === 'priority') {
    return PRIORITIES.filter((p) => used.has(p)).map((p) => ({ key: p, label: priorityLabel(p as Priority) }));
  }
  let lanes: Lane[];
  if (by === 'assignee') {
    const names = new Map(ctx.members.map((m) => [m.user_id, m.name]));
    lanes = [...used]
      .filter((k) => k !== NO_VALUE)
      .map((k) => ({ key: k, label: names.get(k) ?? 'Unknown user' }))
      .sort((a, b) => a.label.localeCompare(b.label));
  } else {
    lanes = ctx.epics.filter((e) => used.has(e.id)).map((e) => ({ key: e.id, label: e.name }));
    // A task may point at an epic missing from the list (deleted meanwhile).
    for (const k of used) {
      if (k !== NO_VALUE && !lanes.some((l) => l.key === k)) lanes.push({ key: k, label: 'Unknown epic' });
    }
  }
  if (used.has(NO_VALUE)) lanes.push({ key: NO_VALUE, label: by === 'assignee' ? 'Unassigned' : 'No epic' });
  return lanes;
}

// Droppable id of one column inside one lane; card ids never contain "|".
export function laneDropId(laneKey: string, columnId: string): string {
  return `lane|${laneKey}|${columnId}`;
}

export function parseLaneDropId(id: string): { laneKey: string; columnId: string } | null {
  const parts = id.split('|');
  return parts.length === 3 && parts[0] === 'lane' ? { laneKey: parts[1] as string, columnId: parts[2] as string } : null;
}
