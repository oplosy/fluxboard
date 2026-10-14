'use client';

import Link from 'next/link';
import type { ComponentProps } from 'react';

import type { Priority, Project, Task } from '@/lib/api/types';
import { priorityLabel, priorityLevel } from '@/lib/board/priority';
import { memberName, useOrgMembers } from '@/lib/org/use-members';
import { useOrgAgents } from '@/lib/org/use-agents';
import { Avatar } from '@/components/ui/avatar';
import { cn } from '@/lib/utils';
import { Highlight } from './highlight';

export function taskHref(orgSlug: string, project: Project | undefined, task: Task): string | undefined {
  return project ? `/app/${orgSlug}/projects/${project.key}/tasks/${task.number}` : undefined;
}

/** Four-bar signal-strength glyph; muted when the task has no priority. */
function PriorityBars({ priority }: { priority: string }) {
  const level = priorityLevel(priority);
  if (level === 0) return null;
  const label = priorityLabel(priority as Priority);
  return (
    <span className="flex h-3.5 shrink-0 items-end gap-px" role="img" aria-label={`${label} priority`} title={label}>
      {[1, 2, 3, 4].map((n) => (
        <span
          key={n}
          style={{ height: `${n * 25}%` }}
          className={cn('w-[3px] rounded-[1px]', n <= level ? (level >= 3 ? 'bg-signal' : 'bg-foreground/70') : 'bg-border')}
        />
      ))}
    </span>
  );
}

type LinkProps = Omit<ComponentProps<typeof Link>, 'href' | 'children'>;

/** One task result: key, highlighted title, project, priority, assignee. Shared by the palette and the search page. */
export function SearchResultRow({
  task,
  project,
  orgSlug,
  query = '',
  active,
  ...linkProps
}: {
  task: Task;
  project?: Project;
  orgSlug: string;
  query?: string;
  active?: boolean;
} & LinkProps) {
  const { byId } = useOrgMembers();
  const { names: agentNames } = useOrgAgents();
  const href = taskHref(orgSlug, project, task) ?? `/app/${orgSlug}/projects`;
  const assignee = task.assignee_id ? memberName(byId, task.assignee_id, agentNames) : null;

  return (
    <Link
      {...linkProps}
      href={href}
      data-active={active ? 'true' : undefined}
      className={cn(
        'flex items-center gap-3 px-3 py-2.5 text-sm outline-none transition-colors',
        'hover:bg-secondary data-[active=true]:bg-secondary',
        linkProps.className,
      )}
    >
      <span className="w-16 shrink-0 truncate font-mono text-xs text-muted-foreground">
        {project ? `${project.key}-${task.number}` : `#${task.number}`}
      </span>
      <span className="min-w-0 flex-1">
        <span className="block truncate font-medium">
          <Highlight text={task.title} query={query} />
        </span>
        {project ? <span className="block truncate text-xs text-muted-foreground">{project.name}</span> : null}
      </span>
      <PriorityBars priority={task.priority} />
      {assignee ? <Avatar name={assignee} size="sm" className="ring-0" /> : <span className="h-6 w-6 shrink-0" aria-hidden />}
    </Link>
  );
}
