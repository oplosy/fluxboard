import type { Priority } from '@/lib/api/types';
import type { ViewFilters } from '@/lib/api/views';
import type { GroupBy } from '@/lib/board/swimlanes';

// Conversion between the board's filter state and a saved view (FR-PROJ-011).

export interface BoardViewState {
  filter: {
    text: string;
    priority: Priority | '';
    assignee: 'any' | 'me' | 'unassigned';
    sprint: string;
    epic: string;
  };
  groupBy: GroupBy;
}

export function toViewFilters(s: BoardViewState): ViewFilters {
  return { ...s.filter, text: s.filter.text.trim(), group_by: s.groupBy };
}

export function fromViewFilters(f: ViewFilters): BoardViewState {
  return {
    filter: {
      text: f.text ?? '',
      priority: (f.priority ?? '') as Priority | '',
      assignee: f.assignee ?? 'any',
      sprint: f.sprint ?? '',
      epic: f.epic ?? '',
    },
    groupBy: f.group_by ?? 'none',
  };
}

/** True when the board shows exactly what the view stores. */
export function sameView(s: BoardViewState, f: ViewFilters): boolean {
  const a = toViewFilters(s);
  const b = toViewFilters(fromViewFilters(f));
  return (Object.keys(a) as (keyof ViewFilters)[]).every((k) => a[k] === b[k]);
}
