'use client';

import type { ReactNode } from 'react';
import { Check, CheckSquare, ChevronDown, Search, User, Users, X, Zap } from 'lucide-react';

import type { BoardColumn, Label, Priority } from '@/lib/api/types';
import type { Sprint } from '@/lib/api/sprints';
import { PRIORITIES, priorityLabel, priorityLevel } from '@/lib/board/priority';
import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import { Menu, MenuItem } from '@/components/ui/menu';

export interface BoardFilter {
  text: string;
  priority: Priority | '';
  assignee: 'any' | 'me' | 'unassigned';
  sprint: '' | 'backlog' | string;
}

export const EMPTY_FILTER: BoardFilter = { text: '', priority: '', assignee: 'any', sprint: '' };

interface PillOption {
  value: string;
  label: string;
  icon?: ReactNode;
}

// Signal-strength glyph: lit bars = priority level.
function PriorityGlyph({ priority }: { priority: Priority | '' }) {
  const level = priority ? priorityLevel(priority) : 0;
  return (
    <span className="inline-flex h-3.5 items-end gap-[2px]" aria-hidden>
      {[1, 2, 3, 4].map((n) => (
        <span
          key={n}
          style={{ height: `${n * 25}%` }}
          className={cn('w-[3px] rounded-[1px]', n <= level ? 'bg-signal' : 'bg-foreground/20')}
        />
      ))}
    </span>
  );
}

// A compact filter control: label + current value, highlighted when it is
// narrowing the board, with an inline × to reset it.
function FilterPill({
  label,
  icon,
  value,
  defaultValue,
  options,
  onChange,
}: {
  label: string;
  icon: ReactNode;
  value: string;
  defaultValue: string;
  options: PillOption[];
  onChange: (value: string) => void;
}) {
  const active = value !== defaultValue;
  const current = options.find((o) => o.value === value);

  return (
    <div
      className={cn(
        'inline-flex h-8 items-stretch rounded-md border text-[13px] transition-colors duration-200',
        active
          ? 'border-foreground/50 bg-secondary text-foreground'
          : 'border-dashed border-input bg-transparent text-muted-foreground hover:border-foreground/40 hover:text-foreground',
      )}
    >
      <Menu
        aria-label={`${label} filter`}
        trigger={
          <button
            type="button"
            aria-label={`${label} filter${active && current ? `: ${current.label}` : ''}`}
            className={cn(
              'inline-flex h-full items-center gap-1.5 px-2.5 outline-none',
              'focus-visible:shadow-[inset_0_-2px_0_hsl(var(--signal))]',
              active ? 'rounded-l-[5px]' : 'rounded-[5px]',
            )}
          >
            <span className="inline-flex shrink-0 items-center [&>svg]:h-3.5 [&>svg]:w-3.5">
              {active && current?.icon ? current.icon : icon}
            </span>
            <span>{label}</span>
            {active && current ? (
              <>
                <span className="h-3.5 w-px bg-foreground/20" aria-hidden />
                <span className="max-w-[9rem] truncate font-medium">{current.label}</span>
              </>
            ) : (
              <ChevronDown className="h-3.5 w-3.5 opacity-60" aria-hidden />
            )}
          </button>
        }
      >
        {options.map((o) => (
          <MenuItem key={o.value} icon={o.icon} onSelect={() => onChange(o.value)}>
            <span className="flex items-center justify-between gap-4">
              <span className="truncate">{o.label}</span>
              {o.value === value ? <Check className="h-3.5 w-3.5 shrink-0 text-foreground" aria-hidden /> : null}
            </span>
          </MenuItem>
        ))}
      </Menu>
      {active ? (
        <button
          type="button"
          onClick={() => onChange(defaultValue)}
          aria-label={`Clear ${label.toLowerCase()} filter`}
          className="inline-flex w-7 items-center justify-center rounded-r-[5px] border-l border-foreground/15 text-muted-foreground outline-none transition-colors hover:bg-foreground/10 hover:text-foreground focus-visible:shadow-[inset_0_-2px_0_hsl(var(--signal))]"
        >
          <X className="h-3.5 w-3.5" aria-hidden />
        </button>
      ) : null}
    </div>
  );
}

// Board toolbar: client-side filters (text / priority / assignee / sprint — the
// board projection carries no labels, so label is not a filter, ADR-017) plus
// the multi-select mode toggle and, when a selection exists, the bulk action bar.
export function BoardToolbar({
  filter,
  onFilter,
  selectMode,
  onToggleSelectMode,
  selectedCount,
  columns,
  labels,
  sprints,
  bulkPending,
  onBulkMove,
  onBulkAssignMe,
  onBulkUnassign,
  onBulkLabel,
  onClearSelection,
}: {
  filter: BoardFilter;
  onFilter: (f: BoardFilter) => void;
  selectMode: boolean;
  onToggleSelectMode: () => void;
  selectedCount: number;
  columns: BoardColumn[];
  labels: Label[];
  sprints: Sprint[];
  bulkPending: boolean;
  onBulkMove: (columnId: string) => void;
  onBulkAssignMe: () => void;
  onBulkUnassign: () => void;
  onBulkLabel: (labelId: string) => void;
  onClearSelection: () => void;
}) {
  const activeCount =
    (filter.text.trim() ? 1 : 0) +
    (filter.priority ? 1 : 0) +
    (filter.assignee !== 'any' ? 1 : 0) +
    (filter.sprint ? 1 : 0);

  return (
    <div className="mb-4 space-y-3">
      <div className="flex flex-wrap items-center gap-2">
        <div className="group relative w-full sm:w-64">
          <Search
            className="pointer-events-none absolute left-2.5 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-muted-foreground"
            aria-hidden
          />
          <input
            id="board-filter"
            type="text"
            value={filter.text}
            onChange={(e) => onFilter({ ...filter, text: e.target.value })}
            placeholder="Filter tasks"
            aria-label="Filter tasks by text"
            className={cn(
              'h-8 w-full rounded-md border border-input bg-card pl-8 pr-8 text-[13px] text-foreground',
              'placeholder:text-muted-foreground/80',
              'transition-[border-color,box-shadow] duration-300 hover:border-foreground/40',
              'focus-visible:border-foreground focus-visible:shadow-[inset_0_-2px_0_hsl(var(--signal))] focus-visible:outline-none',
            )}
          />
          {filter.text ? (
            <button
              type="button"
              onClick={() => onFilter({ ...filter, text: '' })}
              aria-label="Clear text filter"
              className="absolute right-1.5 top-1/2 flex h-5 w-5 -translate-y-1/2 items-center justify-center rounded-sm text-muted-foreground hover:bg-secondary hover:text-foreground"
            >
              <X className="h-3.5 w-3.5" aria-hidden />
            </button>
          ) : (
            <kbd
              aria-hidden
              className="pointer-events-none absolute right-2 top-1/2 hidden h-5 min-w-5 -translate-y-1/2 items-center justify-center rounded-sm border border-border bg-secondary px-1 font-mono text-[11px] text-muted-foreground group-focus-within:hidden sm:flex"
            >
              /
            </kbd>
          )}
        </div>

        <span className="mx-0.5 hidden h-5 w-px bg-border sm:block" aria-hidden />

        <FilterPill
          label="Priority"
          icon={<PriorityGlyph priority="" />}
          value={filter.priority}
          defaultValue=""
          onChange={(v) => onFilter({ ...filter, priority: v as Priority | '' })}
          options={[
            { value: '', label: 'Any priority' },
            ...PRIORITIES.map((p) => ({ value: p, label: priorityLabel(p), icon: <PriorityGlyph priority={p} /> })),
          ]}
        />
        <FilterPill
          label="Assignee"
          icon={<Users />}
          value={filter.assignee}
          defaultValue="any"
          onChange={(v) => onFilter({ ...filter, assignee: v as BoardFilter['assignee'] })}
          options={[
            { value: 'any', label: 'Anyone', icon: <Users /> },
            { value: 'me', label: 'Assigned to me', icon: <User /> },
            { value: 'unassigned', label: 'Unassigned', icon: <User className="opacity-50" /> },
          ]}
        />
        <FilterPill
          label="Sprint"
          icon={<Zap />}
          value={filter.sprint}
          defaultValue=""
          onChange={(v) => onFilter({ ...filter, sprint: v })}
          options={[
            { value: '', label: 'All sprints' },
            { value: 'backlog', label: 'Backlog' },
            ...sprints.map((s) => ({ value: s.id, label: s.name })),
          ]}
        />

        {activeCount > 0 ? (
          <button
            type="button"
            onClick={() => onFilter(EMPTY_FILTER)}
            className="inline-flex h-8 items-center gap-1.5 rounded-md px-2 text-[13px] text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground"
          >
            <X className="h-3.5 w-3.5" aria-hidden />
            Clear filters
            <span className="inline-flex h-4 min-w-4 items-center justify-center rounded-full bg-foreground px-1 text-[10px] font-medium text-background">
              {activeCount}
            </span>
          </button>
        ) : null}

        <Button
          size="sm"
          variant={selectMode ? 'default' : 'outline'}
          onClick={onToggleSelectMode}
          aria-pressed={selectMode}
          className="ml-auto"
        >
          <CheckSquare className="h-3.5 w-3.5" /> {selectMode ? 'Selecting' : 'Select'}
        </Button>
      </div>

      {selectMode && selectedCount > 0 ? (
        <div className="flex flex-wrap items-center gap-2 rounded-lg border border-border bg-card px-3 py-2">
          <span className="mr-1 text-[13px] font-medium">{selectedCount} selected</span>

          <Menu
            aria-label="Move selected tasks"
            trigger={
              <Button size="sm" variant="outline" disabled={bulkPending}>
                Move to <ChevronDown className="h-3.5 w-3.5" />
              </Button>
            }
          >
            {columns.map((c) => (
              <MenuItem key={c.id} onSelect={() => onBulkMove(c.id)}>
                {c.name}
              </MenuItem>
            ))}
          </Menu>

          <Button size="sm" variant="outline" onClick={onBulkAssignMe} disabled={bulkPending}>
            Assign to me
          </Button>
          <Button size="sm" variant="outline" onClick={onBulkUnassign} disabled={bulkPending}>
            Unassign
          </Button>

          {labels.length > 0 ? (
            <Menu
              aria-label="Add label to selected tasks"
              trigger={
                <Button size="sm" variant="outline" disabled={bulkPending}>
                  Add label <ChevronDown className="h-3.5 w-3.5" />
                </Button>
              }
            >
              {labels.map((l) => (
                <MenuItem
                  key={l.id}
                  onSelect={() => onBulkLabel(l.id)}
                  icon={
                    <span
                      className="block h-2.5 w-2.5 rounded-full"
                      style={{ backgroundColor: l.color || 'hsl(var(--muted-foreground))' }}
                    />
                  }
                >
                  {l.name}
                </MenuItem>
              ))}
            </Menu>
          ) : null}

          <Button size="sm" variant="ghost" onClick={onClearSelection} className="ml-auto">
            <X className="h-3.5 w-3.5" /> Clear
          </Button>
        </div>
      ) : null}
    </div>
  );
}
