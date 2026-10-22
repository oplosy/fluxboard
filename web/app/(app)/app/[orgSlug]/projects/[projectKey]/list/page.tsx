'use client';

import { use, useMemo, useState } from 'react';
import Link from 'next/link';
import { useQuery } from '@tanstack/react-query';

import { useOrg } from '@/lib/org/context';
import { useProjectByKey } from '@/lib/board/use-project';
import { getBoard } from '@/lib/api/board';
import { ProjectFrame } from '@/app/(app)/app/[orgSlug]/projects/[projectKey]/_components/project-frame';
import { priorityClass, priorityLabel, PRIORITIES } from '@/lib/board/priority';
import { statusTone, type StatusTone } from '@/lib/board/status';
import { memberName, useOrgMembers } from '@/lib/org/use-members';
import { useOrgAgents } from '@/lib/org/use-agents';
import type { TaskCard } from '@/lib/api/types';
import { Avatar } from '@/components/ui/avatar';
import { cn } from '@/lib/utils';

type SortKey = 'number' | 'title' | 'priority';

const PRIORITY_ORDER = new Map(PRIORITIES.map((p, i) => [p, i]));

interface Group {
  id: string;
  name: string;
  tone: StatusTone;
  tasks: TaskCard[];
}

// Monday-style list: one group per board column (status), each with a coloured
// edge and count; rows sort within their group.
export default function ProjectListPage(props: { params: Promise<{ projectKey: string }> }) {
  const params = use(props.params);
  const { orgId, slug } = useOrg();
  const { project, isLoading, isError } = useProjectByKey(params.projectKey);
  const { byId } = useOrgMembers();
  const { names: agentNames } = useOrgAgents();

  const board = useQuery({
    queryKey: ['board', project?.id],
    queryFn: () => getBoard(orgId, project!.id),
    enabled: Boolean(project),
  });

  const [sort, setSort] = useState<SortKey>('number');
  const [asc, setAsc] = useState(true);

  const groups = useMemo<Group[]>(() => {
    const data = board.data;
    if (!data) return [];
    const dir = asc ? 1 : -1;
    const cmp = (a: TaskCard, b: TaskCard) => {
      switch (sort) {
        case 'title':
          return dir * a.title.localeCompare(b.title);
        case 'priority':
          return dir * ((PRIORITY_ORDER.get(a.priority) ?? 99) - (PRIORITY_ORDER.get(b.priority) ?? 99));
        default:
          return dir * (a.number - b.number);
      }
    };
    return data.columns.map((col) => ({
      id: col.id,
      name: col.name,
      tone: statusTone(data, col.id),
      tasks: [...col.tasks].sort(cmp),
    }));
  }, [board.data, sort, asc]);

  if (isLoading) return <p className="px-6 py-8 text-sm text-muted-foreground">Loading project…</p>;
  if (isError || !project)
    return (
      <div className="mx-auto max-w-md px-6 py-16 text-center">
        <h1 className="text-lg font-semibold">Project not found</h1>
        <Link href={`/app/${slug}/projects`} className="mt-4 inline-block text-sm text-primary hover:underline">
          Back to projects
        </Link>
      </div>
    );

  function header(key: SortKey, label: string) {
    return (
      <button
        onClick={() => (sort === key ? setAsc((v) => !v) : (setSort(key), setAsc(true)))}
        className="flex items-center gap-1 font-medium hover:text-foreground"
      >
        {label}
        {sort === key ? <span className="text-[10px]">{asc ? '▲' : '▼'}</span> : null}
      </button>
    );
  }

  const total = groups.reduce((n, g) => n + g.tasks.length, 0);

  return (
    <ProjectFrame project={project}>
      {board.isLoading ? (
        <p className="text-sm text-muted-foreground">Loading tasks…</p>
      ) : total === 0 ? (
        <div className="rounded-lg border border-dashed border-border px-6 py-12 text-center">
          <p className="text-sm font-medium">No tasks yet</p>
          <p className="mt-1 text-sm text-muted-foreground">Tasks you add on the board show up here, grouped by status.</p>
        </div>
      ) : (
        <div className="space-y-6">
          {groups.map((g) => (
            <section key={g.id}>
              <div className="mb-1.5 flex items-center gap-2 px-1">
                <span className={cn('h-2 w-2 rounded-full', g.tone.dot)} aria-hidden />
                <h2 className="font-sans text-[13px] font-semibold">{g.name}</h2>
                <span className="text-xs tabular-nums text-muted-foreground">{g.tasks.length}</span>
              </div>
              <div className="relative overflow-x-auto rounded-lg border border-border bg-card">
                <span className={cn('absolute inset-y-0 left-0 w-[3px]', g.tone.dot)} aria-hidden />
                <table className="w-full min-w-[640px] text-[13px]">
                  <thead className="text-left text-xs text-muted-foreground">
                    <tr className="border-b border-border">
                      <th className="py-2 pl-4 pr-3 font-medium">{header('title', 'Task')}</th>
                      <th className="w-32 px-3 py-2 font-medium">Status</th>
                      <th className="w-28 px-3 py-2">{header('priority', 'Priority')}</th>
                      <th className="w-44 px-3 py-2 font-medium">Assignee</th>
                      <th className="w-24 px-3 py-2 font-medium">Due</th>
                    </tr>
                  </thead>
                  <tbody>
                    {g.tasks.length === 0 ? (
                      <tr>
                        <td colSpan={5} className="py-3 pl-4 text-xs text-muted-foreground">
                          No tasks in {g.name}
                        </td>
                      </tr>
                    ) : null}
                    {g.tasks.map((t) => (
                      <tr key={t.id} className="border-b border-border/60 transition-colors last:border-0 hover:bg-secondary/40">
                        <td className="py-2 pl-4 pr-3">
                          <Link
                            href={`/app/${slug}/projects/${project.key}/tasks/${t.number}`}
                            className="flex min-w-0 items-baseline gap-2 hover:underline hover:underline-offset-2"
                          >
                            <span className="shrink-0 font-mono text-[11px] text-muted-foreground">
                              {project.key}-{t.number}
                            </span>
                            <span className="truncate">{t.title}</span>
                          </Link>
                        </td>
                        <td className="px-3 py-2">
                          <span className={cn('inline-block max-w-full truncate rounded-full px-2 py-0.5 text-[11px] font-medium', g.tone.pill)}>
                            {g.name}
                          </span>
                        </td>
                        <td className="px-3 py-2">
                          {t.priority !== 'none' ? (
                            <span className={cn('rounded-full px-2 py-0.5 text-[11px] font-medium', priorityClass(t.priority))}>
                              {priorityLabel(t.priority)}
                            </span>
                          ) : (
                            <span className="text-xs text-muted-foreground">—</span>
                          )}
                        </td>
                        <td className="px-3 py-2">
                          {t.assignee_id ? (
                            <span className="flex min-w-0 items-center gap-2">
                              <Avatar name={memberName(byId, t.assignee_id, agentNames)} size="sm" />
                              <span className="truncate">{memberName(byId, t.assignee_id, agentNames)}</span>
                            </span>
                          ) : (
                            <span className="text-xs text-muted-foreground">Unassigned</span>
                          )}
                        </td>
                        <td className="px-3 py-2 text-xs text-muted-foreground">
                          {t.due_date
                            ? new Date(t.due_date).toLocaleDateString(undefined, { month: 'short', day: 'numeric' })
                            : '—'}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </section>
          ))}
        </div>
      )}
    </ProjectFrame>
  );
}
